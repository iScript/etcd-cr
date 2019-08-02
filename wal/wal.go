package wal

//WAL是write ahead log的缩写?
//顾名思义,也就是在执行真正的写操作之前先写一个日志

import (
	"errors"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/iScript/etcd-cr/pkg/fileutil"
	"go.uber.org/zap"
)

const (
	metadataType int64 = iota + 1
	entryType
	stateType
	crcType
	snapshotType

	// warnSyncDuration is the amount of time allotted to an fsync before
	// logging a warning
	warnSyncDuration = time.Second
)

var (
	// SegmentSizeBytes is the preallocated size of each wal segment file.
	// The actual size might be larger than this. In general, the default
	// value should be used, but this is defined as an exported variable
	// so that tests can set a different segment size.
	SegmentSizeBytes int64 = 64 * 1000 * 1000 // 64MB

	ErrMetadataConflict = errors.New("wal: conflicting metadata found")
	ErrFileNotFound     = errors.New("wal: file not found")
	ErrCRCMismatch      = errors.New("wal: crc mismatch")
	ErrSnapshotMismatch = errors.New("wal: snapshot mismatch")
	ErrSnapshotNotFound = errors.New("wal: snapshot not found")
	crcTable            = crc32.MakeTable(crc32.Castagnoli)
)

type WAL struct {
	lg *zap.Logger

	dir string // the living directory of the underlay files

	// dirFile is a fd for the wal directory for syncing on Rename
	dirFile *os.File

	metadata []byte // metadata recorded at the head of each WAL
	//state    raftpb.HardState // hardstate recorded at the head of WAL

	//start     walpb.Snapshot // snapshot to start reading
	//decoder   *decoder       // decoder to decode records
	readClose func() error // closer for decode reader

	mu      sync.Mutex
	enti    uint64   // index of the last entry saved to the wal
	encoder *encoder // encoder to encode records

	locks []*fileutil.LockedFile // the locked files the WAL holds (the name is increasing)
	// fp    *filePipeline
}

//创建一个wal用于日志
//metadata用于记录在每个wal文件的头部
func Create(lg *zap.Logger, dirpath string, metadata []byte) (*WAL, error) {
	// 判断目录是否存在
	if Exist(dirpath) {
		return nil, os.ErrExist
	}

	tmpdirpath := filepath.Clean(dirpath) + ".tmp"
	//如果存在tmp临时文件夹，删除
	if fileutil.Exist(tmpdirpath) {
		if err := os.RemoveAll(tmpdirpath); err != nil {
			return nil, err
		}
	}

	if err := fileutil.CreateDirAll(tmpdirpath); err != nil {
		if lg != nil {
			lg.Warn(
				"failed to create a temporary WAL directory",
				zap.String("tmp-dir-path", tmpdirpath),
				zap.String("dir-path", dirpath),
				zap.Error(err),
			)
		}
		return nil, err
	}

	p := filepath.Join(tmpdirpath, walName(0, 0)) // 文件名类似0000000000000000-0000000000000000.wal
	f, err := fileutil.LockFile(p, os.O_WRONLY|os.O_CREATE, fileutil.PrivateFileMode)
	if err != nil {
		if lg != nil {
			lg.Warn(
				"failed to flock an initial WAL file",
				zap.String("path", p),
				zap.Error(err),
			)
		}
		return nil, err
	}

	// Seek(offset, whence) 从结尾偏移0个字节， 返回新的offset
	if _, err = f.Seek(0, io.SeekEnd); err != nil {
		if lg != nil {
			lg.Warn(
				"failed to seek an initial WAL file",
				zap.String("path", p),
				zap.Error(err),
			)
		}
		return nil, err
	}

	// 预分配？
	// if err = fileutil.Preallocate(f.File, SegmentSizeBytes, true); err != nil {
	// 	if lg != nil {
	// 		lg.Warn(
	// 			"failed to preallocate an initial WAL file",
	// 			zap.String("path", p),
	// 			zap.Int64("segment-bytes", SegmentSizeBytes),
	// 			zap.Error(err),
	// 		)
	// 	}
	// 	return nil, err
	// }

	w := &WAL{
		lg:       lg,
		dir:      dirpath,
		metadata: metadata,
	}

	w.encoder, err = newFileEncoder(f.File, 0)
	if err != nil {
		return nil, err
	}
	w.locks = append(w.locks, f)

	// ...

	return w, nil
}

// func (w *WAL) saveCrc(prevCrc uint32) error {
// 	return w.encoder.encode(&walpb.Record{Type: crcType, Crc: prevCrc})
// }
