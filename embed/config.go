package embed

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/iScript/etcd-cr/etcdserver"
	"github.com/iScript/etcd-cr/pkg/netutil"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
)

const (
	ClusterStateFlagNew      = "new"
	ClusterStateFlagExisting = "existing"

	DefaultName                  = "default"
	DefaultMaxSnapshots          = 5
	DefaultMaxWALs               = 5
	DefaultMaxTxnOps             = uint(128)
	DefaultMaxRequestBytes       = 1.5 * 1024 * 1024
	DefaultGRPCKeepAliveMinTime  = 5 * time.Second
	DefaultGRPCKeepAliveInterval = 2 * time.Hour
	DefaultGRPCKeepAliveTimeout  = 20 * time.Second

	DefaultListenPeerURLs   = "http://localhost:2380" //peer通信，即etcd节点内部通信
	DefaultListenClientURLs = "http://localhost:2379" //监听客户端请求

	DefaultLogOutput = "default"
	JournalLogOutput = "systemd/journal"
	StdErrLogOutput  = "stderr"
	StdOutLogOutput  = "stdout"

	// DefaultStrictReconfigCheck is the default value for "--strict-reconfig-check" flag.
	// It's enabled by default.
	DefaultStrictReconfigCheck = true
	// DefaultEnableV2 is the default value for "--enable-v2" flag.
	// v2 is enabled by default.
	// TODO: disable v2 when deprecated.
	DefaultEnableV2 = true

	// maxElectionMs specifies the maximum value of election timeout.
	// More details are listed in ../Documentation/tuning.md#time-parameters.
	maxElectionMs = 50000
	// backend freelist map type
	freelistMapType = "map"
)

var (
	ErrConflictBootstrapFlags = fmt.Errorf("multiple discovery or bootstrap flags are set. " +
		"Choose one of \"initial-cluster\", \"discovery\" or \"discovery-srv\"")
	ErrUnsetAdvertiseClientURLsFlag = fmt.Errorf("--advertise-client-urls is required when --listen-client-urls is set explicitly")

	DefaultInitialAdvertisePeerURLs = "http://localhost:2380"
	DefaultAdvertiseClientURLs      = "http://localhost:2379"

	defaultHostname   string
	defaultHostStatus error
)

func init() {
	defaultHostname, defaultHostStatus = netutil.GetDefaultHost()
}

