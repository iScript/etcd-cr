package raft

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/iScript/etcd-cr/raft/confchange"
	"github.com/iScript/etcd-cr/raft/quorum"
	pb "github.com/iScript/etcd-cr/raft/raftpb"
	"github.com/iScript/etcd-cr/raft/tracker"
)

//当没有leader的时候 None作为占位的nodeid
const None uint64 = 0
const noLimit = math.MaxUint64

//状态类型
const (
	StateFollower StateType = iota
	StateCandidate
	StateLeader
	StatePreCandidate
	numStates
)

type ReadOnlyOption int

const (
	// 定义常量
	// iota常量计数器，初始值为0，const中每增加一行加1。
	// 这里ReadOnlyOption = 0，下面ReadOnlyLeaseBased = 1
	// 相当于间接实现枚举 enum

	ReadOnlySafe ReadOnlyOption = iota
	ReadOnlyLeaseBased
)

// 竞选类型
const (
	// 当config.prevote = true时，预选代表选举的第一阶段。
	campaignPreElection CampaignType = "CampaignPreElection"
	// 正常选举 ， 当config.prevote = true时代表第二阶段
	campaignElection CampaignType = "CampaignElection"
	// 标示leader的转移
	campaignTransfer CampaignType = "CampaignTransfer"
)

// 某些情况下提案被忽略，返回的错误
var ErrProposalDropped = errors.New("raft proposal dropped")

// rand.Rand的简单封装
type lockedRand struct {
	mu   sync.Mutex
	rand *rand.Rand
}

// 对外方法，返回0到n的随机数
func (r *lockedRand) Intn(n int) int {
	r.mu.Lock()
	v := r.rand.Intn(n) //返回从0到n的随机数
	r.mu.Unlock()
	return v
}

//
var globalRand = &lockedRand{
	rand: rand.New(rand.NewSource(time.Now().UnixNano())), //使用随机数种子初始化rand
}

// 竞选类型
type CampaignType string

// 节点的状态
type StateType uint64

var stmap = [...]string{ //数组类型，不指定长度，但根据后面的初始化列表数量来确定长度
	"StateFollower",     // 平民
	"StateCandidate",    // 竞选
	"StateLeader",       // 领导
	"StatePreCandidate", // 预竞选
}

// 输出StateType返回对应的字符串
func (st StateType) String() string {
	return stmap[uint64(st)]
}

//启动raft的配置
type Config struct {
	// ID 是本地raft的唯一标示符，不能为0
	ID uint64

	// 包含集群中所有节点的id.
	peers []uint64

	//  learner nodes 的id
	learners []uint64

	// ElectionTick 选举计时器，用于初始化raft.electionTimeout
	// 每个follower节点在接收不到leader节点的心跳消息之后，并不会立即发起新一轮选举，而是需要等待一段时间之后才切换成candidate发起新一轮选举
	// ElectionTick设置的必须比ElectionTick高，建议为10*HeartbeatTick
	ElectionTick int

	// heartbeat心跳时间，leader维持它的领导而每间隔心跳时间发送信息
	// 收到心跳消息后会重置选举计时器，所以心跳超时时间要远小于选举超时时间
	HeartbeatTick int

	// raft中的日志存储，持久化存储entries和states。
	Storage Storage

	// 当前已经应用的记录位置（已应用的最后一条entry记录的索引值），在重启时需要设置。
	Applied uint64

	// 限制每个附加消息的字节大小
	// 较小的值会降低raft的recover成本 ，另一方面影响复制时的吞吐量
	MaxSizePerMsg uint64

	// MaxCommittedSizePerReady limits the size of the committed entries which
	// can be applied.
	MaxCommittedSizePerReady uint64
	// MaxUncommittedEntriesSize limits the aggregate byte size of the
	// uncommitted entries that may be appended to a leader's log. Once this
	// limit is exceeded, proposals will begin to return ErrProposalDropped
	// errors. Note: 0 for no limit.
	MaxUncommittedEntriesSize uint64

	// 对当前节点来说，已经发送出去但未收到响应的消息个数上限
	// 如果超过这个阈值，则暂停当前节点的的消息发送，防止网络阻塞
	MaxInflightMsgs int

	// 是否开启checkQuorum模式，用于初始化raft.checkQuorum
	CheckQuorum bool

	// 是否开启prevote模式，用于初始化raft.prevote
	PreVote bool

	//与只读请求的处理相关
	ReadOnlyOption ReadOnlyOption

	// raft日志 ， 需要实现log.go接口里的
	Logger Logger

	// DisableProposalForwarding set to true means that followers will drop
	// proposals, rather than forwarding them to the leader. One use case for
	// this feature would be in a situation where the Raft leader is used to
	// compute the data of a proposal, for example, adding a timestamp from a
	// hybrid logical clock to data in a monotonically increasing way. Forwarding
	// should be disabled to prevent a follower with an inaccurate hybrid
	// logical clock from assigning the timestamp and then forwarding the data
	// to the leader.
	DisableProposalForwarding bool
}

