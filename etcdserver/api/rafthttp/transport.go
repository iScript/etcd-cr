package rafthttp

import (
	"net/http"
	"sync"
	"time"

	stats "github.com/iScript/etcd-cr/etcdserver/api/v2stats"
	"github.com/iScript/etcd-cr/pkg/transport"
	"github.com/iScript/etcd-cr/pkg/types"

	"github.com/xiang90/probing"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

type Raft interface {
	//Process(ctx context.Context, m raftpb.Message) error  // 将制定的消息传递到底层的etcd-raft模块处理
	//IsIDRemoved(id uint64) bool                           // 检测指定节点是否从当前集群中移除
	//ReportUnreachable(id uint64)                          // 通知底层etcd-raft模块，当前节点与指定的节点无法连通
	//ReportSnapshot(id uint64, status raft.SnapshotStatus) // 通知底层etcd-raft模块，快照数据是否发送成功
}

type Transporter interface {

	// 初始化操作
	//Start() error

	// handle实例
	Handler() http.Handler

	// 发送消息
	// Send(m []raftpb.Message)

	// // 发送快照
	// //SendSnapshot(m snap.Message)

	// // 在集群中添加一个节点时，其他节点会通过该方法添加该新加入节点的信息
	// AddRemote(id types.ID, urls []string)

	// // 对peer的相关操作，Peer接口是当前节点对集群中其他节点的抽象表示
	// AddPeer(id types.ID, urls []string)
	// RemovePeer(id types.ID)
	// RemoveAllPeers()
	// UpdatePeer(id types.ID, urls []string)
	// ActiveSince(id types.ID) time.Time
	// ActivePeers() int

	// // 关闭操作，该方法会关闭全部的网络链接
	// Stop()
}

// 是Transporter接口的具体实现
// 用于接收发送raft消息
type Transport struct {
	Logger *zap.Logger

	DialTimeout time.Duration

	DialRetryFrequency rate.Limit

	TLSInfo transport.TLSInfo

	ID        types.ID   // 当前节点自己的id
	URLs      types.URLs // 当前节点与集群中其他节点交互时使用的URL地址
	ClusterID types.ID   // 当前节点所在的集群id
	Raft      Raft       // raft实例
	//Snapshotter *snap.Snapshotter  // 负责管理快照文件
	ServerStats *stats.ServerStats // server状态
	LeaderStats *stats.LeaderStats // Leader状态
	ErrorC      chan error

	streamRt   http.RoundTripper // stream消息通道中使用的RoundTripper （stream消息通道负责传输数据量小、发送频繁的消息）
	pipelineRt http.RoundTripper // pipe消息通道中使用的RoundTripper

	mu      sync.RWMutex         //
	remotes map[types.ID]*remote // remote封装pipeline实例，发送快照消息，帮新加入的节点快速赶上其他节点
	peers   map[types.ID]Peer    // 节点id和peer的映射关系

	pipelineProber probing.Prober // 探测pipeline消息通道是否可用
	streamProber   probing.Prober // 探测stream消息通道是否可用
}

func (t *Transport) Start() error {
	var err error

	//创建stream消息通道使用的http.roundtripper实例
	t.streamRt, err = newStreamRoundTripper(t.TLSInfo, t.DialTimeout)
	if err != nil {
		return err
	}

	//创建pipeline消息通道使用的http.roundtripper实例
	//超时时间永不过期
	t.pipelineRt, err = NewRoundTripper(t.TLSInfo, t.DialTimeout)
	if err != nil {
		return err
	}

	t.remotes = make(map[types.ID]*remote)
	t.peers = make(map[types.ID]Peer)
	t.pipelineProber = probing.NewProber(t.pipelineRt)
	t.streamProber = probing.NewProber(t.streamRt)

	if t.DialRetryFrequency == 0 {
		t.DialRetryFrequency = rate.Every(100 * time.Millisecond)
	}

	//fmt.Println("rafthttp start")

	return nil
}

func (t *Transport) Handler() http.Handler {
	pipelineHandler := newPipelineHandler(t, t.Raft, t.ClusterID)
	// streamHandler := newStreamHandler(t, t, t.Raft, t.ID, t.ClusterID)
	// snapHandler := newSnapshotHandler(t, t.Raft, t.Snapshotter, t.ClusterID)
	// 路由规则
	mux := http.NewServeMux()
	mux.Handle(RaftPrefix, pipelineHandler) //第二个参数Handle是个接口，需要实现servehttp方法
	// mux.Handle(RaftStreamPrefix+"/", streamHandler)
	// mux.Handle(RaftSnapshotPrefix, snapHandler)
	// mux.Handle(ProbingPrefix, probing.NewHandler())
	return mux
}

func (t *Transport) AddRemote(id types.ID, us []string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.remotes == nil {
		// there's no clean way to shutdown the golang http server
		// (see: https://github.com/golang/go/issues/4674) before
		// stopping the transport; ignore any new connections.
		return
	}

	if _, ok := t.peers[id]; ok {
		return
	}
	if _, ok := t.remotes[id]; ok {
		return
	}
	urls, err := types.NewURLs(us)
	if err != nil {
		if t.Logger != nil {
			t.Logger.Panic("failed NewURLs", zap.Strings("urls", us), zap.Error(err))
		}
	}
	t.remotes[id] = startRemote(t, urls, id)
	if t.Logger != nil {
		t.Logger.Info(
			"added new remote peer",
			zap.String("local-member-id", t.ID.String()),
			zap.String("remote-peer-id", id.String()),
			zap.Strings("remote-peer-urls", us),
		)
	}
}

func (t *Transport) AddPeer(id types.ID, us []string) {

}
