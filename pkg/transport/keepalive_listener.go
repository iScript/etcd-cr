package transport

import (
	"crypto/tls"
	"fmt"
	"net"
	"time"
)

type keepAliveConn interface {
	SetKeepAlive(bool) error
	SetKeepAlivePeriod(d time.Duration) error
}

func NewKeepAliveListener(l net.Listener, scheme string, tlscfg *tls.Config) (net.Listener, error) {
	if scheme == "https" {
		if tlscfg == nil {
			return nil, fmt.Errorf("cannot listen on TLS for given listener: KeyFile and CertFile are not presented")
		}
		return newTLSKeepaliveListener(l, tlscfg), nil
	}

	return &keepaliveListener{
		Listener: l,
	}, nil
}

type keepaliveListener struct{ net.Listener }

func (kln *keepaliveListener) Accept() (net.Conn, error) {
	c, err := kln.Listener.Accept()
	if err != nil {
		return nil, err
	}
	kac := c.(keepAliveConn)
	// detection time: tcp_keepalive_time + tcp_keepalive_probes + tcp_keepalive_intvl
	// default on linux:  30 + 8 * 30
	// default on osx:    30 + 8 * 75
	kac.SetKeepAlive(true)
	kac.SetKeepAlivePeriod(30 * time.Second)
	return c, nil
}

// A tlsKeepaliveListener implements a network listener (net.Listener) for TLS connections.
type tlsKeepaliveListener struct {
	net.Listener
	config *tls.Config
}

func newTLSKeepaliveListener(inner net.Listener, config *tls.Config) net.Listener {
	l := &tlsKeepaliveListener{}
	l.Listener = inner
	l.config = config
	return l
}
