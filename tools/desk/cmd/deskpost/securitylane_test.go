package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// ---------------------------------------------------------------------------
// #513 / #438 — the skill prescribed the clean security pass as a
// COMMENT; gate (e) reads reviews only. The prescribed artifact could therefore
// never satisfy the gate it exists for, and a risk-classed PR whose security lane
// had legitimately passed was unflippable through the sanctioned tool.
//
// The direction of the failure was CLOSED (a refused flip, not an exposure), which
// is why it survived: nothing went green that should not have. What it cost was the
// flip itself — such artifacts would be invisible here.
//
// #513 names the bar these tests have to clear: "it needs a test that would go red
// if a comment-shaped pass at head were treated as satisfying — or not satisfying —
// gate (e), since the current behaviour is what an unasserted branch produces." So
// BOTH shapes are pinned below, in both directions:
//
//   - a COMMENT-EVENT REVIEW carrying the pass SATISFIES gate (e)   (the new shape)
//   - an ISSUE COMMENT carrying the pass does NOT, and is not even READ (unchanged)
//   - a retracted or wrong-head pass in either shape satisfies nothing
// ---------------------------------------------------------------------------

// secReviewArgs builds a `deskpost security-review` invocation.
func secReviewArgs(repo, pr, verdict, head, bodyFile string) []string {
	return []string{"security-review", repo, pr, "--verdict", verdict, "--head", head, "--body-file", bodyFile}
}

const secFailBody = "## Security review\n\nShared reader on a per-user template.\n\nSecurity-Review: fail\n"

// riskyFiles is a diff that trips deskkit.RiskPathTriggered, so gate (e) actually runs.
// Without it the security gate is skipped and every assertion below would pass vacuously.
func riskyFiles() []string { return []string{"secrets/prod/token.yaml"} }

// ---------------------------------------------------------------------------
// The fix, and the production symptom it removes
// ---------------------------------------------------------------------------

// TestReadySecurityPassAsCommentedReviewFlips is THE regression test for #513.
//
// Fixture: a risk-classed diff, the correctness APPROVED at head, and the security pass
// carried on a COMMENT-event review (state COMMENTED) at the same head — i.e. exactly what
// `deskpost security-review --verdict pass` posts. Everything gate (e) asks for is present.
//
// REVERT PROOF (run before landing): restore `if r.State != "APPROVED" { return secNone }`
// in securityGrantOf and this test fails with exit 5 and flips=0 — the verbatim production
// symptom, "no App review at head carries a 'Security-Review: pass' line — dispatch
// /security-review before the flip", raised against a PR whose /security-review had run and
// passed.
func TestReadySecurityPassAsCommentedReviewFlips(t *testing.T) {
	f, _ := setupFake(t)
	f.files = riskyFiles()
	f.reviews = []reviewInfo{
		appReview("APPROVED", testHead, okReviewBody), // correctness lane
		appReview("COMMENTED", testHead, secBody),     // security lane, COMMENT event
	}
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code != 0 {
		t.Fatalf("exit = %d, want 0 — a COMMENT-event security review at head IS the artifact "+
			"gate (e) asks for (#513)", code)
	}
	if f.flips != 1 {
		t.Fatalf("flips = %d, want 1", f.flips)
	}
}

