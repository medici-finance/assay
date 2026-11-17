package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/gitcore"
	"github.com/medici-finance/assay/tools/desk/internal/gitquiet"
	"github.com/medici-finance/assay/tools/desk/internal/regression"
)

// pinnedOrigin builds a checkout whose shared origin is example-org/example-repo and, when
// pin is set, whose worktree-scope config pins origin to it. The test's cwd is the checkout.
func pinnedOrigin(t *testing.T, pin string) {
	t.Helper()
	regression.IsolateGit(t)
	dir := filepath.Join(t.TempDir(), "repo")
	template := gitquiet.TemplateDir(t)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Env = regression.FixtureEnv("GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull,
			"GIT_TEMPLATE_DIR="+template)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("fixture git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main", dir)
	run("-C", dir, "remote", "add", "origin", "https://github.com/example-org/example-repo.git")
	if pin != "" {
		run("-C", dir, "config", "extensions.worktreeConfig", "true")
		run("-C", dir, "config", "--worktree", "--replace-all", "remote.origin.url", "")
		run("-C", dir, "config", "--worktree", "--add", "remote.origin.url", pin)
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Error(err)
		}
	})
}

// A worktree pin to another repository changes where git pushes; the claim's repository is
// then not decided from the shared config alone.
//
// regression: #6606
func TestResolveRepoPinRefused(t *testing.T) {
	pinnedOrigin(t, "https://github.com:443/example-other/example-repo.git")
	repo, err := resolveRepo("")
	if !errors.Is(err, gitcore.ErrRemoteScopeUnmerged) || repo != "" {
		t.Fatalf("resolveRepo = %q, %v; want a refusal wrapping ErrRemoteScopeUnmerged", repo, err)
	}
	if strings.Contains(err.Error(), "example-other") {
		t.Fatalf("refusal echoes the configured value: %v", err)
	}
}

// TestResolveRepoPinAccepted is the control: no pin, and a pin of the same repository.
func TestResolveRepoPinAccepted(t *testing.T) {
	for _, pin := range []string{"", "https://github.com:443/example-org/example-repo.git"} {
		t.Run("pin="+pin, func(t *testing.T) {
			pinnedOrigin(t, pin)
			if repo, err := resolveRepo(""); err != nil || repo != "example-org/example-repo" {
				t.Fatalf("resolveRepo = %q, %v; want example-org/example-repo", repo, err)
			}
		})
	}
	if repo, err := resolveRepo(" example-org/flag "); err != nil || repo != "example-org/flag" {
		t.Fatalf("resolveRepo(--repo) = %q, %v", repo, err)
	}
}
