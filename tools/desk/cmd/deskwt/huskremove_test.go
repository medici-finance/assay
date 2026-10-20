package main

// huskremove_test.go — `deskwt remove` on a registered worktree with no git link left (#1905).
//
// The defect: remove opened the worktree to run its gates, a worktree without a `.git` file
// cannot be opened, so remove exited 6 and the registration (and with it the branch) stayed
// held forever. These tests pin the fix AND the bar it must not lower: a husk with content,
// a husk something holds open, a husk whose lock belongs to someone else, and a husk whose
// HEAD would strand commits are all still refused.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// makeHusk turns a registered worktree into the reported shape: the directory survives but
// holds nothing — no files, no `.git` link.
func makeHusk(t *testing.T, target string) {
	t.Helper()
	if err := os.RemoveAll(target); err != nil {
		t.Fatalf("empty %s: %v", target, err)
	}
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatalf("recreate empty %s: %v", target, err)
	}
}

// noHolders stubs the open-handle probe to "nothing holds it", restoring on cleanup.
func noHolders(t *testing.T) {
	t.Helper()
	t.Setenv("DESK_SESSION", "") // resolveSession then falls to withEnv's CLAUDE_SESSION_ID=test
	old := huskHolders
	huskHolders = func(string) ([]string, error) { return nil, nil }
	t.Cleanup(func() { huskHolders = old })
}

func registered(t *testing.T, work, target string) bool {
	t.Helper()
	return strings.Contains(mustGit(t, work, "worktree", "list", "--porcelain"), resolvePath(target))
}

// The acceptance fixture: a registered, LOCKED, empty-husk worktree is removed, and the
// branch it held can be checked out elsewhere afterwards.
// regression: #1905
func TestRemoveEmptyHuskLockedByCallerSucceeds(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work) // the caller's session is "test" (CLAUDE_SESSION_ID)
	noHolders(t)
	target := addWorktree(t, "husk")
	mustGit(t, work, "worktree", "lock", "--reason", "worker-desk live session (deskwt role-init session=test)", target)
	makeHusk(t, target)

	rc, errout := runCapErr(t, []string{"remove", target})
	if rc != deskkit.ExitOK {
		t.Fatalf("remove of an empty locked husk rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if registered(t, work, target) {
		t.Fatalf("the husk registration survived remove:\n%s", mustGit(t, work, "worktree", "list", "--porcelain"))
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("the empty husk directory was left behind: %v", err)
	}
	// The point of the fix: the branch is free again.
	other := filepath.Join(tmpBaseDir, "husk-again")
	mustGit(t, work, "worktree", "add", other, "husk")
}

// A registration whose directory is gone entirely is the same class and clears the same way.
// regression: #1905
func TestRemoveMissingDirectoryRegistrationSucceeds(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	noHolders(t)
	target := addWorktree(t, "gone")
	mustGit(t, work, "worktree", "lock", "--reason", "worker-desk live session (deskwt role-init session=test)", target)
	if err := os.RemoveAll(target); err != nil {
		t.Fatal(err)
	}

	rc, errout := runCapErr(t, []string{"remove", target})
	if rc != deskkit.ExitOK {
		t.Fatalf("remove of a directory-less registration rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if registered(t, work, target) {
		t.Fatalf("the registration survived remove")
	}
}

// An UNLOCKED empty husk needs no owner match.
// regression: #1905
func TestRemoveUnlockedEmptyHuskSucceeds(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	noHolders(t)
	target := addWorktree(t, "unlocked")
	makeHusk(t, target)

	if rc, errout := runCapErr(t, []string{"remove", target}); rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if registered(t, work, target) {
		t.Fatalf("the registration survived remove")
	}
}

// A husk that still holds anything is refused and nothing is touched: those files exist
// nowhere else.
// regression: #1905
func TestRemoveHuskWithFilesStillRefused(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	noHolders(t)
	target := addWorktree(t, "full")
	makeHusk(t, target)
	writeFile(t, filepath.Join(target, "precious.txt"), "only copy\n")

	rc, errout := runCapErr(t, []string{"remove", target})
	if rc == deskkit.ExitOK {
		t.Fatalf("remove cleared a husk that holds a file")
	}
	if !strings.Contains(errout, "holds 1 entr") {
		t.Fatalf("the refusal does not name the content; stderr:\n%s", errout)
	}
	assertExists(t, filepath.Join(target, "precious.txt"))
	if !registered(t, work, target) {
		t.Fatalf("the registration of a husk with content was dropped")
	}
}

// A process holding the directory open (its cwd included) blocks the clear.
// regression: #1905
func TestRemoveHuskHeldOpenRefused(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	t.Setenv("DESK_SESSION", "")
	old := huskHolders
	huskHolders = func(string) ([]string, error) { return []string{"4242"}, nil }
	t.Cleanup(func() { huskHolders = old })
	target := addWorktree(t, "held")
	makeHusk(t, target)

	rc, errout := runCapErr(t, []string{"remove", target})
	if rc != deskkit.ExitRefused {
		t.Fatalf("rc = %d, want %d; stderr:\n%s", rc, deskkit.ExitRefused, errout)
	}
	if !strings.Contains(errout, "4242") || !registered(t, work, target) {
		t.Fatalf("held husk was not refused cleanly; stderr:\n%s", errout)
	}
}

// An open-handle probe that cannot run is could-not-check: exit 6, never a pass.
// regression: #1905
func TestRemoveHuskProbeUnavailableIsCouldNotCheck(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	t.Setenv("DESK_SESSION", "")
	old := huskHolders
	huskHolders = func(string) ([]string, error) { return nil, os.ErrNotExist }
	t.Cleanup(func() { huskHolders = old })
	target := addWorktree(t, "noprobe")
	makeHusk(t, target)

	rc, errout := runCapErr(t, []string{"remove", target})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("rc = %d, want %d; stderr:\n%s", rc, deskkit.ExitUnverifiable, errout)
	}
	if !registered(t, work, target) {
		t.Fatalf("registration dropped although the probe could not run")
	}
}

