package tracker

type Config struct {
	//Voters quorum.JointConfig
	// AutoLeave is true if the configuration is joint and a transition to the
	// incoming configuration should be carried out automatically by Raft when
	// this is possible. If false, the configuration will be joint until the
	// application initiates the transition manually.
	AutoLeave bool
	// Learners is a set of IDs corresponding to the learners active in the
	// current configuration.
	//
	// Invariant: Learners and Voters does not intersect, i.e. if a peer is in
	// either half of the joint config, it can't be a learner; if it is a
	// learner it can't be in either half of the joint config. This invariant
	// simplifies the implementation since it allows peers to have clarity about
	// its current role without taking into account joint consensus.
	Learners map[uint64]struct{}
}

// ProgressTracker追踪当前的活动配置即其他信息
type ProgressTracker struct {
	Config

	//Progress ProgressMap

	Votes map[uint64]bool

	MaxInflight int
}

// 初始化ProgressTracker.
func MakeProgressTracker(maxInflight int) ProgressTracker {
	p := ProgressTracker{
		MaxInflight: maxInflight,
		Config: Config{
			// Voters: quorum.JointConfig{
			// 	quorum.MajorityConfig{},
			// 	nil, // only populated when used
			// },
			Learners: nil, // only populated when used
			//LearnersNext: nil, // only populated when used
		},
		Votes: map[uint64]bool{}, //{}初始化一个空map

		//Progress: map[uint64]*Progress{},
	}
	return p
}

// 重置votes
func (p *ProgressTracker) ResetVotes() {
	p.Votes = map[uint64]bool{}
}
