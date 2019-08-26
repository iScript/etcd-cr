package backend

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	humanize "github.com/dustin/go-humanize"
	bolt "go.etcd.io/bbolt" //应该是原先的bolt停止维护了，该bbolt是fork过来的
	"go.uber.org/zap"
)

var (
	defaultBatchLimit    = 10000
	defaultBatchInterval = 100 * time.Millisecond

	defragLimit = 10000

	// initialMmapSize is the initial size of the mmapped region. Setting this larger than
	// the potential max db size can prevent writer from blocking reader.
	// This only works for linux.
	initialMmapSize = uint64(10 * 1024 * 1024 * 1024)

	// 长时间运行快照警告的最小阈值
	minSnapshotWarningTimeout = 30 * time.Second
)

// 将底层存储与上层存储进行解耦
type Backend interface {
	// 创建一个只读事务
	// ReadTx() ReadTx

	// 创建一个批量事务
	BatchTx() BatchTx
	// // ConcurrentReadTx returns a non-blocking read transaction.
	// ConcurrentReadTx() ReadTx

	// 创建快照
	// Snapshot() Snapshot
	// Hash(ignores map[IgnoreKey]struct{}) (uint32, error)

	// 获取当前已存储的总字节数
	Size() int64
	// SizeInUse returns the current size of the backend logically in use.
	// Since the backend can manage free space in a non-byte unit such as
	// number of pages, the returned value can be not exactly accurate in bytes.
	//SizeInUse() int64
	// OpenReadTxN returns the number of currently open read transactions in the backend.
	// OpenReadTxN() int64
	//Defrag() error	//碎片整理
	// ForceCommit()	// 提交批量读写事务
	Close() error
}

// 快照接口
type Snapshot interface {
	//快照大小
	Size() int64

	// write快照到指定的writer.
	WriteTo(w io.Writer) (n int64, err error)

	// 关闭快照
	Close() error
}

type backend struct {
	// 当前backend实例分配的字节数
	size int64
	// 当前backend实例实际使用的字节数
	sizeInUse int64
	// 从启动到目前为止，已经提交的事务数
	commits int64
	// 当前只读事务
	openReadTxN int64

	// 读写锁
	mu sync.RWMutex

	// 底层的db存储
	db *bolt.DB

	batchInterval time.Duration    //2次批量读写事务提交的最大时间差
	batchLimit    int              // 一次批量事务中最大的事务数，当超过该阈值，当前批量事务会自动提交
	batchTx       *batchTxBuffered //批量读写事务，batchtxbuffer是在batchtx的基础上添加了缓存功能

	readTx *readTx // 只读事务

	stopc chan struct{}
	donec chan struct{}

	lg *zap.Logger
}

type BackendConfig struct {
	// 数据库文件路径
	Path string
	// 提交2次批量事务的最大时间差，默认100ms
	BatchInterval time.Duration
	// 每个批量读写事务能包含的最多的操作个数
	BatchLimit int
	//
	BackendFreelistType bolt.FreelistType
	// bolt使用了mmap技术，该字段用来设置mmap中使用的内存大小
	MmapSize uint64
	// log
	Logger *zap.Logger
}

func DefaultBackendConfig() BackendConfig {
	return BackendConfig{
		BatchInterval: defaultBatchInterval,
		BatchLimit:    defaultBatchLimit,
		MmapSize:      initialMmapSize,
	}
}

func New(bcfg BackendConfig) Backend {
	return newBackend(bcfg)
}

func NewDefaultBackend(path string) Backend {
	bcfg := DefaultBackendConfig()
	bcfg.Path = path
	return newBackend(bcfg)
}

func newBackend(bcfg BackendConfig) *backend {
	fmt.Println("new Backend", bcfg.Path)
	bopts := &bolt.Options{}

	if boltOpenOptions != nil {
		*bopts = *boltOpenOptions
	}

	bopts.InitialMmapSize = bcfg.mmapSize() //mmap使用的内存大小
	bopts.FreelistType = bcfg.BackendFreelistType

	db, err := bolt.Open(bcfg.Path, 0600, bopts) //在路径下打开一个db，若没有则创建一个

	if err != nil {
		if bcfg.Logger != nil {
			bcfg.Logger.Panic("failed to open database", zap.String("path", bcfg.Path), zap.Error(err))
		}
	}

	b := &backend{
		db: db,

		batchInterval: bcfg.BatchInterval,
		batchLimit:    bcfg.BatchLimit,

		// read_tx.go
		readTx: &readTx{
			buf: txReadBuffer{
				txBuffer: txBuffer{make(map[string]*bucketBuffer)},
			},
			buckets: make(map[string]*bolt.Bucket),
			txWg:    new(sync.WaitGroup),
		},

		stopc: make(chan struct{}),
		donec: make(chan struct{}),

		lg: bcfg.Logger,
	}

	b.batchTx = newBatchTxBuffered(b)
	go b.run() //启动一个单独的goroutine，其中会定时提交当前的批量读写事务，并开启新的批量读写事务

	return b

}

