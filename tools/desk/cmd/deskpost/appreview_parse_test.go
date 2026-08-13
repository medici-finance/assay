package main

import (
	"testing"

	"github.com/medici-finance/assay/tools/desk/cmd/deskpost/internal/bodycheck"
)

// mkReview builds a reviewInfo for the cross-session guard's table tests.
func mkReview(login, state, commit, body string) reviewInfo {
	var r reviewInfo
	r.User.Login = login
	r.State = state
	r.CommitID = commit
	r.Body = body
	return r
}

// digOf is the incoming-body digest the guard now takes (#518) — shorthand for tests.
func digOf(body string) string { return reviewBodyDigest([]byte(body)) }

// TestAppReviewExistsAtUnparseableBodyNeverSuppresses replaces the #231
// regression test, whose contract (#233: "when the kind is unknowable, assume duplicate")
// is the defect reported by #238/#239.
//
// #231 was right that #224's parse-REQUIRED condition rested on a false premise — most
// real App bodies do not parse, so requiring a parse posted permanent duplicates. #233's
// remedy went one step too far: an unreadable body suppressed EVERY subsequent verdict at
// that head, including a `Security-Review: fail` of the opposite polarity. The escape it
// offered ("re-post with an explicit Verdict: line") does not exist — this guard reads the
// EXISTING body and never the re-poster's.
//
// The contract now: an unreadable existing body never suppresses, and the guard says so.
// #73's duplicate protection is unaffected because deskpost's own write path (Review +
// VerdictKind, both refusing) guarantees every review it has ever posted parses to exactly
// one kind — so a fresh-session replay is caught by the KIND arm, and an unreadable review
// at head can only have been written out-of-band.
func TestAppReviewExistsAtUnparseableBodyNeverSuppresses(t *testing.T) {
	const head = "0c92fce6e2b1a4d5c6f70819aabbccddeeff0011"

	for _, tc := range []struct {
		name string
		body string
	}{
		{"empty body", ""},
		{"prose with no verdict line", "Looks good to me. Shipping."},
		{"verdict inside emphasis (the common real shape)", "**Verdict:** approve\n\nDetail follows."},
		{"quoted verdict only", "> Verdict: approve\n\nQuoting the earlier review."},
	} {
		for _, want := range []string{"correctness", "security"} {
			t.Run(tc.name+" / "+want+" verdict still posts", func(t *testing.T) {
				reviews := []reviewInfo{mkReview(reviewerBotDisplay(), "APPROVED", head, tc.body)}
				dup, why := appReviewExistsAt(reviews, head, "APPROVED", want, digOf(okReviewBody))
				if dup {
					t.Fatal("an UNREADABLE review at head suppressed a verdict — that is #238/#239: " +
						"a dropped verdict reads as review coverage that never happened")
				}
				if why == "" {
					t.Error("posting over an unreadable review at head must be REPORTED, not silent")
				}
			})
		}
	}
}

// TestAppReviewExistsAtSecurityFailLandsBehindUnparseable is PROBE-A from #239, at the
// unit level: the exact case that used to be swallowed at exit 0. A `Security-Review: fail`
// is a RETRACTION — if it is dropped, the earlier `pass` stays the visible state and the
// PR can flip on a verdict its reviewer tried to withdraw.
func TestAppReviewExistsAtSecurityFailLandsBehindUnparseable(t *testing.T) {
	const head = "1a2b3c4d5e6f70819aabbccddeeff00112233445"
	reviews := []reviewInfo{
		mkReview(reviewerBotDisplay(), "APPROVED", head, "## Security review\n\nSecurity-Review: pass\n"),
		mkReview(reviewerBotDisplay(), "CHANGES_REQUESTED", head, "Red CI, blocking. No verdict line here."),
	}
	if dup, _ := appReviewExistsAt(reviews, head, "CHANGES_REQUESTED", "security", digOf(secFailBody)); dup {
		t.Fatal("PROBE-A: a Security-Review: fail behind an unparseable CHANGES_REQUESTED at the " +
			"same head was suppressed — the retraction can never land (#238, #239)")
	}
}

// TestAppReviewExistsAtPreservesKindSplit pins the #220 fix that #231's
// repair must NOT undo: when the body DOES parse, a different kind is not a match, so
// both required verdicts of a risk-classed PR still land on the fresh-session path.
func TestAppReviewExistsAtPreservesKindSplit(t *testing.T) {
	const head = "a485560d1122334455667788990011223344aabb"

	// Single-kind bodies, asserted parseable up front. NO skip path: a test that can
	// silently opt out of its own assertion proves nothing, which is the whole subject
	// of #229.
	for _, tc := range []struct {
		body, wantKind, otherKind string
	}{
		{"Verdict: approve\n\nLooks correct.", "correctness", "security"},
		{"Security-Review: pass\n\nNo issues found.", "security", "correctness"},
	} {
		k, err := bodycheck.VerdictKind([]byte(tc.body))
		if err != nil {
			t.Fatalf("fixture %q must parse for this test to mean anything, got error: %v", tc.body, err)
		}
		if k != tc.wantKind {
			t.Fatalf("fixture %q parsed as %q, expected %q", tc.body, k, tc.wantKind)
		}

		existing := []reviewInfo{mkReview(reviewerBotDisplay(), "APPROVED", head, tc.body)}

		if dup, _ := appReviewExistsAt(existing, head, "APPROVED", tc.wantKind, digOf(tc.body)); !dup {
			t.Errorf("same parsed kind (%s) with an identical body at head must match — dedup must survive the #231 fix", tc.wantKind)
		}
		if dup, _ := appReviewExistsAt(existing, head, "APPROVED", tc.otherKind, digOf(tc.body)); dup {
			t.Errorf("a parseable %s review must NOT suppress a %s verdict — that is #220 reopening",
				tc.wantKind, tc.otherKind)
		}
	}
}

// TestAppReviewExistsAtIgnoresNonApp pins the login filter, which #231
// reported as unpinned (M5d): a HUMAN approval at head must not suppress the App's
// required verdict. Behaviour is unchanged; only the coverage is added.
func TestAppReviewExistsAtIgnoresNonApp(t *testing.T) {
	const head = "b1c2d3e4f5061728394a5b6c7d8e9f0011223344"
	reviews := []reviewInfo{
		mkReview("ada", "APPROVED", head, ""),
		mkReview("some-other-bot[bot]", "APPROVED", head, "Verdict: approve\n"),
	}
	if dup, _ := appReviewExistsAt(reviews, head, "APPROVED", "correctness", digOf("Verdict: approve\n")); dup {
		t.Error("a non-App review at head must not satisfy the App's own cross-session guard")
	}
}

// TestAppReviewExistsAtRequiresHeadAndState keeps the other two selectors honest.
func TestAppReviewExistsAtRequiresHeadAndState(t *testing.T) {
	const head = "c1c2d3e4f5061728394a5b6c7d8e9f0011223344"
	const older = "d1c2d3e4f5061728394a5b6c7d8e9f0011223344"

	if dup, _ := appReviewExistsAt([]reviewInfo{mkReview(reviewerBotDisplay(), "APPROVED", older, "")}, head, "APPROVED", "correctness", digOf("")); dup {
		t.Error("a review at a DIFFERENT head must not match")
	}
	if dup, _ := appReviewExistsAt([]reviewInfo{mkReview(reviewerBotDisplay(), "CHANGES_REQUESTED", head, "")}, head, "APPROVED", "correctness", digOf("")); dup {
		t.Error("a review with a different STATE must not match")
	}
}
