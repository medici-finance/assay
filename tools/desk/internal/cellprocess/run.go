// Package cellprocess executes a single harness invocation and
// cleans up its owned process group/job before another cadence turn can start.
// It is process supervision, not a sandbox against a hostile harness escaping
// its group or delegating work to an unrelated pre-existing service.
package cellprocess

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellcache"
)

const pipeWait = 500 * time.Millisecond
const cleanupWait = 2 * time.Second

type processTree interface {
	started(*os.Process) error
	kill(*os.Process) error
	empty(*os.Process) (bool, error)
	close() error
}

// Run preserves argv, env and dir, supplies EOF on stdin, and never invokes a
// shell. A nil env is an empty environment, not permission to inherit secrets.
// Cancellation kills the owned tree and Wait reaps the direct child. Descendants
// are also cleaned after normal child exit. Output writers must not block forever.
// A true uncertain means cleanup could not be proved; callers must retain their
// dirty ownership state and refuse further turns. Windows needs native runtime
// verification in addition to cross-compilation.
func Run(ctx context.Context, argv, env []string, dir string, stdout, stderr io.Writer) (exitCode int, uncertain bool, err error) {
	return withCache(env, func() (int, bool, error) {
		return run(ctx, argv, env, dir, stdout, stderr, newProcessTree)
	})
}

// RunInteractive preserves stdin and terminal foreground ownership in addition
// to Run's containment guarantees. It imposes no execution deadline.
func RunInteractive(ctx context.Context, argv, env []string, dir string, stdin *os.File, stdout, stderr io.Writer) (int, bool, error) {
	return withCache(env, func() (int, bool, error) {
		return run(ctx, argv, env, dir, stdout, stderr, func(cmd *exec.Cmd) (processTree, error) {
			if stdin != nil {
				cmd.Stdin = stdin
			}
			return newInteractiveProcessTree(cmd)
		})
	})
}

// RunInteractiveObserved enrolls a child before accepting its completion.
func RunInteractiveObserved(ctx context.Context, argv, env []string, dir string, stdin *os.File, stdout, stderr io.Writer, started func(int) error) (int, bool, error) {
	return RunInteractive(observeLaunch(ctx, started), argv, env, dir, stdin, stdout, stderr)
}

// RunObserved goes through Run's public custody boundary. Observation must never
// create a second route around launch admission or cache ownership.
func RunObserved(ctx context.Context, argv, env []string, dir string, stdout, stderr io.Writer, started func(int) error) (int, bool, error) {
	return Run(observeLaunch(ctx, started), argv, env, dir, stdout, stderr)
}

type observerKey struct{}

func observeLaunch(ctx context.Context, started func(int) error) context.Context {
	if ctx == nil || started == nil {
		return ctx
	}
	return context.WithValue(ctx, observerKey{}, started)
}

// One custody boundary covers cadence, interactive and detached tmux wrappers.
// A crash or uncertain child cleanup leaves a durable active-cache record.
func withCache(env []string, child func() (int, bool, error)) (int, bool, error) {
	lease, err := cellcache.Acquire(env)
	if err != nil {
		return -1, false, err
	}
	code, uncertain, err := child()
	finishErr := lease.Finish(!uncertain)
	return code, uncertain || finishErr != nil, errors.Join(err, finishErr)
}

func run(ctx context.Context, argv, env []string, dir string, stdout, stderr io.Writer, setup func(*exec.Cmd) (processTree, error)) (int, bool, error) {
	var started func(int) error
	if ctx != nil {
		started, _ = ctx.Value(observerKey{}).(func(int) error)
	}
	return runObserved(ctx, argv, env, dir, stdout, stderr, setup, started)
}

func runObserved(ctx context.Context, argv, env []string, dir string, stdout, stderr io.Writer, setup func(*exec.Cmd) (processTree, error), started func(int) error) (int, bool, error) {
	if ctx == nil || len(argv) == 0 || argv[0] == "" {
		return -1, false, errors.New("process launch requires a context and executable")
	}
	if err := ctx.Err(); err != nil {
		return -1, false, err
	}
	commandContext, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(commandContext, argv[0], argv[1:]...)
	cmd.Env = append([]string{}, env...)
	cmd.Dir, cmd.Stdout, cmd.Stderr = dir, stdout, stderr
	cmd.WaitDelay = pipeWait
	tree, err := setup(cmd)
	if err != nil {
		return -1, false, fmt.Errorf("prepare process containment: %w", err)
	}
	// On Windows the child starts suspended. A context cancellation cannot race
	// job assignment/resume; its callback waits for the enrollment result.
	ready := make(chan struct{})
	cmd.Cancel = func() error {
		<-ready
		return tree.kill(cmd.Process)
	}
	if err := cmd.Start(); err != nil {
		close(ready)
		return -1, false, errors.Join(err, tree.close())
	}
	startErr := tree.started(cmd.Process)
	if startErr == nil && started != nil {
		startErr = started(cmd.Process.Pid)
	}
	close(ready)
	var abortErr error
	if startErr != nil {
		abortErr = tree.kill(cmd.Process)
		cancel()
	}
	// If termination is denied, os/exec cannot promise Process.Wait finishes.
	// Keep its reaper alive, but stop this supervisor with an uncertain result
	// rather than wedging the caller or pretending another turn is safe.
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	var waitErr error
	select {
	case waitErr = <-waited:
	case <-commandContext.Done():
		timer := time.NewTimer(pipeWait + cleanupWait)
		defer timer.Stop()
		select {
		case waitErr = <-waited:
		case <-timer.C:
			return -1, true, errors.Join(startErr, abortErr, ctx.Err(), errors.New("direct child could not be reaped after cancellation"), tree.close())
		}
	}
	code := -1
	if cmd.ProcessState != nil {
		code = cmd.ProcessState.ExitCode()
	}
	// WaitDelay bounds pipes inherited by grandchildren; cleanup is necessary
	// even when the direct child succeeded and no deadline was reached.
	cleanErr := cleanup(tree, cmd.Process)
	closeErr := tree.close()
	uncertain := abortErr != nil || cleanErr != nil || closeErr != nil
	return code, uncertain, errors.Join(startErr, abortErr, waitErr, ctx.Err(), cleanErr, closeErr)
}

func cleanup(tree processTree, process *os.Process) error {
	killErr := tree.kill(process)
	until := time.Now().Add(cleanupWait)
	for {
		empty, err := tree.empty(process)
		if err == nil && empty && killErr == nil {
			return nil
		}
		if !time.Now().Before(until) {
			if err != nil {
				return errors.Join(killErr, fmt.Errorf("cannot verify process-tree cleanup: %w", err))
			}
			return errors.Join(killErr, errors.New("process-tree cleanup not confirmed within its bound"))
		}
		time.Sleep(10 * time.Millisecond)
		// Teardown can transiently reject a signal or observation (EPERM was
		// observed on Darwin). Retry within the same cleanup bound;
		// never turn a denied signal or observation into evidence of emptiness.
		if killErr != nil {
			killErr = tree.kill(process)
		}
	}
}
