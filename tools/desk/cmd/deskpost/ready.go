package main

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/medici-finance/assay/tools/desk/cmd/deskpost/internal/bodycheck"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// runReady flips a converged draft PR to ready-for-HUMAN-review. It re-verifies EVERY
// precondition in-tool immediately before acting — it never trusts the
// caller's claim of state. It has NO other power: no un-ready, no merge, no close.
// The flip is not an approval and not a merge; the merge stays the owner's.
//
// Preconditions (all re-read here, in order):
//
//	(d) repo ∈ the fixed desk set
//	(a) PR is OPEN and draft
//	(t) trust gate: the PR author is a trusted desk identity (login+numeric id), or
//	    the PR carries a CURRENT blessing (deskkit/trust.go)
//	(b) the reviewer App's latest verdict is APPROVED, submitted at the CURRENT head
//	(c) the CI rollup at the head is green (per-repo policy for an empty rollup)
//	(e) if the PR is risk-classed — repo visibility (a PUBLIC repo always is, the public-repo risk rule)
//	    or the repo's path triggers — an App review at the CURRENT head carries the
//	    literal `Security-Review: pass` line (#216)
//	TOCTOU: re-read the head immediately before the flip; if it moved, refuse
func runReady(owner, name string, pr int, args []string, opts postOpts) int {
	repo := owner + "/" + name

	return runOutward(args, opts, repo, pr, func(entries []deskkit.Entry, opts postOpts) writeResult {
		// (d) repo in set — cheap refusal, no network.
		if !deskkit.IsAllowedRepo(repo) {
			return refused("ready", repo, pr, "", "repo "+repo+" is not in the fixed desk repo set")
		}

		client, err := newGHClient(owner, name)
		if err != nil {
			return fromReadErr("ready", repo, pr, "", err)
		}

		info, err := client.getPR(pr)
		if err != nil {
			// A number that names an ISSUE is a refusal naming `comment`, not the generic
			// unverifiable (#296) — see requirePRErr.
			return fromReadErr("ready", repo, pr, "", requirePRErr(client, repo, pr, err))
		}
		head := info.Head.SHA

		// Idempotency is reconciled against LIVE GitHub state, not the ledger alone (#805).
		// An audit entry records that a flip was ATTEMPTED at this head — it is NOT proof the
		// GraphQL mutation landed. If the ledger says `ready` but GitHub still reports the PR
		// as a draft (the flip was recorded but never took — e.g. an earlier attempt whose
		// mutation silently failed), reporting a completed no-op strands the PR as a draft
		// forever while the review desk and drain-before-pull believe it flipped. So a prior
		// flip at this exact head is a no-op ONLY when the just-read live state (info.Draft,
		// from getPR above) agrees the PR is no longer a draft; when GitHub still shows draft,
		// fall through and RE-ISSUE the flip through every precondition below.
		if deskkit.AlreadyDoneIn(entries, repo, pr, head, "ready") && !info.Draft {
			return noop("ready", repo, pr, head, "already flipped ready at "+short(head)+" (idempotent no-op)")
		}

		// Model-capability floor. A ready-flip is an authority-bearing write, the SAME
		// class the `deskflip` verb gates — and this is the App-identity flip verb, the OTHER
		// live path that performs the identical markPullRequestReadyForReview mutation. Both
		// flip verbs MUST clear the same floor, or a below-tier session simply chooses the
		// ungated one: a floor on one of two equivalent flip verbs is only as strong as the
		// convention it replaces. The tier is read from the target PR's DISPATCHER-attested
		// stamp (applier-aware, so a self-applied stamp is worthless) and it fails CLOSED —
		// a below-tier or present-but-unreadable attestation refuses; an UNATTESTED PR
		// (human-driven or pre-attestation) proceeds with a NOTICE; the override is loud; an
		// unreadable timeline is could-not-check, never a cleared floor.
		tl, ferr := client.stampTimeline(pr)
		if ferr != nil {
			return fromReadErr("ready", repo, pr, head, ferr)
		}
		fd := deskkit.ModelCapabilityFloor(tl, deskkit.IsDispatcherLogin, deskkit.ModelFloorOverrideEngaged())
		switch fd.Outcome {
		case deskkit.FloorRefuse:
			return refused("ready", repo, pr, head, fd.Message)
		case deskkit.FloorOverrideAllow, deskkit.FloorNoticeAllow:
			fmt.Fprintln(stderr, "deskpost: "+fd.Message)
		case deskkit.FloorAllow:
			// Attested at/above the floor: proceed silently.
		}

		// (a) open + draft.
		if info.State != "open" || !info.Draft {
			return refused("ready", repo, pr, head,
				fmt.Sprintf("PR is not open+draft (state=%s, draft=%t) — nothing to flip", info.State, info.Draft))
		}

		// Trust gate (deskkit/trust.go): never flip unvetted third-party work
		// (exit 5, audited) — author login+id must be a desk identity, or the PR
		// must carry a CURRENT blessing.
		if terr := prTrustGate(client, pr, info.User.Login, info.User.ID); terr != nil {
			return fromReadErr("ready", repo, pr, head, terr)
		}

		// Public-repo gate: refuse to write to a public repo
		// without a qualifying +1 from an authorized human.
		if gerr := deskkit.PublicRepoGate(client, owner, name, pr); gerr != nil {
			return fromErr("ready", repo, pr, head, gerr)
		}

		// (b) latest App verdict APPROVED at the current head.
		reviews, err := client.listReviews(pr)
		if err != nil {
			return fromReadErr("ready", repo, pr, head, err)
		}
		state, vhead, found, noOpApproval := latestAppVerdict(reviews)
		if !found {
			return refused("ready", repo, pr, head,
				"no APPROVED/CHANGES_REQUESTED CORRECTNESS verdict from "+reviewerBotDisplay()+
					" — a security verdict alone does not satisfy the correctness gate")
		}
		// #37: an APPROVED that immediately follows a CHANGES_REQUESTED at the SAME commit
		// (no push in between) is a no-op approval, not a re-verification — GitHub's
		// self-approval block only keys on the PR AUTHOR account and has nothing to say
		// about a third-party App re-posting a verdict at an unchanged head. Any session
		// that can mint the reviewer App's token can flip CHANGES_REQUESTED -> APPROVED
		// without a single new line of diff being read. Refuse loudly rather than let this
		// silently satisfy gate (b) as an ordinary APPROVED.
		if noOpApproval {
			return refused("ready", repo, pr, head,
				"latest App correctness verdict at "+short(vhead)+" is an APPROVED that immediately follows "+
					"a CHANGES_REQUESTED at the SAME head, with no intervening push — that cannot be a "+
					"re-verification (#37); refusing to flip until a new commit lands, or a human clears the "+
					"standing rejection directly on GitHub")
		}
		if state != "APPROVED" {
			return refused("ready", repo, pr, head, "latest App correctness verdict is "+state+" — blocked (not APPROVED)")
		}
		if vhead != head {
			return refused("ready", repo, pr, head,
				"App approval was submitted at "+short(vhead)+", not the current head "+short(head)+" — stale")
		}

		// (c) CI green at the head. BOTH rollups must be readable — an error on either
		// aborts, which is why the "fall back to check-runs when combined-status 403s"
		// hypothesis in #252 could not work: they are gated by two different App
		// permissions and #348 measured both closed at once.
		//
		// ciReadErr re-frames whatever came back so the refusal names the precondition
		// that was lost and, on a 403, the permission that lost it (#348/#252). The exit
		// code is unchanged — 6, fail-closed — because "we could not read CI" is exactly
		// what exit 6 means. What changes is that an operator can now tell a permanent
		// capability gap from a network blip WITHOUT four occurrences and an audit-log
		// archaeology session.
		cs, err := client.combinedStatusAt(head)
		if err != nil {
			return fromReadErr("ready", repo, pr, head, ciReadErr("combined commit status", head, err))
		}
		cr, err := client.checkRunsAt(head)
		if err != nil {
			return fromReadErr("ready", repo, pr, head, ciReadErr("check runs", head, err))
		}
		switch evalCI(cs, cr) {
		case ciRed:
			return refused("ready", repo, pr, head, "CI is not green at "+short(head)+" (a check failed/errored)")
		case ciPending:
			// Two causes, same fail-closed answer: a check has not completed, OR the rollup
			// came back shorter than the total GitHub reported (we did not see every
			// check, so we cannot call the head green). Both rollups WERE read successfully
			// (no error above) — this is a local determination from data in hand, not a
			// failed call, and it is made before the mutating flip is ever attempted:
			// unverifiableNoWrite, not unverifiable (#448).
			return unverifiableNoWrite("ready", repo, pr, head,
				"CI at "+short(head)+" is not positively green (a check is still pending, or the "+
					"rollup could not be read in full) — cannot verify green", nil)
		case ciEmpty:
			if ciRequired(repo) {
				return unverifiableNoWrite("ready", repo, pr, head,
					"no CI rollup at "+short(head)+" on CI-required repo "+repo+" — cannot verify green", nil)
			}
			// A repo with no PR CI at all (deskkit.CIRequired == false): an empty rollup is
			// everything there will ever be, so it is green. See deskkit/config.go.
		case ciGreen:
			// proceed
		}

		// (e0) An explicit security FAIL at head blocks the flip, RISK-CLASSED OR NOT.
		//
		// This is unconditional on purpose. A `Security-Review: fail` is a reviewer's
		// deliberate retraction; whether the path triggers automatic risk-classification is
		// irrelevant to whether that retraction was made. It also closes the gap that the
		// lane split (b) would otherwise open: before the split, a security fail arrived as
		// CHANGES_REQUESTED and blocked at (b) by accident of the two lanes sharing one
		// reduction. Open-PR heads have relied on exactly that accident; splitting the
		// lanes without this check would have converted them from blocked to flippable on
		// any non-risk-classed path.
		secv := securityVerdictAtHead(reviews, head)
		if secv == secFail {
			return refused("ready", repo, pr, head,
				"an App review at head "+short(head)+" carries 'Security-Review: fail' — the security "+
					"verdict is RETRACTED; clear it with a later 'Security-Review: pass' at this head")
		}

		// (e) security-review gate for risk-classed PRs (#216).
		prFiles, err := client.listFiles(pr)
		if err != nil {
			return fromReadErr("ready", repo, pr, head, err) // determination unverifiable → exit 6
		}
		// Reconcile the walk against GitHub's OWN count, exactly as evalCI reconciles the
		// CI rollups against total_count. A files walk that stops early otherwise
		// believes it saw the whole diff: 3000 pad files ahead of src/evil.go, and the
		// gate waives itself. Short read → the classification is UNVERIFIABLE, not clean —
		// and, like the CI determinations above, this is decided from files already read
		// successfully, before any mutating call: unverifiableNoWrite (#448).
		if info.ChangedFiles > 0 && len(prFiles) < info.ChangedFiles {
			return unverifiableNoWrite("ready", repo, pr, head,
				fmt.Sprintf("read %d changed files but GitHub reports %d for PR #%d — the diff could not be "+
					"read in full, so the risk-class determination is unverifiable", len(prFiles), info.ChangedFiles, pr), nil)
		}
		files := prFilePaths(prFiles)
		if deskkit.RiskPathTriggered(repo, files) {
			if secv != secPass {
				return refused("ready", repo, pr, head,
					"risk-classed PR ("+deskkit.RiskClassReason(repo, files)+"): no App review at head "+short(head)+
						" carries a 'Security-Review: pass' line — dispatch /security-review before the flip")
			}
		}

		// TOCTOU: re-read the head immediately before the flip. If it moved since the
		// checks above, refuse — the verified state is stale (GitHub's ready mutation has
		// no compare-and-swap).
		head2, err := client.getPRHead(pr)
		if err != nil {
			return fromReadErr("ready", repo, pr, head, err)
		}
		if head2 != head {
			return refused("ready", repo, pr, head,
				"head moved during checks ("+short(head)+" -> "+short(head2)+") — no flip; re-run against the new head")
		}

		if opts.dryRun {
			return dryRun("ready", repo, pr, head,
				"DRY RUN: every ready precondition holds at "+short(head)+" (open+draft, trusted author, "+
					"App APPROVED at head, CI green, security-review gate satisfied, head stable) — "+
					"stopped before markPullRequestReadyForReview")
		}
		if err := client.markReadyForReview(info.NodeID); err != nil {
			return fromErr("ready", repo, pr, head, err)
		}
		return done("ready", repo, pr, head, "", "flipped PR #"+fmt.Sprint(pr)+" ready-for-human at "+short(head))
	})
}

