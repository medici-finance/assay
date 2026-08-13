package main

import (
	"testing"

	"github.com/medici-finance/assay/tools/desk/cmd/deskpost/internal/bodycheck"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The cross-session kind check (appReviewExistsAt) was pinned in only ONE lane direction:
// every existing test seeds the App's SECURITY verdict at head and posts the CORRECTNESS
// one. Bypassing the kind check *only* when wantKind == security therefore left the whole
// suite green (#231) — a half-fix of exactly the shape that let
// #212 through twice.
//
// The untested direction is also the MORE COMMON real-world order: correctness lands
// first, then the security lane runs in its own window with an empty local audit log, so
// the cross-session guard is the only thing standing between it and a dropped verdict.

// TestReviewSecurityPostsOverCorrectnessAcrossSessions — the M2c direction. The App's
// CORRECTNESS approve is already at head and the local log is empty (fresh window); the
// SECURITY verdict is a DIFFERENT required artifact and must still post.
func TestReviewSecurityPostsOverCorrectnessAcrossSessions(t *testing.T) {
	f, _ := setupFake(t)
	f.pullHeads = []string{testHead}
	f.reviews = []reviewInfo{appReviewAt("APPROVED", testHead, okReviewBody)}
	bf := writeBody(t, "sec.md", okSecurityBody)

	code := run(reviewArgs("example-org/tracker", "1", "approve", testHead, bf))
	if code != 0 {
		t.Fatalf("security verdict exit = %d, want 0", code)
	}
	if f.postedReview != 1 {
		t.Fatalf("postedReview = %d, want 1 — the App's CORRECTNESS approve at this head is "+
			"not the security verdict and must not suppress it (#220, #231 M2c)", f.postedReview)
	}
	if lastAudit(t).Result != deskkit.ResultOK {
		t.Fatalf("audit = %s, want ok", lastAudit(t).Result)
	}
}

// TestReviewSecurityNoopsOverSecurityAcrossSessions — the same direction's other half, so
// the test above cannot be satisfied by a guard that simply stopped suppressing security
// verdicts altogether. Same kind at the same head still no-ops (#73).
func TestReviewSecurityNoopsOverSecurityAcrossSessions(t *testing.T) {
	f, _ := setupFake(t)
	f.pullHeads = []string{testHead}
	f.reviews = []reviewInfo{appReviewAt("APPROVED", testHead, okSecurityBody)}
	bf := writeBody(t, "sec.md", okSecurityBody)

	code := run(reviewArgs("example-org/tracker", "1", "approve", testHead, bf))
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (noop)", code)
	}
	if f.postedReview != 0 {
		t.Fatalf("posted %d duplicate security review(s); want 0", f.postedReview)
	}
	if lastAudit(t).Result != deskkit.ResultNoop {
		t.Fatalf("audit = %s, want noop", lastAudit(t).Result)
	}
}

// TestReviewHumanApproveDoesNotSuppressAppVerdict — the App-login filter (#231 M5d). A
// HUMAN APPROVED at the same head is not the App's required verdict; if the filter were
// dropped, the App's verdict would be silently suppressed as an "idempotent no-op" and
// the gate would be left with no App artifact at all.
func TestReviewHumanApproveDoesNotSuppressAppVerdict(t *testing.T) {
	f, _ := setupFake(t)
	f.pullHeads = []string{testHead}
	human := appReviewAt("APPROVED", testHead, okReviewBody)
	human.User.Login = "ada"
	f.reviews = []reviewInfo{human}
	bf := writeBody(t, "cor.md", okReviewBody)

	code := run(reviewArgs("example-org/tracker", "1", "approve", testHead, bf))
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if f.postedReview != 1 {
		t.Fatalf("postedReview = %d, want 1 — a HUMAN approve at this head is not the App's "+
			"verdict and must not suppress it (#231 M5d)", f.postedReview)
	}
}

// TestReviewVerbCarriesTheFlagNotJustTheKind — the FLAG half of the idempotency key
// (#231 M5a). reviewVerbFor's own comment says "Both components are needed: dropping the
// flag would merge a request-changes into an approve of the same kind" — but nothing
// asserted it, and constant-folding the flag to "approve" left the suite green. A
// request-changes and an approve of the SAME kind at the SAME head are different writes.
func TestReviewVerbCarriesTheFlagNotJustTheKind(t *testing.T) {
	if got := reviewVerbFor(bodycheck.KindCorrectness, "approve"); got != "review:correctness:approve" {
		t.Fatalf("verb = %q, want review:correctness:approve", got)
	}
	if got := reviewVerbFor(bodycheck.KindCorrectness, "request-changes"); got != "review:correctness:request-changes" {
		t.Fatalf("verb = %q, want review:correctness:request-changes", got)
	}
	if reviewVerbFor(bodycheck.KindCorrectness, "approve") == reviewVerbFor(bodycheck.KindCorrectness, "request-changes") {
		t.Fatal("approve and request-changes of the same kind share an idempotency verb — " +
			"a verdict CHANGE at one head would be dropped as a duplicate (#231 M5a)")
	}
}

// TestReviewRequestChangesReachesTheWriteFlow — #231 M5b: --verdict request-changes never
// reached runReview in any test, so the whole REQUEST_CHANGES path (event mapping, state
// mapping, verb construction) was exercised only through unit-level helpers.
func TestReviewRequestChangesReachesTheWriteFlow(t *testing.T) {
	f, _ := setupFake(t)
	f.pullHeads = []string{testHead}
	bf := writeBody(t, "cor.md", "## Review\n\nBlocking on the auth path.\n\nVerdict: request-changes\n")

	code := run(reviewArgs("example-org/tracker", "1", "request-changes", testHead, bf))
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if f.postedReview != 1 {
		t.Fatalf("postedReview = %d, want 1", f.postedReview)
	}
	if got := lastAudit(t).Verb; got != "review:correctness:request-changes" {
		t.Fatalf("audit verb = %q, want review:correctness:request-changes", got)
	}
	// Pin the EVENT mapping too, not just the verb string: a request-changes that
	// submitted as APPROVE would post an approving review under a blocking verdict.
	if n := len(f.reviews); n == 0 || f.reviews[n-1].State != "CHANGES_REQUESTED" {
		t.Fatalf("submitted review state = %q, want CHANGES_REQUESTED — --verdict "+
			"request-changes must submit REQUEST_CHANGES", lastState(f))
	}
}

func lastState(f *fakeGH) string {
	if n := len(f.reviews); n > 0 {
		return f.reviews[n-1].State
	}
	return "<none>"
}
