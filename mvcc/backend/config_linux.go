// 条件编译，通过文件后缀？？

package backend

import (
	"syscall"

	bolt "go.etcd.io/bbolt"
)

// syscall.MAP_POPULATE on linux 2.6.23+ does sequential read-ahead
// which can speed up entire-database read with boltdb. We want to
// enable MAP_POPULATE for faster key-value store recovery in storage
// package. If your kernel version is lower than 2.6.23
// (https://github.com/torvalds/linux/releases/tag/v2.6.23), mmap might
// silently ignore this flag. Please update your kernel to prevent this.
var boltOpenOptions = &bolt.Options{
	MmapFlags:      syscall.MAP_POPULATE,
	NoFreelistSync: true,
}

func (bcfg *BackendConfig) mmapSize() int { return int(bcfg.MmapSize) }
