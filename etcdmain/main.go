package etcdmain

import (
	"fmt"
	"os"
)

// "os"
// "strings"

// "github.com/coreos/go-systemd/daemon"
// "go.uber.org/zap"

func Main() {

	checkSupportArch()

	if len(os.Args) > 1 {
		cmd := os.Args[1]
		fmt.Println(cmd)
	}

	startEtcdOrProxyV2()
}
