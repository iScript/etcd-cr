package transport

import (
	"net"
	"os"
)

type unixListener struct{ net.Listener }

func NewUnixListener(addr string) (net.Listener, error) {
	// 删除 unix socket？ 并且 有错误 错误不是不存在
	if err := os.Remove(addr); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	l, err := net.Listen("unix", addr)
	if err != nil {
		return nil, err
	}
	return &unixListener{l}, nil
}

func (ul *unixListener) Close() error {
	if err := os.Remove(ul.Addr().String()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return ul.Listener.Close()
}
