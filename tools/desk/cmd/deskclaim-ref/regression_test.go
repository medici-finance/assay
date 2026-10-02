package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/regression"
)

// TestReg727WorktreeOrigin pins #727, fixed by 63303b19c: a real
// linked worktree with extensions.worktreeConfig must retain its self-hosted
// origin even when the native git fallback is absent. No remote is contacted.
func TestReg727WorktreeOrigin(t *testing.T) { reg727(t) }

// TestGitDirFixtureIsolation runs the #727 fixture with GIT_DIR, GIT_WORK_TREE and
// GIT_INDEX_FILE naming a second repository, as a git hook running the suite would,
// and requires that repository to be byte-unchanged afterwards.
func TestGitDirFixtureIsolation(t *testing.T) {
	victim := regression.HostileGitDir(t)
	before := regression.TreeDigest(t, victim)
	t.Run("fixture", reg727)
	if after := regression.TreeDigest(t, victim); after != before {
		t.Fatalf("fixture wrote to the GIT_DIR-named repository %s", victim)
	}
}

func reg727(t *testing.T) {
	// A caller's GIT_DIR/GIT_WORK_TREE/GIT_INDEX_FILE would point both the fixture's git
	// children and originRemoteURL's readers at the caller's repository.
	regression.IsolateGit(t)
	dir := t.TempDir()
	main := filepath.Join(dir, "main")
	wt := filepath.Join(dir, "linked")
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	local := func(args ...string) {
		t.Helper()
		cmd := exec.Command(git, args...)
		cmd.Env = regression.FixtureEnv("GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("fixture git %v: %v\n%s", args, err, out)
		}
	}
	local("init", "-q", "-b", "main", main)
	local("-C", main, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-q", "--allow-empty", "-m", "fixture")
	const origin = "https://gitlab.example.invalid/team/repo.git"
	local("-C", main, "remote", "add", "origin", origin)
	local("-C", main, "config", "extensions.worktreeConfig", "true")
	local("-C", main, "worktree", "add", "-q", "-b", "probe", wt)
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(wt); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Error(err)
		}
	})
	t.Setenv("PATH", t.TempDir()) // fixture setup complete: no executable fallback
	if got := originRemoteURL(); got != origin {
		t.Fatalf("linked worktree origin=%q, want %q; never guess a SaaS host", got, origin)
	}
}
