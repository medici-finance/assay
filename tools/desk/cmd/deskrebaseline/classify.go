package main

// classify.go — the SINGLE-POINT-OF-FAILURE predicate of the re-baseline lane: deciding,
// from gathered git/tree facts about one failing `## Verify` row, whether the row is
// PROVABLY STALE-BUT-INTACT (a member of the SAFE set the verb may propose re-baselining)
// or must be REFUSED (filed as an issue, never re-baselined).
//
// It is deliberately a PURE function of a RowFacts value: every fact — path existence, the
// rename hop, the brief's risk declaration, the behaviour-probe result — is gathered by
// facts.go and handed in, so the decision itself is testable without git, a checkout, or a
// live command. That separation is what lets the fail-first tests (rows 1–3 of the brief's
// Verify table) pin the decision boundary directly.
//
// FAIL CLOSED. The default is REFUSAL. A verdict lands in the SAFE set only when a fact
// POSITIVELY proves the work intact — a single rename hop to a file that exists, a
// pure-additions count drift, a tool idiom retired by a recorded ruling. Anything the facts
// cannot positively place is refused:unclassified, because a loose "safe" answer here
// launders a real regression into a green (brief 05 §Context, single-point-of-failure).

// Verdict is the classification of a failing Verify row. The safe:* verdicts authorise a
// one-row re-baseline PR; the refused:* verdicts require the row be filed as an issue.
type Verdict string

const (
	// SafeRename — the row pins a path that no longer exists, and git records a SINGLE
	// rename/move hop from it to a path that exists now. The work is intact; only the
	// pinned path moved.
	SafeRename Verdict = "safe:rename"
	// SafeCount — the row's Expect is a count whose current value differs, and the target
	// files changed ONLY by additions since the Evidence date, so the higher count is
	// growth, not a regression.
	//
	// SCAFFOLDING — NOT YET REACHABLE IN THE SHIPPED VERB. The classifier CAN return this
	// verdict, but gatherRowFacts (facts.go) does not yet populate CountShaped/OnlyAdditions,
	// so the shipped fact-gatherer never produces safe:count. Wiring the count facts (and
	// reconciling the step-3 behaviour probe, which pre-empts a count drift written as
	// `test $(...) -eq N`) is a tracked follow-up; until then only safe:rename fires in
	// production, and the user-facing docs (docs/rebaseline.md) say so. Kept, not deleted, so
	// the follow-up inherits the decision boundary and its tests already in place.
	SafeCount Verdict = "safe:count"
	// SafeIdiom — the row pins a tool idiom retired by a recorded ruling (e.g. a renamed
	// flag). The command idiom moved; the checked behaviour did not.
	//
	// SCAFFOLDING — NOT YET REACHABLE IN THE SHIPPED VERB, for the same reason as SafeCount:
	// gatherRowFacts never sets RetiredIdiom. Wired end-to-end by the same tracked follow-up.
	SafeIdiom Verdict = "safe:idiom"

	// RefusedRiskBearing — the row's OWNING BRIEF is risk-bearing (gate: human, or any
	// risk: flag yes). No shape makes such a row re-baselineable by the verb; it is filed.
	RefusedRiskBearing Verdict = "refused:risk-bearing"
	// RefusedGone — the pinned deliverable path is gone with NO single rename hop to an
	// existing file. The verb cannot prove the work survived; it is filed.
	RefusedGone Verdict = "refused:gone"
	// RefusedBehaviourChanged — the row's command still RUNS and returns a different result
	// than the row pinned. That is a real behaviour change, not a stale oracle; it is filed.
	RefusedBehaviourChanged Verdict = "refused:behaviour-changed"
	// RefusedUnclassified — the facts do not positively place the row in any safe class.
	// The fail-closed default: unproven is never re-baselined.
	RefusedUnclassified Verdict = "refused:unclassified"
)

// IsSafe reports whether a verdict authorises a re-baseline PR (a safe:* verdict). Every
// other verdict — including any not enumerated here — is treated as a refusal, so a future
// verdict added without updating this predicate fails closed.
func (v Verdict) IsSafe() bool {
	switch v {
	case SafeRename, SafeCount, SafeIdiom:
		return true
	default:
		return false
	}
}

