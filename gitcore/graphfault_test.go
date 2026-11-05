package gitcore

import (
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/medici-finance/assay/gitcore/gittest"
)

// regression: #2395
func TestGraphReadGaps(t *testing.T) {
	f := gittest.NewFixture(t)
	tree := mustGitOutput(t, f, "rev-parse", "HEAD^{tree}")
	mint := func(parents []string, when int) string {
		t.Helper()
		raw := "tree " + tree + "\n"
		for _, parent := range parents {
			raw += "parent " + parent + "\n"
		}
		stamp := strconv.Itoa(when)
		raw += "author F <f@example.invalid> " + stamp + " +0000\ncommitter F <f@example.invalid> " + stamp + " +0000\n\nsynthetic graph\n"
		cmd := exec.Command("git", "-C", f.Dir, "hash-object", "-t", "commit", "--literally", "-w", "--stdin")
		cmd.Stdin = strings.NewReader(raw)
		out, err := cmd.Output()
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(out))
	}
	base := mint(nil, 1000)
	badSide := mint([]string{strings.Repeat("e", 40)}, 1500)
	older := mint([]string{base, badSide}, 2000)
	newer := mint([]string{base}, 3000)
	r, err := Open(f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	native, nativeErr := exec.Command("git", "-C", f.Dir, "merge-base", older, newer).Output()
	candidate, candidateErr := r.MergeBase(older, newer)
	t.Logf("merge-base native=(%q,%v), canonical=(%q,%v)", native, nativeErr, candidate, candidateErr)
	if nativeErr == nil || candidateErr != nil || candidate != base {
		t.Fatal("native merge-base uncertainty vs selected success gap no longer demonstrated")
	}
	nativeParents, parentErr := exec.Command("git", "-C", f.Dir, "show", "-s", "--format=%P", older).Output()
	parents, goParentErr := r.ParentHashes(older)
	if parentErr != nil || goParentErr != nil || strings.Join(parents, " ") != strings.TrimSpace(string(nativeParents)) {
		t.Fatalf("parent metadata differs on missing-grandparent fixture: %v %v %q %v", parents, goParentErr, nativeParents, parentErr)
	}

	nativeErr = exec.Command("git", "-C", f.Dir, "merge-base", "--is-ancestor", base, older).Run()
	related, ancestorErr := r.IsAncestor(base, older)
	t.Logf("ancestor native=%v, canonical=(%v,%v)", nativeErr, related, ancestorErr)
	if nativeErr == nil || ancestorErr != nil || !related {
		t.Fatal("native ancestry uncertainty vs selected true gap no longer demonstrated")
	}
}
