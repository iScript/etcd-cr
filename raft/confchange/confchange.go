package confchange

import (
	"errors"
	"fmt"

	"github.com/iScript/etcd-cr/raft/quorum"
	pb "github.com/iScript/etcd-cr/raft/raftpb"
	"github.com/iScript/etcd-cr/raft/tracker"
)

// 方便配置更改
type Changer struct {
	Tracker   tracker.ProgressTracker
	LastIndex uint64
}

// 执行配置变更
//  参数：[]ConfChangeSingle{{
// 		Type:   c.Type,
// 		NodeID: c.NodeID,
// 	}},
// 如bootstrap传入 type=ConfChangeAddNode  NodeID=10276657743932975437
func (c Changer) Simple(ccs ...pb.ConfChangeSingle) (tracker.Config, tracker.ProgressMap, error) {
	cfg, prs, err := c.checkAndCopy() // Tracker.Config  ProgressMap{}
	if err != nil {
		return c.err(err)
	}
	if joint(cfg) {
		err := errors.New("can't apply simple config change in joint config")
		return c.err(err)
	}

	if err := c.apply(&cfg, prs, ccs...); err != nil {
		return c.err(err)
	}
	if n := symdiff(incoming(c.Tracker.Voters), incoming(cfg.Voters)); n > 1 {
		return tracker.Config{}, nil, errors.New("more than one voter changed without entering joint config")
	}

	return checkAndReturn(cfg, prs)
}

// apply a change to the configuration. By convention, changes to voters are
// always made to the incoming majority config Voters[0]. Voters[1] is either
// empty or preserves the outgoing majority configuration while in a joint state.
func (c Changer) apply(cfg *tracker.Config, prs tracker.ProgressMap, ccs ...pb.ConfChangeSingle) error {
	for _, cc := range ccs {
		if cc.NodeID == 0 {
			// etcd replaces the NodeID with zero if it decides (downstream of
			// raft) to not apply a change, so we have to have explicit code
			// here to ignore these.
			continue
		}
		switch cc.Type {
		case pb.ConfChangeAddNode:
			c.makeVoter(cfg, prs, cc.NodeID)
		// case pb.ConfChangeAddLearnerNode:
		// 	c.makeLearner(cfg, prs, cc.NodeID)
		// case pb.ConfChangeRemoveNode:
		// 	c.remove(cfg, prs, cc.NodeID)
		case pb.ConfChangeUpdateNode:
		default:
			return fmt.Errorf("unexpected conf type %d", cc.Type)
		}
	}
	if len(incoming(cfg.Voters)) == 0 {
		return errors.New("removed all voters")
	}
	return nil
}

// makeVoter adds or promotes the given ID to be a voter in the incoming
// majority config.
func (c Changer) makeVoter(cfg *tracker.Config, prs tracker.ProgressMap, id uint64) {

	pr := prs[id]

	if pr == nil {
		c.initProgress(cfg, prs, id, false /* isLearner */)
		return
	}

	pr.IsLearner = false
	nilAwareDelete(&cfg.Learners, id)
	nilAwareDelete(&cfg.LearnersNext, id)
	incoming(cfg.Voters)[id] = struct{}{}
	return
}

// initProgress initializes a new progress for the given node or learner.
func (c Changer) initProgress(cfg *tracker.Config, prs tracker.ProgressMap, id uint64, isLearner bool) {

	if !isLearner {
		incoming(cfg.Voters)[id] = struct{}{}
	} else {
		nilAwareAdd(&cfg.Learners, id)
	}
	//fmt.Println(incoming(cfg.Voters)[id], 19090)
	prs[id] = &tracker.Progress{
		// Initializing the Progress with the last index means that the follower
		// can be probed (with the last index).
		//
		// TODO(tbg): seems awfully optimistic. Using the first index would be
		// better. The general expectation here is that the follower has no log
		// at all (and will thus likely need a snapshot), though the app may
		// have applied a snapshot out of band before adding the replica (thus
		// making the first index the better choice).
		Next:      c.LastIndex,
		Match:     0,
		Inflights: tracker.NewInflights(c.Tracker.MaxInflight),
		IsLearner: isLearner,
		// When a node is first added, we should mark it as recently active.
		// Otherwise, CheckQuorum may cause us to step down if it is invoked
		// before the added node has had a chance to communicate with us.
		RecentActive: true,
	}
}

