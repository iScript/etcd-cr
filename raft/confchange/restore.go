package confchange

import (
	pb "github.com/iScript/etcd-cr/raft/raftpb"
	"github.com/iScript/etcd-cr/raft/tracker"
)

func chain(chg Changer, ops ...func(Changer) (tracker.Config, tracker.ProgressMap, error)) (tracker.Config, tracker.ProgressMap, error) {
	for _, op := range ops {
		cfg, prs, err := op(chg)
		if err != nil {
			return tracker.Config{}, nil, err
		}
		chg.Tracker.Config = cfg
		chg.Tracker.Progress = prs
	}
	return chg.Tracker.Config, chg.Tracker.Progress, nil
}

func Restore(chg Changer, cs pb.ConfState) (tracker.Config, tracker.ProgressMap, error) {
	outgoing, incoming := toConfChangeSingle(cs)

	var ops []func(Changer) (tracker.Config, tracker.ProgressMap, error)

	if len(outgoing) == 0 {
		// No outgoing config, so just apply the incoming changes one by one.
		for _, cc := range incoming {
			cc := cc // loop-local copy
			ops = append(ops, func(chg Changer) (tracker.Config, tracker.ProgressMap, error) {
				return chg.Simple(cc)
			})
		}
	} else {
		// The ConfState describes a joint configuration.
		//
		// First, apply all of the changes of the outgoing config one by one, so
		// that it temporarily becomes the incoming active config. For example,
		// if the config is (1 2 3)&(2 3 4), this will establish (2 3 4)&().
		for _, cc := range outgoing {
			cc := cc // loop-local copy
			ops = append(ops, func(chg Changer) (tracker.Config, tracker.ProgressMap, error) {
				return chg.Simple(cc)
			})
		}
		// Now enter the joint state, which rotates the above additions into the
		// outgoing config, and adds the incoming config in. Continuing the
		// example above, we'd get (1 2 3)&(2 3 4), i.e. the incoming operations
		// would be removing 2,3,4 and then adding in 1,2,3 while transitioning
		// into a joint state.
		ops = append(ops, func(chg Changer) (tracker.Config, tracker.ProgressMap, error) {
			return chg.EnterJoint(cs.AutoLeave, incoming...)
		})
	}

	return chain(chg, ops...)
}
