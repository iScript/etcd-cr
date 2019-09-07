package mvcc

import "github.com/iScript/etcd-cr/mvcc/backend"

type RangeOptions struct {
	Limit int64
	Rev   int64
	Count bool
}

type RangeResult struct {
	//KVs   []mvccpb.KeyValue
	Rev   int64
	Count int
}

// 只读事务视图
type ReadView interface {
	// 与rev方法相同，但是当进行一次压缩后，返回压缩时的revision
	FirstRev() int64

	// 返回开启当前只读事务时的revision信息
	Rev() int64

	// 范围查询
	Range(key, end []byte, ro RangeOptions) (r *RangeResult, err error)
}

// 一个只读事务，在readview基础上扩展了一个end方法
type TxnRead interface {
	ReadView
	// 表示当前事务已经完成，并准备提交
	End()
}

type WriteView interface {
	// 范围删除
	DeleteRange(key, end []byte) (n, rev int64)

	// 添加指定的键值对
	//Put(key, value []byte, lease lease.LeaseID) (rev int64)
}

// 读写事务
type TxnWrite interface {
	TxnRead
	WriteView
	// Changes gets the changes made since opening the write txn.
	//Changes() []mvccpb.KeyValue
}

// txnReadWrite coerces a read txn to a write, panicking on any write operation.
type txnReadWrite struct{ TxnRead }

// func (trw *txnReadWrite) DeleteRange(key, end []byte) (n, rev int64) { panic("unexpected DeleteRange") }
// func (trw *txnReadWrite) Put(key, value []byte, lease lease.LeaseID) (rev int64) {
// 	panic("unexpected Put")
// }
// func (trw *txnReadWrite) Changes() []mvccpb.KeyValue { return nil }

// func NewReadOnlyTxnWrite(txn TxnRead) TxnWrite { return &txnReadWrite{txn} }

type KV interface {
	ReadView
	WriteView

	// 创建只读事务
	Read() TxnRead

	//创建写事务
	Write() TxnWrite

	//
	Hash() (hash uint32, revision int64, err error)

	//
	HashByRev(rev int64) (hash uint32, revision int64, compactRev int64, err error)

	// 压缩
	Compact(rev int64) (<-chan struct{}, error)

	// 提交事务
	Commit()

	// 从bolt中恢复内存索引
	Restore(b backend.Backend) error
	Close() error
}

// 能被watch的kv
type WatchableKV interface {
	KV
	Watchable
}

// Watchable 接口
type Watchable interface {
	// NewWatchStream returns a WatchStream that can be used to
	// watch events happened or happening on the KV.
	// NewWatchStream() WatchStream
}

type ConsistentWatchableKV interface {
	// WatchableKV
	// 返回kv中当前一致的index
	//ConsistentIndex() uint64
}
