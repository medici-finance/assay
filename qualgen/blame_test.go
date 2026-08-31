package main

import (
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
)

// TestBlame_ResolvesLastTouchingCommit dereferences the blame helper against real
// history: after A introduces a line and B rewrites it, blaming the file at HEAD
// must attribute the rewritten line to B and an untouched line to A.
func TestBlame_ResolvesLastTouchingCommit(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	szzGit(t, dir, "2020-01-01T00:00:00Z", "init", "-q", "-b", "main")
	a := commitFile(t, dir, "2020-01-01T00:00:00Z", "f.go", "package f\nalpha\nomega\n", "A: base")
	b := commitFile(t, dir, "2020-02-01T00:00:00Z", "f.go", "package f\nBETA\nomega\n", "B: rewrite line 2")

	repo := openRepo(t, dir)
	head, err := repo.CommitObject(plumbing.NewHash(b))
	if err != nil {
		t.Fatalf("resolve head: %v", err)
	}
	blamed, err := blameFile(head, "f.go")
	if err != nil {
		t.Fatalf("blame: %v", err)
	}
	if blamed[2].Commit != b {
		t.Fatalf("line 2 (rewritten) must blame to B=%s, got %s", b, blamed[2].Commit)
	}
	if blamed[1].Commit != a || blamed[3].Commit != a {
		t.Fatalf("lines 1 and 3 (untouched) must blame to A=%s, got %s / %s", a, blamed[1].Commit, blamed[3].Commit)
	}
	// The blamed line's date must be the introducing commit's date, load-bearing for
	// the postdating refinement.
	if blamed[2].Date.IsZero() {
		t.Fatal("blamed line must carry the introducing commit's date")
	}
}

// TestBlame_SelectsOnlyRequestedLines proves blameLines narrows a full blame to the
// requested old-side line numbers — the lines a fix actually changed.
func TestBlame_SelectsOnlyRequestedLines(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	szzGit(t, dir, "2020-01-01T00:00:00Z", "init", "-q", "-b", "main")
	a := commitFile(t, dir, "2020-01-01T00:00:00Z", "f.go", "one\ntwo\nthree\nfour\n", "A")

	repo := openRepo(t, dir)
	c, err := repo.CommitObject(plumbing.NewHash(a))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	got, err := blameLines(c, "f.go", []int{2, 4})
	if err != nil {
		t.Fatalf("blameLines: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected exactly the 2 requested lines, got %d: %+v", len(got), got)
	}
	if _, ok := got[2]; !ok {
		t.Fatal("requested line 2 missing")
	}
	if _, ok := got[1]; ok {
		t.Fatal("unrequested line 1 must not be present")
	}
}

// TestBlame_MissingPathIsError pins the three-state boundary (spec §3.2): a blame
// that cannot run — here, a path absent at the commit — returns a NON-NIL error,
// never an empty-but-successful map a caller could mistake for "zero inducers".
func TestBlame_MissingPathIsError(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	szzGit(t, dir, "2020-01-01T00:00:00Z", "init", "-q", "-b", "main")
	a := commitFile(t, dir, "2020-01-01T00:00:00Z", "present.go", "x\n", "A")

	repo := openRepo(t, dir)
	c, err := repo.CommitObject(plumbing.NewHash(a))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if _, err := blameFile(c, "absent.go"); err == nil {
		t.Fatal("blaming a path absent at the commit must error, not return an empty success")
	}
}
