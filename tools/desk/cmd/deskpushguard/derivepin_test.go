package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/medici-finance/assay/gitcore"
)

// With no URL from git, the PR-state check's repository comes from the named remote; a
// worktree pin to another repository refuses rather than checking that repository's PRs.
//
// regression: #6606
func TestDeriveRepoPinRefused(t *testing.T) {
	dir := t.TempDir()
	runGitT(t, dir, "init", "-q", "-b", "main")
	runGitT(t, dir, "remote", "add", "origin", "https://github.com/example-org/example-repo.git")
	defer chdir(t, dir)()
	if repo, err := deriveRepo("", "origin"); err != nil || repo != "example-org/example-repo" {
		t.Fatalf("control: deriveRepo = %q, %v; want example-org/example-repo", repo, err)
	}
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
