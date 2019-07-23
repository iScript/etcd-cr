package etcdserver

import (
	"sync/atomic"
)

// 一致性索引.
// 他实现 the mvcc.ConsistentIndexGetter 接口.
type consistentIndex uint64

func (i *consistentIndex) setConsistentIndex(v uint64) {
	atomic.StoreUint64((*uint64)(i), v) //原子操作
}

func (i *consistentIndex) ConsistentIndex() uint64 {
	return atomic.LoadUint64((*uint64)(i))
}
