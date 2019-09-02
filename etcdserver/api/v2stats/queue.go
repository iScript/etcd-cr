package v2stats

import (
	"sync"
	"time"
)

const (
	queueCapacity = 200 //队列容量
)

// 请求状态，一个请求的发送时间和大小
type RequestStats struct {
	SendingTime time.Time
	Size        int
}

// 队列状态
type statsQueue struct {
	items        [queueCapacity]*RequestStats
	size         int
	front        int
	back         int // back和front是干嘛的，初始化为-1，后续看看
	totalReqSize int
	rwl          sync.RWMutex
}

func (q *statsQueue) Len() int {
	return q.size
}

func (q *statsQueue) ReqSize() int {
	return q.totalReqSize
}

// 返回queue中的front和back元素
// 必须同时获取front和back，通过锁
func (q *statsQueue) frontAndBack() (*RequestStats, *RequestStats) {
	q.rwl.RLock()
	defer q.rwl.RUnlock()
	if q.size != 0 {
		return q.items[q.front], q.items[q.back]
	}
	return nil, nil
}

//插入一个requeststate到queue中，并更新状态
func (q *statsQueue) Insert(p *RequestStats) {
	q.rwl.Lock()
	defer q.rwl.Unlock()

	q.back = (q.back + 1) % queueCapacity

	if q.size == queueCapacity {
		q.totalReqSize -= q.items[q.front].Size
		q.front = (q.back + 1) % queueCapacity
	} else {
		q.size++
	}

	q.items[q.back] = p
	q.totalReqSize += q.items[q.back].Size

}

// Rate function returns the package rate and byte rate
func (q *statsQueue) Rate() (float64, float64) {
	front, back := q.frontAndBack()

	if front == nil || back == nil {
		return 0, 0
	}

	if time.Since(back.SendingTime) > time.Second {
		q.Clear()
		return 0, 0
	}

	sampleDuration := back.SendingTime.Sub(front.SendingTime)

	pr := float64(q.Len()) / float64(sampleDuration) * float64(time.Second)

	br := float64(q.ReqSize()) / float64(sampleDuration) * float64(time.Second)

	return pr, br
}

// clear statsQueue
func (q *statsQueue) Clear() {
	q.rwl.Lock()
	defer q.rwl.Unlock()
	q.back = -1
	q.front = 0
	q.size = 0
	q.totalReqSize = 0
}
