package etcdserver

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"path"
	"regexp"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coreos/go-semver/semver"
	humanize "github.com/dustin/go-humanize"
	"github.com/iScript/etcd-cr/etcdserver/api"
	"github.com/iScript/etcd-cr/etcdserver/api/membership"
	"github.com/iScript/etcd-cr/etcdserver/api/rafthttp"
	"github.com/iScript/etcd-cr/etcdserver/api/snap"
	stats "github.com/iScript/etcd-cr/etcdserver/api/v2stats"
	"github.com/iScript/etcd-cr/etcdserver/api/v2store"
	pb "github.com/iScript/etcd-cr/etcdserver/etcdserverpb"
	"github.com/iScript/etcd-cr/lease"
	"github.com/iScript/etcd-cr/mvcc"
	"github.com/iScript/etcd-cr/mvcc/backend"
	"github.com/iScript/etcd-cr/pkg/fileutil"
	"github.com/iScript/etcd-cr/pkg/idutil"
	"github.com/iScript/etcd-cr/pkg/types"
	"github.com/iScript/etcd-cr/pkg/wait"
	"github.com/iScript/etcd-cr/raft"
	"github.com/iScript/etcd-cr/raft/raftpb"
	"github.com/iScript/etcd-cr/version"
	"github.com/iScript/etcd-cr/wal"
	"go.uber.org/zap"
)

