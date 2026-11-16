package deskkit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// workSourceFixture builds a checkout holding one qualified brief with the git binary
// (the reader under test must agree with what git itself writes), plus a linked worktree
// of it, and returns the main checkout root, the linked worktree root and the commit.
func workSourceFixture(t *testing.T, worktreeConfig bool) (string, string, string) {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "main")
	linked := filepath.Join(base, "linked")
	run := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("fixture git %v: %s %v", args, out, err)
		}
		return strings.TrimSpace(string(out))
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	run(root, "init", "-q", "-b", "main")
	run(root, "remote", "add", "origin", "https://github.com/owner/repo.git")
	files := map[string]string{
		"docs/streams/graph-repos.yaml":        "schema: graph-repos-v1\ncell: cell\nrepos:\n  repo: {cell: cell, repo: owner/repo}\n",
		"docs/streams/stream/brief-08-work.md": "---\nbrief: cell:repo:stream:08\nid: 00000000-0000-4000-8000-000000006606\nversion: 2\nschema: brief-v2\n---\n# Work\n",
	}
	for name, body := range files {
		file := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	run(root, "add", "docs")
	run(root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-q", "-m", "fixture")
	commit := run(root, "rev-parse", "HEAD")
	run(root, "worktree", "add", "-q", "--detach", linked, "HEAD")
	if worktreeConfig {
		// The shape git writes when per-worktree config is in use: the extension in the
		// shared config at repositoryformatversion 0, and a worktree-scoped value.
		run(root, "config", "extensions.worktreeConfig", "true")
		run(linked, "config", "--worktree", "user.name", "Linked")
	}
	return root, linked, commit
}

// regression: #6606
func TestWorkSourceWorktreeConfig(t *testing.T) {
	for _, ext := range []bool{false, true} {
		root, linked, commit := workSourceFixture(t, ext)
		for _, dir := range []string{root, linked, filepath.Join(linked, "docs", "streams")} {
			for _, repository := range []string{"owner/repo", ""} {
				catalog, err := LocalWorkCatalog(dir, repository)
				if err != nil {
					t.Fatalf("worktreeConfig=%v dir=%s repository=%q: %v", ext, filepath.Base(dir), repository, err)
				}
				selected, err := catalog.Select("stream:b-08@2")
				if err != nil {
					t.Fatalf("worktreeConfig=%v dir=%s: select: %v", ext, filepath.Base(dir), err)
				}
				if selected.Subject.Commit != commit || selected.Subject.Repository != "owner/repo" || selected.ClaimKey != "repo--stream--08" {
					t.Fatalf("worktreeConfig=%v dir=%s: source not pinned: %+v", ext, filepath.Base(dir), selected.Subject)
				}
			}
		}
	}
}

// An extension no reader here supports keeps the work source fail-closed.
//
// regression: #6606
func TestWorkSourceUnknownExtension(t *testing.T) {
	root, _, _ := workSourceFixture(t, true)
	for _, args := range [][]string{
		{"config", "core.repositoryformatversion", "1"},
		{"config", "extensions.exampleUnknown", "true"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("fixture git %v: %s %v", args, out, err)
		}
	}
	_, err := LocalWorkCatalog(root, "owner/repo")
	if err == nil || !strings.Contains(err.Error(), "work source repository unreadable") {
		t.Fatalf("unsupported extension must stay unreadable, got %v", err)
	}
	if ExitCodeOf(err) != ExitUnverifiable {
		t.Fatalf("unsupported extension must be unverifiable, got exit %d", ExitCodeOf(err))
	}
}
