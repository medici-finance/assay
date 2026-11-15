package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Rule 3c (the latest PASS run only, #4294) is read by verifyflip and the
// reconcile fold alone: both refuse unless the run 3c stops at carries a
// passing witness for every Verify row. Close-verify, the verify-gate card
// (verifyIssues Path B), the sign-off act and autoflip run no such check, so
// they keep rules 3a and 3b. These tests pin that split on the skipped-row
// shape: an earlier entry records row 2 could-not-check and closes BLOCKED; a
// later entry re-runs row 1 only and closes on a strict PASS.

// chSkippedEvidence is that shape in the closeheld fixture's Verify rows. When
// both is true the later entry re-runs both rows clean instead, the control
// rules 3a and 3b already admit.
func chSkippedEvidence(both bool) string {
	runner := vfRunner + " @ " + vfSHA
	head := "| # | Command | Exit | Result | Date | Runner |\n|---|---------|------|--------|------|--------|\n"
	row := func(n, cmd, exit, result, date string) string {
		return "| " + n + " | `" + cmd + "` | " + exit + " | " + result + " | " + date + " | " + runner + " |\n"
	}
	ev := "### Non-implementer run — 2026-07-10\n\n" + head +
		row("1", "go test ./...", "0", "pass exit=0", "2026-07-10") +
		row("2", "go test ./integration/...", "-1", "could-not-check exit=-1", "2026-07-10") +
		"\n**VERIFY: BLOCKED** — row 2 could not run.\n\n" +
		"### Re-run — 2026-07-11\n\n" + head +
		row("1", "go test ./...", "0", "pass exit=0", "2026-07-11")
	if both {
		ev += row("2", "go test ./integration/...", "0", "pass exit=0", "2026-07-11")
	}
	return ev + "\n**VERIFY: PASS** — re-run green.\n"
}

// chWithEvidence returns a closeheld fixture root whose ch/04 (gate:human,
// implemented) carries ev as its Evidence section.
func chWithEvidence(t *testing.T, ev string) string {
	t.Helper()
	root := loadCHRoot(t)
	p := filepath.Join(root, "docs/streams/ch/brief-04-implemented-held.md")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	i, j := strings.Index(s, "## Evidence\n"), strings.Index(s, "## Review\n")
	if i < 0 || j < i {
		t.Fatal("fixture drifted: ch/04 no longer has an Evidence section before Review")
	}
	s = s[:i] + "## Evidence\n\n" + ev + "\n" + s[j:]
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func chCard(t *testing.T, root string) bool {
	t.Helper()
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, iss := range verifyIssues(root, streams, map[string]bool{}) {
		if iss.Brief == "ch/04" {
			return true
		}
	}
	return false
}

// TestHeldLatestNotReadByNonFlipConsumers: on the skipped-row shape the
// flip-side read drops the earlier hold (3c), and every other consumer still
// refuses on it: close-verify on its implemented and verified paths, the
// verify-gate card, the sign-off act and autoflip's pre-check.
// regression: #4294
func TestHeldLatestNotReadByNonFlipConsumers(t *testing.T) {
	ev := chSkippedEvidence(false)
	if err := flipHeldRefusal("ch/04", "implemented", ev); err != nil {
		t.Fatalf("fixture drifted: the flip-side read must admit the shape (rule 3c), so this test reads the split: %v", err)
	}
	if held, _ := verifyPassHeldContradiction(ev); !held {
		t.Error("verifyPassHeldContradiction (card, sign-off act, autoflip) must still read the earlier hold")
	}
	for _, st := range []string{"implemented", "verified"} {
		if err := closeVerifyHeldRefusal("ch/04", st, ev); err == nil || !strings.Contains(err.Error(), "could-not-check") {
			t.Errorf("closeVerifyHeldRefusal(%s) = %v, want the could-not-check refusal", st, err)
		}
	}
	if got := signOffAct(awaitBrief{Status: "implemented", Gate: "human"}, ev); got != nextActSignOffNoCard {
		t.Errorf("signOffAct = %q, want %q: no card is raised over the hold", got, nextActSignOffNoCard)
	}

	root := chWithEvidence(t, ev)
	if chCard(t, root) {
		t.Error("verifyIssues raised a verify-gate card for ch/04 over the earlier hold")
	}
	readme := filepath.Join(root, "docs/streams/ch/README.md")
	before, err := os.ReadFile(readme)
	if err != nil {
		t.Fatal(err)
	}
	err = closeVerify(root, "ch/04", time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC))
	if err == nil || !strings.Contains(err.Error(), "could-not-check") {
		t.Fatalf("close-verify on the implemented path = %v, want the could-not-check refusal", err)
	}
	if after, _ := os.ReadFile(readme); string(after) != string(before) {
		t.Error("a refused close-verify must not write the README")
	}
}

// TestHeldLatestNonFlipControl: the same consumers admit the shape once the
// later entry re-runs both rows clean (rule 3a), so the refusals above are the
// hold, not the fixture.
// regression: #4294
func TestHeldLatestNonFlipControl(t *testing.T) {
	ev := chSkippedEvidence(true)
	if held, why := verifyPassHeldContradiction(ev); held {
		t.Fatalf("control must read clean, got %q", why)
	}
	root := chWithEvidence(t, ev)
	if !chCard(t, root) {
		t.Error("verifyIssues must raise the ch/04 card once both rows re-ran clean")
	}
	if err := closeVerify(root, "ch/04", time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Errorf("close-verify must admit the control: %v", err)
	}
}
