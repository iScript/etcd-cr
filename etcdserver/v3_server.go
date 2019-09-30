package etcdserver

import (
	"context"
	"fmt"
	"time"

	"github.com/gogo/protobuf/proto"
	pb "github.com/iScript/etcd-cr/etcdserver/etcdserverpb"
	//"go.etcd.io/etcd/auth"
)

const (

	// applied索引和committed索引之前有一个最大数量（committed被follower确认后会应用到applied）
	// 2个差距到了5000，则停止接受请求，为了应用的健康
	maxGapBetweenApplyAndCommitIndex = 5000
)

//提供给grpc的相关方法
type RaftKV interface {
	Range(ctx context.Context, r *pb.RangeRequest) (*pb.RangeResponse, error)
	Put(ctx context.Context, r *pb.PutRequest) (*pb.PutResponse, error)
	// DeleteRange(ctx context.Context, r *pb.DeleteRangeRequest) (*pb.DeleteRangeResponse, error)
	// Txn(ctx context.Context, r *pb.TxnRequest) (*pb.TxnResponse, error)
	// Compact(ctx context.Context, r *pb.CompactionRequest) (*pb.CompactionResponse, error)
}

func (s *EtcdServer) Range(ctx context.Context, r *pb.RangeRequest) (*pb.RangeResponse, error) {
	return nil, nil
}

func (s *EtcdServer) Put(ctx context.Context, r *pb.PutRequest) (*pb.PutResponse, error) {

	_, err := s.raftRequest(ctx, pb.InternalRaftRequest{Put: r})
	if err != nil {
		return nil, err
	}
	return nil, nil
	//return resp.(*pb.PutResponse), nil // 类型断言，类似接口的类型转换，resp转为pb.PutResponse
}

func (s *EtcdServer) raftRequestOnce(ctx context.Context, r pb.InternalRaftRequest) (proto.Message, error) {
	result, err := s.processInternalRaftRequestOnce(ctx, r)
	if err != nil {
		return nil, err
	}
	if result.err != nil {
		return nil, result.err
	}
	return result.resp, nil
}

func (s *EtcdServer) raftRequest(ctx context.Context, r pb.InternalRaftRequest) (proto.Message, error) {
	//fmt.Println(r, r.Put)
	for {
		resp, err := s.raftRequestOnce(ctx, r)
		//if err != auth.ErrAuthOldRevision {
		return resp, err
		//}
	}
}

func (s *EtcdServer) processInternalRaftRequestOnce(ctx context.Context, r pb.InternalRaftRequest) (*applyResult, error) {

	ai := s.getAppliedIndex()
	ci := s.getCommittedIndex()

	// ci和ai之间的差距不能大于5000，否则返回TooManyRequests
	if ci > ai+maxGapBetweenApplyAndCommitIndex {
		return nil, ErrTooManyRequests
	}

	r.Header = &pb.RequestHeader{
		ID: s.reqIDGen.Next(), // requestheader 初始化id
	}

	// 认证相关 ， 添加requestheader的其他属性
	// authInfo, err := s.AuthInfoFromCtx(ctx)
	// if err != nil {
	// 	return nil, err
	// }
	// if authInfo != nil {
	// 	r.Header.Username = authInfo.Username
	// 	r.Header.AuthRevision = authInfo.Revision
	// }

	data, err := r.Marshal()

	if err != nil {
		return nil, err
	}

	if len(data) > int(s.Cfg.MaxRequestBytes) {
		return nil, ErrRequestTooLarge
	}

	fmt.Println("request id:", r.Header.ID)

	id := r.ID
	if id == 0 {
		id = r.Header.ID
	}
	ch := s.w.Register(id) //以id注册通道

	cctx, cancel := context.WithTimeout(ctx, s.Cfg.ReqTimeout())
	defer cancel()

	start := time.Now()

	err = s.r.Propose(cctx, data)

	if err != nil {

	}

	//metrics
	proposalsPending.Inc()
	defer proposalsPending.Dec()

	select {
	case x := <-ch:
		return x.(*applyResult), nil
	case <-cctx.Done(): //超时
		proposalsFailed.Inc()
		s.w.Trigger(id, nil)                                // GC wait	删除map里的该通道，传nil，close
		return nil, s.parseProposeCtxErr(cctx.Err(), start) //返回错误
	case <-s.done:
		return nil, ErrStopped
	}

}

// func (s *EtcdServer) AuthInfoFromCtx(ctx context.Context) (*auth.AuthInfo, error) {
// 	authInfo, err := s.AuthStore().AuthInfoFromCtx(ctx)
// 	if authInfo != nil || err != nil {
// 		return authInfo, err
// 	}
// 	if !s.Cfg.ClientCertAuthEnabled {
// 		return nil, nil
// 	}
// 	authInfo = s.AuthStore().AuthInfoFromTLS(ctx)
// 	return authInfo, nil

// }
