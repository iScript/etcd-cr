package tracker

import (
	"fmt"
	"sort"
	"strings"

	"github.com/iScript/etcd-cr/raft/quorum"
	pb "github.com/iScript/etcd-cr/raft/raftpb"
)

type Config struct {
	Voters quorum.JointConfig
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

	LearnersNext map[uint64]struct{}
}

func (c Config) String() string {
	var buf strings.Builder
	fmt.Fprintf(&buf, "voters=%s", c.Voters)
	if c.Learners != nil {
		fmt.Fprintf(&buf, " learners=%s", quorum.MajorityConfig(c.Learners).String())
	}
	if c.LearnersNext != nil {
		fmt.Fprintf(&buf, " learners_next=%s", quorum.MajorityConfig(c.LearnersNext).String())
	}
	if c.AutoLeave {
		fmt.Fprintf(&buf, " autoleave")
	}
	return buf.String()
}

// Clone returns a copy of the Config that shares no memory with the original.
func (c *Config) Clone() Config {
	clone := func(m map[uint64]struct{}) map[uint64]struct{} {
		if m == nil {
			return nil
		}
		mm := make(map[uint64]struct{}, len(m))
		for k := range m {
			mm[k] = struct{}{}
		}
		return mm
	}
	return Config{
		Voters:       quorum.JointConfig{clone(c.Voters[0]), clone(c.Voters[1])},
		Learners:     clone(c.Learners),
		LearnersNext: clone(c.LearnersNext),
	}
}

// ProgressTracker追踪当前的活动配置及其他信息
type ProgressTracker struct {
	Config

	Progress ProgressMap

	Votes map[uint64]bool

	MaxInflight int
}

// 初始化ProgressTracker.
// 参数maxInflight 已经发送出去但未收到响应的消息个数上限
func MakeProgressTracker(maxInflight int) ProgressTracker {
	p := ProgressTracker{
		MaxInflight: maxInflight,
		Config: Config{
			Voters: quorum.JointConfig{
				quorum.MajorityConfig{},
				nil, // only populated when used
			},
			Learners:     nil, // only populated when used
			LearnersNext: nil, // only populated when used
		},
		Votes: map[uint64]bool{}, //{}初始化一个空map

		Progress: map[uint64]*Progress{},
	}
	return p
}

// ConfState returns a ConfState representing the active configuration.
func (p *ProgressTracker) ConfState() pb.ConfState {
	return pb.ConfState{
		Voters:         p.Voters[0].Slice(),
		VotersOutgoing: p.Voters[1].Slice(),
		Learners:       quorum.MajorityConfig(p.Learners).Slice(),
		LearnersNext:   quorum.MajorityConfig(p.LearnersNext).Slice(),
		AutoLeave:      p.AutoLeave,
	}
}

// type matchAckIndexer map[uint64]*Progress

// var _ quorum.AckedIndexer = matchAckIndexer(nil)

// // AckedIndex implements IndexLookuper.
// func (l matchAckIndexer) AckedIndex(id uint64) (quorum.Index, bool) {
// 	pr, ok := l[id]
// 	if !ok {
// 		return 0, false
// 	}
// 	return quorum.Index(pr.Match), true
// }

// // Committed returns the largest log index known to be committed based on what
// // the voting members of the group have acknowledged.
// func (p *ProgressTracker) Committed() uint64 {
// 	return uint64(p.Voters.CommittedIndex(matchAckIndexer(p.Progress)))
// }

// 为tracked progresses调用函数f
func (p *ProgressTracker) Visit(f func(id uint64, pr *Progress)) {
	n := len(p.Progress)

	// We need to sort the IDs and don't want to allocate since this is hot code.
	// The optimization here mirrors that in `(MajorityConfig).CommittedIndex`,
	// see there for details.
	var sl [7]uint64
	ids := sl[:]
	if len(sl) >= n {
		ids = sl[:n]
	} else {
		ids = make([]uint64, n)
	}
	for id := range p.Progress {
		n--
		ids[n] = id
	}
	//insertionSort(ids)
	for _, id := range ids {
		f(id, p.Progress[id])
	}
}

// VoterNodes returns a sorted slice of voters.
func (p *ProgressTracker) VoterNodes() []uint64 {
	m := p.Voters.IDs() //Voters继承Config
	fmt.Println(m, "track.go")
	nodes := make([]uint64, 0, len(m))
	for id := range m {
		nodes = append(nodes, id)
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i] < nodes[j] })
	return nodes
}

// LearnerNodes returns a sorted slice of learners.
func (p *ProgressTracker) LearnerNodes() []uint64 {
	if len(p.Learners) == 0 {
		return nil
	}
	nodes := make([]uint64, 0, len(p.Learners))
	for id := range p.Learners {
		nodes = append(nodes, id)
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i] < nodes[j] })
	return nodes
}

// 重置votes
func (p *ProgressTracker) ResetVotes() {
	p.Votes = map[uint64]bool{}
}
