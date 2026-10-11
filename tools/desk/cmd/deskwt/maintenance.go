package main

// Maintenance owns repository indexes, not resource lifetimes. In particular it never
// expires objects: Git does not provide a complete reverse inventory of reference clones.
import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

var maintenanceCommand = exec.CommandContext

type maintenanceInventory struct {
	RefNamespaces   map[string]int `json:"refNamespaces"`
	Worktrees       int            `json:"worktrees"`
	LockedWorktrees int            `json:"lockedWorktrees"`
	Objects         string         `json:"objects"`
	ReflogRetention string         `json:"reflogRetentionOverrides"`
}

type maintenanceReport struct {
	CommonDir   string                `json:"commonDir"`
	Status      string                `json:"status"`
	Tasks       []string              `json:"tasks"`
	Reclamation string                `json:"objectReclamation"`
	Before      *maintenanceInventory `json:"before,omitempty"`
	After       *maintenanceInventory `json:"after,omitempty"`
}

// All commands are synchronous Git builtins. Protocol denial plus NO_LAZY_FETCH prevent
// a partial clone from silently turning a local index operation into an authenticated fetch.
// Scrubbing GIT_* prevents ambient GIT_DIR / object-store / config redirection.
func maintenanceGit(ctx context.Context, dir string, args ...string) deskkit.ToolRun {
	env := make([]string, 0, len(os.Environ())+4)
	for _, v := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(strings.SplitN(v, "=", 2)[0]), "GIT_") {
			env = append(env, v)
		}
	}
	env = append(env, "GIT_NO_LAZY_FETCH=1", "GIT_ALLOW_PROTOCOL=", "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "LC_ALL=C")
	prefix := []string{"-c", "protocol.allow=never", "-c", "gc.auto=0", "-c", "maintenance.auto=false"}
	return deskkit.Run(deskkit.ToolCall{Name: "git", Args: append(prefix, args...), Dir: dir, Env: env,
		Start: func(name string, argv ...string) *exec.Cmd {
			cmd := maintenanceCommand(ctx, name, argv...)
			cmd.WaitDelay = time.Second
			return cmd
		}})
}

// Git can exit successfully while omitting unreadable state. These bounded builtins
// run without progress output, so any stderr is uncertainty, never silent success.
func maintenanceRead(ctx context.Context, dir string, args ...string) (string, error) {
	r := maintenanceGit(ctx, dir, args...)
	if r.Failed() || r.Stderr != "" {
		return "", r.Fail(deskkit.ExitUnverifiable, "maintenance inventory: git %s", strings.Join(args, " "))
	}
	return r.Stdout, nil
}

// maintenanceCheckHooks uses Git's effective configuration, including global and
// worktree settings. Never interpret list display prefixes as proof a hook is disabled:
// configured names can contain the same bytes. Refuse every listed relevant entry.
func maintenanceCheckHooks(ctx context.Context, dir string) error {
	hookDir, err := maintenanceRead(ctx, dir, "rev-parse", "--path-format=absolute", "--git-path", "hooks")
	if err != nil {
		return err
	}
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
	r := maintenanceGit(ctx, dir, "hook", "list", "-z", "reference-transaction")
	// This is the sole accepted no-hook outcome on a Git version with native inventory.
	// Unsupported commands, missing configuration, extra warnings and empty success fail closed.
	if r.ExitCode == 1 && r.Stdout == "" && r.Stderr == "warning: no hooks found for event 'reference-transaction'" {
		return nil
	}
	if !r.Failed() && r.Stderr == "" && r.Stdout != "" {
		return deskkit.Refused("repository maintenance requires no configured hooks for its ref transaction; hooks were preserved")
	}
	return r.Fail(deskkit.ExitUnverifiable, "verify maintenance hook inventory (native git hook list required)")
}

