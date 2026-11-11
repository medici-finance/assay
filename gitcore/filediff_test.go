package gitcore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/gitcore/gittest"
)

func TestFileDiff(t *testing.T) {
	f := gittest.NewFixture(t)
	if err := os.MkdirAll(filepath.Join(f.Dir, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	f.CommitFile(t, "nested/a file.md", "one\ntwo\n", "first")
	f.CommitFile(t, "nested/a file.md", "one\nthree\n", "second")
	f.CommitFile(t, "other.txt", "unrelated\n", "other")
	repo, err := Open(f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	patch, err := repo.FileDiff("HEAD~2", "HEAD", "nested/a file.md")
	if err != nil {
		t.Fatal(err)
	}
	native := mustGitOutput(t, f, "diff", "--unified=3", "HEAD~2", "HEAD", "--", "nested/a file.md")
	if strings.TrimSpace(patch[strings.Index(patch, "@@ "):]) != native[strings.Index(native, "@@ "):] || strings.Contains(patch, "unrelated") {
		t.Fatalf("unexpected single-file patch: %s", patch)
	}
	if _, err := repo.FileDiff("HEAD~2", "HEAD", "other.txt"); err == nil {
		t.Fatal("a file missing from the old tree must remain unreadable")
	}
	mustGit2(t, f, "update-index", "--chmod=+x", "--", "nested/a file.md")
	mustGit2(t, f, "commit", "-m", "mode change")
	if _, err := repo.FileDiff("HEAD~1", "HEAD", "nested/a file.md"); err == nil {
		t.Fatal("a mode change must not become a content-only patch")
	}
}
