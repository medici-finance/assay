package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// ---------------------------------------------------------------------------
// The lane split has to hold for the form the corpus is actually written in.
//
// PR #399's first cut split the lanes on TWO different readings: the correctness
// lane excluded a body only when the STRICT bodycheck.VerdictKind called it
// security, while gate (e) admitted it on the TOLERANT read. Every case below is
// a body that fell through that crack, or one of the adjacent ambiguities the
// same reduction has to answer. Each pins a DIRECTION, and the direction is
// always the same one: an ambiguous artifact may block, never grant.
// ---------------------------------------------------------------------------

const secBody = "## Security review\n\nSecurity-Review: pass\n"
const emphSecBody = "## Security review\n\n**Security-Review: pass**\n"

// TestReadyEmphasisedSecurityPassDoesNotMaskCorrectnessRequestChanges is R2 on PR #399,
// and it is the SAME fail-open #238 is closed on — reached through the emphasised form
// instead of the canonical one.
//
// `**Security-Review: pass**` is unreadable to the strict VerdictKind, so the correctness
// lane kept it as an APPROVED and it overwrote a live blocking correctness verdict at the
// same head; the tolerant gate (e) simultaneously counted it as the security pass. One
// review, both gates, correctness findings unaddressed. Both live fixtures in this cluster
// are written in exactly this form, so the uncovered shape was the common one.
func TestReadyEmphasisedSecurityPassDoesNotMaskCorrectnessRequestChanges(t *testing.T) {
	f, _ := setupFake(t)
	f.files = []string{"secrets/prod/token.yaml"} // risk-classed
	f.reviews = []reviewInfo{
		appReview("CHANGES_REQUESTED", testHead, "## Review\n\nBlocking finding.\n\nVerdict: request-changes\n"),
		appReview("APPROVED", testHead, emphSecBody),
	}
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d — an EMPHASISED security pass must not satisfy the correctness "+
			"gate over a live request-changes at the same head; the lane split has to be made on "+
			"the same reading both lanes use", code, deskkit.ExitRefused)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d, want 0 — the PR flipped with a blocking correctness verdict at head", f.flips)
	}
	if d := lastAudit(t).Detail; !strings.Contains(d, "CHANGES_REQUESTED") {
		t.Errorf("the refusal must name the standing correctness verdict; audit detail: %s", d)
	}
}

// TestReadyEmphasisedSecurityVerdictAloneDoesNotSatisfyCorrectnessGate — the same crack
// from the other side. A PR whose only App review is an emphasised security pass has had
// no correctness review at all.
func TestReadyEmphasisedSecurityVerdictAloneDoesNotSatisfyCorrectnessGate(t *testing.T) {
	f, _ := setupFake(t)
	f.files = []string{"secrets/prod/token.yaml"}
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, emphSecBody)}
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d, want 0", f.flips)
	}
	if d := lastAudit(t).Detail; !strings.Contains(d, "CORRECTNESS") {
		t.Errorf("refusal must name the missing lane; audit detail: %s", d)
	}
}

// TestReadyStarBulletIsNotASecurityPass is R1 on PR #399, end to end.
//
// The first cut deleted `*`, `_` and backticks from the whole line before matching. `*` is
// also Markdown's alternate BULLET, so a two-bullet lane summary inside an ORDINARY
// correctness review — no security review posted, none ever dispatched — satisfied gate
// (e) on a risk-classed PR and flipped it. The parse must not invent an artifact.
func TestReadyStarBulletIsNotASecurityPass(t *testing.T) {
	f, _ := setupFake(t)
	f.files = []string{"secrets/prod/token.yaml"} // risk-classed
	f.reviews = []reviewInfo{
		appReview("APPROVED", testHead,
			"## Review\n\nLane summary:\n\n* Security-Review: pass\n* Verdict: approve\n\nVerdict: approve\n"),
	}
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d — a `*` LIST ITEM is not a security verdict; no security "+
			"review was ever posted on this PR", code, deskkit.ExitRefused)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d, want 0 — a bulleted list granted a security pass", f.flips)
	}
	if d := lastAudit(t).Detail; !strings.Contains(d, "Security-Review: pass") {
		t.Errorf("refusal must be the security gate, not something else; audit detail: %s", d)
	}
}

