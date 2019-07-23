package v2store

import (
	"time"
)

const defaultVersion = 2

var minExpireTime time.Time

func init() {
	minExpireTime, _ = time.Parse(time.RFC3339, "2000-01-01T00:00:00Z")

}

type Store interface {
	Version() int
}

type store struct {
	// Root           *node
	// WatcherHub     *watcherHub
	// CurrentIndex   uint64
	// Stats          *Stats
	CurrentVersion int
	// ttlKeyHeap     *ttlKeyHeap  // need to recovery manually
	// worldLock      sync.RWMutex // stop the world lock
	// clock          clockwork.Clock
	// readonlySet    types.Set
}

// 指定的namespace作为初始目录创建store
// 返回一个interface ，所以new 的struct需要实现interface的接口
func New(namespaces ...string) Store {
	s := newStore(namespaces...)
	//s.clock = clockwork.NewRealClock()
	return s
}

func newStore(namespaces ...string) *store {
	s := new(store)
	s.CurrentVersion = defaultVersion
	// s.Root = newDir(s, "/", s.CurrentIndex, nil, Permanent)
	// for _, namespace := range namespaces {
	// 	s.Root.Add(newDir(s, namespace, s.CurrentIndex, s.Root, Permanent))
	// }
	// s.Stats = newStats()
	// s.WatcherHub = newWatchHub(1000)
	// s.ttlKeyHeap = newTtlKeyHeap()
	// s.readonlySet = types.NewUnsafeSet(append(namespaces, "/")...)
	return s
}

// 返回当前store的version
func (s *store) Version() int {
	return s.CurrentVersion
}
