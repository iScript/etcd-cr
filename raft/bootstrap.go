package raft

import (
	"errors"
	"fmt"

	pb "github.com/iScript/etcd-cr/raft/raftpb"
)

// 启动，只在startNode中调用
func (rn *RawNode) Bootstrap(peers []Peer) error {
	if len(peers) == 0 {
		return errors.New("must provide at least one peer to Bootstrap")
	}

	lastIndex, err := rn.raft.raftLog.storage.LastIndex()
	if err != nil {
		return err
	}

	// 默认为0
	if lastIndex != 0 {
		return errors.New("can't bootstrap a nonempty Storage")
	}

	rn.prevHardSt = emptyState //空的hardstate赋值给prev

	rn.raft.becomeFollower(1, None) // 切换成follower，因为节点初次启动，任期号为1

	ents := make([]pb.Entry, len(peers))
	fmt.Println(len(ents))
	for i, peer := range peers {
		cc := pb.ConfChange{Type: pb.ConfChangeAddNode, NodeID: peer.ID, Context: peer.Context}
		data, err := cc.Marshal()
		if err != nil {
			return err
		}
		// 集群变更记录封装entry
		ents[i] = pb.Entry{Type: pb.EntryConfChange, Term: 1, Index: uint64(i + 1), Data: data}
	}

	// entry追加到raftlog
	rn.raft.raftLog.append(ents...)

	rn.raft.raftLog.committed = uint64(len(ents))
	//fmt.Println(rn.raft.raftLog.lastIndex(), rn.raft.raftLog.committed, 5656)

	// 应用变更
	for _, peer := range peers {
		rn.raft.applyConfChange(pb.ConfChange{NodeID: peer.ID, Type: pb.ConfChangeAddNode}.AsV2())
		
		// 转为v2版
		// ConfChangeV2{
		// 	Changes: []ConfChangeSingle{{
		// 		Type:   c.Type,
		// 		NodeID: c.NodeID,
		// 	}},
		// 	Context: c.Context,
		// }
	
	}

	return nil
}
