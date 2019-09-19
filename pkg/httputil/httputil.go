package httputil

import (
	"io"
	"io/ioutil"
	"net"
	"net/http"
)

// GracefulClose drains http.Response.Body until it hits EOF
// and closes it. This prevents TCP/TLS connections from closing,
// therefore available for reuse.
// Borrowed from golang/net/context/ctxhttp/cancelreq.go.
// 优雅关闭
func GracefulClose(resp *http.Response) {
	io.Copy(ioutil.Discard, resp.Body)
	resp.Body.Close()
}

// 获取hostname
func GetHostname(req *http.Request) string {
	if req == nil {
		return ""
	}
	h, _, err := net.SplitHostPort(req.Host)
	if err != nil {
		return req.Host
	}
	return h
}
