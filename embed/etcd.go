package embed

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	defaultLog "log"
	"net"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/coreos/pkg/capnslog"
	"github.com/iScript/etcd-cr/etcdserver"
	"github.com/iScript/etcd-cr/etcdserver/api/etcdhttp"
	"github.com/iScript/etcd-cr/etcdserver/api/rafthttp"
	"github.com/iScript/etcd-cr/etcdserver/api/v3rpc"
	"github.com/iScript/etcd-cr/pkg/debugutil"
	runtimeutil "github.com/iScript/etcd-cr/pkg/runtime"
	"github.com/iScript/etcd-cr/pkg/transport"
	"github.com/iScript/etcd-cr/pkg/types"
	"github.com/iScript/etcd-cr/version"
	"github.com/soheilhy/cmux"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

var plog = capnslog.NewPackageLogger("github.com/iScript/etcd-cr", "pkg/flags")

const (
	// internal fd usage includes disk usage and transport usage.
	// To read/write snapshot, snap pkg needs 1. In normal case, wal pkg needs
	// at most 2 to read/lock/write WALs. One case that it needs to 2 is to
	// read all logs after some snapshot index, which locates at the end of
	// the second last and the head of the last. For purging, it needs to read
	// directory, so it needs 1. For fd monitor, it needs 1.
	// For transport, rafthttp builds two long-polling connections and at most
	// four temporary connections with each member. There are at most 9 members
	// in a cluster, so it should reserve 96.
	// For the safety, we set the total reserved number to 150.
	reservedInternalFDNum = 150
)

// Etcd contains a running etcd server and its listeners.
type Etcd struct {
	Peers   []*peerListener
	Clients []net.Listener
	// serve context 对应map
	sctxs            map[string]*serveCtx
	metricsListeners []net.Listener

	Server *etcdserver.EtcdServer

	cfg   Config
	stopc chan struct{}
	errc  chan error

	closeOnce sync.Once
}

type peerListener struct {
	net.Listener //继承
	serve        func() error
	close        func(context.Context) error
}