// RowFacts is everything the classifier needs about one failing Verify row, gathered by
// facts.go. Every field is a POSITIVELY-established fact or its fail-closed absence; the
// classifier never gathers, only decides.
type RowFacts struct {
	// Row is the 1-based row number within the brief's `## Verify` table.
	Row int
	// Command is the row's Command cell, unwrapped of inline-code backticks.
	Command string
	// Expect is the row's Expect cell.
	Expect string

	// RiskBearing is true when the OWNING BRIEF declares itself risk-bearing (gate: human,
	// or any risk: flag yes), read from the brief's own frontmatter. RiskReason names it.
	RiskBearing bool
	RiskReason  string

	// PathRef is the deliverable/target path the row pins, or "" when the row pins no path
	// the gatherer could identify. PathExists says whether it exists in the tree now.
	PathRef    string
	PathExists bool
	// RenameHop is the single existing path a `git log --follow` records the missing PathRef
	// moved to, or "" when there is no hop, more than one hop (a chain, not a single hop),
	// or the target does not exist. Only ever set when PathRef is missing.
	RenameHop string

	// Behaviour probe. CommandProbed is true when facts.go actually ran the row's command.
	// CommandRan distinguishes "the command executed and produced a result" (a real
	// behaviour signal) from "the command could not run at all" (e.g. exit 127 / a missing
	// file — a stale-path signal, not a behaviour change). RCDiffers is true when a probed,
	// runnable command's result no longer matches what the row pinned.
	CommandProbed bool
	CommandRan    bool
	RCDiffers     bool

	// Count-drift facts (SafeCount). CountShaped is true when the Expect asserts a count
	// whose current value differs; OnlyAdditions is true when the command's target files
	// changed only by additions since the Evidence date.
	//
	// SCAFFOLDING: gatherRowFacts (facts.go) does not yet SET either field, so in the shipped
	// verb both are always false and safe:count never fires — only the fixture-fed unit tests
	// exercise this path. Wiring them is the tracked follow-up (see the SafeCount const).
	CountShaped   bool
	OnlyAdditions bool

	// RetiredIdiom names a tool idiom the row pins that a recorded ruling retired (e.g. the
	// `--consumers` form), or "" when the row pins no such idiom.
	//
	// SCAFFOLDING: gatherRowFacts does not yet set it either, so safe:idiom is likewise
	// unreachable in the shipped verb pending the same follow-up.
	RetiredIdiom string
}

// Classification is the verb's decision about one row: the verdict and a one-line reason
// suitable for the dry-run plan, the PR body, or the refusal comment on the filed issue.
type Classification struct {
	Verdict Verdict
	Reason  string
}

// Classify decides one failing row against the safe and refusal sets. It fails CLOSED: the
// default is RefusedUnclassified, and a row reaches a safe:* verdict only by positively
// satisfying that class's proof. The order encodes the precedence the brief's facts set out:
//
//  1. A risk-bearing row is refused regardless of shape — the absolute gate.
//  2. A stale PINNED PATH is decided next: a single rename hop is safe:rename, no hop is
//     refused:gone. This precedes the behaviour probe because a command that fails only
//     because its pinned path moved must not be read as a behaviour change.
//  3. A command that still RUNS and returns a different result is refused:behaviour-changed
//     — a real regression, not a stale oracle.
//  4. A retired tool idiom is safe:idiom; a pure-additions count drift is safe:count.
//  5. Anything left is refused:unclassified — unproven, so never re-baselined.
func Classify(f RowFacts) Classification {
	// 1. Risk gate — absolute. Brief 05 facts: "any row tagged risk-bearing" is refused,
	// "regardless of shape".
	if f.RiskBearing {
		reason := "owning brief is risk-bearing"
		if f.RiskReason != "" {
			reason = f.RiskReason
		}
		return Classification{RefusedRiskBearing, reason + " — a risk-bearing row is never re-baselined by the verb; file it"}
	}

	// 2. Stale pinned path. Decided before the behaviour probe so a command that only fails
	// because its target moved is not misread as a behaviour change.
	if f.PathRef != "" && !f.PathExists {
		if f.RenameHop != "" {
			return Classification{SafeRename, "pinned path " + f.PathRef + " moved by a single rename hop to " + f.RenameHop + " — work intact, path stale"}
		}
		return Classification{RefusedGone, "pinned deliverable " + f.PathRef + " is gone with no single rename hop — cannot prove the work survived; file it"}
	}

	// 3. Behaviour change. A command that still runs and returns a different result is a
	// real regression, not a stale oracle. Only a probed, runnable command counts.
	if f.CommandProbed && f.CommandRan && f.RCDiffers {
		return Classification{RefusedBehaviourChanged, "command still runs and returns a different result than the row pinned — a real behaviour change; file it"}
	}

	// 4. Retired idiom, then pure-additions count drift.
	if f.RetiredIdiom != "" {
		return Classification{SafeIdiom, "row pins tool idiom " + f.RetiredIdiom + " retired by a recorded ruling — idiom stale, behaviour intact"}
	}
	if f.CountShaped && f.OnlyAdditions {
		return Classification{SafeCount, "count Expect differs and the target files changed only by additions since the Evidence date — growth, not a regression"}
	}

	// 5. Fail closed.
	return Classification{RefusedUnclassified, "the facts do not positively place this row in any safe class — unproven is never re-baselined; file it"}
}
