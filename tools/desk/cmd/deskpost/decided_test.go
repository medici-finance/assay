package main

// decided_test.go — the desk-decided ready-flip condition on `deskpost ready` (#1694).
//
// PAIRED WITH cmd/deskflip/decided_test.go. `deskflip` and `deskpost ready` perform the
// identical markPullRequestReadyForReview mutation, so every PR deskflip refuses on
// desk-decided must also be refused here — otherwise the condition is only as strong as the
// convention that picks the verb. Each case below is the deskpost twin of the deskflip test
// named in its comment, with the SAME fixture shape (labels, body, reviewer reviews at head);
// both verbs run the ONE shared deskkit.DeskDecidedRefusal, and these tests prove the call is
// wired on this verb's path, not merely that the shared function is correct.
//
// FAIL-FIRST: before runReady called the shared condition, every refusal case here flipped
// the PR (exit 0, f.flips == 1) — the #1694 gap.

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const undeclaredLine = "Undeclared-desk-decision: chose a default for the retry backoff"

func declaredBlock() string {
	return deskkit.RenderDecidedBlock([]deskkit.DecidedItem{
		{Decision: "chose a default for the retry backoff", Alternative: "ask first", Cost: "one revert"},
	})
}

// readyFake is the every-other-precondition-green fixture: open+draft, trusted author, green
// CI, non-risk files. Each case sets only the desk-decided inputs.
func readyFake(t *testing.T) *fakeGH {
	t.Helper()
	f, _ := setupFake(t)
	f.status = greenStatus()
	return f
}

func assertReadyRefusedDeskDecided(t *testing.T, f *fakeGH, want string) {
	t.Helper()
	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitRefused {
		t.Fatalf("ready exit = %d, want %d (refused on %s)", code, deskkit.ExitRefused, deskkit.DeskDecidedCondition)
	}
	if f.flips != 0 {
		t.Fatalf("the PR was flipped ready despite the %s refusal (flips=%d)", deskkit.DeskDecidedCondition, f.flips)
	}
	e := lastAudit(t)
	if e.Result != deskkit.ResultRefused {
		t.Fatalf("audit result = %q, want %q", e.Result, deskkit.ResultRefused)
	}
	if !strings.Contains(e.Detail, "condition "+deskkit.DeskDecidedCondition) || !strings.Contains(e.Detail, want) {
		t.Fatalf("refusal detail must name the %s condition and %q; got: %s", deskkit.DeskDecidedCondition, want, e.Detail)
	}
}

func assertReadyFlipped(t *testing.T, f *fakeGH) {
	t.Helper()
	if code := run(readyArgs(exampleRepo)); code != 0 {
		t.Fatalf("ready exit = %d, want 0; audit: %+v", code, lastAudit(t))
	}
	if f.flips != 1 {
		t.Fatalf("flips = %d, want 1", f.flips)
	}
}

// Twin of TestFlipRefusedOnUndeclaredDeskDecision (Verify row 5): a reviewer review at head
// carrying `Undeclared-desk-decision:` refuses, naming the line; a declared block + label and
// a clean fresh verdict at a new head flips.
func TestReadyRefusedOnUndeclaredDeskDecision(t *testing.T) {
	f := readyFake(t)
	f.reviews = []reviewInfo{
		appReview("APPROVED", testHead, okReviewBody),
		appReview("COMMENTED", testHead, undeclaredLine),
	}
	assertReadyRefusedDeskDecided(t, f, "undeclared desk decision")

	f2 := readyFake(t)
	f2.prLabels = []string{deskkit.DeskDecidedLabel}
	f2.prBody = declaredBlock()
	f2.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	assertReadyFlipped(t, f2)
}

// Twin of TestFlipNoBlockNoFindingUnchanged (Verify row 6): absence alone never refuses.
func TestReadyNoBlockNoFindingUnchanged(t *testing.T) {
	f := readyFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	assertReadyFlipped(t, f)
}

// Twin of TestFlipLabelBlockMismatchRefused (Verify row 7): the label and block must agree,
// in both directions.
func TestReadyLabelBlockMismatchRefused(t *testing.T) {
	t.Run("label without block", func(t *testing.T) {
		f := readyFake(t)
		f.prLabels = []string{deskkit.DeskDecidedLabel}
		f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
		assertReadyRefusedDeskDecided(t, f, "the label and the block must agree")
	})
	t.Run("block without label", func(t *testing.T) {
		f := readyFake(t)
		f.prBody = declaredBlock()
		f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
		assertReadyRefusedDeskDecided(t, f, "the label and the block must agree")
	})
	t.Run("both present agrees and flips", func(t *testing.T) {
		f := readyFake(t)
		f.prLabels = []string{deskkit.DeskDecidedLabel}
		f.prBody = declaredBlock()
		f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
		assertReadyFlipped(t, f)
	})
}

