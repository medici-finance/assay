package main

import (
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const okReviewBody = "## Review\n\nValues reconcile; settlement fetches PublishedPrice. No blockers.\n\nVerdict: approve\n"

// okSecurityBody is the security-review body. It posts with the SAME
// --verdict approve as okReviewBody above — that identity is the whole of #220.
const okSecurityBody = "## Security review\n\nNo funds-path or auth regressions in the diff.\n\nSecurity-Review: pass\n"

func reviewArgs(repo, pr, verdict, head, bodyFile string) []string {
	return []string{"review", repo, pr, "--verdict", verdict, "--head", head, "--body-file", bodyFile}
}

func TestReviewApproveSuccess(t *testing.T) {
	f, _ := setupFake(t)
	f.pullHeads = []string{testHead}
	bf := writeBody(t, "rev.md", okReviewBody)

	code := run(reviewArgs("example-org/tracker", "1", "approve", testHead, bf))
	if code != 0 {
		t.Fatalf("review exit = %d, want 0", code)
	}
	if f.postedReview != 1 {
		t.Fatalf("postedReview = %d, want 1", f.postedReview)
	}
	e := lastAudit(t)
	if e.Result != deskkit.ResultOK || e.Verb != "review:correctness:approve" {
		t.Fatalf("audit = %+v", e)
	}
	if e.BodyDigest == "" || e.HeadSHA == nil || *e.HeadSHA != testHead {
		t.Fatalf("audit missing bodyDigest/head: %+v", e)
	}
}

// TestReviewHeadMismatchExit5 — the verdict must not land on unreviewed code.
func TestReviewHeadMismatchExit5(t *testing.T) {
	f, _ := setupFake(t)
	f.pullHeads = []string{testNewHead} // current head differs from the reviewed --head
	bf := writeBody(t, "rev.md", okReviewBody)

	code := run(reviewArgs("example-org/tracker", "1", "approve", testOldHead, bf))
	if code != deskkit.ExitRefused {
		t.Fatalf("head-mismatch exit = %d, want 5", code)
	}
	if f.postedReview != 0 {
		t.Fatal("no review posted on a head mismatch")
	}
}

// TestReviewBadBodyExit5NoNetwork — a secret-carrying body refuses BEFORE any network
// (no token mint, no review post).
func TestReviewBadBodyExit5NoNetwork(t *testing.T) {
	f, _ := setupFake(t)
	bf := writeBody(t, "bad.md", "## Review\n\nVerdict: approve\n\ntoken ghp_"+repeat("A", 36)+"\n")

	code := run(reviewArgs("example-org/tracker", "1", "approve", testHead, bf))
	if code != deskkit.ExitRefused {
		t.Fatalf("bad-body exit = %d, want 5", code)
	}
	if len(f.hits) != 0 {
		t.Fatalf("expected ZERO network hits on a body refusal, got %v", f.hits)
	}
}

// TestReviewMissingStructureExit5 — a review body without the verdict schema refuses.
func TestReviewMissingStructureExit5(t *testing.T) {
	f, _ := setupFake(t)
	bf := writeBody(t, "nostruct.md", "looks fine to me\n")
	code := run(reviewArgs("example-org/tracker", "1", "approve", testHead, bf))
	if code != deskkit.ExitRefused {
		t.Fatalf("no-structure exit = %d, want 5", code)
	}
	if len(f.hits) != 0 {
		t.Fatal("no network on a structural refusal")
	}
}

// TestReviewRepeatNoopZeroNetwork — a repeat of the same verdict at the same head is a
// no-op with ZERO HTTP calls (assert NO HTTP call on the repeat).
func TestReviewRepeatNoopZeroNetwork(t *testing.T) {
	f, _ := setupFake(t)
	f.pullHeads = []string{testHead}
	bf := writeBody(t, "rev.md", okReviewBody)
	args := reviewArgs("example-org/tracker", "1", "approve", testHead, bf)

	if code := run(args); code != 0 {
		t.Fatalf("first review exit = %d, want 0", code)
	}
	hitsAfterFirst := len(f.hits)
	postsAfterFirst := f.postedReview

	if code := run(args); code != 0 {
		t.Fatalf("repeat review exit = %d, want 0 (noop)", code)
	}
	if len(f.hits) != hitsAfterFirst {
		t.Fatalf("repeat made HTTP calls: hits %d -> %d", hitsAfterFirst, len(f.hits))
	}
	if f.postedReview != postsAfterFirst {
		t.Fatal("repeat posted another review")
	}
	if lastAudit(t).Result != deskkit.ResultNoop {
		t.Fatal("repeat should audit result=noop")
	}
}

// TestReviewRateLimitExit4 — #450 (U1): `review` was the first of two verbs
// (the other is `ready`) with NO test exercising its write gate at all. Both call
// runOutward with a (repo, pr) scope computed in postVerdictReview and read straight off
// argv (review.go, `return runOutward(args, opts, repo, pr, ...)`) — mirrors
// TestCommentRateLimitExit4, the one verb that WAS covered before this fix.
//
// Without this test, aiming that scope at the wrong repo/PR (or dropping it to
// "nobody/nowhere") is invisible to the suite: nothing else calls `review` enough times on
// one PR to observe the budget refuse.
func TestReviewRateLimitExit4(t *testing.T) {
	f, _ := setupFake(t)
	f.pullHeads = []string{testHead}
	pr := 1
	for i := 0; i < deskkit.RateLimitPerPRPerHour; i++ {
		if err := deskkit.Log(deskkit.Entry{
			Tool: toolName, Verb: "review:seed", Repo: exampleRepo,
			PR: &pr, Result: deskkit.ResultOK,
		}); err != nil {
			t.Fatalf("seed audit: %v", err)
		}
	}
	bf := writeBody(t, "rev.md", okReviewBody)

	code := run(reviewArgs(exampleRepo, "1", "approve", testHead, bf))
	if code != deskkit.ExitRateLimited {
		t.Fatalf("rate-limit exit = %d, want 4", code)
	}
	if f.postedReview != 0 {
		t.Fatal("no review posted when rate limited")
	}
}

// appReviewAt builds a reviewInfo as the reviewer App would render one on the PR. The
// body matters: the cross-session guard reads the verdict KIND out of it (#220), and
// every review this App posts carries a verdict line.
func appReviewAt(state, commit, body string) reviewInfo {
	var r reviewInfo
	r.User.Login = reviewerBotDisplay()
	r.State = state
	r.CommitID = commit
	r.Body = body
	return r
}

// TestReviewGitHubStateIdempotentNoDuplicate — the cross-session guard (#73):
// a retry whose LOCAL audit log is EMPTY (a fresh reviewer subagent) must still no-op when
// the App already carries an equivalent verdict at the current head, so no duplicate review
// is submitted (a submitted review cannot be retracted).
func TestReviewGitHubStateIdempotentNoDuplicate(t *testing.T) {
	f, _ := setupFake(t)
	f.pullHeads = []string{testHead}
	// The App already reviewed this exact head — but nothing in THIS session's audit log.
	f.reviews = []reviewInfo{appReviewAt("APPROVED", testHead, okReviewBody)}
	bf := writeBody(t, "rev.md", okReviewBody)

	code := run(reviewArgs("example-org/tracker", "1", "approve", testHead, bf))
	if code != 0 {
		t.Fatalf("idempotent retry exit = %d, want 0 (noop)", code)
	}
	if f.postedReview != 0 {
		t.Fatalf("posted %d duplicate review(s); want 0 — App verdict already at head", f.postedReview)
	}
	if lastAudit(t).Result != deskkit.ResultNoop {
		t.Fatalf("GitHub-state duplicate should audit result=noop, got %s", lastAudit(t).Result)
	}
}

// TestReviewGitHubStateVerdictChangeStillPosts — an existing App review at head with a
// DIFFERENT state (a genuine verdict change, request-changes → approve at the same head)
// must NOT be suppressed by the idempotency guard.
func TestReviewGitHubStateVerdictChangeStillPosts(t *testing.T) {
	f, _ := setupFake(t)
	f.pullHeads = []string{testHead}
	f.reviews = []reviewInfo{appReviewAt("CHANGES_REQUESTED", testHead, "## Review\n\nblocking finding.\n\nVerdict: request-changes\n")} // prior, different verdict
	bf := writeBody(t, "rev.md", okReviewBody)

	code := run(reviewArgs("example-org/tracker", "1", "approve", testHead, bf))
	if code != 0 {
		t.Fatalf("verdict-change exit = %d, want 0", code)
	}
	if f.postedReview != 1 {
		t.Fatalf("posted %d reviews; want 1 — a verdict change is not a duplicate", f.postedReview)
	}
}

// TestReviewGitHubStateStaleReviewStillPosts — an equivalent App verdict pinned to an OLD
// head (a stale review from before new commits) must not suppress a fresh verdict at the
// CURRENT head.
func TestReviewGitHubStateStaleReviewStillPosts(t *testing.T) {
	f, _ := setupFake(t)
	f.pullHeads = []string{testHead}
	f.reviews = []reviewInfo{appReviewAt("APPROVED", testOldHead, okReviewBody)} // same verdict, wrong head
	bf := writeBody(t, "rev.md", okReviewBody)

	code := run(reviewArgs("example-org/tracker", "1", "approve", testHead, bf))
	if code != 0 {
		t.Fatalf("stale-head exit = %d, want 0", code)
	}
	if f.postedReview != 1 {
		t.Fatalf("posted %d reviews; want 1 — a verdict at a stale head is not a duplicate", f.postedReview)
	}
}

// --- verdict-KIND discrimination (#220) ---
//
// A risk-classed PR requires TWO verdicts at the same head: a correctness verdict and a
// security verdict. Both are submitted with `--verdict approve` in the
// all-clear case, so before the kind entered the idempotency key the second one to arrive
// was matched as a repeat of the first and dropped as a success-shaped "idempotent
// no-op" — the gate silently degraded from two artifacts to one.

// TestReviewBothVerdictKindsLandAtSameHead is the #220 regression test. Post a security
// verdict then a correctness verdict at ONE head; BOTH must be submitted, neither may
// audit as a noop, and the two audit verbs must differ.
//
// Before the fix this fails on the second post: verb `review:approve` for both,
// postedReview stays 1, and the second audit is result=noop.
func TestReviewBothVerdictKindsLandAtSameHead(t *testing.T) {
	f, _ := setupFake(t)
	f.pullHeads = []string{testHead}
	secFile := writeBody(t, "sec.md", okSecurityBody)
	corFile := writeBody(t, "cor.md", okReviewBody)

	if code := run(reviewArgs("example-org/tracker", "1", "approve", testHead, secFile)); code != 0 {
		t.Fatalf("security verdict exit = %d, want 0", code)
	}
	secAudit := lastAudit(t)
	if secAudit.Result != deskkit.ResultOK {
		t.Fatalf("security verdict audited %s, want ok", secAudit.Result)
	}

	if code := run(reviewArgs("example-org/tracker", "1", "approve", testHead, corFile)); code != 0 {
		t.Fatalf("correctness verdict exit = %d, want 0", code)
	}
	corAudit := lastAudit(t)
	if corAudit.Result != deskkit.ResultOK {
		t.Fatalf("the SECOND required verdict audited %s, want ok — a different verdict "+
			"kind at the same head is not a duplicate (#220)", corAudit.Result)
	}
	if f.postedReview != 2 {
		t.Fatalf("postedReview = %d, want 2 — a risk-classed PR needs BOTH verdicts at head; "+
			"one of them was dropped (#220)", f.postedReview)
	}
	if secAudit.Verb == corAudit.Verb {
		t.Fatalf("both verdicts audited under the same verb %q — the idempotency key does not "+
			"carry the verdict kind (#220)", secAudit.Verb)
	}
	if secAudit.Verb != "review:security:approve" || corAudit.Verb != "review:correctness:approve" {
		t.Fatalf("verbs = %q / %q, want review:security:approve / review:correctness:approve",
			secAudit.Verb, corAudit.Verb)
	}
}

// TestReviewSecurityKindRepeatStillNoops — genuine dedup is PRESERVED: the SAME kind at
// the same head with the same body is still a no-op with zero HTTP calls. (The correctness
// side of this is TestReviewRepeatNoopZeroNetwork above; this is the security side, which
// is the kind the new key component could have detached from the store entirely.)
func TestReviewSecurityKindRepeatStillNoops(t *testing.T) {
	f, _ := setupFake(t)
	f.pullHeads = []string{testHead}
	bf := writeBody(t, "sec.md", okSecurityBody)
	args := reviewArgs("example-org/tracker", "1", "approve", testHead, bf)

	if code := run(args); code != 0 {
		t.Fatalf("first security verdict exit = %d, want 0", code)
	}
	hitsAfterFirst, postsAfterFirst := len(f.hits), f.postedReview

	if code := run(args); code != 0 {
		t.Fatalf("repeat exit = %d, want 0 (noop)", code)
	}
	if len(f.hits) != hitsAfterFirst {
		t.Fatalf("repeat made HTTP calls: hits %d -> %d", hitsAfterFirst, len(f.hits))
	}
	if f.postedReview != postsAfterFirst {
		t.Fatalf("repeat posted another review: %d -> %d", postsAfterFirst, f.postedReview)
	}
	if lastAudit(t).Result != deskkit.ResultNoop {
		t.Fatalf("same-kind repeat audited %s, want noop", lastAudit(t).Result)
	}
}

// TestReviewOtherKindPostsAcrossSessions — the CROSS-session half of #220. The local audit
// log is empty (a fresh reviewer subagent, e.g. the security lane running in its own
// window) and the App already carries the OTHER kind's verdict at this head, in the same
// APPROVED state. The GitHub-state guard (#73) keys on the same tuple as the local one, so
// without the kind it swallows the second verdict on exactly the path the local key no
// longer merges.
//
// Before the fix: postedReview = 0, exit 0, audit=noop.
func TestReviewOtherKindPostsAcrossSessions(t *testing.T) {
	f, _ := setupFake(t)
	f.pullHeads = []string{testHead}
	f.reviews = []reviewInfo{appReviewAt("APPROVED", testHead, okSecurityBody)}
	bf := writeBody(t, "cor.md", okReviewBody)

	code := run(reviewArgs("example-org/tracker", "1", "approve", testHead, bf))
	if code != 0 {
		t.Fatalf("correctness verdict exit = %d, want 0", code)
	}
	if f.postedReview != 1 {
		t.Fatalf("postedReview = %d, want 1 — the App's SECURITY approve at this head is not "+
			"the correctness verdict and must not suppress it (#220)", f.postedReview)
	}
	if lastAudit(t).Result != deskkit.ResultOK {
		t.Fatalf("audit = %s, want ok", lastAudit(t).Result)
	}
}

// TestReviewSameKindNoopAcrossSessions — the cross-session guard still fires for the SAME
// kind, so #73's duplicate-suppression is not traded away for #220's fix.
func TestReviewSameKindNoopAcrossSessions(t *testing.T) {
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

// TestReviewAmbiguousKindRefusesNoNetwork — FAIL CLOSED. A body carrying BOTH verdict
// lines has no determinable kind; the key must NOT fall back to one that merges kinds, so
// the write is refused (exit 5) before any network call rather than posted under a guess.
func TestReviewAmbiguousKindRefusesNoNetwork(t *testing.T) {
	f, _ := setupFake(t)
	bf := writeBody(t, "both.md",
		"## Review\n\nlooks fine and no funds-path regressions.\n\nVerdict: approve\n\nSecurity-Review: pass\n")

	code := run(reviewArgs("example-org/tracker", "1", "approve", testHead, bf))
	if code != deskkit.ExitRefused {
		t.Fatalf("ambiguous-kind exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if len(f.hits) != 0 {
		t.Fatalf("expected ZERO network hits on an ambiguous-kind refusal, got %v", f.hits)
	}
	if e := lastAudit(t); e.Result != deskkit.ResultRefused {
		t.Fatalf("audit = %s, want refused", e.Result)
	}
}

// TestReviewUnknownRepoExit5 — repo outside the fixed set refuses with no network.
func TestReviewUnknownRepoExit5(t *testing.T) {
	f, _ := setupFake(t)
	bf := writeBody(t, "rev.md", okReviewBody)
	code := run(reviewArgs("evil/repo", "1", "approve", testHead, bf))
	if code != deskkit.ExitRefused {
		t.Fatalf("unknown-repo exit = %d, want 5", code)
	}
	if len(f.hits) != 0 {
		t.Fatal("no network on an unknown repo")
	}
}

func repeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}