// TestReadySecurityPassAsCommentedReviewCannotSatisfyTheCORRECTNESSLane is the other half
// of the same change, and the reason the COMMENT event was chosen over #513's direction 3
// (`--approve` for a clean pass).
//
// The correctness lane must stay unsatisfied by a security artifact. With the pass carried
// on a COMMENTED review that holds STRUCTURALLY: latestAppVerdict skips every state that is
// not APPROVED / CHANGES_REQUESTED before it ever reads a body, so the classifyLane parse is
// a second line of defence rather than the only one. Under `--approve` there is no such
// backstop — an APPROVED body whose marker does not parse (the measured-majority case) falls
// into the correctness lane and masks a live blocker, which is verbatim #238.
//
// Here the correctness lane has spoken and is BLOCKING. The flip must be refused, and the
// security COMMENTED review must not rescue it.
func TestReadySecurityPassAsCommentedReviewCannotSatisfyTheCORRECTNESSLane(t *testing.T) {
	f, errBuf := setupFake(t)
	f.files = riskyFiles()
	f.reviews = []reviewInfo{
		appReview("CHANGES_REQUESTED", testHead, "## Review\n\nVerdict: request-changes\n"),
		appReview("COMMENTED", testHead, secBody),
	}
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d — a security pass must never satisfy the correctness gate",
			code, deskkit.ExitRefused)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d, want 0", f.flips)
	}
	if !strings.Contains(errBuf.String(), "CHANGES_REQUESTED") {
		t.Fatalf("refusal must name the live correctness blocker, got: %s", errBuf.String())
	}
}

// TestReadySecurityPassOnCommentedReviewAtWRONGHeadRefuses — head pinning survives the new
// shape. The pass is real and submitted; it was formed against the previous commit.
func TestReadySecurityPassOnCommentedReviewAtWRONGHeadRefuses(t *testing.T) {
	f, _ := setupFake(t)
	f.files = riskyFiles()
	f.reviews = []reviewInfo{
		appReview("APPROVED", testHead, okReviewBody),
		appReview("COMMENTED", testOldHead, secBody), // stale
	}
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d — a pass at a superseded head is not a pass at THIS head",
			code, deskkit.ExitRefused)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d, want 0", f.flips)
	}
}

// TestReadyCommentedSecurityPassThenFailRefuses — the retraction direction, on the new
// shape. #216's order-sensitive reduction has to keep working when the pass arrives as
// COMMENTED and the retraction as CHANGES_REQUESTED, which is what the two `security-review`
// verdicts submit.
func TestReadyCommentedSecurityPassThenFailRefuses(t *testing.T) {
	f, _ := setupFake(t)
	f.files = riskyFiles()
	f.reviews = []reviewInfo{
		appReview("APPROVED", testHead, okReviewBody),
		appReview("COMMENTED", testHead, secBody),
		appReview("CHANGES_REQUESTED", testHead, secFailBody), // retraction at the same head
	}
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d — a retracted pass is not a pass", code, deskkit.ExitRefused)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d, want 0", f.flips)
	}
}

// TestReadyCommentedSecurityPassClearingAnEarlierFailFlips — the mirror. An earlier fail must
// not block forever once the reviewer passes at the same head in the new shape.
func TestReadyCommentedSecurityPassClearingAnEarlierFailFlips(t *testing.T) {
	f, _ := setupFake(t)
	f.files = riskyFiles()
	f.reviews = []reviewInfo{
		appReview("APPROVED", testHead, okReviewBody),
		appReview("CHANGES_REQUESTED", testHead, secFailBody),
		appReview("COMMENTED", testHead, secBody),
	}
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code != 0 {
		t.Fatalf("exit = %d, want 0 — the LAST security verdict at head governs", code)
	}
	if f.flips != 1 {
		t.Fatalf("flips = %d, want 1", f.flips)
	}
}

// TestReadyCommentedLaneBothBodyGrantsNothing — NB3 still holds on the new state. One body
// claiming to be both required artifacts grants neither, whatever review state carries it.
func TestReadyCommentedLaneBothBodyGrantsNothing(t *testing.T) {
	f, _ := setupFake(t)
	f.files = riskyFiles()
	f.reviews = []reviewInfo{
		appReview("APPROVED", testHead, okReviewBody),
		appReview("COMMENTED", testHead, "## Both\n\nVerdict: approve\n\nSecurity-Review: pass\n"),
	}
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d — a laneBoth body may block, never grant",
			code, deskkit.ExitRefused)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d, want 0", f.flips)
	}
}

// ---------------------------------------------------------------------------
// The OTHER direction #513 asks for: the comment shape is still not a grant
// ---------------------------------------------------------------------------