func (c *Config) validate() error {
	if c.ID == None {
		return errors.New("cannot use none as id")
	}
	// ...
	return nil
}

type raft struct {
	id uint64 // 当前节点在集群中的id

	Term uint64 //任期，一个全局的连续递增的证书，在raft协议中每进行一次选举任期加一，每个节点都会记录当前的任期值。若为0则为本地消息
	Vote uint64 // 当前任期中当前节点将选票投给了哪个节点

	readStates []ReadState //只读请求

	// 本地log，记录日志
	raftLog *raftLog

	// 单条消息的最大字节数
	maxMsgSize         uint64
	maxUncommittedSize uint64

	//leader节点会记录其他节点日志复制情况，每个follower节点对应的NextIndex和MatchIndex值都封装在Progress实例中
	prs tracker.ProgressTracker

	//当前节点在集群中的角色，4种状态
	state StateType

	// 本机raft是否是learner
	isLearner bool

	// 节点等待发送的消息
	// Type 消息类型 共定义了19种消息类型
	// From 发送消息的节点id  To 目标节点id
	// Term 发送消息节点的任期Term，如果为0则是本地消息
	// Entries entry记录,节点间传递的是消息，消息中有多条entry记录，每条entry记录对应一个独立操作。
	//		字段为 Term  Type  Index  Data
	// 		EntryType:EntryNormal代表普通的数据操作  EntryConfChange 代表集群变更操作
	// LogTerm 该消息携带的第一条entry记录的term值
	// Index 索引值，与具体消息类型相关
	// Commit 消息发送节点的提交位置
	// Snapshot 快照数据
	// Reject 用于消息的响应，标示是否拒绝收到的消息  RejectHint拒绝后，在该字段记录一个Entry索引值
	// Context 消息携带的一些上下文信息
	msgs []pb.Message

	// leader节点的id
	lead uint64

	// leader角色转移的目标节点id
	leadTransferee uint64
	// Only one conf change may be pending (in the log, but not yet
	// applied) at a time. This is enforced via pendingConfIndex, which
	// is set to a value >= the log index of the latest pending
	// configuration change (if any). Config changes are only allowed to
	// be proposed if the leader's applied index is greater than this
	// value.
	pendingConfIndex uint64
	// an estimate of the size of the uncommitted tail of the Raft log. Used to
	// prevent unbounded log growth. Only maintained by the leader. Reset on
	// term changes.
	uncommittedSize uint64

	// 只读请求相关
	readOnly *readOnly

	// 选举计时器，逻辑时钟每推进一次，该字段+1
	electionElapsed int

	// 心跳计时器，逻辑时钟每推进一次，该字段+1
	heartbeatElapsed int

	// 每隔一段时间，Leader尝试连接集群中的其他节点，如果连接的节点没有超过半数，则主动切换成follower节点。防止网络分区等情况
	checkQuorum bool

	//发起选举之前，先进入prevote状态，在prevote状态的节点会先连接集群中的其他节点，
	//能够连接到半数以上的节点，才真正切换成candidate
	//防止不足半数，如只有2个，导致一直选举不成功，term不断增加
	preVote bool

	heartbeatTimeout          int //心跳超时时间，当heartbeatElapsed达到该值，就会触发leader发送心跳
	electionTimeout           int //选举超时时间，当electionElapsed达到该值值，触发选举
	randomizedElectionTimeout int //随机选举超时时间，从固定的选举超时时间加上随机数，防止同时发起选举
	disableProposalForwarding bool

	tick func()   //当前节点推进逻辑时钟的函数，如果节点是leader，指向raft.tickHeartBeat() , 若是其他则指向raft.tickElectioin()
	step stepFunc // 当前节点收到消息时的处理函数，根据节点状态不同指向不同的stepXXX()函数

	logger Logger
}

