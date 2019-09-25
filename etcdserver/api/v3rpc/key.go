package v3rpc

import (
	"context"

	"github.com/iScript/etcd-cr/etcdserver"
	pb "github.com/iScript/etcd-cr/etcdserver/etcdserverpb"
)

type kvServer struct {
	hdr header
	kv  etcdserver.RaftKV // 在v3_server中定义，实际方法在这里处理
	// maxTxnOps is the max operations per txn.
	// e.g suppose maxTxnOps = 128.
	// Txn.Success can have at most 128 operations,
	// and Txn.Failure can have at most 128 operations.
	maxTxnOps uint
}

// 实现grpc kv相关功能
func NewKVServer(s *etcdserver.EtcdServer) pb.KVServer {

	return &kvServer{hdr: newHeader(s), kv: s, maxTxnOps: s.Cfg.MaxTxnOps}
}

func (s *kvServer) Range(ctx context.Context, r *pb.RangeRequest) (*pb.RangeResponse, error) {

	return nil, nil
}

func (s *kvServer) Put(ctx context.Context, r *pb.PutRequest) (*pb.PutResponse, error) {
	//fmt.Println("12312312") // 能收到grpc请求
	return nil, nil
}
