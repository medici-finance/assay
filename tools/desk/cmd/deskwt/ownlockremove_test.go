package main

// ownlockremove_test.go — `deskwt remove` on a LIVE worktree that is locked.
//
// The end-of-PR step for a session that made its own worktree is `deskwt remove <home>` once
// the PR is pushed and the tree is clean. Such a home is normally LOCKED for its session
// (`deskwt add --lock-session`, role-init), and git never prunes a locked registration. remove
// deleted the directory and then ran `git worktree prune`, which kept the locked entry: the
// verb exited 6 with the tree gone and the branch still registered, so the branch stayed
// held and the session could not release its own work.
//
// The fix: before anything is deleted, remove judges the lock. The caller's own session's lock
// is retired as the last step before the delete (and restored if the delete fails) when the tree
// holds no untracked content; any other lock is REFUSED with the tree untouched, whatever the
// state of the owner's roster beacon: a beacon is stamped only when its session writes the
// roster, so an old one is not proof the owner is gone. A dead session's tree is released by
// `deskwt prune --reap-dead-sessions`.

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// lockedHome creates a clean, pushed deskwt home locked for session sess.
func lockedHome(t *testing.T, name, sess string) string {
	t.Helper()
	if rc, stderr := runCapErr(t, []string{"add", name, "--lock-session", sess}); rc != deskkit.ExitOK {
		t.Fatalf("fixture add --lock-session rc=%d:\n%s", rc, stderr)
	}
	return resolvePath(filepath.Join(tmpBaseDir, "tracker-"+name))
}

// The caller removes its OWN locked home: the tree and its registration are both gone.
// regression: F-locked-remove
func TestRemoveOwnLockedHome(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work) // the caller's session is "test" (CLAUDE_SESSION_ID)
	t.Setenv("DESK_SESSION", "")
	home := lockedHome(t, "ownlock", "test")
	assertLocked(t, work, home, true)

	rc, stderr := runCapErr(t, []string{"remove", home})
	if rc != deskkit.ExitOK {
		t.Fatalf("remove of the caller's own locked home rc=%d, want 0:\n%s", rc, stderr)
	}
	assertDeregistered(t, work, home)
}

// The planted second instance: the end-of-PR step is run from INSIDE the home, the shape a
// session's last command takes.
// regression: F-locked-remove
func TestRemoveOwnLockedFromInside(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	t.Setenv("DESK_SESSION", "")
	home := lockedHome(t, "ownlockin", "test")

	getwd = func() (string, error) { return home, nil }
	rc, stderr := runCapErr(t, []string{"remove", home})
	if rc != deskkit.ExitOK {
		t.Fatalf("remove of the caller's own locked home from inside it rc=%d, want 0:\n%s", rc, stderr)
	}
	assertDeregistered(t, work, home)
}

// A lock held by a DIFFERENT live session is not the caller's to retire: refused, and the
// tree, its lock and its registration are all left exactly as they were. Before the fix the
// directory was deleted and only the registration survived.
// regression: F-locked-remove
func TestRemoveForeignLockedRefused(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	t.Setenv("DESK_SESSION", "")
	home := lockedHome(t, "foreignlock", "alive-7")
	plantBeacon(t, "alive-7", time.Now())

	rc, stderr := runCapErr(t, []string{"remove", home})
	if rc != deskkit.ExitRefused {
		t.Fatalf("remove of a home locked by a live other session rc=%d, want %d:\n%s", rc, deskkit.ExitRefused, stderr)
	}
	assertExists(t, filepath.Join(home, "README.md"))
	assertLocked(t, work, home, true)
	if !registered(t, work, home) {
		t.Fatalf("the registration of a refused home was dropped")
	}
}

// A lock that names no session has no identifiable owner and is never retired.
// regression: F-locked-remove
func TestRemoveUnownedLockRefused(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	t.Setenv("DESK_SESSION", "")
	home := addedHome(t, "nosesslock")
	mustGit(t, work, "worktree", "lock", "--reason", "held by hand", home)

	rc, stderr := runCapErr(t, []string{"remove", home})
	if rc != deskkit.ExitRefused {
		t.Fatalf("remove of a home whose lock names no session rc=%d, want %d:\n%s", rc, deskkit.ExitRefused, stderr)
	}
	assertExists(t, filepath.Join(home, "README.md"))
	assertLocked(t, work, home, true)
}

// A lock held for a DIFFERENT session that has written no roster beacon,
// while the roster directory exists (another session's beacon is there, as on any desk machine)
// and the tree holds an untracked file. A missing beacon is not proof the session is gone — a
// Codex session following the end-of-PR recipe writes none — so this is refused with exit 5 and
// the file, the lock and the registration are all intact. Before the fix: rc=0, the file deleted.
// regression: F-locked-remove
func TestRemoveForeignNoBeaconRefused(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	t.Setenv("DESK_SESSION", "")
	home := lockedHome(t, "zznobeacon", "codex-live-1")
	plantBeacon(t, "some-other-desk", time.Now()) // the roster dir exists; none for codex-live-1
	untracked := filepath.Join(home, "wip-untracked.txt")
	writeFile(t, untracked, "work in progress\n")

	rc, stderr := runCapErr(t, []string{"remove", home})
	if rc != deskkit.ExitRefused {
		t.Fatalf("remove of a home locked by a session with no beacon rc=%d, want %d:\n%s", rc, deskkit.ExitRefused, stderr)
	}
	assertExists(t, untracked)
	assertExists(t, filepath.Join(home, "README.md"))
	assertLocked(t, work, home, true)
	if !registered(t, work, home) {
		t.Fatalf("the registration of a refused home was dropped")
	}
}

