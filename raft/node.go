package raft

import (
	"errors"
	"fmt"

	pb "github.com/iScript/etcd-cr/raft/raftpb"
)

// 快照状态
type SnapshotStatus int

const (
	SnapshotFinish  SnapshotStatus = 1 // 完成状态
	SnapshotFailure SnapshotStatus = 2 // 失败状态
)

var (
	emptyState = pb.HardState{}

	// 节点已经停止返回
	ErrStopped = errors.New("raft: stopped")
)

//软状态 ， 该状态是不稳定的不需要持久化存储
type SoftState struct {
	Lead      uint64 // must use atomic operations to access; keep 64-bit aligned.
	RaftState StateType
}

func (a *SoftState) equal(b *SoftState) bool {
	return a.Lead == b.Lead && a.RaftState == b.RaftState
}

// Ready 封装了可以读取的信息，用于保存到存储中，或者提交或发送给其他节点
// 这里所有的字段都是只读的
type Ready struct {

	// SoftState 不稳定的状态，不需要存储
	*SoftState

	//信息发送前稳定存储的状态
	pb.HardState

	// 当前节点等待处理的只读请求
	ReadStates []ReadState

	// 从unstable中读取的entry
	Entries []pb.Entry

	//待持久化的快照数据
	Snapshot pb.Snapshot

	// 已经commit的entry数据
	CommittedEntries []pb.Entry

	// 等待发送到其他节点的message消息
	Messages []pb.Message

	// 是否必须同步写到硬盘
	MustSync bool
}

// 判断hardstate是否相等
func isHardStateEqual(a, b pb.HardState) bool {
	return a.Term == b.Term && a.Vote == b.Vote && a.Commit == b.Commit
}

// 判断hardstat是否为空
func IsEmptyHardState(st pb.HardState) bool {
	return isHardStateEqual(st, emptyState)
}

// Node 接口
type Node interface {
	//推进逻辑时钟，即选举计时器和心跳计时器
	//Tick()

	// 选举计时器超时后会调用该方法，将当前节点切换成Candidate状态
	//Campaign(ctx context.Context) error

	// 收到Client发来的写请求时，调用Propose方法进行处理
	//Propose(ctx context.Context, data []byte) error

	// 收到Client发送的修改集群配置的请求
	//ProposeConfChange(ctx context.Context, cc pb.ConfChangeI) error

	// 收到其他节点的消息时，会通过该方法将消息交给底层封装的raft实例进行处理
	//Step(ctx context.Context, msg pb.Message) error

	// Ready returns a channel that returns the current point-in-time state.
	// Users of the Node must call Advance after retrieving the state returned by Ready.
	//
	// NOTE: No committed entries from the next Ready may be applied until all committed entries
	// and snapshots from the previous one have finished.
	//Ready() <-chan Ready

	// Advance notifies the Node that the application has saved progress up to the last Ready.
	// It prepares the node to return the next available Ready.
	//
	// The application should generally call Advance after it applies the entries in last Ready.
	//
	// However, as an optimization, the application may call Advance while it is applying the
	// commands. For example. when the last Ready contains a snapshot, the application might take
	// a long time to apply the snapshot data. To continue receiving Ready without blocking raft
	// progress, it can call Advance before finishing applying the last ready.
	//Advance()
	// ApplyConfChange applies a config change (previously passed to
	// ProposeConfChange) to the node. This must be called whenever a config
	// change is observed in Ready.CommittedEntries.
	//
	// Returns an opaque non-nil ConfState protobuf which must be recorded in
	// snapshots.
	//ApplyConfChange(cc pb.ConfChangeI) *pb.ConfState

	// TransferLeadership attempts to transfer leadership to the given transferee.
	//TransferLeadership(ctx context.Context, lead, transferee uint64)

	// ReadIndex request a read state. The read state will be set in the ready.
	// Read state has a read index. Once the application advances further than the read
	// index, any linearizable read requests issued before the read request can be
	// processed safely. The read state will have the same rctx attached.
	//ReadIndex(ctx context.Context, rctx []byte) error

	// Status returns the current status of the raft state machine.
	//Status() Status
	// ReportUnreachable reports the given node is not reachable for the last send.
	//ReportUnreachable(id uint64)
	// ReportSnapshot reports the status of the sent snapshot. The id is the raft ID of the follower
	// who is meant to receive the snapshot, and the status is SnapshotFinish or SnapshotFailure.
	// Calling ReportSnapshot with SnapshotFinish is a no-op. But, any failure in applying a
	// snapshot (for e.g., while streaming it from leader to follower), should be reported to the
	// leader with SnapshotFailure. When leader sends a snapshot to a follower, it pauses any raft
	// log probes until the follower can apply the snapshot and advance its state. If the follower
	// can't do that, for e.g., due to a crash, it could end up in a limbo, never getting any
	// updates from the leader. Therefore, it is crucial that the application ensures that any
	// failure in snapshot sending is caught and reported back to the leader; so it can resume raft
	// log probing in the follower.
	//ReportSnapshot(id uint64, status SnapshotStatus)
	// Stop performs any necessary termination of the Node.
	//Stop()
}

