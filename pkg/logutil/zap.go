package logutil

import (
	"sort"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

//默认的zap log 配置
var DefaultZapLoggerConfig = zap.Config{
	Level: zap.NewAtomicLevelAt(zap.InfoLevel),

	Development: false,
	Sampling: &zap.SamplingConfig{
		Initial:    100,
		Thereafter: 100,
	},

	Encoding: "console", //json or console

	// copied from "zap.NewProductionEncoderConfig" with some updates
	EncoderConfig: zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	},

	// Use "/dev/null" to discard all
	OutputPaths:      []string{"stderr"},
	ErrorOutputPaths: []string{"stderr"},
}

// // AddOutputPaths 在已存在的 output paths 添加 output paths , 解决冲突.
// func AddOutputPaths(cfg zap.Config, outputPaths, errorOutputPaths []string) zap.Config {
// 	outputs := make(map[string]struct{})
// 	for _, v := range cfg.OutputPaths {
// 		outputs[v] = struct{}{}
// 	}
// 	for _, v := range outputPaths {
// 		outputs[v] = struct{}{}
// 	}
// 	outputSlice := make([]string, 0)
// 	if _, ok := outputs["/dev/null"]; ok {
// 		// 如果有这个key，直接设为slice里面一个值
// 		outputSlice = []string{"/dev/null"}
// 	} else {
// 		for k := range outputs {
// 			outputSlice = append(outputSlice, k)
// 		}
// 	}
// 	cfg.OutputPaths = outputSlice
// 	sort.Strings(cfg.OutputPaths)

// 	errOutputs := make(map[string]struct{})
// 	for _, v := range cfg.ErrorOutputPaths {
// 		errOutputs[v] = struct{}{}
// 	}
// 	for _, v := range errorOutputPaths {
// 		errOutputs[v] = struct{}{}
// 	}
// 	errOutputSlice := make([]string, 0)
// 	if _, ok := errOutputs["/dev/null"]; ok {
// 		errOutputSlice = []string{"/dev/null"}
// 	} else {
// 		for k := range errOutputs {
// 			errOutputSlice = append(errOutputSlice, k)
// 		}
// 	}
// 	cfg.ErrorOutputPaths = errOutputSlice
// 	sort.Strings(cfg.ErrorOutputPaths)

// 	return cfg
// }

// 合并配置
func MergeOutputPaths(cfg zap.Config) zap.Config {
	outputs := make(map[string]struct{})
	for _, v := range cfg.OutputPaths {
		outputs[v] = struct{}{}
	}
	outputSlice := make([]string, 0)
	if _, ok := outputs["/dev/null"]; ok {
		// "/dev/null" to discard all
		outputSlice = []string{"/dev/null"}
	} else {
		for k := range outputs {
			outputSlice = append(outputSlice, k)
		}
	}
	cfg.OutputPaths = outputSlice
	sort.Strings(cfg.OutputPaths)

	errOutputs := make(map[string]struct{})
	for _, v := range cfg.ErrorOutputPaths {
		errOutputs[v] = struct{}{}
	}
	errOutputSlice := make([]string, 0)
	if _, ok := errOutputs["/dev/null"]; ok {
		// "/dev/null" to discard all
		errOutputSlice = []string{"/dev/null"}
	} else {
		for k := range errOutputs {
			errOutputSlice = append(errOutputSlice, k)
		}
	}
	cfg.ErrorOutputPaths = errOutputSlice
	sort.Strings(cfg.ErrorOutputPaths)

	return cfg
}
