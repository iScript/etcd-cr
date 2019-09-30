package tracker

// leader节点中记录的follower的进展
// leader会维护所有follower的进展，发送entry基于这个进展
type Progress struct {
	// Match对应follower节点当前已经复制成功的entry记录的索引
	// Next 对应follower下一个待复制的entry记录的索引
	Match, Next uint64

	//节点复制状态
	State StateType

	// 当前正在发送的快照数据
	PendingSnapshot uint64

	// 从leader角度来看，改progress对应的follower是否存活
	RecentActive bool

	// ProbeSent is used while this follower is in StateProbe. When ProbeSent is
	// true, raft should pause sending replication message to this peer until
	// ProbeSent is reset. See ProbeAcked() and IsPaused().
	ProbeSent bool

	// 记录了已经发送出去但未收到响应的消息信息
	Inflights *Inflights

	// IsLearner is true if this progress is tracked for a learner.
	IsLearner bool
}

// ProgressMap is a map of *Progress.
type ProgressMap map[uint64]*Progress
