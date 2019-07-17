package etcdmain

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/iScript/etcd-cr/embed"
	"github.com/iScript/etcd-cr/pkg/flags"
	"github.com/iScript/etcd-cr/version"
)

var (
	proxyFlagOff      = "off"
	proxyFlagReadonly = "readonly"
	proxyFlagOn       = "on"

	fallbackFlagExit  = "exit"
	fallbackFlagProxy = "proxy"

	ignored = []string{
		"cluster-active-size",
		"cluster-remove-delay",
		"cluster-sync-interval",
		"config",
		"force",
		"max-result-buffer",
		"max-retry-attempts",
		"peer-heartbeat-interval",
		"peer-election-timeout",
		"retry-interval",
		"snapshot",
		"v",
		"vv",
		// for coverage testing
		"test.coverprofile",
		"test.outputdir",
	}
)

// etcd的配置
type config struct {
	ec           embed.Config //内置配置
	cp           configProxy
	cf           configFlags //命令行配置
	configFile   string
	printVersion bool
	ignored      []string
}

type configProxy struct {
	ProxyFailureWaitMs     uint `json:"proxy-failure-wait"`
	ProxyRefreshIntervalMs uint `json:"proxy-refresh-interval"`
	ProxyDialTimeoutMs     uint `json:"proxy-dial-timeout"`
	ProxyWriteTimeoutMs    uint `json:"proxy-write-timeout"`
	ProxyReadTimeoutMs     uint `json:"proxy-read-timeout"`
	Fallback               string
	Proxy                  string
	ProxyJSON              string `json:"proxy"`
	FallbackJSON           string `json:"discovery-fallback"`
}

// 用于命令行解析的flags
type configFlags struct {
	flagSet *flag.FlagSet
	// clusterState *flags.SelectiveStringValue
	// fallback     *flags.SelectiveStringValue
	// proxy        *flags.SelectiveStringValue
}

func newConfig() *config {

	cfg := &config{
		ec: *embed.NewConfig(),
		cp: configProxy{
			Proxy:                  proxyFlagOff,
			ProxyFailureWaitMs:     5000,
			ProxyRefreshIntervalMs: 30000,
			ProxyDialTimeoutMs:     1000,
			ProxyWriteTimeoutMs:    5000,
		},
		ignored: ignored,
	}
	cfg.cf = configFlags{
		flagSet: flag.NewFlagSet("etcd", flag.ContinueOnError),
		// clusterState: flags.NewSelectiveStringValue(
		// 	embed.ClusterStateFlagNew,
		// 	embed.ClusterStateFlagExisting,
		// ),
		// fallback: flags.NewSelectiveStringValue(
		// 	fallbackFlagProxy,
		// 	fallbackFlagExit,
		// ),
		// proxy: flags.NewSelectiveStringValue(
		// 	proxyFlagOff,
		// 	proxyFlagReadonly,
		// 	proxyFlagOn,
		// ),
	}

	fs := cfg.cf.flagSet

	// 重写帮助信息，可不加，若不加，则系统根据定义的flag来显示
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, usageline)
	}

	// version参数，绑定到变量，默认为false。
	fs.BoolVar(&cfg.printVersion, "version", false, "Print the version and exit.")

	fs.StringVar(&cfg.ec.Name, "name", cfg.ec.Name, "Human-readable name for this member.")
	// 其他参数遇到再回看

	return cfg
}

func (cfg *config) parse(arguments []string) error {

	perr := cfg.cf.flagSet.Parse(arguments)
	switch perr {
	case nil: //没错误，不执行任何操作
	case flag.ErrHelp: //有错误，错误为如果调用了-h或--help，但没定义这样的标志。
		fmt.Println(flagsline)
		os.Exit(0)
	default:
		os.Exit(2)
	}

	// args 里是非flag的参数，  如不加 -- 的字符串
	if len(cfg.cf.flagSet.Args()) != 0 {
		return fmt.Errorf("'%s' is not a valid flag", cfg.cf.flagSet.Arg(0))
	}

	if cfg.printVersion {
		fmt.Printf("etcd Version: %s\n", version.Version)
		fmt.Printf("Git SHA: %s\n", version.GitSHA)
		fmt.Printf("Go Version: %s\n", runtime.Version())
		fmt.Printf("Go OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	var err error

	// 配置文件若为空，获取env的
	if cfg.configFile == "" {
		cfg.configFile = os.Getenv(flags.FlagToEnv("ETCD", "config-file"))
	}

	//若不为空,从配置文件
	if cfg.configFile != "" {
		// err = cfg.configFromFile(cfg.configFile)
		// if lg := cfg.ec.GetLogger(); lg != nil {
		// 	lg.Info(
		// 		"loaded server configuration, other configuration command line flags and environment variables will be ignored if provided",
		// 		zap.String("path", cfg.configFile),
		// 	)
		// } else {
		// 	plog.Infof("Loading server configuration from %q. Other configuration command line flags and environment variables will be ignored if provided.", cfg.configFile)
		// }
	} else {
		//默认为空的情况，从命令行获得配置
		err = cfg.configFromCmdLine()
	}
	// now logger is set up
	return err

}

func (cfg *config) configFromCmdLine() error {
	err := flags.SetFlagsFromEnv("ETCD", cfg.cf.flagSet)
	if err != nil {
		return err
	}

	return nil
}
