package gitcore

import (
	"strings"
	"testing"

	git "github.com/go-git/go-git/v5"
	"github.com/medici-finance/assay/gitcore/gittest"
)

// This synthetic counterexample pins why native provenance blame is retained.
// A future engine change must repair the differing line assignment before its
// caller moves; upstream decoded author fields are not an identity authority.
func TestBlameNativeBoundary(t *testing.T) {
	f := gittest.NewFixture(t)
	f.CommitFile(t, "note", "b\nc\n\nc\nc\nb\na\nb\na\na\n\n  \n", "base")
	f.CommitFile(t, "note", "b\nc\na\nc\nb\n  \n  \na\na\n  \n  \n\n", "candidate")
	r, err := Open(f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	head, err := r.repo.Head()
	if err != nil {
		t.Fatal(err)
	}
	commit, err := r.repo.CommitObject(head.Hash())
	if err != nil {
		t.Fatal(err)
	}
	upstream, err := git.Blame(commit, "note")
	if err != nil {
		t.Fatal(err)
	}
	native, err := f.Git("--no-replace-objects", "-c", "diff.algorithm=myers", "blame", "--no-ignore-revs-file", "--line-porcelain", "HEAD", "--", "note")
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, line := range strings.Split(native, "\n") {
		fields := strings.Fields(line)
		if (len(fields) == 3 || len(fields) == 4) && len(fields[0]) == 40 {
			want = append(want, fields[0])
		}
	}
	if len(want) != 12 || len(upstream.Lines) != 12 {
		t.Fatalf("incomplete differential fixture: native=%d go-git=%d", len(want), len(upstream.Lines))
	}
	differs := false
	for i, line := range upstream.Lines {
		if line.Hash.String() != want[i] {
			differs = true
			t.Logf("line %d has different introducing commit", i+1)
		}
	}
	if !differs {
		t.Fatal("retained native boundary disappeared: requalify provenance callers before removing retention")
	}
}