// etcd 服务器的配置参数.
type Config struct {
	Name   string `json:"name"`
	Dir    string `json:"data-dir"`
	WalDir string `json:"wal-dir"`

	SnapshotCount uint64 `json:"snapshot-count"`

	// SnapshotCatchUpEntries is the number of entries for a slow follower
	// to catch-up after compacting the raft storage entries.
	// We expect the follower has a millisecond level latency with the leader.
	// The max throughput is around 10K. Keep a 5K entries is enough for helping
	// follower to catch up.
	// WARNING: only change this for tests.
	// Always use "DefaultSnapshotCatchUpEntries"
	SnapshotCatchUpEntries uint64

	MaxSnapFiles uint `json:"max-snapshots"`
	MaxWalFiles  uint `json:"max-wals"`

	// TickMs is the number of milliseconds between heartbeat ticks.
	// TODO: decouple tickMs and heartbeat tick (current heartbeat tick = 1).
	// make ticks a cluster wide configuration.
	TickMs     uint `json:"heartbeat-interval"`
	ElectionMs uint `json:"election-timeout"`

	InitialElectionTickAdvance bool `json:"initial-election-tick-advance"`

	// BackendBatchInterval is the maximum time before commit the backend transaction.
	BackendBatchInterval time.Duration `json:"backend-batch-interval"`
	// BackendBatchLimit is the maximum operations before commit the backend transaction.
	BackendBatchLimit int   `json:"backend-batch-limit"`
	Test              int   `json:"test"`
	QuotaBackendBytes int64 `json:"quota-backend-bytes"`
	MaxTxnOps         uint  `json:"max-txn-ops"`
	MaxRequestBytes   uint  `json:"max-request-bytes"`

	LPUrls, LCUrls []url.URL
	APUrls, ACUrls []url.URL
	// ClientTLSInfo  transport.TLSInfo
	// ClientAutoTLS  bool
	// PeerTLSInfo    transport.TLSInfo
	// PeerAutoTLS    bool

	CipherSuites []string `json:"cipher-suites"`

	ClusterState          string `json:"initial-cluster-state"`
	DNSCluster            string `json:"discovery-srv"`
	DNSClusterServiceName string `json:"discovery-srv-name"`
	Dproxy                string `json:"discovery-proxy"`
	Durl                  string `json:"discovery"`
	InitialCluster        string `json:"initial-cluster"`
	InitialClusterToken   string `json:"initial-cluster-token"`
	StrictReconfigCheck   bool   `json:"strict-reconfig-check"`
	EnableV2              bool   `json:"enable-v2"`

	// AutoCompactionMode is either 'periodic' or 'revision'.
	AutoCompactionMode string `json:"auto-compaction-mode"`
	// AutoCompactionRetention is either duration string with time unit
	// (e.g. '5m' for 5-minute), or revision unit (e.g. '5000').
	// If no time unit is provided and compaction mode is 'periodic',
	// the unit defaults to hour. For example, '5' translates into 5-hour.
	AutoCompactionRetention string `json:"auto-compaction-retention"`

	// GRPCKeepAliveMinTime is the minimum interval that a client should
	// wait before pinging server. When client pings "too fast", server
	// sends goaway and closes the connection (errors: too_many_pings,
	// http2.ErrCodeEnhanceYourCalm). When too slow, nothing happens.
	// Server expects client pings only when there is any active streams
	// (PermitWithoutStream is set false).
	GRPCKeepAliveMinTime time.Duration `json:"grpc-keepalive-min-time"`
	// GRPCKeepAliveInterval is the frequency of server-to-client ping
	// to check if a connection is alive. Close a non-responsive connection
	// after an additional duration of Timeout. 0 to disable.
	GRPCKeepAliveInterval time.Duration `json:"grpc-keepalive-interval"`
	// GRPCKeepAliveTimeout is the additional duration of wait
	// before closing a non-responsive connection. 0 to disable.
	GRPCKeepAliveTimeout time.Duration `json:"grpc-keepalive-timeout"`

	// PreVote is true to enable Raft Pre-Vote.
	// If enabled, Raft runs an additional election phase
	// to check whether it would get enough votes to win
	// an election, thus minimizing disruptions.
	// TODO: enable by default in 3.5.
	PreVote bool `json:"pre-vote"`

	CORS          map[string]struct{}
	HostWhitelist map[string]struct{}

	// UserHandlers is for registering users handlers and only used for
	// embedding etcd into other applications.
	// The map key is the route path for the handler, and
	// you must ensure it can't be conflicted with etcd's.
	UserHandlers map[string]http.Handler `json:"-"`
	// ServiceRegister is for registering users' gRPC services. A simple usage example:
	//	cfg := embed.NewConfig()
	//	cfg.ServerRegister = func(s *grpc.Server) {
	//		pb.RegisterFooServer(s, &fooServer{})
	//		pb.RegisterBarServer(s, &barServer{})
	//	}
	//	embed.StartEtcd(cfg)
	ServiceRegister func(*grpc.Server) `json:"-"`

	AuthToken  string `json:"auth-token"`
	BcryptCost uint   `json:"bcrypt-cost"`

	ExperimentalInitialCorruptCheck bool          `json:"experimental-initial-corrupt-check"`
	ExperimentalCorruptCheckTime    time.Duration `json:"experimental-corrupt-check-time"`
	ExperimentalEnableV2V3          string        `json:"experimental-enable-v2v3"`
	// ExperimentalBackendFreelistType specifies the type of freelist that boltdb backend uses (array and map are supported types).
	ExperimentalBackendFreelistType string `json:"experimental-backend-bbolt-freelist-type"`
	// ExperimentalEnableLeaseCheckpoint enables primary lessor to persist lease remainingTTL to prevent indefinite auto-renewal of long lived leases.
	ExperimentalEnableLeaseCheckpoint bool `json:"experimental-enable-lease-checkpoint"`

	// ForceNewCluster starts a new cluster even if previously started; unsafe.
	ForceNewCluster bool `json:"force-new-cluster"`

	EnablePprof           bool   `json:"enable-pprof"`
	Metrics               string `json:"metrics"`
	ListenMetricsUrls     []url.URL
	ListenMetricsUrlsJSON string `json:"listen-metrics-urls"`

	// Logger 可选 "zap", "capnslog".
	// capnslog在v3.5版本中已被废弃.
	Logger string `json:"logger"`

	// DeprecatedLogOutput is to be deprecated in v3.5.
	// Just here for safe migration in v3.4.
	DeprecatedLogOutput []string `json:"log-output"`

	// LogOutputs is either:
	//  - "default" as os.Stderr,
	//  - "stderr" as os.Stderr,
	//  - "stdout" as os.Stdout,
	//  - file path to append server logs to.
	// It can be multiple when "Logger" is zap.
	LogOutputs []string `json:"log-outputs"`

	// Debug is true, to enable debug level logging.
	Debug bool `json:"debug"`

	// ZapLoggerBuilder is used to build the zap logger.
	ZapLoggerBuilder func(*Config) error

	// 日志记录器，需要在服务启动前调用
	loggerMu *sync.RWMutex
	logger   *zap.Logger
	// loggerConfig is server logger configuration for Raft logger.
	// Must be either: "loggerConfig != nil" or "loggerCore != nil && loggerWriteSyncer != nil".
	loggerConfig *zap.Config
	// loggerCore is "zapcore.Core" for raft logger.
	// Must be either: "loggerConfig != nil" or "loggerCore != nil && loggerWriteSyncer != nil".
	loggerCore        zapcore.Core
	loggerWriteSyncer zapcore.WriteSyncer

	// EnableGRPCGateway is false to disable grpc gateway.
	EnableGRPCGateway bool `json:"enable-grpc-gateway"`

	// TO BE DEPRECATED

	// LogPkgLevels is being deprecated in v3.5.
	// Only valid if "logger" option is "capnslog".
	// WARN: DO NOT USE THIS!
	LogPkgLevels string `json:"log-package-levels"`
}

