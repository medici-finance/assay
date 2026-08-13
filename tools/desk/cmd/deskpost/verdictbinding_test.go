package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// ---------------------------------------------------------------------------
// A verdict is only meaningful BOUND TO THE COMMIT IT WAS GIVEN AGAINST, and to
// THE LANE THAT GAVE IT. These are the ways that binding was being lost:
//
//	#232  an emphasised verdict is invisible to the anchored read path
//	#238  an unreadable review at head suppresses a different verdict (write path)
//	#238  a security verdict satisfies the correctness gate    (read path)
//	#239  the same, probed on merged main
//	#197  the rate-limit fallback drops --head pinning entirely
//
// Each test below is the case that used to pass silently.
// ---------------------------------------------------------------------------

// --- #238 direction 3: the two lanes are separate ---------------------------

// TestReadySecurityPassDoesNotMaskCorrectnessRequestChanges is the fail-open. Both
// verdict kinds submit as the SAME GitHub event — `Security-Review: pass` is
// `--verdict approve` → APPROVED — so a reduction filtering on login+state alone let a
// SECURITY pass overwrite a live blocking CORRECTNESS verdict at the same head, and the
// PR flipped with its correctness findings unaddressed.
func TestReadySecurityPassDoesNotMaskCorrectnessRequestChanges(t *testing.T) {
	f, _ := setupFake(t)
	f.files = []string{"secrets/prod/token.yaml"} // risk-classed
	f.reviews = []reviewInfo{
		appReview("CHANGES_REQUESTED", testHead, "## Review\n\nBlocking finding.\n\nVerdict: request-changes\n"),
		appReview("APPROVED", testHead, "## Security review\n\nNo issues.\n\nSecurity-Review: pass\n"),
	}
	f.status = greenStatus()

	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d (ExitRefused) — a SECURITY pass must not satisfy the "+
			"CORRECTNESS gate over a live request-changes at the same head (#238 direction 3)",
			code, deskkit.ExitRefused)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d, want 0 — the PR flipped with a blocking correctness verdict at head", f.flips)
	}
}

// TestReadySecurityVerdictAloneDoesNotSatisfyCorrectnessGate — the same split from the
// other side. A PR carrying ONLY a security verdict has never received a correctness
// review; the gate must say so rather than count the security artifact twice.
func TestReadySecurityVerdictAloneDoesNotSatisfyCorrectnessGate(t *testing.T) {
	f, _ := setupFake(t)
	f.files = []string{"secrets/prod/token.yaml"}
	f.reviews = []reviewInfo{
		appReview("APPROVED", testHead, "## Security review\n\nSecurity-Review: pass\n"),
	}
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d (ExitRefused)", code, deskkit.ExitRefused)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d, want 0", f.flips)
	}
	// The refusal DETAIL is asserted through the audit line: finishAudit prints it to
	// os.Stderr directly, which the harness cannot capture (see headform_test.go).
	if d := lastAudit(t).Detail; !strings.Contains(d, "CORRECTNESS") {
		t.Errorf("refusal must name the MISSING LANE, not just 'no verdict'; audit detail: %s", d)
	}
}

// TestReadyCorrectnessApproveStillFlipsAlongsideSecurity — the split must not break the
// normal risk-classed flip: a correctness approve plus a security pass, both at head.
func TestReadyCorrectnessApproveStillFlipsAlongsideSecurity(t *testing.T) {
	f, _ := setupFake(t)
	f.files = []string{"secrets/prod/token.yaml"}
	f.reviews = []reviewInfo{
		appReview("APPROVED", testHead, okReviewBody),
		appReview("APPROVED", testHead, "## Security review\n\nSecurity-Review: pass\n"),
	}
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code != 0 {
		t.Fatalf("exit = %d, want 0 — both lanes are satisfied at head", code)
	}
	if f.flips != 1 {
		t.Fatalf("flips = %d, want 1", f.flips)
	}
}

