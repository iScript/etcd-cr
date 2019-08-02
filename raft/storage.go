package raft

import (
	"sync"

	pb "github.com/iScript/etcd-cr/raft/raftpb"
)

// 测试一下切片
// type Test struct {
// 	name string
// 	age  int
// }

type Storage interface {
	// InitialState returns the saved HardState and ConfState information.
	//InitialState() (pb.HardState, pb.ConfState, error)
	// Entries returns a slice of log entries in the range [lo,hi).
	// MaxSize limits the total size of the log entries returned, but
	// Entries returns at least one entry if any.
	//Entries(lo, hi, maxSize uint64) ([]pb.Entry, error)
	// Term returns the term of entry i, which must be in the range
	// [FirstIndex()-1, LastIndex()]. The term of the entry before
	// FirstIndex is retained for matching purposes even though the
	// rest of that entry may not be available.
	//Term(i uint64) (uint64, error)
	// LastIndex returns the index of the last entry in the log.
	LastIndex() (uint64, error)
	// FirstIndex returns the index of the first log entry that is
	// possibly available via Entries (older entries have been incorporated
	// into the latest Snapshot; if storage only contains the dummy entry the
	// first log entry is not available).
	FirstIndex() (uint64, error)
	// Snapshot returns the most recent snapshot.
	// If snapshot is temporarily unavailable, it should return ErrSnapshotTemporarilyUnavailable,
	// so raft state machine could know that Storage needs some time to prepare
	// snapshot and call Snapshot later.
	//Snapshot() (pb.Snapshot, error)
}

type MemoryStorage struct {
	// Protects access to all fields. Most methods of MemoryStorage are
	// run on the raft goroutine, but Append() is run on an application
	// goroutine.
	sync.Mutex

	// hardState pb.HardState
	// snapshot  pb.Snapshot
	// // ents[i] has raft log position i+snapshot.Metadata.Index
	ents []pb.Entry
}

// 创建一个空的MemoryStorage.
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		//
		ents: make([]pb.Entry, 1), //ents的值为一个切片，长度为1，值为pb.Entry 空对象, 对象里的值为默认0或空等
		//ents: make([]Test, 1),
	}
}

// 实现 Storage interface接口
func (ms *MemoryStorage) LastIndex() (uint64, error) {
	ms.Lock()
	defer ms.Unlock()
	return ms.lastIndex(), nil
}

// 返回最后一个index
func (ms *MemoryStorage) lastIndex() uint64 {
	// 默认为ents[0].Index=0 ， len(ms.ents)上面定义的是1， 返回0
	return ms.ents[0].Index + uint64(len(ms.ents)) - 1
	//return 0
}

// FirstIndex implements the Storage interface.
func (ms *MemoryStorage) FirstIndex() (uint64, error) {
	ms.Lock()
	defer ms.Unlock()
	return ms.firstIndex(), nil
}

func (ms *MemoryStorage) firstIndex() uint64 {
	return ms.ents[0].Index + 1
	//return 0
}
