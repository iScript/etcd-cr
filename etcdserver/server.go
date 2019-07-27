package etcdserver

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/coreos/go-semver/semver"
	humanize "github.com/dustin/go-humanize"
	"github.com/iScript/etcd-cr/etcdserver/api/membership"
	"github.com/iScript/etcd-cr/etcdserver/api/rafthttp"
	"github.com/iScript/etcd-cr/etcdserver/api/v2store"
	"github.com/iScript/etcd-cr/pkg/fileutil"
	"github.com/iScript/etcd-cr/pkg/types"
	"github.com/iScript/etcd-cr/wal"
	"go.uber.org/zap"
)

const (
	DefaultSnapshotCount = 100000

	// DefaultSnapshotCatchUpEntries is the number of entries for a slow follower
	// to catch-up after compacting the raft storage entries.
	// We expect the follower has a millisecond level latency with the leader.
	// The max throughput is around 10K. Keep a 5K entries is enough for helping
	// follower to catch up.
	DefaultSnapshotCatchUpEntries uint64 = 5000

	StoreClusterPrefix = "/0"
	StoreKeysPrefix    = "/1"

	// HealthInterval is the minimum time the cluster should be healthy
	// before accepting add member requests.
	HealthInterval = 5 * time.Second

	purgeFileInterval = 30 * time.Second
	// monitorVersionInterval should be smaller than the timeout
	// on the connection. Or we will not be able to reuse the connection
	// (since it will timeout).
	//monitorVersionInterval = rafthttp.ConnWriteTimeout - time.Second

	// max number of in-flight snapshot messages etcdserver allows to have
	// This number is more than enough for most clusters with 5 machines.
	maxInFlightMsgSnap = 16

	releaseDelayAfterSnapshot = 30 * time.Second

	// maxPendingRevokes is the maximum number of outstanding expired lease revocations.
	maxPendingRevokes = 16

	recommendedMaxRequestBytes = 10 * 1024 * 1024

	readyPercent = 0.9
)

func init() {
	rand.Seed(time.Now().UnixNano())

	// expvar.Publish(
	// 	"file_descriptor_limit",
	// 	expvar.Func(
	// 		func() interface{} {
	// 			n, _ := runtime.FDLimit()
	// 			return n
	// 		},
	// 	),
	// )
}

type Server interface {
	// AddMember attempts to add a member into the cluster. It will return
	// ErrIDRemoved if member ID is removed from the cluster, or return
	// ErrIDExists if member ID exists in the cluster.
	//AddMember(ctx context.Context, memb membership.Member) ([]*membership.Member, error)
	// RemoveMember attempts to remove a member from the cluster. It will
	// return ErrIDRemoved if member ID is removed from the cluster, or return
	// ErrIDNotFound if member ID is not in the cluster.
	//RemoveMember(ctx context.Context, id uint64) ([]*membership.Member, error)
	// UpdateMember attempts to update an existing member in the cluster. It will
	// return ErrIDNotFound if the member ID does not exist.
	//UpdateMember(ctx context.Context, updateMemb membership.Member) ([]*membership.Member, error)
	// PromoteMember attempts to promote a non-voting node to a voting node. It will
	// return ErrIDNotFound if the member ID does not exist.
	// return ErrLearnerNotReady if the member are not ready.
	// return ErrMemberNotLearner if the member is not a learner.
	//PromoteMember(ctx context.Context, id uint64) ([]*membership.Member, error)

	// ClusterVersion is the cluster-wide minimum major.minor version.
	// Cluster version is set to the min version that an etcd member is
	// compatible with when first bootstrap.
	//
	// ClusterVersion is nil until the cluster is bootstrapped (has a quorum).
	//
	// During a rolling upgrades, the ClusterVersion will be updated
	// automatically after a sync. (5 second by default)
	//
	// The API/raft component can utilize ClusterVersion to determine if
	// it can accept a client request or a raft RPC.
	// NOTE: ClusterVersion might be nil when etcd 2.1 works with etcd 2.0 and
	// the leader is etcd 2.0. etcd 2.0 leader will not update clusterVersion since
	// this feature is introduced post 2.0.
	ClusterVersion() *semver.Version
	// Cluster() api.Cluster
	// Alarms() []*pb.AlarmMember
}

