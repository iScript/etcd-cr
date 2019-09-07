package v3rpc

import (
	"github.com/iScript/etcd-cr/etcdserver"
	pb "github.com/iScript/etcd-cr/etcdserver/etcdserverpb"
)

type kvServer struct {
	hdr header
	kv  etcdserver.RaftKV
	// maxTxnOps is the max operations per txn.
	// e.g suppose maxTxnOps = 128.
	// Txn.Success can have at most 128 operations,
	// and Txn.Failure can have at most 128 operations.
	maxTxnOps uint
}

func NewKVServer(s *etcdserver.EtcdServer) pb.KVServer {
	return nil
	//return &kvServer{hdr: newHeader(s), kv: s, maxTxnOps: s.Cfg.MaxTxnOps}
}
