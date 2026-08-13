package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// ---------------------------------------------------------------------------
// #518 — two reviewers, two lanes, ONE head, same verdict shape.
//
// The desk dispatches parallel review lanes against a single head (one
// test-integrity, one correctness+security on #516). Both concluded
// `request-changes`. The first posted; the second got "already posted
// review:correctness:request-changes at <head> (idempotent no-op)" at exit 0 —
// its DISTINCT findings silently swallowed, success-shaped output, no artifact.
//
// Two mechanisms produced that, and both are pinned here:
//
//  1. the LOCAL guard: the reviewers share one HOME, so one audit ledger, and
//     the key (repo, pr, head, review:<kind>:<flag>) matched lane B's different
//     body against lane A's entry;
//  2. the GITHUB-STATE guard: login + commit_id + state + kind matched too —
//     same App, same head, both CHANGES_REQUESTED correctness.
//
// The fix adds the BODY as the missing discriminator in both guards: a true
// retry re-submits the same body and still no-ops; a distinct body posts.
// ---------------------------------------------------------------------------

// laneABody / laneBBody are the two lanes' verdicts from the #518 incident shape:
// same kind (correctness), same flag (request-changes), entirely different findings.
const laneABody = "## Review\n\nTest-integrity lane: the new guard has no failing-direction test.\n\nVerdict: request-changes\n"
const laneBBody = "## Review\n\nCorrectness lane: the dedup key drops the second reviewer's artifact.\n\nVerdict: request-changes\n"

// TestReviewDistinctSecondLaneSameHeadPosts is THE #518 regression, in the incident's own
// shared-HOME shape: lane A posts, then lane B — same HOME, same head, same verdict flag,
// different findings — must ALSO post. Before the fix lane B exercised mechanism 1 (the
// local ledger already held lane A's ok entry under the identical verb) and was swallowed
// at exit 0 with postedReview stuck at 1.
func TestReviewDistinctSecondLaneSameHeadPosts(t *testing.T) {
	f, errBuf := setupFake(t)
	f.pullHeads = []string{testHead}
	bfA := writeBody(t, "laneA.md", laneABody)
	bfB := writeBody(t, "laneB.md", laneBBody)

	if code := run(reviewArgs("example-org/tracker", "1", "request-changes", testHead, bfA)); code != 0 {
		t.Fatalf("lane A exit = %d, want 0", code)
	}
	if f.postedReview != 1 {
		t.Fatalf("lane A postedReview = %d, want 1", f.postedReview)
	}

	if code := run(reviewArgs("example-org/tracker", "1", "request-changes", testHead, bfB)); code != 0 {
		t.Fatalf("lane B exit = %d, want 0", code)
	}
	if f.postedReview != 2 {
		t.Fatalf("postedReview = %d, want 2 — lane B's DISTINCT findings were swallowed as a "+
			"repeat of lane A's same-shaped verdict (#518)", f.postedReview)
	}
	e := lastAudit(t)
	if e.Result != deskkit.ResultOK {
		t.Fatalf("lane B audited %s, want ok — a distinct second review is not an idempotent no-op", e.Result)
	}
	if e.Verb != "review:correctness:request-changes" {
		t.Fatalf("lane B verb = %q, want review:correctness:request-changes (the verb stays "+
			"lane-stable; the body digest is the extra discriminator)", e.Verb)
	}
	// #518 direction 2: posting next to a same-shaped verdict is never SILENT — the
	// warning names what this landed beside, so a retry-with-drifted-body is noticeable.
	if !strings.Contains(errBuf.String(), "DIFFERENT body") || !strings.Contains(errBuf.String(), "#518") {
		t.Fatalf("posting a distinct same-shaped review must warn about the existing one, got: %s", errBuf.String())
	}
}

// TestReviewDistinctBodyPostsAcrossSessions isolates mechanism 2: the local ledger is
// EMPTY (a fresh reviewer subagent), and the App already carries a same-kind, same-state
// review AT THIS HEAD with different findings — the GitHub-state guard alone decides.
// Before the fix appReviewExistsAt matched on login+commit_id+state+kind and swallowed
// the second lane; now the differing body digest lets it through, and the warning names
// the existing review's id so the caller can tell whose verdict it landed beside.
func TestReviewDistinctBodyPostsAcrossSessions(t *testing.T) {
	f, errBuf := setupFake(t)
	f.pullHeads = []string{testHead}
	prior := appReviewAt("CHANGES_REQUESTED", testHead, laneABody)
	prior.ID = 4000000001
	f.reviews = []reviewInfo{prior}
	bf := writeBody(t, "laneB.md", laneBBody)

	if code := run(reviewArgs("example-org/tracker", "1", "request-changes", testHead, bf)); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if f.postedReview != 1 {
		t.Fatalf("postedReview = %d, want 1 — a distinct same-shaped review at head must not be "+
			"suppressed by the cross-session guard (#518)", f.postedReview)
	}
	if lastAudit(t).Result != deskkit.ResultOK {
		t.Fatalf("audit = %s, want ok", lastAudit(t).Result)
	}
	if !strings.Contains(errBuf.String(), "4000000001") {
		t.Fatalf("the warning must name the existing review's id (direction 2), got: %s", errBuf.String())
	}
}

