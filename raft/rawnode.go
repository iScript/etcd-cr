package raft

import pb "github.com/iScript/etcd-cr/raft/raftpb"

type RawNode struct {
	raft       *raft        //raft对象
	prevSoftSt *SoftState   //上一软状态
	prevHardSt pb.HardState //上一硬状态
}

// 初始化rawnode
func NewRawNode(config *Config) (*RawNode, error) {
	if config.ID == 0 { // 这里将memberid作为config id
		panic("config.ID must not be zero")
	}
	r := newRaft(config)
	rn := &RawNode{
		raft: r,
	}
	rn.prevSoftSt = r.softState() //将当前状态作为上一状态
	rn.prevHardSt = r.hardState()
	return rn, nil
}