// The real probe sees a process whose cwd is the directory, and stops seeing it afterwards.
// Skipped where lsof is not installed — the unavailable case above pins that arm.
// regression: #1905
func TestLsofHoldersSeesCwdHolder(t *testing.T) {
	if _, err := exec.LookPath("lsof"); err != nil {
		t.Skip("lsof not installed")
	}
	dir := t.TempDir()
	if pids, err := lsofHolders(dir); err != nil || len(pids) != 0 {
		t.Fatalf("idle dir: holders = %v, err = %v; want none", pids, err)
	}
	cmd := exec.Command("sleep", "30")
	cmd.Dir = dir
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	deadline := time.Now().Add(5 * time.Second)
	for {
		pids, err := lsofHolders(dir)
		if err == nil && len(pids) != 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("lsof never reported the cwd holder: pids = %v, err = %v", pids, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// A lock whose owner is a DIFFERENT, live session is not the caller's to retire.
// regression: #1905
func TestRemoveHuskLockedByLiveOtherSessionRefused(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	noHolders(t)
	target := addWorktree(t, "other")
	mustGit(t, work, "worktree", "lock", "--reason", "worker-desk live session (deskwt role-init session=alive-9)", target)
	plantBeacon(t, "alive-9", time.Now())
	makeHusk(t, target)

	rc, errout := runCapErr(t, []string{"remove", target})
	if rc != deskkit.ExitRefused {
		t.Fatalf("rc = %d, want %d; stderr:\n%s", rc, deskkit.ExitRefused, errout)
	}
	assertLocked(t, work, target, true)
}

// The same lock held by a session the roster proves GONE is retired, with the evidence named.
// regression: #1905
func TestRemoveHuskLockedByDeadSessionSucceeds(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	noHolders(t)
	target := addWorktree(t, "dead")
	mustGit(t, work, "worktree", "lock", "--reason", "worker-desk live session (deskwt role-init session=dead-9)", target)
	plantBeacon(t, "someone-else", time.Now()) // roster readable, no beacon for dead-9
	makeHusk(t, target)

	if rc, errout := runCapErr(t, []string{"remove", target}); rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if registered(t, work, target) {
		t.Fatalf("registration survived")
	}
}

// A lock that names no session, and git's own in-flight lock, have no identifiable owner and
// are never retired here.
// regression: #1905
func TestRemoveHuskUnattributableLockRefused(t *testing.T) {
	for name, reason := range map[string]string{
		"nosession": "worker-desk dispatch in flight",
		"initing":   "initializing",
	} {
		t.Run(name, func(t *testing.T) {
			work := newRepo(t)
			withEnv(t, work)
			noHolders(t)
			target := addWorktree(t, name)
			mustGit(t, work, "worktree", "lock", "--reason", reason, target)
			makeHusk(t, target)

			if rc, errout := runCapErr(t, []string{"remove", target}); rc != deskkit.ExitRefused {
				t.Fatalf("rc = %d, want %d; stderr:\n%s", rc, deskkit.ExitRefused, errout)
			}
			assertLocked(t, work, target, true)
			if !registered(t, work, target) {
				t.Fatalf("registration dropped")
			}
		})
	}
}

// A husk whose admin HEAD is a detached commit on no remote would strand that commit when its
// admin entry is deleted: refused, as prune's stale-registration sweep refuses it.
// regression: #1905
func TestRemoveHuskUnpushedDetachedHeadRefused(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	noHolders(t)
	target := addWorktree(t, "unpushed")
	mustGit(t, target, "checkout", "--detach")
	writeFile(t, filepath.Join(target, "only-here.txt"), "work\n")
	mustGit(t, target, "add", "only-here.txt")
	mustGit(t, target, "commit", "-m", "unpushed work")
	makeHusk(t, target)

	rc, errout := runCapErr(t, []string{"remove", target})
	if rc != deskkit.ExitRefused {
		t.Fatalf("rc = %d, want %d; stderr:\n%s", rc, deskkit.ExitRefused, errout)
	}
	if !strings.Contains(errout, "on no remote-tracking ref") || !registered(t, work, target) {
		t.Fatalf("unpushed detached husk was not held; stderr:\n%s", errout)
	}
}

// If the clear fails after the lock was retired, the ORIGINAL lock comes back.
// regression: #1905
func TestRemoveHuskRelocksWhenClearFails(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	noHolders(t)
	target := addWorktree(t, "relock")
	reason := "worker-desk live session (deskwt role-init session=test)"
	mustGit(t, work, "worktree", "lock", "--reason", reason, target)
	makeHusk(t, target)

	// `git worktree prune` fails after the unlock, so the registration survives.
	inner := execCommand
	unlocked, injected := false, false
	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "git" && len(args) >= 2 && args[0] == "worktree" {
			if args[1] == "unlock" {
				unlocked = true
			}
			if unlocked && !injected && args[1] == "prune" {
				injected = true
				return inner("git", "fixture-injected-failure")
			}
		}
		return inner(name, args...)
	}
	t.Cleanup(func() { execCommand = inner })

	rc, errout := runCapErr(t, []string{"remove", target})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("rc = %d, want %d; stderr:\n%s", rc, deskkit.ExitUnverifiable, errout)
	}
	if !injected || !unlocked {
		t.Fatal("positive control: the injected prune failure never fired after the unlock")
	}
	if !strings.Contains(errout, "was restored") {
		t.Fatalf("no restoration reported; stderr:\n%s", errout)
	}
	assertLocked(t, work, target, true)
	if got := mustGit(t, work, "worktree", "list", "--porcelain"); !strings.Contains(got, "locked "+reason) {
		t.Fatalf("the original reason was not restored:\n%s", got)
	}
}

// A worktree that still has its git link never reaches the husk arm: the ordinary gates run.
// regression: #1905
func TestRemoveHuskArmIgnoresLiveWorktree(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "live")
	handled, _, err := removeHusk(mustGuard(t, work), work, resolvePath(target))
	if handled || err != nil {
		t.Fatalf("husk arm claimed a live worktree: handled=%v err=%v", handled, err)
	}
}

