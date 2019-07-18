package flags

import (
	"errors"
	"sort"
)

// 可选择的string，string值需要为valids中的其中一个
// 需要实现flag.Value 接口.
type SelectiveStringValue struct {
	v      string
	valids map[string]struct{}
}

// 实现 "flag.Value" 接口的string方法
func (ss *SelectiveStringValue) String() string {
	return ss.v
}

// 实现 "flag.Value" 接口的set方法
func (ss *SelectiveStringValue) Set(s string) error {
	if _, ok := ss.valids[s]; ok { //返回第一个参数为key对应的值，第二个参数该key是否存在。 即set传入的值需要在valids的值里有。
		ss.v = s
		return nil
	}
	return errors.New("invalid value")
}

// 返回SelectiveStringValue
func NewSelectiveStringValue(valids ...string) *SelectiveStringValue {
	vm := make(map[string]struct{})
	for _, v := range valids {
		vm[v] = struct{}{}
	}
	return &SelectiveStringValue{valids: vm, v: valids[0]}
}

// 返回可选的值.
func (ss *SelectiveStringValue) Valids() []string {
	s := make([]string, 0, len(ss.valids))
	for k := range ss.valids {
		s = append(s, k)
	}
	sort.Strings(s)
	return s
}
