package etcdmain

import (
	"fmt"
	"os"

	"github.com/coreos/go-systemd/daemon"
	"go.uber.org/zap"
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

func notifySystemd(lg *zap.Logger) {
	_, err := daemon.SdNotify(false, daemon.SdNotifyReady)
	if err != nil {
		if lg != nil {
			lg.Error("failed to notify systemd for readiness", zap.Error(err))
		}
	}
}
