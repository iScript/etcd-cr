package etcdmain

import (
	"fmt"
	"os"
	"runtime"

	"google.golang.org/grpc"
)

func startEtcdOrProxyV2() {
	grpc.EnableTracing = false

	cfg := newConfig()

	//defaultInitialCluster := cfg.ec.InitialCluster //初始集群  default=http://..

	err := cfg.parse(os.Args[1:])

	if err != nil {

	}
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
