package main

import (
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// queueclass.go — QUEUE TRUTHFULNESS for `verifyloop plan`.
//
// `plan` used to emit EVERY implemented brief as a DISPATCH candidate. Some implemented briefs
// cannot convert on an offline verifier run, so dispatching them only reproduces the identical
// non-verdict every pass and burns a dispatchable slot. This file classifies each queue item
// into ONE disposition; only dispDispatch reaches the dispatchable list, and the rest are
// surfaced honestly (a deferred section, or a counted bucket with a one-line "why it waits").
//
// The signals are the ones ALREADY present in the brief/board — no parallel taxonomy:
//   - the human gate is the engine's own predicate (RiskFlags.Flagged via TierPolicy → TierHuman),
//   - the three markers (blocked-until / verify-lane / in-repair) are frontmatter fields parsed by
//     parseFrontmatter and carried on Item.Payload by scanAwaiting.
type disposition int

const (
	// dispDispatch: genuinely actionable this run — offline-convertible, risk-clear, unblocked.
	// This is the ONLY disposition that reaches the dispatchable list. It is also the default:
	// a brief with none of the markers below and no human gate stays a DISPATCH candidate,
	// so the change is backward-compatible.
	dispDispatch disposition = iota
	// dispDeferred: a `blocked-until:` marker — the brief's Verify exit criteria depend on a
	// condition/window that has not been met (a shadow/accrual window). Re-verifying it now only
	// reproduces the same "not met" result, so it is deferred out of the dispatchable list until
	// the condition can be met, printed in the deferred section with its reason.
	dispDeferred
	// dispAwaitingHuman: gated on a human sign-off (gate: human, or a risk answer yes) — the
	// exact set TierPolicy routes to TierHuman. A model may not flip it, so it is not dispatchable.
	dispAwaitingHuman
	// dispAwaitingOnlineLane: the Verify substrate is a live cluster / online / live session,
	// not this repo's offline tree — an offline verifier run cannot produce the verdict.
	dispAwaitingOnlineLane
	// dispInRepair: an in-flight table-repair pipeline already owns this brief's Verify table
	// (a stale-artifact re-baseline). Dispatching a verifier at it races the repair.
	dispInRepair
)

// String is the stable bucket slug used in the plan output and tests.
func (d disposition) String() string {
	switch d {
	case dispDispatch:
		return "dispatch"
	case dispDeferred:
		return "deferred"
	case dispAwaitingHuman:
		return "awaiting-human"
	case dispAwaitingOnlineLane:
		return "awaiting-online-lane"
	case dispInRepair:
		return "in-repair"
	default:
		return "unknown"
	}
}

// whyItWaits is the one-line explanation printed once per non-dispatch disposition.
func (d disposition) whyItWaits() string {
	switch d {
	case dispDeferred:
		return "blocked until a stated condition/window is met — re-verifying now only reproduces the same non-verdict"
	case dispAwaitingHuman:
		return "gated on human sign-off (gate:human or a risk answer) — a model may not flip it"
	case dispAwaitingOnlineLane:
		return "needs a cluster / online / live-session hand-off — an offline verifier run cannot produce the verdict"
	case dispInRepair:
		return "an in-flight table-repair pipeline already owns this Verify table — leave it to the repair"
	default:
		return ""
	}
}

// onlineLaneValues is the set of `verify-lane:` values that mean "not this repo's offline tree".
// A value outside this set (or empty) is not an online lane and does not bucket the brief.
var onlineLaneValues = map[string]bool{
	"online":       true,
	"live":         true,
	"live-session": true,
	"cluster":      true,
	"session":      true,
}

// notInRepairValues are the falsey `in-repair:` values that DO NOT bucket a brief, so an author
// can carry the field explicitly set to "no" without stranding the brief out of the queue.
var notInRepairValues = map[string]bool{
	"no":    true,
	"false": true,
	"none":  true,
	"":      true,
}

// classifyItem places one queue item in exactly one disposition. It takes the tier the loop's
// TierPolicy already computed (so the human gate is the engine's own routing, reused, not a
// second copy of the predicate) plus the markers scanAwaiting carried onto Item.Payload.
//
// The returned reason is the human-facing detail for that item's line: the blocked-until
// condition, the lane name, or the repair pipeline reference. It is empty for dispDispatch and
// for dispAwaitingHuman (whose why is the same for every member — carried by whyItWaits).
//
// Precedence, most-specific first:
//  1. blocked-until — the brief cannot even be attempted this run, whatever else is true of it.
//  2. human gate (tier == TierHuman) — preserves the pre-change routing exactly: these items
//     were never dispatched (they printed as ROUTE-HUMAN), they are now the awaiting-human bucket.
//  3. in-repair — a pipeline owns the table; do not race it.
//  4. online lane — no offline verdict is possible.
//  5. otherwise DISPATCH.
func classifyItem(it loopengine.Item, tier loopengine.Tier) (disposition, string) {
	if bu := payloadValue(it, "blocked_until"); bu != "" {
		return dispDeferred, bu
	}
	if tier == loopengine.TierHuman {
		return dispAwaitingHuman, ""
	}
	if ir := payloadValue(it, "in_repair"); !notInRepairValues[strings.ToLower(ir)] {
		return dispInRepair, ir
	}
	if lane := strings.ToLower(payloadValue(it, "verify_lane")); onlineLaneValues[lane] {
		return dispAwaitingOnlineLane, lane
	}
	return dispDispatch, ""
}

// payloadValue is a nil-safe trimmed read of one Item.Payload key.
func payloadValue(it loopengine.Item, key string) string {
	if it.Payload == nil {
		return ""
	}
	return strings.TrimSpace(it.Payload[key])
}
