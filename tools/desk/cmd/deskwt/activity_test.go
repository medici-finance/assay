package main

// Tests for the prune activity gate (activity.go), `deskwt add --lock-session`, and the
// singleton's repository key.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// ageActivity backdates every piece of activity evidence for the worktree at target —
// its admin entry's HEAD, index, logs/HEAD, ORIG_HEAD and FETCH_HEAD (where present)
// and the worktree root — by d, so only what a test touches afterwards reads as recent.
func ageActivity(t *testing.T, work, target string, d time.Duration) {
	t.Helper()
	old := time.Now().Add(-d)
	admin := filepath.Join(work, ".git", "worktrees", filepath.Base(target))
	paths := []string{admin, target}
	for _, f := range activityFiles {
		paths = append(paths, filepath.Join(admin, f.name))
	}
	for _, p := range paths {
		if err := os.Chtimes(p, old, old); err != nil && !os.IsNotExist(err) {
			t.Fatalf("age %s: %v", p, err)
		}
	}
}

// touchNow refreshes one file's mtime to now, the way git rewriting it would.
func touchNow(t *testing.T, path string) {
	t.Helper()
	now := time.Now()
	if err := os.Chtimes(path, now, now); err != nil {
		t.Fatalf("touch %s: %v", path, err)
	}
}