// TestReviewIdenticalRepostStillNoopsLocally — the idempotency HALF of the contract: the
// same reviewer re-running the exact same verdict body is still a zero-HTTP no-op, and
// the message now says WHY it is safe ("this exact body"). The digest discriminator must
// not have traded away #73/#220's true-duplicate protection.
func TestReviewIdenticalRepostStillNoopsLocally(t *testing.T) {
	f, _ := setupFake(t)
	f.pullHeads = []string{testHead}
	bf := writeBody(t, "laneA.md", laneABody)
	args := reviewArgs("example-org/tracker", "1", "request-changes", testHead, bf)

	if code := run(args); code != 0 {
		t.Fatalf("first exit = %d, want 0", code)
	}
	hitsAfterFirst, postsAfterFirst := len(f.hits), f.postedReview

	if code := run(args); code != 0 {
		t.Fatalf("identical re-post exit = %d, want 0 (noop)", code)
	}
	if len(f.hits) != hitsAfterFirst {
		t.Fatalf("identical re-post made HTTP calls: hits %d -> %d", hitsAfterFirst, len(f.hits))
	}
	if f.postedReview != postsAfterFirst {
		t.Fatalf("identical re-post posted another review: %d -> %d", postsAfterFirst, f.postedReview)
	}
	e := lastAudit(t)
	if e.Result != deskkit.ResultNoop {
		t.Fatalf("identical re-post audited %s, want noop", e.Result)
	}
	if !strings.Contains(e.Detail, "with this exact body") {
		t.Fatalf("the noop must claim only what the digest match proves — an IDENTICAL body — got: %s", e.Detail)
	}
}

// TestReviewIdenticalRepostStillNoopsAcrossSessions — the same protection on the
// GitHub-state side: a fresh subagent (empty ledger) re-posting a body byte-identical to
// the App's review at head still no-ops, and the noop detail names the suppressing
// review's id (direction 2) so "my retry" is distinguishable from "someone else's
// verdict" without an API call.
func TestReviewIdenticalRepostStillNoopsAcrossSessions(t *testing.T) {
	f, _ := setupFake(t)
	f.pullHeads = []string{testHead}
	prior := appReviewAt("CHANGES_REQUESTED", testHead, laneABody)
	prior.ID = 4000000001
	f.reviews = []reviewInfo{prior}
	bf := writeBody(t, "laneA.md", laneABody)

	if code := run(reviewArgs("example-org/tracker", "1", "request-changes", testHead, bf)); code != 0 {
		t.Fatalf("exit = %d, want 0 (noop)", code)
	}
	if f.postedReview != 0 {
		t.Fatalf("posted %d duplicate review(s); want 0 — an identical body at head IS a retry", f.postedReview)
	}
	e := lastAudit(t)
	if e.Result != deskkit.ResultNoop {
		t.Fatalf("audit = %s, want noop", e.Result)
	}
	if !strings.Contains(e.Detail, "4000000001") || !strings.Contains(e.Detail, "IDENTICAL body") {
		t.Fatalf("the suppression must name the review id and the identical body, got: %s", e.Detail)
	}
}

// TestReviewPreFixSwallowedNoopDoesNotResuppress — the recovery path for the #518
// incident itself. Before the fix, the swallowed lane's ledger line was a NOOP carrying
// ITS OWN body digest; a local guard that counted noop entries as done would match that
// line on re-run and re-swallow the same body forever. The guard counts ok entries only,
// so the re-run falls through to the GitHub-state read, sees that only the OTHER lane's
// body is at head, and posts.
func TestReviewPreFixSwallowedNoopDoesNotResuppress(t *testing.T) {
	f, _ := setupFake(t)
	f.pullHeads = []string{testHead}
	f.reviews = []reviewInfo{appReviewAt("CHANGES_REQUESTED", testHead, laneABody)} // lane A is on GitHub

	// Lane B's pre-fix ledger line: suppressed (noop) under the digest-less key, digest of
	// the body that never landed.
	pr, head := 1, testHead
	if err := deskkit.Log(deskkit.Entry{
		Tool: "deskpost", Verb: "review:correctness:request-changes",
		ArgsDigest: "test", BodyDigest: deskkit.Sha256Hex([]byte(laneBBody)),
		Repo: "example-org/tracker", PR: &pr, HeadSHA: &head,
		Result: deskkit.ResultNoop,
		Detail: "already posted review:correctness:request-changes at " + short(testHead) + " (idempotent no-op)",
	}); err != nil {
		t.Fatalf("seed pre-fix noop entry: %v", err)
	}

	bf := writeBody(t, "laneB.md", laneBBody)
	if code := run(reviewArgs("example-org/tracker", "1", "request-changes", testHead, bf)); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if f.postedReview != 1 {
		t.Fatalf("postedReview = %d, want 1 — a body wrongly swallowed before the fix must land on "+
			"re-run, not be re-suppressed by its own pre-fix noop line", f.postedReview)
	}
	if lastAudit(t).Result != deskkit.ResultOK {
		t.Fatalf("audit = %s, want ok", lastAudit(t).Result)
	}
}