type Peer struct {
	ID      uint64
	Context []byte
}

// 通过config和peer启动一台node （第二个参数记录了当前集群中全部节点的id）
func StartNode(c *Config, peers []Peer) Node {
	fmt.Println("raft.node.StartNode")

	if len(peers) == 0 {
		// panic 主动抛出错误，立即返回
		panic("no peers given; use RestartNode instead")
	}

	rn, err := NewRawNode(c)
	if err != nil {
		panic(err)
	}
	rn.Bootstrap(peers) //
	n := newNode(rn)
	go n.run()
	//n.run() //调试
	return &n

}

func RestartNode(c *Config) Node {
	rn, err := NewRawNode(c)
	if err != nil {
		panic(err)
	}
	n := newNode(rn)
	go n.run()
	return &n
}

//消息类型
type msgWithResult struct {
	m      pb.Message
	result chan error
}

type node struct {
	propc      chan msgWithResult   // 该通道用于接收MsgProp类型的消息
	recvc      chan pb.Message      // 除MsgProp外的其他类型的消息都由该通道接收
	confc      chan pb.ConfChangeV2 // 当节点收到EntryConfChange类型的Entry记录时，会转换成confchange，并写入该通道中等待
	confstatec chan pb.ConfState    // 该通道用于向上层模块返回ConfState实例
	readyc     chan Ready           // 用于向上层模块返回ready实例
	advancec   chan struct{}        // 上层模块处理完通过readyc获取的ready实例之后，会通过node.Advance方法向该通道写入信号，从而通知底层raft实例
	tickc      chan struct{}        // 接收逻辑时钟发出的信号
	done       chan struct{}        // done通道关闭后，进行相应的操作
	stop       chan struct{}        // 当node.Stop方法被滴啊用时，会向该通道发送信息
	status     chan chan Status     // 传递status

	rn *RawNode
}

// 初始化node
func newNode(rn *RawNode) node {
	return node{
		propc:      make(chan msgWithResult),
		recvc:      make(chan pb.Message),
		confc:      make(chan pb.ConfChangeV2),
		confstatec: make(chan pb.ConfState),
		readyc:     make(chan Ready),
		advancec:   make(chan struct{}),
		tickc:      make(chan struct{}, 128),
		done:       make(chan struct{}),
		stop:       make(chan struct{}),
		status:     make(chan chan Status),
		rn:         rn,
	}
}

// 处理node中封装的全部通道
func (n *node) run() {
	var propc chan msgWithResult
	var readyc chan Ready
	var advancec chan struct{}
	var rd Ready

	r := n.rn.raft
	lead := None

	for {
		if advancec != nil {
			readyc = nil
		}
		// else if n.rn.HasReady() {

		// 	rd = n.rn.readyWithoutAccept()
		// 	readyc = n.readyc
		// }

		// r.lead 不为none
		if lead != r.lead {
			if r.hasLeader() {
				if lead == None {
					r.logger.Infof("raft.node: %x elected leader %x at term %d", r.id, r.lead, r.Term)
				} else {
					r.logger.Infof("raft.node: %x changed leader from %x to %x at term %d", r.id, lead, r.lead, r.Term)
				}
				propc = n.propc
			} else {
				r.logger.Infof("raft.node: %x lost leader %x at term %d", r.id, lead, r.Term)
				propc = nil
			}
			lead = r.lead
		}

		select {
		case pm := <-propc: //读取propc通道，获取MsgPropc消息，并交给raft.Step处理
			m := pm.m
			m.From = r.id
			err := r.Step(m)
			if pm.result != nil {
				pm.result <- err
				close(pm.result)
			}
		case readyc <- rd:
			//n.rn.acceptReady(rd)
			//advancec = n.advancec
		case <-advancec:
			// n.rn.Advance(rd)
			// rd = Ready{}
			// advancec = nil
		}

	}
}
