package gitcore

import (
	"bytes"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/medici-finance/assay/gitcore/gittest"
)

// regression: #2395
func TestHeaderReadParity(t *testing.T) {
	f := gittest.NewFixture(t)
	tree := mustGitOutput(t, f, "rev-parse", "HEAD^{tree}")
	raw := "tree " + tree + "\nauthor Alpha <alpha@example.invalid> 1 +0000\nauthor Beta <beta@example.invalid> 1 +0000\ncommitter Test <test@example.invalid> 1 +0000\n\nambiguous author\n"
	cmd := exec.Command("git", "-C", f.Dir, "hash-object", "-t", "commit", "--literally", "-w", "--stdin")
	cmd.Stdin = strings.NewReader(raw)
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	id := strings.TrimSpace(string(out))
	native, err := exec.Command("git", "--no-replace-objects", "-C", f.Dir, "show", id+":seed.txt").Output()
	if err != nil || !bytes.Equal(native, []byte("seed\n")) {
		t.Fatalf("native gap fixture not readable: %q %v", native, err)
	}
	r, err := Open(f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if b, err := r.RawCommit(id); err != nil || string(b) != raw {
		t.Fatalf("raw byte path lost original: %q %v", b, err)
	}
	nativeTime, err := exec.Command("git", "--no-replace-objects", "-C", f.Dir, "show", "-s", "--format=%ct", id).Output()
	if err != nil {
		t.Fatal(err)
	}
	sec, err := strconv.ParseInt(strings.TrimSpace(string(nativeTime)), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := r.repo.CommitObject(plumbing.NewHash(id))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Committer.When.Unix() == sec {
		t.Fatal("retained native timestamp exception no longer demonstrated")
	}
	t.Logf("retained native timestamp: native %d, selected decoded timestamp %d", sec, decoded.Committer.When.Unix())
	candidate, err := r.FileAt(id, "seed.txt")
	if err != nil || candidate != string(native) {
		t.Fatalf("selected FileAt differs from native blob: %q %v", candidate, err)
	}
}