// ciReadErr wraps a failed CI rollup read so the refusal names the precondition that was
// lost, not just the URL that failed (#348, #252).
//
// The 403 case is the one that mattered: `GET .../commits/<sha>/status returned HTTP 403`
// is indistinguishable from a transient API problem, so a recurring run of them read as
// bad luck rather than as "the installation does not hold statuses: read". Each was
// re-run through a raw `gh pr ready`, which flips with none of these preconditions — the
// gate did not fail closed in practice, it fail-closed into an ungated path. Naming the
// cause is what makes the difference actionable at occurrence one.
//
// It is a MESSAGE change only: the error keeps its exit code (6 for an apiError, via
// deskkit.ExitCodeOf's fail-closed default), and no 403 is ever treated as green.
func ciReadErr(what, head string, err error) error {
	// The cause is appended by DeskError.Error(), so the message must NOT repeat it.
	var ae *apiError
	if errors.As(err, &ae) && ae.status == http.StatusForbidden {
		return deskkit.Unverifiable(fmt.Sprintf(
			"cannot verify CI at %s: the %s rollup is not readable by the reviewer App, so `ready` has "+
				"no working CI check at all. Do NOT flip by hand as a workaround — the hand path "+
				"verifies nothing, which is how a fail-CLOSED gate became an ungated flip",
			short(head), what), err)
	}
	return deskkit.Unverifiable(fmt.Sprintf(
		"cannot verify CI at %s: the %s rollup could not be read", short(head), what), err)
}