// TestReadyIssueCommentSecurityPassNeitherSatisfiesNorIsRead pins the decision #513 left
// open, in the direction the fix did NOT take.
//
// Direction 1 was "teach gate (e) to read issue comments". It was rejected: an issue comment
// has no server-side commit_id, so the head association would have to come from text in the
// body (hence "`deskpost comment` must grow `--head`"), and a comment is editable and
// deletable after the fact by anyone with write access, which a submitted review is not. A
// gate artifact should not be the mutable one.
//
// So the five live comment-shaped artifacts of #513 remain invisible here BY DESIGN — the
// fix is the new post shape plus the skill change, not a widened reader. This test fails if
// someone later teaches gate (e) to read comments without revisiting that decision, and it
// asserts the stronger form: the ready path makes NO request to the comments endpoint at all.
func TestReadyIssueCommentSecurityPassNeitherSatisfiesNorIsRead(t *testing.T) {
	f, _ := setupFake(t)
	f.files = riskyFiles()
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	// The exact shape of the five live artifacts: an App issue comment carrying the pass.
	f.issueComments = []map[string]any{
		{"id": 5221167895, "body": "## /security-review — clean\n\nSecurity-Review: pass\n",
			"user": map[string]any{"login": reviewerBotDisplay()}},
	}
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d — an issue comment is not a review and must not satisfy gate (e)",
			code, deskkit.ExitRefused)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d, want 0", f.flips)
	}
	if n := f.hitCount("GET", "/issues/1/comments"); n != 0 {
		t.Fatalf("ready read the comments endpoint %d time(s) — gate (e) is review-only by "+
			"design (#513); changing that is a decision, not a patch", n)
	}
}

// ---------------------------------------------------------------------------
// The `security-review` verb itself
// ---------------------------------------------------------------------------

// TestSecurityReviewPassPostsACommentEventReview — the write side of the fix. The artifact
// must be a REVIEW (so gate (e) sees it) submitted with the COMMENT event (so the board's
// review state is not flipped to APPROVED over live correctness findings). Asserted by
// posting and then reading the fake's recorded review back: a fake that mapped the event to
// APPROVED would satisfy a state-blind assertion and prove nothing.
func TestSecurityReviewPassPostsACommentEventReview(t *testing.T) {
	f, _ := setupFake(t)
	bf := writeBody(t, "sec.md", secBody)

	if code := run(secReviewArgs(exampleRepo, "1", "pass", testHead, bf)); code != 0 {
		t.Fatalf("security-review exit = %d, want 0", code)
	}
	if f.postedReview != 1 {
		t.Fatalf("postedReview = %d, want 1", f.postedReview)
	}
	got := f.reviews[len(f.reviews)-1]
	if got.State != "COMMENTED" {
		t.Fatalf("submitted review state = %q, want COMMENTED — an APPROVED security pass "+
			"re-couples the lanes at the GitHub level, which is what the comment shape was "+
			"avoiding (#513)", got.State)
	}
	if got.CommitID != testHead {
		t.Fatalf("commit_id = %q, want %q — the verdict must be pinned to the reviewed head",
			got.CommitID, testHead)
	}
	e := lastAudit(t)
	if e.Result != deskkit.ResultOK || e.Verb != "review:security:pass" {
		t.Fatalf("audit = %+v, want ok/review:security:pass — the verb must not collide with "+
			"`review:security:approve` or `review:correctness:*`", e)
	}
}

// TestSecurityReviewPassThenReadyFlips is the end-to-end the two issues describe: run the
// security lane through the sanctioned tool, then flip through the sanctioned tool. Before
// the fix there was NO pair of deskpost commands that reached a flip on a risk-classed PR
// whose security lane had passed.
func TestSecurityReviewPassThenReadyFlips(t *testing.T) {
	f, _ := setupFake(t)
	f.files = riskyFiles()
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()
	bf := writeBody(t, "sec.md", secBody)

	if code := run(secReviewArgs(exampleRepo, "1", "pass", testHead, bf)); code != 0 {
		t.Fatalf("security-review exit = %d, want 0", code)
	}
	if code := run(readyArgs(exampleRepo)); code != 0 {
		t.Fatalf("ready exit = %d, want 0 — the artifact deskpost just posted must be one "+
			"deskpost's own gate accepts", code)
	}
	if f.flips != 1 {
		t.Fatalf("flips = %d, want 1", f.flips)
	}
}

