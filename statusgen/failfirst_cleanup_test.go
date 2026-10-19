package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// regression: #2808
func TestFailFirstCleanupScope(t *testing.T) {
	// Every Git effect below belongs to this temporary repository, never the
	// checkout running go test. Clear repository redirection and caller config.
	for _, key := range []string{"GIT_DIR", "GIT_COMMON_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_CONFIG_PARAMETERS", "BASH_ENV", "ENV"} {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
	home := t.TempDir()
	empty := filepath.Join(home, "empty-config")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", empty)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_COUNT", "0")
	root := filepath.Join(home, "repo")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "init", "-q", "-b", "main", "--template=")
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("fixture content\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", "tracked.txt")
	gitRun(t, root, "commit", "-q", "-m", "fixture")
	base := gitOutT(t, root, "rev-parse", "HEAD")
	common := gitOutT(t, root, "rev-parse", "--git-common-dir")
	if !filepath.IsAbs(common) {
		common = filepath.Join(root, common)
	}
	common = canonicalPath(common)
	if common != canonicalPath(filepath.Join(root, ".git")) {
		t.Fatal("fixture is not an independent Git repository")
	}

	type registration struct{ old, moved, admin string }
	var others []registration
	for _, name := range []string{"other-one", "other-two"} {
		old := filepath.Join(home, name)
		gitRun(t, root, "worktree", "add", "--detach", "--quiet", old, base)
		raw, err := os.ReadFile(filepath.Join(old, ".git"))
		if err != nil {
			t.Fatal(err)
		}
		admin := strings.TrimSpace(strings.TrimPrefix(string(raw), "gitdir: "))
		rel, err := filepath.Rel(common, admin)
		if err != nil || !filepath.IsLocal(rel) {
			t.Fatal("registration escaped the owned fixture")
		}
		moved := old + "-moved"
		if err := os.Rename(old, moved); err != nil {
			t.Fatal(err)
		}
		aged := time.Now().Add(-30 * 24 * time.Hour)
		if err := os.Chtimes(filepath.Join(admin, "gitdir"), aged, aged); err != nil {
			t.Fatal(err)
		}
		others = append(others, registration{old, moved, admin})
	}
	// Positive eligibility proof: the REAL prune command, dry-run only, must
	// select BOTH owned fixture registrations before the production cleanup.
	cmd := exec.Command("git", "-C", root, "worktree", "prune", "--dry-run", "--verbose")
	dry, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("fixture eligibility: %v: %s", err, dry)
	}
	for _, r := range others {
		if !strings.Contains(string(dry), filepath.Base(r.admin)) {
			t.Fatalf("fixture registration is not eligible: %s", filepath.Base(r.admin))
		}
	}
	scratch := filepath.Join(home, "scratch")
	if err := os.Mkdir(scratch, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(key, scratch)
	}
	rows := []verifyRow{{ID: "1", Command: "test -f tracked.txt", Expect: "exit 0"}}
	got, err := runFailFirstBase(resolveShellPlan(), sandboxUnshare, root, base, rows, map[string]bool{"1": true}, 5*time.Second, false)
	if err != nil {
		t.Fatal(err)
	}
	if got["1"].State != baseGreen {
		t.Fatalf("owned base worktree did not execute its fixture check: %+v", got["1"])
	}

	listed := gitOutT(t, root, "worktree", "list", "--porcelain")
	for _, r := range others {
		if _, err := os.Stat(r.admin); err != nil {
			t.Errorf("unrelated eligible registration %s was removed", filepath.Base(r.admin))
		}
		if !strings.Contains(listed, filepath.ToSlash(r.old)) && !strings.Contains(listed, r.old) {
			t.Errorf("unrelated registration %s disappeared from Git", filepath.Base(r.admin))
		}
		content, err := os.ReadFile(filepath.Join(r.moved, "tracked.txt"))
		if err != nil || string(content) != "fixture content\n" {
			t.Errorf("moved fixture content changed: %v", err)
		}
	}
	entries, err := os.ReadDir(scratch)
	if err != nil || len(entries) != 0 {
		t.Errorf("owned temporary worktree/scratch was not removed: %v (%d entries)", err, len(entries))
	}
	// Main plus the TWO unrelated registrations; the owned base registration is gone.
	if n := strings.Count(listed, "worktree "); n != 3 {
		t.Errorf("want main plus two unrelated registrations only, got %d", n)
	}
}
