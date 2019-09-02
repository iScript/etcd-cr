package rafthttp

import (
	"sync"

	stats "github.com/iScript/etcd-cr/etcdserver/api/v2stats"
	"github.com/iScript/etcd-cr/pkg/types"
	"github.com/iScript/etcd-cr/raft/raftpb"
)

type pipeline struct {
	peerID types.ID

	tr *Transport
	//picker *urlPicker
	status *peerStatus
	raft   Raft
	errorc chan error

	// deprecate when we depercate v2 API
	followerStats *stats.FollowerStats

	msgc chan raftpb.Message
	// wait for the handling routines
	wg    sync.WaitGroup
	stopc chan struct{}
}
