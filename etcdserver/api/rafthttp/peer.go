package rafthttp

import (
	"context"
	"sync"
	"time"

	"github.com/iScript/etcd-cr/pkg/types"
	"github.com/iScript/etcd-cr/raft/raftpb"
	"go.uber.org/zap"
)

const (
	ConnReadTimeout  = 5 * time.Second
	ConnWriteTimeout = 5 * time.Second

	recvBufSize = 4096
	// maxPendingProposals holds the proposals during one leader election process.
	// Generally one leader election takes at most 1 sec. It should have
	// 0-2 election conflicts, and each one takes 0.5 sec.
	// We assume the number of concurrent proposers is smaller than 4096.
	// One client blocks on its proposal for at least 1 sec, so 4096 is enough
	// to hold all proposals.
	maxPendingProposals = 4096

	streamAppV2 = "streamMsgAppV2"
	streamMsg   = "streamMsg"
	pipelineMsg = "pipeline"
	sendSnap    = "sendMsgSnap"
)

type Peer interface {
	// send sends the message to the remote peer. The function is non-blocking
	// and has no promise that the message will be received by the remote.
	// When it fails to send message out, it will report the status to underlying
	// raft.
	// send(m raftpb.Message)

	// // sendSnap sends the merged snapshot message to the remote peer. Its behavior
	// // is similar to send.
	// sendSnap(m snap.Message)

	// // update updates the urls of remote peer.
	// update(urls types.URLs)

	// // attachOutgoingConn attaches the outgoing connection to the peer for
	// // stream usage. After the call, the ownership of the outgoing
	// // connection hands over to the peer. The peer will close the connection
	// // when it is no longer used.
	// attachOutgoingConn(conn *outgoingConn)
	// // activeSince returns the time that the connection with the
	// // peer becomes active.
	// activeSince() time.Time
	// // stop performs any necessary finalization and terminates the peer
	// // elegantly.
	// stop()
}

type peer struct {
	lg *zap.Logger

	localID types.ID
	// id of the remote raft peer node
	id types.ID

	r Raft

	status *peerStatus

	// picker *urlPicker

	// msgAppV2Writer *streamWriter
	// writer         *streamWriter
	// pipeline       *pipeline
	// snapSender     *snapshotSender // snapshot sender to send v3 snapshot messages
	// msgAppV2Reader *streamReader
	// msgAppReader   *streamReader

	recvc chan raftpb.Message
	propc chan raftpb.Message

	mu     sync.Mutex
	paused bool

	cancel context.CancelFunc // cancel pending works in go routine created by peer.
	stopc  chan struct{}
}
