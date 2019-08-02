package flags

import (
	"flag"
	"net/url"
	"sort"
	"strings"

	"github.com/iScript/etcd-cr/pkg/types"
)

type UniqueURLs struct {
	Values  map[string]struct{} //空结构体，节省内存。
	uss     []url.URL
	Allowed map[string]struct{}
}

// 通过flagname 返回该flag对应的UniqueURLs的uss值.
func UniqueURLsFromFlag(fs *flag.FlagSet, urlsFlagName string) []url.URL {
	return (*fs.Lookup(urlsFlagName).Value.(*UniqueURLs)).uss
	// fs.Lookup 通过名称返回该flag
	// flag.Value 为interface
	// Value.(T) 返回该值
	// 即获得flagset中的该flag对应的UniqueURLs
}

// 实现 "flag.Value" 接口的set方法
// 可逗号分隔批量设置
func (us *UniqueURLs) Set(s string) error {

	if _, ok := us.Values[s]; ok {
		return nil
	}
	if _, ok := us.Allowed[s]; ok {
		us.Values[s] = struct{}{}
		return nil
	}
	ss, err := types.NewURLs(strings.Split(s, ","))
	if err != nil {
		return err
	}
	us.Values = make(map[string]struct{})
	us.uss = make([]url.URL, 0)
	for _, v := range ss {
		us.Values[v.String()] = struct{}{}
		us.uss = append(us.uss, v)
	}
	return nil
}

//  实现 "flag.Value" 接口的string方法.
func (us *UniqueURLs) String() string {
	all := make([]string, 0, len(us.Values)) //参数 类型、初始长度、容量，指定容量主要为提供效率
	for u := range us.Values {               // rang map返回的是key值
		all = append(all, u)
	}
	sort.Strings(all)
	return strings.Join(all, ",")
}

// url.URL 切片 实现 flag.Value 接口，.
// 逗号隔开
func NewUniqueURLsWithExceptions(s string, exceptions ...string) *UniqueURLs {
	us := &UniqueURLs{Values: make(map[string]struct{}), Allowed: make(map[string]struct{})}
	for _, v := range exceptions {
		us.Allowed[v] = struct{}{}
	}

	if s == "" {
		return us
	}

	if err := us.Set(s); err != nil {
		plog.Panicf("new UniqueURLs should never fail: %v", err)
	}

	return us
}

// 通过flagname 返回该flag对应的UniqueURLs 的value值.
func UniqueURLsMapFromFlag(fs *flag.FlagSet, urlsFlagName string) map[string]struct{} {
	return (*fs.Lookup(urlsFlagName).Value.(*UniqueURLs)).Values
}