// TestReadyFencedSecurityPassIsNotAVerdict — NB1 on PR #399. A skill, a README or a review
// that SHOWS the marker in a fenced example is documenting the format, not issuing a
// verdict. The grant path skips fenced lines for that reason.
func TestReadyFencedSecurityPassIsNotAVerdict(t *testing.T) {
	f, _ := setupFake(t)
	f.files = []string{"secrets/prod/token.yaml"}
	f.reviews = []reviewInfo{
		appReview("APPROVED", testHead,
			"## Review\n\nThe security lane writes:\n\n```\nSecurity-Review: pass\n```\n\nVerdict: approve\n"),
	}
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d — a FENCED example is documentation, not a verdict",
			code, deskkit.ExitRefused)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d, want 0", f.flips)
	}
}

// TestReadyFencedSecurityFailStillBlocks is the asymmetry, end to end, and the reason the
// two fence rules point in opposite directions.
//
// The WRITE gate is not fence-aware, so a body whose only verdict line sits inside a fence
// is postable. If the block path skipped fenced lines the way the grant path does, that
// body's retraction would be unreadable — and on a NON-risk-classed path gate (e0) is the
// only thing a fail has left to block with. Note the repo here is deliberately the one
// with no path triggers.
func TestReadyFencedSecurityFailStillBlocks(t *testing.T) {
	f, _ := setupFake(t)
	f.files = []string{"README.md"} // not risk-classed: only (e0) can block
	f.reviews = []reviewInfo{
		appReview("APPROVED", testHead, okReviewBody),
		appReview("CHANGES_REQUESTED", testHead,
			"Retracting the earlier verdict:\n\n```\nSecurity-Review: fail\n```\n"),
	}
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d — a retraction inside a fence must still be read, or a fence "+
			"becomes a place to hide one from gate (e0)", code, deskkit.ExitRefused)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d, want 0", f.flips)
	}
}

// TestReadyUnsubmittedSecurityPassStatesDoNotSatisfyTheGate — what NB2 on PR #399 actually
// protects, restated as an allow-list (#513).
//
// NB2 installed `r.State != "APPROVED" → not the artifact`, derived from the convention of
// the day ("the convention posted a pass as --approve"). #513 changed the convention: a
// clean pass is now a COMMENT-event review, so COMMENTED is admitted and is covered by
// TestReadySecurityPassAsCommentedReviewFlips. The part of NB2 that was never about the
// convention is here — a review the App never SUBMITTED, or one that was DISMISSED, is not
// a grant whatever its body says. PENDING matters concretely: GitHub serves a PENDING draft
// review on this endpoint TO ITS AUTHOR, and deskpost reads as the author.
func TestReadyUnsubmittedSecurityPassStatesDoNotSatisfyTheGate(t *testing.T) {
	for _, state := range []string{"PENDING", "DISMISSED"} {
		t.Run(state, func(t *testing.T) {
			f, _ := setupFake(t)
			f.files = []string{"secrets/prod/token.yaml"}
			f.reviews = []reviewInfo{
				appReview("APPROVED", testHead, okReviewBody),
				appReview(state, testHead, secBody),
			}
			f.status = greenStatus()

			if code := run(readyArgs(exampleRepo)); code != deskkit.ExitRefused {
				t.Fatalf("exit = %d, want %d — a %s review is not a submitted, standing security "+
					"grant and must not satisfy gate (e)", code, deskkit.ExitRefused, state)
			}
			if f.flips != 0 {
				t.Fatalf("flips = %d, want 0", f.flips)
			}
		})
	}
}

