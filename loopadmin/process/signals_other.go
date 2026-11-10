//go:build !unix

package process

import (
	"context"
	"os"
	"os/signal"
)

func NotifyContext(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, os.Interrupt)
}