// TestSecurityReviewFailPostsRequestChanges — a blocker stays as loud as GitHub can make it.
func TestSecurityReviewFailPostsRequestChanges(t *testing.T) {
	f, _ := setupFake(t)
	bf := writeBody(t, "secfail.md", secFailBody)

	if code := run(secReviewArgs(exampleRepo, "1", "fail", testHead, bf)); code != 0 {
		t.Fatalf("security-review exit = %d, want 0", code)
	}
	got := f.reviews[len(f.reviews)-1]
	if got.State != "CHANGES_REQUESTED" {
		t.Fatalf("submitted review state = %q, want CHANGES_REQUESTED", got.State)
	}
	if e := lastAudit(t); e.Verb != "review:security:fail" {
		t.Fatalf("audit verb = %q, want review:security:fail", e.Verb)
	}
}

// TestSecurityReviewRefusesACorrectnessBody — the verb declares its kind, so a body from the
// other lane is refused with no network call. Posting it would put a correctness verdict on
// a COMMENT-event review, where it satisfies neither lane and says so to nobody.
//
// The stderr assertion is what makes this test discriminate, and it is not decoration. A
// correctness body reaches TWO guards in review.go: the kind mismatch (:140) and, right
// behind it, the flag-vs-body mismatch (:146) — a correctness body carries no readable
// `Security-Review:` line, so it reads as NEITHER pass nor fail. Asserting only the exit
// code and postedReview cannot tell which one fired, and the second refuses on its own:
// disabling the kind check entirely leaves this test green. Naming the message pins the
// guard, not merely the outcome.
func TestSecurityReviewRefusesACorrectnessBody(t *testing.T) {
	f, errBuf := setupFake(t)
	bf := writeBody(t, "corr.md", okReviewBody)

	if code := run(secReviewArgs(exampleRepo, "1", "pass", testHead, bf)); code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if f.postedReview != 0 {
		t.Fatalf("postedReview = %d, want 0 — a refusal must make no write", f.postedReview)
	}
	if !strings.Contains(errBuf.String(), "posts the SECURITY verdict") {
		t.Fatalf("the KIND guard must be the one that refused — without this the "+
			"flag-vs-body guard behind it refuses instead and the test stays green with "+
			"the kind check removed entirely. stderr: %s", errBuf.String())
	}
}

// TestSecurityReviewKindGuardIsTheSoleDefenceOnAMixedBody is the case where the kind check
// has nothing standing behind it, recommended by the correctness reviewer on PR #516 §5.
//
// TestSecurityReviewRefusesACorrectnessBody above pins the kind guard by its MESSAGE, which
// kills the mutant but leaves the guard's necessity resting on a string. This body makes it
// structural: the second guard AGREES with the flag here, so the kind check is the only
// thing refusing.
//
// It works because the write and read paths parse differently on purpose. `verdictLine`
// (VerdictKind, the WRITE gate) is strictly anchored, so `**Security-Review: pass**` is
// invisible to it and only `Verdict: approve` is seen — kind = correctness, and no
// "carries BOTH" refusal. `classifySecurityBody` (the READ path) is emphasis-tolerant
// (#232), so it reads the very same line as a security PASS. So with `--verdict pass`:
//
//	kind        = correctness  != wantKind "security"  → guard 1 REFUSES
//	classifySec = secPass      == wantSec               → guard 2 would let it through
//
// Delete the kind check and this body POSTS: a review that GitHub records as COMMENTED,
// that gate (e) then reads as a security pass, carrying a body whose own visible verdict
// line says `Verdict: approve`. One artifact silently satisfying the lane it does not claim
// is exactly what the verb split exists to prevent.
func TestSecurityReviewKindGuardIsTheSoleDefenceOnAMixedBody(t *testing.T) {
	f, errBuf := setupFake(t)
	mixed := "## Review\n\nNo blockers on either lane.\n\nVerdict: approve\n\n**Security-Review: pass**\n"
	bf := writeBody(t, "mixed.md", mixed)

	if code := run(secReviewArgs(exampleRepo, "1", "pass", testHead, bf)); code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d — a body the kind guard alone rejects must be refused",
			code, deskkit.ExitRefused)
	}
	if f.postedReview != 0 {
		t.Fatalf("postedReview = %d, want 0 — a refusal must make no write", f.postedReview)
	}
	if !strings.Contains(errBuf.String(), "posts the SECURITY verdict") {
		t.Fatalf("the KIND guard must be the refusing one; the flag-vs-body guard AGREES "+
			"with --verdict pass on this body and cannot refuse it. stderr: %s", errBuf.String())
	}
}

