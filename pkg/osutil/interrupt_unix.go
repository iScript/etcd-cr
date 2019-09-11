package osutil

import (
	"os"
	"os/signal"
	"sync"
	"syscall"

	"go.uber.org/zap"
)

// InterruptHandler is a function that is called on receiving a
// SIGTERM or SIGINT signal.
type InterruptHandler func()

var (
	interruptRegisterMu, interruptExitMu sync.Mutex
	// interruptHandlers holds all registered InterruptHandlers in order
	// they will be executed.
	interruptHandlers = []InterruptHandler{}
)

// 注册 InterruptHandler.
func RegisterInterruptHandler(h InterruptHandler) {
	interruptRegisterMu.Lock()
	defer interruptRegisterMu.Unlock()
	interruptHandlers = append(interruptHandlers, h)
}

// HandleInterrupts calls the handler functions on receiving a SIGINT or SIGTERM.
func HandleInterrupts(lg *zap.Logger) {
	notifier := make(chan os.Signal, 1)
	signal.Notify(notifier, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-notifier

		interruptRegisterMu.Lock()
		ihs := make([]InterruptHandler, len(interruptHandlers))
		copy(ihs, interruptHandlers)
		interruptRegisterMu.Unlock()

		interruptExitMu.Lock()

		if lg != nil {
			lg.Info("received signal; shutting down", zap.String("signal", sig.String()))
		}

		for _, h := range ihs {
			h()
		}
		signal.Stop(notifier)
		pid := syscall.Getpid()
		// exit directly if it is the "init" process, since the kernel will not help to kill pid 1.
		if pid == 1 {
			os.Exit(0)
		}
		//setDflSignal(sig.(syscall.Signal))
		syscall.Kill(pid, sig.(syscall.Signal))
	}()
}

// Exit relays to os.Exit if no interrupt handlers are running, blocks otherwise.
func Exit(code int) {
	interruptExitMu.Lock()
	os.Exit(code)
}
