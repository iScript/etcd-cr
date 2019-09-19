package embed

import (
	"context"
	"fmt"
	"io/ioutil"
	defaultLog "log"
	"net"
	"net/http"
	"strings"

	gw "github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/iScript/etcd-cr/etcdserver"
	"github.com/iScript/etcd-cr/etcdserver/api/v3rpc"
	"github.com/iScript/etcd-cr/pkg/debugutil"
	"github.com/iScript/etcd-cr/pkg/transport"
	"github.com/soheilhy/cmux"
	"go.uber.org/zap"
	"golang.org/x/net/trace"
	"google.golang.org/grpc"
)

/**
http service通过cmux实现同一端口提供不同服务
一般步骤：
1. 创建一个cmux ，参数为listener  m := cmux.New(l)
2. 按顺序匹配链接 httpL := m.Match(cmux.HTTP1Fast())
3. 创建服务 httpS := &http.Server{}
4. 为服务使用muxed listener go httpS.Serve(httpL)
5. 启动 m.Serve()
*/

type serveCtx struct {
	lg       *zap.Logger
	l        net.Listener
	addr     string // 地址
	network  string // 网络 tcp
	secure   bool   // 是否安全的，如https
	insecure bool

	ctx    context.Context
	cancel context.CancelFunc

	userHandlers    map[string]http.Handler
	serviceRegister func(*grpc.Server)
	serversC        chan *servers
}

type servers struct {
	secure bool
	grpc   *grpc.Server
	http   *http.Server
}

func newServeCtx(lg *zap.Logger) *serveCtx {
	ctx, cancel := context.WithCancel(context.Background())
	return &serveCtx{
		lg:           lg,
		ctx:          ctx,
		cancel:       cancel,
		userHandlers: make(map[string]http.Handler),
		serversC:     make(chan *servers, 2), // in case sctx.insecure,sctx.secure true
	}
}

func (sctx *serveCtx) serve(
	s *etcdserver.EtcdServer,
	tlsinfo *transport.TLSInfo,
	handler http.Handler,
	errHandler func(error),
	gopts ...grpc.ServerOption) (err error) {

	logger := defaultLog.New(ioutil.Discard, "etcdhttp", 0)

	<-s.ReadyNotify() // 返回readych通道,这里会阻塞，等待readych通知 , 发通知地点为 server.go publish 结束

	if sctx.lg == nil {
		fmt.Println("ready to serve client requests")
	}

	m := cmux.New(sctx.l)

	// v3c := v3client.New(s)
	// servElection := v3election.NewElectionServer(v3c)
	// servLock := v3lock.NewLockServer(v3c)

	var gs *grpc.Server
	defer func() {
		if err != nil && gs != nil {
			gs.Stop()
		}
	}()

	if sctx.insecure {
		gs = v3rpc.Server(s, nil, gopts...)
		// v3electionpb.RegisterElectionServer(gs, servElection)
		// v3lockpb.RegisterLockServer(gs, servLock)
		if sctx.serviceRegister != nil {
			sctx.serviceRegister(gs)
		}

		var gwmux *gw.ServeMux
		if s.Cfg.EnableGRPCGateway {
			// gwmux, err = sctx.registerGateway([]grpc.DialOption{grpc.WithInsecure()})
			// if err != nil {
			// 	return err
			// }
		}

		httpmux := sctx.createMux(gwmux, handler)
		srvhttp := &http.Server{
			Handler:  createAccessController(sctx.lg, s, httpmux),
			ErrorLog: logger, // do not log user error
		}
		httpl := m.Match(cmux.HTTP1())

		go func() { errHandler(srvhttp.Serve(httpl)) }() //errorHandler来自e.errHandler

		sctx.serversC <- &servers{grpc: gs, http: srvhttp}
		if sctx.lg != nil {
			sctx.lg.Info(
				"serving client traffic insecurely; this is strongly discouraged!",
				zap.String("address", sctx.l.Addr().String()),
			)
		}
	}

	return m.Serve() // 开始监听 , 这里监听着，一直没返回，所以不会触发etcdmain里的通道
}

