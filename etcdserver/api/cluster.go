package api

import (
	"github.com/coreos/go-semver/semver"
	"github.com/iScript/etcd-cr/pkg/types"
)

// 集群接口
type Cluster interface {
	// 集群id
	ID() types.ID
	// // 监听客户端的url
	// ClientURLs() []string
	// // 里面的成员
	// Members() []*membership.Member
	// // 根据id获取成员
	// Member(id types.ID) *membership.Member
	// 版本
	Version() *semver.Version
}
