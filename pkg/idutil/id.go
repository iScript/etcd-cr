package idutil

import (
	"math"
	"sync/atomic"
	"time"
)

const (
	tsLen     = 5 * 8 // 5字节的时间戳
	cntLen    = 8     // 1字节的计数器
	suffixLen = tsLen + cntLen
)

// Generator 根据计数器、时间戳和nodeid生成一个唯一标识符.
//
// The initial id is in this format:
// High order 2 bytes are from memberID, next 5 bytes are from timestamp,
// and low order one byte is a counter.
// | prefix   | suffix              |
// | 2 bytes  | 5 bytes   | 1 byte  |
// | memberID | timestamp | cnt     |

type Generator struct {
	// high order 2 bytes
	prefix uint64
	// low order 6 bytes
	suffix uint64
}

func NewGenerator(memberID uint16, now time.Time) *Generator {

	prefix := uint64(memberID) << suffixLen // 左移运算符，左移n位就是乘以2的n次方。 uint64(memberID)乘以2的48次方
	unixMilli := uint64(now.UnixNano()) / uint64(time.Millisecond/time.Nanosecond)
	suffix := lowbit(unixMilli, tsLen) << cntLen
	return &Generator{
		prefix: prefix,
		suffix: suffix,
	}
}

// 通过next生成一个唯一id
func (g *Generator) Next() uint64 {
	suffix := atomic.AddUint64(&g.suffix, 1)   //该随机数每次+1
	id := g.prefix | lowbit(suffix, suffixLen) //或运算
	return id
}

func lowbit(x uint64, n uint) uint64 {
	return x & (math.MaxUint64 >> (64 - n)) // << 右移运算符  math.MaxUint64 除以 2的(64 - n)次方 ,然后和x做与运算
}
