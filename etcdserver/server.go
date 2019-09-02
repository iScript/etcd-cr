package etcdserver

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/coreos/go-semver/semver"
	humanize "github.com/dustin/go-humanize"
	"github.com/iScript/etcd-cr/etcdserver/api/membership"
	"github.com/iScript/etcd-cr/etcdserver/api/rafthttp"
	"github.com/iScript/etcd-cr/etcdserver/api/snap"
	stats "github.com/iScript/etcd-cr/etcdserver/api/v2stats"
	"github.com/iScript/etcd-cr/etcdserver/api/v2store"
	"github.com/iScript/etcd-cr/lease"
	"github.com/iScript/etcd-cr/mvcc/backend"
	"github.com/iScript/etcd-cr/pkg/fileutil"
	"github.com/iScript/etcd-cr/pkg/types"
	"github.com/iScript/etcd-cr/raft"
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

type Response struct {
	Term  uint64
	Index uint64
	// Event   *v2store.Event
	// Watcher v2store.Watcher
	Err error
}

type Server interface {
	//向当前etcd集群添加一个节点
	//AddMember(ctx context.Context, memb membership.Member) ([]*membership.Member, error)

	// 从当前集群删除一个节点
	//RemoveMember(ctx context.Context, id uint64) ([]*membership.Member, error)

	// 修改集群成员属性
	//UpdateMember(ctx context.Context, updateMemb membership.Member) ([]*membership.Member, error)

	// 无投票节点提升为有投票节点
	//PromoteMember(ctx context.Context, id uint64) ([]*membership.Member, error)

	//
	ClusterVersion() *semver.Version

	// Cluster() api.Cluster
	// Alarms() []*pb.AlarmMember
}

type EtcdServer struct {
	// 当前已发出去但未收到响应的快照个数
	inflightSnapshots int64  // must use atomic operations to access; keep 64-bit aligned.
	appliedIndex      uint64 // must use atomic operations to access; keep 64-bit aligned.
	committedIndex    uint64 // must use atomic operations to access; keep 64-bit aligned.
	term              uint64 // must use atomic operations to access; keep 64-bit aligned.
	lead              uint64 // must use atomic operations to access; keep 64-bit aligned.

	// consistIndex used to hold the offset of current executing entry
	// It is initialized to 0 before executing any entry.
	consistIndex consistentIndex // must use atomic operations to access; keep 64-bit aligned.
	r            raftNode        // raftnode，是etcdserver实例与底层etcd-raft模块通信的桥梁

	readych chan struct{}
	Cfg     ServerConfig

	lgMu *sync.RWMutex
	lg   *zap.Logger
	//w    wait.Wait	//负责协调后台多个goroutine之间的执行

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

	errorc     chan error
	id         types.ID
	attributes membership.Attributes //记录当前节点的名称及接收集群中其他节点请求的url地址

	cluster *membership.RaftCluster

	v2store v2store.Store
	// snapshotter *snap.Snapshotter	//读写快照文件

	applyV2 ApplierV2

	// // applyV3 is the applier with auth and quotas
	// applyV3 applierV3
	// // applyV3Base is the core applier without auth or quotas
	// applyV3Base applierV3
	// applyWait   wait.WaitTime	//WaitTime是上面介绍的Wait之上的一层扩展。记录的id是有序的

	// kv         mvcc.ConsistentWatchableKV	//v3版本的存储
	lessor lease.Lessor
	// bemu       sync.Mutex
	be backend.Backend // 后端存储
	// authStore  auth.AuthStore	// 在backend之上封装的一层存储，用于记录权限控制
	// alarmStore *v3alarm.AlarmStore	// 在backend之上封装的一层存储，用于记录报警相关

	stats  *stats.ServerStats
	lstats *stats.LeaderStats

	SyncTicker *time.Ticker // 控制Leader节点定期发送sync消息的频率

	//compactor v3compactor.Compactor	// Leader节点会对存储进行定期压缩，该字段用于控制压缩频率。

	// peerRt used to send requests (version, lease) to peers.
	// peerRt   http.RoundTripper
	// reqIDGen *idutil.Generator	// 用于生成请求的唯一标示

	// forceVersionC is used to force the version monitor loop
	// to detect the cluster version immediately.
	forceVersionC chan struct{}

	// wgMu blocks concurrent waitgroup mutation while server stopping
	wgMu sync.RWMutex

	wg sync.WaitGroup // 通过该字段等待后台所有goroutine全部退出

	// ctx is used for etcd-initiated requests that may need to be canceled
	// on etcd server shutdown.
	ctx    context.Context
	cancel context.CancelFunc

	leadTimeMu      sync.RWMutex
	leadElectedTime time.Time // 记录当前节点最近一次转换成Leader状态的时间戳

	//*AccessController
}

