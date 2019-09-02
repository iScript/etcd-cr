package etcdserver

import (
	"github.com/iScript/etcd-cr/etcdserver/api/snap"
	"github.com/iScript/etcd-cr/wal"
)

type Storage interface {
	// Save function saves ents and state to the underlying stable storage.
	// Save MUST block until st and ents are on stable storage.
	//Save(st raftpb.HardState, ents []raftpb.Entry) error
	// SaveSnap function saves snapshot to the underlying stable storage.
	//SaveSnap(snap raftpb.Snapshot) error
	// Close closes the Storage and performs finalization.
	//Close() error
}

type storage struct {
	*wal.WAL
	*snap.Snapshotter
}

func NewStorage(w *wal.WAL, s *snap.Snapshotter) Storage {
	return &storage{w, s}
}
