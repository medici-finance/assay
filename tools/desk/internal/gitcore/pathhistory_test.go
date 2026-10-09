package gitcore

import (
	"fmt"
	"os"
	"os/exec"
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

// TestPathHistoryAtAMerge: a history with two lines whose commit times interleave, joined by
// a merge. The walk visits them in `git rev-list` order (committer time, not one line and
// then the other), and at the merge it reports the path's id at EACH parent — the merge took
// the path from its second parent, so it did not change it, and a walk that looked at the
// first parent only would say it did.
func TestPathHistoryAtAMerge(t *testing.T) {
	f := gittest.NewFixture(t)
	clock := 0
	git := func(args ...string) string {
		t.Helper()
		clock++
		when := fmt.Sprintf("%d +0000", 1000000000+clock)
		cmd := exec.Command("git", args...)
		cmd.Dir = f.Dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.invalid",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.invalid",
			"GIT_AUTHOR_DATE="+when, "GIT_COMMITTER_DATE="+when)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	commit := func(path, content, msg string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(f.Dir, path), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		git("add", path)
		git("commit", "-q", "-m", msg)
	}
	commit("note.txt", "one\n", "add the note")
	git("checkout", "-q", "-b", "side")
	commit("note.txt", "two\n", "side: change the note")
	git("checkout", "-q", "main")
	commit("other.txt", "other\n", "main: an unrelated change")
	git("checkout", "-q", "side")
	commit("side.txt", "side\n", "side: an unrelated change")
	git("checkout", "-q", "main")
	git("merge", "-q", "--no-ff", "-m", "merge side", "side")
	merge := git("rev-parse", "HEAD")

	repo, err := Open(f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	var visited []string
	sawMerge := false
	err = repo.PathHistory("HEAD", "note.txt", func(c PathCommit) bool {
		visited = append(visited, c.Hash)
		for i, p := range c.Parents {
			// Absent at the seed commit: rev-parse fails there and the walk reports "".
			want, err := f.Git("rev-parse", "--verify", "--quiet", p+":note.txt")
			if err != nil {
				want = ""
			}
			if c.ParentPathIDs[i] != want {
				t.Errorf("%s: path id at parent %d (%s) = %q, want %q", c.Hash, i, p, c.ParentPathIDs[i], want)
			}
		}
		if c.Hash == merge {
			sawMerge = true
			if len(c.Parents) != 2 {
				t.Fatalf("the merge has %d parents, want 2", len(c.Parents))
			}
			if c.PathID == c.ParentPathIDs[0] || c.PathID != c.ParentPathIDs[1] {
				t.Errorf("the merge holds the note as its second parent does and not as its first; got %q against %q",
					c.PathID, c.ParentPathIDs)
			}
		}
		return true
	})
	if err != nil {
		t.Fatal(err)
	}
	if !sawMerge {
		t.Fatal("the walk did not visit the merge")
	}
	if got, want := strings.Join(visited, "\n"), git("rev-list", "HEAD"); got != want {
		t.Errorf("visited:\n%s\nwant (git rev-list HEAD):\n%s", got, want)
	}
	// Control: the two lines interleave, so the order asked for is not one line then the other.
	if firstParentFirst := git("rev-list", "--topo-order", "HEAD"); firstParentFirst == strings.Join(visited, "\n") {
		t.Fatalf("control: this fixture cannot tell commit-time order from a line-by-line one")
	}
}
