package runtime

import (
	"io/ioutil"
	"syscall"
)

func FDLimit() (uint64, error) {
	var rlimit syscall.Rlimit
	// syscall.RLIMIT_NOFILE 一个进程能打开的最大文件数，内核默认是1024
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlimit); err != nil {
		return 0, err
	}
	//soft limit,是指内核所能支持的资源上限
	return rlimit.Cur, nil
}

func FDUsage() (uint64, error) {
	// /proc/self/fd 当前进程打开文件的情况
	fds, err := ioutil.ReadDir("/proc/self/fd")
	if err != nil {
		return 0, err
	}
	return uint64(len(fds)), nil
}