// TestReadyCommentedSecurityFailStillBlocks — the same asymmetry as the fence rule. A pass
// needs the right state; a FAIL is heard whatever state carries it, because every reason
// to doubt a retraction is a reason to keep blocking.
func TestReadyCommentedSecurityFailStillBlocks(t *testing.T) {
	f, _ := setupFake(t)
	f.files = []string{"README.md"} // not risk-classed: only (e0) can block
	f.reviews = []reviewInfo{
		appReview("APPROVED", testHead, okReviewBody),
		appReview("COMMENTED", testHead, "## Security review\n\nSecurity-Review: fail\n"),
	}
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d — a retraction blocks whatever review state carries it",
			code, deskkit.ExitRefused)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d, want 0", f.flips)
	}
}

// TestReadyOneBodyCannotSatisfyBothLanes — NB3 on PR #399. A body carrying BOTH a
// correctness verdict line and a security marker is one review claiming to be both of the
// two artifacts a risk-classed PR requires at one head. deskpost's own write path refuses
// it (VerdictKind), so it can only arrive through the raw `gh pr review` fallback — the
// path #197 documents as the COMMON one under a saturated budget. It grants neither lane.
func TestReadyOneBodyCannotSatisfyBothLanes(t *testing.T) {
	f, _ := setupFake(t)
	f.files = []string{"secrets/prod/token.yaml"}
	f.reviews = []reviewInfo{
		appReview("APPROVED", testHead,
			"## Review\n\nAll clear on both lanes.\n\nVerdict: approve\n\nSecurity-Review: pass\n"),
	}
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d — one review cannot be BOTH required artifacts at the same "+
			"head; that is the whole content of \"from their own artifacts\"", code, deskkit.ExitRefused)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d, want 0", f.flips)
	}
}

// TestReadyBothLanesInOneBodyStillBlocks — the other direction of NB3. The ambiguous body
// may not GRANT, but it must still BLOCK: a `fail` marker in it is a retraction, and it
// counts even on a path the risk classifier does not trigger.
func TestReadyBothLanesInOneBodyStillBlocks(t *testing.T) {
	f, _ := setupFake(t)
	f.files = []string{"README.md"} // not risk-classed: only (e0) can block
	f.reviews = []reviewInfo{
		appReview("APPROVED", testHead, okReviewBody),
		appReview("APPROVED", testHead,
			"## Review\n\nVerdict: approve\n\nSecurity-Review: fail\n"),
	}
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d — an ambiguous body grants nothing but still retracts",
			code, deskkit.ExitRefused)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d, want 0", f.flips)
	}
}

// TestReadyTwoLanesTwoArtifactsStillFlips is the control for every refusal above: the
// tightening must not cost the normal risk-classed flip. Two reviews, two lanes, one head,
// the security one emphasised — the form the live corpus is written in.
func TestReadyTwoLanesTwoArtifactsStillFlips(t *testing.T) {
	f, _ := setupFake(t)
	f.files = []string{"secrets/prod/token.yaml"}
	f.reviews = []reviewInfo{
		appReview("APPROVED", testHead, okReviewBody),
		appReview("APPROVED", testHead, emphSecBody),
	}
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code != 0 {
		t.Fatalf("exit = %d, want 0 — both lanes satisfied at head by their own artifacts", code)
	}
	if f.flips != 1 {
		t.Fatalf("flips = %d, want 1", f.flips)
	}
}

// TestReadyProseCorrectnessReviewStillCountsInItsLane guards the measured majority. Most
// live App bodies carry no parseable verdict line at all and are prose correctness
// reviews; if the lane classifier ever stopped admitting them, nearly every real flip
// would refuse. The direction that must never be guessed is the other one — an unreadable
// body reaching the SECURITY lane — and that is asserted separately.
func TestReadyProseCorrectnessReviewStillCountsInItsLane(t *testing.T) {
	f, _ := setupFake(t)
	f.files = []string{"README.md"}
	f.reviews = []reviewInfo{
		appReview("CHANGES_REQUESTED", testHead, "## Review\n\nThree findings, no verdict line.\n"),
		appReview("APPROVED", testHead, "## Review\n\nAll addressed. No verdict line here either.\n"),
	}
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code != 0 {
		t.Fatalf("exit = %d, want 0 — a prose correctness review is still a correctness verdict", code)
	}
	if f.flips != 1 {
		t.Fatalf("flips = %d, want 1", f.flips)
	}
}
