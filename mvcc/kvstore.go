package mvcc

import "errors"

var (
	keyBucketName  = []byte("key")
	metaBucketName = []byte("meta")

	consistentIndexKeyName  = []byte("consistent_index")
	scheduledCompactKeyName = []byte("scheduledCompactRev")
	finishedCompactKeyName  = []byte("finishedCompactRev")

	ErrCompacted = errors.New("mvcc: required revision has been compacted")
	ErrFutureRev = errors.New("mvcc: required revision is a future revision")
	ErrCanceled  = errors.New("mvcc: watcher is canceled")
	ErrClosed    = errors.New("mvcc: closed")

	//plog = capnslog.NewPackageLogger("go.etcd.io/etcd", "mvcc")
)

const (
	// markedRevBytesLen is the byte length of marked revision.
	// The first `revBytesLen` bytes represents a normal revision. The last
	// one byte is the mark.
	//markedRevBytesLen      = revBytesLen + 1
	//markBytePosition      = markedRevBytesLen - 1
	markTombstone byte = 't'
)

var restoreChunkKeys = 10000 // non-const for testing
var defaultCompactBatchLimit = 1000

// ConsistentIndexGetter is an interface that wraps the Get method.
// Consistent index is the offset of an entry in a consistent replicated log.
type ConsistentIndexGetter interface {
	// ConsistentIndex returns the consistent index of current executing entry.
	//ConsistentIndex() uint64
}

type StoreConfig struct {
	CompactionBatchLimit int
}

type store struct {
	// ReadView
	// WriteView

	// consistentIndex caches the "consistent_index" key's value. Accessed
	// through atomics so must be 64-bit aligned.
	consistentIndex uint64

	cfg StoreConfig

	// mu read locks for txns and write locks for non-txn store changes.
	// mu sync.RWMutex

	// ig ConsistentIndexGetter

	// b       backend.Backend
	// kvindex index

	// le lease.Lessor

	// // revMuLock protects currentRev and compactMainRev.
	// // Locked at end of write txn and released after write txn unlock lock.
	// // Locked before locking read txn and released after locking.
	// revMu sync.RWMutex
	// // currentRev is the revision of the last completed transaction.
	// currentRev int64
	// // compactMainRev is the main revision of the last compaction.
	// compactMainRev int64

	// // bytesBuf8 is a byte slice of length 8
	// // to avoid a repetitive allocation in saveIndex.
	// bytesBuf8 []byte

	// fifoSched schedule.Scheduler

	// stopc chan struct{}

	// lg *zap.Logger
}