// 返回一个http.handle 用于grpc
//
func grpcHandlerFunc(grpcServer *grpc.Server, otherHandler http.Handler) http.Handler {
	if otherHandler == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			grpcServer.ServeHTTP(w, r)
		})
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor == 2 && strings.Contains(r.Header.Get("Content-Type"), "application/grpc") {
			grpcServer.ServeHTTP(w, r)
		} else {
			otherHandler.ServeHTTP(w, r)
		}
	})
}

func (sctx *serveCtx) createMux(gwmux *gw.ServeMux, handler http.Handler) *http.ServeMux {
	httpmux := http.NewServeMux()
	for path, h := range sctx.userHandlers {
		httpmux.Handle(path, h)
	}
	if gwmux != nil {
		// httpmux.Handle(
		// 	"/v3/",
		// 	wsproxy.WebsocketProxy(
		// 		gwmux,
		// 		wsproxy.WithRequestMutator(
		// 			// Default to the POST method for streams
		// 			func(_ *http.Request, outgoing *http.Request) *http.Request {
		// 				outgoing.Method = "POST"
		// 				return outgoing
		// 			},
		// 		),
		// 	),
		// )
	}
	if handler != nil {
		httpmux.Handle("/", handler)
	}
	return httpmux
}

// createAccessController 封装了 HTTP multiplexer:
// - 转换 gRPC 网关路径
// - 检查主机白名单
// http请求都先走这里过
// accessController需要实现http.Handler接口
func createAccessController(lg *zap.Logger, s *etcdserver.EtcdServer, mux *http.ServeMux) http.Handler {
	return &accessController{lg: lg, s: s, mux: mux}
}

type accessController struct {
	lg  *zap.Logger
	s   *etcdserver.EtcdServer
	mux *http.ServeMux
}

func (ac *accessController) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// 兼容处理
	if req != nil && req.URL != nil && strings.HasPrefix(req.URL.Path, "/v3beta/") {
		req.URL.Path = strings.Replace(req.URL.Path, "/v3beta/", "/v3/", 1)
	}

	// 非https
	if req.TLS == nil {
		//host := httputil.GetHostname(req) //获取主机名
		// 是否在白名单中
		// ...

		//} else if ac.s.Cfg.ClientCertAuthEnabled && ac.s.Cfg.EnableGRPCGateway && ac.s.AuthStore().IsAuthEnabled() && strings.HasPrefix(req.URL.Path, "/v3/") {
	} else if false {

	}

	// Write CORS header.
	// if ac.s.AccessController.OriginAllowed("*") {
	// 	addCORSHeader(rw, "*")
	// } else if origin := req.Header.Get("Origin"); ac.s.OriginAllowed(origin) {
	// 	addCORSHeader(rw, origin)
	// }

	if req.Method == "OPTIONS" {
		rw.WriteHeader(http.StatusOK)
		return
	}

	ac.mux.ServeHTTP(rw, req)
}

func (sctx *serveCtx) registerUserHandler(s string, h http.Handler) {
	if sctx.userHandlers[s] != nil {
		if sctx.lg != nil {
			sctx.lg.Warn("path is already registered by user handler", zap.String("path", s))
		}
		return
	}
	sctx.userHandlers[s] = h
}

func (sctx *serveCtx) registerPprof() {
	for p, h := range debugutil.PProfHandlers() {
		sctx.registerUserHandler(p, h)
	}
}

func (sctx *serveCtx) registerTrace() {
	reqf := func(w http.ResponseWriter, r *http.Request) { trace.Render(w, r, true) }
	sctx.registerUserHandler("/debug/requests", http.HandlerFunc(reqf))
	evf := func(w http.ResponseWriter, r *http.Request) { trace.RenderEvents(w, r, true) }
	sctx.registerUserHandler("/debug/events", http.HandlerFunc(evf))
}