// TestSecurityReviewRefusesFlagBodyMismatch — `--verdict pass` with a `fail` body. Gate (e0)
// would still read the fail and block, so nothing fails open; the refusal is about the
// artifact not misstating its own verdict to the humans reading the thread, and a submitted
// review cannot be retracted.
func TestSecurityReviewRefusesFlagBodyMismatch(t *testing.T) {
	f, errBuf := setupFake(t)
	bf := writeBody(t, "mismatch.md", secFailBody)

	if code := run(secReviewArgs(exampleRepo, "1", "pass", testHead, bf)); code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if f.postedReview != 0 {
		t.Fatalf("postedReview = %d, want 0", f.postedReview)
	}
	if !strings.Contains(errBuf.String(), "Security-Review: fail") {
		t.Fatalf("refusal must name what the body actually says, got: %s", errBuf.String())
	}
}

// TestSecurityReviewRefusesStaleHead — #197's protection is not re-derived for the new verb,
// it is the SAME code path; this pins that the sharing actually happened.
func TestSecurityReviewRefusesStaleHead(t *testing.T) {
	f, _ := setupFake(t)
	bf := writeBody(t, "sec.md", secBody)

	if code := run(secReviewArgs(exampleRepo, "1", "pass", testOldHead, bf)); code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d — a verdict must not land on unreviewed code",
			code, deskkit.ExitRefused)
	}
	if f.postedReview != 0 {
		t.Fatalf("postedReview = %d, want 0", f.postedReview)
	}
}

// TestSecurityReviewIsIdempotentAtHead — the #73 cross-session guard has to key on the new
// state too. A repeat must no-op rather than leave a second, unretractable review.
func TestSecurityReviewIsIdempotentAtHead(t *testing.T) {
	f, _ := setupFake(t)
	bf := writeBody(t, "sec.md", secBody)

	if code := run(secReviewArgs(exampleRepo, "1", "pass", testHead, bf)); code != 0 {
		t.Fatalf("first security-review exit = %d, want 0", code)
	}
	if code := run(secReviewArgs(exampleRepo, "1", "pass", testHead, bf)); code != 0 {
		t.Fatalf("repeat security-review exit = %d, want 0 (idempotent no-op)", code)
	}
	if f.postedReview != 1 {
		t.Fatalf("postedReview = %d, want 1 — the repeat must not post a second review", f.postedReview)
	}
}

