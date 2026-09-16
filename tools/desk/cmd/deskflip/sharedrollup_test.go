package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// countRollupReads counts the requests that fetched a CI rollup at a head — the two endpoints
// ChecksAtHead walks. This is the measurement the de-duplication is about: two functions want
// the same rollup, and before this they bought two round trips for it.
func countRollupReads(s *stub) int {
	n := 0
	for _, r := range s.requests {
		if strings.HasSuffix(r.Path, "/check-runs") {
			n++
		}
	}
	return n
}

// TestRollupIsReadOnceAcrossBothConsumers.
//
// readChecks (checks-green) and checkRunsAtHeadReader (reviewer-approved's check-only-CR
// exemption) both need the rollup at the verified head. They were deliberately separate
// FUNCTIONS, for a reason that still stands — "a refusal has to name the condition it belongs
// to or it sends the operator to the wrong gate" — but that argues for two WRAPPERS, not two
// round trips. checksAtHeadOnce is the shared memo; this is the measurement that it works.
//
// The exemption path is used deliberately: it is the one flow in which BOTH consumers
// actually read, so a memo that quietly failed would show up as two.
func TestRollupIsReadOnceAcrossBothConsumers(t *testing.T) {
	s := checkOnlyStub(t)
	s.install(t)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitOK {
		t.Fatalf("the exemption flow did not flip: rc = %d — this test needs the flow that reaches BOTH consumers", rc)
	}
	if n := countRollupReads(s); n != 1 {
		t.Errorf("the check rollup at head was fetched %d times, want exactly 1 — two conditions "+
			"consume it and the memo (checksAtHeadOnce) is what makes that one round trip\nrequests: %v",
			n, s.requests)
	}
}

// TestSharedRollupFailureKeepsEachConditionsOwnName is the property the two-function split
// was protecting, asserted directly now that both functions read through one memo.
//
// A rollup read that fails is could-not-check for BOTH conditions — but each must report it
// under ITS OWN name, because a caller (a loop, a runbook, a human) keys on that name and an
// operator sent to the CI gate for a rejection that was never about CI goes looking in the
// wrong place. The memo caches the RAW error precisely so each wrapper can still say its own
// thing about it.
func TestSharedRollupFailureKeepsEachConditionsOwnName(t *testing.T) {
	// The exemption path: a standing CHANGES_REQUESTED claiming the check-only exemption, so
	// reviewer-approved is the condition that needs the rollup. It runs BEFORE checks-green,
	// so its name is the one that must appear.
	s := checkOnlyStub(t)
	s.checkTotalOverride = 99 // the forge asserts far more runs than it served: a SHORT read
	s.install(t)

	var rc int
	out := captureStderr(t, func() { rc = run([]string{"7", "--repo", privateCIRepo}) })

	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("a short rollup read: rc = %d, want %d (could-not-check)", rc, deskkit.ExitUnverifiable)
	}
	if !strings.Contains(out, "condition "+condReviewerApproved) {
		t.Errorf("the short read was reported under the wrong condition — want %s, got:\n%s",
			condReviewerApproved, out)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("a short rollup read produced mutations: %v", m)
	}

	// And the OTHER consumer must still be able to say its own name: with no exemption claim
	// in play, reviewer-approved never reads the rollup, so the same short read surfaces as
	// checks-green's could-not-check.
	s2 := newStub()
	s2.reviews = approvalAtHead(t, headSHA)
	s2.checkTotalOverride = 99
	s2.install(t)

	var rc2 int
	out2 := captureStderr(t, func() { rc2 = run([]string{"7", "--repo", privateCIRepo}) })

	if rc2 != deskkit.ExitUnverifiable {
		t.Fatalf("a short rollup read on the plain path: rc = %d, want %d", rc2, deskkit.ExitUnverifiable)
	}
	if !strings.Contains(out2, "condition "+condChecksGreen) {
		t.Errorf("the short read on the plain path was reported under the wrong condition — want %s, got:\n%s",
			condChecksGreen, out2)
	}
	if m := s2.mutated(); len(m) != 0 {
		t.Fatalf("a short rollup read produced mutations: %v", m)
	}

	// And the ERROR path, which is a different branch from the short-read reconcile above:
	// the fetch itself fails. On the plain path that is checks-green's could-not-check, and
	// it must say so under its own name — the memo caches the RAW error precisely so this
	// wrapper can still name its own condition.
	s3 := newStub()
	s3.reviews = approvalAtHead(t, headSHA)
	s3.failPath = "/check-runs"
	s3.install(t)

	var rc3 int
	out3 := captureStderr(t, func() { rc3 = run([]string{"7", "--repo", privateCIRepo}) })

	if rc3 != deskkit.ExitUnverifiable {
		t.Fatalf("an unreadable rollup: rc = %d, want %d (could-not-check is never green)", rc3, deskkit.ExitUnverifiable)
	}
	if !strings.Contains(out3, "condition "+condChecksGreen) {
		t.Errorf("an unreadable rollup was reported under the wrong condition — want %s, got:\n%s",
			condChecksGreen, out3)
	}
	if m := s3.mutated(); len(m) != 0 {
		t.Fatalf("an unreadable rollup produced mutations: %v", m)
	}
}

// TestMergeableRefusalCostsOneForgeRead is the cost half of the reorder, measured rather than
// asserted in prose.
//
// `mergeable` reads pr.Mergeable — a field the pr-open-draft read already populated — and
// issues no forge call of its own. Moving it from seventh to fourth means a CONFLICTING PR is
// refused after ONE forge read (the PR document) instead of after four, and in particular
// without buying the paginated label-event timeline the model-floor condition reads. This
// counts the reads rather than trusting the ordering.
func TestMergeableRefusalCostsOneForgeRead(t *testing.T) {
	s := newStub()
	s.pr.Mergeable = "CONFLICTING"
	s.reviews = approvalAtHead(t, headSHA)
	s.install(t)

	var rc int
	out := captureStderr(t, func() { rc = run([]string{"7", "--repo", privateCIRepo}) })

	if rc != deskkit.ExitRefused {
		t.Fatalf("a CONFLICTING PR: rc = %d, want %d", rc, deskkit.ExitRefused)
	}
	if !strings.Contains(out, "condition "+condMergeable) {
		t.Errorf("the refusal does not name its own condition:\n%s", out)
	}
	// No rollup, no reviews, no timeline: the refusal is decided on a field already in hand.
	if n := countRollupReads(s); n != 0 {
		t.Errorf("a mergeable refusal fetched the check rollup %d time(s) — it is decided on the "+
			"PR document alone and must cost nothing beyond it", n)
	}
	for _, r := range s.requests {
		if strings.Contains(r.Path, "/timeline") || strings.Contains(r.Path, "/labels") ||
			strings.Contains(r.Path, "/reviews") || strings.Contains(r.Path, "/files") {
			t.Errorf("a mergeable refusal bought %s — every condition below it is now skipped, "+
				"which is the whole point of moving a zero-read condition up", r.String())
		}
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("a refused flip produced mutations: %v", m)
	}
}