//  embed.Config
func StartEtcd(inCfg *Config) (e *Etcd, err error) {

	//之前验证过
	if err = inCfg.Validate(); err != nil {
		return nil, err
	}

	serving := false

	e = &Etcd{cfg: *inCfg, stopc: make(chan struct{})} // 空struct内存友好
	cfg := &e.cfg

	defer func() {
		// 都没错误直接返回
		if e == nil || err == nil {
			return
		}
		// 有错误则关闭
		if !serving {
			//
			for _, sctx := range e.sctxs {
				close(sctx.serversC)
			}
		}
		e.Close()
		e = nil
	}()

	if e.cfg.logger != nil {
		e.cfg.logger.Info(
			"configuring peer listeners",
			zap.Strings("listen-peer-urls", e.cfg.getLPURLs()),
		)
	}

	// 配置peer listener
	if e.Peers, err = configurePeerListeners(cfg); err != nil {
		return e, err
	}

	if e.cfg.logger != nil {
		e.cfg.logger.Info(
			"configuring client listeners",
			zap.Strings("listen-client-urls", e.cfg.getLCURLs()),
		)
	}

	// 配置configure listener
	if e.sctxs, err = configureClientListeners(cfg); err != nil {
		return e, err
	}

	var (
		urlsmap types.URLsMap
		token   string
	)
	memberInitialized := true
	if !isMemberInitialized(cfg) {
		memberInitialized = false
		urlsmap, token, err = cfg.PeerURLsMapAndToken("etcd")
		if err != nil {
			return e, fmt.Errorf("error setting up initial cluster: %v", err)
		}
	}

	srvcfg := etcdserver.ServerConfig{
		Name:                       cfg.Name,
		ClientURLs:                 cfg.ACUrls,
		PeerURLs:                   cfg.APUrls,
		DataDir:                    cfg.Dir,
		DedicatedWALDir:            cfg.WalDir,
		SnapshotCount:              cfg.SnapshotCount,
		SnapshotCatchUpEntries:     cfg.SnapshotCatchUpEntries,
		MaxSnapFiles:               cfg.MaxSnapFiles,
		MaxWALFiles:                cfg.MaxWalFiles,
		InitialPeerURLsMap:         urlsmap,
		InitialClusterToken:        token,
		DiscoveryURL:               cfg.Durl,
		DiscoveryProxy:             cfg.Dproxy,
		NewCluster:                 cfg.IsNewCluster(),
		PeerTLSInfo:                cfg.PeerTLSInfo,
		TickMs:                     cfg.TickMs,
		ElectionTicks:              cfg.ElectionTicks(),
		InitialElectionTickAdvance: cfg.InitialElectionTickAdvance,
		//AutoCompactionRetention:    autoCompactionRetention,
		AutoCompactionMode: cfg.AutoCompactionMode,
		QuotaBackendBytes:  cfg.QuotaBackendBytes,
		BackendBatchLimit:  cfg.BackendBatchLimit,
		//BackendFreelistType:        backendFreelistType,
		BackendBatchInterval: cfg.BackendBatchInterval,
		MaxTxnOps:            cfg.MaxTxnOps,
		MaxRequestBytes:      cfg.MaxRequestBytes,
		StrictReconfigCheck:  cfg.StrictReconfigCheck,
		//ClientCertAuthEnabled: cfg.ClientTLSInfo.ClientCertAuth,
		AuthToken:             cfg.AuthToken,
		BcryptCost:            cfg.BcryptCost,
		CORS:                  cfg.CORS,
		HostWhitelist:         cfg.HostWhitelist,
		InitialCorruptCheck:   cfg.ExperimentalInitialCorruptCheck,
		CorruptCheckTime:      cfg.ExperimentalCorruptCheckTime,
		PreVote:               cfg.PreVote,
		Logger:                cfg.logger,
		LoggerConfig:          cfg.loggerConfig,
		LoggerCore:            cfg.loggerCore,
		LoggerWriteSyncer:     cfg.loggerWriteSyncer,
		Debug:                 cfg.Debug,
		ForceNewCluster:       cfg.ForceNewCluster,
		EnableGRPCGateway:     cfg.EnableGRPCGateway,
		EnableLeaseCheckpoint: cfg.ExperimentalEnableLeaseCheckpoint,
	}
	print(e.cfg.logger, *cfg, srvcfg, memberInitialized)

	if e.Server, err = etcdserver.NewServer(srvcfg); err != nil {
		return e, err
	}

	// 创建带缓冲的通道
	// 不带缓冲的通道是阻塞的，往通道发送数据后，这个数据如果没有被取，就是不能继续向通道里面发送数据。
	e.errc = make(chan error, len(e.Peers)+len(e.Clients)+2*len(e.sctxs))

	//第一次初始化为false
	if memberInitialized {
		// if err = e.Server.CheckInitialHashKV(); err != nil {
		// 	// set "EtcdServer" to nil, so that it does not block on "EtcdServer.Close()"
		// 	// (nothing to close since rafthttp transports have not been started)
		// 	e.Server = nil
		// 	return e, err
		// }
	}

	e.Server.Start()

	if err = e.servePeers(); err != nil {
		return e, err
	}

	if err = e.serveClients(); err != nil {
		return e, err
	}

	// if err = e.serveMetrics(); err != nil {
	// 	return e, err
	// }

	if e.cfg.logger != nil {
		e.cfg.logger.Info(
			"now serving peer/client/metrics",
			zap.String("local-member-id", e.Server.ID().String()),
			zap.Strings("initial-advertise-peer-urls", e.cfg.getAPURLs()),
			zap.Strings("listen-peer-urls", e.cfg.getLPURLs()),
			zap.Strings("advertise-client-urls", e.cfg.getACURLs()),
			zap.Strings("listen-client-urls", e.cfg.getLCURLs()),
			zap.Strings("listen-metrics-urls", e.cfg.getMetricsURLs()),
		)
	}

	serving = true

	return e, nil
}