// TestSecurityReviewCrossSessionDedup — the #73 guard for the security verb, in the case
// the test above CANNOT reach.
//
// TestSecurityReviewIsIdempotentAtHead calls run() twice in one HOME, so the LOCAL audit
// log short-circuits the repeat before any network read. That leaves the GitHub-state guard
// — `appReviewExistsAt(existing, curHead, shape.state, kind, reviewBodyDigest(body))` — with no coverage at all on
// this verb: a fresh reviewer subagent with an EMPTY audit log (re-dispatched after a masked
// exit code) is exactly the case #73 exists for, and it is the case the security lane had
// no test for. The correctness lane has its equivalent in
// TestReviewGitHubStateIdempotentNoDuplicate; this is the security twin.
//
// What it pins is `reviewShape.state`, the field whose own doc-comment names the regression:
// "a hand-maintained second mapping is how 'COMMENT produces COMMENTED' becomes wrong
// later." Replace shape.state with a hardcoded "APPROVED" and the guard looks for an
// artifact that is not there, so it posts a SECOND, UNRETRACTABLE security review at the
// same head. Both shapes are asserted, because the mapping has two security entries and
// nothing else observes either one through the dup guard.
func TestSecurityReviewCrossSessionDedup(t *testing.T) {
	cases := []struct {
		name    string
		verdict string
		state   string // the state the verb's event produces, per reviewShape
		body    string
	}{
		{"pass is COMMENTED", "pass", "COMMENTED", secBody},
		{"fail is CHANGES_REQUESTED", "fail", "CHANGES_REQUESTED", secFailBody},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f, _ := setupFake(t)
			f.pullHeads = []string{testHead}
			// The App already carries this verdict at this exact head — but nothing in
			// THIS session's audit log, so only the GitHub-state read can catch it.
			f.reviews = []reviewInfo{appReviewAt(c.state, testHead, c.body)}
			bf := writeBody(t, "sec.md", c.body)

			if code := run(secReviewArgs(exampleRepo, "1", c.verdict, testHead, bf)); code != 0 {
				t.Fatalf("cross-session retry exit = %d, want 0 (noop)", code)
			}
			if f.postedReview != 0 {
				t.Fatalf("posted %d duplicate security review(s); want 0 — the App's %s "+
					"verdict is already at head, and a submitted review cannot be retracted",
					f.postedReview, c.verdict)
			}
			if lastAudit(t).Result != deskkit.ResultNoop {
				t.Fatalf("GitHub-state duplicate should audit result=noop, got %s", lastAudit(t).Result)
			}
		})
	}
}

// TestSecurityReviewCrossSessionVerdictChangeStillPosts is the other half of the same
// guard: matching on state is what lets a genuine RETRACTION through. A standing pass at
// head followed by a fail at that same head must POST — the states differ, so the dup guard
// must not swallow it. A guard that matched on login and head alone would silently drop the
// retraction, which is #216's shape.
func TestSecurityReviewCrossSessionVerdictChangeStillPosts(t *testing.T) {
	f, _ := setupFake(t)
	f.pullHeads = []string{testHead}
	f.reviews = []reviewInfo{appReviewAt("COMMENTED", testHead, secBody)} // a standing PASS
	bf := writeBody(t, "secfail.md", secFailBody)

	if code := run(secReviewArgs(exampleRepo, "1", "fail", testHead, bf)); code != 0 {
		t.Fatalf("retraction exit = %d, want 0", code)
	}
	if f.postedReview != 1 {
		t.Fatalf("postedReview = %d, want 1 — a fail over a standing pass at the same head "+
			"is a verdict CHANGE, not a duplicate; suppressing it drops the retraction",
			f.postedReview)
	}
}

// TestSecurityReviewAndCorrectnessBothLandAtOneHead — #220's kinded idempotency key across
// the verb split. A risk-classed PR needs BOTH artifacts at one head; neither may be
// swallowed as a repeat of the other.
func TestSecurityReviewAndCorrectnessBothLandAtOneHead(t *testing.T) {
	f, _ := setupFake(t)
	corr := writeBody(t, "corr.md", okReviewBody)
	sec := writeBody(t, "sec.md", secBody)

	if code := run(reviewArgs(exampleRepo, "1", "approve", testHead, corr)); code != 0 {
		t.Fatalf("correctness review exit = %d, want 0", code)
	}
	if code := run(secReviewArgs(exampleRepo, "1", "pass", testHead, sec)); code != 0 {
		t.Fatalf("security review exit = %d, want 0", code)
	}
	if f.postedReview != 2 {
		t.Fatalf("postedReview = %d, want 2 — the two required artifacts must both land", f.postedReview)
	}
}

