package snap

import (
	"errors"
	"hash/crc32"

	"go.uber.org/zap"
)

const snapSuffix = ".snap"

var (
	//plog = capnslog.NewPackageLogger("go.etcd.io/etcd/v3", "snap")

	ErrNoSnapshot    = errors.New("snap: no available snapshot")
	ErrEmptySnapshot = errors.New("snap: empty snapshot")
	ErrCRCMismatch   = errors.New("snap: crc mismatch")
	crcTable         = crc32.MakeTable(crc32.Castagnoli)

	// A map of valid files that can be present in the snap folder.
	validFiles = map[string]bool{
		"db": true,
	}
)

type Snapshotter struct {
	lg  *zap.Logger
	dir string
}

func New(lg *zap.Logger, dir string) *Snapshotter {
	return &Snapshotter{
		lg:  lg,
		dir: dir,
	}
}
