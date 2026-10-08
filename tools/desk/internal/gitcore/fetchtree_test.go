package gitcore

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/gittest"
)

// fetchTreeSpecs is the refspec the production caller (deskadvisory) fetches with.
var fetchTreeSpecs = []string{"+refs/heads/*:refs/heads/*"}

func TestFetchTreeWritesTreeAndNoRepository(t *testing.T) {
	server := gittest.NewFixture(t)
	if err := os.MkdirAll(filepath.Join(server.Dir, "sub", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	server.CommitFile(t, "sub/deep/a.txt", "deep\n", "nested")
	sha := server.CommitFile(t, "run.sh", "#!/bin/sh\n", "script")
	if _, err := server.Git("update-index", "--chmod=+x", "run.sh"); err != nil {
		t.Fatal(err)
	}
	if _, err := server.Git("commit", "-q", "-m", "exec bit"); err != nil {
		t.Fatal(err)
	}
	sha, _ = server.Git("rev-parse", "HEAD")

	dest := t.TempDir()
	res, err := FetchTree(TreeOpts{URL: server.Dir, RefSpecs: fetchTreeSpecs, Commit: sha}, dest)
	if err != nil {
		t.Fatalf("FetchTree: %v", err)
	}
	if res.Files != 3 || res.Skipped != 0 {
		t.Fatalf("result = %+v, want 3 files, 0 skipped", res)
	}
	if b, err := os.ReadFile(filepath.Join(dest, "sub", "deep", "a.txt")); err != nil || string(b) != "deep\n" {
		t.Fatalf("nested file = %q, %v", b, err)
	}
	if fi, err := os.Stat(filepath.Join(dest, "run.sh")); err != nil || fi.Mode()&0o100 == 0 {
		t.Fatalf("run.sh not executable: %v %v", fi, err)
	}
	// The destination carries the tree and nothing else: no repository, no config, no
	// credential file for a token to be written into.
	if _, err := os.Stat(filepath.Join(dest, ".git")); !os.IsNotExist(err) {
		t.Fatalf("a .git exists under the destination: %v", err)
	}
	entries, _ := os.ReadDir(dest)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if got := strings.Join(names, ","); got != "run.sh,seed.txt,sub" {
		t.Fatalf("destination entries = %s", got)
	}
}

func TestFetchTreeSkipsSymlinksAndSubmodules(t *testing.T) {
	server := gittest.NewFixture(t)
	if err := os.Symlink("/etc/passwd", filepath.Join(server.Dir, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := server.Git("add", "link"); err != nil {
		t.Fatal(err)
	}
	// A gitlink entry (mode 160000) with no .gitmodules: enough to be a submodule in the tree.
	if _, err := server.Git("update-index", "--add", "--cacheinfo", "160000,"+strings.Repeat("a", 40)+",vendor"); err != nil {
		t.Fatal(err)
	}
	if _, err := server.Git("commit", "-q", "-m", "link and gitlink"); err != nil {
		t.Fatal(err)
	}
	sha, _ := server.Git("rev-parse", "HEAD")

	dest := t.TempDir()
	res, err := FetchTree(TreeOpts{URL: server.Dir, RefSpecs: fetchTreeSpecs, Commit: sha}, dest)
	if err != nil {
		t.Fatalf("FetchTree: %v", err)
	}
	if res.Files != 1 || res.Skipped != 2 {
		t.Fatalf("result = %+v, want 1 file, 2 skipped", res)
	}
	for _, n := range []string{"link", "vendor"} {
		if _, err := os.Lstat(filepath.Join(dest, n)); !os.IsNotExist(err) {
			t.Fatalf("%s was written: %v", n, err)
		}
	}
}

// gitStdin runs git in dir with stdin, for the plumbing (hash-object, mktree) that builds a
// tree porcelain refuses to: `git add` will not stage a ".GIT" path, but a hostile server can
// still serve one.
func gitStdin(t *testing.T, dir, stdin string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(stdin)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.invalid",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.invalid")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s: %v", args[0], err)
	}
	return strings.TrimSpace(string(out))
}

func TestFetchTreeRefusesDotGitElement(t *testing.T) {
	server := gittest.NewFixture(t)
	blob := gitStdin(t, server.Dir, "[core]\n", "hash-object", "-w", "--stdin")
	inner := gitStdin(t, server.Dir, "100644 blob "+blob+"\tconfig\n", "mktree")
	mid := gitStdin(t, server.Dir, "040000 tree "+inner+"\t.GIT\n", "mktree")
	root := gitStdin(t, server.Dir, "040000 tree "+mid+"\tx\n", "mktree")
	sha := gitStdin(t, server.Dir, "", "commit-tree", root, "-m", "hostile")
	gitStdin(t, server.Dir, "", "update-ref", "refs/heads/main", sha)

	dest := t.TempDir()
	if _, err := FetchTree(TreeOpts{URL: server.Dir, RefSpecs: fetchTreeSpecs, Commit: sha}, dest); err == nil ||
		!strings.Contains(strings.ToLower(err.Error()), ".git") {
		// go-git's own walker already refuses the ".GIT" element; materialise's check is the
		// second layer behind it, and either one refusing is the contract.
		t.Fatalf("expected a .git-element refusal, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "x", ".GIT", "config")); !os.IsNotExist(err) {
		t.Fatalf("the hostile entry was written: %v", err)
	}
}

func TestFetchTreeRefusesNonCommitAndUnreachable(t *testing.T) {
	server := gittest.NewFixture(t)
	for _, c := range []string{"main", "HEAD", strings.Repeat("0", 40), strings.Repeat("A", 40)} {
		if _, err := FetchTree(TreeOpts{URL: server.Dir, RefSpecs: fetchTreeSpecs, Commit: c}, t.TempDir()); err == nil {
			t.Fatalf("commit %q was accepted", c)
		}
	}
}

func TestFetchTreeErrorDoesNotLeakToken(t *testing.T) {
	const tok = "fixture-fetchtree-token-zz9"
	_, err := FetchTree(TreeOpts{
		URL:      filepath.Join(t.TempDir(), "absent"),
		RefSpecs: fetchTreeSpecs,
		Auth:     BasicAuth(tok),
		Commit:   strings.Repeat("a", 40),
	}, t.TempDir())
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), tok) {
		t.Fatalf("token leaked in error: %v", err)
	}
}
