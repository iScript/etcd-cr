package etcdserver

import (
	"github.com/iScript/etcd-cr/etcdserver/api/membership"
	"github.com/iScript/etcd-cr/etcdserver/api/v2store"
	"go.uber.org/zap"
)

// 用于处理raft消息的接口。
// RequestV2在v2_server中定义，Response在server中定义
type ApplierV2 interface {
	// Delete(r *RequestV2) Response
	// Post(r *RequestV2) Response
	// Put(r *RequestV2) Response
	// QGet(r *RequestV2) Response
	// Sync(r *RequestV2) Response
}

func NewApplierV2(lg *zap.Logger, s v2store.Store, c *membership.RaftCluster) ApplierV2 {
	return &applierV2store{lg: lg, store: s, cluster: c}
}

type applierV2store struct {
	lg      *zap.Logger
	store   v2store.Store
	cluster *membership.RaftCluster
}
