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
	installStatusgenFixture(t)
	o := vfDefaults()
	o.outcome = "blocked"
	vfRefuses(t, o, "outcome record result mismatch")
}

// regression: #6734
func TestVflipOutcomeOtherRev(t *testing.T) {
	installStatusgenFixture(t)
	o := vfDefaults()
	o.outcome = "none"
	root, readme := vfFixture(t, o)
	vfOutcome(t, root, "verified", "0123456789abcdef", "2026-08-13T12:00:00Z")
	vfCommit(t, root, o.author)
	vfRefusesAt(t, root, readme, "outcome record revision mismatch")
}

// regression: #6734
func TestVflipOutcomeAbsent(t *testing.T) {
	installStatusgenFixture(t)
	o := vfDefaults()
	o.outcome = "none"
	vfRefuses(t, o, "outcome record absent")
}

// regression: #6734
func TestVflipOutcomeAgrees(t *testing.T) {
	installStatusgenFixture(t)
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
	installStatusgenFixture(t)
	root, readme := vfFixture(t, vfDefaults())
	vfOutcome(t, root, "blocked", vfSHA, "2026-08-14T12:00:00Z")
	vfCommit(t, root, vfMailVerifier)
	vfRefusesAt(t, root, readme, "outcome record result mismatch")
}

// regression: #6734
func TestVflipOutcomeUnreadable(t *testing.T) {
	installStatusgenFixture(t)
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
	installStatusgenFixture(t)
	o := vfDefaults()
	o.outcome = "none"
	root, readme := vfFixture(t, o)
	vfOutcomeRaw(t, root, `{"brief":"vf/01","ts":"2026-08-13T12:00:00Z","outcome":"verified"}`)
	vfCommit(t, root, o.author)
	vfCouldNotCheck(t, root, readme, "vf/01", "names no readable revision")
}

// TestVflipOutcomeBadTS: a newer blocked record whose ts is not RFC 3339
// would rank oldest and lose to the older verified one; it is could-not-check.
// regression: #6734 F-sec-2
func TestVflipOutcomeBadTS(t *testing.T) {
	installStatusgenFixture(t)
	root, readme := vfFixture(t, vfDefaults())
	vfWriteAt(t, root, "docs/streams/verify-outcomes/vf/01-20260814T120000Z-aaaaaaaaaaaa.json",
		[]byte(`{"brief":"vf/01","ts":"2026-08-14 12:00:00","outcome":"blocked","sha":"0000000000aa","source_sha":"`+vfSHA+`"}`+"\n"))
	vfCommit(t, root, vfMailVerifier)
	vfCouldNotCheck(t, root, readme, "vf/01", "not RFC 3339")
}

// TestVflipOutcomeLegacyBadTS: the legacy log's tolerance drops a line with an
// unparseable ts; a dropped line naming the brief is could-not-check.
// regression: #6734 F-sec-2
func TestVflipOutcomeLegacyBadTS(t *testing.T) {
	installStatusgenFixture(t)
	root, readme := vfFixture(t, vfDefaults())
	vfWriteAt(t, root, "docs/streams/verify-outcomes.jsonl",
		[]byte(`{"brief":"vf/01","ts":"2026-08-14 12:00:00","outcome":"blocked","source_sha":"`+vfSHA+`"}`+"\n"))
	vfCommit(t, root, vfMailVerifier)
	vfCouldNotCheck(t, root, readme, "vf/01", "was skipped as unreadable")
}

// TestVflipOutcomeMisfiled: a newer record naming the brief filed under
// another stream's directory is counted by the reader and would decide the
// flip; the gate refuses to read past a record that is not at its own name.
// regression: #6734 F-sec-3
func TestVflipOutcomeMisfiled(t *testing.T) {
	installStatusgenFixture(t)
	root, readme := vfFixture(t, vfDefaults())
	line := `{"brief":"vf/01","ts":"2026-08-14T12:00:00Z","outcome":"verified","sha":"0000000000aa","source_sha":"` + vfSHA + `"}`
	name, err := outcomeRecordName([]byte(line))
	if err != nil {
		t.Fatal(err)
	}
	vfWriteAt(t, root, "docs/streams/verify-outcomes/zz/"+filepath.Base(name), outcomeCanonicalBytes([]byte(line)))
	vfCommit(t, root, vfMailVerifier)
	vfCouldNotCheck(t, root, readme, "vf/01", "belong at")
}

// TestVflipOutcomeUntracked: a newer verified record left uncommitted over the
// verifier's committed blocked one refuses, like uncommitted Evidence.
// regression: #6734 F-sec-1
func TestVflipOutcomeUntracked(t *testing.T) {
	installStatusgenFixture(t)
	o := vfDefaults()
	o.outcome = "blocked"
	root, readme := vfFixture(t, o)
	vfOutcome(t, root, "verified", vfSHA, "2026-08-14T12:00:00Z")
	vfRefusesAt(t, root, readme, "uncommitted verify-outcome record")
}

// TestVflipOutcomeNotVerifier: a newer verified record committed by another
// identity does not override the verifier's own blocked record.
// regression: #6734 F-sec-1
func TestVflipOutcomeNotVerifier(t *testing.T) {
	installStatusgenFixture(t)
	o := vfDefaults()
	o.outcome = "blocked"
	root, readme := vfFixture(t, o)
	vfOutcome(t, root, "verified", vfSHA, "2026-08-14T12:00:00Z")
	vfCommit(t, root, vfMailWorker)
	vfRefusesAt(t, root, readme, "not added by the bound verifier")
}

// TestVflipOutcomeLegacyRev: a record without source_sha whose sha is not the
// flip's revision says its sha is likely the Evidence landing commit.
// regression: #6734 A-1
func TestVflipOutcomeLegacyRev(t *testing.T) {
	installStatusgenFixture(t)
	o := vfDefaults()
	o.outcome = "none"
	root, readme := vfFixture(t, o)
	vfOutcomeRaw(t, root, `{"brief":"vf/01","ts":"2026-08-13T12:00:00Z","outcome":"verified","sha":"0000000000aa"}`)
	vfCommit(t, root, o.author)
	vfRefusesAt(t, root, readme, "carries no source_sha")
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
