package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// flipoutcome_test.go — fail-first tests for verifyflip's verify-outcome
// cross-check (#6734). Each refusal names which disagreement it was, and
// vfRefuses asserts the README is untouched.

// regression: #6734
func TestVflipOutcomeBlocked(t *testing.T) {
	o := vfDefaults()
	o.outcome = "blocked"
	vfRefuses(t, o, "outcome record result mismatch")
}

// regression: #6734
func TestVflipOutcomeOtherRev(t *testing.T) {
	o := vfDefaults()
	o.outcome = "none"
	root, readme := vfFixture(t, o)
	vfOutcome(t, root, "verified", "0123456789abcdef", "2026-08-13T12:00:00Z")
	vfCommit(t, root, o.author)
	vfRefusesAt(t, root, readme, "outcome record revision mismatch")
}

// regression: #6734
func TestVflipOutcomeAbsent(t *testing.T) {
	o := vfDefaults()
	o.outcome = "none"
	vfRefuses(t, o, "outcome record absent")
}

// regression: #6734
func TestVflipOutcomeAgrees(t *testing.T) {
	o := vfDefaults()
	o.outcome = "none"
	root, _ := vfFixture(t, o)
	vfOutcome(t, root, "verified", vfSHA+"cd0123456789abcdef0123456789", "2026-08-13T12:00:00Z")
	vfCommit(t, root, o.author)
	if code, out, errOut := runVF(t, root); code != verifyflipExitOK {
		t.Fatalf("exit = %d, want 0; out=%q err=%q", code, out, errOut)
	}
}

// TestVflipOutcomeLatestWins: an older agreeing record does not outvote a
// newer blocked one for the same revision.
// regression: #6734
func TestVflipOutcomeLatestWins(t *testing.T) {
	root, readme := vfFixture(t, vfDefaults())
	vfOutcome(t, root, "blocked", vfSHA, "2026-08-14T12:00:00Z")
	vfCommit(t, root, vfMailVerifier)
	vfRefusesAt(t, root, readme, "outcome record result mismatch")
}

// regression: #6734
func TestVflipOutcomeUnreadable(t *testing.T) {
	root, readme := vfFixture(t, vfDefaults())
	bad := filepath.Join(root, "docs", "streams", "verify-outcomes", "vf", "01-20260814T120000Z-000000000000.json")
	if err := os.WriteFile(bad, []byte("{not json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	vfCommit(t, root, vfMailVerifier)
	vfCouldNotCheck(t, root, readme, "vf/01", "outcome record")
}

// regression: #6734
func TestVflipOutcomeNoRevision(t *testing.T) {
	o := vfDefaults()
	o.outcome = "none"
	root, readme := vfFixture(t, o)
	vfOutcomeRaw(t, root, `{"brief":"vf/01","ts":"2026-08-13T12:00:00Z","outcome":"verified"}`)
	vfCommit(t, root, o.author)
	vfCouldNotCheck(t, root, readme, "vf/01", "names no readable revision")
}

// vfRefusesAt asserts exit 1 naming want, with the README untouched, on an
// already-built root.
func vfRefusesAt(t *testing.T, root, readme, want string) {
	t.Helper()
	before, _ := os.ReadFile(readme)
	code, out, errOut := runVF(t, root)
	if code != verifyflipExitRefused || !strings.Contains(errOut, want) {
		t.Fatalf("exit = %d, want %d naming %q; out=%q err=%q", code, verifyflipExitRefused, want, out, errOut)
	}
	if after, _ := os.ReadFile(readme); string(before) != string(after) {
		t.Fatalf("a refusal wrote the README:\n%s", after)
	}
}
