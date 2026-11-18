package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// originScopeFixture is a checkout with origin set to an https remote, a linked worktree of
// it with extensions.worktreeConfig on, and the given git commands run in the linked tree.
func originScopeFixture(t *testing.T, cmds [][]string) string {
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
	run(root, "remote", "add", "origin", "https://github.com/owner/repo.git")
	run(root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-q", "--allow-empty", "-m", "fixture")
	run(root, "worktree", "add", "-q", "--detach", linked, "HEAD")
	run(root, "config", "extensions.worktreeConfig", "true")
	for _, args := range cmds {
		run(linked, args...)
	}
	return linked
}

// regression: #6606
func TestPreflightOriginScope(t *testing.T) {
	isolateReplyFixture(t)
	wt := func(args ...string) []string { return append([]string{"config", "--worktree"}, args...) }
	steer := "https://github.com/other/steer.git"
	refused := originScopeFixture(t, [][]string{
		wt("--replace-all", "remote.origin.url", ""), wt("--add", "remote.origin.url", steer)})
	_, err := preflight(refused)
	if err == nil || !strings.Contains(err.Error(), "worktree-scope or include config") ||
		deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable || strings.Contains(err.Error(), "steer") {
		t.Fatalf("a worktree pin naming another repository must refuse unverifiable, got %v", err)
	}
	// Control: the transport pin for the same repository decides owner/repo; whether that
	// repository is in the desk-tools set is a later check.
	pinned := originScopeFixture(t, [][]string{
		wt("--replace-all", "remote.origin.url", ""), wt("--add", "remote.origin.url", "https://github.com:443/owner/repo.git")})
	facts, err := preflight(pinned)
	if err != nil && !strings.Contains(err.Error(), "origin owner/repo is not in") {
		t.Fatalf("the same-repository pin must decide owner/repo, got %v", err)
	}
	if err == nil && facts.repo != "owner/repo" {
		t.Fatalf("the same-repository pin decided %q", facts.repo)
	}
}
