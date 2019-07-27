package etcdserver

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"
	"time"

	"github.com/iScript/etcd-cr/etcdserver/api/membership"
	"go.uber.org/zap"
)

// 指定的member在集群中是否已经启动
// bootstrap timeout默认是1s
func isMemberBootstrapped(lg *zap.Logger, cl *membership.RaftCluster, member string, rt http.RoundTripper, timeout time.Duration) bool {

	rcl, err := getClusterFromRemotePeers(lg, getRemotePeerURLs(cl, member), timeout, false, rt)
	if err != nil {
		return false
	}
	id := cl.MemberByName(member).ID
	m := rcl.Member(id)
	if m == nil {
		return false
	}
	if len(m.ClientURLs) > 0 {
		return true
	}
	return false

}

func GetClusterFromRemotePeers(lg *zap.Logger, urls []string, rt http.RoundTripper) (*membership.RaftCluster, error) {
	return getClusterFromRemotePeers(lg, urls, 10*time.Second, true, rt)
}

func getClusterFromRemotePeers(lg *zap.Logger, urls []string, timeout time.Duration, logerr bool, rt http.RoundTripper) (*membership.RaftCluster, error) {

	cc := &http.Client{
		Transport: rt,      //传入自定义的transport，实现http.RoundTripper接口
		Timeout:   timeout, //
	}

	// 这里url为空？ 后续再看
	for _, u := range urls {
		addr := u + "/members"
		resp, err := cc.Get(addr)
		if err != nil {
			if logerr {
				if lg != nil {
					lg.Warn("failed to get cluster response", zap.String("address", addr), zap.Error(err))
				}
			}
			continue
		}
		b, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()

		fmt.Println(b, "cluster_util urls")

	}
	return nil, fmt.Errorf("could not retrieve cluster information from the given URLs")

}

// 返回集群中远程成员的peer url ， 不包括自己
func getRemotePeerURLs(cl *membership.RaftCluster, local string) []string {
	us := make([]string, 0)

	for _, m := range cl.Members() {
		// member中的name和本地一样，则continue，不放到数组里
		if m.Name == local {
			continue
		}
		us = append(us, m.PeerURLs...)
	}
	sort.Strings(us)
	return us
}