type EtcdServer struct {
	// inflightSnapshots holds count the number of snapshots currently inflight.
	inflightSnapshots int64  // must use atomic operations to access; keep 64-bit aligned.
	appliedIndex      uint64 // must use atomic operations to access; keep 64-bit aligned.
	committedIndex    uint64 // must use atomic operations to access; keep 64-bit aligned.
	term              uint64 // must use atomic operations to access; keep 64-bit aligned.
	lead              uint64 // must use atomic operations to access; keep 64-bit aligned.

	// consistIndex used to hold the offset of current executing entry
	// It is initialized to 0 before executing any entry.
	consistIndex consistentIndex // must use atomic operations to access; keep 64-bit aligned.
	//r            raftNode        // uses 64-bit atomics; keep 64-bit aligned.

	readych chan struct{}
	Cfg     ServerConfig

	lgMu *sync.RWMutex
	lg   *zap.Logger
	//w    wait.Wait

	readMu sync.RWMutex
	// read routine notifies etcd server that it waits for reading by sending an empty struct to
	// readwaitC
	readwaitc chan struct{}
	// readNotifier is used to notify the read routine that it can process the request
	// when there is no error
	//readNotifier *notifier

	// stop signals the run goroutine should shutdown.
	stop chan struct{}
	// stopping is closed by run goroutine on shutdown.
	stopping chan struct{}
	// done is closed when all goroutines from start() complete.
	done chan struct{}
	// leaderChanged is used to notify the linearizable read loop to drop the old read requests.
	leaderChanged   chan struct{}
	leaderChangedMu sync.RWMutex

	errorc chan error
	id     types.ID
	//attributes membership.Attributes

	cluster *membership.RaftCluster

	v2store v2store.Store
	// snapshotter *snap.Snapshotter

	// applyV2 ApplierV2

	// // applyV3 is the applier with auth and quotas
	// applyV3 applierV3
	// // applyV3Base is the core applier without auth or quotas
	// applyV3Base applierV3
	// applyWait   wait.WaitTime

	// kv         mvcc.ConsistentWatchableKV
	// lessor     lease.Lessor
	// bemu       sync.Mutex
	// be         backend.Backend
	// authStore  auth.AuthStore
	// alarmStore *v3alarm.AlarmStore

	// stats  *stats.ServerStats
	// lstats *stats.LeaderStats

	SyncTicker *time.Ticker
	// compactor is used to auto-compact the KV.
	//compactor v3compactor.Compactor

	// peerRt used to send requests (version, lease) to peers.
	// peerRt   http.RoundTripper
	// reqIDGen *idutil.Generator

	// forceVersionC is used to force the version monitor loop
	// to detect the cluster version immediately.
	forceVersionC chan struct{}

	// wgMu blocks concurrent waitgroup mutation while server stopping
	wgMu sync.RWMutex
	// wg is used to wait for the go routines that depends on the server state
	// to exit when stopping the server.
	wg sync.WaitGroup

	// ctx is used for etcd-initiated requests that may need to be canceled
	// on etcd server shutdown.
	ctx    context.Context
	cancel context.CancelFunc

	leadTimeMu      sync.RWMutex
	leadElectedTime time.Time

	//*AccessController
}