// latestAppVerdict returns the reviewer App's latest CORRECTNESS verdict (APPROVED or
// CHANGES_REQUESTED, ignoring COMMENTED / PENDING / DISMISSED) and the head it was
// submitted at. Reviews arrive in ascending submitted order, so the last matching one
// wins. found is false when the App has posted no correctness verdict.
//
// noOpApproval (#37 — "Reviewer-App gate is forgeable — approve→flip→merge
// in 14s") is true when the returned state is CHANGES_REQUESTED specifically because the
// reduction SUPPRESSED an APPROVED that immediately followed a CHANGES_REQUESTED at the
// SAME commit_id, with no intervening push. "Un-forgeable because a PR author cannot
// approve its own PR" (methodology/brief-17) only defends against the AUTHOR account —
// GitHub's self-approval block has no opinion on a third-party App re-posting a verdict.
// Any session that can read the reviewer App's private key can mint its token and post an
// APPROVED over a standing CHANGES_REQUESTED at an unchanged head — LIVE EVIDENCE on
// decks#17 / decks#11 (2026-07-14, 34 seconds apart): both flipped APPROVED at an
// unchanged head with a blocking review still standing, one of them over the repo owner's
// own direct instruction. "An approval at an unchanged head cannot be a re-verification — there is
// nothing new to verify." Once a commit carries a CHANGES_REQUESTED, only a NEW commit_id
// (a genuine push) can produce a verdict this gate accepts as APPROVED; a same-commit
// APPROVED — including a retried one — keeps reading as the standing CHANGES_REQUESTED.
// KEEP IN SYNC with deskboard/board.go's reduceReviews, which mirrors this same reduction
// for the advisory board (the board must never report FLIP/MERGE-NOW on a no-op approval
// either — that is exactly what would have hidden the live-evidence forgeries from a human
// reading the board instead of the raw review timeline by hand). NOT AN EXACT MIRROR: this
// reduction is lane-filtered (correctness only, see classifyLane below) and the board's is
// not, so the board can raise SUSPECT-APPROVAL on a cross-lane sequence this gate treats as
// an ordinary block. The divergence flags on an advisory surface rather than granting on a
// mutating one, so it is the safe direction; see the note at reduceReviews for the detail.
//
// IT MUST DISCRIMINATE THE VERDICT KIND (#238, direction 3). Both kinds are
// submitted as the same GitHub event — a `Security-Review: pass` is `--verdict approve` →
// APPROVED, a `Security-Review: fail` is CHANGES_REQUESTED. Filtering on login+state
// alone therefore let the two lanes overwrite each other in one reduction, and in one
// direction that is FAIL-OPEN:
//
//	correctness CHANGES_REQUESTED @head, then security pass @head
//	  → last verdict = APPROVED → gate (b) satisfied
//
// i.e. a security PASS silently satisfied the correctness gate over a live blocking
// correctness verdict, and the PR flipped with its correctness findings unaddressed. The
// mirror case (a security `fail` reading as the correctness verdict) blocks rather than
// flips, but it is equally wrong about which lane spoke.
//
// The two lanes are now separate: this reduction is the CORRECTNESS lane, and
// securityPassAtHead (gate (e)) is the security lane. On a risk-classed PR both must be
// satisfied at the same head, from their own artifacts.
//
// A body whose kind CANNOT be determined counts toward the CORRECTNESS lane, never the
// security lane. That is deliberate, not a shrug: many App review
// bodies do not parse to a kind, and they
// are overwhelmingly prose correctness reviews. Dropping them would refuse nearly every
// real flip; admitting them to the SECURITY lane would let an unreadable body satisfy a
// security gate, which is the direction that must never be guessed. Fail-closed here means
// "an unreadable verdict blocks a flip it should have blocked", not "it grants one".
func latestAppVerdict(reviews []reviewInfo) (state, head string, found, noOpApproval bool) {
	var lastCommit string
	var commitBlocked bool // a CHANGES_REQUESTED stands at lastCommit with no push since
	for _, r := range reviews {
		if !isReviewerBot(r.User.Login) {
			continue
		}
		if r.State != "APPROVED" && r.State != "CHANGES_REQUESTED" {
			continue
		}
		switch classifyLane(r.Body) {
		case laneSecurity:
			continue // the security lane is gate (e), not this one
		case laneBoth:
			// Ambiguous: one body claiming both required artifacts. It may BLOCK but it
			// may never GRANT — see classifyLane.
			if r.State != "CHANGES_REQUESTED" {
				continue
			}
		}

		// #37: a NEW commit_id is a genuine push — it clears any standing block. Reviews
		// arrive in ascending submitted order, so seeing a commit_id different from the
		// one just tracked means the PR head moved since the last decisive review.
		//
		// HOW MUCH THIS ASKS OF commit_id, given what statusgen/mergecheck.go's header
		// establishes about that field. mergecheck reports approval currency as
		// could-not-check because commit_id was observed ONCE to disagree with the head
		// a review body named, and the direction and frequency of that error are
		// unmeasured. That correctly disqualifies it there, because there the question is
		// ABSOLUTE — "is this approval at the current head?" — and an unmeasured error
		// direction cannot be trusted either way.
		//
		// This reduction asks a strictly weaker, RELATIVE question: did commit_id CHANGE
		// between two consecutive decisive reviews read from the same payload? And the one
		// error direction anyone has observed — commit_id drifting TOWARD head — makes two
		// genuinely-different commits read as the SAME one, which suppresses the approval
		// and REFUSES the flip. That is the fail-closed direction. The error that would
		// fail this gate OPEN is the opposite one (commit_id differing when no push
		// happened), and no observation of it exists. So: not proof the field is sound —
		// its error direction remains could-not-check — but the residual is a gate that
		// over-refuses, never one that over-grants.
		if r.CommitID != lastCommit {
			lastCommit = r.CommitID
			commitBlocked = false
		}

		if r.State == "CHANGES_REQUESTED" {
			commitBlocked = true
			state, head, found, noOpApproval = r.State, r.CommitID, true, false
			continue
		}

		// r.State == "APPROVED" here.
		if commitBlocked {
			// No push since the standing CHANGES_REQUESTED at this exact commit — this
			// APPROVED verifies nothing new. Keep reporting the standing rejection, and
			// leave commitBlocked set so a retried forgery at the same commit is refused
			// too (it does not matter how many APPROVED reviews pile up at this commit;
			// none of them clear the block).
			state, head, found, noOpApproval = "CHANGES_REQUESTED", r.CommitID, true, true
			continue
		}
		state, head, found, noOpApproval = r.State, r.CommitID, true, false
	}
	return state, head, found, noOpApproval
}

