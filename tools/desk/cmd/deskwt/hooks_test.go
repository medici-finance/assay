package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// plantHooks writes hooks.yaml into the desk-tools state dir (<HOME>/.config/assay), the ONE
// place hooks are read from. withEnv() sets HOME to a fresh temp dir, so this is that
// session's state dir. Call it AFTER withEnv.
func plantHooks(t *testing.T, body string) {
	t.Helper()
	dir := filepath.Join(os.Getenv("HOME"), ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "hooks.yaml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestAfterCreateRunsOnceForNewPath (Verify row 6): the after_create hook runs exactly once
// when a NEW worktree is created — the pre-mortem "after_create re-runs on an existing
// worktree and clobbers identity" is caught here (deskwt add never reuses a path, so a
// second add of the same name refuses before any hook runs).
func TestAfterCreateRunsOnceForNewPath(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)

	counter := filepath.Join(t.TempDir(), "after_create.count")
	plantHooks(t, "after_create: printf 'x' >> "+counter+"\n")

	if rc := run([]string{"add", "hooked"}); rc != deskkit.ExitOK {
		t.Fatalf("add rc = %d, want 0", rc)
	}
	target := filepath.Join(tmpBaseDir, "tracker-hooked")
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("worktree %s not created: %v", target, err)
	}
	got := readCount(t, counter)
	if got != 1 {
		t.Fatalf("after_create ran %d time(s), want exactly 1", got)
	}

	// A second add of the SAME name is refused (never clobbers) — so after_create does NOT
	// run again over an existing worktree.
	if rc := run([]string{"add", "hooked"}); rc != deskkit.ExitRefused {
		t.Fatalf("second add rc = %d, want 5 (refused, never clobbered)", rc)
	}
	if got := readCount(t, counter); got != 1 {
		t.Fatalf("after_create ran again on the refused re-add (count=%d) — it must run for a NEW path only", got)
	}
}

// TestAfterCreateFailureRollsBackWorktree: a fatal after_create failure ABORTS creation —
// the worktree it just made is rolled back rather than left half-provisioned.
func TestAfterCreateFailureRollsBackWorktree(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	plantHooks(t, "after_create: exit 1\n")

	if rc := run([]string{"add", "doomed"}); rc != deskkit.ExitUnverifiable {
		t.Fatalf("add with failing after_create rc = %d, want 6", rc)
	}
	target := filepath.Join(tmpBaseDir, "tracker-doomed")
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("worktree %s survived a failed after_create — creation must be rolled back (err=%v)", target, err)
	}
	list := mustGit(t, work, "worktree", "list", "--porcelain")
	if strings.Contains(list, target) {
		t.Fatalf("worktree still registered after a failed after_create:\n%s", list)
	}
}

// TestBeforeRemoveFailureStillRemoves (Verify row 6): a before_remove hook that FAILS is
// logged, and the deletion PROCEEDS — a cleanup hook must never strand a worktree the caller
// asked to remove.
func TestBeforeRemoveFailureStillRemoves(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)

	// after_create empty (default no-op); before_remove fails.
	plantHooks(t, "before_remove: exit 1\n")

	if rc := run([]string{"add", "goner"}); rc != deskkit.ExitOK {
		t.Fatalf("add rc = %d, want 0", rc)
	}
	target := filepath.Join(tmpBaseDir, "tracker-goner")

	if rc := run([]string{"remove", target}); rc != deskkit.ExitOK {
		t.Fatalf("remove rc = %d, want 0 — a failed before_remove must NOT block the deletion", rc)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("worktree %s still exists after remove though the caller asked for it (err=%v)", target, err)
	}
	list := mustGit(t, work, "worktree", "list", "--porcelain")
	if strings.Contains(list, target) {
		t.Fatalf("worktree still registered after remove:\n%s", list)
	}
}

// TestDeskwtAddDryRunTouchesNothing: --dry-run reports the after_create hook plan and
// creates no worktree.
func TestDeskwtAddDryRunTouchesNothing(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	plantHooks(t, "after_create: true\n")

	rc, errOut := runCapErr(t, []string{"add", "dry", "--dry-run"})
	if rc != deskkit.ExitOK {
		t.Fatalf("dry-run add rc = %d, want 0", rc)
	}
	if !strings.Contains(errOut, "HOOK after_create: would run") {
		t.Errorf("dry-run add did not report the hook plan; stderr:\n%s", errOut)
	}
	target := filepath.Join(tmpBaseDir, "tracker-dry")
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("dry-run add created a worktree at %s — it must touch nothing", target)
	}
}

func readCount(t *testing.T, path string) int {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatalf("read count file: %v", err)
	}
	return len(strings.TrimSpace(string(b)))
}
