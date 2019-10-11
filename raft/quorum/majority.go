package quorum

import (
	"fmt"
	"sort"
	"strings"
)

// 一组set，里面id
type MajorityConfig map[uint64]struct{}

func (c MajorityConfig) String() string {
	sl := make([]uint64, 0, len(c))
	for id := range c {
		sl = append(sl, id)
	}
	sort.Slice(sl, func(i, j int) bool { return sl[i] < sl[j] })
	var buf strings.Builder
	buf.WriteByte('(')
	for i := range sl {
		if i > 0 {
			buf.WriteByte(' ')
		}
		fmt.Fprint(&buf, sl[i])
	}
	buf.WriteByte(')')
	return buf.String()
}

// Slice returns the MajorityConfig as a sorted slice.
func (c MajorityConfig) Slice() []uint64 {
	var sl []uint64
	for id := range c {
		sl = append(sl, id)
	}
	sort.Slice(sl, func(i, j int) bool { return sl[i] < sl[j] })
	return sl
}