const (
	DefaultSnapshotCount = 100000

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

var (
	//plog = capnslog.NewPackageLogger("go.etcd.io/etcd", "etcdserver")

	storeMemberAttributeRegexp = regexp.MustCompile(path.Join(membership.StoreMembersPrefix, "[[:xdigit:]]{1,16}", "attributes"))
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
	Event *v2store.Event
	// Watcher v2store.Watcher
	Err error
}

type ServerV2 interface {
	Server
	Leader() types.ID

	// 处理一个request ， 返回一个response
	//Do(ctx context.Context, r pb.Request) (Response, error)
	// stats.Stats
	// ClientCertAuthEnabled() bool
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

	Cluster() api.Cluster
	// Alarms() []*pb.AlarmMember
}

type EtcdServer struct {
	// 当前已发出去但未收到响应的快照个数
	inflightSnapshots int64  // must use atomic operations to access; keep 64-bit aligned.
	appliedIndex      uint64 // must use atomic operations to access; keep 64-bit aligned.
	committedIndex    uint64 // must use atomic operations to access; keep 64-bit aligned.
	term              uint64 // must use atomic operations to access; keep 64-bit aligned.
	lead              uint64 // must use atomic operations to access; keep 64-bit aligned.
	// 底层的原子级内存操作，其执行过程不能被中断，这也就保证了同一时刻一个线程的执行不会被其他线程中断，也保证了多线程下数据操作的一致性。

	consistIndex consistentIndex // 当前执行entry的index
	r            raftNode        // raftnode，是etcdserver实例与底层etcd-raft模块通信的桥梁

	readych chan struct{}
	Cfg     ServerConfig

	lgMu *sync.RWMutex
	lg   *zap.Logger
	w    wait.Wait //负责协调后台多个goroutine之间的执行

	readMu sync.RWMutex
	// read routine notifies etcd server that it waits for reading by sending an empty struct to
	// readwaitC
	readwaitc chan struct{}
	// 通知read协程可以处理请求
	readNotifier *notifier

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
	applyWait wait.WaitTime //WaitTime是上面介绍的Wait之上的一层扩展。记录的id是有序的

	kv     mvcc.ConsistentWatchableKV //v3版本的存储
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
	reqIDGen *idutil.Generator // 用于生成请求的唯一标示

	// forceVersionC is used to force the version monitor loop
	// to detect the cluster version immediately.
	forceVersionC chan struct{}

	// wgMu blocks concurrent waitgroup mutation while server stopping
	wgMu sync.RWMutex

	wg sync.WaitGroup // 通过该字段等待后台所有goroutine全部退出

	// ctx用于etcd启动的请求，这些请求可能需要在etcd服务器关闭时取消。
	ctx    context.Context
	cancel context.CancelFunc

	leadTimeMu      sync.RWMutex
	leadElectedTime time.Time // 记录当前节点最近一次转换成Leader状态的时间戳

	//*AccessController
}

func NewServer(cfg ServerConfig) (srv *EtcdServer, err error) {

	// 创建v2版本存储，这里指定了2个只读目录，常量值分别是 /0 /1
	st := v2store.New(StoreClusterPrefix, StoreKeysPrefix)

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

	fmt.Println("serverid:", id, uint16(id))

	heartbeat := time.Duration(cfg.TickMs) * time.Millisecond //100毫秒， TickMs在embed/config中定义，默认为100
	srv = &EtcdServer{
		readych: make(chan struct{}),
		Cfg:     cfg, //serverConfig
		lgMu:    new(sync.RWMutex),
		lg:      cfg.Logger,
		errorc:  make(chan error, 1),
		v2store: st,
		//snapshotter: ss,  etcdserver/api/snap
		r: *newRaftNode(
			raftNodeConfig{
				lg:          cfg.Logger,
				isIDRemoved: func(id uint64) bool { return cl.IsIDRemoved(types.ID(id)) },
				Node:        n, // n实现了Node接口
				heartbeat:   heartbeat,
				raftStorage: s,
				storage:     NewStorage(w, ss),
			},
		),
		id:         id,
		attributes: membership.Attributes{Name: cfg.Name, ClientURLs: cfg.ClientURLs.StringSlice()},
		cluster:    cl,
		stats:      sstats,
		lstats:     lstats,
		SyncTicker: time.NewTicker(500 * time.Millisecond), //同步ticker，500毫秒
		reqIDGen:   idutil.NewGenerator(uint16(id), time.Now()),
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
	//srv.kv = mvcc.New(srv.getLogger(), srv.be, srv.lessor, &srv.consistIndex, mvcc.StoreConfig{CompactionBatchLimit: cfg.CompactionBatchLimit})
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
	srv.r.transport = tr

	return srv, nil

}

func (s *EtcdServer) getLogger() *zap.Logger {
	s.lgMu.RLock()
	l := s.lg
	s.lgMu.RUnlock()
	return l
}

func tickToDur(ticks int, tickMs uint) string {
	return fmt.Sprintf("%v", time.Duration(ticks)*time.Duration(tickMs)*time.Millisecond)
}

func (s *EtcdServer) adjustTicks() {

	lg := s.getLogger()
	clusterN := len(s.cluster.Members())
	// 单机
	if clusterN == 1 {
		ticks := s.Cfg.ElectionTicks - 1
		//fmt.Println("electiontttt", s.Cfg.ElectionTicks)
		if lg != nil {
			lg.Info(
				"started as single-node; fast-forwarding election ticks",
				zap.String("local-member-id", s.ID().String()),
				zap.Int("forward-ticks", ticks),
				zap.String("forward-duration", tickToDur(ticks, s.Cfg.TickMs)),
				zap.Int("election-ticks", s.Cfg.ElectionTicks),
				zap.String("election-timeout", tickToDur(s.Cfg.ElectionTicks, s.Cfg.TickMs)),
			)
		}
		s.r.advanceTicks(ticks)
		return
	}
	//fmt.Println(clusterN)
}

//Server初始化开始处理请求
func (s *EtcdServer) Start() {
	s.start()

	//开启几个协程
	s.goAttach(func() { s.adjustTicks() })
	s.goAttach(func() { s.publish(s.Cfg.ReqTimeout()) })
	// s.goAttach(s.purgeFile)
	// s.goAttach(func() { monitorFileDescriptor(s.getLogger(), s.stopping) })
	// s.goAttach(s.monitorVersions)
	// s.goAttach(s.linearizableReadLoop)
	// s.goAttach(s.monitorKVHash)
}

func (s *EtcdServer) start() {
	//fmt.Println("etcdserver start")

	lg := s.getLogger()

	// 默认上面定义的10000
	if s.Cfg.SnapshotCount == 0 {
		if lg != nil {
			lg.Info(
				"updating snapshot-count to default",
				zap.Uint64("given-snapshot-count", s.Cfg.SnapshotCount),
				zap.Uint64("updated-snapshot-count", DefaultSnapshotCount),
			)
		}
		s.Cfg.SnapshotCount = DefaultSnapshotCount
	}
	if s.Cfg.SnapshotCatchUpEntries == 0 {
		if lg != nil {
			lg.Info(
				"updating snapshot catch-up entries to default",
				zap.Uint64("given-snapshot-catchup-entries", s.Cfg.SnapshotCatchUpEntries),
				zap.Uint64("updated-snapshot-catchup-entries", DefaultSnapshotCatchUpEntries),
			)
		}
		s.Cfg.SnapshotCatchUpEntries = DefaultSnapshotCatchUpEntries
	}

	s.w = wait.New()
	s.applyWait = wait.NewTimeList()
	s.done = make(chan struct{})
	s.stop = make(chan struct{})
	s.stopping = make(chan struct{})

	//context.Background函数的返回Context树的根节点
	//func WithCancel(parent Context) 根据父节点 返回子节点和cancel function
	//s.ctx传入goroutine中，通过s.cancel取消该goroutine
	s.ctx, s.cancel = context.WithCancel(context.Background())

	// 第二个参数size，带缓存的channel，该通道最多可以暂存size个元素值，当发送第size +1个元素值后, 才会阻塞
	// 若没设置2参数或0，接收channel前一直被阻塞着，直到向channel发消息
	s.readwaitc = make(chan struct{}, 1)
	s.readNotifier = newNotifier()
	//特殊的struct{}类型的channel，它不能被写入任何数据，只有通过close()函数进行关闭操作，才能进行输出操作
	s.leaderChanged = make(chan struct{})

	// fmt.Println(s.ClusterVersion(), s.cluster)
	// nil
	if s.ClusterVersion() != nil {
		if lg != nil {
			lg.Info(
				"starting etcd server",
				zap.String("local-member-id", s.ID().String()),
				zap.String("local-server-version", version.Version),
				//zap.String("cluster-id", s.Cluster().ID().String()),
				zap.String("cluster-version", version.Cluster(s.ClusterVersion().String())),
			)
		}
		//membership.ClusterVersionMetrics.With(prometheus.Labels{"cluster_version": s.ClusterVersion().String()}).Set(1)
	} else {
		if lg != nil {
			lg.Info(
				"starting etcd server",
				zap.String("local-member-id", s.ID().String()),
				zap.String("local-server-version", version.Version),
				zap.String("cluster-version", "to_be_decided"),
			)
		}
	}

	go s.run()
}

func (s *EtcdServer) purgeFile() {}

func (s *EtcdServer) Cluster() api.Cluster { return s.cluster } // s.cluster需要实现api.Cluster接口

func (s *EtcdServer) ApplyWait() <-chan struct{} { return s.applyWait.Wait(s.getCommittedIndex()) }

type ServerPeer interface {
	ServerV2
	RaftHandler() http.Handler
	LeaseHandler() http.Handler
}

func (s *EtcdServer) LeaseHandler() http.Handler {
	if s.lessor == nil {
		return nil
	}
	return nil
	//
	//return leasehttp.NewHandler(s.lessor, s.ApplyWait)
}

// RaftHandle 返回 raftnode -> transport(api/rafthttp.Transporter) ->Handle
func (s *EtcdServer) RaftHandler() http.Handler { return s.r.transport.Handler() }

type etcdProgress struct {
	confState raftpb.ConfState
	snapi     uint64
	appliedt  uint64
	appliedi  uint64
}

// 方法用于被raftnode调用
// 使逻辑与raft算法分离
type raftReadyHandler struct {
	getLead              func() (lead uint64)
	updateLead           func(lead uint64)
	updateLeadership     func(newLeader bool)
	updateCommittedIndex func(uint64)
}

func (s *EtcdServer) run() {
	lg := s.getLogger()
	_, err := s.r.raftStorage.Snapshot() // raft/storage MemoryStorage

	if err != nil {
		if lg != nil {
			lg.Panic("failed to get snapshot from Raft storage", zap.Error(err))
		}
	}

	//sched := schedule.NewFIFOScheduler()	// FIFO调度器

	// var (
	// 	smu   sync.RWMutex
	// 	syncC <-chan time.Time
	// )

	//设置发送sync消息的定时器
	// setSyncC := func(ch <-chan time.Time) {
	// 	smu.Lock()
	// 	syncC = ch
	// 	smu.Unlock()
	// }
	// getSyncC := func() (ch <-chan time.Time) {
	// 	smu.RLock()
	// 	ch = syncC
	// 	smu.RUnlock()
	// 	return
	// }

	rh := &raftReadyHandler{
		getLead:    func() (lead uint64) { return s.getLead() },
		updateLead: func(lead uint64) { s.setLead(lead) },

		// raftnode在处理etcd-raft模块返回的Ready.SoftState字段时，会调用该方法
		updateLeadership: func(newLeader bool) {
			// if !s.isLeader() {
			// 	if s.lessor != nil {
			// 		s.lessor.Demote()
			// 	}
			// 	if s.compactor != nil {
			// 		s.compactor.Pause()
			// 	}
			// 	setSyncC(nil)
			// } else {
			// 	if newLeader {
			// 		t := time.Now()
			// 		s.leadTimeMu.Lock()
			// 		s.leadElectedTime = t
			// 		s.leadTimeMu.Unlock()
			// 	}
			// 	setSyncC(s.SyncTicker.C)
			// 	if s.compactor != nil {
			// 		s.compactor.Resume()
			// 	}
			// }
			// if newLeader {
			// 	s.leaderChangedMu.Lock()
			// 	lc := s.leaderChanged
			// 	s.leaderChanged = make(chan struct{})
			// 	close(lc)
			// 	s.leaderChangedMu.Unlock()
			// }

			// if s.stats != nil {
			// 	s.stats.BecomeLeader()
			// }
		},
		// raftnode处理apply实例时会调用该方法
		updateCommittedIndex: func(ci uint64) {
			cci := s.getCommittedIndex()
			if ci > cci {
				s.setCommittedIndex(ci)
			}
		},
	}

	s.r.start(rh)

	//记录当前快照相关的元数据信息和已应用entry记录的位置信息
	// ep := etcdProgress{
	// 	confState: sn.Metadata.ConfState,
	// 	snapi:     sn.Metadata.Index,
	// 	appliedt:  sn.Metadata.Term,
	// 	appliedi:  sn.Metadata.Index,
	// }

	defer func() {
		s.wgMu.Lock() // block concurrent waitgroup adds in goAttach while stopping
		close(s.stopping)
		s.wgMu.Unlock()
		s.cancel()

		//...
	}()

	var expiredLeaseC <-chan []*lease.Lease
	if s.lessor != nil {
		expiredLeaseC = s.lessor.ExpiredLeasesC()
	}

	select {
	case ap := <-s.r.apply():
		fmt.Println("11199999000000000", ap)
	case leases := <-expiredLeaseC:
		fmt.Println("222", leases)
	case err := <-s.errorc:
		if lg != nil {
			lg.Warn("server error", zap.Error(err))
			lg.Warn("data-dir used by this member must be removed")
		}
		return
	// case <-getSyncC():
	// 	if s.v2store.HasTTLKeys() {
	// 		s.sync(s.Cfg.ReqTimeout())
	// 	}
	case <-s.stop:
		return
	}

}

func (s *EtcdServer) isLeader() bool {
	return uint64(s.ID()) == s.Lead()
}

// 当server准备好时返回一个通道
//
func (s *EtcdServer) ReadyNotify() <-chan struct{} { return s.readych }

func (s *EtcdServer) stopWithDelay(d time.Duration, err error) {
	select {
	case <-time.After(d):
	case <-s.done:
	}
	select {
	case s.errorc <- err:
	default:
	}
}

// StopNotify returns a channel that receives a empty struct
// when the server is stopped.
func (s *EtcdServer) StopNotify() <-chan struct{} { return s.done }

func (s *EtcdServer) ClusterVersion() *semver.Version {
	if s.cluster == nil {
		return nil
	}
	return s.cluster.Version()
}

// 相关 get/set

func (s *EtcdServer) setCommittedIndex(v uint64) {
	atomic.StoreUint64(&s.committedIndex, v)
}

func (s *EtcdServer) getCommittedIndex() uint64 {
	return atomic.LoadUint64(&s.committedIndex)
}

func (s *EtcdServer) setAppliedIndex(v uint64) {
	atomic.StoreUint64(&s.appliedIndex, v)
}

func (s *EtcdServer) getAppliedIndex() uint64 {
	return atomic.LoadUint64(&s.appliedIndex)
}

func (s *EtcdServer) setTerm(v uint64) {
	atomic.StoreUint64(&s.term, v)
}

func (s *EtcdServer) getTerm() uint64 {
	return atomic.LoadUint64(&s.term)
}

func (s *EtcdServer) setLead(v uint64) {
	atomic.StoreUint64(&s.lead, v)
}

func (s *EtcdServer) getLead() uint64 {
	return atomic.LoadUint64(&s.lead)
}

func (s *EtcdServer) leaderChangedNotify() <-chan struct{} {
	s.leaderChangedMu.RLock()
	defer s.leaderChangedMu.RUnlock()
	return s.leaderChanged
}

//
type RaftStatusGetter interface {
	ID() types.ID
	Leader() types.ID
	CommittedIndex() uint64
	AppliedIndex() uint64
	Term() uint64
}

//相关get

func (s *EtcdServer) ID() types.ID { return s.id }

func (s *EtcdServer) Leader() types.ID { return types.ID(s.getLead()) }

func (s *EtcdServer) Lead() uint64 { return s.getLead() }

func (s *EtcdServer) CommittedIndex() uint64 { return s.getCommittedIndex() }

func (s *EtcdServer) AppliedIndex() uint64 { return s.getAppliedIndex() }

func (s *EtcdServer) Term() uint64 { return s.getTerm() }

// 注册server信息到集群
func (s *EtcdServer) publish(timeout time.Duration) {
	fmt.Println("注册server信息到集群")

	b, err := json.Marshal(s.attributes) // membership.Attributes 成员相关属性，包含name和clienturl
	if err != nil {
		if lg := s.getLogger(); lg != nil {
			lg.Panic("failed to marshal JSON", zap.Error(err))
		}
		return
	}

	req := pb.Request{
		Method: "PUT",
		Path:   membership.MemberAttributesStorePath(s.id), // /0/members/8e9e05c52164694d/attributes
		Val:    string(b),
	}
	//fmt.Println(string(b), membership.MemberAttributesStorePath(s.id))

	for {
		ctx, cancel := context.WithTimeout(s.ctx, timeout) // WithTimeout , 发生超时了，这个时候cancel会自动调用
		_, err := s.Do(ctx, req)
		cancel()

		switch err {
		case nil:

			close(s.readych) // 关闭s.readych ， 会发通知给如 embed/serve.go
			if lg := s.getLogger(); lg != nil {
				lg.Info(
					"published local member to cluster through raft",
					zap.String("local-member-id", s.ID().String()),
					zap.String("local-member-attributes", fmt.Sprintf("%+v", s.attributes)),
					zap.String("request-path", req.Path),
					zap.String("cluster-id", s.cluster.ID().String()),
					zap.Duration("publish-timeout", timeout),
				)
			}
			return
		}
	}
}

func (s *EtcdServer) parseProposeCtxErr(err error, start time.Time) error {
	switch err {
	case context.Canceled:
		return ErrCanceled

	case context.DeadlineExceeded:
		s.leadTimeMu.RLock()
		curLeadElected := s.leadElectedTime
		s.leadTimeMu.RUnlock()
		prevLeadLost := curLeadElected.Add(-2 * time.Duration(s.Cfg.ElectionTicks) * time.Duration(s.Cfg.TickMs) * time.Millisecond)
		if start.After(prevLeadLost) && start.Before(curLeadElected) {
			return ErrTimeoutDueToLeaderFail
		}
		//lead := types.ID(s.getLead())
		// switch lead {
		// case types.ID(raft.None):
		// 	// TODO: return error to specify it happens because the cluster does not have leader now
		// case s.ID():
		// 	if !isConnectedToQuorumSince(s.r.transport, start, s.ID(), s.cluster.Members()) {
		// 		return ErrTimeoutDueToConnectionLost
		// 	}
		// default:
		// 	if !isConnectedSince(s.r.transport, start, lead) {
		// 		return ErrTimeoutDueToConnectionLost
		// 	}
		// }
		return ErrTimeout

	default:
		return err
	}
}

//返回实现了ConsistentWatchableKV接口的对象
func (s *EtcdServer) KV() mvcc.ConsistentWatchableKV { return s.kv }

// func (s *EtcdServer) Backend() backend.Backend {
// 	s.bemu.Lock()
// 	defer s.bemu.Unlock()
// 	return s.be
// }

// func (s *EtcdServer) AuthStore() auth.AuthStore { return s.authStore }

// func (s *EtcdServer) restoreAlarms() error {
// 	s.applyV3 = s.newApplierV3()
// 	as, err := v3alarm.NewAlarmStore(s)
// 	if err != nil {
// 		return err
// 	}
// 	s.alarmStore = as
// 	if len(as.Get(pb.AlarmType_NOSPACE)) > 0 {
// 		s.applyV3 = newApplierV3Capped(s.applyV3)
// 	}
// 	if len(as.Get(pb.AlarmType_CORRUPT)) > 0 {
// 		s.applyV3 = newApplierV3Corrupt(s.applyV3)
// 	}
// 	return nil
// }

// 根据传入的f开启一个协程
func (s *EtcdServer) goAttach(f func()) {
	s.wgMu.RLock()
	defer s.wgMu.RUnlock()
	select {
	case <-s.stopping: // 检查当前etcd实例是否已经停止，停止则return
		if lg := s.getLogger(); lg != nil {
			lg.Warn("server has stopped; skipping goAttach")
		}
		return
	default:
		//fmt.Println("select default")
	}

	// now safe to add since waitgroup wait has not started yet
	s.wg.Add(1) //
	go func() { // 启动一个goroutine
		defer s.wg.Done() // goroutine结束时调用
		f()
	}()
}
