package etcdserver

import (
	"time"

	"github.com/iScript/etcd-cr/mvcc/backend"
	"go.uber.org/zap"
)

func newBackend(cfg ServerConfig) backend.Backend {

	bcfg := backend.DefaultBackendConfig()
	bcfg.Path = cfg.backendPath()

	if cfg.BackendBatchLimit != 0 { //没赋值，默认为0
		bcfg.BatchLimit = cfg.BackendBatchLimit
		if cfg.Logger != nil {
			cfg.Logger.Info("setting backend batch limit", zap.Int("batch limit", cfg.BackendBatchLimit))
		}
	}

	if cfg.BackendBatchInterval != 0 {
		bcfg.BatchInterval = cfg.BackendBatchInterval
		if cfg.Logger != nil {
			cfg.Logger.Info("setting backend batch interval", zap.Duration("batch interval", cfg.BackendBatchInterval))
		}
	}
	bcfg.BackendFreelistType = cfg.BackendFreelistType
	bcfg.Logger = cfg.Logger

	if cfg.QuotaBackendBytes > 0 && cfg.QuotaBackendBytes != DefaultQuotaBytes {
		// permit 10% excess over quota for disarm
		bcfg.MmapSize = uint64(cfg.QuotaBackendBytes + cfg.QuotaBackendBytes/10)
	}
	return backend.New(bcfg)

}

// 使用当前etcd db 返回一个backend
func openBackend(cfg ServerConfig) backend.Backend {

	// fn := cfg.backendPath()
	// fmt.Println(fn, 222)

	_, beOpened := time.Now(), make(chan backend.Backend)

	go func() {
		beOpened <- newBackend(cfg)
	}()

	return nil
}
