package flags

import (
	"flag"
	"sort"
	"strings"
)

//
type StringsValue sort.StringSlice

// 实现 "flag.Value" 接口的set方法
func (ss *StringsValue) Set(s string) error {
	*ss = strings.Split(s, ",")
	return nil
}

// 实现 "flag.Value" 接口的string方法
func (ss *StringsValue) String() string { return strings.Join(*ss, ",") }

//
func NewStringsValue(s string) (ss *StringsValue) {
	if s == "" {
		return &StringsValue{}
	}
	ss = new(StringsValue)
	if err := ss.Set(s); err != nil {
		plog.Panicf("new StringsValue should never fail: %v", err)
	}
	return ss
}

//
func StringsFromFlag(fs *flag.FlagSet, flagName string) []string {
	return []string(*fs.Lookup(flagName).Value.(*StringsValue))
}
