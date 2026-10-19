package reviewflow

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	ActMergeNow = "MERGE-NOW"
	// ActHumanGate (#241) is terminal for a PR carrying a machine-readable human-gate
	// declaration: it replaces MERGE-NOW/FLIP so the board can never print "merge now"
	// for work whose merge is reserved to a human, and it is excluded from the
	// MERGE-NOW count and decay banner.
	ActHumanGate = "HUMAN-GATE"
	// ActSuspectApproval (#37) ranks above BLOCKED: a no-op APPROVED posted over a
	// standing CHANGES_REQUESTED at an unchanged head (no intervening push) is a
	// forgery-shaped verdict, not a routine "needs work" row — surface it LOUDLY rather
	// than fold it into an ordinary BLOCKED read.
	ActSuspectApproval = "SUSPECT-APPROVAL"
	// ActExternalPrereqReview (brief 21) is the SUSPECT-APPROVAL row's benign
	// twin: an APPROVED over a standing CHANGES_REQUESTED at an unchanged head where the CR
	// DECLARED itself external-prereq-only. It is not a forgery signal — it is a reviewer
	// clearing a rejection whose only blockers were external prerequisites — so it is
	// surfaced for the ready gate to verify, never folded into SUSPECT-APPROVAL and never a
	// FLIP on the advisory board (the board does not perform the independent verification).
	ActExternalPrereqReview = "EXTERNAL-PREREQ-REVIEW"
	ActBlocked              = "BLOCKED"
	ActSecReview            = "SECURITY-REVIEW-REQUIRED"
	// ActCIUnknown (#268) is the CI three-state's fourth outcome: the rollup carried
	// entries this board cannot interpret, so the CI verdict is not established. It
	// blocks the approve+green path rather than defaulting either way.
	ActCIUnknown = "CI-UNKNOWN"
	// ActCIUnverified (#400 R2) is the OTHER way the CI verdict fails to exist: not an
	// entry we could not read, but NO entry at all on a repo that runs PR CI (an empty
	// or absent `statusCheckRollup`, or one carrying only SKIPPED/NEUTRAL). ciUnknown
	// counts what was unreadable; this counts what was never there. Both mean "green was
	// not established", and neither may reach a FLIP/MERGE-NOW note that says "CI green".
	ActCIUnverified = "CI-UNVERIFIED"
	ActCIRed        = "CI-RED"
	// ActCINeverRan (#1652) is the rollup being empty for a DIFFERENT reason than
	// ActCIUnverified: a workflow that WOULD fire on this diff never ran at head at
	// all (dispatch failure, not "no CI configured"), so the PR is UNVALIDATED rather
	// than CI-green-or-red.
	ActCINeverRan = "CI-NEVER-RAN"
	ActWaitCI     = "WAIT-CI"
	ActConflict   = "CONFLICT"
	// ActMergeStateUnknown (#400 R3) is the mergeability four-state's third arm:
	// GitHub returned `UNKNOWN` (mergeability still computing) or nothing at all. The
	// board must not convert that silence into MERGE-NOW.
	ActMergeStateUnknown = "MERGE-STATE-UNKNOWN"
	// ActMergeBehind (#54) is the mergeability four-state's fourth arm: mergeStateStatus
	// read BEHIND — a MEASURED "main has moved since this head was last synced", not an
	// unmeasured unknown. The review that approved this head verified it against a `main`
	// that is no longer the merge target. MERGE-NOW must not fire off a stale-base review;
	// this is the retro finding #54 exists to close.
	ActMergeBehind = "MERGE-BEHIND"
	ActFlip        = "FLIP"
	ActReReview    = "RE-REVIEW"
	ActNeedsReview = "NEEDS-REVIEW"
	ActMergeCurr   = "MERGE-CURR"
	ActReady       = "READY"
	ActCheck       = "CHECK"
)