// 输出etcd server相关信息
func print(lg *zap.Logger, ec Config, sc etcdserver.ServerConfig, memberInitialized bool) {
	lg.Info(
		"starting an etcd server",
		zap.String("etcd-version", version.Version),
		zap.String("git-sha", version.GitSHA),
		zap.String("go-version", runtime.Version()),
		zap.String("go-os", runtime.GOOS),
		zap.String("go-arch", runtime.GOARCH),
		zap.Int("max-cpu-set", runtime.GOMAXPROCS(0)),
		zap.Int("max-cpu-available", runtime.NumCPU()),
		zap.Bool("member-initialized", memberInitialized),
		zap.String("name", sc.Name),
		zap.String("data-dir", sc.DataDir),
		zap.String("wal-dir", ec.WalDir),
		zap.String("wal-dir-dedicated", sc.DedicatedWALDir),
		zap.String("member-dir", sc.MemberDir()),
		zap.Bool("force-new-cluster", sc.ForceNewCluster),
		zap.String("heartbeat-interval", fmt.Sprintf("%v", time.Duration(sc.TickMs)*time.Millisecond)),
		zap.String("election-timeout", fmt.Sprintf("%v", time.Duration(sc.ElectionTicks*int(sc.TickMs))*time.Millisecond)),
		zap.Bool("initial-election-tick-advance", sc.InitialElectionTickAdvance),
		zap.Uint64("snapshot-count", sc.SnapshotCount),
		zap.Uint64("snapshot-catchup-entries", sc.SnapshotCatchUpEntries),
		zap.Strings("initial-advertise-peer-urls", ec.getAPURLs()),
		zap.Strings("listen-peer-urls", ec.getLPURLs()),
		zap.Strings("advertise-client-urls", ec.getACURLs()),
		zap.Strings("listen-client-urls", ec.getLCURLs()),
		zap.Strings("listen-metrics-urls", ec.getMetricsURLs()),
	)
}

// 返回当前配置
func (e *Etcd) Config() Config {
	return e.cfg
}

func (e *Etcd) Close() {}

// 停止server
func stopServers(ctx context.Context, ss *servers) {
	shutdownNow := func() {
		// 首先关闭 http.Server
		ss.http.Shutdown(ctx)
		// 然后关闭grpc server
		ss.grpc.Stop()
	}

	if ss.secure {
		shutdownNow()
		return
	}

	ch := make(chan struct{})
	go func() {
		defer close(ch)
		ss.grpc.GracefulStop()
	}()

	// wait until all pending RPCs are finished
	select {
	case <-ch:
	case <-ctx.Done():
		// took too long, manually close open transports
		// e.g. watch streams
		shutdownNow()

		// concurrent GracefulStop should be interrupted
		<-ch
	}
}

// 返回error通道 ， 返回后的通道只能出数据
func (e *Etcd) Err() <-chan error { return e.errc }

func configurePeerListeners(cfg *Config) (peers []*peerListener, err error) {
	if err = updateCipherSuites(&cfg.PeerTLSInfo, cfg.CipherSuites); err != nil {
		return nil, err
	}

	if err = cfg.PeerSelfCert(); err != nil {
		if cfg.logger != nil {
			cfg.logger.Fatal("failed to get peer self-signed certs", zap.Error(err))
		}
	}

	if !cfg.PeerTLSInfo.Empty() {
		if cfg.logger != nil {
			cfg.logger.Info(
				"starting with peer TLS",
				zap.String("tls-info", fmt.Sprintf("%+v", cfg.PeerTLSInfo)),
				zap.Strings("cipher-suites", cfg.CipherSuites),
			)
		}
	}

	// LPUrls为url.URL类型 默认为DefaultListenPeerURLs="http://localhost:2380"
	// 创建切片
	peers = make([]*peerListener, len(cfg.LPUrls))
	defer func() {
		if err == nil {
			return
		}
		for i := range peers {
			if peers[i] != nil && peers[i].close != nil {
				if cfg.logger != nil {
					cfg.logger.Warn(
						"closing peer listener",
						zap.String("address", cfg.LPUrls[i].String()),
						zap.Error(err),
					)
				}
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				peers[i].close(ctx)
				cancel()
			}
		}
	}()

	for i, u := range cfg.LPUrls {
		// 如果是http
		if u.Scheme == "http" {
			//忽略key和cert
			if !cfg.PeerTLSInfo.Empty() {
				if cfg.logger != nil {
					cfg.logger.Warn("scheme is HTTP while key and cert files are present; ignoring key and cert files", zap.String("peer-url", u.String()))
				}
			}
			// 忽略cert auth
			if cfg.PeerTLSInfo.ClientCertAuth {
				if cfg.logger != nil {
					cfg.logger.Warn("scheme is HTTP while --peer-client-cert-auth is enabled; ignoring client cert auth for this URL", zap.String("peer-url", u.String()))
				}
			}
		}

		peers[i] = &peerListener{close: func(context.Context) error { return nil }}
		peers[i].Listener, err = rafthttp.NewListener(u, &cfg.PeerTLSInfo) // 返回 net.Listen("xxx", xxx)
		if err != nil {
			return nil, err
		}
		peers[i].close = func(context.Context) error {
			return peers[i].Listener.Close()
		}
	}

	return peers, nil
}