// 获取Batch ， 返回Batch接口
func (b *backend) BatchTx() BatchTx {
	return b.batchTx
}

// 返回只读事务实例
func (b *backend) ReadTx() ReadTx { return b.readTx }

// ConcurrentReadTx creates and returns a new ReadTx, which:
// A) creates and keeps a copy of backend.readTx.txReadBuffer,
// B) references the boltdb read Tx (and its bucket cache) of current batch interval.
// func (b *backend) ConcurrentReadTx() ReadTx {
// 	b.readTx.RLock()
// 	defer b.readTx.RUnlock()
// 	// prevent boltdb read Tx from been rolled back until store read Tx is done. Needs to be called when holding readTx.RLock().
// 	b.readTx.txWg.Add(1)
// 	// TODO: might want to copy the read buffer lazily - create copy when A) end of a write transaction B) end of a batch interval.
// 	return &concurrentReadTx{
// 		buf:     b.readTx.buf.unsafeCopy(),
// 		tx:      b.readTx.tx,
// 		txMu:    &b.readTx.txMu,
// 		buckets: b.readTx.buckets,
// 		txWg:    b.readTx.txWg,
// 	}
// }

// 提交当前的读写事务，并立即开启新的读写事务
func (b *backend) ForceCommit() {
	b.batchTx.Commit()
}

// 创建快照，用tx.writeto方法备份整个boltdb数据库
func (b *backend) Snapshot() Snapshot {
	b.batchTx.Commit()

	b.mu.RLock()
	defer b.mu.RUnlock()
	tx, err := b.db.Begin(false) //开启一个只读事务

	if err != nil {
		if b.lg != nil {
			b.lg.Fatal("failed to begin tx", zap.Error(err))
		}
	}

	stopc, donec := make(chan struct{}), make(chan struct{})
	dbBytes := tx.Size() //获取整个boltdb中保存的数据大小

	go func() {
		defer close(donec)

		var sendRateBytes int64 = 100 * 1024 * 1014 //假设发送快照的最小速度是100MB/s

		//总大小/速率 ， 需要多少秒
		warningTimeout := time.Duration(int64((float64(dbBytes) / float64(sendRateBytes)) * float64(time.Second)))
		//是否低于最小阈值30秒
		if warningTimeout < minSnapshotWarningTimeout {
			warningTimeout = minSnapshotWarningTimeout
		}

		start := time.Now()
		ticker := time.NewTicker(warningTimeout) // 定时器
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C: //定时器通道，超过warningtimeout。超时未发送完快照数据，则会输出警告
				if b.lg != nil {
					b.lg.Warn(
						"snapshotting taking too long to transfer",
						zap.Duration("taking", time.Since(start)),
						zap.Int64("bytes", dbBytes),
						zap.String("size", humanize.Bytes(uint64(dbBytes))),
					)
				}
			case <-stopc: // 快照发送结束
				// snapshotTransferSec.Observe(time.Since(start).Seconds())
				return
			}
		}
	}()

	return &snapshot{tx, stopc, donec}
}

type IgnoreKey struct {
	Bucket string
	Key    string
}

//func (b *backend) Hash(ignores map[IgnoreKey]struct{}) (uint32, error) {}

//返回size字段
func (b *backend) Size() int64 {
	return atomic.LoadInt64(&b.size)
}

func (b *backend) SizeInUse() int64 {
	return atomic.LoadInt64(&b.sizeInUse)
}

func (b *backend) run() {

	defer close(b.donec)                //关闭done channel
	t := time.NewTimer(b.batchInterval) //新建计时器，100毫秒后触发，通过t.C
	defer t.Stop()                      // 停止timer
	for {
		// 监听，如果都不满足case，则阻塞，满足继续执行下面的
		select {
		case <-t.C: //定时器时间到后接收,不做操作
		case <-b.stopc: //接收stop channel
			fmt.Println("mvcc backend run")
			//b.batchTx.CommitAndStop()
			return
		}

		// 提交当前的批量读写事务，并开启一个新的批量读写事务
		// if b.batchTx.safePending() != 0 {
		// 	b.batchTx.Commit()
		// }

		t.Reset(b.batchInterval) // timer需要reset实现定时器效果，不然只执行一次
	}

}

func (b *backend) Close() error {

	close(b.stopc) // 关闭stop通道，会通知select里的<-b.stopc
	<-b.donec
	return b.db.Close()
}

// 返回backend commit字段
func (b *backend) Commits() int64 {
	return atomic.LoadInt64(&b.commits)
}

// 整理碎片
// 其实就是通过创建新的数据库文件并将旧数据顺序写入新数据库，提高填充比例
func (b *backend) Defrag() error {
	return b.defrag()
}

func (b *backend) defrag() error {
	// 用到再看
	// ..
	return nil
}

type snapshot struct {
	*bolt.Tx
	stopc chan struct{}
	donec chan struct{}
}

func (s *snapshot) Close() error {
	close(s.stopc)
	<-s.donec
	return s.Tx.Rollback()
}
