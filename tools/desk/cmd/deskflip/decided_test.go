package main

// decided_test.go — the desk-decided condition (attention-budget/19, option A per the
// driver's ruling on assay#1677: refuse the ready-flip ONLY on a finding).
//
// FAIL-FIRST. Before checkDeskDecided existed, none of these could even compile against the
// package (no condDeskDecided, no deskkit.DeskDecidedLabel reader wired into flip.go), so
// every case here was RED — a build failure, not a wrong verdict. With the condition wired
// but its refusal disarmed (the mutations.json entry this file's tests pin), the same cases
// go GREEN when they should refuse and RED (a nil deref, or a wrong exit code) is caught by
// mutation testing, not by this comment.

import (
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

	// A DIFFERENT PR (a fresh stub — the finding is cleared by an EDIT + a fresh verdict at
	// the edit's new head, not by re-running against the same fixture): the block and label
	// are now present, and the reviewer's fresh verdict at the new head carries no finding.
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
