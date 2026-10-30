package gitcore

import (
	"fmt"
	"github.com/medici-finance/assay/gitcore/gittest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReferenceSnapshotParityAndFreshness(t *testing.T) {
	f := gittest.NewFixture(t)
	mustGitOutput(t, f, "tag", "-a", "annotated", "-m", "tag")
	mustGitOutput(t, f, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/heads/main")
	mustGitOutput(t, f, "symbolic-ref", "refs/heads/alias", "refs/remotes/origin/HEAD")
	mustGitOutput(t, f, "symbolic-ref", "refs/heads/dangling", "refs/heads/absent")
	for i := 0; i < 10; i++ {
		target := "refs/heads/main"
		if i > 0 {
			target = fmt.Sprintf("refs/heads/chain%d", i-1)
		}
		mustGitOutput(t, f, "symbolic-ref", fmt.Sprintf("refs/heads/chain%d", i), target)
	}
	mustGitOutput(t, f, "symbolic-ref", "refs/heads/cycle-a", "refs/heads/cycle-b")
	mustGitOutput(t, f, "symbolic-ref", "refs/heads/cycle-b", "refs/heads/cycle-a")
	r, err := Open(f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	check := func() {
		t.Helper()
		rows, err := r.ReferenceSnapshot()
		if err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		for _, row := range rows {
			fmt.Fprintf(&b, "%s%c%s%c%s\n", row.Name, 0, row.ObjectID, 0, row.SymbolicTarget)
		}
		want := mustGitOutput(t, f, "for-each-ref", "--format=%(refname)%00%(objectname)%00%(symref)")
		if strings.TrimSuffix(b.String(), "\n") != want {
			t.Fatalf("snapshot %q want %q", b.String(), want)
		}
	}
	check()
	mustGitOutput(t, f, "pack-refs", "--all")
	check()
	f.CommitFile(t, "new", "new", "advance main")
	check()
	mustGitOutput(t, f, "update-ref", "-d", "refs/tags/annotated")
	check()
}

func TestLastPathCommitParity(t *testing.T) {
	f := pathHistoryFixture(t)
	r, err := Open(f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	check := func(path string) {
		t.Helper()
		got, err := r.LastPathCommit("HEAD", path)
		if err != nil {
			t.Fatal(err)
		}
		want := mustGitOutput(t, f, "log", "-1", "--format=%H %ct", "--", path)
		actual := ""
		if got != nil {
			actual = fmt.Sprintf("%s %d", got.ID, got.CommitterTime.Unix())
		}
		if actual != want {
			t.Fatalf("%s: %q want %q", path, actual, want)
		}
	}
	for _, p := range []string{"sub/note.txt", "sub", "other.txt", "absent", "."} {
		check(p)
	}
	mustGitOutput(t, f, "checkout", "-b", "side")
	f.CommitFile(t, "sub/note.txt", "side\n", "side change")
	mustGitOutput(t, f, "checkout", "main")
	f.CommitFile(t, "other.txt", "main\n", "main change")
	mustGitOutput(t, f, "merge", "--no-ff", "side", "-m", "merge side")
	check("sub/note.txt")
	check("sub")
	check("other.txt")
	mustGitOutput(t, f, "rm", "sub/note.txt")
	mustGitOutput(t, f, "commit", "-m", "delete")
	check("sub/note.txt")
	check("sub")
	f.CommitFile(t, "mode", "mode\n", "create mode")
	mustGitOutput(t, f, "update-index", "--chmod=+x", "mode")
	mustGitOutput(t, f, "commit", "-m", "chmod")
	check("mode")
	// No cached negative/HEAD answer survives a second call.
	if err := os.MkdirAll(filepath.Join(f.Dir, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	f.CommitFile(t, "sub/note.txt", "restored\n", "restore")
	check("sub/note.txt")
}

func TestAbbreviationCommonDirShadow(t *testing.T) {
	f := gittest.NewFixture(t)
	id := mustGitOutput(t, f, "rev-parse", "HEAD")
	linked := filepath.Join(t.TempDir(), "linked")
	mustGitOutput(t, f, "worktree", "add", "--detach", linked, "HEAD")
	r, err := Open(linked)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.Dir, ".git", id[:7]), []byte(id+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	why, err := r.NativeAbbreviationReason(id[:7])
	if err != nil || why == "" {
		t.Fatalf("common pseudo-ref lost: %q %v", why, err)
	}
}

func TestLastPathCommitNativeHeaderProfile(t *testing.T) {
	f := gittest.NewFixture(t)
	tree := mustGitOutput(t, f, "rev-parse", "HEAD^{tree}")
	raw := "tree " + tree + "\nauthor A <a@example.invalid> 1 +0000\nauthor B <b@example.invalid> 1 +0000\ncommitter C <c@example.invalid> 1 +0000\n\nambiguous author\n"
	cmd := exec.Command("git", "-C", f.Dir, "hash-object", "--literally", "-w", "-t", "commit", "--stdin")
	cmd.Stdin = strings.NewReader(raw)
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	r, err := Open(f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	row, err := r.LastPathCommit(strings.TrimSpace(string(out)), "seed.txt")
	if err != nil || row == nil || row.NativeReason == "" {
		t.Fatalf("decoded timestamp discrepancy lost: %+v %v", row, err)
	}
}
