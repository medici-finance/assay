//go:build darwin || linux

package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellcache"
	"github.com/medici-finance/assay/tools/desk/internal/cellcadence"
)

func TestResetRetainsCredentialsBeforeCleanup(t *testing.T) {
	for _, placement := range []string{"cache-build-key", "cache-mod-key", "soft-login", "hard-login", "reader-link", "queue-login", "queue-reader-link", "queue-claude", "hard-provider"} {
		t.Run(placement, func(t *testing.T) {
			c, cfg, manifest := commsFixture(t)
			if err := os.MkdirAll(c.Dir, 0700); err != nil {
				t.Fatal(err)
			}
			base := t.TempDir()
			root := resetFixtureRepo(t, base, "repo")
			c.Repo = root
			c.Env.Put("CELL_ROOTS", "example/repo="+root)
			hard := strings.HasPrefix(placement, "hard-") || strings.HasPrefix(placement, "queue-")
			var credential string
			switch placement {
			case "cache-build-key", "cache-mod-key":
				policy, err := c.maintenanceCachePolicy()
				if err != nil {
					t.Fatal(err)
				}
				kind := strings.TrimSuffix(strings.TrimPrefix(placement, "cache-"), "-key")
				credential = filepath.Join(policy.Root, kind, "signing.key")
				cfg.SigningKeys["worker-desk"] = credential
			case "soft-login", "reader-link":
				wt := filepath.Join(c.Dir, "worktrees", "login")
				resetFixtureGit(t, root, "worktree", "add", "-q", "-b", "login", wt, "main")
				credential = filepath.Join(wt, "codex", "auth.json")
			case "hard-login":
				credential = filepath.Join(root, "codex", "auth.json")
			case "queue-login", "queue-reader-link":
				credential = filepath.Join(cfg.QueueDir, "codex", "auth.json")
			case "queue-claude":
				credential = filepath.Join(cfg.QueueDir, "claude", "credentials.json")
				cfg.ClaudeConfigDir = filepath.Dir(credential)
			case "hard-provider":
				credential = filepath.Join(root, "providers.json")
				c.Env.Put("CELL_PROVIDER_DEFAULTS", credential)
			}
			if err := os.MkdirAll(filepath.Dir(credential), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(credential, []byte("nonsecret fixture\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if strings.HasSuffix(placement, "login") {
				c.Env.Put("CODEX_HOME", filepath.Dir(credential))
			}
			if strings.HasSuffix(placement, "reader-link") {
				readerHome := filepath.Join(c.Dir, "comms", "selected-reader")
				if err := os.MkdirAll(readerHome, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(credential, filepath.Join(readerHome, "auth.json")); err != nil {
					t.Fatal(err)
				}
				cfg.Decider, _ = json.Marshal(map[string]any{"cmd": []string{"cellctl", "comms-reader", "--home", readerHome}, "model": "fixture", "pin": "1", "isolate": true, "contained": true})
			}
			writeCommsFixture(t, manifest, cfg)
			plan, err := c.planReset(hard)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = c.preflightReset(plan); err == nil {
				t.Fatal("preflight accepted a retained credential/configuration in disposable storage")
			}
			if b, err := os.ReadFile(credential); err != nil || string(b) != "nonsecret fixture\n" {
				t.Fatal("preflight changed retained fixture", err)
			}
			if _, err := os.Stat(filepath.Join(c.cadenceDir("worker-desk"), "STOP")); !os.IsNotExist(err) {
				t.Fatal("preflight stopped service before refusing layout", err)
			}
		})
	}
}

func resetFixtureGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "-c", "core.hooksPath=/dev/null"}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("fixture git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
func resetFixtureRepo(t *testing.T, base, name string) string {
	t.Helper()
	root := filepath.Join(base, name)
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	resetFixtureGit(t, root, "init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(root, "tracked"), []byte("original\n"), 0600)
	resetFixtureGit(t, root, "add", "tracked")
	resetFixtureGit(t, root, "commit", "-qm", "fixture")
	remote := filepath.Join(base, name+"-remote")
	resetFixtureGit(t, base, "clone", "-q", "--bare", root, remote)
	resetFixtureGit(t, root, "remote", "add", "origin", remote)
	resetFixtureGit(t, root, "fetch", "-q", "origin")
	return root
}
func TestResetSoftAndHard(t *testing.T) {
	for _, hard := range []bool{false, true} {
		t.Run(map[bool]string{false: "soft", true: "hard"}[hard], func(t *testing.T) {
			c, cfg, _ := commsFixture(t)
			base := t.TempDir()
			roots := []string{resetFixtureRepo(t, base, "one"), resetFixtureRepo(t, base, "two")}
			c.Repo = roots[0]
			c.Env.Put("CELL_ROOTS", "example/one="+roots[0]+",example/two="+roots[1])
			c.Env.Put("CELL_GO_CACHE", "on")
			c.Env.Put("CELL_GO_CACHE_MIN_FREE", "0")
			os.MkdirAll(cfg.QueueDir, 0700)
			message := filepath.Join(cfg.QueueDir, "queued")
			os.WriteFile(message, []byte("keep"), 0600)
			for i, root := range roots {
				name := []string{"one", "two"}[i]
				wt := filepath.Join(c.Dir, "worktrees", name)
				resetFixtureGit(t, root, "worktree", "add", "-q", "-b", "worker", wt, "main")
				resetFixtureGit(t, root, "worktree", "lock", wt)
				resetFixtureGit(t, root, "branch", "keep-local")
				os.WriteFile(filepath.Join(root, "tracked"), []byte("dirty\n"), 0600)
				os.WriteFile(filepath.Join(root, "untracked"), []byte("dirty"), 0600)
			}
			residue := filepath.Join(c.Dir, "worktrees", "unregistered", "residue")
			os.MkdirAll(filepath.Dir(residue), 0700)
			os.WriteFile(residue, []byte("disposable"), 0600)
			foreign := filepath.Join(base, "foreign-worktree")
			resetFixtureGit(t, roots[0], "worktree", "add", "-q", "-b", "foreign", foreign, "main")
			policy, err := cellcache.Resolve(c.Dir, c.Env.Get)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = cellcache.Check(*policy, false); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(policy.Root, "build"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(policy.Root, "build", "disposable"), []byte("cache"), 0600); err != nil {
				t.Fatal(err)
			}
			p, err := c.planReset(hard)
			if err != nil {
				t.Fatal(err)
			}
			if err = c.resetCell(p); err != nil {
				t.Fatal(err)
			}
			if _, err = os.Stat(residue); !os.IsNotExist(err) {
				t.Fatal("unregistered worktree residue retained")
			}
			for _, root := range roots {
				if _, err = os.Stat(filepath.Join(c.Dir, "worktrees", filepath.Base(root))); !os.IsNotExist(err) {
					t.Fatal("cell worktree retained", err)
				}
				branches := resetFixtureGit(t, root, "for-each-ref", "--format=%(refname:short)", "refs/heads/")
				if hard && branches != "main" {
					t.Fatalf("hard branches %q", branches)
				}
				if !hard && !strings.Contains(branches, "keep-local") {
					t.Fatal("soft lost branch")
				}
				b, _ := os.ReadFile(filepath.Join(root, "tracked"))
				if hard && string(b) != "original\n" {
					t.Fatal("hard didn't refresh checkout")
				}
				if !hard && string(b) != "dirty\n" {
					t.Fatal("soft modified primary checkout")
				}
			}
			if _, err = os.Stat(foreign); hard && !os.IsNotExist(err) || !hard && err != nil {
				t.Fatal("foreign worktree scope", err)
			}
			if _, err = os.Stat(message); hard && !os.IsNotExist(err) || !hard && err != nil {
				t.Fatal("message scope", err)
			}
			if _, err = os.Stat(filepath.Join(policy.Root, "build", "disposable")); !os.IsNotExist(err) {
				t.Fatal("cache retained")
			}
			for _, key := range cfg.SigningKeys {
				if _, err = os.Stat(key); err != nil {
					t.Fatal("key lost", err)
				}
			}
			if _, err = os.Stat(filepath.Join(c.Dir, ".reset-in-progress")); !os.IsNotExist(err) {
				t.Fatal("successful reset left marker")
			}
		})
	}
}
func TestResetPrefetchFailurePreservesWork(t *testing.T) {
	c, _, _ := commsFixture(t)
	base := t.TempDir()
	root := resetFixtureRepo(t, base, "one")
	c.Repo = root
	c.Env.Put("CELL_ROOTS", "example/one="+root)
	resetFixtureGit(t, root, "remote", "set-url", "origin", filepath.Join(base, "missing"))
	wt := filepath.Join(c.Dir, "worktrees", "keep")
	resetFixtureGit(t, root, "worktree", "add", "-q", "-b", "keep", wt, "main")
	p, err := c.planReset(true)
	if err != nil {
		t.Fatal(err)
	}
	if err = c.resetCell(p); err == nil {
		t.Fatal("failed fetch accepted")
	}
	if !strings.Contains(err.Error(), "stage=prefetch") || !strings.Contains(err.Error(), "quiescence=not established") {
		t.Fatal("prefetch failure claims a stopped cell", err)
	}
	if _, err = os.Stat(wt); err != nil {
		t.Fatal("worktree removed before failed fetch")
	}
}

func TestResetHardUsesUnambiguousRemoteRef(t *testing.T) {
	c, _, _ := commsFixture(t)
	if err := os.MkdirAll(c.Dir, 0700); err != nil {
		t.Fatal(err)
	}
	root := resetFixtureRepo(t, t.TempDir(), "repo")
	c.Repo = root
	c.Env.Put("CELL_ROOTS", "example/repo="+root)
	resetFixtureGit(t, root, "checkout", "-qb", "origin/main")
	os.WriteFile(filepath.Join(root, "tracked"), []byte("shadow branch\n"), 0600)
	resetFixtureGit(t, root, "commit", "-qam", "shadow remote name")
	plan, err := c.planReset(true)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.resetCell(plan); err != nil {
		t.Fatal(err)
	}
	if got := resetFixtureGit(t, root, "rev-parse", "HEAD"); got != resetFixtureGit(t, root, "rev-parse", "refs/remotes/origin/main") {
		t.Fatal("reset selected shadow branch")
	}
}

func TestResetPreservesSharedFetchLock(t *testing.T) {
	for _, hard := range []bool{false, true} {
		t.Run(map[bool]string{false: "soft", true: "hard"}[hard], func(t *testing.T) {
			c, _, _ := commsFixture(t)
			os.MkdirAll(c.Dir, 0700)
			root := resetFixtureRepo(t, t.TempDir(), "repo")
			c.Repo = root
			c.Env.Put("CELL_ROOTS", "example/repo="+root)
			lock := filepath.Join(root, ".git", "cellctl-fetch.lock")
			os.Mkdir(lock, 0700)
			sentinel := filepath.Join(lock, "peer-owner")
			os.WriteFile(sentinel, []byte("peer"), 0600)
			plan, err := c.planReset(hard)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := c.preflightReset(plan); err == nil {
				t.Fatal("reset accepts unknown shared fetch-lock owner")
			}
			if hard {
				if err := resetFetchMain(plan.Repositories[0]); err == nil {
					t.Fatal("prefetch entered a peer's shared lock")
				}
			}
			if b, err := os.ReadFile(sentinel); err != nil || string(b) != "peer" {
				t.Fatal("shared fetch lock changed", err)
			}
		})
	}
}
func TestResetPlanRefusesConfigInsideWorktree(t *testing.T) {
	base := t.TempDir()
	root := resetFixtureRepo(t, base, "one")
	c := &Cell{Name: "fixture", Dir: filepath.Join(root, "cell"), Repo: root, Env: envWith(map[string]string{"CELL_ROOTS": "example/one=" + root})}
	os.Mkdir(c.Dir, 0700)
	if _, err := c.planReset(true); err == nil {
		t.Fatal("hard accepts nested cell")
	}
}

func TestResetDryRunAndAdmission(t *testing.T) {
	f := newCLIWorld(t, "")
	resetFixtureGit(t, f.repoDir, "symbolic-ref", "HEAD", "refs/heads/main")
	os.WriteFile(filepath.Join(f.repoDir, "tracked"), []byte("fixture"), 0600)
	resetFixtureGit(t, f.repoDir, "add", "tracked")
	resetFixtureGit(t, f.repoDir, "commit", "-qm", "fixture")
	before := snapshotTree(t, f.root)
	r := f.run(t, []string{"DRY_RUN=1"}, "reset", "example", "--hard")
	if r.Code != 0 {
		t.Fatalf("dry reset: %+v", r)
	}
	if after := snapshotTree(t, f.root); after != before {
		t.Fatal("dry reset mutated fixture")
	}
	c := &Cell{Name: "example", Dir: f.cellDir, Env: envWith(map[string]string{})}
	maintenanceCommand = "up"
	defer func() { maintenanceCommand = ""; releaseMaintenanceAdmissions() }()
	if err := holdMaintenanceAdmission(c); err != nil {
		t.Fatal(err)
	}
	if l, err := acquireCellLock(c, ".reset.lock"); err == nil {
		l.Close()
		t.Fatal("reset entered while launch held admission")
	}
	releaseMaintenanceAdmissions()
	l, err := acquireCellLock(c, ".reset.lock")
	if err != nil {
		t.Fatal(err)
	}
	l.Close()
}

func TestResetStopsIdleCadenceOwner(t *testing.T) {
	c, _, _ := commsFixture(t)
	base := t.TempDir()
	root := resetFixtureRepo(t, base, "one")
	c.Repo = root
	c.Env.Put("CELL_ROOTS", "example/one="+root)
	l, err := cellcadence.Acquire(c.cadenceDir("worker-desk"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		err := l.Run(ctx, cellcadence.Config{Cell: c.Name, Role: "worker-desk", Interval: time.Hour, Budget: time.Minute, Heartbeat: 20 * time.Millisecond, Guard: func() error { return c.cadenceGuard("worker-desk") }}, func(context.Context) cellcadence.Result {
			close(started)
			return cellcadence.Result{Outcome: "noop", Summary: "fixture noop"}
		})
		l.Close()
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("cadence fixture did not start")
	}
	p, err := c.planReset(false)
	if err != nil {
		t.Fatal(err)
	}
	if err = c.resetCell(p); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("idle cadence owner survived reset")
	}
	if _, err := cellcadence.Read(c.cadenceDir("worker-desk")); err != cellcadence.ErrNoState {
		t.Fatal("cadence state retained", err)
	}
}