func (e *Etcd) servePeers() (err error) {
	ph := etcdhttp.NewPeerHandler(e.GetLogger(), e.Server) // 返回http.handle
	fmt.Println(ph)

	var peerTLScfg *tls.Config
	if !e.cfg.PeerTLSInfo.Empty() { //PeerTLSInfo => pkg/transport.TLSInfo
		// if peerTLScfg, err = e.cfg.PeerTLSInfo.ServerConfig(); err != nil {
		// 	return err
		// }
	}

	//循环Peers  （peerListener struct）
	for _, p := range e.Peers {

		u := p.Listener.Addr().String()          // url  127.0.0.1:2380
		gs := v3rpc.Server(e.Server, peerTLScfg) // 返回grpc.server

		m := cmux.New(p.Listener) //cmux库用于在同一的端口监听不同的服务 如grpc http HTTP2
		go gs.Serve(m.Match(cmux.HTTP2()))

		srv := &http.Server{
			Handler:     grpcHandlerFunc(gs, ph), // 传入grpc.server, http.handle
			ReadTimeout: 5 * time.Minute,
			ErrorLog:    defaultLog.New(ioutil.Discard, "", 0), // do not log user error
		}

		go srv.Serve(m.Match(cmux.Any()))
		p.serve = func() error { return m.Serve() }
		p.close = func(ctx context.Context) error {

			if e.cfg.logger != nil {
				e.cfg.logger.Info(
					"stopping serving peer traffic",
					zap.String("address", u),
				)
			}
			stopServers(ctx, &servers{secure: peerTLScfg != nil, grpc: gs, http: srv})
			if e.cfg.logger != nil {
				e.cfg.logger.Info(
					"stopped serving peer traffic",
					zap.String("address", u),
				)
			}
			return nil
		}

	}

	for _, pl := range e.Peers {
		go func(l *peerListener) {
			u := l.Addr().String()
			if e.cfg.logger != nil {
				e.cfg.logger.Info(
					"serving peer traffic",
					zap.String("address", u),
				)
			}
			e.errHandler(l.serve())
		}(pl)
	}

	return nil
}

func configureClientListeners(cfg *Config) (sctxs map[string]*serveCtx, err error) {

	// tsl 相关
	if err = updateCipherSuites(&cfg.ClientTLSInfo, cfg.CipherSuites); err != nil {
		return nil, err
	}

	if err = cfg.ClientSelfCert(); err != nil {
		if cfg.logger != nil {
			cfg.logger.Fatal("failed to get client self-signed certs", zap.Error(err))
		}
	}

	if cfg.EnablePprof {
		if cfg.logger != nil {
			cfg.logger.Info("pprof is enabled", zap.String("path", debugutil.HTTPPrefixPProf))
		}
	}

	sctxs = make(map[string]*serveCtx)

	// LCUrls为url.URL类型 默认为DefaultListenClientURLs = "http://localhost:2379"
	for _, u := range cfg.LCUrls {
		sctx := newServeCtx(cfg.logger)

		if u.Scheme == "http" || u.Scheme == "unix" {
			// 如果是http,但是配置了证书 , 则忽略
			if !cfg.ClientTLSInfo.Empty() {
				if cfg.logger != nil {
					cfg.logger.Warn("scheme is HTTP while key and cert files are present; ignoring key and cert files", zap.String("client-url", u.String()))
				}
			}
			// 如果是http,开启了ClientCertAuth，则忽略
			if cfg.ClientTLSInfo.ClientCertAuth {
				if cfg.logger != nil {
					cfg.logger.Warn("scheme is HTTP while --client-cert-auth is enabled; ignoring client cert auth for this URL", zap.String("client-url", u.String()))
				}
			}
		}
		// 如果是https ， 但是没配置证书 ， 则错误
		if (u.Scheme == "https" || u.Scheme == "unixs") && cfg.ClientTLSInfo.Empty() {
			return nil, fmt.Errorf("TLS key/cert (--cert-file, --key-file) must be provided for client url %s with HTTPS scheme", u.String())
		}

		network := "tcp"
		addr := u.Host
		if u.Scheme == "unix" || u.Scheme == "unixs" {
			network = "unix"
			addr = u.Host + u.Path
		}

		sctx.network = network
		sctx.secure = u.Scheme == "https" || u.Scheme == "unixs"
		sctx.insecure = !sctx.secure

		// 如果addr有值，更新secure后直接下一个循环
		if oldctx := sctxs[addr]; oldctx != nil {
			oldctx.secure = oldctx.secure || sctx.secure
			oldctx.insecure = oldctx.insecure || sctx.insecure
			continue
		}

		if sctx.l, err = net.Listen(network, addr); err != nil {
			return nil, err
		}

		sctx.addr = addr

		// 检测进程打开文件数限制
		if fdLimit, fderr := runtimeutil.FDLimit(); fderr == nil {
			// 如果小于
			if fdLimit <= reservedInternalFDNum {
				if cfg.logger != nil {
					cfg.logger.Fatal(
						"file descriptor limit of etcd process is too low; please set higher",
						zap.Uint64("limit", fdLimit),
						zap.Int("recommended-limit", reservedInternalFDNum),
					)
				}
			}
			sctx.l = transport.LimitListener(sctx.l, int(fdLimit-reservedInternalFDNum))
		}

		// tcp or unix
		if network == "tcp" {
			if sctx.l, err = transport.NewKeepAliveListener(sctx.l, network, nil); err != nil {
				return nil, err
			}
		}

		// 函数结束时判断有没错误
		defer func() {
			if err == nil {
				return
			}
			sctx.l.Close()
			if cfg.logger != nil {
				cfg.logger.Warn(
					"closing peer listener",
					zap.String("address", u.Host),
					zap.Error(err),
				)
			}
		}()

		//默认map空？
		for k := range cfg.UserHandlers {
			sctx.userHandlers[k] = cfg.UserHandlers[k]
		}
		//
		sctx.serviceRegister = cfg.ServiceRegister

		// 是否启用pprof
		if cfg.EnablePprof || cfg.Debug {
			sctx.registerPprof()
		}
		// 是否debug 模式
		if cfg.Debug {
			sctx.registerTrace()
		}
		sctxs[addr] = sctx

	}

	return sctxs, nil
}

