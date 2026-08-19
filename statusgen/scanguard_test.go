package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Isolation-guard tests (scanguard.go, 2026-08-13 incident): --scan-issues
// must REFUSE a primary-checkout root (the shared-checkout shape) and ACCEPT
// a linked worktree (the sanctioned isolation), a non-git root (offline
// fixtures), and a human-claimed override.

// guardGit runs git in dir with a hermetic config, failing the test on error.
func guardGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{
		"-C", dir,
		"-c", "user.name=t", "-c", "user.email=t@example.invalid",
		"-c", "commit.gpgsign=false",
	}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
}

// guardFixtureRepos builds a primary checkout with one commit and a linked
// worktree off it, returning both roots. Tests that need only one still get
// both — the pair is cheap and keeps the two shapes side by side.
func guardFixtureRepos(t *testing.T) (primary, worktree string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	base := t.TempDir()
	primary = filepath.Join(base, "primary")
	if err := os.MkdirAll(primary, 0o755); err != nil {
		t.Fatal(err)
	}
	guardGit(t, primary, "init", "-q")
	guardGit(t, primary, "commit", "-q", "--allow-empty", "-m", "init")
	worktree = filepath.Join(base, "wt")
	guardGit(t, primary, "worktree", "add", "-q", worktree, "-b", "guard-test")
	return primary, worktree
}

func TestScanIsolationGuardNonGitRootAllowed(t *testing.T) {
	if reason := scanIsolationRefusal(t.TempDir()); reason != "" {
		t.Errorf("non-git root must be allowed (offline fixtures), got refusal: %s", reason)
	}
}

func TestScanIsolationGuardPrimaryCheckoutRefused(t *testing.T) {
	primary, _ := guardFixtureRepos(t)
	reason := scanIsolationRefusal(primary)
	if reason == "" {
		t.Fatal("primary checkout root must be refused")
	}
	// The refusal must carry the recipe and the override, so a mis-invoked
	// session can correct itself without archaeology.
	for _, want := range []string{"PRIMARY checkout", "worktree add", scanPrimaryOKEnv} {
		if !strings.Contains(reason, want) {
			t.Errorf("refusal message missing %q:\n%s", want, reason)
		}
	}
}

func TestScanIsolationGuardLinkedWorktreeAllowed(t *testing.T) {
	_, worktree := guardFixtureRepos(t)
	if reason := scanIsolationRefusal(worktree); reason != "" {
		t.Errorf("linked-worktree root is the sanctioned isolation, got refusal: %s", reason)
	}
}

func TestScanIsolationGuardOverrideAllowsPrimary(t *testing.T) {
	primary, _ := guardFixtureRepos(t)
	t.Setenv(scanPrimaryOKEnv, "1")
	if reason := scanIsolationRefusal(primary); reason != "" {
		t.Errorf("%s=1 must allow a primary-checkout root, got refusal: %s", scanPrimaryOKEnv, reason)
	}
}

// TestScanIssuesRefusesPrimaryCheckoutRoot: the full entrypoint refuses a
// primary-checkout root with exit 2 and writes NOTHING — in write mode AND in
// --dry-run mode (dry-run still performs un-block writes today, so it is
// guarded the same).
func TestScanIssuesRefusesPrimaryCheckoutRoot(t *testing.T) {
	primary, _ := guardFixtureRepos(t)
	if err := os.CopyFS(primary, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	issueLoopDir := filepath.Join(primary, "docs/streams/issue-loop")
	if err := os.MkdirAll(issueLoopDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, issueLoopDir, "README.md", scanStreamREADME)

	data := map[string][]ghIssue{
		scanHomeRepo(): {{Number: 601, Title: "bug", Labels: lbl("bug")}},
	}
	for _, dryRun := range []bool{false, true} {
		if code := runScanIssues(primary, dryRun, fixtureLister(data, ""), nilCommentLister, blessAll); code != 2 {
			t.Fatalf("dryRun=%v: primary-checkout scan exited %d, want 2 (refused)", dryRun, code)
		}
		if _, err := os.Stat(filepath.Join(issueLoopDir, "issue-601.md")); !os.IsNotExist(err) {
			t.Fatalf("dryRun=%v: refused scan must write nothing", dryRun)
		}
	}
}

// TestScanIssuesLinkedWorktreeRootSucceeds: the sanctioned environment — an
// isolated linked worktree — scans exactly as before.
func TestScanIssuesLinkedWorktreeRootSucceeds(t *testing.T) {
	_, worktree := guardFixtureRepos(t)
	if err := os.CopyFS(worktree, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	issueLoopDir := filepath.Join(worktree, "docs/streams/issue-loop")
	if err := os.MkdirAll(issueLoopDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, issueLoopDir, "README.md", scanStreamREADME)

	data := map[string][]ghIssue{
		scanHomeRepo(): {{Number: 602, Title: "bug", Labels: lbl("bug")}},
	}
	if code := runScanIssues(worktree, false, fixtureLister(data, ""), nilCommentLister, blessAll); code != 0 {
		t.Fatalf("worktree scan exited %d, want 0", code)
	}
	if _, err := os.Stat(filepath.Join(issueLoopDir, "issue-602.md")); err != nil {
		t.Fatalf("worktree scan should have written issue-602.md: %v", err)
	}
}
