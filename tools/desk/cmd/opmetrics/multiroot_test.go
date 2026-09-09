package main

// multiroot_test.go — reading every ~/.claude*/projects and deduping across them.
//
// The live operator runs several Claude profiles; a session synced to two of them
// must be counted ONCE, or the relay ratio's denominator inflates by whatever
// fraction of sessions happen to be mirrored. These tests pin the dedup, the
// skip-a-missing-root NOTICE path, and the zero-readable-roots could-not-check.

import (
	"path/filepath"
	"testing"
)

const (
	fxRootA = "testdata/multiroot/root-a"
	fxRootB = "testdata/multiroot/root-b"
)

// TestMultiRootDedupe is Verify row 5: two roots carrying the same session id
// count its messages once. root-a and root-b both hold session "sess-shared"
// (two turns); root-b also holds "sess-onlyb" (one turn). The merged read must
// see three turns from two files, not five from three.
func TestMultiRootDedupe(t *testing.T) {
	out, skipped, readable, err := ReadOperatorMessagesMulti([]string{fxRootA, fxRootB}, fixtureDay())
	if err != nil {
		t.Fatalf("ReadOperatorMessagesMulti: %v", err)
	}
	if len(skipped) != 0 {
		t.Fatalf("unexpected skipped roots: %v", skipped)
	}
	if readable != 2 {
		t.Fatalf("readable roots = %d, want 2", readable)
	}
	if out.Files != 2 {
		t.Fatalf("files counted = %d, want 2 (the shared session's second copy is deduped, not counted)", out.Files)
	}
	if len(out.Messages) != 3 {
		t.Fatalf("messages = %d, want 3 (2 shared counted once + 1 only-in-b); a naive sum would be 5", len(out.Messages))
	}
}

// TestMultiRootMissingRootIsSkippedNotFatal pins the NOTICE path: a root that
// does not exist is returned in `skipped` and the read still succeeds off the
// roots that do exist. A missing profile on one machine is normal, not a failure.
func TestMultiRootMissingRootIsSkippedNotFatal(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-profile", "projects")
	out, skipped, readable, err := ReadOperatorMessagesMulti([]string{missing, fxRootA}, fixtureDay())
	if err != nil {
		t.Fatalf("a single missing root should not fail the whole read: %v", err)
	}
	if len(skipped) != 1 || skipped[0] != missing {
		t.Fatalf("skipped = %v, want exactly [%s]", skipped, missing)
	}
	if readable != 1 {
		t.Fatalf("readable roots = %d, want 1", readable)
	}
	if len(out.Messages) != 2 {
		t.Fatalf("messages = %d, want 2 (root-a's shared session)", len(out.Messages))
	}
}

// TestMultiRootZeroReadableIsCouldNotCheck is the three-state pin: when NOT ONE
// candidate root can be read, the reader errors so Build reports could-not-check —
// never a confident zero over a blind read.
func TestMultiRootZeroReadableIsCouldNotCheck(t *testing.T) {
	a := filepath.Join(t.TempDir(), "a", "projects")
	b := filepath.Join(t.TempDir(), "b", "projects")
	_, skipped, readable, err := ReadOperatorMessagesMulti([]string{a, b}, fixtureDay())
	if err == nil {
		t.Fatal("no readable root returned no error — a blind read would be reported as zero messages")
	}
	if readable != 0 {
		t.Fatalf("readable roots = %d, want 0", readable)
	}
	if len(skipped) != 2 {
		t.Fatalf("skipped = %v, want both roots", skipped)
	}
}
