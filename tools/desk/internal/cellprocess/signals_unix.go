//go:build unix

package cellprocess

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// NotifyContext also catches ordinary cockpit/OS termination so the caller can
// cancel and reap an active pass before its ownership lease is released.
func NotifyContext(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
}
