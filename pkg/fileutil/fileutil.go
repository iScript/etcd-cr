package fileutil

import (
	"io/ioutil"
	"os"
	"path/filepath"
)

const (
	// 授权owner文件读写权限.
	PrivateFileMode = 0600
	// 授权owner 文件夹读写权限
	PrivateDirMode = 0700
)

// 返回文件夹是否可写. 如果可写返回nil.
// 判断可写的方式为创建一个文件，然后删除
func IsDirWriteable(dir string) error {
	f := filepath.Join(dir, ".touch")
	//fmt.Println(f)
	if err := ioutil.WriteFile(f, []byte(""), PrivateFileMode); err != nil {
		return err
	}
	return os.Remove(f)
}

// TouchDirAll 类似 os.MkdirAll. 创建一个 0700 权限的文件夹 如果该文件夹不存在的话。
func TouchDirAll(dir string) error {
	// 如果文件夹已存在，则不做任何事，返回nil
	err := os.MkdirAll(dir, PrivateDirMode)
	if err != nil {
		return err
	}
	return IsDirWriteable(dir)
}