// TestSecurityReviewBadVerdictFlagIsUsage — the flag set is pass|fail, not approve.
func TestSecurityReviewBadVerdictFlagIsUsage(t *testing.T) {
	f, _ := setupFake(t)
	bf := writeBody(t, "sec.md", secBody)

	if code := run(secReviewArgs(exampleRepo, "1", "approve", testHead, bf)); code != 2 {
		t.Fatalf("exit = %d, want 2 (usage)", code)
	}
	if f.postedReview != 0 {
		t.Fatalf("postedReview = %d, want 0", f.postedReview)
	}
}

// TestSecurityReviewRateLimitOmitsTheFallbackRecipe — cmdSecurityReview deliberately does
// NOT print fallbackRecipe on exit 4, and a six-line comment says so. Nothing observed it,
// so someone restoring symmetry with cmdReview would reintroduce the recipe silently.
//
// It matters because of what the recipe SAYS: fall back to a raw `gh pr review --<verdict>`.
// For a security pass that is `--approve` — the APPROVED artifact this whole fix exists to
// stop the desk from posting, since it re-couples the lanes and flips the board green while
// correctness findings stand. The recipe would be handing the caller the defect as the
// remedy. The correct move on exit 4 here is --wait.
//
// The correctness verb is asserted in the SAME test as a positive control, and that is
// load-bearing rather than thorough: `!strings.Contains(...)` passes for free if the needle
// is wrong, if stderr is empty, or if the recipe's wording drifts. Pinning that the same
// needle DOES appear on `review` under the identical exhausted budget is what makes the
// negative assertion mean "omitted here" instead of "not looked for correctly".
func TestSecurityReviewRateLimitOmitsTheFallbackRecipe(t *testing.T) {
	const needle = "gh pr review" // the raw command the recipe hands back

	f, errBuf := setupFake(t)
	pr := 1
	for i := 0; i < deskkit.RateLimitPerPRPerHour; i++ {
		if err := deskkit.Log(deskkit.Entry{
			Tool: toolName, Verb: "comment:seed", Repo: exampleRepo,
			PR: &pr, Result: deskkit.ResultOK,
		}); err != nil {
			t.Fatalf("seed audit: %v", err)
		}
	}
	sec := writeBody(t, "sec.md", secBody)
	corr := writeBody(t, "corr.md", okReviewBody)

	// The security verb: rate-limited, and NO recipe.
	if code := run(secReviewArgs(exampleRepo, "1", "pass", testHead, sec)); code != deskkit.ExitRateLimited {
		t.Fatalf("security-review exit = %d, want %d", code, deskkit.ExitRateLimited)
	}
	if f.postedReview != 0 {
		t.Fatalf("postedReview = %d, want 0 — nothing posts under a saturated budget", f.postedReview)
	}
	secErr := errBuf.String()
	if strings.Contains(secErr, needle) {
		t.Fatalf("security-review printed the exit-4 fallback recipe. It tells the caller to "+
			"re-post a pass via `gh pr review --approve`, which is the APPROVED artifact "+
			"#513 exists to avoid. stderr:\n%s", secErr)
	}

	// Positive control: the SAME needle on the correctness verb, same budget. Without this
	// the assertion above would also pass if the needle simply never matched anything.
	errBuf.Reset()
	if code := run(reviewArgs(exampleRepo, "1", "approve", testHead, corr)); code != deskkit.ExitRateLimited {
		t.Fatalf("review exit = %d, want %d", code, deskkit.ExitRateLimited)
	}
	if !strings.Contains(errBuf.String(), needle) {
		t.Fatalf("control failed: `review` did not print the fallback recipe either, so the "+
			"negative assertion above proves nothing — the needle %q no longer matches "+
			"fallbackRecipe. stderr:\n%s", needle, errBuf.String())
	}
}
