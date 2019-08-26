package lease

import (
	"errors"
	"sync"
	"time"

	"github.com/iScript/etcd-cr/mvcc/backend"
	"go.uber.org/zap"
)

//
const NoLease = LeaseID(0)

// MaxLeaseTTL is the maximum lease TTL value
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
}

type lessor struct {
	mu sync.RWMutex

	// demotec is set when the lessor is the primary.
	// demotec will be closed if the lessor is demoted.
	demotec chan struct{}

	leaseMap             map[LeaseID]*Lease
	leaseExpiredNotifier *LeaseExpiredNotifier
	leaseCheckpointHeap  LeaseQueue
	itemMap              map[LeaseItem]LeaseID

	// When a lease expires, the lessor will delete the
	// leased range (or key) by the RangeDeleter.
	//rd RangeDeleter

	// When a lease's deadline should be persisted to preserve the remaining TTL across leader
	// elections and restarts, the lessor will checkpoint the lease by the Checkpointer.
	//cp Checkpointer

	// backend to persist leases. We only persist lease ID and expiry for now.
	// The leased items can be recovered by iterating all the keys in kv.
	b backend.Backend

	// minLeaseTTL is the minimum lease TTL that can be granted for a lease. Any
	// requests for shorter TTLs are extended to the minimum TTL.
	minLeaseTTL int64

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
	ID           LeaseID
	ttl          int64 // time to live of the lease in seconds
	remainingTTL int64 // remaining time to live in seconds, if zero valued it is considered unset and the full ttl should be used
	// expiryMu protects concurrent accesses to expiry
	expiryMu sync.RWMutex
	// expiry is time when lease should expire. no expiration when expiry.IsZero() is true
	expiry time.Time

	// mu protects concurrent accesses to itemSet
	mu sync.RWMutex
	//itemSet map[LeaseItem]struct{}
	revokec chan struct{}
}

type LeaseItem struct {
	Key string
}
