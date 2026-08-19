package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const exampleRepo = "example-org/tracker"

func appReview(state, commitID, body string) reviewInfo {
	r := reviewInfo{State: state, CommitID: commitID, Body: body}
	r.User.Login = reviewerBotDisplay()
	return r
}

func greenStatus() combinedStatus {
	cs := combinedStatus{State: "success", TotalCount: 1}
	cs.Statuses = append(cs.Statuses, struct {
		State   string `json:"state"`
		Context string `json:"context"`
	}{State: "success", Context: "ci/build"})
	return cs
}

func checksWith(status, conclusion string) checkRunsResp {
	cr := checkRunsResp{TotalCount: 1}
	cr.CheckRuns = append(cr.CheckRuns, struct {
		Name       string `json:"name"`
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
	}{Name: "test", Status: status, Conclusion: conclusion})
	return cr
}

func readyArgs(repo string) []string { return []string{"ready", repo, "1"} }

// TestReadySuccess — open+draft, App APPROVED at head, green CI, non-risk files → flip.
func TestReadySuccess(t *testing.T) {
	f, _ := setupFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()

	code := run(readyArgs(exampleRepo))
	if code != 0 {
		t.Fatalf("ready exit = %d, want 0", code)
	}
	if f.flips != 1 {
		t.Fatalf("flips = %d, want 1", f.flips)
	}
	if e := lastAudit(t); e.Result != deskkit.ResultOK || e.Verb != "ready" {
		t.Fatalf("audit = %+v", e)
	}
}

// TestReadyRateLimitExit4 — #450 (U1): `ready` was the second of two verbs
// (the other is `review`) with NO test exercising its write gate at all. It calls
// runOutward with a (repo, pr) scope computed in runReady and read straight off argv
// (ready.go, `return runOutward(args, opts, repo, pr, ...)`) — mirrors
// TestCommentRateLimitExit4, the one verb that WAS covered before this fix.
//
// Without this test, aiming that scope at the wrong repo/PR (or dropping it to
// "nobody/nowhere") is invisible to the suite: nothing else calls `ready` enough times on
// one PR to observe the budget refuse, and the fixture's every-precondition-green state
// means a mis-gated call would otherwise sail straight through to a flip.
func TestReadyRateLimitExit4(t *testing.T) {
	f, _ := setupFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()
	pr := 1
	for i := 0; i < deskkit.RateLimitPerPRPerHour; i++ {
		if err := deskkit.Log(deskkit.Entry{
			Tool: toolName, Verb: "ready:seed", Repo: exampleRepo, PR: &pr, Result: deskkit.ResultOK,
		}); err != nil {
			t.Fatalf("seed audit: %v", err)
		}
	}

	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitRateLimited {
		t.Fatalf("rate-limit exit = %d, want 4", code)
	}
	if f.flips != 0 {
		t.Fatal("no flip when rate limited")
	}
}

func TestReadyNotDraftExit5(t *testing.T) {
	f, _ := setupFake(t)
	f.prDraft = false
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()

	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitRefused {
		t.Fatalf("not-draft exit = %d, want 5", code)
	}
	if f.flips != 0 {
		t.Fatal("no flip on a non-draft PR")
	}
}

func TestReadyStaleApprovalExit5(t *testing.T) {
	f, _ := setupFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testOldHead, okReviewBody)} // approved at a stale head
	f.status = greenStatus()

	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitRefused {
		t.Fatalf("stale-approval exit = %d, want 5", code)
	}
	if f.flips != 0 {
		t.Fatal("no flip on a stale approval")
	}
}

func TestReadyNoVerdictExit5(t *testing.T) {
	f, _ := setupFake(t)
	f.reviews = nil // App never posted a verdict
	f.status = greenStatus()

	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitRefused {
		t.Fatalf("no-verdict exit = %d, want 5", code)
	}
	if f.flips != 0 {
		t.Fatal("no flip without an App verdict")
	}
}

func TestReadyChangesRequestedExit5(t *testing.T) {
	f, _ := setupFake(t)
	f.reviews = []reviewInfo{
		appReview("APPROVED", testHead, okReviewBody),
		appReview("CHANGES_REQUESTED", testHead, "## Review\n\nblocker.\n\nVerdict: request-changes\n"),
	}
	f.status = greenStatus()

	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitRefused {
		t.Fatalf("changes-requested exit = %d, want 5", code)
	}
	if f.flips != 0 {
		t.Fatal("no flip when the latest verdict is CHANGES_REQUESTED")
	}
}