// lane is which of the two required artifacts a review body is.
type lane int

const (
	laneCorrectness lane = iota // a correctness verdict, or no readable verdict at all
	laneSecurity                // a security verdict and nothing else
	laneBoth                    // both markers in one body — grants nothing
)

// classifyLane assigns a review body to a lane on the SAME emphasis-tolerant reading both
// lanes use.
//
// WHY NOT VerdictKind (#238 direction 3, R2 on PR #399): the first cut of the
// lane split excluded a body from the correctness lane only when the STRICT
// bodycheck.VerdictKind called it security, while gate (e) admitted it on the TOLERANT
// read. `**Security-Review: pass**` — the form BOTH live fixtures use — therefore fell
// through the crack: unreadable to the strict parse, so it stayed in the correctness lane
// as an APPROVED and masked a live correctness CHANGES_REQUESTED at the same head, while
// simultaneously satisfying gate (e). That is verbatim the defect #238 is closed on, so
// the split has to be made on one reading or it is not a split at all.
//
// The three answers, and why each falls the way it does:
//
//   - a security marker and NO correctness verdict line → laneSecurity. A pure security
//     artifact never speaks for the correctness gate.
//   - a security marker AND a correctness verdict line → laneBoth. This is one review
//     claiming to be both of the two artifacts a risk-classed PR requires at one head,
//     which defeats "both must be satisfied at the same head, from their own artifacts".
//     deskpost's own write path refuses such a body (VerdictKind), so it can only arrive
//     out-of-band through the raw `gh pr review` fallback — the path #197 documents as the
//     COMMON one under a saturated budget. It is admitted to NEITHER lane as a GRANT: it
//     cannot be the correctness APPROVED (latestAppVerdict skips it) and it cannot be the
//     security pass (securityVerdictAtHead skips it). It CAN still block, in both lanes: a
//     CHANGES_REQUESTED still counts, and a `fail` marker still retracts. Ambiguity
//     resolves toward blocking in every direction, never toward a grant.
//   - anything else → laneCorrectness, including a body with no readable verdict of any
//     kind. That is deliberate and measured, not a shrug: most live App review bodies do
//     not parse to a kind and are overwhelmingly prose correctness reviews. Dropping them
//     would refuse nearly every real flip; admitting them to the SECURITY lane would let
//     an unreadable body satisfy a security gate, which is the direction that must never
//     be guessed.
func classifyLane(body string) lane {
	sec := classifySecurityBody(body)
	if sec == secNone {
		return laneCorrectness
	}
	if bodycheck.CorrectnessVerdictLine(body) {
		return laneBoth
	}
	return laneSecurity
}

