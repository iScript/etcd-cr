package v3rpc

import (
	"github.com/iScript/etcd-cr/etcdserver"
	"github.com/iScript/etcd-cr/lease"
	"github.com/iScript/etcd-cr/mvcc"
	"github.com/iScript/etcd-cr/etcdserver/api/membership"
	"github.com/iScript/etcd-cr/etcdserver/api/v3rpc/rpctypes"
)

var toGRPCErrorMap = map[error]error{
	membership.ErrIDRemoved:               rpctypes.ErrGRPCMemberNotFound,
	membership.ErrIDNotFound:              rpctypes.ErrGRPCMemberNotFound,
	membership.ErrIDExists:                rpctypes.ErrGRPCMemberExist,
	membership.ErrPeerURLexists:           rpctypes.ErrGRPCPeerURLExist,
	membership.ErrMemberNotLearner:        rpctypes.ErrGRPCMemberNotLearner,
	membership.ErrTooManyLearners:         rpctypes.ErrGRPCTooManyLearners,
	etcdserver.ErrNotEnoughStartedMembers: rpctypes.ErrMemberNotEnoughStarted,
	etcdserver.ErrLearnerNotReady:         rpctypes.ErrGRPCLearnerNotReady,

	mvcc.ErrCompacted:             rpctypes.ErrGRPCCompacted,
	mvcc.ErrFutureRev:             rpctypes.ErrGRPCFutureRev,
	etcdserver.ErrRequestTooLarge: rpctypes.ErrGRPCRequestTooLarge,
	etcdserver.ErrNoSpace:         rpctypes.ErrGRPCNoSpace,
	etcdserver.ErrTooManyRequests: rpctypes.ErrTooManyRequests,

	etcdserver.ErrNoLeader:                   rpctypes.ErrGRPCNoLeader,
	etcdserver.ErrNotLeader:                  rpctypes.ErrGRPCNotLeader,
	etcdserver.ErrLeaderChanged:              rpctypes.ErrGRPCLeaderChanged,
	etcdserver.ErrStopped:                    rpctypes.ErrGRPCStopped,
	etcdserver.ErrTimeout:                    rpctypes.ErrGRPCTimeout,
	etcdserver.ErrTimeoutDueToLeaderFail:     rpctypes.ErrGRPCTimeoutDueToLeaderFail,
	etcdserver.ErrTimeoutDueToConnectionLost: rpctypes.ErrGRPCTimeoutDueToConnectionLost,
	etcdserver.ErrUnhealthy:                  rpctypes.ErrGRPCUnhealthy,
	etcdserver.ErrKeyNotFound:                rpctypes.ErrGRPCKeyNotFound,
	etcdserver.ErrCorrupt:                    rpctypes.ErrGRPCCorrupt,
	etcdserver.ErrBadLeaderTransferee:        rpctypes.ErrGRPCBadLeaderTransferee,

	lease.ErrLeaseNotFound:    rpctypes.ErrGRPCLeaseNotFound,
	lease.ErrLeaseExists:      rpctypes.ErrGRPCLeaseExist,
	lease.ErrLeaseTTLTooLarge: rpctypes.ErrGRPCLeaseTTLTooLarge,

	// auth.ErrRootUserNotExist:     rpctypes.ErrGRPCRootUserNotExist,
	// auth.ErrRootRoleNotExist:     rpctypes.ErrGRPCRootRoleNotExist,
	// auth.ErrUserAlreadyExist:     rpctypes.ErrGRPCUserAlreadyExist,
	// auth.ErrUserEmpty:            rpctypes.ErrGRPCUserEmpty,
	// auth.ErrUserNotFound:         rpctypes.ErrGRPCUserNotFound,
	// auth.ErrRoleAlreadyExist:     rpctypes.ErrGRPCRoleAlreadyExist,
	// auth.ErrRoleNotFound:         rpctypes.ErrGRPCRoleNotFound,
	// auth.ErrRoleEmpty:            rpctypes.ErrGRPCRoleEmpty,
	// auth.ErrAuthFailed:           rpctypes.ErrGRPCAuthFailed,
	// auth.ErrPermissionDenied:     rpctypes.ErrGRPCPermissionDenied,
	// auth.ErrRoleNotGranted:       rpctypes.ErrGRPCRoleNotGranted,
	// auth.ErrPermissionNotGranted: rpctypes.ErrGRPCPermissionNotGranted,
	// auth.ErrAuthNotEnabled:       rpctypes.ErrGRPCAuthNotEnabled,
	// auth.ErrInvalidAuthToken:     rpctypes.ErrGRPCInvalidAuthToken,
	// auth.ErrInvalidAuthMgmt:      rpctypes.ErrGRPCInvalidAuthMgmt,
}