// The security-reviewed case: a lock held for a DIFFERENT session whose beacon is 61 minutes old
// (older than the 60-minute fresh window) while that session is alive: a process is running with
// its cwd in the tree and the tree holds an untracked source file. A beacon is written only when
// its session mutates the roster, so silence for an hour is normal for a working session; the
// beacon is not evidence of death. Refused with exit 5, nothing deleted. Before the fix: rc=0,
// tree, registration and the untracked file gone, the process left running in a deleted cwd.
// regression: F-locked-remove
func TestRemoveForeignStaleBeaconLiveOwnerRefused(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs a POSIX sleep process")
	}
	work := newRepo(t)
	withEnv(t, work)
	t.Setenv("DESK_SESSION", "")
	home := lockedHome(t, "zzstalelive", "codex-live-2")
	plantBeacon(t, "codex-live-2", time.Now().Add(-61*time.Minute))
	untracked := filepath.Join(home, "src-in-progress.go")
	writeFile(t, untracked, "package wip\n")

	live := exec.Command("sleep", "120")
	live.Dir = home
	if err := live.Start(); err != nil {
		t.Fatalf("start the live process in the tree: %v", err)
	}
	t.Cleanup(func() { _ = live.Process.Kill(); _ = live.Wait() })

	rc, stderr := runCapErr(t, []string{"remove", home})
	if rc != deskkit.ExitRefused {
		t.Fatalf("remove of a live session's home with a 61-minute beacon rc=%d, want %d:\n%s", rc, deskkit.ExitRefused, stderr)
	}
	assertExists(t, untracked)
	assertExists(t, filepath.Join(home, "README.md"))
	assertLocked(t, work, home, true)
	if !registered(t, work, home) {
		t.Fatalf("the registration of a refused home was dropped")
	}
	if !strings.Contains(stderr, "prune --reap-dead-sessions") {
		t.Fatalf("the refusal does not name the way out for a dead session:\n%s", stderr)
	}
}

// The own-lock arm matches a self-asserted, inherited session id, so it does not delete untracked
// content either: a never-added file in the caller's own locked home is refused (exit 5) with the
// file, lock and registration intact. Once the file is moved away the same call succeeds.
// regression: F-locked-remove
func TestRemoveOwnLockedUntrackedRefused(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	t.Setenv("DESK_SESSION", "")
	home := lockedHome(t, "ownuntracked", "test")
	untracked := filepath.Join(home, "never-added.go")
	writeFile(t, untracked, "package wip\n")

	rc, stderr := runCapErr(t, []string{"remove", home})
	if rc != deskkit.ExitRefused {
		t.Fatalf("remove of an own locked home holding an untracked file rc=%d, want %d:\n%s", rc, deskkit.ExitRefused, stderr)
	}
	assertExists(t, untracked)
	assertLocked(t, work, home, true)
	if !registered(t, work, home) {
		t.Fatalf("the registration of a refused home was dropped")
	}

	if err := os.Remove(untracked); err != nil {
		t.Fatal(err)
	}
	rc, stderr = runCapErr(t, []string{"remove", home})
	if rc != deskkit.ExitOK {
		t.Fatalf("remove after clearing the untracked file rc=%d, want 0:\n%s", rc, stderr)
	}
	assertDeregistered(t, work, home)
}

// A delete that fails after the unlock restores the original lock and its reason: the tree is
// not left unlocked for the next sweep. The failure is injected at the deregistration (git's
// `worktree prune`), the step that follows the directory delete.
// regression: F-locked-remove
func TestRemoveOwnLockedRestoredOnFailure(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	t.Setenv("DESK_SESSION", "")
	home := lockedHome(t, "ownrestore", "test")

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

	rc, stderr := runCapErr(t, []string{"remove", home})
	if !injected || !unlocked {
		t.Fatal("positive control: the injected failure never fired after the unlock")
	}
	if rc == deskkit.ExitOK {
		t.Fatalf("remove reported success although its delete failed:\n%s", stderr)
	}
	if !strings.Contains(stderr, "was restored") {
		t.Fatalf("no restoration reported; stderr:\n%s", stderr)
	}
	assertLocked(t, work, home, true)
	if got := mustGit(t, work, "worktree", "list", "--porcelain"); !strings.Contains(got, "locked ") || !strings.Contains(got, "session=test") {
		t.Fatalf("the original lock reason was not restored:\n%s", got)
	}
}