// GuardArmMarker is the distinct phrase the fail-loud guard arm (#400 R2) puts in its
// note, and nothing else in this package may say. It is a named constant rather than an
// inline string because of N6: the tripwire proving that arm is UNREACHABLE keyed on a
// substring of the note the arm itself returns, so a mutation that replaced the return
// with the old `FLIP` + "CI green" note deleted the tripwire along with the arm and the
// whole suite stayed green — a guard that removes itself when it is defeated. Keyed on
// this constant, and with TestClassify_GuardArmIsStillTheGuardArm asserting the arm still
// returns it, the same mutation is now visible.
const GuardArmMarker = "NO CI/merge arm of the classifier claimed this row"

type ClassifyInput struct {
	Ever            bool
	AtHead          bool
	Blocking        bool
	ApprovedAtHead  bool
	Draft           bool
	Pass            int
	Pending         int
	Fail            int
	OwnFilesChanged bool
	RiskClassed     bool
	// riskReason names WHY the row is risk-classed (which union term fired), so the
	// SECURITY-REVIEW-REQUIRED row says why rather than always blaming a security path —
	// e.g. "trailer absent on App-authored PR" (#587).
	RiskReason    string
	SecurityPass  bool
	CiGreen       bool
	MergeConflict bool
	// mergeStateUnknown (#400 R3): the mergeStateStatus could not be read (UNKNOWN,
	// absent, or a value this code does not recognise). Distinct from mergeConflict —
	// that is a measured "no", this is "we did not measure". It blocks MERGE-NOW.
	MergeStateUnknown bool
	// mergeBehind (#54): mergeStateStatus is BEHIND — main has moved since this head was
	// last synced. Distinct from BOTH mergeConflict (a measured "no, it will not merge")
	// and mergeStateUnknown (an unmeasured "we don't know"): this is a measured "the base
	// this review verified is no longer the base a merge would use". It blocks MERGE-NOW
	// the same way mergeStateUnknown does, but for the opposite reason — GitHub read the
	// state fine, and what it read says the review is stale against current main.
	MergeBehind bool
	// mergeStateRaw is the verbatim mergeStateStatus, so the could-not-check note can
	// name what actually came back instead of asserting a cause.
	MergeStateRaw string
	// ciUnknown (#268): rollup entries this board could not interpret. >0 means the CI
	// verdict was not established — never treated as green, never as merely pending.
	CiUnknown int
	// humanGate (#241): the PR carries a machine-readable human-gate declaration.
	HumanGate       bool
	HumanGateReason string
	// zeroCI (#1652): the probed state of a 0/0/0 rollup — see the type doc above.
	ZeroCI string
	// suspectNoOp (#37): the effective blocking state is a suppressed no-op approval —
	// an APPROVED posted over a standing CHANGES_REQUESTED at the same head, with no
	// intervening push. See reduceReviews.
	SuspectNoOp bool
	// externalPrereqDeclared (brief 21): the suspectNoOp row's standing CR
	// declared itself external-prereq-only, so it is a declared external-prerequisite
	// re-review, not a forgery. Consulted ONLY with suspectNoOp, to route the row to
	// EXTERNAL-PREREQ-REVIEW instead of SUSPECT-APPROVAL.
	ExternalPrereqDeclared bool
	// nonCommitResolution: the PR carries a standing CHANGES_REQUESTED at head,
	// but a finding-relevant change that leaves the head sha UNCHANGED — a resolution
	// label (e.g. `*:skip`) added, or the body/title edited — landed AFTER the last
	// review. The head-sha-keyed re-review trigger never sees such a fix, so the row would
	// sit BLOCKED (worker-owns) indefinitely until a human flagged it. When set, the
	// blocking arm re-flags the row RE-REVIEW so a reviewer re-examines it (and verifies
	// the check state at head, since a label/body edit does not re-run CI). Consulted ONLY
	// in the plain blocking arm — a suspected forged no-op flip (suspectNoOp) still wins,
	// so this can never mask that. Self-limiting: once the re-review posts, the new
	// verdict's submitted time is AFTER the label/edit, so the signal clears — no loop.
	NonCommitResolution     bool
	NonCommitResolutionNote string
}

