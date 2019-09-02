package v2stats

import (
	"sync"
	"time"

	"github.com/iScript/etcd-cr/raft"
)

// ServerStats 封装了etcdserver的各种统计信息
type ServerStats struct {
	serverStats
	sync.Mutex
}

func NewServerStats(name, id string) *ServerStats {
	ss := &ServerStats{
		serverStats: serverStats{
			Name: name,
			ID:   id,
		},
	}
	now := time.Now()
	ss.StartTime = now
	ss.LeaderInfo.StartTime = now
	ss.sendRateQueue = &statsQueue{back: -1}
	ss.recvRateQueue = &statsQueue{back: -1}
	return ss
}

type serverStats struct {
	Name string `json:"name"`
	// ID is the raft ID of the node.
	// TODO(jonboulle): use ID instead of name?
	ID        string         `json:"id"`
	State     raft.StateType `json:"state"`
	StartTime time.Time      `json:"startTime"`

	LeaderInfo struct {
		Name      string    `json:"leader"`
		Uptime    string    `json:"uptime"`
		StartTime time.Time `json:"startTime"`
	} `json:"leaderInfo"`

	RecvAppendRequestCnt uint64  `json:"recvAppendRequestCnt,"`
	RecvingPkgRate       float64 `json:"recvPkgRate,omitempty"`
	RecvingBandwidthRate float64 `json:"recvBandwidthRate,omitempty"`

	SendAppendRequestCnt uint64  `json:"sendAppendRequestCnt"`
	SendingPkgRate       float64 `json:"sendPkgRate,omitempty"`
	SendingBandwidthRate float64 `json:"sendBandwidthRate,omitempty"`

	sendRateQueue *statsQueue
	recvRateQueue *statsQueue
}
