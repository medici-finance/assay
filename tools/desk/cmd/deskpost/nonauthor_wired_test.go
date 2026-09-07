package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestNonAuthorVerdictWired proves the sdlc/10 non-author assertion is a REAL run-time
// control on the verdict-posting path, not only a pure function: postVerdictReview reads
// the head commit's author and REFUSES to post when it is the same actor as the reviewer
// App that would post the verdict. This is the second layer behind the forge's own
// author-cannot-approve refusal, on the collapsed identity path where that refusal may not
// fire.
func TestNonAuthorVerdictWired(t *testing.T) {
	// COLLAPSE (positive path): the head commit's author IS the reviewer App identity that
	// would post the verdict. The verdict must be REFUSED and nothing posted.
	t.Run("collapse_refuses_and_posts_nothing", func(t *testing.T) {
		f, errBuf := setupFake(t)
		f.pullHeads = []string{testHead}
		f.headAuthorLogin = reviewerBotDisplay() // poster == head author
		bf := writeBody(t, "rev.md", okReviewBody)

		code := run(reviewArgs("example-org/tracker", "1", "approve", testHead, bf))
		if code != deskkit.ExitRefused {
			t.Fatalf("collapse exit = %d, want 5 (refused); stderr=%s", code, errBuf.String())
		}
		if f.postedReview != 0 {
			t.Fatalf("a verdict was posted on a collapsed identity path (postedReview=%d)", f.postedReview)
		}
		e := lastAudit(t)
		if e.Result != deskkit.ResultRefused {
			t.Fatalf("audit result = %v, want refused", e.Result)
		}
		// The refusal names both identities (the poster and the head author are the same
		// rendering here, so the one login appears).
		if !strings.Contains(e.Detail, reviewerBotDisplay()) {
			t.Fatalf("refusal detail must name the colliding identity; got %q", e.Detail)
		}
	})

	// DISTINCT (negative path, layer independence): a DIFFERENT head author is PERMITTED —
	// the verdict posts. A control that refused here too would be an outage.
	t.Run("distinct_permits_and_posts", func(t *testing.T) {
		f, errBuf := setupFake(t)
		f.pullHeads = []string{testHead}
		f.headAuthorLogin = "assay-worker-app[bot]" // implementer, distinct from the reviewer
		bf := writeBody(t, "rev.md", okReviewBody)

		code := run(reviewArgs("example-org/tracker", "1", "approve", testHead, bf))
		if code != 0 {
			t.Fatalf("distinct-author exit = %d, want 0; stderr=%s", code, errBuf.String())
		}
		if f.postedReview != 1 {
			t.Fatalf("postedReview = %d, want 1", f.postedReview)
		}
	})

	// COULD-NOT-CHECK FALLBACK: when the head-commit author read FAILS, the control does
	// not vanish — it falls back to the PR author (always present). Here the PR author is
	// the reviewer identity, so the fallback still catches the collapse and refuses.
	t.Run("unreadable_head_author_falls_back_to_pr_author", func(t *testing.T) {
		f, errBuf := setupFake(t)
		f.pullHeads = []string{testHead}
		f.headAuthorErr = true            // GET /commits/{sha} fails → could-not-check
		f.prAuthor = reviewerBotDisplay() // PR author == the poster identity
		f.prAuthorID = 300000004          // the reviewer App's trusted bot id (fixture roster)
		bf := writeBody(t, "rev.md", okReviewBody)

		code := run(reviewArgs("example-org/tracker", "1", "approve", testHead, bf))
		if code != deskkit.ExitRefused {
			t.Fatalf("fallback collapse exit = %d, want 5 (refused); stderr=%s", code, errBuf.String())
		}
		if f.postedReview != 0 {
			t.Fatalf("a verdict posted despite the fallback catching the collapse (postedReview=%d)", f.postedReview)
		}
	})
}
