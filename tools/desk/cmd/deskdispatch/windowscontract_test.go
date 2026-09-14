package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// windowscontract_test.go pins the CONSUMER half of the Windows-portability contract (#757,
// the #727/#732 family). cmd/deskwt is Windows-aware and on native Windows selects the
// sanctioned <repo-root>\.claude\worktrees\ prefix, whose worktree home is a drive-rooted
// `C:\...` path; cmd/deskwt/windowsprefix_test.go pins that PRODUCER half. deskdispatch must
// ACCEPT the home deskwt prints — a POSIX-only leading-slash check rejected it and made
// dispatch impossible on Windows, leaving a phantom HELD claim behind. Only the producer
// half was pinned before this file; these tests pin that "deskwt picks the Windows prefix"
// and "deskdispatch accepts it" are ONE contract.
//
// The goos seam lets these exercise the Windows codepath on a POSIX test runner, exactly as
// the deskwt test does. Genuine drive-root resolution is a compile-time property of
// path/filepath on a real Windows build and cannot otherwise be reproduced here.

// setGOOS points the host-OS seam at goosVal for one test and restores it after.
func setGOOS(t *testing.T, goosVal string) {
	t.Helper()
	old := goos
	goos = goosVal
	t.Cleanup(func() { goos = old })
}

// TestHomeIsAbsoluteAcceptsWhatDeskwtEmitsPerOS is the unit-level guard on the acceptance
// predicate. A POSIX-absolute home is accepted; a relative or empty home is rejected on
// every OS; and the drive-rooted / UNC forms deskwt prints on Windows are accepted UNDER
// the windows seam — the acceptance a leading-slash check dropped — and are NOT treated as
// absolute off it.
func TestHomeIsAbsoluteAcceptsWhatDeskwtEmitsPerOS(t *testing.T) {
	posixHome := "/private/tmp/tracker-item-1"
	winHome := `C:\repo\.claude\worktrees\tracker-item-1`

	setGOOS(t, "linux")
	if !homeIsAbsolute(posixHome) {
		t.Errorf("POSIX home %q rejected on linux", posixHome)
	}
	// A `C:\` string is not an absolute path on a POSIX host, so it must NOT be accepted there.
	if homeIsAbsolute(winHome) {
		t.Errorf("a drive-rooted home %q was treated as absolute on a POSIX host", winHome)
	}
	for _, rel := range []string{"", "tracker-item-1", "./x", "(no output)"} {
		if homeIsAbsolute(rel) {
			t.Errorf("relative/empty home %q accepted on linux", rel)
		}
	}

	setGOOS(t, "windows")
	for _, abs := range []string{
		winHome, // drive-rooted, backslash
		`C:/repo/.claude/worktrees/tracker-item-1`, // drive-rooted, forward slash
		`\\srv\share\tracker-item-1`,               // UNC
	} {
		if !homeIsAbsolute(abs) {
			t.Errorf("absolute Windows home %q rejected on windows — deskwt's own worktree home would be refused", abs)
		}
	}
	for _, rel := range []string{"", "tracker-item-1", `relative\path`, "C:relativeNoSlash"} {
		if homeIsAbsolute(rel) {
			t.Errorf("relative home %q accepted on windows", rel)
		}
	}
}

// TestDispatchAcceptsDeskwtWindowsWorktreeHome is the contract test end-to-end and the
// fail-first proof of the acceptance fix: with the old POSIX-only `strings.HasPrefix(home,
// "/")` check, the drive-rooted home deskwt prints on Windows is rejected and the whole
// dispatch aborts exit 6 (deskkit.Unverifiable); the fix accepts it and the dispatch
// completes, emitting a prompt that names that home as the agent's isolation floor.
func TestDispatchAcceptsDeskwtWindowsWorktreeHome(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	setGOOS(t, "windows")
	winHome := `C:\repo\.claude\worktrees\tracker-item-1`
	s.replies = happyReplies(winHome)

	promptFile := filepath.Join(t.TempDir(), "p.md")
	rc := run([]string{"item-1", "--root", root, "--repo", allowedRepo, "--prompt-file", promptFile})
	if rc != deskkit.ExitOK {
		t.Fatalf("dispatch with deskwt's Windows worktree home rc = %d, want 0 — the #757 refusal", rc)
	}
	body, err := os.ReadFile(promptFile)
	if err != nil {
		t.Fatalf("prompt file: %v", err)
	}
	if !strings.Contains(string(body), winHome) {
		t.Errorf("the emitted prompt does not name the Windows worktree home %q as the agent's isolation floor", winHome)
	}
}

// TestNonAbsoluteHomeReleasesTheOrphanedClaim pins the claim-release fix on THIS abort
// branch. When deskwt exits 0 but names a non-absolute home the dispatch is refused, and —
// exactly as the deskwt-add-failed branch just above it does — the durable claim placed one
// step earlier is RELEASED rather than left HELD to wedge every later re-dispatch behind a
// phantom claim.
//
// Fail-first on the unfixed code: this branch returned deskkit.Unverifiable directly with no
// release call, so `dispatch-claim.sh release` never ran and the claim stayed HELD.
func TestNonAbsoluteHomeReleasesTheOrphanedClaim(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = []reply{
		{match: "remote get-url origin", stdout: "git@github.com:medici-finance/assay.git"},
		{match: "deskwt add", stdout: "relative/not-absolute"},
	}

	promptFile := filepath.Join(t.TempDir(), "p.md")
	rc := run([]string{"item-1", "--root", root, "--prompt-file", promptFile})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("non-absolute home rc = %d, want %d", rc, deskkit.ExitUnverifiable)
	}
	// Precondition: the claim WAS acquired, so a release is the thing under test.
	if !s.ran("dispatch-claim.sh acquire") {
		t.Fatal("the claim was never acquired — the precondition for this test does not hold")
	}
	// The fix: the placed claim is released rather than orphaned.
	if !s.ran("dispatch-claim.sh release") {
		t.Error("a non-absolute worktree home aborted WITHOUT releasing the claim — the item is wedged " +
			"behind a phantom HELD claim, the queue suppressor the worker-desk runbook warns about")
	}
	// No agent may be launched on a home this verb refused to state.
	if _, err := os.Stat(promptFile); err == nil {
		t.Error("a prompt was emitted despite the rejected home")
	}
}