func NewServer(cfg ServerConfig) (srv *EtcdServer, err error) {

	var (
		// w  *wal.WAL
		// n  raft.Node
		// s  *raft.MemoryStorage
		// id types.ID
		cl *membership.RaftCluster
	)

	//默认cfg.MaxRequestBytes = 1.5 * 1024 * 1024  recommendedMaxRequestBytes = 10 * 1024 * 1024，没有大于
	if cfg.MaxRequestBytes > recommendedMaxRequestBytes {
		//
		if cfg.Logger != nil {
			cfg.Logger.Warn(
				"exceeded recommended request limit",
				zap.Uint("max-request-bytes", cfg.MaxRequestBytes),
				zap.String("max-request-size", humanize.Bytes(uint64(cfg.MaxRequestBytes))),
				zap.Int("recommended-request-bytes", recommendedMaxRequestBytes),
				zap.String("recommended-request-size", humanize.Bytes(uint64(recommendedMaxRequestBytes))),
				//humanize 人方面读的size大小，如返回mb kb
			)
		}
	}

	if terr := fileutil.TouchDirAll(cfg.DataDir); terr != nil {
		return nil, fmt.Errorf("cannot access data directory: %v", terr)
	}

	//member/wal中是否有.wal文件
	haveWAL := wal.Exist(cfg.WALDir())

	if err = fileutil.TouchDirAll(cfg.SnapDir()); err != nil {
		if cfg.Logger != nil {
			cfg.Logger.Fatal(
				"failed to create snapshot directory ",
				zap.String("path", cfg.SnapDir()),
				zap.Error(err),
			)
		}
	}

	//ss := snap.New(cfg.Logger, cfg.SnapDir())

	//bepath := cfg.backendPath()
	//beExist := fileutil.Exist(bepath)
	be := openBackend(cfg)

	defer func() {
		if err != nil {
			be.Close()
		}
	}()

	prt, err := rafthttp.NewRoundTripper(cfg.PeerTLSInfo, cfg.peerDialTimeout())
	if err != nil {
		return nil, err
	}

	// var (
	// 	remotes  []*membership.Member
	// 	snapshot *raftpb.Snapshot
	// )

	//fmt.Println(haveWAL, cfg.NewCluster) // 默认false true
	switch {
	case !haveWAL && !cfg.NewCluster:
		fmt.Println(111)
	case !haveWAL && cfg.NewCluster:
		// 一些验证
		if err = cfg.VerifyBootstrap(); err != nil {
			return nil, err
		}
		//根据参数初始化cluster
		cl, err = membership.NewClusterFromURLsMap(cfg.Logger, cfg.InitialClusterToken, cfg.InitialPeerURLsMap)

		if err != nil {
			return nil, err
		}
		m := cl.MemberByName(cfg.Name)
		//指定的member是否在集群中已启动
		if isMemberBootstrapped(cfg.Logger, cl, cfg.Name, prt, cfg.bootstrapTimeout()) {
			return nil, fmt.Errorf("member %s has already been bootstrapped", m.ID)
		}

		if cfg.ShouldDiscover() {
			// 默认false ， 后续再看
		}

	case haveWAL:
		fmt.Println(333)
	default:
		return nil, fmt.Errorf("unsupported bootstrap config")
	}

	if terr := fileutil.TouchDirAll(cfg.MemberDir()); terr != nil {
		return nil, fmt.Errorf("cannot access member directory: %v", terr)
	}

	srv = &EtcdServer{
		readych: make(chan struct{}),
		Cfg:     cfg, //serverConfig
		lgMu:    new(sync.RWMutex),
		lg:      cfg.Logger,
		errorc:  make(chan error, 1),
		//v2store: st,
		//snapshotter: ss,  etcdserver/api/snap
		// r: *newRaftNode(
		// 	raftNodeConfig{
		// 		lg:          cfg.Logger,
		// 		isIDRemoved: func(id uint64) bool { return cl.IsIDRemoved(types.ID(id)) },
		// 		Node:        n,
		// 		heartbeat:   heartbeat,
		// 		raftStorage: s,
		// 		storage:     NewStorage(w, ss),
		// 	},
		// ),
		cluster: cl,
	}

	return nil, nil
}
