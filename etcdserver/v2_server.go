package etcdserver

import (
	"context"

	"github.com/iScript/etcd-cr/etcdserver/api/v2store"
	pb "github.com/iScript/etcd-cr/etcdserver/etcdserverpb"
)

type RequestV2 pb.Request

type RequestV2Handler interface {
	// Post(ctx context.Context, r *RequestV2) (Response, error)
	Put(ctx context.Context, r *RequestV2) (Response, error)
	// Delete(ctx context.Context, r *RequestV2) (Response, error)
	// QGet(ctx context.Context, r *RequestV2) (Response, error)
	// Get(ctx context.Context, r *RequestV2) (Response, error)
	// Head(ctx context.Context, r *RequestV2) (Response, error)
}

type reqV2HandlerEtcdServer struct {
	reqV2HandlerStore
	s *EtcdServer
}

type reqV2HandlerStore struct {
	store   v2store.Store // 实现了Store接口的
	applier ApplierV2
}

// func (a *reqV2HandlerStore) Post(ctx context.Context, r *RequestV2) (Response, error) {
// 	return a.applier.Post(r), nil
// }

func (a *reqV2HandlerStore) Put(ctx context.Context, r *RequestV2) (Response, error) {
	return a.applier.Put(r), nil
}

// 处理一个request请求
func (s *EtcdServer) Do(ctx context.Context, r pb.Request) (Response, error) {

	r.ID = s.reqIDGen.Next()
	h := &reqV2HandlerEtcdServer{
		reqV2HandlerStore: reqV2HandlerStore{
			store:   s.v2store,
			applier: s.applyV2,
		},
		s: s,
	}
	rp := &r                                       //r的指针赋值给rp
	resp, err := ((*RequestV2)(rp)).Handle(ctx, h) // 将rp指针类型转化为equestV2指针类型， 然后Handle
	return resp, err
	//return nil
}

func (r *RequestV2) Handle(ctx context.Context, v2api RequestV2Handler) (Response, error) {
	if r.Method == "GET" && r.Quorum {
		r.Method = "QGET"
	}
	switch r.Method {
	// case "POST":
	// 	return v2api.Post(ctx, r)
	case "PUT":
		return v2api.Put(ctx, r)
		// case "DELETE":
		// 	return v2api.Delete(ctx, r)
		// case "QGET":
		// 	return v2api.QGet(ctx, r)
		// case "GET":
		// 	return v2api.Get(ctx, r)
		// case "HEAD":
		// 	return v2api.Head(ctx, r)
	}
	return Response{}, ErrUnknownMethod
}
