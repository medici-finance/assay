package main

import "github.com/medici-finance/assay/tools/desk/internal/loopengine"

// TierPolicy FAILS SAFE on risk (maintainer ruling 2026-09-06). Precedence:
//
//  1. irreversible: yes  -> TierHuman, FIRST, before any branch the dormant reversible-risk
//     flag can divert. Irreversible is the most serious risk answer, so it is routed to the
//     human — never dispatched by the model path — whatever the brief's gate value says.
//     The Evidence-only lane the ruling preserves lives in Land, not here: on an irreversible
//     PASS Land still writes Evidence with NO status flip and opens a checkpoint PR for a
//     human (statusgen's brieffile.go human-gate stays authoritative). A model may gather
//     Evidence; it may never flip.
//  2. risk-flagged but REVERSIBLE (gate: human OR any OTHER risk answer yes)  -> TierSession
//     when the dormant middle-rung flag is on, else TierHuman. Not dispatched to a model at
//     TierHuman: routed to the checkpoint-PR / labeled-issue path, drain continues.
//  3. otherwise (risk-clear)  -> TierLocal. The normal path, the majority of the queue.
//
// The precedence used to put irreversible ahead of the risk branch but route it to TierLocal
// (dispatched-for-evidence). That was a fail-OPEN: an irreversible brief whose gate was `model`
// was handed a dispatchable tier and, keyed only on the computed tier, read as dispatchable in
// the plan. The fix is one-way — it can only move an item OUT of the dispatchable set.
//
// When a model does run (TierLocal / TierSession) it is ALWAYS the local session model — never
// opus, never an external/paid tier. The adapter maps those onto the local model; there is no
// tier that maps onto a bigger model.
func (v *VerifyLoop) TierPolicy(it loopengine.Item) (loopengine.Tier, error) {
	if it.Risk.Irreversible {
		// FAIL SAFE: irreversible is human-only, never dispatched — and ahead of the
		// reversible-risk branch so the dormant flag below can never divert it.
		return loopengine.TierHuman, nil
	}
	if it.Risk.Flagged(it.Gate) {
		// MIDDLE RUNG (arch doc §9.2) — the owner's OPEN decision, left OFF.
		// Flipping v.F16ReversibleRiskToSession to true is the ENTIRE change needed to
		// restore the middle rung: session-tier verification for risk-flagged
		// -but-REVERSIBLE briefs, reserving the human for irreversible only (which the arm
		// above already routes to the human regardless of this flag). It is one line and
		// deliberately dormant — enabling it loosens the cost rule and widens the
		// model-sign-off surface on risk-flagged work, which is the owner's trade to make, not
		// the engine's.
		if v.F16ReversibleRiskToSession {
			return loopengine.TierSession, nil
		}
		return loopengine.TierHuman, nil
	}
	return loopengine.TierLocal, nil
}

// reachableTiers is the set of DISPATCHABLE tiers this loop's TierPolicy can actually emit —
// the set the runner table is validated against at boot (an unconfigured
// reachable tier is a startup error, not a dispatch-time surprise). It mirrors TierPolicy's
// routing exactly: TierLocal is always reachable; TierSession is reachable ONLY when the
// middle-rung flag is enabled. TierCheap is never emitted by verify-desk, and TierHuman is
// non-dispatchable — both are excluded.
func (v *VerifyLoop) reachableTiers() []loopengine.Tier {
	tiers := []loopengine.Tier{loopengine.TierLocal}
	if v.F16ReversibleRiskToSession {
		tiers = append(tiers, loopengine.TierSession)
	}
	return tiers
}
