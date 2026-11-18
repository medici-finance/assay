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
	installDeskkitOriginFixture(t)
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
	installDeskkitOriginFixture(t)
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

// The origin slug comes from the fetch URL alone: a pushurl beside one fetch url is not a
// second origin (the reader used to count it and refuse), while two fetch urls stay refused.
//
// regression: #6606
func TestWorkSourceOriginPushURL(t *testing.T) {
	installDeskkitOriginFixture(t)
	root, _, _ := workSourceFixture(t, false)
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("fixture git %v: %s %v", args, out, err)
		}
	}
	git("config", "remote.origin.pushurl", "https://github.com/other/fork.git")
	catalog, err := LocalWorkCatalog(root, "")
	if err != nil {
		t.Fatalf("one fetch url plus a pushurl must be accepted: %v", err)
	}
	selected, err := catalog.Select("stream:b-08@2")
	if err != nil || selected.Subject.Repository != "owner/repo" {
		t.Fatalf("slug must come from the fetch url: %+v %v", selected.Subject, err)
	}
	git("config", "--add", "remote.origin.url", "https://github.com/other/second.git")
	if _, err := LocalWorkCatalog(root, ""); err == nil || !strings.Contains(err.Error(), "unambiguous work source origin unavailable") {
		t.Fatalf("two fetch urls must stay refused, got %v", err)
	}
}

// Config the repository-local reader does not merge could change origin as native git
// resolves it, so the repository-less read refuses it rather than deriving a slug.
//
// regression: #6606
func TestWorkSourceOriginScope(t *testing.T) {
	installDeskkitOriginFixture(t)
	const steer = "https://github.com/other/steer.git"
	cases := []struct {
		name string
		ext  bool
		at   func(root, linked string) string
		args [][]string
	}{
		{"worktree url, linked", true, func(_, l string) string { return l }, [][]string{{"config", "--worktree", "remote.origin.url", steer}}},
		{"worktree url, main", true, func(r, _ string) string { return r }, [][]string{{"config", "--worktree", "remote.origin.url", steer}}},
		{"worktree reset", true, func(_, l string) string { return l }, [][]string{{"config", "--worktree", "remote.origin.url", ""}}},
		{"worktree insteadOf", true, func(_, l string) string { return l }, [][]string{{"config", "--worktree", "url.https://github.com/other/.insteadOf", "https://github.com/owner/"}}},
		{"worktree include", true, func(_, l string) string { return l }, [][]string{{"config", "--worktree", "include.path", "steer.cfg"}}},
		{"include", false, func(r, _ string) string { return r }, [][]string{{"config", "include.path", "steer.cfg"}}},
		{"includeIf", false, func(_, l string) string { return l }, [][]string{{"config", "includeIf.gitdir:/.path", "steer.cfg"}}},
		{"activation grammar", false, func(r, _ string) string { return r }, [][]string{{"config", "extensions.worktreeConfig", "maybe"}}},
	}
	for _, tc := range cases {
		root, linked, _ := workSourceFixture(t, tc.ext)
		dir := tc.at(root, linked)
		for _, args := range tc.args {
			cmd := exec.Command("git", args...)
			cmd.Dir = dir
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("%s: fixture git %v: %s %v", tc.name, args, out, err)
			}
		}
		_, err := LocalWorkCatalog(dir, "")
		if err == nil || !strings.Contains(err.Error(), "worktree-scope or include config") {
			t.Errorf("%s: origin read must refuse, got %v", tc.name, err)
			continue
		}
		if ExitCodeOf(err) != ExitUnverifiable {
			t.Fatalf("%s: refusal must be unverifiable, got exit %d", tc.name, ExitCodeOf(err))
		}
		if strings.Contains(err.Error(), "other/") || strings.Contains(err.Error(), "steer") {
			t.Fatalf("%s: refusal echoes a configured value: %v", tc.name, err)
		}
		// An explicit repository reads no origin, so it is unaffected.
		if _, err := LocalWorkCatalog(dir, "owner/repo"); err != nil {
			t.Fatalf("%s: explicit repository must still resolve: %v", tc.name, err)
		}
	}
}

// Worktree-scope keys that cannot change origin's fetch list keep the read working, as do
// the same declarations while extensions.worktreeConfig is off (git ignores the file then).
//
// regression: #6606
func TestWorkSourceOriginScopeOK(t *testing.T) {
	installDeskkitOriginFixture(t)
	for _, tc := range []struct {
		name string
		ext  bool
		args [][]string
	}{
		{"pushurl and identity", true, [][]string{
			{"config", "--worktree", "remote.origin.pushurl", "https://github.com/owner/repo.git"},
			{"config", "--worktree", "credential.helper", ""},
			{"config", "--worktree", "url.https://example.invalid/.pushInsteadOf", "https://github.com/"},
		}},
		{"extension off", false, [][]string{{"config", "extensions.worktreeConfig", "false"}}},
	} {
		root, linked, _ := workSourceFixture(t, tc.ext)
		for _, args := range tc.args {
			cmd := exec.Command("git", args...)
			cmd.Dir = linked
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("%s: fixture git %v: %s %v", tc.name, args, out, err)
			}
		}
		if !tc.ext {
			// A config.worktree file that git does not honour while the extension is off.
			gitDir := strings.TrimSpace(fixtureGit(t, linked, "rev-parse", "--absolute-git-dir"))
			if err := os.WriteFile(filepath.Join(gitDir, "config.worktree"), []byte("[remote \"origin\"]\n\turl = https://github.com/other/steer.git\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		for _, dir := range []string{root, linked} {
			catalog, err := LocalWorkCatalog(dir, "")
			if err != nil {
				t.Fatalf("%s %s: %v", tc.name, filepath.Base(dir), err)
			}
			if sel, err := catalog.Select("stream:b-08@2"); err != nil || sel.Subject.Repository != "owner/repo" {
				t.Fatalf("%s %s: slug must come from the shared fetch url: %+v %v", tc.name, filepath.Base(dir), sel.Subject, err)
			}
		}
	}
}

func fixtureGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("fixture git %v: %s %v", args, out, err)
	}
	return string(out)
}
