package rafthttp

import (
	"github.com/iScript/etcd-cr/pkg/types"
	"go.uber.org/zap"
)

type remote struct {
	lg       *zap.Logger
	localID  types.ID
	id       types.ID
	status   *peerStatus
	pipeline *pipeline
}

func startRemote(tr *Transport, urls types.URLs, id types.ID) *remote {
	return nil
	// picker := newURLPicker(urls)
	// status := newPeerStatus(tr.Logger, tr.ID, id)
	// pipeline := &pipeline{
	// 	peerID: id,
	// 	tr:     tr,
	// 	picker: picker,
	// 	status: status,
	// 	raft:   tr.Raft,
	// 	errorc: tr.ErrorC,
	// }
	// pipeline.start()

	// return &remote{
	// 	lg:       tr.Logger,
	// 	localID:  tr.ID,
	// 	id:       id,
	// 	status:   status,
	// 	pipeline: pipeline,
	// }
}
