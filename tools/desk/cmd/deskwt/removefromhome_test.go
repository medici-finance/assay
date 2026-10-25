package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// `cd <home> && deskwt remove <home>` is the natural last step of a worker that cleans up
// after itself. The directory delete removes the process's own working directory, and
// the deregistration that followed ran `git worktree prune` IN that directory: the child
// could not chdir, git printed nothing, and the verb exited 6 with "git worktree prune
// failed after removing <home> — git said: (no output)". The tree was gone but its
// registration stayed `prunable`, and the next dispatch of the item failed at
// worktree-create until someone ran `git worktree prune` by hand in the shared checkout.

// addedHome creates a real deskwt worktree on a branch tracking origin/main (so it is clean
// and provably pushed) and returns its resolved path.
func addedHome(t *testing.T, name string) string {
	t.Helper()
	if rc, stderr := runCapErr(t, []string{"add", name}); rc != deskkit.ExitOK {
		t.Fatalf("fixture add rc=%d:\n%s", rc, stderr)
	}
	return resolvePath(filepath.Join(tmpBaseDir, "tracker-"+name))
}

// assertDeregistered proves the home is gone from disk AND from the registration list,
// including as a `prunable` entry, read from the shared checkout.
func assertDeregistered(t *testing.T, work, home string) {
	t.Helper()
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Fatalf("home still on disk (err=%v)", err)
	}
	list := mustGit(t, work, "worktree", "list", "--porcelain")
	if strings.Contains(list, filepath.Base(home)) {
		t.Fatalf("registration survived the remove:\n%s", list)
	}
}

// regression: #4272
func TestRemoveFromInsideHome(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	home := addedHome(t, "selfremove")

	getwd = func() (string, error) { return home, nil }
	rc, stderr := runCapErr(t, []string{"remove", home})
	if rc != deskkit.ExitOK {
		t.Fatalf("remove from inside the home rc=%d, want 0:\n%s", rc, stderr)
	}
	assertDeregistered(t, work, home)
}

// The planted second instance: the cwd is a SUBDIRECTORY of the home (a worker sitting
// in tools/desk), which the delete removes just the same.
//
// regression: #4272
func TestRemoveFromHomeSubdir(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	home := addedHome(t, "subremove")
	sub := filepath.Join(home, "nested", "dir")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	getwd = func() (string, error) { return sub, nil }
	rc, stderr := runCapErr(t, []string{"remove", home})
	if rc != deskkit.ExitOK {
		t.Fatalf("remove from a home subdirectory rc=%d, want 0:\n%s", rc, stderr)
	}
	assertDeregistered(t, work, home)
}

// The choke point every remover reaches (remove, prune, role-init, add's rollbacks): a
// caller whose dir is the path just deleted still gets its registration dropped.
//
// regression: #4272
func TestDeregisterFromDeletedDir(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	home := addedHome(t, "chokepoint")
	guard, err := newPathGuard(home)
	if err != nil {
		t.Fatal(err)
	}
	if err := removeWorktreeDir(guard, home, home); err != nil {
		t.Fatalf("removeWorktreeDir with dir == the removed tree: %v", err)
	}
	assertDeregistered(t, work, home)
}
