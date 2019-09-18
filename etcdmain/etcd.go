package etcdmain

import (
	"fmt"
	"os"
	"runtime"

	"github.com/coreos/pkg/capnslog"
	"github.com/iScript/etcd-cr/embed"
	"github.com/iScript/etcd-cr/pkg/fileutil"
	"github.com/iScript/etcd-cr/pkg/osutil"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

var plog = capnslog.NewPackageLogger("github.com/iScript/etcd-cr", "pkg/flags")

type dirType string

var (
	dirMember = dirType("member")
	dirProxy  = dirType("proxy")
	dirEmpty  = dirType("empty")
)

func startEtcdOrProxyV2() {
	grpc.EnableTracing = false

	cfg := newConfig()

	defaultInitialCluster := cfg.ec.InitialCluster //初始集群  default=http://..

	err := cfg.parse(os.Args[1:])
	lg := cfg.ec.GetLogger()

	// flag.parse 是否有错误
	if err != nil {
		if lg != nil {
			lg.Warn("failed to verify flags", zap.Error(err))
		} else {
			plog.Errorf("error verifying flags, %v. See 'etcd --help'.", err)
		}

		switch err {
		case embed.ErrUnsetAdvertiseClientURLsFlag: //embed.validate 验证返回的错误
			if lg != nil {
				lg.Warn("advertise client URLs are not set", zap.Error(err))
			} else {
				plog.Errorf("When listening on specific address(es), this etcd process must advertise accessible url(s) to each connected client.")
			}
		}
		os.Exit(1)
	}

	// 函数结束后执行
	defer func() {
		logger := cfg.ec.GetLogger()
		if logger != nil {
			logger.Sync() //zap api,进程退出前调用Sync是个好习惯
		}
	}()

	defaultHost, dhErr := (&cfg.ec).UpdateDefaultClusterFromName(defaultInitialCluster) //mac电脑暂时返回空

	if defaultHost != "" {
		if lg != nil {
			lg.Info(
				"detected default host for advertise",
				zap.String("host", defaultHost),
			)
		} else {
			plog.Infof("advertising using detected default host %q", defaultHost)
		}
	}

	if dhErr != nil {
		if lg != nil {
			lg.Info("failed to detect default host", zap.Error(dhErr))
		} else {
			plog.Noticef("failed to detect default host (%v)", dhErr)
		}
	}

	if cfg.ec.Dir == "" {
		cfg.ec.Dir = fmt.Sprintf("%v.etcd", cfg.ec.Name)
		if lg != nil {
			lg.Warn(
				"'data-dir' was empty; using default",
				zap.String("data-dir", cfg.ec.Dir),
			)
		} else {
			plog.Warningf("no data-dir provided, using default data-dir ./%s", cfg.ec.Dir)
		}
	}

	var stopped <-chan struct{} // stopped只能接收通道中的数据
	var errc <-chan error

	which := identifyDataDirOrDie(cfg.ec.GetLogger(), cfg.ec.Dir)
	// 初次启动为空即dirEmpty , 第二次启动则为member
	if which != dirEmpty {
		if lg != nil {
			lg.Info(
				"server has been already initialized",
				zap.String("data-dir", cfg.ec.Dir),
				zap.String("dir-type", string(which)),
			)
		}
		switch which {
		case dirMember:
			stopped, errc, err = startEtcd(&cfg.ec)
		case dirProxy:
			//err = startProxy(cfg)
		default:
			if lg != nil {
				lg.Panic(
					"unknown directory type",
					zap.String("dir-type", string(which)),
				)
			}
		}
	} else {
		shouldProxy := cfg.isProxy() // 默认off ， 返回false
		if !shouldProxy {
			_, _, err = startEtcd(&cfg.ec)

			if err != nil {
				if lg != nil {
					lg.Warn("failed to start etcd", zap.Error(err))
				}
			}
		}
	}

	// 启动etcd有错误的情况
	if err != nil {

	}

	osutil.HandleInterrupts(lg)

	//到了这一步，已经初始化完成了etcd
	//tcp端口已经准备好监听请求
	//etcd已经加入了集群
	notifySystemd(lg)

	// 阻塞 监听是否有错误或停止了
	// 有则退出程序
	// errc 为embed etcd.errc ， e.errHandler 有错误则传入
	// stopped 为 embed etcd.server.done
	select {
	case lerr := <-errc:
		fmt.Println(lerr)
		if lg != nil {
			lg.Fatal("listener failed", zap.Error(lerr))
		}
	case <-stopped:
	}

	osutil.Exit(0)
}

func startEtcd(cfg *embed.Config) (<-chan struct{}, <-chan error, error) {
	e, err := embed.StartEtcd(cfg)
	if err != nil {
		return nil, nil, err
	}
	osutil.RegisterInterruptHandler(e.Close)

	// 监听和channel有关的IO操作
	// 会阻塞，直到监听到一个可以执行的IO操作为止
	select {
	case <-e.Server.ReadyNotify(): // wait for e.Server to join the cluster
	case <-e.Server.StopNotify(): // publish aborted from 'ErrStopped'
	}
	return e.Server.StopNotify(), e.Err(), nil
}

// 返回文件夹类型
func identifyDataDirOrDie(lg *zap.Logger, dir string) dirType {
	names, err := fileutil.ReadDir(dir)

	if err != nil {
		if os.IsNotExist(err) { //是否不存在dir或file，第一次启动为空
			return dirEmpty
		}
		if lg != nil {
			lg.Fatal("failed to list data directory", zap.String("dir", dir), zap.Error(err))
		} else {
			plog.Fatalf("error listing data dir: %s", dir)
		}
	}

	var m, p bool
	for _, name := range names {
		switch dirType(name) {
		case dirMember:
			m = true
		case dirProxy:
			p = true
		default:
			if lg != nil {
				lg.Warn(
					"found invalid file under data directory",
					zap.String("filename", name),
					zap.String("data-dir", dir),
				)
			} else {
				plog.Warningf("found invalid file/dir %s under data dir %s (Ignore this if you are upgrading etcd)", name, dir)
			}
		}
	}

	if m && p {
		if lg != nil {
			lg.Fatal("invalid datadir; both member and proxy directories exist")
		} else {
			plog.Fatal("invalid datadir. Both member and proxy directories exist.")
		}
	}
	if m {
		return dirMember
	}
	if p {
		return dirProxy
	}
	return dirEmpty
}

// 预检查体系架构
func checkSupportArch() {

	//如果是amd64或ppc64le（64位体系），直接过
	if runtime.GOARCH == "amd64" || runtime.GOARCH == "ppc64le" {
		return
	}

	//检查环境变量 ， 如export TEST=111
	defer os.Unsetenv("ETCD_UNSUPPORTED_ARCH")
	if env, ok := os.LookupEnv("ETCD_UNSUPPORTED_ARCH"); ok && env == runtime.GOARCH {
		fmt.Printf("running etcd on unsupported architecture %q since ETCD_UNSUPPORTED_ARCH is set\n", env)
		return
	}

	fmt.Printf("etcd on unsupported platform without ETCD_UNSUPPORTED_ARCH=%s set\n", runtime.GOARCH)
	os.Exit(1)
}
