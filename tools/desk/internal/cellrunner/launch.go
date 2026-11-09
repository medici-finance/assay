// Package cellrunner binds the desk's launch custody to loopadmin's process
// runner. loopadmin/process is the one process runner (#5492);
// this package supplies the custody it requires in tools/desk: the cell's
// Go-cache admission lease (cellcache) and its launch-cache-enrollment
// telemetry step (#3353). Every desk launch goes through this package. It is
// the only tools/desk package that imports loopadmin/process (see
// TestLaunchCustodyClassGuard).
package cellrunner

import (
	"context"
	"io"
	"os"
	"strings"
	"time"

	"github.com/medici-finance/assay/loopadmin/process"
	"github.com/medici-finance/assay/tools/desk/internal/cellcache"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// cacheCustody admits a launch by taking the cell's cache lease. A crash or
// uncertain child cleanup leaves a durable active-cache record.
type cacheCustody struct{}

func (cacheCustody) Admit(env []string) (process.Admission, error) {
	began := time.Now()
	lease, err := cellcache.Acquire(env)
	enabled := false
	for _, value := range env {
		if k, v, ok := strings.Cut(value, "="); ok && k == "ASSAY_TELEMETRY" {
			enabled = v == "1"
		}
	}
	phaseCode := deskkit.ExitOK
	if err != nil {
		phaseCode = deskkit.ExitCodeOf(err)
	}
	deskkit.PerfStepEnabled(enabled, "cellctl", "launch-cache-enrollment", time.Since(began), phaseCode)
	if err != nil {
		return nil, err
	}
	// A disabled cache returns a nil lease; (*cellcache.Lease)(nil).Finish is a no-op.
	return lease, nil
}

var runner = func() *process.Runner {
	r, err := process.New(cacheCustody{})
	if err != nil {
		panic(err)
	}
	return r
}()

// Run is process.Runner.Run under the desk's cache custody.
func Run(ctx context.Context, argv, env []string, dir string, stdout, stderr io.Writer) (int, bool, error) {
	return runner.Run(ctx, argv, env, dir, stdout, stderr)
}

// RunInteractive is process.Runner.RunInteractive under the desk's cache custody.
func RunInteractive(ctx context.Context, argv, env []string, dir string, stdin *os.File, stdout, stderr io.Writer) (int, bool, error) {
	return runner.RunInteractive(ctx, argv, env, dir, stdin, stdout, stderr)
}

// RunObserved is process.Runner.RunObserved under the desk's cache custody.
func RunObserved(ctx context.Context, argv, env []string, dir string, stdout, stderr io.Writer, started func(int) error) (int, bool, error) {
	return runner.RunObserved(ctx, argv, env, dir, stdout, stderr, started)
}

// RunInteractiveObserved is process.Runner.RunInteractiveObserved under the
// desk's cache custody.
func RunInteractiveObserved(ctx context.Context, argv, env []string, dir string, stdin *os.File, stdout, stderr io.Writer, started func(int) error) (int, bool, error) {
	return runner.RunInteractiveObserved(ctx, argv, env, dir, stdin, stdout, stderr, started)
}

// NotifyContext is process.NotifyContext.
func NotifyContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return process.NotifyContext(ctx)
}
