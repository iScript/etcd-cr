package etcdserver

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/iScript/etcd-cr/etcdserver/api/membership"
	"github.com/iScript/etcd-cr/pkg/logutil"
	"github.com/iScript/etcd-cr/pkg/types"
	"github.com/iScript/etcd-cr/raft"
	"github.com/iScript/etcd-cr/wal"
	"go.uber.org/zap"
)

const (

	//etcd的最大吞吐量， 不超过100MB/s , 假设RTT 10毫秒，1秒中有100次，每次最多1M
	maxSizePerMsg = 1 * 1024 * 1024
	// 不要超过 rafthttp buffer, 即4096.
	maxInflightMsgs = 4096 / 8
)

var (
	// protects raftStatus
	raftStatusMu sync.Mutex
	// indir

	//ection for expvar func interface
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

//func startNode(cfg ServerConfig, cl *membership.RaftCluster, ids []types.ID) (id types.ID, n raft.Node, s *raft.MemoryStorage, w *wal.WAL) {
func startNode(cfg ServerConfig, cl *membership.RaftCluster, ids []types.ID) (id types.ID, n raft.Node, s *raft.MemoryStorage, w *wal.WAL) {
	fmt.Println("start Node")
	var err error
	member := cl.MemberByName(cfg.Name)

	metadata := []byte("test") //暂时用test测试
	// 创建wal
	if w, err = wal.Create(cfg.Logger, cfg.WALDir(), metadata); err != nil {
		if cfg.Logger != nil {
			cfg.Logger.Panic("failed to create WAL", zap.Error(err))
		}
	}

	peers := make([]raft.Peer, len(ids))
	for i, id := range ids {
		var ctx []byte
		ctx, err = json.Marshal((*cl).Member(id)) // 转为json  fmt.Println(string(ctx))   {"id":10276657743932975437,"peerURLs":["http://localhost:2380"],"name":"test"}

		if err != nil {
			if cfg.Logger != nil {
				cfg.Logger.Panic("failed to marshal member", zap.Error(err))
			}
		}
		peers[i] = raft.Peer{ID: uint64(id), Context: ctx}
	}

	id = member.ID //这里id赋值，没有定义id，赋值给返回值

	if cfg.Logger != nil {
		cfg.Logger.Info(
			"starting local member",
			zap.String("local-member-id", id.String()),
			zap.String("cluster-id", cl.ID().String()),
		)
	}

	s = raft.NewMemoryStorage()

	c := &raft.Config{
		ID:              uint64(id),
		ElectionTick:    cfg.ElectionTicks,
		HeartbeatTick:   1,
		Storage:         s,
		MaxSizePerMsg:   maxSizePerMsg,
		MaxInflightMsgs: maxInflightMsgs,
		CheckQuorum:     true,
		PreVote:         cfg.PreVote,
	}

	//设置raft.Config的日志
	if cfg.Logger != nil {

		if cfg.LoggerConfig != nil {
			c.Logger, err = logutil.NewRaftLogger(cfg.LoggerConfig)
			if err != nil {
				log.Fatalf("cannot create raft logger %v", err)
			}
		} else if cfg.LoggerCore != nil && cfg.LoggerWriteSyncer != nil {
			c.Logger = logutil.NewRaftLoggerFromZapCore(cfg.LoggerCore, cfg.LoggerWriteSyncer)
		}
	}

	// peers默认一个本机
	if len(peers) == 0 {
		//raft.RestartNode(c)
	} else {
		n = raft.StartNode(c, peers)
	}

	return id, n, s, w
	//return nil, nil, nil, nil
}
