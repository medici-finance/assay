package gitmaintenance

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellprocess"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

var indexProcess = cellprocess.Run

// RunIndexes is the legacy index-only maintenance operation. Its caller holds
// the shared main-then-temprefs locks and owns attempt cadence. The factory is
// the existing fixed-command diagnostic seam, never a caller-selected task.
func RunIndexes(ctx context.Context, repo string, start func(context.Context, string, ...string) *exec.Cmd) (err error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	if start == nil {
		start = exec.CommandContext
	}
	common, err := commonDir(repo)
	if err != nil {
		return err
	}
	if err := CheckCustody(ctx, repo); err != nil {
		return err
	}
	if err := beginActive(common); err != nil {
		return err
	}
	clean := true
	defer func() {
		if clean {
			err = errors.Join(err, os.Remove(filepath.Join(common, activeName)))
		}
	}()
	// These fixed builtins cannot select gc, prefetch, object deletion or repack
	// through maintenance configuration. Each runs at most once.
	for _, argv := range [][]string{
		{"pack-refs", "--all"},
		{"-c", "core.commitGraph=true", "-c", "commitGraph.changedPaths=false", "commit-graph", "write", "--reachable", "--split", "--no-progress", "--max-new-filters=0"},
	} {
		if err := deskkit.Guard(); err != nil {
			return err
		}
		// Match the actual worktree and effective hook scope. CheckCustody would
		// refuse our own active marker, so only hook policy is repeated here.
		if err := CheckHooksWithStart(ctx, repo, start); err != nil {
			return err
		}
		args := append([]string{"-c", "protocol.allow=never", "-c", "gc.auto=0", "-c", "maintenance.auto=false"}, argv...)
		prototype := start(ctx, "git", args...)
		if prototype == nil || len(prototype.Args) == 0 || prototype.Path == "" {
			return errors.New("maintenance: invalid index command factory")
		}
		launch := append([]string{prototype.Path}, prototype.Args[1:]...)
		var stdout, stderr boundedBuffer
		code, uncertain, runErr := indexProcess(ctx, launch, indexEnv(os.Environ()), repo, &stdout, &stderr)
		if uncertain {
			clean = false
			runErr = errors.Join(runErr, errCleanupUnknown)
		}
		if runErr != nil || code != 0 || stderr.Len() != 0 {
			return fmt.Errorf("maintenance: index task %s could not be checked (exit %d): %w: %s", argv[0], code, errors.Join(runErr, errors.New("index task failed or emitted stderr")), strings.TrimSpace(stderr.String()))
		}
	}
	return nil
}

// Keep the legacy legitimate global/worktree configuration scope while
// stripping ambient Git redirection and denying transport/lazy fetching.
func indexEnv(parent []string) []string {
	var env []string
	for _, pair := range parent {
		if !strings.HasPrefix(strings.ToUpper(pair), "GIT_") {
			env = append(env, pair)
		}
	}
	return append(env, "GIT_NO_LAZY_FETCH=1", "GIT_ALLOW_PROTOCOL=", "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "LC_ALL=C")
}
