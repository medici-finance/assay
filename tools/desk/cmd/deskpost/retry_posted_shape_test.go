package main

import (
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestVerdictRetryFromAnotherSessionIsNotPostedTwice: a verdict verb posts; the same command
// then runs with no row for it in the audit log, as a re-dispatched reviewer's would. The
// forge holds the first review in the shape the writer posted — with its on-behalf-of line —
// and that review must stop the second post. Both verdict verbs share one post path and one
// forge-state check; each is run here so neither can drift from it.
//
// The older cross-session tests seed the recorded review with the caller's bytes, a shape the
// writer never posts, so they pass whether or not the check can match a real review.
func TestVerdictRetryFromAnotherSessionIsNotPostedTwice(t *testing.T) {
	for _, tc := range []struct{ name, verb, verdict, body string }{
		{"review approve", "review", "approve", okReviewBody},
		{"security-review pass", "security-review", "pass", secBody},
		{"security-review fail", "security-review", "fail", secFailBody},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, errBuf := setupFake(t)
			f.pullHeads = []string{testHead}
			bf := writeBody(t, "v.md", tc.body)
			args := []string{tc.verb, exampleRepo, "1", "--verdict", tc.verdict, "--head", testHead, "--body-file", bf}

			if code := run(args); code != 0 {
				t.Fatalf("first run exit = %d, want 0\nstderr: %s", code, errBuf.String())
			}
			if f.postedReview != 1 {
				t.Fatalf("first run: postedReview = %d, want 1", f.postedReview)
			}
			if len(f.reviews) != 1 || f.reviews[0].Body == tc.body {
				t.Fatalf("fixture defect: the forge does not hold the review with an on-behalf-of line appended: %+v", f.reviews)
			}

			forgetThisSession(t)
			if code := run(args); code != 0 {
				t.Fatalf("re-run exit = %d, want 0\nstderr: %s", code, errBuf.String())
			}
			if f.postedReview != 1 {
				t.Fatalf("postedReview = %d after the re-run, want 1 — a posted review cannot be withdrawn", f.postedReview)
			}
			if e := lastAudit(t); e.Result != deskkit.ResultNoop {
				t.Fatalf("re-run audit result = %q, want noop: %+v", e.Result, e)
			}
		})
	}
}
