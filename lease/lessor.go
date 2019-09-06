package lease

import (
	"errors"
	"sync"
	"time"

	"github.com/iScript/etcd-cr/mvcc/backend"
	"go.uber.org/zap"
)

// etcd3 通过Lease（租约）实现键值对过期的效果

//
const NoLease = LeaseID(0)

// 最大生命周期
const MaxLeaseTTL = 9000000000

var (
	forever = time.Time{}

	leaseBucketName = []byte("lease")

	// maximum number of leases to revoke per second; configurable for tests
	leaseRevokeRate = 1000

	// maximum number of lease checkpoints recorded to the consensus log per second; configurable for tests
	leaseCheckpointRate = 1000

	// the default interval of lease checkpoint
	defaultLeaseCheckpointInterval = 5 * time.Minute

	// maximum number of lease checkpoints to batch into a single consensus log entry
	maxLeaseCheckpointBatchSize = 1000

	// the default interval to check if the expired lease is revoked
	defaultExpiredleaseRetryInterval = 3 * time.Second

	ErrNotPrimary       = errors.New("not a primary lessor")
	ErrLeaseNotFound    = errors.New("lease not found")
	ErrLeaseExists      = errors.New("lease already exists")
	ErrLeaseTTLTooLarge = errors.New("too large lease TTL")
)

type LeaseID int64

type Lessor interface {
	// SetRangeDeleter lets the lessor create TxnDeletes to the store.
	// Lessor deletes the items in the revoked or expired lease by creating
	// new TxnDeletes.
	// SetRangeDeleter(rd RangeDeleter)

	// SetCheckpointer(cp Checkpointer)

	// 授权一个lease，会在指定时间过期
	// Grant(id LeaseID, ttl int64) (*Lease, error)
	// // Revoke revokes a lease with given ID. The item attached to the
	// // given lease will be removed. If the ID does not exist, an error
	// // will be returned.
	// Revoke(id LeaseID) error

	// // Checkpoint applies the remainingTTL of a lease. The remainingTTL is used in Promote to set
	// // the expiry of leases to less than the full TTL when possible.
	// Checkpoint(id LeaseID, remainingTTL int64) error

	// // Attach attaches given leaseItem to the lease with given LeaseID.
	// // If the lease does not exist, an error will be returned.
	// Attach(id LeaseID, items []LeaseItem) error

	// // GetLease returns LeaseID for given item.
	// // If no lease found, NoLease value will be returned.
	// GetLease(item LeaseItem) LeaseID
	// // Detach detaches given leaseItem from the lease with given LeaseID.
	// // If the lease does not exist, an error will be returned.
	// Detach(id LeaseID, items []LeaseItem) error

	// Promote promotes the lessor to be the primary lessor. Primary lessor manages
	// the expiration and renew of leases.
	// Newly promoted lessor renew the TTL of all lease to extend + previous TTL.
	// Promote(extend time.Duration)

	// // Demote demotes the lessor from being the primary lessor.
	// Demote()

	// // Renew renews a lease with given ID. It returns the renewed TTL. If the ID does not exist,
	// // an error will be returned.
	// Renew(id LeaseID) (int64, error)

	// // Lookup gives the lease at a given lease id, if any
	// Lookup(id LeaseID) *Lease

	// // Leases lists all leases.
	// Leases() []*Lease

	// // ExpiredLeasesC returns a chan that is used to receive expired leases.
	ExpiredLeasesC() <-chan []*Lease

	// // Recover recovers the lessor state from the given backend and RangeDeleter.
	// Recover(b backend.Backend, rd RangeDeleter)

	// // Stop stops the lessor for managing leases. The behavior of calling Stop multiple
	// // times is undefined.
	// Stop()
}

type lessor struct {
	mu sync.RWMutex

	// demotec is set when the lessor is the primary.
	// demotec will be closed if the lessor is demoted.
	demotec chan struct{}

	leaseMap             map[LeaseID]*Lease // 记录id到Lease之间的映射
	leaseExpiredNotifier *LeaseExpiredNotifier
	leaseCheckpointHeap  LeaseQueue
	itemMap              map[LeaseItem]LeaseID

	// 从底层存储中删除过期的lease
	//rd RangeDeleter

	// When a lease's deadline should be persisted to preserve the remaining TTL across leader
	// elections and restarts, the lessor will checkpoint the lease by the Checkpointer.
	//cp Checkpointer

	// 底层持久化Lease的存储
	b backend.Backend

	// Lease实例过期的最小值
	minLeaseTTL int64

	// 过期的实例会写入该通道
	expiredC chan []*Lease

	// stopC is a channel whose closure indicates that the lessor should be stopped.
	stopC chan struct{}
	// doneC is a channel whose closure indicates that the lessor is stopped.
	doneC chan struct{}

	lg *zap.Logger

	// Wait duration between lease checkpoints.
	checkpointInterval time.Duration
	// the interval to check if the expired lease is revoked
	expiredLeaseRetryInterval time.Duration
}

type LessorConfig struct {
	MinLeaseTTL                int64
	CheckpointInterval         time.Duration
	ExpiredLeasesRetryInterval time.Duration
}

func NewLessor(lg *zap.Logger, b backend.Backend, cfg LessorConfig) Lessor {
	return newLessor(lg, b, cfg)
}

func newLessor(lg *zap.Logger, b backend.Backend, cfg LessorConfig) *lessor {
	checkpointInterval := cfg.CheckpointInterval
	expiredLeaseRetryInterval := cfg.ExpiredLeasesRetryInterval
	if checkpointInterval == 0 {
		checkpointInterval = defaultLeaseCheckpointInterval
	}
	if expiredLeaseRetryInterval == 0 {
		expiredLeaseRetryInterval = defaultExpiredleaseRetryInterval
	}

	l := &lessor{
		leaseMap:                  make(map[LeaseID]*Lease),
		itemMap:                   make(map[LeaseItem]LeaseID),
		leaseExpiredNotifier:      newLeaseExpiredNotifier(),
		leaseCheckpointHeap:       make(LeaseQueue, 0),
		b:                         b,
		minLeaseTTL:               cfg.MinLeaseTTL,
		checkpointInterval:        checkpointInterval,
		expiredLeaseRetryInterval: expiredLeaseRetryInterval,
		// expiredC is a small buffered chan to avoid unnecessary blocking.
		expiredC: make(chan []*Lease, 16),
		stopC:    make(chan struct{}),
		doneC:    make(chan struct{}),
		lg:       lg,
	}
	//l.initAndRecover()

	// l.initAndRecover()

	// go l.runLoop()

	return l
}

type Lease struct {
	ID           LeaseID // 唯一标识
	ttl          int64   // Lease实例的生命周期，秒
	remainingTTL int64   // 剩余时间
	expiryMu     sync.RWMutex

	expiry time.Time // 过期时间戳

	mu      sync.RWMutex
	itemSet map[LeaseItem]struct{} // key是当前Lease实例绑定的LeaseItem实例，Value始终为空结构体
	revokec chan struct{}          // 该实例被撤销时会关闭该通道
}

type LeaseItem struct {
	Key string
}

func (le *lessor) ExpiredLeasesC() <-chan []*Lease {
	return le.expiredC
}
