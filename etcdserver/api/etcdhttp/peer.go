package etcdhttp

import (
	"net/http"

	"github.com/iScript/etcd-cr/etcdserver"

	"go.uber.org/zap"
)

const (
	peerMembersPath         = "/members"
	peerMemberPromotePrefix = "/members/promote/"
)

// 创建一个http.Handle 处理peer请求
// 2参数传入的serever实现了ServerPeer接口的
func NewPeerHandler(lg *zap.Logger, s etcdserver.ServerPeer) http.Handler {
	return newPeerHandler(lg, s, s.RaftHandler(), s.LeaseHandler())
}

func newPeerHandler(lg *zap.Logger, s etcdserver.Server, raftHandler http.Handler, leaseHandler http.Handler) http.Handler {

	mux := http.NewServeMux()
	mux.HandleFunc("/", http.NotFound)
	return mux

}