// new raft对象
func newRaft(c *Config) *raft {
	// 验证配置
	if err := c.validate(); err != nil {
		panic(err.Error())
	}

	// 创建raftlog实例
	raftlog := newLogWithSize(c.Storage, c.Logger, c.MaxCommittedSizePerReady)

	// c.storage = raft.NewMemoryStorage()
	// 返回storage初始状态信息， hardstate、confstate、error
	hs, cs, err := c.Storage.InitialState()

	if err != nil {
		panic(err)
	}

	//peers := c.peers
	if len(cs.Voters) > 0 || len(cs.Learners) > 0 {

	}

	r := &raft{
		id:                        c.ID,
		lead:                      None,
		isLearner:                 false,
		raftLog:                   raftlog,
		maxMsgSize:                c.MaxSizePerMsg,
		maxUncommittedSize:        c.MaxUncommittedEntriesSize,
		prs:                       tracker.MakeProgressTracker(c.MaxInflightMsgs), // 返回ProgressTracker
		electionTimeout:           c.ElectionTick,
		heartbeatTimeout:          c.HeartbeatTick,
		logger:                    c.Logger,
		checkQuorum:               c.CheckQuorum,
		preVote:                   c.PreVote,
		readOnly:                  newReadOnly(c.ReadOnlyOption),
		disableProposalForwarding: c.DisableProposalForwarding,
	}

	// _, _, err := confchange.Restore(confchange.Changer{
	// 	Tracker:   r.prs,
	// 	LastIndex: raftlog.lastIndex(),
	// }, cs)
	// if err != nil {
	// 	panic(err)
	// }

	// 如果不为空  (刚开始为空)
	if !isHardStateEqual(hs, emptyState) {
		r.loadState(hs)
	}
	if c.Applied > 0 {
		//raftlog.appliedTo(c.Applied)
	}

	//成为follower
	r.becomeFollower(r.Term, None)

	var nodesStrs []string
	for _, n := range r.prs.VoterNodes() {
		nodesStrs = append(nodesStrs, fmt.Sprintf("%x", n))
	}

	r.logger.Infof("newRaft %x [peers: [%s], term: %d, commit: %d, applied: %d, lastindex: %d, lastterm: %d]",
		r.id, strings.Join(nodesStrs, ","), r.Term, r.raftLog.committed, r.raftLog.applied, r.raftLog.lastIndex(), r.raftLog.lastTerm())

	return r
}

type stepFunc func(r *raft, m pb.Message) error

// 是否存在leader
func (r *raft) hasLeader() bool { return r.lead != None }

// 获得当前raft的软状态，软状态不持久化存储
func (r *raft) softState() *SoftState { return &SoftState{Lead: r.lead, RaftState: r.state} }

// 获得当前raft的硬状态
func (r *raft) hardState() pb.HardState {

	return pb.HardState{
		Term:   r.Term,
		Vote:   r.Vote,
		Commit: r.raftLog.committed,
	}
}

// maybeCommit attempts to advance the commit index. Returns true if
// the commit index changed (in which case the caller should call
// r.bcastAppend).
// func (r *raft) maybeCommit() bool {
// 	mci := r.prs.Committed()
// 	return r.raftLog.maybeCommit(mci, r.Term)
// }

// 重置字段
func (r *raft) reset(term uint64) {
	if r.Term != term {
		r.Term = term
		r.Vote = None
	}
	r.lead = None

	r.electionElapsed = 0
	r.heartbeatElapsed = 0
	r.resetRandomizedElectionTimeout() //重置选举计时器超时时间

	r.abortLeaderTransfer()

	r.prs.ResetVotes()

	// 重置prs，其中每个progress中的next设置为raftlog.lastIndex
	r.prs.Visit(func(id uint64, pr *tracker.Progress) {
		*pr = tracker.Progress{
			Match:     0,
			Next:      r.raftLog.lastIndex() + 1,
			Inflights: tracker.NewInflights(r.prs.MaxInflight),
			IsLearner: pr.IsLearner,
		}
		if id == r.id {
			pr.Match = r.raftLog.lastIndex()
		}
	})

	r.pendingConfIndex = 0
	r.uncommittedSize = 0
	r.readOnly = newReadOnly(r.readOnly.option)
}