func TestReadyRedCIExit5(t *testing.T) {
	f, _ := setupFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.checks = checksWith("completed", "failure")

	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitRefused {
		t.Fatalf("red-CI exit = %d, want 5", code)
	}
	if f.flips != 0 {
		t.Fatal("no flip on red CI")
	}
}

func TestReadyPendingCIExit6(t *testing.T) {
	f, _ := setupFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.checks = checksWith("in_progress", "")

	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("pending-CI exit = %d, want 6", code)
	}
	if f.flips != 0 {
		t.Fatal("no flip on pending CI")
	}
	// #448: pending CI is a LOCAL determination from a rollup that WAS read
	// successfully — no write was ever attempted — so it must audit the non-charging
	// ResultUnwritten, not ResultUnverifiable (which charges the outward-write budget on
	// the theory the call may have reached the remote).
	if got := lastAudit(t).Result; got != deskkit.ResultUnwritten {
		t.Fatalf("pending-CI audit result = %q, want %q", got, deskkit.ResultUnwritten)
	}
}

func TestReadyEmptyCIRequiredRepoExit6(t *testing.T) {
	f, _ := setupFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	// f.status / f.checks left empty → empty rollup on a CI-required repo → exit 6

	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("empty-CI required-repo exit = %d, want 6", code)
	}
	if f.flips != 0 {
		t.Fatal("no flip on an unverifiable empty rollup")
	}
	if got := lastAudit(t).Result; got != deskkit.ResultUnwritten {
		t.Fatalf("empty-CI-required audit result = %q, want %q", got, deskkit.ResultUnwritten)
	}
}

// TestReadyPendingCIDoesNotConsumeWriteBudget is #448's end-to-end proof,
// driven through the real `ready` verb rather than seeded audit lines: repeated pending-CI
// calls — which reach GitHub, read the rollup successfully, and stop BEFORE the flip — must
// never advance the per-PR write budget, while a genuine flip still does.
//
// The seeding leaves the budget at RateLimitPerPRPerHour-1 (one slot free) on purpose: if a
// pending-CI call's audit line charged the budget the way ResultUnverifiable used to, the
// SECOND pending-CI call below would find the budget already at the cap and refuse with
// exit 4 (rate-limited) before it ever reached the CI check — not exit 6. Getting exit 6
// twice in a row is only possible if neither call moved the meter.
func TestReadyPendingCIDoesNotConsumeWriteBudget(t *testing.T) {
	f, _ := setupFake(t)
	pr := 1
	for i := 0; i < deskkit.RateLimitPerPRPerHour-1; i++ {
		if err := deskkit.Log(deskkit.Entry{
			Tool: toolName, Verb: "ready:seed", Repo: exampleRepo, PR: &pr, Result: deskkit.ResultOK,
		}); err != nil {
			t.Fatalf("seed audit: %v", err)
		}
	}
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.checks = checksWith("in_progress", "")

	for i := 0; i < 2; i++ {
		code := run(readyArgs(exampleRepo))
		if code != deskkit.ExitUnverifiable {
			t.Fatalf("pending-CI call %d exit = %d, want 6 (unwritten precondition failures must "+
				"never be shadowed by a budget refusal they themselves caused)", i+1, code)
		}
		if got := lastAudit(t).Result; got != deskkit.ResultUnwritten {
			t.Fatalf("pending-CI call %d audit result = %q, want %q", i+1, got, deskkit.ResultUnwritten)
		}
	}
	if f.flips != 0 {
		t.Fatal("no flip across any pending-CI call")
	}

	// The budget is still exactly RateLimitPerPRPerHour-1 charged — the two unwritten
	// calls above added nothing. One genuine write now reaches the cap...
	f.checks = checksWith("completed", "success")
	if code := run(readyArgs(exampleRepo)); code != 0 {
		t.Fatalf("green-CI flip exit = %d, want 0", code)
	}
	if f.flips != 1 {
		t.Fatalf("flips = %d, want 1", f.flips)
	}
	if got := lastAudit(t).Result; got != deskkit.ResultOK {
		t.Fatalf("flip audit result = %q, want %q", got, deskkit.ResultOK)
	}

	// ...and NOW the budget is genuinely spent (RateLimitPerPRPerHour real/charged writes):
	// a further attempt is rate-limited, proving actual writes still bind the cap even
	// after the unwritten lines were absorbed for free.
	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitRateLimited {
		t.Fatalf("post-cap exit = %d, want %d (exit 4) — real writes must still hit the budget",
			code, deskkit.ExitRateLimited)
	}
}