// Twin of TestFlipRefusedOnMalformedDeskDecidedBlock: a section that does not parse refuses.
func TestReadyRefusedOnMalformedDeskDecidedBlock(t *testing.T) {
	f := readyFake(t)
	f.prLabels = []string{deskkit.DeskDecidedLabel}
	f.prBody = deskkit.DeskDecidedHeading + "\n\nno marker, no items\n"
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	assertReadyRefusedDeskDecided(t, f, "does not parse")
}

// Twin of TestFlipUndeclaredFindingNotLaunderedBySecurityLane: a security-lane review, in
// either posting order, never clears a correctness-lane finding.
func TestReadyUndeclaredFindingNotLaunderedBySecurityLane(t *testing.T) {
	cases := []struct {
		name    string
		reviews []reviewInfo
	}{
		{"finding on the APPROVE itself, then a security pass", []reviewInfo{
			appReview("APPROVED", testHead, okReviewBody+"\n"+undeclaredLine),
			appReview("COMMENTED", testHead, "Security-Review: pass"),
		}},
		{"security pass first, then the finding on the APPROVE (reversed order)", []reviewInfo{
			appReview("COMMENTED", testHead, "Security-Review: pass"),
			appReview("APPROVED", testHead, okReviewBody+"\n"+undeclaredLine),
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := readyFake(t)
			f.reviews = c.reviews
			assertReadyRefusedDeskDecided(t, f, "correctness review")
		})
	}
}

// Twin of TestFlipUndeclaredFindingClearedAtSameHead: `deskpr edit --decided` moves no head,
// so a fresh correctness APPROVE at the same head that omits the line clears the finding; a
// COMMENTED note that omits it does not.
func TestReadyUndeclaredFindingClearedAtSameHead(t *testing.T) {
	t.Run("fresh APPROVE at the same head without the line clears", func(t *testing.T) {
		f := readyFake(t)
		f.prLabels = []string{deskkit.DeskDecidedLabel}
		f.prBody = declaredBlock()
		// A second APPROVED at an unchanged head, with no CHANGES_REQUESTED between, is an
		// ordinary re-approval (not the #37 no-op-approval shape), so gate (b) still holds.
		f.reviews = []reviewInfo{
			appReview("APPROVED", testHead, okReviewBody+"\n"+undeclaredLine),
			appReview("APPROVED", testHead, okReviewBody),
		}
		assertReadyFlipped(t, f)
	})
	t.Run("a later COMMENTED correctness note without the line does not clear", func(t *testing.T) {
		f := readyFake(t)
		f.prLabels = []string{deskkit.DeskDecidedLabel}
		f.prBody = declaredBlock()
		f.reviews = []reviewInfo{
			appReview("APPROVED", testHead, okReviewBody+"\n"+undeclaredLine),
			appReview("COMMENTED", testHead, "a side note, not a verdict"),
		}
		assertReadyRefusedDeskDecided(t, f, "undeclared desk decision")
	})
}

// Twin of TestFlipUndeclaredFindingPostedDuringChecksIsCaught: a finding posted at the SAME
// head between the precondition reads and the mutation is caught by the pre-mutation
// re-read — a stable head is not a stable verdict.
func TestReadyUndeclaredFindingPostedDuringChecksIsCaught(t *testing.T) {
	f := readyFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.reviewsAfterFirstRead = append(append([]reviewInfo{}, f.reviews...),
		appReview("COMMENTED", testHead, "Undeclared-desk-decision: chose a default"))
	assertReadyRefusedDeskDecided(t, f, "undeclared desk decision")
	if n := f.hitCount("GET", "/reviews"); n < 2 {
		t.Errorf("the reviews were read %d time(s) — the pre-mutation re-read is what closes this race", n)
	}
}

// A label or block edited between the precondition read and the mutation is caught by the
// same re-read: the body and labels are re-read with the head.
func TestReadyDeskDecidedLabelAddedDuringChecksIsCaught(t *testing.T) {
	f := readyFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.pullHeads = []string{testHead, testHead} // the head does NOT move
	f.onSecondPullRead = func() { f.prLabels = []string{deskkit.DeskDecidedLabel} }
	assertReadyRefusedDeskDecided(t, f, "the label and the block must agree")
}

// The condition is evaluated at BOTH reads, as deskflip evaluates it both before its other
// late conditions and on its head-stable re-gate: a finding standing at the first read
// refuses even if the re-read would no longer show it. The flip is decided on state that
// held throughout the checks, never on the last read alone.
func TestReadyDeskDecidedCheckedAtFirstReadToo(t *testing.T) {
	f := readyFake(t)
	f.reviews = []reviewInfo{
		appReview("APPROVED", testHead, okReviewBody),
		appReview("COMMENTED", testHead, undeclaredLine),
	}
	f.reviewsAfterFirstRead = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	assertReadyRefusedDeskDecided(t, f, "undeclared desk decision")
	if n := f.hitCount("GET", "/reviews"); n != 1 {
		t.Errorf("the reviews were read %d time(s), want 1 — the first-read refusal comes before the re-read", n)
	}
}
