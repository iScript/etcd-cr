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
	"github.com/iScript/etcd-cr/pkg/types"
	"github.com/iScript/etcd-cr/version"
	"github.com/soheilhy/cmux"
	"go.uber.org/zap"
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

	//之前验证过  ？
	if err = inCfg.Validate(); err != nil {
		return nil, err
	}

	serving := false

	e = &Etcd{cfg: *inCfg, stopc: make(chan struct{})} // 空struct内存友好
	cfg := &e.cfg

	defer func() {
		if e == nil || err == nil {
			return
		}
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

	if e.Peers, err = configurePeerListeners(cfg); err != nil {
		return e, err
	}

	if e.cfg.logger != nil {
		e.cfg.logger.Info(
			"configuring client listeners",
			zap.Strings("listen-client-urls", e.cfg.getLCURLs()),
		)
	}
	// if e.sctxs, err = configureClientListeners(cfg); err != nil {
	// 	return e, err
	// }

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

	//e.errc = make(chan error, len(e.Peers)+len(e.Clients)+2*len(e.sctxs))

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

// 返回error通道
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

func (e *Etcd) errHandler(err error) {
	select {
	case <-e.stopc:
		return
	default:
	}
	select {
	case <-e.stopc:
	case e.errc <- err:
	}
}

// 返回logger
func (e *Etcd) GetLogger() *zap.Logger {
	e.cfg.loggerMu.RLock()
	l := e.cfg.logger
	e.cfg.loggerMu.RUnlock()
	return l
}