func Classify(in ClassifyInput, reviewer string) (action, note string) {
	switch {
	case !in.Ever:
		// #2028: authorship is NOT a reason to skip review. The trust gate in
		// classifyPRFrom (deskkit.TrustedAuthor, then a current blessing) is the ONE
		// authorship filter: an unblessed author never reaches classify (quarantined in
		// EXTERNAL / UNBLESSED), and every author it admits — a trusted human, a trusted
		// shared login, a role App, a blessed outsider — is dispatched alike. The retired
		// #177 HUMAN-OWNED arm skipped a trusted human's own PR here; a reintroduction of
		// that shape is held by TestAuthorSkipClassGuard.
		return ActNeedsReview, "no bot APPROVED/CHANGES_REQUESTED at head — dispatch a reviewer"
	case !in.AtHead:
		if !in.OwnFilesChanged {
			return ActMergeCurr, "keep-current merge; PR's own files unchanged since last review — no re-review"
		}
		return ActReReview, "head advanced past last review (PR's own files changed) — delta re-review"
	// #37: an APPROVED that immediately follows the App's own CHANGES_REQUESTED at the
	// SAME head, with no push in between, is not a re-verification — surface it as a
	// suspected forgery/no-op-flip attempt, distinct from an ordinary blocking review, so
	// a human reading the board sees the attempt rather than a routine BLOCKED row. Never
	// FLIP-eligible either way, but which of the two it is matters for the response (chase
	// down who minted the App token vs. wait on the worker).
	// brief 21: the declared external-prerequisite re-review. It has the SAME
	// shape as a suspected no-op flip — an APPROVED over a standing CHANGES_REQUESTED at an
	// unchanged head — but the CR declared itself external-prereq-only, so it is a reviewer
	// clearing a rejection whose only blockers were external prerequisites, not a forgery.
	// Checked BEFORE the suspectNoOp arm so a declared row never reads as SUSPECT-APPROVAL.
	// It is SURFACED, never FLIP-eligible: the board does not perform the independent
	// prerequisite verification — the ready gate does, at the ready boundary — so the board
	// only says "this is a declared external-prereq re-review; the ready gate will verify".
	case in.Blocking && in.SuspectNoOp && in.ExternalPrereqDeclared:
		return ActExternalPrereqReview, reviewer + " cleared a standing CHANGES_REQUESTED at the " +
			"unchanged head that DECLARED itself external-prereq-only — a declared external-prerequisite " +
			"re-review, not a no-op forgery. The ready gate independently verifies every prerequisite from " +
			"fresh evidence before any flip; surfaced here, never auto-flipped (brief 21)"
	case in.Blocking && in.SuspectNoOp:
		return ActSuspectApproval, reviewer + " posted APPROVED at head immediately after its " +
			"own CHANGES_REQUESTED at the SAME head, with no intervening push — that cannot be a " +
			"re-verification (#37); treating as a suspected forged/no-op flip attempt, never FLIP-eligible, " +
			"until a new commit lands"
	// Non-commit-resolution re-review: a standing CHANGES_REQUESTED at head normally stays
	// BLOCKED (worker-owns) until a NEW commit moves the head and re-triggers review. But a
	// worker can resolve a finding WITHOUT a commit — adding a resolution label (e.g.
	// `*:skip`) or editing the body — which leaves the head unchanged, so the head-sha
	// re-review trigger never fires and the row sits BLOCKED forever. When such a
	// finding-relevant non-commit change landed after the last review, re-flag RE-REVIEW so
	// the row re-enters the dispatch gate and a reviewer re-examines it. This never WEAKENS
	// the blocking signal: with no such change (the common case) the row stays BLOCKED
	// exactly as before, and a suspected forged no-op flip is handled by the arm above,
	// never here.
	case in.Blocking && in.NonCommitResolution:
		note := in.NonCommitResolutionNote
		if note == "" {
			note = "standing CHANGES_REQUESTED at head, but a finding-relevant non-commit change " +
				"(resolution label or body edit) landed after the last review"
		}
		return ActReReview, note + " — re-flag for re-review (the head-sha trigger cannot see " +
			"a non-commit fix); the reviewer must verify the check state at head, since a label/body " +
			"edit does not re-run CI"
	case in.Blocking:
		return ActBlocked, reviewer + " requested changes at head — worker must act"
	// A DEFINITE failure outranks an indefinite unknown. When the rollup carries both,
	// reporting CI-UNKNOWN alone hid a red behind "inspect the checks" — and a red check
	// is a work item with a known next step, while an unknown is an instruction to go
	// look. Both are stated: the action names the red, the note carries the unmeasured
	// count so the unknown is not quietly absorbed into a state that was measured.
	case in.Fail > 0 && in.CiUnknown > 0:
		return ActCIRed, fmt.Sprintf("a check FAILED (%d) — that is a definite work item; %d rollup "+
			"entr%s ALSO could not be interpreted, so the REST of CI is not established either",
			in.Fail, in.CiUnknown, plural(in.CiUnknown, "y", "ies"))
	// #268: an uninterpretable check rollup means the CI verdict was NOT established.
	// It has to preempt every approve+green path below, which would otherwise treat
	// "no failures and no pendings" as green and reach FLIP.
	case in.CiUnknown > 0:
		return ActCIUnknown, fmt.Sprintf("%d check-rollup entr%s could not be interpreted — "+
			"CI state is NOT established (not green, not merely pending); inspect the checks before any flip",
			in.CiUnknown, plural(in.CiUnknown, "y", "ies"))
	// #400 R2: approved at head, nothing failed, nothing pending, nothing unreadable —
	// and STILL not green. On a CI-required repo that means NOT ONE check reported a
	// verdict at this head: an empty/absent rollup, or one carrying only SKIPPED/NEUTRAL
	// entries. Without this arm the flow fell through to the late `case in.ApprovedAtHead:`
	// arm, which returns FLIP with a note that ASSERTS "CI green" and never consults
	// ciGreen — the tool printing a conclusion about checks it never saw.
	//
	// deskpost's ready gate refuses the identical state (cmd/deskpost/ready.go, `case
	// ciEmpty` on a CIRequired repo → unverifiable, exit 6). One tool exiting 6 while the
	// other recommends the flip is a live divergence on the CI axis, so the two now agree.
	//
	// The zero-CI disambiguation folds in here rather than beside it: `ciGreen` false with fail==0 && pending==0
	// (and ciUnknown==0, excluded above) is ALWAYS the same zero-rollup shape the zero-CI probe
	// exists to explain, so its three states drive the SPECIFIC reason instead of one flat
	// "unverified" — never-ran (a real dispatch failure) is a different work item than a
	// rollup the board could not probe, which is different again from a probed, CONFIRMED
	// no-checks zero that a CI-required repo's policy still refuses to call green. An empty
	// `zeroCI` (never probed — e.g. no caller-side probe was run) keeps the original R2
	// blanket note as the fallback.
	case in.ApprovedAtHead && !in.CiGreen && in.Fail == 0 && in.Pending == 0:
		// A risk-classed PR without a security-review pass at head must stay
		// SEC-REVIEW-REQUIRED regardless of the CI-zero reason.
		if in.RiskClassed && !in.SecurityPass {
			return ActSecReview, "risk-classed (" + secReviewReason(in.RiskReason) + ") and no '" + "Security-Review: pass" +
				"' from " + reviewer + " at head — security review required before FLIP or merge"
		}
		switch in.ZeroCI {
		case ZeroCINeverRan:
			return ActCINeverRan, "workflow(s) exist that would fire on this diff but NO check ran at head — " +
				"the PR is UNVALIDATED (0 checks is absence of evidence, not green); retrigger/investigate CI " +
				"before any flip"
		case ZeroCIUnverified:
			return ActCheck, "check rollup is empty and the board could NOT verify why — could-not-check is " +
				"not green; do not flip on an unverified zero"
		case ZeroCINoChecks:
			return ActCheck, "approved at head and verified NO workflow fires on this diff, but repo CI policy " +
				"treats an empty rollup as unverifiable at the deskpost gate — human call, not a routine flip"
		}
		return ActCIUnverified, reviewer + " APPROVED at head, but NO check reported a verdict there — " +
			"the rollup was empty/absent (or carried only skipped/neutral entries) on a repo that runs PR CI, " +
			"so CI green is NOT established; deskpost `ready` refuses this same state (exit 6). Confirm the " +
			"checks actually ran before any flip"
	// approved at head + CI green → MERGE-NOW (ranks above READY/FLIP).
	// A risk-classed row without a security pass at head stays blocked
	// (SECURITY-REVIEW-REQUIRED), draft OR ready (#2158): a ready PR that took a new head
	// and a correctness re-approval there, but no fresh security pass, must not read
	// MERGE-NOW. securityPass is head-bound by reduceReviews (sameHead), so a pass
	// recorded at an older head never satisfies this check.
	case in.ApprovedAtHead && in.CiGreen && !in.MergeConflict:
		// #400 N9: "CI green" is a VERDICT, and on a repo the policy marks as running no
		// PR CI (deskkit.CIRequired false) with nothing in the rollup, no check ran to
		// return one. ciGreen is VACUOUSLY true there — a policy statement, not a result.
		// The action is policy-backed and unchanged; the board simply stops asserting a
		// result nobody produced, which is this PR's own thesis applied to its wording.
		ciPhrase := "CI green"
		if in.Pass == 0 {
			ciPhrase = "no PR CI configured for this repo and nothing ran (not a green verdict)"
		}
		if in.RiskClassed && !in.SecurityPass {
			return ActSecReview, "risk-classed (" + secReviewReason(in.RiskReason) + ") and no '" + "Security-Review: pass" +
				"' from " + reviewer + " at head — security review required before FLIP or merge"
		}
		// #1652: on a CI-less repo with a probed no-checks zero the green is
		// vacuous — say so, so "CI green" never silently includes it.
		checkedZero := ""
		if in.ZeroCI == ZeroCINoChecks {
			checkedZero = " (CI is a CHECKED zero: verified no workflow fires on this diff, #1652)"
		}
		// #241: a declared human gate never resolves to MERGE-NOW. Security still
		// ranks above it — a risk-classed PR needs its verdict either way.
		if in.HumanGate {
			return ActHumanGate, HumanGateNote(in.Draft, in.HumanGateReason, ciPhrase+checkedZero, reviewer)
		}
		// #400 R3: mergeability was never established. MERGE-NOW is a recommendation to
		// merge, and it must rest on a READ merge state, not on the absence of a
		// conflict signal. Flipping a draft ready is still safe here (it does not depend
		// on mergeability), so the note says which half survives.
		if in.MergeStateUnknown {
			if in.Draft {
				return ActFlip, reviewer + " APPROVED at head, " + ciPhrase + checkedZero + ", draft — the ready-flip is safe " +
					"(it does not depend on mergeability). MERGE-NOW is WITHHELD: mergeStateStatus came back " +
					MergeStateDisplay(in.MergeStateRaw) + ", so mergeability was NOT established — re-read " +
					"the PR before merging"
			}
			return ActMergeStateUnknown, reviewer + " APPROVED at head and " + ciPhrase + checkedZero + ", but mergeStateStatus " +
				"came back " + MergeStateDisplay(in.MergeStateRaw) + " — GitHub had not finished computing " +
				"mergeability, or the field could not be read. MERGE-NOW is NOT established; re-read the PR " +
				"before merging"
		}
		// #54: mergeStateStatus read BEHIND — a MEASURED "main has moved since this head
		// was last synced", not an unmeasured unknown. The App's APPROVED verdict verified
		// the diff against the `main` this head was based on; it never re-asked the
		// question against CURRENT main. Flipping a draft ready is still safe (it does not
		// depend on mergeability, same as the mergeStateUnknown arm above) — but MERGE-NOW
		// is withheld, because that recommendation would be trusting a review that never
		// saw the tree it would actually land in.
		if in.MergeBehind {
			if in.Draft {
				return ActFlip, reviewer + " APPROVED at head, " + ciPhrase + checkedZero + ", draft — the ready-flip is safe " +
					"(it does not depend on mergeability). MERGE-NOW is WITHHELD: mergeStateStatus is " +
					MergeStateDisplay(in.MergeStateRaw) + " — main has moved since this head was last synced, so " +
					"the review did not verify against the CURRENT base (#54). Sync with main and get a fresh " +
					"review before merging"
			}
			return ActMergeBehind, reviewer + " APPROVED at head and " + ciPhrase + checkedZero + ", but mergeStateStatus is " +
				MergeStateDisplay(in.MergeStateRaw) + " — main has moved since this head was last synced. The " +
				"review verified this diff against a base that is no longer current (#54). MERGE-NOW is NOT " +
				"established; sync with main and get a fresh review before merging"
		}
		if !in.Draft {
			return ActMergeNow, reviewer + " APPROVED at head, " + ciPhrase + ", ready — merge now" + checkedZero
		}
		return ActMergeNow, reviewer + " APPROVED at head, " + ciPhrase + ", draft — desk flips, then merge now" + checkedZero
	case !in.Draft:
		return ActReady, "already ready — awaiting human/merge"
	case in.Fail > 0:
		return ActCIRed, "bot APPROVED but a check failed/cancelled"
	case in.Pending > 0:
		return ActWaitCI, "bot APPROVED, CI still pending"
	case in.MergeConflict:
		return ActConflict, "mergeStateStatus is DIRTY/BLOCKED — PR cannot be merged; flip is unsafe"
	// #400 R2: this arm used to be `case in.ApprovedAtHead:` returning FLIP with a note
	// that ASSERTED "CI green" while never consulting ciGreen — every FLIP the live board
	// has ever printed came out of here, and every one of them was a row whose CI was
	// never established (the MERGE-NOW arm above already takes the genuinely green case).
	// With CI-UNVERIFIED above and MERGE-NOW/FLIP below it, approvedAtHead is now fully
	// resolved before reaching here. It is kept as a FAIL-LOUD guard rather than deleted:
	// if a later arm change ever routes an approved row down here, the board must say it
	// has no verdict instead of inventing the friendliest one.
	// TestClassify_ActionInventory pins that no reachable input reaches this arm.
	case in.ApprovedAtHead:
		return ActCheck, reviewer + " APPROVED at head, but " + GuardArmMarker +
			" — deskboard cannot state a verdict for it. This is a deskboard bug, not a PR state: " +
			"inspect, and do not read it as a flip"
	default:
		return ActCheck, "bot review at head is neither APPROVED nor CHANGES_REQUESTED — inspect"
	}
}

func secReviewReason(reason string) string {
	if strings.TrimSpace(reason) == "" {
		return "touches a security path"
	}
	return reason
}
func MergeStateDisplay(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return "ABSENT (no mergeStateStatus in the payload)"
	}
	return strconv.Quote(raw)
}
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

const (
	ZeroCINoChecks   = "no-checks"
	ZeroCINeverRan   = "checks-never-ran"
	ZeroCIUnverified = "unverified"
)
