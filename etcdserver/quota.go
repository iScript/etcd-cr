package etcdserver

import (
	"sync"

	humanize "github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

const (
	// 默认限额
	DefaultQuotaBytes = int64(2 * 1024 * 1024 * 1024) // 2GB

	// 后端限额的最大字节数
	MaxQuotaBytes = int64(8 * 1024 * 1024 * 1024) // 8GB
)

// 对任意请求的限额
type Quota interface {
	// 请求是否符合限额
	Available(req interface{}) bool
	// 根据请求计算费用
	Cost(req interface{}) int
	// 配额的剩余费用
	Remaining() int64
}

type passthroughQuota struct{}

func (*passthroughQuota) Available(interface{}) bool { return true }
func (*passthroughQuota) Cost(interface{}) int       { return 0 }
func (*passthroughQuota) Remaining() int64           { return 1 }

type backendQuota struct {
	s               *EtcdServer
	maxBackendBytes int64
}

const (
	// leaseOverhead is an estimate for the cost of storing a lease
	leaseOverhead = 64
	// kvOverhead is an estimate for the cost of storing a key's metadata
	kvOverhead = 256
)

var (
	// only log once
	quotaLogOnce sync.Once

	DefaultQuotaSize = humanize.Bytes(uint64(DefaultQuotaBytes))
	maxQuotaSize     = humanize.Bytes(uint64(MaxQuotaBytes))
)

//创建具有给定存储限制的限额层
func NewBackendQuota(s *EtcdServer, name string) Quota {
	lg := s.getLogger()
	quotaBackendBytes.Set(float64(s.Cfg.QuotaBackendBytes)) // prometheus指标

	if s.Cfg.QuotaBackendBytes < 0 {
		quotaLogOnce.Do(func() {
			if lg != nil {
				lg.Info(
					"disabled backend quota",
					zap.String("quota-name", name),
					zap.Int64("quota-size-bytes", s.Cfg.QuotaBackendBytes),
				)
			}
		})
		return &passthroughQuota{}
	}
	return nil
}