// An unlocked worktree whose HEAD has become reachable from origin/main passes every
// content gate — and is still HELD while git shows recent activity in it. Once that
// activity is older than --min-idle the ordinary rules remove it.
func TestPruneActivityGateHoldsRecentWorktree(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "active-merged")
	advanceMain(t, work, "first") // HEAD is now below the tip, an ancestor of origin/main
	ageActivity(t, work, target, 3*time.Hour)
	touchNow(t, filepath.Join(work, ".git", "worktrees", filepath.Base(target), "index"))

	rc, errout := runCapErr(t, []string{"prune", "--min-idle", "30m"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d; stderr:\n%s", rc, errout)
	}
	assertExists(t, target)
	if !strings.Contains(errout, "active under 1m ago") {
		t.Fatalf("the hold did not name the activity; stderr:\n%s", errout)
	}

	// Idle past the window: the same tree drains under the unchanged content gates.
	ageActivity(t, work, target, 45*time.Minute)
	rc, errout = runCapErr(t, []string{"prune", "--min-idle", "30m"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d; stderr:\n%s", rc, errout)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("an idle merged worktree survived (err=%v); stderr:\n%s", err, errout)
	}
}

// Each piece of evidence holds on its own: HEAD, logs/HEAD and the worktree root.
func TestPruneActivityGateEachSignal(t *testing.T) {
	for _, c := range []struct {
		name  string
		touch func(admin, target string) string
	}{
		{"HEAD", func(admin, _ string) string { return filepath.Join(admin, "HEAD") }},
		{"logs/HEAD", func(admin, _ string) string { return filepath.Join(admin, "logs", "HEAD") }},
		{"worktree root", func(_, target string) string { return target }},
		// ORIG_HEAD and FETCH_HEAD are written lazily (a merge, reset or rebase; a
		// fetch): the fixture tree has neither, so the case writes the one it tests.
		{"ORIG_HEAD", func(admin, _ string) string { return writeNow(admin, "ORIG_HEAD") }},
		{"FETCH_HEAD", func(admin, _ string) string { return writeNow(admin, "FETCH_HEAD") }},
	} {
		t.Run(c.name, func(t *testing.T) {
			work := newRepo(t)
			withEnv(t, work)
			target := addWorktree(t, "signal")
			advanceMain(t, work, "first")
			ageActivity(t, work, target, 3*time.Hour)
			admin := filepath.Join(work, ".git", "worktrees", filepath.Base(target))
			touchNow(t, c.touch(admin, target))
			rc, errout := runCapErr(t, []string{"prune", "--min-idle", "1h"})
			if rc != deskkit.ExitOK {
				t.Fatalf("prune rc = %d; stderr:\n%s", rc, errout)
			}
			assertExists(t, target)
			if !strings.Contains(errout, "--min-idle window") {
				t.Fatalf("not held by the activity gate; stderr:\n%s", errout)
			}
		})
	}
}

// Unreadable evidence is not idleness: a worktree whose admin HEAD cannot be read is held.
func TestPruneActivityGateUnreadableHolds(t *testing.T) {
	rt := t.TempDir()
	admins := map[string]string{rt: filepath.Join(t.TempDir(), "gone")}
	why := activityHold(admins, nil, rt, time.Hour, time.Now())
	if !strings.HasPrefix(why, "unverifiable worktree activity") {
		t.Fatalf("missing evidence was judged idle: %q", why)
	}
	if why := activityHold(nil, os.ErrPermission, rt, time.Hour, time.Now()); !strings.HasPrefix(why, "unverifiable") {
		t.Fatalf("an unreadable registry was judged idle: %q", why)
	}
	if why := activityHold(map[string]string{}, nil, rt, time.Hour, time.Now()); !strings.HasPrefix(why, "unverifiable") {
		t.Fatalf("a worktree with no admin entry was judged idle: %q", why)
	}
}

// The dead-session arm respects the gate: an unowned, clean, pushed worktree it would reap
// is KEPT while recently active, and reaped once idle.
func TestReapRespectsActivityGate(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target, _ := openPRWorktree(t, work, "reap-active")
	ageActivity(t, work, target, 3*time.Hour)
	touchNow(t, filepath.Join(work, ".git", "worktrees", filepath.Base(target), "HEAD"))

	rc, errout := runCapErr(t, []string{"prune", "--reap-dead-sessions", "--min-idle", "1h", "--dry-run"})
	if rc != deskkit.ExitOK {
		t.Fatalf("dry run rc = %d; stderr:\n%s", rc, errout)
	}
	if !strings.Contains(errout, "KEEP "+resolvePath(target)) || !strings.Contains(errout, "held: active") {
		t.Fatalf("the plan did not KEEP the active tree by the activity gate; stderr:\n%s", errout)
	}
	rc, errout = runCapErr(t, []string{"prune", "--reap-dead-sessions", "--min-idle", "1h"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d; stderr:\n%s", rc, errout)
	}
	assertExists(t, target)

	ageActivity(t, work, target, 2*time.Hour)
	rc, errout = runCapErr(t, []string{"prune", "--reap-dead-sessions", "--min-idle", "1h"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d; stderr:\n%s", rc, errout)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("an idle unowned clean pushed worktree was not reaped (err=%v); stderr:\n%s", err, errout)
	}
}

// --min-idle tunes the gate and can never switch it off; with --branches-only (no worktree
// arm) it is refused as inert.
func TestPruneMinIdleFlagRefusals(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	for _, args := range [][]string{
		{"prune", "--min-idle", "0"},
		{"prune", "--min-idle", "-5m"},
		{"prune", "--min-idle", "soon"},
		{"prune", "--branches-only", "--min-idle", "1h"},
	} {
		if rc := run(args); rc != deskkit.ExitRefused {
			t.Errorf("%v rc = %d, want refused", args, rc)
		}
	}
}

// The compiled default arms the gate on every sweep that names no --min-idle.
func TestPruneActivityGateDefaultOn(t *testing.T) {
	compiled := defaultMinIdle
	if compiled != time.Hour {
		t.Fatalf("compiled --min-idle default = %v, want 1h", compiled)
	}
	work := newRepo(t)
	withEnv(t, work)
	defaultMinIdle = compiled // withEnv zeroes it for the content-gate suite
	target := addWorktree(t, "default-on")
	advanceMain(t, work, "first")
	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d; stderr:\n%s", rc, errout)
	}
	assertExists(t, target)
	if !strings.Contains(errout, "1h0m0s --min-idle window") {
		t.Fatalf("the default sweep did not hold a just-used worktree; stderr:\n%s", errout)
	}
}

// lockReasonOf returns the lock reason git records for the worktree at target.
func lockReasonOf(t *testing.T, work, target string) (string, bool) {
	t.Helper()
	rt := resolvePath(target)
	for _, b := range strings.Split(mustGit(t, work, "worktree", "list", "--porcelain"), "\n\n") {
		if !strings.Contains(b, "worktree "+rt+"\n") && !strings.Contains(b, "worktree "+target+"\n") {
			continue
		}
		for _, line := range strings.Split(b, "\n") {
			if line == "locked" {
				return "", true
			}
			if r, ok := strings.CutPrefix(line, "locked "); ok {
				return r, true
			}
		}
	}
	return "", false
}

// add --lock-session locks the new tree in role-init's reason format, and the lock
// lifecycle judges it like any role-init lock: held while the session's beacon is fresh,
// reclaimable once it is stale.
func TestAddLockSession(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	const name = "locked-one"
	if rc := run([]string{"add", name, "--lock-session", "sess-42"}); rc != deskkit.ExitOK {
		t.Fatalf("add --lock-session rc = %d", rc)
	}
	target := filepath.Join(tmpBaseDir, "tracker-"+name)
	reason, locked := lockReasonOf(t, work, target)
	if !locked || reason != addLockReason("sess-42") {
		t.Fatalf("lock = %v %q, want %q", locked, reason, addLockReason("sess-42"))
	}
	if got := sessionFromLockReason(reason); got != "sess-42" {
		t.Fatalf("the lock reason does not parse back to its session: %q", got)
	}
	now := time.Now()
	plantBeacon(t, "sess-42", now)
	if lv := judgeLock(reason, now, true, 0, rosterBeaconDir(), now); lv.stale {
		t.Fatalf("a fresh session's lock judged stale: %s", lv.why)
	}
	plantBeacon(t, "sess-42", now.Add(-6*time.Hour))
	if lv := judgeLock(reason, now, true, 0, rosterBeaconDir(), now); !lv.stale {
		t.Fatalf("a stale session's lock judged live: %s", lv.why)
	}

	// The default sweep holds it as locked; the reclaim pass retires it once stale.
	advanceMain(t, work, "first")
	rc, errout := runCapErr(t, []string{"prune", "--reclaim-stale-locks"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d; stderr:\n%s", rc, errout)
	}
	if _, still := lockReasonOf(t, work, target); still {
		t.Fatalf("a stale --lock-session lock was not reclaimed; stderr:\n%s", errout)
	}
}

// A session id that could not parse back out of the lock reason — or could traverse the
// roster directory — is refused before anything is created.
func TestAddLockSessionRefusesUnsafeID(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	const name = "unsafe"
	for _, id := range []string{"../x", "a/b", "-x", ".hidden", "a..b"} {
		if rc := run([]string{"add", name, "--lock-session", id}); rc != deskkit.ExitRefused {
			t.Errorf("--lock-session %q rc = %d, want refused", id, rc)
		}
	}
	if _, err := os.Stat(filepath.Join(tmpBaseDir, "tracker-"+name)); !os.IsNotExist(err) {
		t.Fatalf("a refused add left a worktree behind (err=%v)", err)
	}
}

// writeNow writes a lazily-created admin file (ORIG_HEAD, FETCH_HEAD) the way git would
// on first use and returns its path, so touchNow then stamps it as the only recent signal.
func writeNow(admin, name string) string {
	p := filepath.Join(admin, name)
	if err := os.WriteFile(p, []byte("0000000000000000000000000000000000000000\n"), 0o644); err != nil {
		panic(err)
	}
	return p
}

// A FUTURE activity mtime (clock skew, a `touch -d`) is the most recent activity there
// can be: the tree is held, never read as idle for "negative" time.
func TestActivityGateFutureMtimeHolds(t *testing.T) {
	rt := t.TempDir()
	admin := t.TempDir()
	if err := os.WriteFile(filepath.Join(admin, "HEAD"), []byte("ref: refs/heads/x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	future := now.Add(3 * time.Hour)
	for _, p := range []string{rt, filepath.Join(admin, "HEAD")} {
		if err := os.Chtimes(p, future, future); err != nil {
			t.Fatal(err)
		}
	}
	why := activityHold(map[string]string{rt: admin}, nil, rt, time.Hour, now)
	if !strings.Contains(why, "--min-idle window") {
		t.Fatalf("a worktree with future-dated activity was judged idle: %q", why)
	}
}

// The admin registry is keyed on the RESOLVED worktree root: a gitdir recorded through a
// symlinked path still maps to the root prune resolves its candidates to. Keyed on the
// raw path, that worktree would have no admin entry and be held forever.
func TestWorktreeAdminDirsResolvesSymlinkedGitdir(t *testing.T) {
	base := resolvePath(t.TempDir())
	real := filepath.Join(base, "real")
	if err := os.MkdirAll(filepath.Join(real, "wt"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	common := filepath.Join(base, "common")
	admin := filepath.Join(common, "worktrees", "wt")
	if err := os.MkdirAll(admin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(admin, "gitdir"), []byte(filepath.Join(link, "wt", ".git")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := (&pathGuard{commonDir: common}).worktreeAdminDirs()
	if err != nil {
		t.Fatal(err)
	}
	if got[filepath.Join(real, "wt")] != admin {
		t.Fatalf("admin map = %v, want %s keyed on the resolved root %s", got, admin, filepath.Join(real, "wt"))
	}
}

// A gitdir recorded relative to its admin entry (git's worktree.useRelativePaths) maps to
// the worktree root it names. Resolved against the sweep's working directory instead, it
// matches no candidate, so every such worktree is held and prune quietly stops reclaiming.
func TestWorktreeAdminDirsResolvesRelativeGitdir(t *testing.T) {
	base := resolvePath(t.TempDir())
	if err := os.MkdirAll(filepath.Join(base, "wt"), 0o755); err != nil {
		t.Fatal(err)
	}
	common := filepath.Join(base, "main", ".git")
	admin := filepath.Join(common, "worktrees", "wt")
	if err := os.MkdirAll(admin, 0o755); err != nil {
		t.Fatal(err)
	}
	// The exact form git 2.56 writes under -c worktree.useRelativePaths=true.
	if err := os.WriteFile(filepath.Join(admin, "gitdir"), []byte("../../../../wt/.git\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := (&pathGuard{commonDir: common}).worktreeAdminDirs()
	if err != nil {
		t.Fatal(err)
	}
	if got[filepath.Join(base, "wt")] != admin {
		t.Fatalf("admin map = %v, want %s keyed on %s", got, admin, filepath.Join(base, "wt"))
	}
}

// assertAddRolledBack checks that a refused `add --lock-session` left nothing behind:
// no worktree directory and no registration git still lists.
func assertAddRolledBack(t *testing.T, work, target string) {
	t.Helper()
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("the worktree directory survived a failed lock (err=%v)", err)
	}
	out, err := runGit(work, "worktree", "list", "--porcelain")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(out, "\n") {
		if p, ok := strings.CutPrefix(line, "worktree "); ok && resolvePath(p) == resolvePath(target) {
			t.Fatalf("the worktree is still registered after a failed lock:\n%s", out)
		}
	}
}

// A `git worktree lock` that FAILS rolls the new worktree back: the caller asked for a
// session lock, and an unlocked tree is not what it asked for.
func TestAddLockSessionRollsBackWhenLockFails(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	old := worktreeLockFn
	t.Cleanup(func() { worktreeLockFn = old })
	worktreeLockFn = func(string, string, string) error { return errors.New("lock refused (fixture)") }
	const name = "lock-fails"
	rc, errout := runCapErr(t, []string{"add", name, "--lock-session", "sess-42"})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("add with a failing lock rc = %d, want %d; stderr:\n%s", rc, deskkit.ExitUnverifiable, errout)
	}
	if !strings.Contains(errout, "ROLLED BACK") {
		t.Fatalf("the refusal did not report the rollback; stderr:\n%s", errout)
	}
	assertAddRolledBack(t, work, filepath.Join(tmpBaseDir, "tracker-"+name))
}

// A lock that REPORTS success but is not there when read back is refused the same way:
// git's exit status alone is never taken as proof, and the tree is rolled back.
func TestAddLockSessionReadsTheLockBack(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	old := worktreeLockFn
	t.Cleanup(func() { worktreeLockFn = old })
	worktreeLockFn = func(string, string, string) error { return nil } // "succeeds", locks nothing
	const name = "lock-silent"
	rc, errout := runCapErr(t, []string{"add", name, "--lock-session", "sess-42"})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("add with an unverified lock rc = %d, want %d; stderr:\n%s", rc, deskkit.ExitUnverifiable, errout)
	}
	if !strings.Contains(errout, "reported success but") {
		t.Fatalf("the read-back mismatch was not named; stderr:\n%s", errout)
	}
	assertAddRolledBack(t, work, filepath.Join(tmpBaseDir, "tracker-"+name))
}
