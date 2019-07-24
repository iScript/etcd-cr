package etcdserver

import (
	"fmt"
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

	fn := cfg.backendPath()
	// fmt.Println(fn, 222)

	now, beOpened := time.Now(), make(chan backend.Backend)

	go func() {
		beOpened <- newBackend(cfg) //向通道传入backend
	}()
	//fmt.Println("555566666", newBackend(cfg)) //调试

	//监听channel
	select {
	case be := <-beOpened: //如果通道中有传入
		fmt.Println("opened")
		if cfg.Logger != nil {
			cfg.Logger.Info("opened backend db", zap.String("path", fn), zap.Duration("took", time.Since(now)))
		}
		return be //返回，监听结束

	case <-time.After(10 * time.Second): //time.After 返回time channel，等待一定时间后channel输出当前时间
		if cfg.Logger != nil {
			cfg.Logger.Info(
				"db file is flocked by another process, or taking too long",
				zap.String("path", fn),
				zap.Duration("took", time.Since(now)),
			)
		}
	}

	return <-beOpened
}