// secVerdict is one review body's security-review verdict.
type secVerdict int

const (
	secNone secVerdict = iota // the body carries no Security-Review line
	secPass
	secFail
)

// classifySecurityBody reduces a single review body to its security verdict. A body
// carrying BOTH a pass line and a fail line is ambiguous and classifies as FAIL — the
// gate fails closed, so a `pass` line smuggled alongside a `fail` cannot read as green
// (#216).
func classifySecurityBody(body string) secVerdict {
	pass := bodycheck.HasSecurityReviewPass(body)
	fail := bodycheck.HasSecurityReviewFail(body)
	switch {
	case fail:
		return secFail
	case pass:
		return secPass
	default:
		return secNone
	}
}

// securityVerdictAtHead reduces every App security verdict at head to ONE governing
// verdict — secPass, secFail, or secNone when nobody spoke.
//
// Reviews arrive in ascending submitted order, so the reduction is ORDER-SENSITIVE:
// per author, the LAST security verdict at head governs, and every author that has
// spoken must end on `pass` for the result to be secPass. A `pass` later retracted by a
// `fail` at the same head is therefore NOT green (#216) — the old
// any-pass-wins reduction could not see a retraction at all, because `fail` was never
// parsed.
//
// The per-author reduction is deliberate even though the loop currently admits only
// reviewerBotDisplay(): the "last verdict wins" property must belong to the reduction, not
// to the accident of there being exactly one admitted author.
//
// Returning the VERDICT rather than a bool is what lets runReady distinguish the two
// answers that are not the same: "no security verdict at head" (blocks a risk-classed
// flip only) and "the security verdict is FAIL" (blocks every flip — see gate (e0)).
// Collapsing both to `false`, as the old securityPassAtHead did, is what made an explicit
// retraction indistinguishable from silence.
func securityVerdictAtHead(reviews []reviewInfo, head string) secVerdict {
	last := map[string]secVerdict{}
	var authors []string
	for _, r := range reviews {
		if !isReviewerBot(r.User.Login) || r.CommitID != head {
			continue
		}
		v := securityGrantOf(r)
		if v == secPass && head == "" {
			// A verdict GRANTS only by binding to the commit it was issued against, and
			// the empty string is not a commit. If head ever reads empty, `r.CommitID !=
			// head` above stops rejecting anything with an empty commit_id, so every
			// head comparison in the gate goes vacuous at once — exactly when there is
			// least reason to believe a grant. gate (a) refuses a malformed PR payload
			// before this is reachable, so this is depth rather than a live bypass; it is
			// here because head-binding is the property gate (e) rests on, and a property
			// that silently holds only for non-empty inputs is not the property.
			//
			// The asymmetry of securityGrantOf is preserved deliberately: a FAIL is NOT
			// skipped here. An unbound retraction still blocks, because every reason to
			// doubt a fail is a reason to keep blocking.
			continue
		}
		if v == secNone {
			continue // a review with no security line neither grants nor retracts
		}
		if _, seen := last[r.User.Login]; !seen {
			authors = append(authors, r.User.Login)
		}
		last[r.User.Login] = v
	}
	if len(authors) == 0 {
		return secNone
	}
	for _, a := range authors {
		if last[a] == secFail {
			return secFail // any standing retraction governs the whole reduction
		}
	}
	return secPass
}