// 周期性地调用该方法推进electionElapsed并检测是否超时,follower时
func (r *raft) tickElection() {
	r.electionElapsed++

	//fmt.Println("选举时间", r.electionElapsed, r.randomizedElectionTimeout, r.promotable(), r.pastElectionTimeout())
	if r.promotable() && r.pastElectionTimeout() {
		r.electionElapsed = 0 //重置
		// 发起选举
		r.Step(pb.Message{From: r.id, Type: pb.MsgHup})
	}
}

// 成为平民
func (r *raft) becomeFollower(term uint64, lead uint64) {
	r.step = stepFollower   //将step字段设置成stepFollower，该函数中封装了Follower节点处理消息的行为
	r.reset(term)           // 重置相关字段
	r.tick = r.tickElection //func赋值
	r.lead = lead           //设置当前集群的leader节点
	r.state = StateFollower // 设置当前节点的角色
	r.logger.Infof("%x became follower at term %d", r.id, r.Term)
}

//成为候选人
func (r *raft) becomeCandidate() {
	// TODO(xiangli) remove the panic when the raft implementation is stable
	if r.state == StateLeader {
		panic("invalid transition [leader -> candidate]")
	}
	r.step = stepCandidate
	r.reset(r.Term + 1)
	r.tick = r.tickElection
	r.Vote = r.id
	r.state = StateCandidate
	r.logger.Infof("%x became candidate at term %d", r.id, r.Term)
}

// 参与选举，转换为候选人状态
func (r *raft) campaign(t CampaignType) {
	//fmt.Println(t)
	if !r.promotable() {
		// This path should not be hit (callers are supposed to check), but
		// better safe than sorry.
		r.logger.Warningf("%x is unpromotable; campaign() should have been called", r.id)
	}

	//var term uint64
	var voteMsg pb.MessageType
	if t == campaignPreElection {
		// r.becomePreCandidate()
		// voteMsg = pb.MsgPreVote
		// // PreVote RPCs are sent for the next term before we've incremented r.Term.
		// term = r.Term + 1
	} else {
		r.becomeCandidate()
		voteMsg = pb.MsgVote
		//term = r.Term
	}

	// 投票
	if _, _, res := r.poll(r.id, voteRespMsgType(voteMsg), true); res == quorum.VoteWon {
		// We won the election after voting for ourselves (which must mean that
		// this is a single-node cluster). Advance to the next state.
		if t == campaignPreElection {
			//r.campaign(campaignElection)

		} else {
			fmt.Println("开始成为leader")
			//
			//r.becomeLeader()
		}
		return
	}

}

// 投票
func (r *raft) poll(id uint64, t pb.MessageType, v bool) (granted int, rejected int, result quorum.VoteResult) {
	if v {
		r.logger.Infof("%x received %s from %x at term %d", r.id, t, id, r.Term)
	} else {
		r.logger.Infof("%x received %s rejection from %x at term %d", r.id, t, id, r.Term)
	}
	r.prs.RecordVote(id, v)
	return r.prs.TallyVotes()
}

func (r *raft) Step(m pb.Message) error {

	switch {
	case m.Term == 0:
		// 本地消息
	case m.Term > r.Term:

	}

	switch m.Type {
	case pb.MsgHup:
		//fmt.Println(r.state, StateLeader)	StateFollower StateLeader
		// 如果不是leader
		if r.state != StateLeader {
			if !r.promotable() {
				r.logger.Warningf("%x is unpromotable and can not campaign; ignoring MsgHup", r.id)
				return nil
			}

			// 判断需要全部applied了才能参与
			// ents, err := r.raftLog.slice(r.raftLog.applied+1, r.raftLog.committed+1, noLimit)
			// if err != nil {
			// 	r.logger.Panicf("unexpected error getting unapplied entries (%v)", err)
			// }
			// if n := numOfPendingConf(ents); n != 0 && r.raftLog.committed > r.raftLog.applied {
			// 	r.logger.Warningf("%x cannot campaign at term %d since there are still %d pending configuration changes to apply", r.id, r.Term, n)
			// 	return nil
			// }

			//参与选举
			if r.preVote {
				r.campaign(campaignPreElection)
			} else {
				r.campaign(campaignElection)
			}
		} else {
			r.logger.Debugf("%x ignoring MsgHup because already leader", r.id)
		}
	}

	return nil
}

func stepCandidate(r *raft, m pb.Message) error {
	return nil
}

