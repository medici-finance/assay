package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// windowsprefix_test.go pins the fix for assay#656: `deskwt add` / `role-init` must build
// their worktree target under a prefix that is PORTABLE on the host OS. On native Windows
// the compiled `/private/tmp` prefix becomes a drive-rooted `\private\tmp\...` that fails
// the sanctioned-prefix guard, so no desk worktree can be created and `deskboot` refuses
// the shared checkout. The fix chooses the OTHER documented sanctioned prefix —
// `<repo-root>/.claude/worktrees/` — on Windows. It picks WHICH sanctioned prefix to use;
// it never widens or bypasses the allowlist (`allowed` still gates both prefixes on every
// platform), so the isolation guarantee is intact.
//
// The `goos` seam lets these run on a POSIX test runner while exercising the Windows
// codepath; genuine backslash / drive-root resolution is a compile-time property of the
// `filepath` package on an actual Windows build and cannot be reproduced here.

// setGOOS points the host-OS seam at goosVal for one test and restores it after.
func setGOOS(t *testing.T, goosVal string) {
	t.Helper()
	old := goos
	goos = goosVal
	t.Cleanup(func() { goos = old })
}

// TestWorktreeTargetChoosesPrefixPerOS is the unit-level guard on the selection logic:
// POSIX targets the `/private/tmp` prefix; Windows targets the portable
// `<repo-root>/.claude/worktrees/` prefix and NEVER the tmp prefix.
func TestWorktreeTargetChoosesPrefixPerOS(t *testing.T) {
	sep := string(filepath.Separator)
	g := &pathGuard{worktreesDir: filepath.Join(sep+"repo", ".claude", "worktrees")}
	oldTmp := tmpBaseDir
	tmpBaseDir = sep + "posixtmp"
	t.Cleanup(func() { tmpBaseDir = oldTmp })

	setGOOS(t, "linux")
	if got, want := g.worktreeTarget("tracker-x"), filepath.Join(tmpBaseDir, "tracker-x"); got != want {
		t.Fatalf("POSIX target = %q, want %q (the /private/tmp prefix)", got, want)
	}

	setGOOS(t, "windows")
	if got, want := g.worktreeTarget("tracker-x"), filepath.Join(g.worktreesDir, "tracker-x"); got != want {
		t.Fatalf("Windows target = %q, want %q (the portable <repo-root>/.claude/worktrees/ prefix)", got, want)
	}
	if strings.HasPrefix(g.worktreeTarget("tracker-x"), tmpBaseDir) {
		t.Fatalf("Windows target is still under the tmp prefix %q — the assay#656 bug", tmpBaseDir)
	}
}

// TestAddOnWindowsCreatesUnderPortablePrefix exercises the whole `add` verb on the Windows
// codepath: the worktree is created under `<repo-root>/.claude/worktrees/`, passes the
// sanctioned-prefix guard (rc 0), and is NOT placed under the tmp prefix.
func TestAddOnWindowsCreatesUnderPortablePrefix(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)
	setGOOS(t, "windows")

	if rc := run([]string{"add", "win"}); rc != deskkit.ExitOK {
		t.Fatalf("add on the windows codepath rc = %d, want 0", rc)
	}
	portable := filepath.Join(work, ".claude", "worktrees", "tracker-win")
	if _, err := os.Stat(portable); err != nil {
		t.Fatalf("expected the worktree under the portable prefix %s: %v", portable, err)
	}
	if !hasWorktreeVerb(*calls, "add") {
		t.Fatalf("expected a `git worktree add`; git calls: %v", gitCalls(*calls))
	}
	if anyGitForce(*calls) {
		t.Fatalf("a git argv carried a force flag on add: %v", gitCalls(*calls))
	}
	// It must NOT have gone to the /private/tmp prefix that fails the guard on Windows.
	if _, err := os.Stat(filepath.Join(tmpBaseDir, "tracker-win")); !os.IsNotExist(err) {
		t.Fatalf("worktree was created under the tmp prefix on the windows codepath (err=%v) — assay#656 not fixed", err)
	}
}

// TestRoleInitOnWindowsUsesPortablePrefix: role-init must also land under the portable
// prefix on Windows, and role-clean must resolve the SAME prefix to tear it down.
func TestRoleInitOnWindowsUsesPortablePrefix(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	setGOOS(t, "windows")

	rc, stderr := runCapErr(t, []string{"role-init", "--role", "verifier", "--session", "winsess"})
	if rc != deskkit.ExitOK {
		t.Fatalf("role-init on the windows codepath rc = %d, want 0; stderr: %s", rc, stderr)
	}
	portable := filepath.Join(work, ".claude", "worktrees", "tracker-verify-desk-winsess")
	if _, err := os.Stat(portable); err != nil {
		t.Fatalf("expected the role worktree under the portable prefix %s: %v", portable, err)
	}
	if _, err := os.Stat(filepath.Join(tmpBaseDir, "tracker-verify-desk-winsess")); !os.IsNotExist(err) {
		t.Fatalf("role worktree created under the tmp prefix on the windows codepath — assay#656 not fixed")
	}

	// role-clean resolves the same portable target and removes it.
	rc, stderr = runCapErr(t, []string{"role-clean", "--role", "verifier", "--session", "winsess"})
	if rc != deskkit.ExitOK {
		t.Fatalf("role-clean on the windows codepath rc = %d, want 0; stderr: %s", rc, stderr)
	}
	if _, err := os.Stat(portable); !os.IsNotExist(err) {
		t.Fatalf("role worktree %s still present after role-clean: %v", portable, err)
	}
}
