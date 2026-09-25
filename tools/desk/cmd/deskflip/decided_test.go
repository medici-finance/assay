package main

// decided_test.go — the desk-decided condition (attention-budget/19, option A per the
// driver's ruling on #1677: refuse the ready-flip ONLY on a finding).
//
// FAIL-FIRST. Before checkDeskDecided existed, none of these could even compile against the
// package (no condDeskDecided, no deskkit.DeskDecidedLabel reader wired into flip.go), so
// every case here was RED — a build failure, not a wrong verdict. With the condition wired
// but its refusal disarmed (the mutations.json entry this file's tests pin), the same cases
// go GREEN when they should refuse and RED (a nil deref, or a wrong exit code) is caught by
// mutation testing, not by this comment.

import (
	"net/http"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// undeclaredFinding builds the reviewer's finding review — the fixed
// `Undeclared-desk-decision:` line, in the shape the security-pass tests already use for a
// non-verdict-bearing review that must not disturb the correctness lane (COMMENTED, no
// APPROVED/CHANGES_REQUESTED state of its own).
func undeclaredFinding(t *testing.T, head, line string) reviewInfo {
	t.Helper()
	r := reviewInfo{State: "COMMENTED", CommitID: head, Body: "Undeclared-desk-decision: " + line,
		SubmittedAt: "2026-01-01T00:02:00Z"}
	r.User.Login = reviewerBot(t)
	return r
}

// TestFlipRefusedOnUndeclaredDeskDecision is Verify row 5: a reviewer verdict at head
// carrying `Undeclared-desk-decision:` refuses, naming the line; after an edit adds the
// block (and applies the label) and a fresh verdict at the new head omits the line, the PR
// flips.
func TestFlipRefusedOnUndeclaredDeskDecision(t *testing.T) {
	s := newStub()
	s.install(t)
	s.reviews = append(approvalAtHead(t, headSHA), undeclaredFinding(t, headSHA, "chose a default for the retry backoff"))

	rc := run([]string{"7", "--repo", privateCIRepo})
	if rc != deskkit.ExitRefused {
		t.Fatalf("undeclared-desk-decision finding rc = %d, want %d (refused)", rc, deskkit.ExitRefused)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("a standing undeclared-desk-decision finding still produced mutations: %v", m)
	}

	// A DIFFERENT PR (a fresh stub, at another head): the block and label are present, and the
	// reviewer's verdict at that head carries no finding. The SAME-head clear — the path
	// `deskpr edit --decided` actually takes, since an edit moves no head — is pinned
	// separately by TestFlipUndeclaredFindingClearedAtSameHead.
	newHead := "1111111122222222333333334444444455555555"
	s2 := newStub()
	s2.pr.HeadRefOid = newHead
	s2.pr.Labels = []string{labelBeforeFlip, deskkit.DeskDecidedLabel}
	s2.pr.Body = deskkit.RenderDecidedBlock([]deskkit.DecidedItem{
		{Decision: "chose a default for the retry backoff", Alternative: "ask first", Cost: "one revert"},
	})
	s2.install(t)
	s2.reviews = approvalAtHead(t, newHead) // no Undeclared-desk-decision line this time

	rc2 := run([]string{"7", "--repo", privateCIRepo})
	if rc2 != deskkit.ExitOK {
		t.Fatalf("declared decision + clean fresh verdict rc = %d, want 0", rc2)
	}
	if !s2.flipped() {
		t.Errorf("the ready mutation never ran: %v", s2.requests)
	}
}

// TestFlipNoBlockNoFindingUnchanged is Verify row 6, the NEGATIVE row in the other
// direction: an ordinary PR with no Desk-decided block, no desk-decided label, and no
// reviewer finding is NOT refused by this condition — absence alone must never block.
func TestFlipNoBlockNoFindingUnchanged(t *testing.T) {
	s := newStub()
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitOK {
		t.Fatalf("ordinary PR with no block/label/finding rc = %d, want 0 — absence alone must not refuse", rc)
	}
	if !s.flipped() {
		t.Errorf("the ready mutation never ran: %v", s.requests)
	}
}

// TestFlipLabelBlockMismatchRefused is Verify row 7: the label and the block must agree,
// in both directions.
func TestFlipLabelBlockMismatchRefused(t *testing.T) {
	t.Run("label without block", func(t *testing.T) {
		s := newStub()
		s.pr.Labels = []string{labelBeforeFlip, deskkit.DeskDecidedLabel}
		s.install(t)
		s.reviews = approvalAtHead(t, headSHA)

		rc := run([]string{"7", "--repo", privateCIRepo})
		if rc != deskkit.ExitRefused {
			t.Fatalf("label without a block rc = %d, want %d (refused)", rc, deskkit.ExitRefused)
		}
		if m := s.mutated(); len(m) != 0 {
			t.Fatalf("a label/block mismatch still produced mutations: %v", m)
		}
	})

	t.Run("block without label", func(t *testing.T) {
		s := newStub()
		s.pr.Body = deskkit.RenderDecidedBlock([]deskkit.DecidedItem{
			{Decision: "d", Alternative: "a", Cost: "c"},
		})
		s.install(t)
		s.reviews = approvalAtHead(t, headSHA)

		rc := run([]string{"7", "--repo", privateCIRepo})
		if rc != deskkit.ExitRefused {
			t.Fatalf("block without a label rc = %d, want %d (refused)", rc, deskkit.ExitRefused)
		}
		if m := s.mutated(); len(m) != 0 {
			t.Fatalf("a label/block mismatch still produced mutations: %v", m)
		}
	})

	t.Run("both present agrees and flips", func(t *testing.T) {
		s := newStub()
		s.pr.Labels = []string{labelBeforeFlip, deskkit.DeskDecidedLabel}
		s.pr.Body = deskkit.RenderDecidedBlock([]deskkit.DecidedItem{
			{Decision: "d", Alternative: "a", Cost: "c"},
		})
		s.install(t)
		s.reviews = approvalAtHead(t, headSHA)

		if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitOK {
			t.Fatalf("agreeing label+block rc = %d, want 0", rc)
		}
		if !s.flipped() {
			t.Errorf("the ready mutation never ran: %v", s.requests)
		}
	})
}

// TestFlipRefusedOnMalformedDeskDecidedBlock pins the OTHER mechanical half: a
// `## Desk-decided` section that does not parse (here: no marker) refuses, distinctly from
// "no block at all".
func TestFlipRefusedOnMalformedDeskDecidedBlock(t *testing.T) {
	s := newStub()
	s.pr.Labels = []string{labelBeforeFlip, deskkit.DeskDecidedLabel}
	s.pr.Body = deskkit.DeskDecidedHeading + "\n\nno marker, no items\n"
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	rc := run([]string{"7", "--repo", privateCIRepo})
	if rc != deskkit.ExitRefused {
		t.Fatalf("malformed Desk-decided section rc = %d, want %d (refused)", rc, deskkit.ExitRefused)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("a malformed block still produced mutations: %v", m)
	}
}

// reviewAt builds one reviewer-App review at head — the shape every lane-ordering case below
// composes from, so each case states only its state, body and posting time.
func reviewAt(t *testing.T, head, state, body, at string) reviewInfo {
	t.Helper()
	r := reviewInfo{State: state, CommitID: head, Body: body, SubmittedAt: at}
	r.User.Login = reviewerBot(t)
	return r
}

// TestFlipUndeclaredFindingNotLaunderedBySecurityLane pins the lane rule (review findings
// SEC-1 / F1 on this brief's PR): the correctness verdict and the security verdict are posted
// by the SAME reviewer App and run in parallel, so which lands last is arbitrary. A later
// security verdict that never looked at the question must not clear a correctness-lane
// finding — the result must not depend on posting order.
//
// FAIL-FIRST: against the pre-fix "last reviewer-App review at head, any lane" selection,
// both orderings below flipped (rc=0) — the security pass became the governing review.
func TestFlipUndeclaredFindingNotLaunderedBySecurityLane(t *testing.T) {
	const line = "Undeclared-desk-decision: chose a default for the retry backoff"
	cases := []struct {
		name    string
		reviews func(t *testing.T) []reviewInfo
	}{
		{"finding on a COMMENTED correctness note, then a security pass", func(t *testing.T) []reviewInfo {
			return []reviewInfo{
				reviewAt(t, headSHA, "APPROVED", "Verdict: approve", "2026-01-01T00:00:00Z"),
				reviewAt(t, headSHA, "COMMENTED", line, "2026-01-01T00:02:00Z"),
				reviewAt(t, headSHA, "COMMENTED", "Security-Review: pass", "2026-01-01T00:05:00Z"),
			}
		}},
		{"finding on the APPROVE itself, then a security pass", func(t *testing.T) []reviewInfo {
			return []reviewInfo{
				reviewAt(t, headSHA, "APPROVED", "Verdict: approve\n"+line, "2026-01-01T00:00:00Z"),
				reviewAt(t, headSHA, "COMMENTED", "Security-Review: pass", "2026-01-01T00:05:00Z"),
			}
		}},
		{"security pass first, then the finding on the APPROVE (reversed order)", func(t *testing.T) []reviewInfo {
			return []reviewInfo{
				reviewAt(t, headSHA, "COMMENTED", "Security-Review: pass", "2026-01-01T00:00:00Z"),
				reviewAt(t, headSHA, "APPROVED", "Verdict: approve\n"+line, "2026-01-01T00:05:00Z"),
			}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newStub()
			s.install(t)
			s.reviews = c.reviews(t)

			rc := run([]string{"7", "--repo", privateCIRepo})
			if rc != deskkit.ExitRefused {
				t.Fatalf("finding + security-lane review rc = %d, want %d (refused) — a security verdict "+
					"must not clear a correctness-lane finding", rc, deskkit.ExitRefused)
			}
			if m := s.mutated(); len(m) != 0 {
				t.Fatalf("a standing finding was laundered by a security-lane review: %v", m)
			}
		})
	}
}

// TestFlipUndeclaredFindingClearedAtSameHead pins the promised clear path: `deskpr edit
// --decided` moves NO head, so the finding must be clearable by a fresh correctness verdict
// at the SAME head that omits the line — and only by a decisive verdict in the lane that
// raised it. A COMMENTED note that omits the line is not a fresh verdict and clears nothing.
func TestFlipUndeclaredFindingClearedAtSameHead(t *testing.T) {
	const line = "Undeclared-desk-decision: chose a default for the retry backoff"
	declared := deskkit.RenderDecidedBlock([]deskkit.DecidedItem{
		{Decision: "chose a default for the retry backoff", Alternative: "ask first", Cost: "one revert"},
	})

	t.Run("fresh APPROVE at the same head without the line clears", func(t *testing.T) {
		s := newStub()
		s.pr.Labels = []string{labelBeforeFlip, deskkit.DeskDecidedLabel}
		s.pr.Body = declared
		s.install(t)
		s.reviews = []reviewInfo{
			reviewAt(t, headSHA, "APPROVED", "Verdict: approve\n"+line, "2026-01-01T00:00:00Z"),
			reviewAt(t, headSHA, "APPROVED", "Verdict: approve", "2026-01-01T00:10:00Z"),
		}
		if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitOK {
			t.Fatalf("declared + fresh same-head APPROVE without the line rc = %d, want 0", rc)
		}
		if !s.flipped() {
			t.Errorf("the ready mutation never ran: %v", s.requests)
		}
	})

	t.Run("a later COMMENTED correctness note without the line does not clear", func(t *testing.T) {
		s := newStub()
		s.pr.Labels = []string{labelBeforeFlip, deskkit.DeskDecidedLabel}
		s.pr.Body = declared
		s.install(t)
		s.reviews = []reviewInfo{
			reviewAt(t, headSHA, "APPROVED", "Verdict: approve\n"+line, "2026-01-01T00:00:00Z"),
			reviewAt(t, headSHA, "COMMENTED", "a side note, not a verdict", "2026-01-01T00:10:00Z"),
		}
		if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
			t.Fatalf("finding followed only by a COMMENTED note rc = %d, want %d (refused)", rc, deskkit.ExitRefused)
		}
	})

	t.Run("a finding raised in the security lane stands until that lane re-verdicts", func(t *testing.T) {
		s := newStub()
		s.install(t)
		s.reviews = []reviewInfo{
			reviewAt(t, headSHA, "APPROVED", "Verdict: approve", "2026-01-01T00:00:00Z"),
			reviewAt(t, headSHA, "COMMENTED", "Security-Review: pass\n"+line, "2026-01-01T00:02:00Z"),
			reviewAt(t, headSHA, "APPROVED", "Verdict: approve", "2026-01-01T00:05:00Z"),
		}
		if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
			t.Fatalf("security-lane finding followed by a correctness APPROVE rc = %d, want %d (refused) — "+
				"one lane's verdict never clears the other lane's finding", rc, deskkit.ExitRefused)
		}
	})
}

// TestFlipUndeclaredFindingPostedDuringChecksIsCaught pins review finding F2: the head-stable
// re-gate re-reads the reviews because a stable head is not a stable verdict, so it must
// re-run THIS condition against the re-read too — a finding posted at the same head between
// the first reviews read and the mutation is exactly the race that re-read exists to close.
//
// FAIL-FIRST: before the re-gate re-ran checkDeskDecided, this flipped (rc=0).
func TestFlipUndeclaredFindingPostedDuringChecksIsCaught(t *testing.T) {
	s := newStub()
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)
	s.reviewsAfterFirstRead = append(append([]reviewInfo{}, s.reviews...),
		reviewAt(t, headSHA, "COMMENTED", "Undeclared-desk-decision: chose a default", "2026-01-01T00:05:00Z"))

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("finding posted during the checks rc = %d, want %d (refused)", rc, deskkit.ExitRefused)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("the PR was flipped over a finding posted at the same head: %v", m)
	}
	if reads := s.count(http.MethodGet, "/reviews"); reads < 2 {
		t.Errorf("the reviews were read %d time(s) — the pre-mutation re-read is what closes this race", reads)
	}
}
