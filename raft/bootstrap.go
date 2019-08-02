package raft

import "errors"

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

	rn.raft.becomeFollower(1, None)

	return nil
}
