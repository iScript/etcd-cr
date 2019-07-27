package etcdserver

import (
	"fmt"
	"net/http"
	"time"

	"github.com/iScript/etcd-cr/etcdserver/api/membership"
	"go.uber.org/zap"
)

// 指定的member在集群中是否已经启动
func isMemberBootstrapped(lg *zap.Logger, cl *membership.RaftCluster, member string, rt http.RoundTripper, timeout time.Duration) bool {
	//rcl, err := getClusterFromRemotePeers(lg, getRemotePeerURLs(cl, member), timeout, false, rt)
	// if err != nil {
	// 	return false
	// }
	// id := cl.MemberByName(member).ID
	// m := rcl.Member(id)
	// if m == nil {
	// 	return false
	// }
	// if len(m.ClientURLs) > 0 {
	// 	return true
	// }
	return false
}

func GetClusterFromRemotePeers(lg *zap.Logger, urls []string, rt http.RoundTripper) (*membership.RaftCluster, error) {
	return getClusterFromRemotePeers(lg, urls, 10*time.Second, true, rt)
}

func getClusterFromRemotePeers(lg *zap.Logger, urls []string, timeout time.Duration, logerr bool, rt http.RoundTripper) (*membership.RaftCluster, error) {
	cc := &http.Client{
		Transport: rt, //传入自定义的transport，实现http.RoundTripper接口
		Timeout:   timeout,
	}
	fmt.Println(cc, timeout)
	return nil, nil
}
