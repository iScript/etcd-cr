package etcdserver

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/iScript/etcd-cr/etcdserver/api/membership"
	"github.com/iScript/etcd-cr/etcdserver/api/rafthttp"
	"github.com/iScript/etcd-cr/pkg/contention"
	"github.com/iScript/etcd-cr/pkg/logutil"
	"github.com/iScript/etcd-cr/pkg/types"
	"github.com/iScript/etcd-cr/raft"
	"github.com/iScript/etcd-cr/raft/raftpb"
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

type apply struct {
	entries  []raftpb.Entry
	snapshot raftpb.Snapshot
	// notifyc synchronizes etcd server applies with the raft node
	notifyc chan struct{}
}

type raftNode struct {
	lg *zap.Logger

	tickMu *sync.Mutex
	raftNodeConfig

	// a chan to send/receive snapshot
	msgSnapC chan raftpb.Message

	// a chan to send out apply
	applyc chan apply

	// a chan to send out readState
	readStateC chan raft.ReadState

	// utility
	ticker *time.Ticker

	td *contention.TimeoutDetector

	stopped chan struct{}
	done    chan struct{}
}

type raftNodeConfig struct {
	lg *zap.Logger

	isIDRemoved func(id uint64) bool //检测接收者是否已经从集群中删除
	raft.Node                        // 接口，该字段需要实现接口，在raft/node.go 实现
	raftStorage *raft.MemoryStorage
	storage     Storage
	heartbeat   time.Duration // for logging
	transport   rafthttp.Transporter
}

func newRaftNode(cfg raftNodeConfig) *raftNode {
	var lg raft.Logger
	if cfg.lg != nil {
		lg = logutil.NewRaftLoggerZap(cfg.lg)
	} else {
		lcfg := logutil.DefaultZapLoggerConfig
		var err error
		lg, err = logutil.NewRaftLogger(&lcfg)
		if err != nil {
			log.Fatalf("cannot create raft logger %v", err)
		}
	}
	raft.SetLogger(lg)
	r := &raftNode{
		lg:             cfg.lg,
		tickMu:         new(sync.Mutex),
		raftNodeConfig: cfg,

		td:         contention.NewTimeoutDetector(2 * cfg.heartbeat),
		readStateC: make(chan raft.ReadState, 1),
		msgSnapC:   make(chan raftpb.Message, maxInFlightMsgSnap),
		applyc:     make(chan apply),
		stopped:    make(chan struct{}),
		done:       make(chan struct{}),
	}

	// heartbeat 100ms , 在server中的raftNodeConfig传入
	if r.heartbeat == 0 {
		r.ticker = &time.Ticker{}
	} else {
		r.ticker = time.NewTicker(r.heartbeat) // 创建100ms定时器,会在每100ms向自身的C字段通道发送当前时间
	}
	return r
}

// 推进逻辑时钟
func (r *raftNode) tick() {
	r.tickMu.Lock()
	r.Tick() //Node接口的tick
	r.tickMu.Unlock()
}

// 在一个新的goroutine中启动raftnode
// 在etcdserver run()中调用
func (r *raftNode) start(rh *raftReadyHandler) {
	//internalTimeout := time.Second //1s

	go func() {
		defer r.onStop()
		//islead := false

		for {
			select {
			case <-r.ticker.C: // 100ms的定时器，推进逻辑时钟
				r.tick()
			case rd := <-r.Ready(): //Node Ready接口，返回node.readyc
				fmt.Println("node ready:", rd)
				fmt.Println("node ready:", rd)
				//
			case <-r.stopped:
				return

			}
		}
	}()
}

func (r *raftNode) apply() chan apply {
	return r.applyc
}

func (r *raftNode) onStop() {
	// r.Stop()
	// r.ticker.Stop()
	// r.transport.Stop()
	// if err := r.storage.Close(); err != nil {
	// 	if r.lg != nil {
	// 		r.lg.Panic("failed to close Raft storage", zap.Error(err))
	// 	} else {
	// 		plog.Panicf("raft close storage error: %v", err)
	// 	}
	// }
	// close(r.done)
}

// advanceTicks advances ticks of Raft node.
// This can be used for fast-forwarding election
// ticks in multi data-center deployments, thus
// speeding up election process.
// 快速推进逻辑时钟。
func (r *raftNode) advanceTicks(ticks int) {
	for i := 0; i < ticks; i++ {
		r.tick()
	}
}

//
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
		fmt.Println("111")
		//raft.RestartNode(c)
	} else {
		n = raft.StartNode(c, peers)
	}

	return id, n, s, w
	//return nil, nil, nil, nil
}