// 重置选举超时时间
// 加随机数是因为如果防止2个节点同时过期而同时参与选举，一定程度防止选票不到半数而选举失败
func (r *raft) resetRandomizedElectionTimeout() {
	//electionTimeout = ElectionTick 默认为10
	r.randomizedElectionTimeout = r.electionTimeout + globalRand.Intn(r.electionTimeout)

}

func (r *raft) sendTimeoutNow(to uint64) {
	//r.send(pb.Message{To: to, Type: pb.MsgTimeoutNow})
}

// 中断leader传输
func (r *raft) abortLeaderTransfer() {
	r.leadTransferee = None
}

// 标示状态机是否可提升为leader,
func (r *raft) promotable() bool {
	pr := r.prs.Progress[r.id]
	//fmt.Println(r.prs, 789789)
	return pr != nil && !pr.IsLearner
}

// 集群变更操作
func (r *raft) applyConfChange(cc pb.ConfChangeV2) pb.ConfState {
	cfg, prs, err := func() (tracker.Config, tracker.ProgressMap, error) {

		changer := confchange.Changer{
			Tracker:   r.prs,
			LastIndex: r.raftLog.lastIndex(),
		}
		// if cc.LeaveJoint() {
		// 	return changer.LeaveJoint()
		// } else if autoLeave, ok := cc.EnterJoint(); ok {
		// 	return changer.EnterJoint(autoLeave, cc.Changes...)
		// }

		return changer.Simple(cc.Changes...)

	}()

	if err != nil {
		// TODO(tbg): return the error to the caller.
		panic(err)
	}

	return r.switchToConfig(cfg, prs)

}

func (r *raft) switchToConfig(cfg tracker.Config, prs tracker.ProgressMap) pb.ConfState {
	r.prs.Config = cfg
	r.prs.Progress = prs
	r.logger.Infof("%x switched to configuration %s", r.id, r.prs.Config)

	cs := r.prs.ConfState()
	pr, ok := r.prs.Progress[r.id]

	// Update whether the node itself is a learner, resetting to false when the
	// node is removed.
	r.isLearner = ok && pr.IsLearner

	if (!ok || r.isLearner) && r.state == StateLeader {
		// This node is leader and was removed or demoted. We prevent demotions
		// at the time writing but hypothetically we handle them the same way as
		// removing the leader: stepping down into the next Term.
		//
		// TODO(tbg): step down (for sanity) and ask follower with largest Match
		// to TimeoutNow (to avoid interruption). This might still drop some
		// proposals but it's better than nothing.
		//
		// TODO(tbg): test this branch. It is untested at the time of writing.
		return cs
	}

	// The remaining steps only make sense if this node is the leader and there
	// are other nodes.
	if r.state != StateLeader || len(cs.Voters) == 0 {
		return cs
	}

	// 判断commit ， 广播
	// if r.maybeCommit() {
	// 	// If the configuration change means that more entries are committed now,
	// 	// broadcast/append to everyone in the updated config.
	// 	r.bcastAppend()
	// } else {
	// 	// Otherwise, still probe the newly added replicas; there's no reason to
	// 	// let them wait out a heartbeat interval (or the next incoming
	// 	// proposal).
	// 	r.prs.Visit(func(id uint64, pr *tracker.Progress) {
	// 		r.maybeSendAppend(id, false /* sendIfEmpty */)
	// 	})
	// }
	// If the the leadTransferee was removed, abort the leadership transfer.
	if _, tOK := r.prs.Progress[r.leadTransferee]; !tOK && r.leadTransferee != 0 {
		r.abortLeaderTransfer()
	}

	return cs
}

func (r *raft) loadState(state pb.HardState) {
	fmt.Println(state.Vote, state.Term)
	// if state.Commit < r.raftLog.committed || state.Commit > r.raftLog.lastIndex() {
	// 	r.logger.Panicf("%x state.commit %d is out of range [%d, %d]", r.id, state.Commit, r.raftLog.committed, r.raftLog.lastIndex())
	// }
	// r.raftLog.committed = state.Commit
	// r.Term = state.Term
	// r.Vote = state.Vote
}

// 检测是否超时
func (r *raft) pastElectionTimeout() bool {
	return r.electionElapsed >= r.randomizedElectionTimeout
}

func stepFollower(r *raft, m pb.Message) error {
	return nil
}

func numOfPendingConf(ents []pb.Entry) int {
	n := 0
	for i := range ents {
		if ents[i].Type == pb.EntryConfChange {
			n++
		}
	}
	return n
}