func inventoryMaintenance(ctx context.Context, dir string) (*maintenanceInventory, error) {
	inv := &maintenanceInventory{RefNamespaces: map[string]int{}}
	refs, err := maintenanceRead(ctx, dir, "for-each-ref", "--format=%(refname)")
	if err != nil {
		return nil, err
	}
	for _, ref := range strings.Split(refs, "\n") {
		if ref == "" {
			continue
		}
		p := strings.Split(ref, "/")
		if len(p) < 2 || p[0] != "refs" || p[1] == "" {
			return nil, deskkit.Unverifiable("malformed ref inventory", nil)
		}
		inv.RefNamespaces[strings.Join(p[:2], "/")]++
	}
	holders, err := maintenanceRead(ctx, dir, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(holders, "\x00") {
		if strings.HasPrefix(line, "worktree ") {
			inv.Worktrees++
		}
		if line == "locked" || strings.HasPrefix(line, "locked ") {
			inv.LockedWorktrees++
		}
	}
	inv.Objects, err = maintenanceRead(ctx, dir, "count-objects", "-v")
	if err != nil {
		return nil, err
	}
	retention := maintenanceGit(ctx, dir, "config", "--get-regexp", `^gc\.(.*\.)?reflogexpire(unreachable)?$`)
	if (retention.Failed() && retention.ExitCode != 1) || retention.Stderr != "" {
		return nil, retention.Fail(deskkit.ExitUnverifiable, "read reflog retention")
	}
	inv.ReflogRetention = retention.Stdout
	return inv, nil
}

// The lock lives IN the canonical common directory: different worktrees, roles and HOME
// values converge on the same inode. Never unlink it, which would split the lock domain.
// The stamp records attempts, including failed/interrupted work, to bound retry storms.
func maintenanceLock(common string, interval time.Duration) (*os.File, string, error) {
	f, err := openMaintenanceLock(filepath.Join(common, "assay-maintenance.lock"))
	if err != nil {
		return nil, "", deskkit.Unverifiable("open repository maintenance lock", err)
	}
	if err = deskkit.TryLockExclusive(f); err != nil {
		f.Close()
		if errors.Is(err, deskkit.ErrLockBusy) {
			return nil, "held", nil
		}
		return nil, "", deskkit.Unverifiable("lock repository maintenance", err)
	}
	release := func() { _ = deskkit.UnlockFile(f); _ = f.Close() }
	var last time.Time
	if err = json.NewDecoder(f).Decode(&last); err == nil {
		age := time.Since(last)
		if age < 0 {
			release()
			return nil, "", deskkit.Unverifiable("maintenance attempt timestamp is in the future", nil)
		}
		if age < interval {
			release()
			return nil, "recent", nil
		}
	} else {
		info, statErr := f.Stat()
		if statErr != nil || info.Size() != 0 {
			release()
			return nil, "", deskkit.Unverifiable("unreadable maintenance attempt timestamp", err)
		}
	}
	if _, err = f.Seek(0, 0); err == nil {
		err = f.Truncate(0)
	}
	if err == nil {
		err = json.NewEncoder(f).Encode(time.Now().UTC())
	}
	if err == nil {
		err = f.Sync()
	}
	if err != nil {
		release()
		return nil, "", deskkit.Unverifiable("record maintenance attempt", err)
	}
	return f, "run", nil
}

func cmdMaintain(args []string) (err error) {
	audit := auditCtx{verb: "maintain"}
	defer func() { audit.finalize(err) }()
	fs := flag.NewFlagSet("maintain", flag.ContinueOnError)
	repo := fs.String("repo", "", "repository or linked worktree")
	apply := fs.Bool("apply", false, "apply safe index tasks; otherwise read-only plan")
	interval := fs.Duration("min-interval", 6*time.Hour, "minimum time between attempts (at least 1h)")
	timeout := fs.Duration("timeout", 30*time.Second, "total work budget (1s to 5m)")
	if e := fs.Parse(args); e != nil {
		if deskkit.IsHelpRequest(e) {
			return deskkit.ErrHelpRequested
		}
		return deskkit.Refused("bad maintenance flags: " + e.Error())
	}
	if fs.NArg() != 0 || *interval < time.Hour || *timeout < time.Second || *timeout > 5*time.Minute {
		return deskkit.Refused("maintain requires no positional args, min-interval >=1h and timeout between 1s and 5m")
	}
	if *repo == "" {
		*repo, err = getwd()
		if err != nil {
			return deskkit.Unverifiable("resolve current directory", err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	common, e := maintenanceRead(ctx, *repo, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if e != nil {
		return e
	}
	common, e = filepath.EvalSymlinks(common)
	if e != nil {
		return deskkit.Unverifiable("resolve common Git directory", e)
	}
	audit.repo = common
	report := maintenanceReport{CommonDir: common, Status: "plan", Tasks: []string{"pack-refs", "commit-graph"}, Reclamation: "held: reverse borrower inventory unavailable; no object deletion or reflog expiry"}
	defer func() {
		if err != nil {
			report.Status = "could-not-check"
		}
		if e := json.NewEncoder(os.Stdout).Encode(report); e != nil && err == nil {
			err = e
		}
	}()
	if e := maintenanceCheckHooks(ctx, *repo); e != nil {
		return e
	}
	if *apply {
		lock, status, e := maintenanceLock(common, *interval)
		if e != nil {
			return e
		}
		report.Status = status
		if lock == nil {
			audit.successResult = deskkit.ResultNoop
			audit.detail = status
			return nil
		}
		defer func() { _ = deskkit.UnlockFile(lock); _ = lock.Close() }()
	}
	report.Before, err = inventoryMaintenance(ctx, *repo)
	if err != nil {
		return err
	}
	if !*apply {
		audit.detail = "read-only plan"
		return nil
	}
	// These two explicit builtins cannot select gc, prefetch, loose-object deletion or
	// incremental-repack through maintenance configuration. Each runs at most once.
	for _, argv := range [][]string{
		{"pack-refs", "--all"},
		{"-c", "core.commitGraph=true", "-c", "commitGraph.changedPaths=false", "commit-graph", "write", "--reachable", "--split", "--no-progress", "--max-new-filters=0"},
	} {
		if e := deskkit.Guard(); e != nil {
			return e
		}
		// Recheck immediately before each task. The cooperative maintenance lock does
		// not serialize external configuration edits; this is not an atomic sandbox.
		if e := maintenanceCheckHooks(ctx, *repo); e != nil {
			return e
		}
		r := maintenanceGit(ctx, *repo, argv...)
		if r.Failed() || r.Stderr != "" {
			return r.Fail(deskkit.ExitUnverifiable, "offline repository maintenance")
		}
	}
	report.After, err = inventoryMaintenance(ctx, *repo)
	if err != nil {
		return err
	}
	report.Status = "completed"
	audit.detail = fmt.Sprintf("offline indexes complete: worktrees=%d namespaces=%d; reclamation held", report.After.Worktrees, len(report.After.RefNamespaces))
	return nil
}
