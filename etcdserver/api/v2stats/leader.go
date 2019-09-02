package v2stats

import (
	"encoding/json"
	"fmt"
	"sync"
)

// 集群中leader的相关状态，封装了和follower通信的相关统计数据
type LeaderStats struct {
	leaderStats
	sync.Mutex
}

type leaderStats struct {
	// Leader字段是leader id
	// 各个Follower的统计数据
	Leader    string                    `json:"leader"`
	Followers map[string]*FollowerStats `json:"followers"`
}

// 通过lead id初始化对象
func NewLeaderStats(id string) *LeaderStats {
	return &LeaderStats{
		leaderStats: leaderStats{
			Leader:    id,
			Followers: make(map[string]*FollowerStats),
		},
	}
}

// 返回json数据
func (ls *LeaderStats) JSON() []byte {
	ls.Lock()
	stats := ls.leaderStats
	ls.Unlock()
	b, err := json.Marshal(stats)
	// TODO(jonboulle): appropriate error handling?
	if err != nil {
		//plog.Errorf("error marshalling leader stats (%v)", err)
		fmt.Println("error marshalling leader stats (%v)", err)

	}
	return b
}

// 封装了集群中Follower的数据统计
type FollowerStats struct {
	Latency LatencyStats `json:"latency"`
	Counts  CountsStats  `json:"counts"`

	sync.Mutex
}

// 延迟数据数据
type LatencyStats struct {
	Current           float64 `json:"current"`
	Average           float64 `json:"average"`
	averageSquare     float64
	StandardDeviation float64 `json:"standardDeviation"`
	Minimum           float64 `json:"minimum"`
	Maximum           float64 `json:"maximum"`
}

//
type CountsStats struct {
	Fail    uint64 `json:"fail"`
	Success uint64 `json:"success"`
}
