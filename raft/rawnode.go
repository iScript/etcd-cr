package raft

import (
	pb "github.com/iScript/etcd-cr/raft/raftpb"
)

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
	rn.prevSoftSt = r.softState() // 将当前状态作为上一状态
	rn.prevHardSt = r.hardState() // 当前硬状态 ，如果是刚start则为 pb.HardState{Term:0,Vote:0,Commit: 0}

	return rn, nil
}

// 推进逻辑时钟，node.go中调用
func (rn *RawNode) Tick() {
	rn.raft.tick()
}

// Ready returns the outstanding work that the application needs to handle. This
// includes appending and applying entries or a snapshot, updating the HardState,
// and sending messages. The returned Ready() *must* be handled and subsequently
// passed back via Advance().
func (rn *RawNode) Ready() Ready {
	rd := rn.readyWithoutAccept()
	rn.acceptReady(rd)
	return rd
}

// readyWithoutAccept returns a Ready. This is a read-only operation, i.e. there
// is no obligation that the Ready must be handled.
func (rn *RawNode) readyWithoutAccept() Ready {
	return newReady(rn.raft, rn.prevSoftSt, rn.prevHardSt)
}

// acceptReady is called when the consumer of the RawNode has decided to go
// ahead and handle a Ready. Nothing must alter the state of the RawNode between
// this call and the prior call to Ready().
func (rn *RawNode) acceptReady(rd Ready) {
	if rd.SoftState != nil {
		rn.prevSoftSt = rd.SoftState
	}
	if len(rd.ReadStates) != 0 {
		rn.raft.readStates = nil
	}
	rn.raft.msgs = nil
}

//
func (rn *RawNode) HasReady() bool {
	r := rn.raft
	if !r.softState().equal(rn.prevSoftSt) {
		return true
	}
	if hardSt := r.hardState(); !IsEmptyHardState(hardSt) && !isHardStateEqual(hardSt, rn.prevHardSt) {
		return true
	}
	if r.raftLog.unstable.snapshot != nil && !IsEmptySnap(*r.raftLog.unstable.snapshot) {
		return true
	}
	if len(r.msgs) > 0 || len(r.raftLog.unstableEntries()) > 0 || r.raftLog.hasNextEnts() {
		return true
	}
	if len(r.readStates) != 0 {
		return true
	}
	return false
}
