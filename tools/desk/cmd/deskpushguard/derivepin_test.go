package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/gitcore"
	"github.com/medici-finance/assay/gitcore/testenv"
	"github.com/medici-finance/assay/tools/desk/internal/regression"
)

// With no URL from git, the PR-state check's repository comes from the named remote; a
// worktree pin to another repository refuses rather than checking that repository's PRs.
//
// regression: #6606
func TestDeriveRepoPinRefused(t *testing.T) {
	isolateDeriveFixture(t)
	dir := t.TempDir()
	runGitT(t, dir, "init", "-q", "-b", "main")
	runGitT(t, dir, "remote", "add", "origin", "https://github.com/example-org/example-repo.git")
	defer chdir(t, dir)()
	if repo, err := deriveRepo("", "origin"); err != nil || repo != "example-org/example-repo" {
		t.Fatalf("control: deriveRepo = %q, %v; want example-org/example-repo", repo, err)
	}
	// Deliberate injection after fixture setup must still be refused.
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "grep.patternType")
	t.Setenv("GIT_CONFIG_VALUE_0", "basic")
	if repo, err := deriveRepo("", "origin"); err == nil || repo != "" {
		t.Fatalf("injected command config accepted: repo=%q error=%v", repo, err)
	}
	t.Setenv("GIT_CONFIG_COUNT", "0")
	runGitT(t, dir, "config", "extensions.worktreeConfig", "true")
	runGitT(t, dir, "config", "--worktree", "--replace-all", "remote.origin.url", "")
	runGitT(t, dir, "config", "--worktree", "--add", "remote.origin.url", "https://github.com:443/example-other/example-repo.git")
	repo, err := deriveRepo("", "origin")
	if !errors.Is(err, gitcore.ErrRemoteScopeUnmerged) || repo != "" {
		t.Fatalf("deriveRepo = %q, %v; want a refusal wrapping ErrRemoteScopeUnmerged", repo, err)
	}
	if strings.Contains(err.Error(), "example-other") {
		t.Fatalf("refusal echoes the configured value: %v", err)
	}
}

// A URL git hands the hook is echoed redacted when it does not parse.
//
// regression: #6606
func TestParseRepoRedacts(t *testing.T) {
	_, err := parseRepo("user:secret@example.invalid:o/r")
	if err == nil || strings.Contains(err.Error(), "secret") || !strings.Contains(err.Error(), "<redacted>@example.invalid") {
		t.Fatalf("parseRepo error = %v; want the userinfo redacted and the host kept", err)
	}
}

func isolateDeriveFixture(t *testing.T) {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	git, err = filepath.Abs(git)
	if err != nil {
		t.Fatal(err)
	}
	testenv.Install(t, testenv.Options{Tools: map[string]string{"git": git}})
}

func TestFixtureSiblingParentIsolation05(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	parent := t.TempDir()
	victim := filepath.Join(parent, "global")
	original := []byte("[fixture]\n\tvalue = parent\n")
	if err := os.WriteFile(victim, original, 0600); err != nil {
		t.Fatal(err)
	}
	repository := filepath.Join(parent, "repository")
	setup := exec.Command(git, "init", "-q", repository)
	setup.Env = []string{"PATH=" + filepath.Dir(git), "HOME=" + parent,
		"GIT_CONFIG_GLOBAL=" + victim, "GIT_CONFIG_SYSTEM=" + os.DevNull, "GIT_CONFIG_NOSYSTEM=1"}
	if systemRoot := os.Getenv("SystemRoot"); systemRoot != "" {
		setup.Env = append(setup.Env, "SystemRoot="+systemRoot)
	}
	if out, err := setup.CombinedOutput(); err != nil {
		t.Fatalf("independent victim setup: %v: %s", err, out)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", victim)
	t.Setenv("GIT_DIR", filepath.Join(repository, ".git"))
	t.Setenv("GIT_WORK_TREE", repository)
	t.Setenv("GIT_INDEX_FILE", filepath.Join(repository, ".git", "index"))
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "grep.patternType")
	t.Setenv("GIT_CONFIG_VALUE_0", "basic")
	control := exec.Command(git, "config", "--global", "fixture.value", "escaped")
	if out, err := control.CombinedOutput(); err != nil {
		t.Fatalf("victim control: %v: %s", err, out)
	}
	changed, err := os.ReadFile(victim)
	if err != nil || bytes.Equal(changed, original) {
		t.Fatal("unsanitized global control did not reach victim")
	}
	if err := os.WriteFile(victim, original, 0600); err != nil {
		t.Fatal(err)
	}
	control = exec.Command(git, "config", "fixture.value", "escaped")
	if out, err := control.CombinedOutput(); err != nil {
		t.Fatalf("repository control: %v: %s", err, out)
	}
	config, err := os.ReadFile(filepath.Join(repository, ".git", "config"))
	if err != nil || !bytes.Contains(config, []byte("escaped")) {
		t.Fatal("unsanitized repository control did not reach victim")
	}
	before := regression.TreeDigest(t, parent)
	// Exercise the actual migrated origin/worktree refusal fixture, not a second
	// copy of its setup. Removing its Install makes its valid-origin control fail.
	t.Run("actual-consumer", TestDeriveRepoPinRefused)
	after, err := os.ReadFile(victim)
	if err != nil || !bytes.Equal(after, original) || regression.TreeDigest(t, parent) != before {
		t.Fatal("fixture modified parent Git configuration or repository")
	}
}