func mustGuard(t *testing.T, dir string) *pathGuard {
	t.Helper()
	g, err := newPathGuard(dir)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// before_remove keeps its contract on the husk arm: it runs with the worktree being removed
// as its cwd and ASSAY_WORKTREE set, and when the directory is already gone it is SKIPPED —
// it must never run in the caller's checkout with an empty ASSAY_WORKTREE.
// regression: #1905
func TestRemoveHuskHookContract(t *testing.T) {
	for _, tc := range []struct {
		name    string
		husk    bool
		wantLog bool
	}{
		{"emptydir", true, true},
		{"dirgone", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			work := newRepo(t)
			withEnv(t, work)
			noHolders(t)
			target := addWorktree(t, "hook-"+tc.name)
			if tc.husk {
				makeHusk(t, target)
			} else if err := os.RemoveAll(target); err != nil {
				t.Fatal(err)
			}
			logf := filepath.Join(t.TempDir(), "hook.log")
			plantHooks(t, "before_remove: echo \"cwd=$(pwd -P) wt=[$ASSAY_WORKTREE]\" >> "+logf+"\n")

			if rc, errout := runCapErr(t, []string{"remove", target}); rc != deskkit.ExitOK {
				t.Fatalf("rc = %d, want 0; stderr:\n%s", rc, errout)
			}
			b, _ := os.ReadFile(logf)
			got := strings.TrimSpace(string(b))
			if !tc.wantLog {
				if got != "" {
					t.Fatalf("before_remove ran for a worktree whose directory is gone: %q", got)
				}
				return
			}
			want := "cwd=" + resolvePath(target) + " wt=[" + target + "]"
			if got != want && got != "cwd="+resolvePath(target)+" wt=["+resolvePath(target)+"]" {
				t.Fatalf("hook log = %q, want cwd = the husk and ASSAY_WORKTREE set (%q)", got, want)
			}
		})
	}
}
