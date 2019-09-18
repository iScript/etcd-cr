package etcdhttp

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/iScript/etcd-cr/etcdserver"
	"github.com/iScript/etcd-cr/etcdserver/api"
	"github.com/iScript/etcd-cr/version"
)

const (
	configPath  = "/config"
	varsPath    = "/debug/vars"
	versionPath = "/version"
)

// HandleBasic 给mux添加handle
func HandleBasic(mux *http.ServeMux, server etcdserver.ServerPeer) {
	//mux.HandleFunc(varsPath, serveVars)

	// TODO: deprecate '/config/local/log' in v3.5
	//mux.HandleFunc(configPath+"/local/log", logHandleFunc)

	//HandleMetricsHealth(mux, server)

	mux.HandleFunc(versionPath, versionHandler(server.Cluster(), serveVersion))
}

func versionHandler(c api.Cluster, fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		v := c.Version()
		if v != nil {
			fn(w, r, v.String())
		} else {
			fn(w, r, "not_decided")
		}
	}
}

func serveVersion(w http.ResponseWriter, r *http.Request, clusterV string) {
	if !allowMethod(w, r, "GET") {
		return
	}
	vs := version.Versions{
		Server:  version.Version,
		Cluster: clusterV,
	}

	w.Header().Set("Content-Type", "application/json")
	b, err := json.Marshal(&vs)
	if err != nil {
		fmt.Println("cannot marshal versions to json (%v)", err)
	}
	w.Write(b)
}

// 是否允许某个http method
func allowMethod(w http.ResponseWriter, r *http.Request, m string) bool {
	if m == r.Method {
		return true
	}
	w.Header().Set("Allow", m)
	http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	return false
}
