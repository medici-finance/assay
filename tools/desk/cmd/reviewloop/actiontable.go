package main

import (
	"sort"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Disposition is what the reactor does with one board row. It is deliberately NOT a
// drain-style "work item / done" two-state: the waiting dispositions are first-class,
// because a row the reactor takes no action on must still be VISIBLE. A board row that
// falls off the reactor's output because nothing was to be done about it is the same
// class of defect as a false idle.
type Disposition int

const (
	// DispositionUnknown is the ZERO VALUE, and it is deliberately not a real
	// disposition. A rule that was never populated — a lookup that returned early, a
	// table entry someone half-wrote — must decay to "I do not know what this row is",
	// not to an action. The zero value of an enum is what a bug hands you, so it is the
	// one value that must be safe.
	DispositionUnknown Disposition = iota
	// DispositionDispatch — fill a reviewer slot for this row.
	DispositionDispatch
	// DispositionFlip — the desk's ready-flip verb (deskpost), gated on the
	// dual-verdict-at-head rule for risk-classed PRs. The reactor EMITS the verb; every
	// existing deskpost constraint still applies at the call. This is a caller, never a
	// bypass, and it is not a merge.
	DispositionFlip
	// DispositionSurface — no automatic verb, but the row is a live work item somebody
	// must see: CI red, a conflict, an unvalidated or unreadable CI verdict, a
	// human-gated PR, a MERGE-NOW awaiting the human's merge.
	DispositionSurface
	// DispositionWait — waiting on someone else's move (the worker, CI, a merge). No
	// dispatch, no verb; the row stays on the board and is never dropped.
	DispositionWait
	// DispositionNoOp — the board classified the head-advance as benign; explicitly
	// nothing to do, and explicitly not a re-review.
	DispositionNoOp
)

func (d Disposition) String() string {
	switch d {
	case DispositionDispatch:
		return "DISPATCH"
	case DispositionFlip:
		return "FLIP-VERB"
	case DispositionSurface:
		return "SURFACE"
	case DispositionWait:
		return "WAIT"
	case DispositionNoOp:
		return "NO-OP"
	default:
		return "UNKNOWN-DISPOSITION"
	}
}

// Actionable reports whether this disposition consumes a reviewer slot. Only the two
// dispatching actions do; it is the predicate behind the idle gate's count.
func (d Disposition) Actionable() bool { return d == DispositionDispatch }

// rule is one row of the action table: what to do, and the one-line reason, so the
// reactor's output states WHY a row got no verb rather than merely omitting it.
type rule struct {
	Disposition Disposition
	// Verb is the deskpost verb this disposition would call, "" when none. It is half of
	// the coalescing key, so a row with no verb cannot collapse into one that has one.
	Verb string
	Why  string
}

// actionTable is the reactor's complete action semantics, keyed by the ACTION strings
// deskboard emits. It MUST stay exhaustive over deskboard's act* constant block:
// TestActionTableIsExhaustiveOverDeskboard parses those constants out of
// tools/desk/cmd/deskboard/board.go and fails on any action missing here.
//
// The nine actions the pr-review-desk skill names (NEEDS-REVIEW, RE-REVIEW, BLOCKED,
// CHECK, WAIT-CI, CI-RED, MERGE-CURR, FLIP, READY) are a SUBSET of what the board
// computes. The other nine are the ones a reactor written from the skill would drop.
var actionTable = map[string]rule{
	// ---- the two dispatching states ----
	"NEEDS-REVIEW": {DispositionDispatch, "review", "no reviewer verdict at head — fill a reviewer slot at the risk-keyed tier"},
	"RE-REVIEW":    {DispositionDispatch, "review", "head advanced past the last review and the PR's own files changed — delta re-review; resume the original reviewer where its session survives"},

	// ---- the outward-write state ----
	// PR-state labels (pr-review-desk skill, §PR-state labels): the desk executing this
	// verb also swaps `authorization-needed` → `approval-needed` in the same turn — the
	// review lane has approved everything, so the PR now visibly waits on the HUMAN's
	// merge approval. The swap rides the executed verb; this planner stays read-only and
	// never applies labels itself.
	"FLIP": {DispositionFlip, "ready", "approved at head and green — the desk's ready-flip verb, under deskpost's existing App-identity and dual-verdict-at-head gates"},

	// ---- benign head advance ----
	"MERGE-CURR": {DispositionNoOp, "", "keep-current merge; the PR's own files are unchanged since the last review — deliberately NOT a re-review"},

	// ---- waiting on someone else, visible, never dropped ----
	"BLOCKED": {DispositionWait, "", "the reviewer bot requested changes at head — the worker must act"},
	"WAIT-CI": {DispositionWait, "", "CI is still running at head — waiting, not idle"},
	// PR-state labels: a READY row carries `approval-needed` until the human merges; the
	// sweep that observes the row gone because the PR merged clears the label (skill-side —
	// see pr-review-desk §PR-state labels; this planner never writes).
	"READY": {DispositionWait, "", "already flipped ready; waiting on the human's merge — the desk does not merge"},

	// ---- work items with no automatic verb ----
	"CHECK":                    {DispositionSurface, "", "the board could not claim this row with any CI/merge arm — a deskboard defect signal, surfaced rather than swallowed"},
	"CI-RED":                   {DispositionSurface, "", "a check failed at head — red is the worker's; the desk routes it"},
	"CONFLICT":                 {DispositionSurface, "", "the PR does not merge cleanly — the worker's to resolve"},
	"MERGE-NOW":                {DispositionSurface, "", "approved, green and mergeable — the MERGE is the human's (standing duty: surface it, never merge it)"},
	"HUMAN-GATE":               {DispositionSurface, "", "the PR carries a machine-readable human-gate declaration — terminal for the reactor"},
	"SECURITY-REVIEW-REQUIRED": {DispositionSurface, "", "risk-classed PR without Security-Review:pass — routed to the desk's security lane, and never a FLIP signal"},
	// SURFACE, emphatically not WAIT (#37). BLOCKED is a WAIT because it is routine: the
	// reviewer asked for changes and the worker owes a push. SUSPECT-APPROVAL is the same
	// row with an APPROVED sitting on top of the standing rejection at an UNCHANGED head —
	// an approval that verified nothing, which is the observable signature of the forgery
	// this action was added for. Filing it as WAIT would fold a suspected forged verdict
	// back into the routine bucket it was ranked above precisely to escape, and the reactor
	// would say nothing about it. A human must see this row.
	"SUSPECT-APPROVAL": {DispositionSurface, "", "an APPROVED at an unchanged head over a standing CHANGES_REQUESTED — it cannot be a re-verification, so the board suppressed it (#37); surfaced for a human, never a FLIP and never folded into BLOCKED"},

	// ---- the CI three-state's could-not-check arms: not green, not red ----
	"CI-UNKNOWN":    {DispositionSurface, "", "the rollup carried entries the board could not interpret — the CI verdict is NOT established"},
	"CI-UNVERIFIED": {DispositionSurface, "", "no CI entry at all on a repo that runs PR CI — green was never established"},
	"CI-NEVER-RAN":  {DispositionSurface, "", "a workflow that would fire on this diff never ran at head — the PR is UNVALIDATED"},

	// ---- the mergeability four-state's non-OK arms ----
	"MERGE-STATE-UNKNOWN": {DispositionSurface, "", "GitHub did not report mergeability — could-not-check, never read as mergeable"},
	"MERGE-BEHIND":        {DispositionSurface, "", "the base moved since this head was synced — the approving review verified a main that is no longer the merge target"},
}

// LookupAction returns the rule for an ACTION string. An action the table does not know
// is deskkit.Unverifiable (exit 6) — the fail-closed direction. The tempting alternative,
// treating an unrecognised action as "nothing to do", is precisely how a board surface
// disappears silently: deskboard has grown from 9 actions to 18, and each addition would
// otherwise have been an invisible narrowing of what the reactor watches.
func LookupAction(action string) (rule, error) {
	r, ok := actionTable[action]
	if !ok {
		return rule{}, deskkit.Unverifiable(
			"reviewloop: deskboard emitted ACTION "+action+" which this reactor's action table does not know — "+
				"COULD-NOT-CHECK for that row (add it to actionTable; never treat an unknown action as no-op)", nil)
	}
	return r, nil
}

// KnownActions returns the table's keys, sorted. Used by the exhaustiveness test and by
// the `plan` header, which prints the watched set so a narrowing is visible in the output
// rather than only in a diff.
func KnownActions() []string {
	out := make([]string, 0, len(actionTable))
	for a := range actionTable {
		out = append(out, a)
	}
	sort.Strings(out)
	return out
}