// TestReadyEmptyCINoPRCIRepoGreen pins the OTHER side of ciRequired: a census repo that
// genuinely runs no PR CI still flips on an empty rollup.
//
// The census repo used here is one whose row declares no PR CI (no-ci), so
// render is `on: push[main]` only and an empty rollup there really is everything
// there will ever be.
func TestReadyEmptyCINoPRCIRepoGreen(t *testing.T) {
	f, _ := setupFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}

	const noPRCIRepo = "example-org/examples"
	if ciRequired(noPRCIRepo) {
		t.Fatalf("%s is CI-required — this test needs a census repo with no PR CI", noPRCIRepo)
	}
	code := run(readyArgs(noPRCIRepo))
	if code != 0 {
		t.Fatalf("%s empty-CI exit = %d, want 0 (green)", noPRCIRepo, code)
	}
	if f.flips != 1 {
		t.Fatalf("flips = %d, want 1", f.flips)
	}
}

func TestReadyUnknownRepoExit5(t *testing.T) {
	f, _ := setupFake(t)
	code := run(readyArgs("evil/repo"))
	if code != deskkit.ExitRefused {
		t.Fatalf("unknown-repo exit = %d, want 5", code)
	}
	if len(f.hits) != 0 {
		t.Fatal("no network on an unknown repo")
	}
}

func TestReadyAPI500Exit6(t *testing.T) {
	f, _ := setupFake(t)
	f.intercept = func(method, path string) (int, bool) {
		if method == "GET" && rePull.MatchString(path) {
			return 500, true
		}
		return 0, false
	}
	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("API-500 exit = %d, want 6", code)
	}
	if f.flips != 0 {
		t.Fatal("no flip on an API error")
	}
}

// TestReadyRiskNoSecurityPassExit5 is the #216 gate: a risk-classed PR, App-APPROVED at
// head, green CI, but NO Security-Review: pass line → exit 5 and NO flip.
func TestReadyRiskNoSecurityPassExit5(t *testing.T) {
	f, _ := setupFake(t)
	f.files = []string{"secrets/prod/token.yaml"} // risk path
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()

	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitRefused {
		t.Fatalf("risk-no-security exit = %d, want 5", code)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d, want 0 — a risk PR must not flip without Security-Review: pass", f.flips)
	}
}

// TestReadyRiskWithSecurityPassFlips — the same risk PR with a second App review at head
// carrying Security-Review: pass flips ready.
func TestReadyRiskWithSecurityPassFlips(t *testing.T) {
	f, _ := setupFake(t)
	f.files = []string{"secrets/service/jwt.key"}
	f.reviews = []reviewInfo{
		appReview("APPROVED", testHead, okReviewBody),
		appReview("APPROVED", testHead, "## Security review\n\nNo auth regressions.\n\nSecurity-Review: pass\n"),
	}
	f.status = greenStatus()

	code := run(readyArgs(exampleRepo))
	if code != 0 {
		t.Fatalf("risk-with-security exit = %d, want 0", code)
	}
	if f.flips != 1 {
		t.Fatalf("flips = %d, want 1", f.flips)
	}
}

// TestReadyTOCTOUHeadMovedExit5 — the head moves between the checks and the pre-flip
// re-read → refuse, no flip.
func TestReadyTOCTOUHeadMovedExit5(t *testing.T) {
	f, _ := setupFake(t)
	f.pullHeads = []string{"head1", "head2"} // first getPR: head1; TOCTOU re-read: head2
	f.reviews = []reviewInfo{appReview("APPROVED", "head1", okReviewBody)}
	f.status = greenStatus()

	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitRefused {
		t.Fatalf("TOCTOU exit = %d, want 5", code)
	}
	if f.flips != 0 {
		t.Fatal("no flip when the head moved during checks")
	}
}

