package gitmaintenance

// Hook custody is the approved #2516 policy: inventory effective Git settings
// and refuse relevant hooks; never disable them or treat a display prefix as
// proof a configured entry is inactive. The check is cooperative, not an atomic
// sandbox against concurrent configuration edits.

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// CheckHooks includes effective global and worktree settings while dropping
// inherited GIT_* command/config injection. It writes neither config nor locks.
func CheckHooks(ctx context.Context, repo string) error {
	return CheckHooksWithStart(ctx, repo, exec.CommandContext)
}

// CheckHooksWithStart preserves the legacy command's diagnostic/test launch
// factory. The factory receives only the two fixed inspection operations below;
// callers cannot supply a verb, flags, event or configuration override.
func CheckHooksWithStart(ctx context.Context, repo string, start func(context.Context, string, ...string) *exec.Cmd) error {
	return checkHooks(ctx, repo, "", start)
}

// Native tasks use the common directory and its main-worktree configuration.
// Inspect that exact scope as well as the caller's linked-worktree scope.
func checkCommonHooks(ctx context.Context, common string) error {
	return checkHooks(ctx, filepath.Dir(common), common, exec.CommandContext)
}

func checkHooks(ctx context.Context, dir, common string, start func(context.Context, string, ...string) *exec.Cmd) error {
	r := hookRead(ctx, dir, common, "path", start)
	if r.Failed() || r.Stderr != "" {
		return r.Fail(deskkit.ExitUnverifiable, "maintenance inventory: resolve effective hook directory")
	}
	hookDir := r.Stdout
	f, err := openMaintenanceDirectory(hookDir)
	if err != nil {
		return deskkit.Unverifiable("read maintenance hook directory", err)
	}
	_, readErr := f.ReadDir(1)
	closeErr := f.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return deskkit.Unverifiable("read maintenance hook directory", readErr)
	}
	if closeErr != nil {
		return deskkit.Unverifiable("close maintenance hook directory", closeErr)
	}
	path := filepath.Join(hookDir, "reference-transaction")
	if _, err := os.Lstat(path); err == nil {
		if _, err := os.Stat(path); err != nil {
			return deskkit.Unverifiable("resolve maintenance hook", err)
		}
	} else if !os.IsNotExist(err) {
		return deskkit.Unverifiable("inspect maintenance hook", err)
	}
	r = hookRead(ctx, dir, common, "list", start)
	if r.ExitCode == 1 && r.Stdout == "" && r.Stderr == "warning: no hooks found for event 'reference-transaction'" {
		return nil
	}
	if !r.Failed() && r.Stderr == "" && r.Stdout != "" {
		return deskkit.Refused("repository maintenance requires no configured hooks for its ref transaction; hooks were preserved")
	}
	return r.Fail(deskkit.ExitUnverifiable, "verify maintenance hook inventory (native git hook list required)")
}

func hookRead(ctx context.Context, dir, common, operation string, start func(context.Context, string, ...string) *exec.Cmd) deskkit.ToolRun {
	args := []string{"-c", "protocol.allow=never", "-c", "gc.auto=0", "-c", "maintenance.auto=false"}
	if common != "" {
		args = append(args, "--git-dir="+common)
	}
	switch operation {
	case "path":
		args = append(args, "rev-parse", "--path-format=absolute", "--git-path", "hooks")
	case "list":
		args = append(args, "hook", "list", "-z", "reference-transaction")
	default:
		panic("unaudited maintenance hook inspection")
	}
	env := make([]string, 0, len(os.Environ())+4)
	for _, value := range os.Environ() {
		key, _, _ := strings.Cut(value, "=")
		if !strings.HasPrefix(strings.ToUpper(key), "GIT_") {
			env = append(env, value)
		}
	}
	env = append(env, "GIT_NO_LAZY_FETCH=1", "GIT_ALLOW_PROTOCOL=", "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "LC_ALL=C")
	return deskkit.Run(deskkit.ToolCall{Name: "git", Args: args, Dir: dir, Env: env,
		Start: func(name string, argv ...string) *exec.Cmd {
			cmd := start(ctx, name, argv...)
			cmd.WaitDelay = time.Second
			return cmd
		}})
}
