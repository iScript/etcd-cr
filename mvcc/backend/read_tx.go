package backend

import (
	"sync"

	bolt "go.etcd.io/bbolt"
)

// 名称为key的bucket
// 该bucket中的key就是前面介绍的revision 版本号，value为键值对
var safeRangeBucket = []byte("key")

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
	buf txReadBuffer // 缓存bucket与其中键值对集合的映射关系

	// .
	txMu    sync.RWMutex // 在查询之前，需要获取该锁进行同步
	tx      *bolt.Tx     // 只读事务
	buckets map[string]*bolt.Bucket

	txWg *sync.WaitGroup
}

func (rt *readTx) Lock()    { rt.mu.Lock() }    // 写锁定
func (rt *readTx) Unlock()  { rt.mu.Unlock() }  // 写解锁
func (rt *readTx) RLock()   { rt.mu.RLock() }   // 读锁定
func (rt *readTx) RUnlock() { rt.mu.RUnlock() } // 读解锁

// func (rt *readTx) UnsafeRange(bucketName, key, endKey []byte, limit int64) ([][]byte, [][]byte) {
// 	//对非法的limit进行重新设置
// 	if endKey == nil {
// 		limit = 1
// 	}
// 	if limit <= 0 {
// 		limit = math.MaxInt64
// 	}

// 	// 只有查询safeRangeBucket时，才是真正的范围查询，否则只能返回一个键值对
// 	if limit > 1 && !bytes.Equal(bucketName, safeRangeBucket) {
// 		panic("do not use unsafeRange on non-keys bucket")
// 	}

// 首先从缓存中查询
// keys, vals := rt.buf.Range(bucketName, key, endKey, limit)
// if int64(len(keys)) == limit {
// 	return keys, vals
// }

// find/cache bucket
// 	bn := string(bucketName)
// 	rt.txMu.RLock()
// 	bucket, ok := rt.buckets[bn]
// 	rt.txMu.RUnlock()

// 	if !ok {
// 		rt.txMu.Lock()
// 		bucket = rt.tx.Bucket(bucketName)
// 		rt.buckets[bn] = bucket
// 		rt.txMu.Unlock()
// 	}

// 	// ignore missing bucket since may have been created in this batch
// 	if bucket == nil {
// 		return keys, vals
// 	}
// 	rt.txMu.Lock()
// 	c := bucket.Cursor()
// 	rt.txMu.Unlock()

// 	k2, v2 := unsafeRange(c, key, endKey, limit-int64(len(keys)))
// 	return append(k2, keys...), append(v2, vals...)
// }

func (rt *readTx) reset() {
	rt.buf.reset()
	rt.buckets = make(map[string]*bolt.Bucket)
	rt.tx = nil
	rt.txWg = new(sync.WaitGroup)
}
