package membership

import (
	"errors"

	"github.com/iScript/etcd-cr/etcdserver/api/v2error"
)

var (
	ErrIDRemoved        = errors.New("membership: ID removed")
	ErrIDExists         = errors.New("membership: ID exists")
	ErrIDNotFound       = errors.New("membership: ID not found")
	ErrPeerURLexists    = errors.New("membership: peerURL exists")
	ErrMemberNotLearner = errors.New("membership: can only promote a learner member")
	ErrTooManyLearners  = errors.New("membership: too many learner members in cluster")
)

func isKeyNotFound(err error) bool {
	e, ok := err.(*v2error.Error)
	return ok && e.ErrorCode == v2error.EcodeKeyNotFound
}