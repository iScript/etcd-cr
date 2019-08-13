package raft

import (
	pb "github.com/iScript/etcd-cr/raft/raftpb"
	"github.com/iScript/etcd-cr/raft/tracker"
)

// raft peer的信息和系统
type Status struct {
	BasicStatus
	Config   tracker.Config
	Progress map[uint64]tracker.Progress // 该字段就leader用到，记录了集群中每个节点对应的progress实例
}

// raft peer的基本信息
type BasicStatus struct {
	ID uint64

	pb.HardState
	SoftState

	Applied uint64

	LeadTransferee uint64
}