// TestReadyUnreadableBodyStaysInTheCorrectnessLane — the deliberate default. ~43% of live
// App review bodies do not parse to a kind and are overwhelmingly prose correctness
// reviews; dropping them from the correctness lane would refuse nearly every real flip.
// The direction that must never be guessed is the other one, pinned by
// TestReadyUnreadableBodyNeverSatisfiesTheSecurityGate below.
func TestReadyUnreadableBodyStaysInTheCorrectnessLane(t *testing.T) {
	f, _ := setupFake(t)
	f.files = []string{"README.md"} // not risk-classed: correctness lane only
	f.reviews = []reviewInfo{
		appReview("APPROVED", testHead, "## Review\n\n**Verdict: approve.** Prose, no anchored line."),
	}
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code != 0 {
		t.Fatalf("exit = %d, want 0 — an unreadable prose correctness approve at head still counts "+
			"in the correctness lane", code)
	}
}

// TestReadyUnreadableBodyNeverSatisfiesTheSecurityGate — the fail-closed half. An
// unreadable body may stand in for a correctness verdict; it may NEVER stand in for a
// security one.
func TestReadyUnreadableBodyNeverSatisfiesTheSecurityGate(t *testing.T) {
	f, _ := setupFake(t)
	f.files = []string{"secrets/prod/token.yaml"} // risk-classed
	f.reviews = []reviewInfo{
		appReview("APPROVED", testHead, "## Review\n\n**Verdict: approve.** Security looks fine to me."),
	}
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d — prose that merely MENTIONS security is not a security verdict",
			code, deskkit.ExitRefused)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d, want 0", f.flips)
	}
}

// TestReadySecurityFailBlocksEvenWhenNotRiskClassed — gate (e0). Before the lane split, a
// security `fail` arrived as CHANGES_REQUESTED and blocked at gate (b) by accident of the
// two lanes sharing one reduction. Splitting them without this check would have converted
// live open-PR heads from blocked to flippable on any path the risk
// classifier does not trigger on.
//
// A retraction is a reviewer's deliberate act. Whether the diff happens to touch a
// risk-classed path has nothing to do with whether it was made.
func TestReadySecurityFailBlocksEvenWhenNotRiskClassed(t *testing.T) {
	f, _ := setupFake(t)
	f.files = []string{"README.md"} // NOT risk-classed — gate (e) will not run
	f.reviews = []reviewInfo{
		appReview("APPROVED", testHead, okReviewBody), // correctness lane: clean
		appReview("CHANGES_REQUESTED", testHead, "## Security review\n\nSecurity-Review: fail\n"),
	}
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d — a standing Security-Review: fail must block the flip "+
			"whether or not the PR is risk-classed", code, deskkit.ExitRefused)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d, want 0 — the PR flipped with a retracted security verdict at head", f.flips)
	}
	if d := lastAudit(t).Detail; !strings.Contains(d, "RETRACTED") {
		t.Errorf("the refusal must name the retraction, not report a generic missing verdict; got %q", d)
	}
}

// TestReadySecurityFailClearedByLaterPassFlips — (e0) must not become a permanent veto: a
// later `pass` at the same head clears the retraction.
func TestReadySecurityFailClearedByLaterPassFlips(t *testing.T) {
	f, _ := setupFake(t)
	f.files = []string{"README.md"}
	f.reviews = []reviewInfo{
		appReview("APPROVED", testHead, okReviewBody),
		appReview("CHANGES_REQUESTED", testHead, "## Security review\n\nSecurity-Review: fail\n"),
		appReview("APPROVED", testHead, "## Security review\n\nFixed.\n\nSecurity-Review: pass\n"),
	}
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code != 0 {
		t.Fatalf("exit = %d, want 0 — the last security verdict at head governs", code)
	}
	if f.flips != 1 {
		t.Fatalf("flips = %d, want 1", f.flips)
	}
}

// --- #232: the emphasised verdict, end to end -------------------------------

// TestReadyEmphasisedSecurityFailBlocksTheFlip is the live case from #1284 (heads
// 676846dd and 76f0a802): a head-pinned `**Security-Review: fail**` RETRACTION that the
// anchored read path could not see. Invisible, the earlier pass stayed the visible state
// and the PR stayed flippable — the #216/#219 hole, reopened at the parse.
func TestReadyEmphasisedSecurityFailBlocksTheFlip(t *testing.T) {
	f, _ := setupFake(t)
	f.files = []string{"secrets/prod/token.yaml"}
	f.reviews = []reviewInfo{
		appReview("APPROVED", testHead, okReviewBody),
		appReview("APPROVED", testHead, "## Security review\n\nSecurity-Review: pass\n"),
		appReview("CHANGES_REQUESTED", testHead,
			"Security review — @ head\n\n**Security-Review: fail**\n\nRetracting: the shared reader leaks.\n"),
	}
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d — an EMPHASISED retraction at head must block the flip (#232)",
			code, deskkit.ExitRefused)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d, want 0 — a PR flipped on a security verdict its reviewer withdrew", f.flips)
	}
}

