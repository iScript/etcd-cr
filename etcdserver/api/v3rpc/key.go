package v3rpc

import (
	"context"

	"github.com/iScript/etcd-cr/etcdserver"
	"github.com/iScript/etcd-cr/etcdserver/api/v3rpc/rpctypes"
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
	if err := checkPutRequest(r); err != nil {
		return nil, err
	}
	// s.kv是实现了etcdserver.RaftKV接口的， v3_server中etcdserver实现了相关接口
	_, err := s.kv.Put(ctx, r)
	return nil, err
}

func checkRangeRequest(r *pb.RangeRequest) error {
	// key byte类型，是否有key
	if len(r.Key) == 0 {
		return rpctypes.ErrGRPCEmptyKey
	}
	return nil
}

func checkPutRequest(r *pb.PutRequest) error {

	// key byte类型，是否有key
	if len(r.Key) == 0 {
		return rpctypes.ErrGRPCEmptyKey
	}
	// 同时传了ignore和值
	if r.IgnoreValue && len(r.Value) != 0 {
		return rpctypes.ErrGRPCValueProvided
	}
	// 同时传了ignore和值
	if r.IgnoreLease && r.Lease != 0 {
		return rpctypes.ErrGRPCLeaseProvided
	}
	return nil
}
