package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// driftFixture builds a REAL git repo whose `refs/remotes/origin/main` carries one
// `.assay-versions` (originPin) while the working tree carries another (worktreePin), and
// makes it the working directory. originPin == "" plants NO origin/main ref at all — the
// could-not-check shape. Real git rather than a seam: the origin/main read is the thing
// under test, and a seam would let the test pass with the read never wired.
func driftFixture(t *testing.T, worktreePin, originPin string) string {
	t.Helper()
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=x", "GIT_AUTHOR_EMAIL=x@example.invalid",
			"GIT_COMMITTER_NAME=x", "GIT_COMMITTER_EMAIL=x@example.invalid",
			"GIT_CONFIG_NOSYSTEM=1", "HOME="+dir)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	git("init", "-q", "-b", "main")
	pinPath := filepath.Join(dir, deskkit.AssayVersionsFile)
	if originPin != "" {
		if err := os.WriteFile(pinPath, []byte(originPin+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		git("add", deskkit.AssayVersionsFile)
		git("commit", "-q", "-m", "origin pin")
		git("update-ref", "refs/remotes/origin/main", "HEAD")
	}
	// The WORKTREE's pin is what the pin walk reads from disk; it differs from origin's.
	if err := os.WriteFile(pinPath, []byte(worktreePin+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	return dir
}

// TestStaleDriftNamesAllThreeSides is #1157's third point. A drift banner that names only
// "installed vs this worktree's pin" and always recommends the shim reinstall sent an
// operator to reinstall a CURRENT binary while the stale side was the worktree (a role
// tree three days behind main). The verdict must name all three — installed release,
// this worktree's pin, origin/main's pin — and recommend the reinstall ONLY when the
// installed release is the one behind main's pin.
func TestStaleDriftNamesAllThreeSides(t *testing.T) {
	const sha = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	oldPinned, oldTree, oldTag := isPinned, gitTree, deskkit.ReleaseTag
	oldSrc := deskToolsSourcePin
	t.Cleanup(func() {
		isPinned, gitTree, deskkit.ReleaseTag = oldPinned, oldTree, oldTag
		deskToolsSourcePin = oldSrc
	})
	isPinned = func() bool { return true }
	gitTree = func(string) (string, error) { return "", os.ErrNotExist }
	deskToolsSourcePin = func() (string, string, bool) { return "", "", false }

	t.Run("worktree behind main: installed == origin/main pin — merge, never reinstall", func(t *testing.T) {
		deskkit.ReleaseTag = "v1.0.9"
		driftFixture(t, "desk-tools v1.0.6 "+sha, "desk-tools v1.0.9 "+sha)
		state, stale, detail := staleState()
		if state != staleStateDrift || !stale {
			t.Fatalf("state=%q stale=%v, want drift: %s", state, stale, detail)
		}
		for _, want := range []string{"installed", "v1.0.9", "worktree", "v1.0.6", "origin/main"} {
			if !strings.Contains(detail, want) {
				t.Errorf("drift detail does not name %q — all three sides must be visible: %s", want, detail)
			}
		}
		if strings.Contains(detail, "desk-install") {
			t.Errorf("the shim reinstall is recommended although the installed release IS main's pin — the worktree is the stale side: %s", detail)
		}
		if !strings.Contains(detail, "merge refs/remotes/origin/main") {
			t.Errorf("the remediation for a behind-main worktree is a merge of origin/main; got: %s", detail)
		}
	})

	t.Run("installed behind main: worktree == origin/main pin — reinstall", func(t *testing.T) {
		deskkit.ReleaseTag = "v1.0.6"
		driftFixture(t, "desk-tools v1.0.9 "+sha, "desk-tools v1.0.9 "+sha)
		state, stale, detail := staleState()
		if state != staleStateDrift || !stale {
			t.Fatalf("state=%q stale=%v, want drift: %s", state, stale, detail)
		}
		if !strings.Contains(detail, "desk-install") {
			t.Errorf("the installed release is the one behind main's pin, so the reinstall IS the fix; got: %s", detail)
		}
		if strings.Contains(detail, "merge refs/remotes/origin/main") {
			t.Errorf("a merge is recommended although the worktree already carries main's pin: %s", detail)
		}
		for _, want := range []string{"v1.0.6", "v1.0.9", "origin/main"} {
			if !strings.Contains(detail, want) {
				t.Errorf("drift detail does not name %q: %s", want, detail)
			}
		}
	})

	t.Run("all three differ: both sides named, worktree first, reinstall only after", func(t *testing.T) {
		deskkit.ReleaseTag = "v1.0.8"
		driftFixture(t, "desk-tools v1.0.6 "+sha, "desk-tools v1.0.9 "+sha)
		_, stale, detail := staleState()
		if !stale {
			t.Fatalf("three-way disagreement reported fresh: %s", detail)
		}
		for _, want := range []string{"v1.0.6", "v1.0.8", "v1.0.9", "merge refs/remotes/origin/main"} {
			if !strings.Contains(detail, want) {
				t.Errorf("drift detail does not carry %q: %s", want, detail)
			}
		}
		if m, r := strings.Index(detail, "merge refs/remotes/origin/main"), strings.Index(detail, "desk-install"); r >= 0 && r < m {
			t.Errorf("the reinstall is recommended BEFORE bringing the worktree current: %s", detail)
		}
	})

	t.Run("origin/main pin unreadable: could-not-check named, no reinstall recommended", func(t *testing.T) {
		deskkit.ReleaseTag = "v1.0.9"
		driftFixture(t, "desk-tools v1.0.6 "+sha, "")
		state, stale, detail := staleState()
		if state != staleStateDrift || !stale {
			t.Fatalf("state=%q stale=%v, want drift (the two readable sides DO differ): %s", state, stale, detail)
		}
		if !strings.Contains(detail, "origin/main") || !strings.Contains(strings.ToLower(detail), "could-not-check") {
			t.Errorf("origin/main's side is unreadable and must be reported as could-not-check by name: %s", detail)
		}
		if strings.Contains(detail, "desk-install") {
			t.Errorf("the reinstall is recommended with no evidence the installed release is the stale side: %s", detail)
		}
	})
}
