//go:build linux

package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/medici-finance/assay/statusgen/evaluation"
)

// readSetRecorderAbsent is the could-not-check reason a Linux row records
// when no recorder ran for it: the row writes no read-set file, so its
// witness keeps the conservative scope (fail closed).
const readSetRecorderAbsent = evaluation.ReasonNoRecorder

// Reasons a traced row records when the tracer itself could not finish.
const (
	reasonTracerRefused = "the host refused the read-set tracer"
	reasonTracerArch    = "the read-set tracer does not support this architecture"
)

// rowReadSetToken is an executed row's read-set cell on Linux when the run
// named no recorder target.
func rowReadSetToken() string { return evaluation.ReadSetCellToken("", readSetRecorderAbsent) }

// newReadSetTarget is the run's recorder target: the brief at repository path
// brief, recorded against the tree at root. nil (no recorder; every row then
// records a could-not-check cell) when the session context cannot be read.
func newReadSetTarget(root, briefPath string, rowEnv []string) *readSetTarget {
	env, commit, err := newRecordEnv(root, rowEnv, time.Now())
	if err != nil {
		return nil
	}
	abs, err := filepath.Abs(briefPath)
	if err != nil {
		return nil
	}
	abs = resolveBest(abs)
	rel, ok := env.inTree(abs)
	if !ok || rel == "." {
		return nil
	}
	return &readSetTarget{env: env, brief: filepath.ToSlash(rel), commit: commit}
}

// rowTrace is a traced row's outcome for runVerifyCommandWith.
type rowTrace struct {
	output []byte
	err    error  // nil, or rowExitError for a non-zero exit or a signal
	reads  string // the row's read-set cell
}

// traceRow runs cmd under the read-set tracer. handled is false when the
// tracer could not start the row at all; the caller then runs it untraced and
// records reads (always a could-not-check cell). wrapped says the row runs
// under the network-off wrapper, whose two execs precede the row's shell.
func traceRow(ctx context.Context, cmd *exec.Cmd, target *readSetTarget, wrapped bool) (rowTrace, bool) {
	if target == nil || target.env == nil {
		return rowTrace{reads: evaluation.ReadSetCellToken("", readSetRecorderAbsent)}, false
	}
	if !tracerArch {
		return rowTrace{reads: evaluation.ReadSetCellToken("", reasonTracerArch)}, false
	}
	arm := 0
	if wrapped {
		arm = 2
	}
	// A fresh command with no context: the tracer reaps every tracee itself
	// and enforces the deadline, so os/exec's own wait and cancel never run.
	attr := syscall.SysProcAttr{}
	if cmd.SysProcAttr != nil {
		attr = *cmd.SysProcAttr
	}
	c := &exec.Cmd{Path: cmd.Path, Args: cmd.Args, Dir: cmd.Dir, Env: cmd.Env, SysProcAttr: &attr}
	if c.Path == "" || cmd.Err != nil {
		return rowTrace{reads: evaluation.ReadSetCellToken("", reasonTracerRefused)}, false
	}
	goBin := ""
	if target.env.GOROOT != "" {
		goBin = filepath.Join(target.env.GOROOT, "bin", "go")
	}
	tr := runTraced(ctx, c, arm, func(exe string) bool { return goBin != "" && exe == goBin })
	if tr.startErr != nil {
		return rowTrace{reads: evaluation.ReadSetCellToken("", reasonTracerRefused)}, false
	}
	var err error
	switch {
	case !tr.exited:
		err = rowExitError{code: -1}
	case tr.exit != 0:
		err = rowExitError{code: tr.exit}
	}
	return rowTrace{output: tr.output, err: err, reads: target.finish(tr.log)}, true
}
