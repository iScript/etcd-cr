package etcdserver

import (
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt" //应该是原先的bolt停止维护了，该bbolt是fork过来的
	"go.etcd.io/etcd/pkg/types"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type ServerConfig struct {
	Name           string
	DiscoveryURL   string
	DiscoveryProxy string
	ClientURLs     types.URLs //[]url.URL类型
	PeerURLs       types.URLs
	DataDir        string
	// DedicatedWALDir config will make the etcd to write the WAL to the WALDir
	// rather than the dataDir/member/wal.
	DedicatedWALDir string

	SnapshotCount uint64

	// SnapshotCatchUpEntries is the number of entries for a slow follower
	// to catch-up after compacting the raft storage entries.
	// We expect the follower has a millisecond level latency with the leader.
	// The max throughput is around 10K. Keep a 5K entries is enough for helping
	// follower to catch up.
	// WARNING: only change this for tests. Always use "DefaultSnapshotCatchUpEntries"
	SnapshotCatchUpEntries uint64

	MaxSnapFiles uint
	MaxWALFiles  uint

	// BackendBatchInterval is the maximum time before commit the backend transaction.
	BackendBatchInterval time.Duration
	// BackendBatchLimit is the maximum operations before commit the backend transaction.
	BackendBatchLimit int

	// BackendFreelistType is the type of the backend boltdb freelist.
	BackendFreelistType bolt.FreelistType

	//InitialPeerURLsMap  types.URLsMap
	InitialClusterToken string
	NewCluster          bool
	//PeerTLSInfo         transport.TLSInfo

	CORS map[string]struct{}

	// HostWhitelist lists acceptable hostnames from client requests.
	// If server is insecure (no TLS), server only accepts requests
	// whose Host header value exists in this white list.
	HostWhitelist map[string]struct{}

	TickMs                     uint
	ElectionTicks              int
	InitialElectionTickAdvance bool

	BootstrapTimeout time.Duration

	AutoCompactionRetention time.Duration
	AutoCompactionMode      string
	QuotaBackendBytes       int64
	MaxTxnOps               uint

	// MaxRequestBytes is the maximum request size to send over raft.
	MaxRequestBytes uint

	StrictReconfigCheck bool

	// ClientCertAuthEnabled is true when cert has been signed by the client CA.
	ClientCertAuthEnabled bool

	AuthToken  string
	BcryptCost uint

	// InitialCorruptCheck is true to check data corruption on boot
	// before serving any peer/client traffic.
	InitialCorruptCheck bool
	CorruptCheckTime    time.Duration

	// PreVote is true to enable Raft Pre-Vote.
	PreVote bool

	// Logger logs server-side operations.
	// If not nil, it disables "capnslog" and uses the given logger.
	Logger *zap.Logger
	// LoggerConfig is server logger configuration for Raft logger.
	// Must be either: "LoggerConfig != nil" or "LoggerCore != nil && LoggerWriteSyncer != nil".
	LoggerConfig *zap.Config
	// LoggerCore is "zapcore.Core" for raft logger.
	// Must be either: "LoggerConfig != nil" or "LoggerCore != nil && LoggerWriteSyncer != nil".
	LoggerCore        zapcore.Core
	LoggerWriteSyncer zapcore.WriteSyncer

	Debug bool

	ForceNewCluster bool

	// EnableLeaseCheckpoint enables primary lessor to persist lease remainingTTL to prevent indefinite auto-renewal of long lived leases.
	EnableLeaseCheckpoint bool
	// LeaseCheckpointInterval time.Duration is the wait duration between lease checkpoints.
	LeaseCheckpointInterval time.Duration

	EnableGRPCGateway bool
}

func (c *ServerConfig) MemberDir() string { return filepath.Join(c.DataDir, "member") }

func (c *ServerConfig) WALDir() string {
	if c.DedicatedWALDir != "" {
		return c.DedicatedWALDir
	}
	return filepath.Join(c.MemberDir(), "wal")
}

func (c *ServerConfig) SnapDir() string { return filepath.Join(c.MemberDir(), "snap") }

func (c *ServerConfig) ShouldDiscover() bool { return c.DiscoveryURL != "" }

// ReqTimeout returns timeout for request to finish.
func (c *ServerConfig) ReqTimeout() time.Duration {
	// 5s for queue waiting, computation and disk IO delay
	// + 2 * election timeout for possible leader election
	return 5*time.Second + 2*time.Duration(c.ElectionTicks*int(c.TickMs))*time.Millisecond
}

func (c *ServerConfig) electionTimeout() time.Duration {
	return time.Duration(c.ElectionTicks*int(c.TickMs)) * time.Millisecond
}

func (c *ServerConfig) peerDialTimeout() time.Duration {
	// 1s for queue wait and election timeout
	return time.Second + time.Duration(c.ElectionTicks*int(c.TickMs))*time.Millisecond
}

func checkDuplicateURL(urlsmap types.URLsMap) bool {
	um := make(map[string]bool)
	for _, urls := range urlsmap {
		for _, url := range urls {
			u := url.String()
			if um[u] {
				return true
			}
			um[u] = true
		}
	}
	return false
}

func (c *ServerConfig) bootstrapTimeout() time.Duration {
	if c.BootstrapTimeout != 0 {
		return c.BootstrapTimeout
	}
	return time.Second
}

func (c *ServerConfig) backendPath() string { return filepath.Join(c.SnapDir(), "db") }
