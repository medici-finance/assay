//go:build !linux

package main

import (
	"context"
	"os/exec"
	"runtime"
)

// rowReadSetToken is an executed row's read-set token off Linux: always the
// could-not-check token, never a read set.
func rowReadSetToken() string { return readSetOffLinuxToken(runtime.GOOS) }

// rowTrace is a traced row's outcome; off Linux nothing is ever traced.
type rowTrace struct {
	output []byte
	err    error
	reads  string
}

// traceRow never traces off Linux: the row runs untraced and records the
// off-Linux could-not-check cell.
func traceRow(context.Context, *exec.Cmd, *readSetTarget, bool) (rowTrace, bool) {
	return rowTrace{reads: rowReadSetToken()}, false
}

// newReadSetTarget is nil off Linux: no recorder runs.
func newReadSetTarget(string, string, []string) *readSetTarget { return nil }
