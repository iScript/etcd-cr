package wait

import "sync"

type WaitTime interface {
	// Wait returns a chan that waits on the given logical deadline.
	// The chan will be triggered when Trigger is called with a
	// deadline that is later than the one it is waiting for.
	Wait(deadline uint64) <-chan struct{}
	// Trigger triggers all the waiting chans with an earlier logical deadline.
	Trigger(deadline uint64)
}

var closec chan struct{}

func init() {
	closec = make(chan struct{})
	close(closec)
}

type timeList struct {
	l                   sync.Mutex
	lastTriggerDeadline uint64
	m                   map[uint64]chan struct{}
}

func NewTimeList() *timeList {
	return &timeList{m: make(map[uint64]chan struct{})}
}

func (tl *timeList) Wait(deadline uint64) <-chan struct{} {
	tl.l.Lock()
	defer tl.l.Unlock()
	if tl.lastTriggerDeadline >= deadline {
		return closec
	}
	ch := tl.m[deadline]
	if ch == nil {
		ch = make(chan struct{})
		tl.m[deadline] = ch
	}
	return ch
}

func (tl *timeList) Trigger(deadline uint64) {
	tl.l.Lock()
	defer tl.l.Unlock()
	tl.lastTriggerDeadline = deadline
	for t, ch := range tl.m {
		if t <= deadline {
			delete(tl.m, t)
			close(ch)
		}
	}
}