// TestReadyFlipPOSTFailureStillCharges is the mirror proof to
// TestReadyPendingCIDoesNotConsumeWriteBudget: the split in #448 must not
// widen into a blanket exemption for "unverifiable". A failure on the ACTUAL mutating call
// — the markPullRequestReadyForReview GraphQL mutation, sent after every precondition
// passed — is the genuinely ambiguous case (the call was SENT; whether it landed cannot be
// confirmed from a 500), and it must keep charging the write budget exactly as before this
// fix.
//
// Forcing every POST /graphql call to 500 also covers the trust-gate query, but the fixture
// author defaults to "shared-agent", a configured trusted identity — prTrustGate returns nil
// without ever calling GraphQL for it (deskkit.TrustedAuthorID short-circuits), so the ONLY
// GraphQL call this scenario reaches is the flip mutation itself.
func TestReadyFlipPOSTFailureStillCharges(t *testing.T) {
	f, _ := setupFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()
	f.intercept = func(method, path string) (int, bool) {
		if method == "POST" && path == "/graphql" {
			return http.StatusInternalServerError, true
		}
		return 0, false
	}

	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("flip-POST-failure exit = %d, want 6", code)
	}
	if f.flips != 0 {
		t.Fatal("no flip recorded when the mutation 500s")
	}
	if got := lastAudit(t).Result; got != deskkit.ResultUnverifiable {
		t.Fatalf("flip-POST-failure audit result = %q, want %q (charges the budget — the call WAS sent)",
			got, deskkit.ResultUnverifiable)
	}
}

// TestReadyIdempotentNoop is the POSITIVE control for the #805 reconciliation: a prior
// successful flip at this head AND a live PR that GitHub agrees is no longer a draft
// (prDraft=false) → genuine no-op, no second flip. The recorded flip really landed, so
// re-issuing it would be pointless churn.
func TestReadyIdempotentNoop(t *testing.T) {
	f, _ := setupFake(t)
	f.prDraft = false // GitHub agrees the earlier flip landed — the PR is no longer a draft
	pr := 1
	head := testHead
	if err := deskkit.Log(deskkit.Entry{
		Tool: toolName, Verb: "ready", Repo: exampleRepo, PR: &pr, HeadSHA: &head, Result: deskkit.ResultOK,
	}); err != nil {
		t.Fatalf("seed audit: %v", err)
	}
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()

	code := run(readyArgs(exampleRepo))
	if code != 0 {
		t.Fatalf("idempotent ready exit = %d, want 0 (noop)", code)
	}
	if f.flips != 0 {
		t.Fatal("no second flip on an idempotent repeat")
	}
	if lastAudit(t).Result != deskkit.ResultNoop {
		t.Fatal("repeat should audit result=noop")
	}
}

// --- #37: "Reviewer-App gate is forgeable — approve→flip→merge in 14s" ---
//
// "Un-forgeable because a PR author cannot approve its own PR" (methodology/brief-17) only
// defends the AUTHOR account. GitHub's self-approval block has nothing to say about a
// third-party App re-posting a verdict, and the reviewer App's private key is readable by
// every session running as the same OS user — any of them can mint the App's token and post
// whatever review it likes. LIVE EVIDENCE (2026-07-14, decks#17 and decks#11, 34 seconds
// apart): both flipped to APPROVED at an UNCHANGED head, with no intervening commit, while a
// blocking CHANGES_REQUESTED stood — on #17 that blocker was the repo owner's own direct instruction
// given 20 minutes earlier. "An approval at an unchanged head cannot be a re-verification —
// there is nothing new to verify."
//
// These tests reproduce the exact forge-and-flip sequence against runReady — the tool that
// actually performs the mutating `ready` flip — and prove it is refused.

// TestReadyNoOpApprovalSameHeadExit5 is the exact live-evidence shape: CHANGES_REQUESTED at
// head, then a second review from the same App flips to APPROVED at the SAME head (no push,
// no new commit_id in between). Before this fix, latestAppVerdict took "the chronologically
// last decisive review" unconditionally, so this sequence satisfied gate (b) and flipped —
// this test is what would have caught decks#17 / decks#11 before they were live.
func TestReadyNoOpApprovalSameHeadExit5(t *testing.T) {
	f, _ := setupFake(t)
	f.reviews = []reviewInfo{
		appReview("CHANGES_REQUESTED", testHead, "## Review\n\nblocker.\n\nVerdict: request-changes\n"),
		appReview("APPROVED", testHead, okReviewBody), // no push between this and the line above
	}
	f.status = greenStatus()

	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitRefused {
		t.Fatalf("no-op-approval exit = %d, want 5 (refused)", code)
	}
	if f.flips != 0 {
		t.Fatal("a same-head APPROVED following a CHANGES_REQUESTED must never flip — no push occurred " +
			"between the rejection and the approval, so this cannot be a re-verification (#37)")
	}
	if d := lastAudit(t).Detail; !strings.Contains(d, "#37") || !strings.Contains(d, "no intervening push") {
		t.Fatalf("refusal must name the no-op-approval mechanism (#37), got: %s", d)
	}
}

