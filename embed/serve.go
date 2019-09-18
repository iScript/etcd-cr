package embed

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/iScript/etcd-cr/etcdserver"
	"github.com/iScript/etcd-cr/pkg/debugutil"
	"github.com/iScript/etcd-cr/pkg/transport"
	"github.com/soheilhy/cmux"
	"go.uber.org/zap"
	"golang.org/x/net/trace"
	"google.golang.org/grpc"
)

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

	//logger := defaultLog.New(ioutil.Discard, "etcdhttp", 0)

	<-s.ReadyNotify() // 返回readych通道,这里会阻塞，等待readych通知 , 发通知地点为 server.go publish 结束
	fmt.Println("2222")
	if sctx.lg == nil {
		fmt.Println("ready to serve client requests")
	}

	m := cmux.New(sctx.l)
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