// TestReadyEmphasisedSecurityPassSatisfiesTheGate — the other half of #232. Symmetry, not
// a live population: an emphasised pass need not appear on any current
// head (a minority of App bodies mention the marker, and the read change moves the
// verdict only for the #1284 RETRACTIONS above). The parse must nonetheless read
// a pass on the same terms it reads a fail — a reader that could see only retractions
// would be a different bug, and the historical corpus these bodies came from is not
// frozen.
func TestReadyEmphasisedSecurityPassSatisfiesTheGate(t *testing.T) {
	f, _ := setupFake(t)
	f.files = []string{"secrets/prod/token.yaml"}
	f.reviews = []reviewInfo{
		appReview("APPROVED", testHead, okReviewBody),
		appReview("APPROVED", testHead, "## Security review\n\n**Security-Review: pass**\n"),
	}
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code != 0 {
		t.Fatalf("exit = %d, want 0 — a genuine verdict written with emphasis must be readable (#232)", code)
	}
	if f.flips != 1 {
		t.Fatalf("flips = %d, want 1", f.flips)
	}
}

// TestReadyQuotedSecurityPassIsStillNotAVerdict — the escape hatch that emphasis
// tolerance must not eat. `> ` quoting is the documented way to reference the other
// lane's line in prose; if stripping made a quoted line match, any review body could
// manufacture a security pass.
func TestReadyQuotedSecurityPassIsStillNotAVerdict(t *testing.T) {
	f, _ := setupFake(t)
	f.files = []string{"secrets/prod/token.yaml"}
	f.reviews = []reviewInfo{
		appReview("APPROVED", testHead,
			"## Review\n\nThe security lane already said:\n\n> Security-Review: pass\n\nVerdict: approve\n"),
	}
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d — a QUOTED verdict line is a citation, not a verdict",
			code, deskkit.ExitRefused)
	}
}

// --- #238 / #239 at the write path, through the real flow -------------------

// TestReviewSecurityFailPostsBehindUnparseableAtHead is PROBE-A from #239 driven through
// runReview, not the guard alone: on merged main this returned exit=0, postedReview=0,
// audit=noop — a retraction swallowed in success-shaped output.
func TestReviewSecurityFailPostsBehindUnparseableAtHead(t *testing.T) {
	f, errBuf := setupFake(t)
	f.pullHeads = []string{testHead}
	f.reviews = []reviewInfo{
		appReviewAt("CHANGES_REQUESTED", testHead, "Red CI, blocking. Prose only, no verdict line."),
	}
	bf := writeBody(t, "secfail.md", "## Security review\n\nRetracting.\n\nSecurity-Review: fail\n")

	code := run(reviewArgs("example-org/tracker", "1", "request-changes", testHead, bf))
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if f.postedReview != 1 {
		t.Fatalf("postedReview = %d, want 1 — the Security-Review: fail was DROPPED behind an "+
			"unreadable review at the same head (#238, #239)", f.postedReview)
	}
	if lastAudit(t).Result != deskkit.ResultOK {
		t.Fatalf("audit = %s, want ok", lastAudit(t).Result)
	}
	// #238 item 2 — the write must not be silent about what it could not read.
	//
	// The needle must not be the bare word "WARNING": every unpinned test binary
	// (i.e. every `go test` run) also emits deskkit.WarnIfUnpinned's unconditional
	// "desk-tools WARNING: running UNPINNED ..." banner to stderr before the code
	// under test runs, so a bare "WARNING" assertion passes vacuously even if
	// review.go's own warning is never printed (#521). Anchor on the
	// message review.go:226 actually writes, plus a fragment of the "why" this
	// scenario produces (the unreadable-review arm of appReviewExistsAt), so the
	// assertion can only pass if THIS warning fired.
	if !strings.Contains(errBuf.String(), "deskpost: WARNING: ") ||
		!strings.Contains(errBuf.String(), "NO READABLE verdict kind") {
		t.Errorf("posting over an unreadable review at head must WARN with the unreadable-kind reason; stderr was: %s", errBuf.String())
	}
}