// TestReadyNoOpApprovalRetrySameHeadExit5 covers a forger retrying at the same commit after
// the first forged APPROVED is refused: the standing block must not be a one-shot check that
// a second APPROVED can slip past.
func TestReadyNoOpApprovalRetrySameHeadExit5(t *testing.T) {
	f, _ := setupFake(t)
	f.reviews = []reviewInfo{
		appReview("CHANGES_REQUESTED", testHead, "## Review\n\nblocker.\n\nVerdict: request-changes\n"),
		appReview("APPROVED", testHead, okReviewBody),         // first forged/no-op approval
		appReview("APPROVED", testHead, "Approving again.\n"), // retried forgery, same head
	}
	f.status = greenStatus()

	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitRefused {
		t.Fatalf("retried no-op-approval exit = %d, want 5 (refused)", code)
	}
	if f.flips != 0 {
		t.Fatal("a retried forged approval at the same unchanged head must still be refused")
	}
}

// TestReadyGenuineReReviewAfterPushFlips is the control: the SAME CHANGES_REQUESTED ->
// APPROVED transition, but with a genuine push (a new commit_id) between them — proving the
// #37 fix targets the no-op-at-unchanged-head shape specifically and does not block an
// honest re-review after the worker addresses findings.
func TestReadyGenuineReReviewAfterPushFlips(t *testing.T) {
	f, _ := setupFake(t)
	f.reviews = []reviewInfo{
		appReview("CHANGES_REQUESTED", testOldHead, "## Review\n\nblocker.\n\nVerdict: request-changes\n"),
		appReview("APPROVED", testHead, okReviewBody), // testHead != testOldHead: a real push happened
	}
	f.status = greenStatus()

	code := run(readyArgs(exampleRepo))
	if code != 0 {
		t.Fatalf("genuine re-review exit = %d, want 0 — a push between CHANGES_REQUESTED and APPROVED "+
			"is a real re-verification and must still flip", code)
	}
	if f.flips != 1 {
		t.Fatalf("flips = %d, want 1", f.flips)
	}
}

// TestReadyReflipsLiveDraft is the #805 regression: the ledger carries a prior `ready` at
// this exact head (the idempotency key would normally short-circuit to a no-op), but GitHub
// still reports the PR as a LIVE DRAFT (prDraft=true) — the recorded flip never actually
// landed. The old code no-op'd on the audit key alone and stranded the PR as a draft
// forever. `ready` must instead reconcile against the live isDraft state, NOT no-op, and
// RE-ISSUE the flip. Every input here is injected into the offline fake, so no network is
// touched.
func TestReadyReflipsLiveDraft(t *testing.T) {
	f, _ := setupFake(t)
	// prDraft defaults to true in setupFake — GitHub still shows the PR as a draft even
	// though the audit below records a completed flip at this head.
	pr := 1
	head := testHead
	if err := deskkit.Log(deskkit.Entry{
		Tool: toolName, Verb: "ready", Repo: exampleRepo, PR: &pr, HeadSHA: &head, Result: deskkit.ResultOK,
	}); err != nil {
		t.Fatalf("seed audit: %v", err)
	}
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()

	code := run(readyArgs(exampleRepo))
	if code != 0 {
		t.Fatalf("audit-done-but-live-draft exit = %d, want 0 (re-issued flip)", code)
	}
	if f.flips != 1 {
		t.Fatalf("flips = %d, want 1 — a stale audit no-op must NOT strand a live draft; "+
			"ready must reconcile against isDraft and re-flip (#805)", f.flips)
	}
	if got := lastAudit(t).Result; got != deskkit.ResultOK {
		t.Fatalf("re-flip audit result = %q, want %q (a real flip, not a noop)", got, deskkit.ResultOK)
	}
}
