package etcdserver

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/coreos/go-semver/semver"
	humanize "github.com/dustin/go-humanize"
	"github.com/iScript/etcd-cr/etcdserver/api/v2store"
	"github.com/iScript/etcd-cr/pkg/fileutil"
	"github.com/iScript/etcd-cr/pkg/types"
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

	//cluster *membership.RaftCluster

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

	//haveWAL := wal.Exist(cfg.WALDir())
	fmt.Println(cfg.SnapDir())
	// if err = fileutil.TouchDirAll(cfg.SnapDir()); err != nil {
	// 	if cfg.Logger != nil {
	// 		cfg.Logger.Fatal(
	// 			"failed to create snapshot directory",
	// 			zap.String("path", cfg.SnapDir()),
	// 			zap.Error(err),
	// 		)
	// 	}
	// }

	return nil, nil
}