// TestReviewFreshSessionReplayStillNoopsWithARecordedReason — the #73 protection the fix
// must keep, plus #238 item 2. A fresh reviewer subagent (empty local audit log) replaying
// a verdict at the same head must still no-op, because a submitted GitHub review cannot be
// retracted — AND the audit line must say WHY it was suppressed, so the ledger
// distinguishes "duplicate" from every other reason a write did not happen.
//
// This is also the empirical half of the argument for removing #233's unreadable-body
// suppression: the replay a fresh session actually sends went through deskpost, so it
// parses, so it is caught HERE by the kind arm — not by the unreadable arm.
func TestReviewFreshSessionReplayStillNoopsWithARecordedReason(t *testing.T) {
	f, _ := setupFake(t)
	f.pullHeads = []string{testHead}
	f.reviews = []reviewInfo{appReviewAt("APPROVED", testHead, okReviewBody)}
	bf := writeBody(t, "same.md", okReviewBody)

	code := run(reviewArgs("example-org/tracker", "1", "approve", testHead, bf))
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (noop)", code)
	}
	if f.postedReview != 0 {
		t.Fatalf("postedReview = %d, want 0 — a cross-session replay must still no-op (#73)", f.postedReview)
	}
	a := lastAudit(t)
	if a.Result != deskkit.ResultNoop {
		t.Fatalf("audit = %s, want noop", a.Result)
	}
	if !strings.Contains(a.Detail, "same verdict kind") {
		t.Errorf("audit detail must record WHY the write was suppressed (#238 item 2); got %q", a.Detail)
	}
}

// TestReviewCannotPostAnUnreadableBody is the property the #238/#239 fix RESTS ON: because
// deskpost refuses a body with no verdict line and a body with two, every review it has
// ever posted parses to exactly one kind. An unreadable App review at head therefore came
// from somewhere else, and the unreadable arm was never guarding a deskpost replay.
//
// If this ever stops holding, appReviewExistsAt's reasoning stops holding with it.
func TestReviewCannotPostAnUnreadableBody(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"no verdict line", "## Review\n\nLooks right to me. Shipping."},
		{"both kinds — ambiguous", "## Review\n\nVerdict: approve\n\nSecurity-Review: pass\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, _ := setupFake(t)
			f.pullHeads = []string{testHead}
			bf := writeBody(t, "bad.md", tc.body)

			code := run(reviewArgs("example-org/tracker", "1", "approve", testHead, bf))
			if code != deskkit.ExitRefused {
				t.Fatalf("exit = %d, want %d (ExitRefused)", code, deskkit.ExitRefused)
			}
			if f.postedReview != 0 {
				t.Fatalf("postedReview = %d, want 0 — deskpost must never post a body whose "+
					"verdict kind it cannot read", f.postedReview)
			}
		})
	}
}

// --- #197: the fallback recipe carries the head assertion -------------------

// TestFallbackRecipeCarriesTheHeadAssertion. `gh pr review` has no head-pinning flag, so
// the sanctioned exit-4 fallback attaches a verdict to whatever the head is at post time.
// Observed on #195: a CHANGES_REQUESTED written against 151ebe99 landed on 2bbf529c. The
// tool must hand the caller the assertion, not assume a skill in another repo does.
func TestFallbackRecipeCarriesTheHeadAssertion(t *testing.T) {
	const head = "151ebe994c3d2b1a0f9e8d7c6b5a4938271605f4"
	got := fallbackRecipe("medici-finance", "assay", 195, "request-changes", head, "/tmp/body.md")

	for _, want := range []string{
		head,             // the FULL sha the caller must compare against
		"--jq .head.sha", // the re-read
		"DO NOT post",    // abort, do not degrade
		"#197",           // the record
		"gh pr review",   // names the unsafe command explicitly
		"-R medici-finance/assay",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("fallback recipe is missing %q — a recipe without it hands back the\n"+
				"unsafe command with none of the protection:\n%s", want, got)
		}
	}
}
