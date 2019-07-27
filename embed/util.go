package embed

import (
	"path/filepath"

	"go.etcd.io/etcd/wal"
)

// 成员是否初始化，判断依据是是否存在wal文件夹
func isMemberInitialized(cfg *Config) bool {
	waldir := cfg.WalDir
	if waldir == "" {
		waldir = filepath.Join(cfg.Dir, "member", "wal")
	}
	return wal.Exist(waldir)
}
