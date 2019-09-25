package v3rpc

import (
	"github.com/iScript/etcd-cr/etcdserver"
	pb "github.com/iScript/etcd-cr/etcdserver/etcdserverpb"
	"github.com/iScript/etcd-cr/pkg/types"
)

type quotaKVServer struct {
	pb.KVServer //接口，该变量可以存任意实现了该接口的对象，主要是这里，返回grpc相关方法
	qa          quotaAlarmer
}

type quotaAlarmer struct {
	q  etcdserver.Quota
	a  Alarmer
	id types.ID
}

func NewQuotaKVServer(s *etcdserver.EtcdServer) pb.KVServer {
	return &quotaKVServer{
		NewKVServer(s),
		quotaAlarmer{etcdserver.NewBackendQuota(s, "kv"), s, s.ID()},
	}
}
