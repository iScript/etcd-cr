package backend

import (
	"sync"

	bolt "go.etcd.io/bbolt"
)

// 只读事务接口
type ReadTx interface {
	Lock()
	Unlock()
	RLock()
	RUnlock()

	//在指定的bucket中进行范围查找
	//UnsafeRange(bucketName []byte, key, endKey []byte, limit int64) (keys [][]byte, vals [][]byte)

	// 遍历指定bucket中的全部键值对
	//UnsafeForEach(bucketName []byte, visitor func(k, v []byte) error) error
}

type readTx struct {
	//
	mu  sync.RWMutex //读写锁
	buf txReadBuffer

	// .
	txMu    sync.RWMutex
	tx      *bolt.Tx
	buckets map[string]*bolt.Bucket

	txWg *sync.WaitGroup
}

func (rt *readTx) Lock()    { rt.mu.Lock() }    // 写锁定
func (rt *readTx) Unlock()  { rt.mu.Unlock() }  // 写解锁
func (rt *readTx) RLock()   { rt.mu.RLock() }   // 读锁定
func (rt *readTx) RUnlock() { rt.mu.RUnlock() } // 读解锁

func (rt *readTx) reset() {
	rt.buf.reset()
	rt.buckets = make(map[string]*bolt.Bucket)
	rt.tx = nil
	rt.txWg = new(sync.WaitGroup)
}
