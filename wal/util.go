package wal

import (
	"fmt"

	"github.com/iScript/etcd-cr/pkg/fileutil"
)

func Exist(dir string) bool {
	names, err := fileutil.ReadDir(dir, fileutil.WithExt(".wal"))
	if err != nil {
		return false
	}
	return len(names) != 0
}

func walName(seq, index uint64) string {
	//以16进制输出
	return fmt.Sprintf("%016x-%016x.wal", seq, index)
}
