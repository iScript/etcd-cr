package rafthttp

import (
	"sync"
	"time"

	"github.com/iScript/etcd-cr/pkg/types"
	"go.uber.org/zap"
)

type failureType struct {
	source string
	action string
}

type peerStatus struct {
	lg     *zap.Logger
	local  types.ID
	id     types.ID
	mu     sync.Mutex // protect variables below
	active bool
	since  time.Time
}