// securityGrantOf reduces ONE review — body and GitHub review STATE together — to what it
// contributes to the security lane. It is the asymmetric half of the reduction: what it
// takes to GRANT a pass is strictly more than what it takes to record a FAIL.
//
// A pass requires all three of: a readable `Security-Review: pass` marker, a body that is
// not also claiming the correctness lane (laneBoth), and a review STATE from the SUBMITTED
// allow-list below. Two of those are new:
//
//   - STATE (NB2 on PR #399): the reduction filtered on login and commit only, so ANY App
//     review containing the marker satisfied gate (e) — including a PENDING draft the App
//     had never submitted. See securityPassStates for what the allow-list is and is not.
//   - laneBoth (NB3): one body cannot be both of the two artifacts a risk-classed PR
//     requires at the same head. See classifyLane.
//
// A FAIL is subject to neither. It counts from ANY review state and from a laneBoth body,
// because a retraction that is not heard is the failure this whole cluster exists to
// prevent — and because every reason to doubt a fail is a reason to keep blocking, not a
// reason to flip.
func securityGrantOf(r reviewInfo) secVerdict {
	v := classifySecurityBody(r.Body)
	if v != secPass {
		return v // secFail and secNone are unconditional
	}
	if classifyLane(r.Body) == laneBoth {
		return secNone
	}
	if !securityPassStates[r.State] {
		return secNone
	}
	return secPass
}

