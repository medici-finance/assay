package deskkit

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// rulingcite_test.go — THE CLASS GUARD for person-attributed, dated ruling citations in
// the published tools/desk tree.
//
// WHAT IT GUARDS. This tree is published from a private origin. A citation of the shape
// "<Name>'s <YYYY-MM-DD> ruling" in source prose ties the public copy back to the
// origin's own operating record: a named person, a dated decision, and (by implication)
// a private tracker entry a reader outside the origin cannot resolve. The residual scrub
// genericizes such citations to "the repository owner's ruling; the dated record lives in
// the private project tracker" — the treatment an earlier ratification citation already
// received. Genericizing the one site is only half the fix: without a guard, the next
// doc-comment written from memory re-introduces the class, and a tokens-only leak sweep
// cannot see it because the shape, not any one token, is the defect.
//
// THE SHAPE. A capitalised word in the possessive, followed by an ISO date:
// rulingCitePattern below. It is deliberately narrow — a possessive name plus a date —
// so ordinary Go prose ("cmdPush's caller", "the owner's ruling", an undated example
// name in a fixture) never trips it. Widening it is a reviewed edit of this file.
//
// THREE STATES, NOT TWO. A walk that reads nothing, or a matcher that no longer matches,
// would report clean while looking at nothing. TestRulingCiteMatcherIsLive runs the SAME
// matcher over a planted instance (assembled at run time, so this file never matches
// itself) and requires the walk to have read a floor of files; either failing is a red
// run, never a pass.

// rulingCiteRoot is the published tools/desk root; this package is tools/desk/internal/deskkit.
const rulingCiteRoot = "../.."

// rulingCitePattern matches "<Capitalised>'s <YYYY-MM-DD>".
var rulingCitePattern = regexp.MustCompile(`[A-Z][a-z]+'s 20[0-9]{2}-[0-9]{2}-[0-9]{2}`)

// rulingCiteMinFiles is a floor on how many text files the walk must read before its
// empty result counts as clean: well under the tree's real size, far above what a
// mis-rooted walk would see.
const rulingCiteMinFiles = 500

// rulingCiteScan walks root and returns "path:line: text" for every line matching
// rulingCitePattern, plus the number of text files read. Binary files (a NUL byte) are
// skipped; this file is exempt so the guard never flags its own documentation.
func rulingCiteScan(t *testing.T, root string) (hits []string, read int) {
	t.Helper()
	self, err := filepath.Abs("rulingcite_test.go")
	if err != nil {
		t.Fatalf("resolving this file: %v", err)
	}
	err = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		if abs, _ := filepath.Abs(p); abs == self {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if bytes.IndexByte(b, 0) >= 0 {
			return nil
		}
		read++
		for i, line := range strings.Split(string(b), "\n") {
			if rulingCitePattern.MatchString(line) {
				rel, _ := filepath.Rel(root, p)
				hits = append(hits, rel+":"+strconv.Itoa(i+1)+": "+strings.TrimSpace(line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	sort.Strings(hits)
	return hits, read
}

// TestNoPersonDatedRulingCite fails, naming every site, when any published tools/desk
// file carries a person-attributed dated ruling citation.
func TestNoPersonDatedRulingCite(t *testing.T) {
	hits, read := rulingCiteScan(t, rulingCiteRoot)
	if read < rulingCiteMinFiles {
		t.Fatalf("could-not-check: the walk read %d text files under %s, below the floor of %d — "+
			"the root is wrong, so an empty result would mean nothing", read, rulingCiteRoot, rulingCiteMinFiles)
	}
	for _, h := range hits {
		t.Errorf("person-attributed dated ruling citation in the published tree: %s\n"+
			"  cite the ruling generically (\"the repository owner's ruling; the dated record lives "+
			"in the private project tracker\") — the public copy must not carry the origin's operating record", h)
	}
}

// TestRulingCiteMatcherIsLive is the positive control: the same matcher must flag a
// planted instance, in a file the same walker reads, or the guard above is blind.
func TestRulingCiteMatcherIsLive(t *testing.T) {
	dir := t.TempDir()
	// Assembled at run time so this source file never contains a matching line.
	planted := "// per " + "Alex" + "'s " + "2020-01-02" + " ruling, the sink stays withheld\n"
	if err := os.WriteFile(filepath.Join(dir, "planted.go"), []byte(planted), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "clean.go"), []byte("// the owner's ruling\n// cmdPush's caller\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	hits, read := rulingCiteScan(t, dir)
	if read != 2 {
		t.Fatalf("walker read %d files in the control dir, want 2", read)
	}
	if len(hits) != 1 || !strings.HasPrefix(hits[0], "planted.go:1:") {
		t.Fatalf("matcher is not live: want exactly the planted line flagged, got %q", hits)
	}
}
