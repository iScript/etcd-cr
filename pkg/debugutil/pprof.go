package debugutil

import (
	"net/http"
	"net/http/pprof"
	"runtime"
)

// pprof 性能分析

const HTTPPrefixPProf = "/debug/pprof"

// PProfHandlers returns a map of pprof handlers keyed by the HTTP path.
// 返回一个http path对应pprof handler的map
func PProfHandlers() map[string]http.Handler {
	// 设置采样频率
	if runtime.SetMutexProfileFraction(-1) == 0 {

		runtime.SetMutexProfileFraction(5)
	}

	m := make(map[string]http.Handler)

	m[HTTPPrefixPProf+"/"] = http.HandlerFunc(pprof.Index)
	m[HTTPPrefixPProf+"/profile"] = http.HandlerFunc(pprof.Profile)
	m[HTTPPrefixPProf+"/symbol"] = http.HandlerFunc(pprof.Symbol)
	m[HTTPPrefixPProf+"/cmdline"] = http.HandlerFunc(pprof.Cmdline)
	m[HTTPPrefixPProf+"/trace "] = http.HandlerFunc(pprof.Trace)
	m[HTTPPrefixPProf+"/heap"] = pprof.Handler("heap")
	m[HTTPPrefixPProf+"/goroutine"] = pprof.Handler("goroutine")
	m[HTTPPrefixPProf+"/threadcreate"] = pprof.Handler("threadcreate")
	m[HTTPPrefixPProf+"/block"] = pprof.Handler("block")
	m[HTTPPrefixPProf+"/mutex"] = pprof.Handler("mutex")

	return m
}
