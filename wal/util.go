package wal

import (
	"github.com/iScript/etcd-cr/pkg/fileutil"
)

func Exist(dir string) bool {
	names, err := fileutil.ReadDir(dir, fileutil.WithExt(".wal"))
	if err != nil {
		return false
	}
	return len(names) != 0
}