// securityPassStates is the allow-list of GitHub review STATES a `Security-Review: pass`
// may be carried on. It is an allow-list, not a deny-list, so a state GitHub adds later
// grants nothing until someone decides it should.
//
// APPROVED is the legacy shape (`deskpost review --verdict approve` with a security body).
// COMMENTED is the shape `deskpost security-review --verdict pass` posts, and the reason
// this is a set rather than the single `== "APPROVED"` NB2 installed
// (#513 / #438).
//
// WHY COMMENTED IS ADMITTED, since NB2 excluded it deliberately. NB2's rule was derived
// from the convention of the day — "the convention posted a pass as `--approve`, so a
// non-APPROVED state is not that artifact" — and the convention is what #513 changed. The
// pr-review-desk skill required the clean security pass to be posted in a shape that does
// NOT flip the board's review state to APPROVED while correctness findings stand, and the
// only shape it had was an issue COMMENT, which no review reader can see. gate (e)
// therefore could not be satisfied by the artifact it exists for, on any risk-classed PR.
// A COMMENT-event REVIEW is the shape that satisfies
// both constraints at once: GitHub records it in GET /pulls/{n}/reviews with a commit_id
// this reduction can head-pin on, and its state is COMMENTED, which never enters GitHub's
// approval reduction. The commit_id is caller-supplied on POST, not server-derived — what
// makes it truthful is that deskpost verifies it against the live head before submitting
// (see postVerdictReview in review.go). The pin is real; the guarantee is this tool's.
//
// It is NOT a widening of who may grant. Every review in this loop has already been
// filtered to reviewerBotLogin, so the only identity that can produce one is the reviewer
// App itself — the same identity that could post `--approve`. The state was never an
// authorization boundary; it was a ceremony check, and the ceremony moved.
//
// What the allow-list still excludes, and why each matters:
//
//   - PENDING — an unsubmitted draft review. GitHub returns PENDING reviews on this
//     endpoint TO THEIR AUTHOR, and deskpost reads as the author, so an abandoned draft
//     carrying the marker is visible here. It was never submitted; it asserts nothing.
//   - DISMISSED — an approval a maintainer explicitly dismissed. Reading a dismissal as a
//     standing grant inverts it.
//
// It also closes a divergence rather than opening one: deskboard's review reduction
// has never filtered on state at all, so a
// COMMENTED marker already counted there while deskpost refused it. The two now agree on
// the shape the desk actually posts.
var securityPassStates = map[string]bool{
	"APPROVED":  true, // legacy: `deskpost review --verdict approve`, security body
	"COMMENTED": true, // `deskpost security-review --verdict pass`
}

// securityPassAtHead reports whether the governing security verdict at head is pass.
// Absence is never a pass.
func securityPassAtHead(reviews []reviewInfo, head string) bool {
	return securityVerdictAtHead(reviews, head) == secPass
}
