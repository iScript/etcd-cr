package etcdserver

import (
	"encoding/json"
	"fmt"
	"path"
	"time"

	"github.com/iScript/etcd-cr/etcdserver/api/membership"
	"github.com/iScript/etcd-cr/etcdserver/api/v2store"
	"github.com/iScript/etcd-cr/pkg/pbutil"
	"go.uber.org/zap"
)

// 用于处理raft消息的接口。
// RequestV2在v2_server中定义，Response在server中定义
type ApplierV2 interface {
	// Delete(r *RequestV2) Response
	// Post(r *RequestV2) Response
	Put(r *RequestV2) Response
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

// func (a *applierV2store) Post(r *RequestV2) Response {
// 	return toResponse(a.store.Create(r.Path, r.Dir, r.Val, true, r.TTLOptions()))
// }

func (a *applierV2store) Put(r *RequestV2) Response {
	fmt.Println("publish do put")
	//ttlOptions := r.TTLOptions()
	_, existsSet := pbutil.GetBool(r.PrevExist)
	switch {
	case existsSet:
		fmt.Println(111)
	case r.PrevIndex > 0 || r.PrevValue != "":
		fmt.Println(222)
	default:

		if storeMemberAttributeRegexp.MatchString(r.Path) { //是否匹配这样的正则 /0/members/8e9e05c52164694d/attributes

			id := membership.MustParseMemberIDFromKey(path.Dir(r.Path)) // 获得id ， 如8e9e05c52164694d
			var attr membership.Attributes
			if err := json.Unmarshal([]byte(r.Val), &attr); err != nil {
				if a.lg != nil {
					a.lg.Panic("failed to unmarshal", zap.String("value", r.Val), zap.Error(err))
				}
			}

			if a.cluster != nil {
				a.cluster.UpdateAttributes(id, attr)
			}

			// 没有消费者，返回一个空的response
			return Response{}
		}
	}

	return Response{}
}

func (r *RequestV2) TTLOptions() v2store.TTLOptionSet {
	refresh, _ := pbutil.GetBool(r.Refresh)
	ttlOptions := v2store.TTLOptionSet{Refresh: refresh}
	if r.Expiration != 0 {
		ttlOptions.ExpireTime = time.Unix(0, r.Expiration)
	}
	return ttlOptions
}

func toResponse(ev *v2store.Event, err error) Response {
	return Response{Event: ev, Err: err}
}
