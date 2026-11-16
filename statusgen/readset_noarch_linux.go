//go:build linux && !amd64 && !arm64

package main

import (
	"context"
	"os/exec"
)

// tracerArch is false: the read-set tracer's system-call table covers amd64
// and arm64 only, so every row here records could-not-check.
const tracerArch = false

type tracedRun struct {
	output   []byte
	exit     int
	exited   bool
	startErr error
	log      traceLog
}

func runTraced(context.Context, *exec.Cmd, int, func(string) bool) tracedRun {
	return tracedRun{exit: -1, startErr: errNoTracer}
}

var errNoTracer = exec.ErrNotFound
