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
	for _, pin := range []string{
		"https://github.com:443/example-other/example-repo.git",
		// A host that folds onto github.com only under Unicode case mapping (U+0130).
		"https://g\u0130thub.com:443/example-org/example-repo.git",
	} {
		t.Run(pin, func(t *testing.T) {
			pinnedOrigin(t, pin)
			repo, err := resolveRepo("", func() (string, error) { return checkedOriginURL(".") })
			if !errors.Is(err, gitcore.ErrRemoteScopeUnmerged) || repo != "" {
				t.Fatalf("resolveRepo = %q, %v; want a refusal wrapping ErrRemoteScopeUnmerged", repo, err)
			}
			if strings.Contains(err.Error(), "example-other") || strings.Contains(err.Error(), "\u0130") {
				t.Fatalf("refusal echoes the configured value: %v", err)
			}
		})
	}
}

// TestResolveRepoPinAccepted is the control: no pin, and a pin of the same repository.
func TestResolveRepoPinAccepted(t *testing.T) {
	for _, pin := range []string{"", "https://github.com:443/example-org/example-repo.git"} {
		t.Run("pin="+pin, func(t *testing.T) {
			pinnedOrigin(t, pin)
			if repo, err := resolveRepo("", func() (string, error) { return checkedOriginURL(".") }); err != nil || repo != "example-org/example-repo" {
				t.Fatalf("resolveRepo = %q, %v; want example-org/example-repo", repo, err)
			}
		})
	}
	if repo, err := resolveRepo(" example-org/flag ", nil); err != nil || repo != "example-org/flag" {
		t.Fatalf("resolveRepo(--repo) = %q, %v", repo, err)
	}
}

// regression: #6735
func TestClaimStoreOriginChecked(t *testing.T) {
	for _, tc := range []struct{ name, pin string }{
		{"different-repository", "https://github.com/example-org/other.git"},
		{"command-config", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pinnedOrigin(t, tc.pin)
			if tc.pin == "" {
				t.Setenv("GIT_CONFIG_COUNT", "1")
				t.Setenv("GIT_CONFIG_KEY_0", "grep.patternType")
				t.Setenv("GIT_CONFIG_VALUE_0", "default")
			}
			// Both routes must stop before opening this credential file.
			tokenFile := filepath.Join(t.TempDir(), "absent")
			readOrigin := func() (string, error) { return checkedOriginURL(".") }
			for _, flag := range []string{"", "example-org/selected"} {
				repo, err := resolveRepo(flag, readOrigin)
				if flag != "" {
					if err != nil || repo != flag {
						t.Fatalf("explicit repo = %q, %v", repo, err)
					}
					_, err = newForgeStore(repo, tokenFile, readOrigin)
				}
				if !errors.Is(err, gitcore.ErrRemoteScopeUnmerged) {
					t.Errorf("repo flag %q: want checked origin refusal, got %v", flag, err)
				}
			}
		})
	}
}

// regression: #6735
func TestResolveHostAliasChecksResolvedHost(t *testing.T) {
	old := sshConfigHostname
	t.Cleanup(func() { sshConfigHostname = old })
	sshConfigHostname = func(string) (string, error) { return "forgé.example", nil }
	if got, err := resolveHostAlias("fixture"); err == nil || got != "" {
		t.Fatalf("alias resolved to %q, %v; want ASCII hostname refusal", got, err)
	}
}

// regression: #6735
func TestClaimStoreKeepsExplicitTarget(t *testing.T) {
	pinnedOrigin(t, "https://github.com:443/example-org/example-repo.git")
	const target = "example-org/selected"
	readOrigin := func() (string, error) { return checkedOriginURL(".") }
	repo, err := resolveRepo(target, readOrigin)
	if err != nil {
		t.Fatal(err)
	}
	// Only construct the store. No transport operation uses this fixture value.
	t.Setenv("GH_TOKEN", "offline-fixture")
	store, err := newForgeStore(repo, "", readOrigin)
	if err != nil {
		t.Fatal(err)
	}
	if got := store.(*gogitStore).url; got != "https://github.com/example-org/selected.git" {
		t.Fatalf("claim endpoint = %q, want explicit target on github.com", got)
	}
}
