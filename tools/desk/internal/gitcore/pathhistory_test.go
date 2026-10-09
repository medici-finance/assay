package gitcore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/gittest"
)

// pathHistoryFixture is a linear history in which sub/note.txt is absent, added, left
// alone, changed, and left alone again.
func pathHistoryFixture(t *testing.T) *gittest.Fixture {
	t.Helper()
	f := gittest.NewFixture(t)
	if err := os.MkdirAll(filepath.Join(f.Dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	f.CommitFile(t, "sub/note.txt", "one\n", "add the note\n\nTrailer: first")
	f.CommitFile(t, "other.txt", "other\n", "an unrelated change")
	f.CommitFile(t, "sub/note.txt", "two\n", "change the note\r\n\r\nTrailer: second")
	f.CommitFile(t, "other.txt", "other again\n", "another unrelated change")
	return f
}

// TestPathHistoryMatchesGitLog: the walk visits what `git rev-list` lists, in its order, and
// the commits whose path id differs from every parent's are exactly `git log -- <path>`.
func TestPathHistoryMatchesGitLog(t *testing.T) {
	f := pathHistoryFixture(t)
	repo, err := Open(f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	var visited, changed []string
	err = repo.PathHistory("HEAD", "sub/note.txt", func(c PathCommit) bool {
		visited = append(visited, c.Hash)
		if len(c.Parents) != len(c.ParentPathIDs) {
			t.Fatalf("%s: %d parents but %d parent path ids", c.Hash, len(c.Parents), len(c.ParentPathIDs))
		}
		// `git log -- <path>` lists a commit whose path differs from EVERY parent's; a
		// parentless commit is listed when it carries the path.
		differs := c.PathID != ""
		if len(c.ParentPathIDs) > 0 {
			differs = true
			for _, id := range c.ParentPathIDs {
				if c.PathID == id {
					differs = false
				}
			}
		}
		if differs {
			changed = append(changed, c.Hash)
		}
		want, err := f.Git("rev-parse", "--verify", "--quiet", c.Hash+":sub/note.txt")
		if err != nil {
			want = ""
		}
		if c.PathID != want {
			t.Errorf("%s: PathID = %q, want %q (git rev-parse <commit>:<path>)", c.Hash, c.PathID, want)
		}
		if strings.Contains(c.Message, "\r") {
			t.Errorf("%s: the message kept a carriage return: %q", c.Hash, c.Message)
		}
		return true
	})
	if err != nil {
		t.Fatal(err)
	}
	wantVisited, err := f.Git("rev-list", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(visited, "\n"); got != wantVisited {
		t.Errorf("visited:\n%s\nwant (git rev-list HEAD):\n%s", got, wantVisited)
	}
	wantChanged, err := f.Git("log", "--format=%H", "HEAD", "--", "sub/note.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(changed, "\n"); got != wantChanged {
		t.Errorf("commits that changed the path:\n%s\nwant (git log -- sub/note.txt):\n%s", got, wantChanged)
	}
	if len(changed) != 2 {
		t.Fatalf("the fixture changes the path in 2 commits; the walk found %d", len(changed))
	}
}

// TestPathHistoryStopsWhenTheVisitorSaysSo: returning false ends the walk at that commit,
// and stopping is not an error.
func TestPathHistoryStopsWhenTheVisitorSaysSo(t *testing.T) {
	f := pathHistoryFixture(t)
	repo, err := Open(f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	if err := repo.PathHistory("HEAD", "sub/note.txt", func(PathCommit) bool { n++; return n < 2 }); err != nil {
		t.Fatalf("a stopped walk returned %v", err)
	}
	if n != 2 {
		t.Fatalf("the visitor was called %d times after asking to stop at 2", n)
	}
}

// TestPathHistoryDoesNotCallAGapAnAbsence: a commit whose parent cannot be read ends the
// walk with an error. Reporting the path as absent at that parent would let a caller read
// the gap as "this commit added the path".
func TestPathHistoryDoesNotCallAGapAnAbsence(t *testing.T) {
	f := pathHistoryFixture(t)
	clone := filepath.Join(t.TempDir(), "shallow")
	if _, err := f.Git("clone", "-q", "--depth", "2", "file://"+f.Dir, clone); err != nil {
		t.Fatal(err)
	}
	repo, err := Open(clone)
	if err != nil {
		t.Fatal(err)
	}
	var visited []PathCommit
	err = repo.PathHistory("HEAD", "sub/note.txt", func(c PathCommit) bool {
		visited = append(visited, c)
		return true
	})
	if err == nil {
		t.Fatalf("a walk past a shallow boundary returned no error after %d commits", len(visited))
	}
	for _, c := range visited {
		for i, id := range c.ParentPathIDs {
			if id == "" {
				t.Errorf("%s: parent %s was reported as not carrying the path", c.Hash, c.Parents[i])
			}
		}
	}
	if _, err := repo.Resolve("no-such-ref"); err == nil {
		t.Fatal("control: an unknown revision resolved")
	}
	if err := repo.PathHistory("no-such-ref", "sub/note.txt", func(PathCommit) bool { return true }); err == nil {
		t.Fatal("an unknown revision walked")
	}
}
