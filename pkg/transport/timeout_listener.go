package transport

import (
	"net"
	"time"
)

func NewTimeoutListener(addr string, scheme string, tlsinfo *TLSInfo, rdtimeoutd, wtimeoutd time.Duration) (net.Listener, error) {
	//fmt.Println(addr, scheme, "123123")

	ln, err := newListener(addr, scheme)
	if err != nil {
		return nil, err
	}

	// 返回net.Listener接口， rwTimeoutListener内嵌了net.Listener
	ln = &rwTimeoutListener{
		Listener:   ln,
		rdtimeoutd: rdtimeoutd,
		wtimeoutd:  wtimeoutd,
	}
	if ln, err = wrapTLS(scheme, tlsinfo, ln); err != nil {
		return nil, err
	}

	return ln, nil
}

//
type rwTimeoutListener struct {
	net.Listener
	wtimeoutd  time.Duration
	rdtimeoutd time.Duration
}

func (rwln *rwTimeoutListener) Accept() (net.Conn, error) {
	c, err := rwln.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return timeoutConn{
		Conn:       c,
		wtimeoutd:  rwln.wtimeoutd,
		rdtimeoutd: rwln.rdtimeoutd,
	}, nil
}
