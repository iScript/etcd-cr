package raft

import pb "github.com/iScript/etcd-cr/raft/raftpb"

// 使用内存数组维护entry记录
// 对于Leader而言它维护了客户端赌赢的entry记录，对于follower而言，他维护的是从Leader节点复制来的entry记录
// 无论是Leader还是Follower，对于刚刚接收到的entry记录首先会被存储在unstable中
// 然后交给上层模块处理
type unstable struct {
	snapshot *pb.Snapshot // 快照数据，未写入storage中的
	entries  []pb.Entry   //保存未写入Storage中的Entry记录
	offset   uint64       // entries中的第一条entry记录的索引

	logger Logger
}

// 返回第一条entry记录的索引值
func (u *unstable) maybeFirstIndex() (uint64, bool) {
	// 如果记录了快照，则直接通过快照元数据返回索引值
	if u.snapshot != nil {
		return u.snapshot.Metadata.Index + 1, true
	}
	return 0, false
}

// 返回最后一条索引值
func (u *unstable) maybeLastIndex() (uint64, bool) {
	if l := len(u.entries); l != 0 {
		return u.offset + uint64(l) - 1, true
	}
	if u.snapshot != nil {
		return u.snapshot.Metadata.Index, true
	}
	return 0, false
}

// 获取指定entry的term值
func (u *unstable) maybeTerm(i uint64) (uint64, bool) {
	// 索引对应的entry已经不再unstable中，可能已经被持久化等
	if i < u.offset {
		if u.snapshot == nil {
			return 0, false
		}
		if u.snapshot.Metadata.Index == i {
			return u.snapshot.Metadata.Term, true
		}
		return 0, false
	}

	last, ok := u.maybeLastIndex()
	if !ok {
		return 0, false
	}
	if i > last {
		return 0, false
	}
	return u.entries[i-u.offset].Term, true
}

func (u *unstable) stableTo(i, t uint64) {
	gt, ok := u.maybeTerm(i)
	if !ok {
		return
	}
	// if i < offset, term is matched with the snapshot
	// only update the unstable entries if term is matched with
	// an unstable entry.
	if gt == t && i >= u.offset {
		u.entries = u.entries[i+1-u.offset:]
		u.offset = i + 1
		u.shrinkEntriesArray() // 对数组压缩
	}
}

// shrinkEntriesArray discards the underlying array used by the entries slice
// if most of it isn't being used. This avoids holding references to a bunch of
// potentially large entries that aren't needed anymore. Simply clearing the
// entries wouldn't be safe because clients might still be using them.
func (u *unstable) shrinkEntriesArray() {
	// We replace the array if we're using less than half of the space in
	// it. This number is fairly arbitrary, chosen as an attempt to balance
	// memory usage vs number of allocations. It could probably be improved
	// with some focused tuning.
	const lenMultiple = 2
	if len(u.entries) == 0 {
		u.entries = nil
	} else if len(u.entries)*lenMultiple < cap(u.entries) {
		newEntries := make([]pb.Entry, len(u.entries))
		copy(newEntries, u.entries)
		u.entries = newEntries
	}
}

// ....
