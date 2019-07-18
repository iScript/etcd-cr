package embed

import "go.uber.org/zap"

// 返回logger.
func (cfg Config) GetLogger() *zap.Logger {
	cfg.loggerMu.RLock()
	l := cfg.logger //返回logger，为zap.Logger类型。   小写private相对于包
	cfg.loggerMu.RUnlock()
	return l
}
