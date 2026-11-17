package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #6606
func TestCurrentRepoOriginScope(t *testing.T) {
	fixture := func(cmds ...[]string) string {
		t.Helper()
		base := t.TempDir()
		root, linked := filepath.Join(base, "main"), filepath.Join(base, "linked")
		run := func(dir string, args ...string) {
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = dir
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("fixture git %v: %s %v", args, out, err)
			}
		}
		run(base, "init", "-q", "-b", "main", root)
		run(root, "remote", "add", "origin", "git@github.com:owner/repo.git")
		run(root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-q", "--allow-empty", "-m", "fixture")
		run(root, "worktree", "add", "-q", "--detach", linked, "HEAD")
		run(root, "config", "extensions.worktreeConfig", "true")
		for _, args := range cmds {
			run(linked, args...)
		}
		return linked
	}
	wt := func(args ...string) []string { return append([]string{"config", "--worktree"}, args...) }
	reset := wt("--replace-all", "remote.origin.url", "")

	for name, dir := range map[string]string{
		"pin other repo":   fixture(reset, wt("--add", "remote.origin.url", "https://github.com/other/steer.git")),
		"worktree include": fixture(wt("include.path", "steer.cfg")),
	} {
		repo, err := currentRepo(dir)
		if err == nil || !strings.Contains(err.Error(), "worktree-scope or include config") ||
			deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
			t.Errorf("%s: currentRepo must refuse unverifiable, got %q %v", name, repo, err)
		}
	}
	// Control: the transport pin deskwt itself writes over an ssh shared origin.
	pinned := fixture(reset, wt("--add", "remote.origin.url", "https://github.com:443/owner/repo.git"))
	if repo, err := currentRepo(pinned); err != nil || repo != "owner/repo" {
		t.Fatalf("the same-repository pin must decide owner/repo, got %q %v", repo, err)
	}
}