// checkInvariants makes sure that the config and progress are compatible with
// each other. This is used to check both what the Changer is initialized with,
// as well as what it returns.
func checkInvariants(cfg tracker.Config, prs tracker.ProgressMap) error {
	// NB: intentionally allow the empty config. In production we'll never see a
	// non-empty config (we prevent it from being created) but we will need to
	// be able to *create* an initial config, for example during bootstrap (or
	// during tests). Instead of having to hand-code this, we allow
	// transitioning from an empty config into any other legal and non-empty
	// config.
	for _, ids := range []map[uint64]struct{}{
		cfg.Voters.IDs(),
		cfg.Learners,
		cfg.LearnersNext,
	} {
		for id := range ids {
			if _, ok := prs[id]; !ok {
				return fmt.Errorf("no progress for %d", id)
			}
		}
	}

	// Any staged learner was staged because it could not be directly added due
	// to a conflicting voter in the outgoing config.
	for id := range cfg.LearnersNext {
		if _, ok := outgoing(cfg.Voters)[id]; !ok {
			return fmt.Errorf("%d is in LearnersNext, but not Voters[1]", id)
		}
		if prs[id].IsLearner {
			return fmt.Errorf("%d is in LearnersNext, but is already marked as learner", id)
		}
	}
	// Conversely Learners and Voters doesn't intersect at all.
	for id := range cfg.Learners {
		if _, ok := outgoing(cfg.Voters)[id]; ok {
			return fmt.Errorf("%d is in Learners and Voters[1]", id)
		}
		if _, ok := incoming(cfg.Voters)[id]; ok {
			return fmt.Errorf("%d is in Learners and Voters[0]", id)
		}
		if !prs[id].IsLearner {
			return fmt.Errorf("%d is in Learners, but is not marked as learner", id)
		}
	}

	if !joint(cfg) {
		// We enforce that empty maps are nil instead of zero.
		if outgoing(cfg.Voters) != nil {
			return fmt.Errorf("Voters[1] must be nil when not joint")
		}
		if cfg.LearnersNext != nil {
			return fmt.Errorf("LearnersNext must be nil when not joint")
		}
		if cfg.AutoLeave {
			return fmt.Errorf("AutoLeave must be false when not joint")
		}
	}

	return nil
}

// checkAndCopy copies the tracker's config and progress map (deeply enough for
// the purposes of the Changer) and returns those copies. It returns an error
// if checkInvariants does.
func (c Changer) checkAndCopy() (tracker.Config, tracker.ProgressMap, error) {
	cfg := c.Tracker.Config.Clone()
	prs := tracker.ProgressMap{}

	for id, pr := range c.Tracker.Progress {
		// A shallow copy is enough because we only mutate the Learner field.
		ppr := *pr
		prs[id] = &ppr
	}
	return checkAndReturn(cfg, prs)
}

// checkAndReturn calls checkInvariants on the input and returns either the
// resulting error or the input.
func checkAndReturn(cfg tracker.Config, prs tracker.ProgressMap) (tracker.Config, tracker.ProgressMap, error) {
	if err := checkInvariants(cfg, prs); err != nil {
		return tracker.Config{}, tracker.ProgressMap{}, err
	}
	return cfg, prs, nil
}

// err returns zero values and an error.
func (c Changer) err(err error) (tracker.Config, tracker.ProgressMap, error) {
	return tracker.Config{}, nil, err
}

// nilAwareAdd populates a map entry, creating the map if necessary.
func nilAwareAdd(m *map[uint64]struct{}, id uint64) {
	if *m == nil {
		*m = map[uint64]struct{}{}
	}
	(*m)[id] = struct{}{}
}

// nilAwareDelete deletes from a map, nil'ing the map itself if it is empty after.
func nilAwareDelete(m *map[uint64]struct{}, id uint64) {
	if *m == nil {
		return
	}
	delete(*m, id)
	if len(*m) == 0 {
		*m = nil
	}
}

// symdiff returns the count of the symmetric difference between the sets of
// uint64s, i.e. len( (l - r) \union (r - l)).
func symdiff(l, r map[uint64]struct{}) int {
	var n int
	pairs := [][2]quorum.MajorityConfig{
		{l, r}, // count elems in l but not in r
		{r, l}, // count elems in r but not in l
	}
	for _, p := range pairs {
		for id := range p[0] {
			if _, ok := p[1][id]; !ok {
				n++
			}
		}
	}
	return n
}

func joint(cfg tracker.Config) bool {
	return len(outgoing(cfg.Voters)) > 0
}

func incoming(voters quorum.JointConfig) quorum.MajorityConfig      { return voters[0] }
func outgoing(voters quorum.JointConfig) quorum.MajorityConfig      { return voters[1] }
func outgoingPtr(voters *quorum.JointConfig) *quorum.MajorityConfig { return &voters[1] }
