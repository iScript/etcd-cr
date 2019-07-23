package etcdserver

import (
	"sync"
)

const (
	// The max throughput of etcd will not exceed 100MB/s (100K * 1KB value).
	// Assuming the RTT is around 10ms, 1MB max size is large enough.
	maxSizePerMsg = 1 * 1024 * 1024
	// Never overflow the rafthttp buffer, which is 4096.
	// TODO: a better const?
	maxInflightMsgs = 4096 / 8
)

var (
	// protects raftStatus
	raftStatusMu sync.Mutex
	// indirection for expvar func interface
	// expvar panics when publishing duplicate name
	// expvar does not support remove a registered name
	// so only register a func that calls raftStatus
	// and change raftStatus as we need.
	//raftStatus func() raft.Status
)

// type raftNode struct {
// 	lg *zap.Logger

// 	tickMu *sync.Mutex
// 	raftNodeConfig

// 	// a chan to send/receive snapshot
// 	msgSnapC chan raftpb.Message

// 	// a chan to send out apply
// 	applyc chan apply

// 	// a chan to send out readState
// 	readStateC chan raft.ReadState

// 	// utility
// 	ticker *time.Ticker
// 	// contention detectors for raft heartbeat message
// 	td *contention.TimeoutDetector

// 	stopped chan struct{}
// 	done    chan struct{}
// }
