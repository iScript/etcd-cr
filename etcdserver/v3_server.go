package etcdserver

import (
	"context"

	pb "github.com/iScript/etcd-cr/etcdserver/etcdserverpb"
)

const (
	// In the health case, there might be a small gap (10s of entries) between
	// the applied index and committed index.
	// However, if the committed entries are very heavy to apply, the gap might grow.
	// We should stop accepting new proposals if the gap growing to a certain point.
	maxGapBetweenApplyAndCommitIndex = 5000
)

type RaftKV interface {
	Range(ctx context.Context, r *pb.RangeRequest) (*pb.RangeResponse, error)
	// Put(ctx context.Context, r *pb.PutRequest) (*pb.PutResponse, error)
	// DeleteRange(ctx context.Context, r *pb.DeleteRangeRequest) (*pb.DeleteRangeResponse, error)
	// Txn(ctx context.Context, r *pb.TxnRequest) (*pb.TxnResponse, error)
	// Compact(ctx context.Context, r *pb.CompactionRequest) (*pb.CompactionResponse, error)
}