func NewServer(cfg ServerConfig) (srv *EtcdServer, err error) {

	var (
		w  *wal.WAL
		n  raft.Node
		s  *raft.MemoryStorage
		id types.ID
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

	// 快照目录
	if err = fileutil.TouchDirAll(cfg.SnapDir()); err != nil {
		if cfg.Logger != nil {
			cfg.Logger.Fatal(
				"failed to create snapshot directory ",
				zap.String("path", cfg.SnapDir()),
				zap.Error(err),
			)
		}
	}

	// 创建snapshotter实例，用来读写snap目录下的快照文件
	ss := snap.New(cfg.Logger, cfg.SnapDir())

	//bepath := cfg.backendPath()
	//beExist := fileutil.Exist(bepath)
	be := openBackend(cfg)

	defer func() {
		if err != nil {
			be.Close()
		}
	}()

	// 创建roundtripper实例
	prt, err := rafthttp.NewRoundTripper(cfg.PeerTLSInfo, cfg.peerDialTimeout())
	if err != nil {
		return nil, err
	}

	var (
		remotes []*membership.Member
	//	snapshot *raftpb.Snapshot
	)

	//fmt.Println(haveWAL, cfg.NewCluster) // 默认false true

	switch {
	case !haveWAL && !cfg.NewCluster: //没有wal文件且当前节点正在加入一个正在运行的节点
		fmt.Println(111)
	case !haveWAL && cfg.NewCluster: //没有wal文件且当前集群是新建的
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
		//cl.SetStore(st)
		cl.SetBackend(be)
		id, n, s, w = startNode(cfg, cl, cl.MemberIDs())

	case haveWAL: //存在wal文件
		fmt.Println(333)
	default:
		return nil, fmt.Errorf("unsupported bootstrap config")
	}

	if terr := fileutil.TouchDirAll(cfg.MemberDir()); terr != nil {
		return nil, fmt.Errorf("cannot access member directory: %v", terr)
	}

	sstats := stats.NewServerStats(cfg.Name, id.String()) //server的相关统计数据
	lstats := stats.NewLeaderStats(id.String())           //如果是leader，leader相关的数据

	heartbeat := time.Duration(cfg.TickMs) * time.Millisecond //100毫秒， TickMs在embed/config中定义，默认为100
	srv = &EtcdServer{
		readych: make(chan struct{}),
		Cfg:     cfg, //serverConfig
		lgMu:    new(sync.RWMutex),
		lg:      cfg.Logger,
		errorc:  make(chan error, 1),
		//v2store: st,
		//snapshotter: ss,  etcdserver/api/snap
		r: *newRaftNode(
			raftNodeConfig{
				lg:          cfg.Logger,
				isIDRemoved: func(id uint64) bool { return cl.IsIDRemoved(types.ID(id)) },
				Node:        n,
				heartbeat:   heartbeat,
				raftStorage: s,
				storage:     NewStorage(w, ss),
			},
		),
		cluster:    cl,
		stats:      sstats,
		lstats:     lstats,
		SyncTicker: time.NewTicker(500 * time.Millisecond), //同步ticker，500毫秒
	}

	//prometheus相关
	//serverID.With(prometheus.Labels{"server_id": id.String()}).Set(1)

	srv.applyV2 = &applierV2store{store: srv.v2store, cluster: srv.cluster}

	srv.be = be //设置backend

	minTTL := time.Duration((3*cfg.ElectionTicks)/2) * heartbeat // 1.5s

	// 因为在store.restore中除了恢复内存索引，还会重新绑定键值对与对应的lease
	// 先恢复lessor，再恢复kv
	srv.lessor = lease.NewLessor(
		srv.getLogger(),
		srv.be,
		lease.LessorConfig{
			MinLeaseTTL:                int64(math.Ceil(minTTL.Seconds())),
			CheckpointInterval:         cfg.LeaseCheckpointInterval,
			ExpiredLeasesRetryInterval: srv.Cfg.ReqTimeout(),
		})

	tr := &rafthttp.Transport{
		Logger:      cfg.Logger,
		TLSInfo:     cfg.PeerTLSInfo,
		DialTimeout: cfg.peerDialTimeout(),
		ID:          id,
		URLs:        cfg.PeerURLs,
		ClusterID:   cl.ID(),
		Raft:        srv, //srv需要实现Raft的几个接口
		//Snapshotter: ss,
		ServerStats: sstats,
		LeaderStats: lstats,
		ErrorC:      srv.errorc,
	}
	if err = tr.Start(); err != nil {
		return nil, err
	}

	for _, m := range remotes {
		if m.ID != id {
			tr.AddRemote(m.ID, m.PeerURLs)
		}
	}

	for _, m := range cl.Members() {
		if m.ID != id {
			tr.AddPeer(m.ID, m.PeerURLs)
		}
	}
	//srv.r.transport = tr

	return srv, nil

}

func (s *EtcdServer) getLogger() *zap.Logger {
	s.lgMu.RLock()
	l := s.lg
	s.lgMu.RUnlock()
	return l
}
