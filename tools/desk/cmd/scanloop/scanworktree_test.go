package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// regression: #6282
//
// TestScanWorktreeIsSanctioned — a scan worktree the lane retains after a failure must be removable
// through the sanctioned worktree tooling, so the DEFAULT path must pass that tooling's own prefix
// check. It is asserted against the shared rule deskwt calls, with the production tmp base, never a
// copy of the rule.
func TestScanWorktreeIsSanctioned(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the POSIX default; the Windows default is TestWindowsScanBaseUnderRepo")
	}
	root := filepath.Join(t.TempDir(), "target")
	wt := (&ScanLoop{Root: root}).worktreeFor(loopengine.Item{ID: "example-org/tracker#5"})
	if fi, err := os.Stat(scanTmpBase); err != nil || !fi.IsDir() {
		// A host without the tmp base (a stock Linux) has no default: it is refused up front, and
		// TestPosixBaseMissingRefuses pins that. What must hold here is that no path is invented.
		if wt != "" {
			t.Fatalf("the tmp base is absent on this host, yet a default scan worktree %q was produced", wt)
		}
		return
	}
	if wt == "" {
		t.Fatal("no default scan worktree path")
	}
	if !deskkit.InSanctionedWorktreePrefix(wt, deskkit.SanctionedWorktreeTmpBase, filepath.Join(root, ".claude", "worktrees")) {
		t.Fatalf("the default scan worktree %q is outside the sanctioned worktree prefixes, so a retained one "+
			"cannot be removed through the sanctioned tooling", wt)
	}
	if strings.HasPrefix(wt, root+string(filepath.Separator)) {
		t.Fatalf("the default scan worktree %q is nested inside the target checkout %q", wt, root)
	}
}

// regression: #6282
//
// TestWindowsScanBaseUnderRepo — on Windows the sanctioned prefix is <repo-root>/.claude/worktrees.
// From a LINKED worktree the default sits there; from the MAIN checkout it would be nested in the
// checkout being scanned, which is refused up front, never placed somewhere deskwt cannot reach.
func TestWindowsScanBaseUnderRepo(t *testing.T) {
	prev := scanGOOS
	scanGOOS = "windows"
	t.Cleanup(func() { scanGOOS = prev })

	base := t.TempDir()
	mainDir := filepath.Join(base, "main")
	scanGit(t, base, "init", "-b", "main", mainDir)
	scanGit(t, mainDir, "-c", "user.name=t", "-c", "user.email=t@example.test", "commit", "--allow-empty", "-m", "seed")
	linked := filepath.Join(base, "linked")
	scanGit(t, mainDir, "worktree", "add", linked, "-b", "desk")

	got, err := defaultScanWorktreeBase(linked)
	if err != nil {
		t.Fatalf("from a linked worktree: %v", err)
	}
	mainResolved, _ := filepath.EvalSymlinks(mainDir)
	gotResolved, _ := filepath.EvalSymlinks(filepath.Dir(filepath.Dir(got)))
	if filepath.Base(got) != "worktrees" || filepath.Base(filepath.Dir(got)) != ".claude" || gotResolved != mainResolved {
		t.Fatalf("Windows default base = %q, want <main checkout>/.claude/worktrees", got)
	}

	if _, err := defaultScanWorktreeBase(mainDir); deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("from the main checkout: err = %v, want a refusal — the prefix is nested in the scanned checkout", err)
	}
}

// regression: #6282
//
// TestPosixBaseMissingRefuses — the sanctioned tmp base is a fixed path that not every POSIX host
// has (a stock Linux has no /private). Without --worktree-base the default must be refused up
// front, by name, never handed to git to fail on every pass.
func TestPosixBaseMissingRefuses(t *testing.T) {
	prevOS, prevBase := scanGOOS, scanTmpBase
	scanGOOS, scanTmpBase = "linux", filepath.Join(t.TempDir(), "absent")
	t.Cleanup(func() { scanGOOS, scanTmpBase = prevOS, prevBase })

	_, err := defaultScanWorktreeBase(filepath.Join(t.TempDir(), "target"))
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused || !strings.Contains(fmt.Sprint(err), "--worktree-base") {
		t.Fatalf("err = %v, want a refusal that names --worktree-base", err)
	}
	if wt := (&ScanLoop{Root: filepath.Join(t.TempDir(), "target")}).worktreeFor(loopengine.Item{ID: "example-org/tracker#5"}); wt != "" {
		t.Fatalf("worktreeFor = %q with no usable base, want \"\" (the lane's guard refuses it)", wt)
	}
}

// pinScanTmpBase points the POSIX default tmp base at an existing private directory for one test,
// so a test about the shape of the default path holds on a host without the real tmp base (a stock
// Linux runner), where the default is refused up front instead.
//
// regression: #6282
func pinScanTmpBase(t *testing.T) {
	t.Helper()
	prev := scanTmpBase
	scanTmpBase = t.TempDir()
	t.Cleanup(func() { scanTmpBase = prev })
}
