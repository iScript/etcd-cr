package fileutil

import (
	"os"
	"path/filepath"
	"sort"
)

// read dir operation
type ReadDirOp struct {
	ext string
}

type ReadDirOption func(*ReadDirOp)

func (op *ReadDirOp) applyOpts(opts []ReadDirOption) {
	for _, opt := range opts {
		opt(op)
	}
}

func WithExt(ext string) ReadDirOption {
	return func(op *ReadDirOp) { op.ext = ext }
}

//返回该目录中的文件名 , 参数为目录名及回调函数
func ReadDir(d string, opts ...ReadDirOption) ([]string, error) {

	op := &ReadDirOp{}
	op.applyOpts(opts)

	dir, err := os.Open(d)
	if err != nil {
		//第一次启动为空，直接返回
		return nil, err
	}
	defer dir.Close()

	names, err := dir.Readdirnames(-1) //-1 返回文件夹中所有name
	if err != nil {
		return nil, err
	}
	sort.Strings(names)

	if op.ext != "" {
		tss := make([]string, 0)
		for _, v := range names {
			if filepath.Ext(v) == op.ext {
				tss = append(tss, v)
			}
		}
		names = tss
	}
	return names, nil
}
