package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellcache"
	"github.com/medici-finance/assay/tools/desk/internal/cellcadence"
	"github.com/medici-finance/assay/tools/desk/internal/cellstop"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

type resetWorktree struct{ Path, Branch string }
type resetRepo struct {
	Root, Common string
	Worktrees    []resetWorktree
}
type resetPlan struct {
	Cell              string          `json:"cell"`
	Hard              bool            `json:"hard"`
	Repositories      []resetRepo     `json:"repositories"`
	Worktrees         []resetWorktree `json:"worktrees"`
	DiscardMessages   bool            `json:"discard_messages"`
	WorktreeDirectory string          `json:"cell_worktree_directory"`
}

func pathInside(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
func listResetWorktrees(root string) ([]resetWorktree, error) {
	cmd := exec.Command("git", "-C", root, "worktree", "list", "--porcelain", "-z")
	b, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list worktrees in %s: %w", root, err)
	}
	var result []resetWorktree
	for _, field := range strings.Split(string(b), "\x00") {
		if strings.HasPrefix(field, "worktree ") {
			result = append(result, resetWorktree{Path: strings.TrimPrefix(field, "worktree ")})
		}
		if strings.HasPrefix(field, "branch ") && len(result) > 0 {
			result[len(result)-1].Branch = strings.TrimPrefix(field, "branch refs/heads/")
		}
		if field == "bare" {
			return nil, fmt.Errorf("reset does not accept a bare repository: %s", root)
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("no primary worktree in %s", root)
	}
	return result, nil
}
func (c *Cell) planReset(hard bool) (resetPlan, error) {
	p := resetPlan{Cell: c.Name, Hard: hard, DiscardMessages: hard, WorktreeDirectory: filepath.Join(c.Dir, "worktrees")}
	roots := rootEntries(c.Env.Get("CELL_ROOTS"))
	if len(roots) == 0 && c.Repo != "" {
		roots = append(roots, [2]string{"primary", c.Repo})
	}
	seen := map[string]bool{}
	for _, entry := range roots {
		root, err := filepath.EvalSymlinks(entry[1])
		if err != nil {
			return p, fmt.Errorf("reset root %s: %w", entry[0], err)
		}
		common, err := gitOut(root, "rev-parse", "--path-format=absolute", "--git-common-dir")
		if err != nil || common == "" {
			return p, fmt.Errorf("reset root is not a Git checkout: %s", root)
		}
		common, err = filepath.EvalSymlinks(common)
		if err != nil {
			return p, err
		}
		if seen[common] {
			continue
		}
		seen[common] = true
		worktrees, err := listResetWorktrees(root)
		if err != nil {
			return p, err
		}
		if hard && filepath.Clean(worktrees[0].Path) != root {
			return p, fmt.Errorf("hard reset root must be its primary checkout: %s (primary %s)", root, worktrees[0].Path)
		}
		r := resetRepo{Root: worktrees[0].Path, Common: common, Worktrees: worktrees}
		if hard && pathInside(c.Dir, r.Root) {
			return p, fmt.Errorf("hard reset requires cell configuration outside repository %s", r.Root)
		}
		p.Repositories = append(p.Repositories, r)
		for i, wt := range worktrees {
			if i == 0 {
				continue
			}
			if hard || pathInside(wt.Path, filepath.Join(c.Dir, "worktrees")) {
				p.Worktrees = append(p.Worktrees, wt)
			}
		}
	}
	if len(p.Repositories) == 0 {
		return p, fmt.Errorf("reset requires CELL_REPO or CELL_ROOTS")
	}
	return p, nil
}

func resetGit(root string, args ...string) error {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s in %s: %w: %s", args[0], root, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func acquireCellLock(c *Cell, name string) (*os.File, error) {
	path := filepath.Join(c.Dir, name)
	if _, err := os.Lstat(path); err == nil {
		if err = privateCommsFile(path); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = deskkit.TryLockExclusive(f); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

// Reset stages quiescence before data removal. Its marker blocks new cellctl
// launches; held custody locks stay on their original inodes until released.
func (c *Cell) resetCell(plan resetPlan) (err error) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return fmt.Errorf("reset currently requires macOS or Linux")
	}
	lock, err := acquireCellLock(c, ".reset-owner.lock")
	if err != nil {
		return fmt.Errorf("reset ownership: %w", err)
	}
	defer func() { deskkit.UnlockFile(lock); lock.Close() }()
	marker := filepath.Join(c.Dir, ".reset-in-progress")
	if _, e := os.Lstat(marker); e == nil {
		if e = privateCommsFile(marker); e != nil {
			return e
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	if err = os.WriteFile(marker, []byte("cell maintenance in progress\n"), 0600); err != nil {
		return err
	}
	defer func() {
		if err == nil {
			err = os.Remove(marker)
		}
	}()
	cfg, err := c.preflightReset(plan)
	if err != nil {
		return err
	}
	// All fetches must succeed before any repository is destructively changed.
	if plan.Hard {
		for _, repo := range plan.Repositories {
			if err = resetGit(repo.Root, "fetch", "--no-tags", "origin", "+refs/heads/main:refs/remotes/origin/main"); err != nil {
				return fmt.Errorf("hard reset prefetch: %w", err)
			}
		}
	}
	policy, err := c.maintenanceCachePolicy()
	if err != nil {
		return err
	}
	if err = c.requestResetStop(); err != nil {
		return err
	}
	scope := cellstop.Scope{Cell: c.Name, Directory: c.Dir, Roots: c.Env.Get("CELL_ROOTS"), CacheRoot: policy.Root}
	scope.Worktrees = append(scope.Worktrees, plan.WorktreeDirectory)
	if plan.Hard {
		for _, repo := range plan.Repositories {
			scope.Worktrees = append(scope.Worktrees, repo.Root)
		}
	}
	for _, wt := range plan.Worktrees {
		scope.Worktrees = append(scope.Worktrees, wt.Path)
	}
	if err = cellstop.Stop(context.Background(), scope); err != nil {
		return fmt.Errorf("reset stop: %w", err)
	}
	admission, e := waitResetAdmission(c)
	if e != nil {
		return e
	}
	defer func() { deskkit.UnlockFile(admission); admission.Close() }()
	// The recorded comms/cockpit owner knows how to close its own surface. A
	// free lease makes stale checkpoint recovery safe after the process sweep.
	if surface, e := c.readResetCommsSurface(); e != nil {
		return e
	} else if surface != nil {
		if surface.Handle == "" {
			return fmt.Errorf("reset cannot identify the pending comms surface; close the recorded cell workspace first")
		}
		if e = c.closeCommsSurface(surface); e != nil {
			return fmt.Errorf("reset comms surface: %w", e)
		}
	}
	if err = c.closeHerdrWorkspace(); err != nil {
		return err
	}
	var leases []*cellcadence.Lease
	defer func() {
		for _, l := range leases {
			l.Close()
		}
	}()
	for _, role := range append(append([]string{}, knownRoles...), "comms") {
		dir := filepath.Join(c.Dir, "run", "cadence", role)
		if role == "comms" {
			dir = c.commsDir()
		}
		if _, e := os.Stat(dir); os.IsNotExist(e) {
			continue
		}
		l, e := cellcadence.Acquire(dir)
		deadline := time.Now().Add(10 * time.Second)
		for errors.Is(e, cellcadence.ErrBusy) && time.Now().Before(deadline) {
			time.Sleep(50 * time.Millisecond)
			l, e = cellcadence.Acquire(dir)
		}
		if e != nil {
			return fmt.Errorf("reset lease %s: %w", role, e)
		}
		leases = append(leases, l)
		for _, name := range []string{"checkpoint.json", "STOP", "owner-mode"} {
			if e = os.Remove(filepath.Join(dir, name)); e != nil && !os.IsNotExist(e) {
				return e
			}
		}
	}
	if _, e := os.Stat(policy.Root); e == nil {
		if e = cellcache.Recover(*policy, true); e != nil {
			return e
		}
		if _, e = cellcache.Wipe(*policy, false); e != nil {
			return e
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	for _, repo := range plan.Repositories {
		for _, wt := range repo.Worktrees[1:] {
			if plan.Hard || pathInside(wt.Path, filepath.Join(c.Dir, "worktrees")) {
				if e := resetGit(repo.Root, "worktree", "remove", "--force", "--force", wt.Path); e != nil {
					return e
				}
			}
		}
		if e := os.RemoveAll(filepath.Join(repo.Common, "cellctl-fetch.lock")); e != nil {
			return e
		}
		if plan.Hard {
			if e := resetGit(repo.Root, "checkout", "--force", "-B", "main", "origin/main"); e != nil {
				return e
			}
			if e := resetGit(repo.Root, "clean", "-ffdx"); e != nil {
				return e
			}
			branches, e := gitOut(repo.Root, "for-each-ref", "--format=%(refname:short)", "refs/heads/")
			if e != nil {
				return e
			}
			for _, branch := range strings.Split(branches, "\n") {
				if branch != "" && branch != "main" {
					if e = resetGit(repo.Root, "branch", "-D", branch); e != nil {
						return e
					}
				}
			}
		}
	}
	// Unregistered residue belongs to the same cell worktree directory. Git
	// registrations above were removed through Git before clearing that residue.
	if fi, e := os.Lstat(plan.WorktreeDirectory); e == nil {
		if fi.Mode()&os.ModeSymlink != 0 || !fi.IsDir() {
			return fmt.Errorf("cell worktree root must be a real directory")
		}
		if e = os.RemoveAll(plan.WorktreeDirectory); e != nil {
			return e
		}
		if e = os.MkdirAll(plan.WorktreeDirectory, fi.Mode().Perm()); e != nil {
			return e
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	if plan.Hard {
		var e error
		if cfg != nil {
			for _, key := range append([]string{cfg.TrustStore, c.Env.Get("CELL_COMMS_CONFIG")}, mapValues(cfg.SigningKeys)...) {
				if pathInside(key, cfg.QueueDir) {
					return fmt.Errorf("queue directory contains retained configuration or credentials")
				}
			}
			if _, e = os.Stat(cfg.QueueDir); e == nil {
				if e = privateCommsDir(cfg.QueueDir); e != nil {
					return e
				}
				if e = os.RemoveAll(cfg.QueueDir); e != nil {
					return e
				}
				if e = os.MkdirAll(cfg.QueueDir, 0700); e != nil {
					return e
				}
			} else if !os.IsNotExist(e) {
				return e
			}
		}
	}
	if cfg != nil {
		if fi, e := os.Lstat(cfg.Socket); e == nil {
			if fi.Mode()&os.ModeSocket == 0 {
				return fmt.Errorf("reset refuses a non-socket comms endpoint")
			}
			if e = os.Remove(cfg.Socket); e != nil {
				return e
			}
		} else if !os.IsNotExist(e) {
			return e
		}
	}
	for _, name := range []string{filepath.Join(c.Dir, "run", "lock.d"), filepath.Join(c.commsLaunchDir(), "surface.json")} {
		if e := os.RemoveAll(name); e != nil {
			return e
		}
	}
	return nil
}

func mapValues(m map[string]string) []string {
	var r []string
	for _, v := range m {
		r = append(r, v)
	}
	return r
}

func (c *Cell) cockpitWantValue() string {
	want, src := c.cockpitWant("")
	return c.resolveCockpit(want, src).Cockpit
}

func cmdReset(cell string, hard bool) {
	c := loadCell(cell)
	if c.Kind != "house" {
		die("reset currently supports house cells only")
	}
	plan, err := c.planReset(hard)
	if err != nil {
		die("reset plan: %v", err)
	}
	b, _ := json.MarshalIndent(plan, "", "  ")
	fmt.Println(string(b))
	if c.Env.Get("DRY_RUN") == "1" {
		fmt.Println("[dry-run] reset: nothing changed")
		return
	}
	if err = c.resetCell(plan); err != nil {
		die("reset: %v; cell remains stopped; rerun reset after resolving the error", err)
	}
	fmt.Printf("[reset] %s complete (hard=%t); configuration and keys preserved\n", cell, hard)
}

// Reject dangerous layouts before cache, messages or Git registrations change.
func (c *Cell) preflightReset(plan resetPlan) (*deskCommsConfig, error) {
	cfg, err := c.loadDeskComms()
	if err != nil {
		return nil, err
	}
	policy, err := c.maintenanceCachePolicy()
	if err != nil {
		return nil, err
	}
	if fi, e := os.Lstat(plan.WorktreeDirectory); e == nil {
		if fi.Mode()&os.ModeSymlink != 0 || !fi.IsDir() {
			return nil, fmt.Errorf("cell worktree root must be a real directory")
		}
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	retained := []string{c.Dir, policy.Root}
	if cfg != nil {
		retained = append(retained, c.Env.Get("CELL_COMMS_CONFIG"), cfg.TrustStore)
		if !plan.Hard {
			retained = append(retained, cfg.QueueDir)
		}
		retained = append(retained, mapValues(cfg.SigningKeys)...)
		if _, err = os.Lstat(cfg.QueueDir); err == nil {
			if err = privateCommsDir(cfg.QueueDir); err != nil {
				return nil, err
			}
		} else if !os.IsNotExist(err) {
			return nil, err
		}
		if fi, e := os.Lstat(cfg.Socket); e == nil {
			if fi.Mode()&os.ModeSocket == 0 {
				return nil, fmt.Errorf("reset refuses a non-socket comms endpoint")
			}
		} else if !os.IsNotExist(e) {
			return nil, e
		}
		if plan.Hard {
			for _, path := range retained {
				if pathInside(path, cfg.QueueDir) {
					return nil, fmt.Errorf("queue contains retained cell configuration or credentials")
				}
			}
		}
	}
	if plan.Hard {
		for _, repo := range plan.Repositories {
			for _, path := range retained {
				if pathInside(path, repo.Root) {
					return nil, fmt.Errorf("hard reset would discard retained configuration in %s", repo.Root)
				}
			}
			for _, other := range plan.Repositories {
				if repo.Root != other.Root && pathInside(other.Root, repo.Root) {
					return nil, fmt.Errorf("hard reset requires non-nested repository roots")
				}
			}
			if cfg != nil && pathInside(cfg.QueueDir, repo.Root) {
				return nil, fmt.Errorf("hard reset requires comms storage outside repository roots")
			}
		}
	}
	for _, path := range retained {
		if pathInside(path, plan.WorktreeDirectory) {
			return nil, fmt.Errorf("cell worktree root contains retained configuration")
		}
	}
	for _, wt := range plan.Worktrees {
		for _, path := range retained {
			if pathInside(path, wt.Path) {
				return nil, fmt.Errorf("worktree removal would discard retained cell configuration")
			}
		}
	}
	return cfg, nil
}

func (c *Cell) readResetCommsSurface() (*commsSurface, error) {
	if _, err := os.Lstat(c.commsLaunchDir()); os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return c.readCommsSurface()
}

// Idle supervisors have no attributed child to signal. Stop their clocks before
// sweeping orphan consumers, and let their original leases close normally.
func (c *Cell) requestResetStop() error {
	dirs := []string{c.commsDir()}
	for _, role := range knownRoles {
		dirs = append(dirs, c.cadenceDir(role))
	}
	for _, dir := range dirs {
		if _, err := os.Lstat(dir); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		}
		if err := privateCommsDir(dir); err != nil {
			return err
		}
		f, err := os.OpenFile(filepath.Join(dir, "STOP"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil && !os.IsExist(err) {
			return err
		}
		if f != nil {
			if err = f.Close(); err != nil {
				return err
			}
		}
	}
	return nil
}