// TestReviewAlreadyPostedInArms pins the local guard's two narrowings at the unit level:
// the digest arm (a different body is not a repeat) and the result arm (only an ok entry
// proves the body reached GitHub; a noop proves only that a write was once suppressed —
// possibly wrongly, under the pre-#518 key).
func TestReviewAlreadyPostedInArms(t *testing.T) {
	const repo = "example-org/tracker"
	const verb = "review:correctness:request-changes"
	pr, head := 1, testHead
	dig := deskkit.Sha256Hex([]byte(laneBBody))
	entry := func(result, digest, entryHead string) deskkit.Entry {
		h := entryHead
		return deskkit.Entry{Tool: "deskpost", Verb: verb, BodyDigest: digest,
			Repo: repo, PR: &pr, HeadSHA: &h, Result: result}
	}

	for _, tc := range []struct {
		name string
		e    deskkit.Entry
		want bool
	}{
		{"ok + same digest = done", entry(deskkit.ResultOK, dig, testHead), true},
		{"ok + DIFFERENT digest is another lane's review, not a repeat", entry(deskkit.ResultOK, deskkit.Sha256Hex([]byte(laneABody)), testHead), false},
		{"noop + same digest must NOT count as done (pre-#518 noops can be wrong)", entry(deskkit.ResultNoop, dig, testHead), false},
		{"refused + same digest never counts", entry(deskkit.ResultRefused, dig, testHead), false},
		{"ok + same digest at another head never counts", entry(deskkit.ResultOK, dig, testOldHead), false},
	} {
		if got := reviewAlreadyPostedIn([]deskkit.Entry{tc.e}, repo, pr, head, verb, dig); got != tc.want {
			t.Errorf("%s: reviewAlreadyPostedIn = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestAppReviewExistsAtDigestArm pins the cross-session guard's body comparison directly:
// an identical body (even across the trailing-newline trim a round trip can introduce) is
// a duplicate whose why names the review id; a different body of the same kind/state/head
// is NOT a duplicate, and the reason says so loudly.
func TestAppReviewExistsAtDigestArm(t *testing.T) {
	const head = testHead

	identical := mkReview(reviewerBotDisplay(), "CHANGES_REQUESTED", head, laneABody)
	identical.ID = 77
	trimmed := mkReview(reviewerBotDisplay(), "CHANGES_REQUESTED", head, strings.TrimRight(laneABody, "\n"))
	trimmed.ID = 78
	other := mkReview(reviewerBotDisplay(), "CHANGES_REQUESTED", head, laneBBody)
	other.ID = 79

	dup, why := appReviewExistsAt([]reviewInfo{identical}, head, "CHANGES_REQUESTED", "correctness", digOf(laneABody))
	if !dup {
		t.Fatal("an IDENTICAL body at head is a true retry and must still be suppressed")
	}
	if !strings.Contains(why, "77") {
		t.Fatalf("the suppression why must name the review id, got: %s", why)
	}

	if dup, _ := appReviewExistsAt([]reviewInfo{trimmed}, head, "CHANGES_REQUESTED", "correctness", digOf(laneABody)); !dup {
		t.Fatal("a body differing only in trailing newlines is the same body — normalization must not defeat the retry match")
	}

	dup, why = appReviewExistsAt([]reviewInfo{other}, head, "CHANGES_REQUESTED", "correctness", digOf(laneABody))
	if dup {
		t.Fatal("a DIFFERENT body of the same kind/state at head is a distinct review and must not be suppressed (#518)")
	}
	if !strings.Contains(why, "#518") || !strings.Contains(why, "79") {
		t.Fatalf("the not-a-duplicate reason must cite #518 and the existing review id, got: %s", why)
	}
}
