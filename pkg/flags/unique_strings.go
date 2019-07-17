package flags

import (
	"flag"
	"sort"
	"strings"
)

type UniqueStringsValue struct {
	Values map[string]struct{}
}

// 实现 "flag.Value" 接口的set方法
func (us *UniqueStringsValue) Set(s string) error {
	us.Values = make(map[string]struct{})
	for _, v := range strings.Split(s, ",") {
		us.Values[v] = struct{}{}
	}
	return nil
}

// 实现 "flag.Value" 接口的string方法
func (us *UniqueStringsValue) String() string {
	return strings.Join(us.stringSlice(), ",")
}

func (us *UniqueStringsValue) stringSlice() []string {
	ss := make([]string, 0, len(us.Values))
	for v := range us.Values {
		ss = append(ss, v)
	}
	sort.Strings(ss)
	return ss
}

//返回UniqueStringsValue ，实现 "flag.Value" interface.
func NewUniqueStringsValue(s string) (us *UniqueStringsValue) {
	us = &UniqueStringsValue{Values: make(map[string]struct{})}
	if s == "" {
		return us
	}
	if err := us.Set(s); err != nil {
		plog.Panicf("new UniqueStringsValue should never fail: %v", err)
	}
	return us
}

// 返回 a string slice
func UniqueStringsFromFlag(fs *flag.FlagSet, flagName string) []string {
	return (*fs.Lookup(flagName).Value.(*UniqueStringsValue)).stringSlice()
}

//  返回 a map of strings
func UniqueStringsMapFromFlag(fs *flag.FlagSet, flagName string) map[string]struct{} {
	return (*fs.Lookup(flagName).Value.(*UniqueStringsValue)).Values
}
