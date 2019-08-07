package confchange

import "github.com/iScript/etcd-cr/raft/tracker"

// 方便配置更改
type Changer struct {
	Tracker   tracker.ProgressTracker
	LastIndex uint64
}
