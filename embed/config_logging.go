package embed

import (
	"fmt"

	"github.com/iScript/etcd-cr/pkg/logutil"
	"go.uber.org/zap"
	//zap是对zapcore的封装
)

// 返回logger.
func (cfg Config) GetLogger() *zap.Logger {
	cfg.loggerMu.RLock()
	l := cfg.logger //返回logger，为zap.Logger类型。   小写private相对于包
	cfg.loggerMu.RUnlock()
	return l
}

func (cfg *Config) setupLogging() error {

	switch cfg.Logger {
	case "zap":
		outputPaths, errOutputPaths := make([]string, 0), make([]string, 0)

		for _, v := range cfg.LogOutputs {
			switch v {
			case DefaultLogOutput:
				outputPaths = append(outputPaths, StdErrLogOutput)
				errOutputPaths = append(errOutputPaths, StdErrLogOutput)
			// case JournalLogOutput:
			// 	isJournal = true
			case StdErrLogOutput:
				outputPaths = append(outputPaths, StdErrLogOutput)
				errOutputPaths = append(errOutputPaths, StdErrLogOutput)

			case StdOutLogOutput:
				outputPaths = append(outputPaths, StdOutLogOutput)
				errOutputPaths = append(errOutputPaths, StdOutLogOutput)

			default:
				outputPaths = append(outputPaths, v)
				errOutputPaths = append(errOutputPaths, v)
			}
		}

		// 返回zap.Config  , 更新默认config的outpath
		copied := logutil.AddOutputPaths(logutil.DefaultZapLoggerConfig, outputPaths, errOutputPaths)
		if cfg.Debug {
			copied.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
			// grpc.EnableTracing = true
		}
		if cfg.ZapLoggerBuilder == nil {
			cfg.ZapLoggerBuilder = func(c *Config) error {
				var err error
				c.logger, err = copied.Build() //zap build
				if err != nil {
					return err
				}
				c.loggerMu.Lock()
				defer c.loggerMu.Unlock()
				c.loggerConfig = &copied
				c.loggerCore = nil
				c.loggerWriteSyncer = nil
				// grpcLogOnce.Do(func() {
				// 	// debug true, enable info, warning, error
				// 	// debug false, only discard info
				// 	var gl grpclog.LoggerV2
				// 	gl, err = logutil.NewGRPCLoggerV2(copied)
				// 	if err == nil {
				// 		grpclog.SetLoggerV2(gl)
				// 	}
				// })
				return nil
			}
		}

		// 执行前面定义的builder
		err := cfg.ZapLoggerBuilder(cfg)
		if err != nil {
			return err
		}

		// logTLSHandshakeFailure ?? 用到再看

	default:
		return fmt.Errorf("unknown logger option %q", cfg.Logger)
	}

	return nil
}
