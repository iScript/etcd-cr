package etcdserver

const (
	// DefaultQuotaBytes is the number of bytes the backend Size may
	// consume before exceeding the space quota.
	DefaultQuotaBytes = int64(2 * 1024 * 1024 * 1024) // 2GB
	// MaxQuotaBytes is the maximum number of bytes suggested for a backend
	// quota. A larger quota may lead to degraded performance.
	MaxQuotaBytes = int64(8 * 1024 * 1024 * 1024) // 8GB
)
