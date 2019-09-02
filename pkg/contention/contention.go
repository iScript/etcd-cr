package contention

import (
	"sync"
	"time"
)

type TimeoutDetector struct {
	mu          sync.Mutex // protects all
	maxDuration time.Duration
	// map from event to time
	// time is the last seen time of the event.
	records map[uint64]time.Time
}

// NewTimeoutDetector creates the TimeoutDetector.
func NewTimeoutDetector(maxDuration time.Duration) *TimeoutDetector {
	return &TimeoutDetector{
		maxDuration: maxDuration,
		records:     make(map[uint64]time.Time),
	}
}
