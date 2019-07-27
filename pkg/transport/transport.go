package transport

import (
	"net"
	"net/http"
	"strings"
	"time"
)

type unixTransport struct{ *http.Transport }

// 返回http transport
// 并注册2个协议
func NewTransport(info TLSInfo, dialtimeoutd time.Duration) (*http.Transport, error) {
	cfg, err := info.ClientConfig()
	if err != nil {
		return nil, err
	}

	t := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout: dialtimeoutd,
			// value taken from http.DefaultTransport
			KeepAlive: 30 * time.Second,
		}).Dial,
		// value taken from http.DefaultTransport
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     cfg,
	}

	dialer := (&net.Dialer{
		Timeout:   dialtimeoutd,
		KeepAlive: 30 * time.Second,
	})
	dial := func(net, addr string) (net.Conn, error) {
		return dialer.Dial("unix", addr)
	}

	tu := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		Dial:                dial,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     cfg,
	}

	ut := &unixTransport{tu}

	t.RegisterProtocol("unix", ut) //注册协议
	t.RegisterProtocol("unixs", ut)
	return t, err
}

// 该transport 实现 http.RoundTripper接口
func (urt *unixTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	url := *req.URL
	req.URL = &url
	req.URL.Scheme = strings.Replace(req.URL.Scheme, "unix", "http", 1)
	return urt.Transport.RoundTrip(req)
}