func NewConfig() *Config {

	// url字符串转为URL structure
	lpurl, _ := url.Parse(DefaultListenPeerURLs)
	apurl, _ := url.Parse(DefaultInitialAdvertisePeerURLs)
	lcurl, _ := url.Parse(DefaultListenClientURLs)
	acurl, _ := url.Parse(DefaultAdvertiseClientURLs)
	cfg := &Config{
		MaxSnapFiles: DefaultMaxSnapshots,
		MaxWalFiles:  DefaultMaxWALs,

		Name: DefaultName,

		SnapshotCount:          etcdserver.DefaultSnapshotCount,
		SnapshotCatchUpEntries: etcdserver.DefaultSnapshotCatchUpEntries,

		MaxTxnOps:       DefaultMaxTxnOps,
		MaxRequestBytes: DefaultMaxRequestBytes,

		GRPCKeepAliveMinTime:  DefaultGRPCKeepAliveMinTime,
		GRPCKeepAliveInterval: DefaultGRPCKeepAliveInterval,
		GRPCKeepAliveTimeout:  DefaultGRPCKeepAliveTimeout,

		TickMs:                     100,
		ElectionMs:                 1000,
		InitialElectionTickAdvance: true,

		LPUrls: []url.URL{*lpurl},
		LCUrls: []url.URL{*lcurl},
		APUrls: []url.URL{*apurl},
		ACUrls: []url.URL{*acurl},

		ClusterState:        ClusterStateFlagNew,
		InitialClusterToken: "etcd-cluster",

		StrictReconfigCheck: DefaultStrictReconfigCheck,
		Metrics:             "basic",
		EnableV2:            DefaultEnableV2,

		CORS:          map[string]struct{}{"*": {}},
		HostWhitelist: map[string]struct{}{"*": {}},

		AuthToken:  "simple",
		BcryptCost: uint(bcrypt.DefaultCost),

		PreVote: false, // TODO: enable by default in v3.5

		loggerMu:            new(sync.RWMutex),
		logger:              nil,
		Logger:              "zap",
		DeprecatedLogOutput: []string{DefaultLogOutput},
		LogOutputs:          []string{DefaultLogOutput},
		Debug:               false,
		LogPkgLevels:        "",
	}

	cfg.InitialCluster = cfg.InitialClusterFromName(cfg.Name)
	return cfg
}

// 从name初始化cluster，默认name 为default
func (cfg Config) InitialClusterFromName(name string) (ret string) {

	if len(cfg.APUrls) == 0 {
		return ""
	}
	n := name
	if name == "" {
		n = DefaultName
	}

	// name拼接APURL
	for i := range cfg.APUrls {
		ret = ret + "," + n + "=" + cfg.APUrls[i].String()
	}
	return ret[1:] //去除前面的逗号
}

// 验证*embed.Config是否被正确配置
func (cfg *Config) Validate() error {

	if err := cfg.setupLogging(); err != nil {
		return err
	}

	//..

	return nil
}

