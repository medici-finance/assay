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
// (or one whose session the roster proves gone) is retired as the last step before the delete
// and restored if the delete fails; any other lock is REFUSED with the tree untouched.

import (
	"path/filepath"
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
