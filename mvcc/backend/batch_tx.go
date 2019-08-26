package backend

import (
	"sync"

	bolt "go.etcd.io/bbolt"
	"go.uber.org/zap"
)

// 批量读写事务接口
type BatchTx interface {
	ReadTx                          //内嵌了只读事务接口
	UnsafeCreateBucket(name []byte) // 创建bucket
	// UnsafePut(bucketName []byte, key []byte, value []byte)	//向指定bucket中添加键值对
	// UnsafeSeqPut(bucketName []byte, key []byte, value []byte)	//向指定bucket中添加键值对，和unsafeput不同的是会设置填充比例，提高bucket利用率
	// UnsafeDelete(bucketName []byte, key []byte)		// 删除指定的键值对

	// 提交当前的读写事务，之后立即打开一个新的读写事务
	// Commit()

	// 提交当前的读写事务，之后并不打开新的读写事务
	// CommitAndStop()
}

type batchTx struct {
	sync.Mutex //互斥锁 ,防止struct被多线程修改数据
	tx         *bolt.Tx
	backend    *backend

	pending int
}

func (t *batchTx) Lock() {
	t.Mutex.Lock()
}

func (t *batchTx) Unlock() {
	// if t.pending >= t.backend.batchLimit {
	// 	t.commit(false)
	// }
	t.Mutex.Unlock()
}

// BatchTx 接口内嵌了 ReadTx接口.  但是 RLock() and RUnlock() 不需要
func (t *batchTx) RLock() {
	panic("unexpected RLock")
}

func (t *batchTx) RUnlock() {
	panic("unexpected RUnlock")
}

type batchTxBuffered struct {
	batchTx
	buf txWriteBuffer
}

func (t *batchTx) UnsafeCreateBucket(name []byte) {
	// 创建库
	_, err := t.tx.CreateBucket(name)
	// 如果有错误，并且错误不是库已存在
	if err != nil && err != bolt.ErrBucketExists {
		if t.backend.lg != nil {
			t.backend.lg.Fatal(
				"failed to create a bucket",
				zap.String("bucket-name", string(name)),
				zap.Error(err),
			)
		}
	}
	t.pending++
}

// 返回batchTxBuffered对象
func newBatchTxBuffered(backend *backend) *batchTxBuffered {
	tx := &batchTxBuffered{
		batchTx: batchTx{backend: backend},
		buf: txWriteBuffer{
			txBuffer: txBuffer{make(map[string]*bucketBuffer)},
			seq:      true,
		},
	}
	tx.Commit()
	return tx
}

func (t *batchTxBuffered) Commit() {
	t.Lock()
	t.commit(false)
	t.Unlock()
}

func (t *batchTxBuffered) commit(stop bool) {
	// 所有的读tx必须关闭才能获取bolt commit
	t.backend.readTx.Lock()
	t.unsafeCommit(stop)
	t.backend.readTx.Unlock()
}

func (t *batchTxBuffered) unsafeCommit(stop bool) {
	// 后续用到再看

	if t.backend.readTx.tx != nil {
		// wait all store read transactions using the current boltdb tx to finish,
		// then close the boltdb tx
		go func(tx *bolt.Tx, wg *sync.WaitGroup) {
			wg.Wait()
			if err := tx.Rollback(); err != nil {
				if t.backend.lg != nil {
					t.backend.lg.Fatal("failed to rollback tx", zap.Error(err))
				}
			}
		}(t.backend.readTx.tx, t.backend.readTx.txWg)
		t.backend.readTx.reset()
	}

	// t.batchTx.commit(stop)

	// if !stop {
	// 	t.backend.readTx.tx = t.backend.begin(false)
	// }
}
