package raft

import pb "github.com/iScript/etcd-cr/raft/raftpb"

// 限定entry总的字节数
func limitSize(ents []pb.Entry, maxSize uint64) []pb.Entry {
	if len(ents) == 0 {
		return ents
	}
	size := ents[0].Size()
	var limit int
	for limit = 1; limit < len(ents); limit++ {
		size += ents[limit].Size()
		if uint64(size) > maxSize {
			break
		}
	}
	return ents[:limit] // 一直到limit-1的索引元素
}
