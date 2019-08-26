package lease

// LeaseWithTime contains lease object with a time.
// For the lessor's lease heap, time identifies the lease expiration time.
// For the lessor's lease checkpoint heap, the time identifies the next lease checkpoint time.
type LeaseWithTime struct {
	id LeaseID
	// Unix nanos timestamp.
	time  int64
	index int
}

type LeaseQueue []*LeaseWithTime

func (pq LeaseQueue) Len() int { return len(pq) }

func (pq LeaseQueue) Less(i, j int) bool {
	return pq[i].time < pq[j].time
}

func (pq LeaseQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *LeaseQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*LeaseWithTime)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *LeaseQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}

// LeaseExpiredNotifier is a queue used to notify lessor to revoke expired lease.
// Only save one item for a lease, `Register` will update time of the corresponding lease.
type LeaseExpiredNotifier struct {
	m     map[LeaseID]*LeaseWithTime
	queue LeaseQueue
}

func newLeaseExpiredNotifier() *LeaseExpiredNotifier {
	return &LeaseExpiredNotifier{
		m:     make(map[LeaseID]*LeaseWithTime),
		queue: make(LeaseQueue, 0),
	}
}