func (e *Etcd) serveClients() (err error) {
	if !e.cfg.ClientTLSInfo.Empty() {
		if e.cfg.logger != nil {
			e.cfg.logger.Info(
				"starting with client TLS",
				zap.String("tls-info", fmt.Sprintf("%+v", e.cfg.ClientTLSInfo)),
				zap.Strings("cipher-suites", e.cfg.CipherSuites),
			)
		}
	}

	var h http.Handler

	// 可以响应client的请求
	//if e.Config().EnableV2 {
	if false {
		fmt.Println("enablev2")

	} else { // 只提供基本的查询功能，不能响应client的请求
		fmt.Println("enablev2 false")
		mux := http.NewServeMux()           // 相当于一个路由管理器
		etcdhttp.HandleBasic(mux, e.Server) // 设置基本路由，如/versioin
		h = mux                             // ServeMux同时也实现了ServeHTTP方法，因此代码中的mux也是一种handler
	}

	// grpc 相关配置
	gopts := []grpc.ServerOption{}
	if e.cfg.GRPCKeepAliveMinTime > time.Duration(0) {
		gopts = append(gopts, grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             e.cfg.GRPCKeepAliveMinTime,
			PermitWithoutStream: false,
		}))
	}
	if e.cfg.GRPCKeepAliveInterval > time.Duration(0) &&
		e.cfg.GRPCKeepAliveTimeout > time.Duration(0) {
		gopts = append(gopts, grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    e.cfg.GRPCKeepAliveInterval,
			Timeout: e.cfg.GRPCKeepAliveTimeout,
		}))
	}

	for _, sctx := range e.sctxs {
		go func(s *serveCtx) {
			e.errHandler(s.serve(e.Server, &e.cfg.ClientTLSInfo, h, e.errHandler, gopts...))
		}(sctx)
	}

	fmt.Println("serveclient over")
	return nil
}

// 错误处理
func (e *Etcd) errHandler(err error) {
	fmt.Println(err, 112233333)
	select {
	case <-e.stopc:
		return
	default:
	}
	select {
	case <-e.stopc:
	case e.errc <- err: //如果发生错误，将错误传入通道
	}
}

// 返回logger
func (e *Etcd) GetLogger() *zap.Logger {
	e.cfg.loggerMu.RLock()
	l := e.cfg.logger
	e.cfg.loggerMu.RUnlock()
	return l
}
