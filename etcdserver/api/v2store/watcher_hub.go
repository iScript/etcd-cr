package v2store

import (
	"container/list"
	"sync"
)

type watcherHub struct {
	// count must be the first element to keep 64-bit alignment for atomic
	// access

	count int64 // current number of watchers.

	mutex    sync.Mutex
	watchers map[string]*list.List
	//EventHistory *EventHistory
}
