package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveReposArgsWin(t *testing.T) {
	repos, err := resolveRepos([]string{"example-org/a", "example-org/b"}, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repos) != 2 || repos[0] != "example-org/a" || repos[1] != "example-org/b" {
		t.Fatalf("want the args back verbatim, got %v", repos)
	}
}

func TestResolveReposFromAssayRepoTxt(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".assay"), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "# a comment\n\nexample-org/one\nexample-org/two\n"
	if err := os.WriteFile(filepath.Join(dir, ".assay", "repos.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	repos, err := resolveRepos(nil, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repos) != 2 || repos[0] != "example-org/one" || repos[1] != "example-org/two" {
		t.Fatalf("want [example-org/one example-org/two], got %v (comments/blanks must be ignored)", repos)
	}
}

func TestResolveReposEmptyWhenNothingResolves(t *testing.T) {
	// A tmp dir with no .assay/repos.txt and (almost certainly) no git origin remote.
	repos, err := resolveRepos(nil, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repos) != 0 {
		t.Fatalf("want an empty (never nil-panic, never guessed) result, got %v", repos)
	}
}

func TestValidateReposRefusesBadSlug(t *testing.T) {
	if err := validateRepos([]string{"example-org/example-repo"}); err != nil {
		t.Errorf("valid slug must pass, got %v", err)
	}
	for _, bad := range []string{"no-slash-here", "owner/", "/repo", "owner/repo; rm -rf /"} {
		if err := validateRepos([]string{bad}); err == nil {
			t.Errorf("expected %q to be refused as an invalid repo token", bad)
		}
	}
}
