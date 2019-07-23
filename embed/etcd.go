package embed

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"sync"
	"time"

	"github.com/coreos/pkg/capnslog"
	"github.com/iScript/etcd-cr/etcdserver"
	"github.com/iScript/etcd-cr/version"
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
	// // a map of contexts for the servers that serves client requests.
	//sctxs map[string]*serveCtx
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

	//serving := false

	e = &Etcd{cfg: *inCfg, stopc: make(chan struct{})} // 空struct内存友好
	cfg := &e.cfg

	defer func() {
		if e == nil || err == nil {
			return
		}
	}()

	if e.cfg.logger != nil {
		e.cfg.logger.Info(
			"configuring peer listeners",
			zap.Strings("listen-peer-urls", e.cfg.getLPURLs()),
		)
	}

	// if e.Peers, err = configurePeerListeners(cfg); err != nil {
	// 	return e, err
	// }

	if e.cfg.logger != nil {
		e.cfg.logger.Info(
			"configuring client listeners",
			zap.Strings("listen-client-urls", e.cfg.getLCURLs()),
		)
	}
	// if e.sctxs, err = configureClientListeners(cfg); err != nil {
	// 	return e, err
	// }

	memberInitialized := true
	if !isMemberInitialized(cfg) {
		memberInitialized = false
		// urlsmap, token, err = cfg.PeerURLsMapAndToken("etcd")
		// if err != nil {
		// 	return e, fmt.Errorf("error setting up initial cluster: %v", err)
		// }
	}

	srvcfg := etcdserver.ServerConfig{
		Name:                   cfg.Name,
		ClientURLs:             cfg.ACUrls,
		PeerURLs:               cfg.APUrls,
		DataDir:                cfg.Dir,
		DedicatedWALDir:        cfg.WalDir,
		SnapshotCount:          cfg.SnapshotCount,
		SnapshotCatchUpEntries: cfg.SnapshotCatchUpEntries,
		MaxSnapFiles:           cfg.MaxSnapFiles,
		MaxWALFiles:            cfg.MaxWalFiles,
		//InitialPeerURLsMap:         urlsmap,
		//InitialClusterToken:        token,
		DiscoveryURL:   cfg.Durl,
		DiscoveryProxy: cfg.Dproxy,
		NewCluster:     cfg.IsNewCluster(),
		//PeerTLSInfo:                cfg.PeerTLSInfo,
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

	return e, nil
}

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

// func configurePeerListeners(cfg *Config) (peers []*peerListener, err error) {

// }
