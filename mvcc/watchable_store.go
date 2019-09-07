package mvcc

import (
	"sync"

	"github.com/iScript/etcd-cr/lease"
	"github.com/iScript/etcd-cr/mvcc/backend"

	"go.uber.org/zap"
)

// non-const so modifiable by tests
var (
	// chanBufLen is the length of the buffered chan
	// for sending out watched events.
	// TODO: find a good buf value. 1024 is just a random one that
	// seems to be reasonable.
	chanBufLen = 1024

	// maxWatchersPerSync is the number of watchers to sync in a single batch
	maxWatchersPerSync = 512
)

type watchable interface {
	// watch(key, end []byte, startRev int64, id WatchID, ch chan<- WatchResponse, fcs ...FilterFunc) (*watcher, cancelFunc)
	// progress(w *watcher)
	// rev() int64
}

type watchableStore struct {
	*store

	// mu protects watcher groups and batches. It should never be locked
	// before locking store.mu to avoid deadlock.
	mu sync.RWMutex

	// victims are watcher batches that were blocked on the watch channel
	//victims []watcherBatch
	victimc chan struct{}

	// contains all unsynced watchers that needs to sync with events that have happened
	//unsynced watcherGroup

	// contains all synced watchers that are in sync with the progress of the store.
	// The key of the map is the key that the watcher watches on.
	//synced watcherGroup

	stopc chan struct{}
	wg    sync.WaitGroup
}

func New(lg *zap.Logger, b backend.Backend, le lease.Lessor, ig ConsistentIndexGetter, cfg StoreConfig) ConsistentWatchableKV {
	return newWatchableStore(lg, b, le, ig, cfg)
}

func newWatchableStore(lg *zap.Logger, b backend.Backend, le lease.Lessor, ig ConsistentIndexGetter, cfg StoreConfig) *watchableStore {
	// s := &watchableStore{
	// 	store:    NewStore(lg, b, le, ig, cfg),
	// 	victimc:  make(chan struct{}, 1),
	// 	unsynced: newWatcherGroup(),
	// 	synced:   newWatcherGroup(),
	// 	stopc:    make(chan struct{}),
	// }
	return nil
}
