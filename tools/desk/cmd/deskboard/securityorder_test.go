package main

import (
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func secReview(state, commitID, body string) review {
	var r review
	r.User.Login = reviewerBotDisplay()
	r.State, r.CommitID, r.Body = state, commitID, body
	r.SubmittedAt = "2026-07-30T00:00:00Z"
	return r
}

// TestReduceReviewsSecurityOrderSensitive — the board's securityPass must mirror the
// ready gate: the LAST security verdict at head governs (#216). The old
// reduction latched on ANY pass at head and never parsed `fail`, so a retracted pass
// still rendered the PR as security-green on the board the desk works from.
func TestReduceReviewsSecurityOrderSensitive(t *testing.T) {
	const head = "H"
	pass := "## Security review\n\nSecurity-Review: pass\n"
	fail := "## Security review\n\nSecurity-Review: fail\n"
	silent := "## Review\n\nVerdict: approve\n"

	cases := []struct {
		name    string
		reviews []review
		want    bool
	}{
		{"no security verdict", []review{secReview("APPROVED", head, silent)}, false},
		{"pass only", []review{secReview("APPROVED", head, pass)}, true},
		{"fail only", []review{secReview("CHANGES_REQUESTED", head, fail)}, false},
		{"pass then fail", []review{
			secReview("APPROVED", head, pass),
			secReview("CHANGES_REQUESTED", head, fail),
		}, false},
		{"pass, fail, then a silent approval", []review{
			secReview("APPROVED", head, pass),
			secReview("CHANGES_REQUESTED", head, fail),
			secReview("APPROVED", head, silent),
		}, false},
		{"fail then pass", []review{
			secReview("CHANGES_REQUESTED", head, fail),
			secReview("APPROVED", head, pass),
		}, true},
		{"both markers in one body fails closed", []review{
			secReview("APPROVED", head, "Security-Review: fail\n\nSecurity-Review: pass\n"),
		}, false},
		{"fail at a superseded head is ignored", []review{
			secReview("CHANGES_REQUESTED", "OLD", fail),
			secReview("APPROVED", head, pass),
		}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := reduceReviews(c.reviews, head).securityPass; got != c.want {
				t.Fatalf("securityPass = %v, want %v", got, c.want)
			}
		})
	}
}

// TestReduceReviewsIgnoresNonAppSecurityVerdict — only the reviewer App speaks; a
// `Security-Review: pass` from anyone else must not colour the board.
func TestReduceReviewsIgnoresNonAppSecurityVerdict(t *testing.T) {
	r := secReview("APPROVED", "H", "Security-Review: pass")
	r.User.Login = "some-random-user"
	if reduceReviews([]review{r}, "H").securityPass {
		t.Fatal("a non-App review must never set securityPass")
	}
}

// TestReduceReviewsEmphasisTolerant (#408) proves deskboard now reads an
// EMPHASISED Security-Review line the same way deskpost's ready gate does, instead of
// carrying its own exact, case-sensitive compare.
//
// Before the fix this was worse than "under-counts a pass" (the safe direction #408
// describes for the read-nothing case): classifySecurityBody's OLD hasMarkerLine-based
// reduction returned secNone for an emphasised body, and reduceReviews only updates
// lastSec[author] when the classification is NOT secNone — so a later EMPHASISED
// retraction was silently ignored and the STALE plain `pass` from an earlier review by
// the same author kept governing. securityPass stayed true past a real retraction. This
// test pins the corrected behaviour: the emphasised fail must overwrite the earlier pass.
func TestReduceReviewsEmphasisTolerant(t *testing.T) {
	const head = "H"
	plainPass := "## Security review\n\nSecurity-Review: pass\n"
	emphasisedFail := "## Security review\n\n**Security-Review: fail**\n"
	emphasisedPass := "## Security review\n\n**Security-Review: pass**\n"

	t.Run("emphasised retraction overwrites an earlier plain pass", func(t *testing.T) {
		reviews := []review{
			secReview("APPROVED", head, plainPass),
			secReview("CHANGES_REQUESTED", head, emphasisedFail),
		}
		if got := reduceReviews(reviews, head).securityPass; got {
			t.Fatal("securityPass = true, want false — the emphasised fail must retract the earlier pass")
		}
	})

	t.Run("emphasised pass alone is read as a pass", func(t *testing.T) {
		reviews := []review{secReview("APPROVED", head, emphasisedPass)}
		if got := reduceReviews(reviews, head).securityPass; !got {
			t.Fatal("securityPass = false, want true — an emphasised pass must still be read")
		}
	})

	t.Run("hasSecurityPassLine/hasSecurityFailLine match the deskkit canonical reader", func(t *testing.T) {
		bodies := []string{plainPass, emphasisedFail, emphasisedPass, "security-review: PASS", "* Security-Review: pass"}
		for _, b := range bodies {
			if got, want := hasSecurityPassLine(b), deskkit.HasSecurityReviewPass(b); got != want {
				t.Errorf("hasSecurityPassLine(%q) = %v, want %v (deskkit.HasSecurityReviewPass)", b, got, want)
			}
			if got, want := hasSecurityFailLine(b), deskkit.HasSecurityReviewFail(b); got != want {
				t.Errorf("hasSecurityFailLine(%q) = %v, want %v (deskkit.HasSecurityReviewFail)", b, got, want)
			}
		}
	})

	t.Run("case-insensitive key and value are read as a pass", func(t *testing.T) {
		if !hasSecurityPassLine("security-review: PASS") {
			t.Fatal("a case-varied Security-Review: pass line must still be read")
		}
	})
}