func (cfg Config) IsNewCluster() bool { return cfg.ClusterState == ClusterStateFlagNew }
func (cfg Config) ElectionTicks() int { return int(cfg.ElectionMs / cfg.TickMs) }

// 是否为默认的peerHost ， 数组里就一个值且值为默认http://localhost:2380
func (cfg Config) defaultPeerHost() bool {
	return len(cfg.APUrls) == 1 && cfg.APUrls[0].String() == DefaultInitialAdvertisePeerURLs
}

// 是否为默认的clientHost ， 数组里就一个值且值为默认http://localhost:2379
func (cfg Config) defaultClientHost() bool {
	return len(cfg.ACUrls) == 1 && cfg.ACUrls[0].String() == DefaultAdvertiseClientURLs
}

// 更新DefaultCluster
func (cfg *Config) UpdateDefaultClusterFromName(defaultInitialCluster string) (string, error) {
	// 为空的情况， 本机mac为空
	if defaultHostname == "" || defaultHostStatus != nil {
		// 当name被指定(e.g. 'etcd --name=abc')，则更新 'initial-cluster'
		if cfg.Name != DefaultName && cfg.InitialCluster == defaultInitialCluster {
			cfg.InitialCluster = cfg.InitialClusterFromName(cfg.Name)
		}
		return "", defaultHostStatus
	}
	used := false
	pip, pport := cfg.LPUrls[0].Hostname(), cfg.LPUrls[0].Port()
	if cfg.defaultPeerHost() && pip == "0.0.0.0" {
		//更改使用defaultHostname ， defaultHostname在linux平台在netutil中返回
		cfg.APUrls[0] = url.URL{Scheme: cfg.APUrls[0].Scheme, Host: fmt.Sprintf("%s:%s", defaultHostname, pport)}
		used = true
	}

	// 当name被指定(e.g. 'etcd --name=abc')，则更新 'initial-cluster'
	if cfg.Name != DefaultName && cfg.InitialCluster == defaultInitialCluster {
		cfg.InitialCluster = cfg.InitialClusterFromName(cfg.Name)
	}

	cip, cport := cfg.LCUrls[0].Hostname(), cfg.LCUrls[0].Port()
	if cfg.defaultClientHost() && cip == "0.0.0.0" {
		cfg.ACUrls[0] = url.URL{Scheme: cfg.ACUrls[0].Scheme, Host: fmt.Sprintf("%s:%s", defaultHostname, cport)}
		used = true
	}

	dhost := defaultHostname
	if !used {
		dhost = ""
	}
	return dhost, defaultHostStatus

}

// func updateCipherSuites(tls *transport.TLSInfo, ss []string) error {
// 	if len(tls.CipherSuites) > 0 && len(ss) > 0 {
// 		return fmt.Errorf("TLSInfo.CipherSuites is already specified (given %v)", ss)
// 	}
// 	if len(ss) > 0 {
// 		cs := make([]uint16, len(ss))
// 		for i, s := range ss {
// 			var ok bool
// 			cs[i], ok = tlsutil.GetCipherSuite(s)
// 			if !ok {
// 				return fmt.Errorf("unexpected TLS cipher suite %q", s)
// 			}
// 		}
// 		tls.CipherSuites = cs
// 	}
// 	return nil
// }

func (cfg *Config) getAPURLs() (ss []string) {
	ss = make([]string, len(cfg.APUrls))
	for i := range cfg.APUrls {
		ss[i] = cfg.APUrls[i].String()
	}
	return ss
}

func (cfg *Config) getLPURLs() (ss []string) {
	ss = make([]string, len(cfg.LPUrls))
	for i := range cfg.LPUrls {
		ss[i] = cfg.LPUrls[i].String()
	}
	return ss
}

func (cfg *Config) getACURLs() (ss []string) {
	ss = make([]string, len(cfg.ACUrls))
	for i := range cfg.ACUrls {
		ss[i] = cfg.ACUrls[i].String()
	}
	return ss
}

func (cfg *Config) getLCURLs() (ss []string) {
	ss = make([]string, len(cfg.LCUrls))
	for i := range cfg.LCUrls {
		ss[i] = cfg.LCUrls[i].String()
	}
	return ss
}

func (cfg *Config) getMetricsURLs() (ss []string) {
	ss = make([]string, len(cfg.ListenMetricsUrls))
	for i := range cfg.ListenMetricsUrls {
		ss[i] = cfg.ListenMetricsUrls[i].String()
	}
	return ss
}
