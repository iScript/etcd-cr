package backend

import (
	"fmt"
	"sync/atomic"
	"time"

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

	// minSnapshotWarningTimeout is the minimum threshold to trigger a long running snapshot warning.
	minSnapshotWarningTimeout = 30 * time.Second
)

type Backend interface {
	// ReadTx returns a read transaction. It is replaced by ConcurrentReadTx in the main data path, see #10523.
	// ReadTx() ReadTx
	// BatchTx() BatchTx
	// // ConcurrentReadTx returns a non-blocking read transaction.
	// ConcurrentReadTx() ReadTx

	// Snapshot() Snapshot
	// Hash(ignores map[IgnoreKey]struct{}) (uint32, error)
	// Size returns the current size of the backend physically allocated.
	// The backend can hold DB space that is not utilized at the moment,
	// since it can conduct pre-allocation or spare unused space for recycling.
	// Use SizeInUse() instead for the actual DB size.
	Size() int64
	// SizeInUse returns the current size of the backend logically in use.
	// Since the backend can manage free space in a non-byte unit such as
	// number of pages, the returned value can be not exactly accurate in bytes.
	//SizeInUse() int64
	// OpenReadTxN returns the number of currently open read transactions in the backend.
	// OpenReadTxN() int64
	// Defrag() error
	// ForceCommit()
	Close() error
}

type backend struct {
	// size and commits are used with atomic operations so they must be
	// 64-bit aligned, otherwise 32-bit tests will crash

	// size is the number of bytes allocated in the backend
	size int64
	// sizeInUse is the number of bytes actually used in the backend
	sizeInUse int64
	// commits counts number of commits since start
	commits int64
	// openReadTxN is the number of currently open read transactions in the backend
	openReadTxN int64

	// mu sync.RWMutex
	db *bolt.DB

	batchInterval time.Duration
	batchLimit    int
	//batchTx       *batchTxBuffered

	// readTx *readTx

	stopc chan struct{}
	donec chan struct{}

	lg *zap.Logger
}

type BackendConfig struct {
	// Path is the file path to the backend file.
	Path string
	// BatchInterval is the maximum time before flushing the BatchTx.
	BatchInterval time.Duration
	// BatchLimit is the maximum puts before flushing the BatchTx.
	BatchLimit int
	// BackendFreelistType is the backend boltdb's freelist type.
	BackendFreelistType bolt.FreelistType
	// MmapSize is the number of bytes to mmap for the backend.
	MmapSize uint64
	// Logger logs backend-side operations.
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

	bopts.InitialMmapSize = bcfg.mmapSize()
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

		// readTx: &readTx{
		// 	buf: txReadBuffer{
		// 		txBuffer: txBuffer{make(map[string]*bucketBuffer)},
		// 	},
		// 	buckets: make(map[string]*bolt.Bucket),
		// 	txWg:    new(sync.WaitGroup),
		// },

		stopc: make(chan struct{}),
		donec: make(chan struct{}),

		lg: bcfg.Logger,
	}

	go b.run() //要加go，不然

	return b

}

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
