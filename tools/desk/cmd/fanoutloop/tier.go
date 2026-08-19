package main

import (
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// TierPolicy encodes worker-desk's CURRENT dispatch rule: tier by EFFORT, overridden by EXEC-TIER
// (SKILL.md § "tier by effort, overridden by exec-tier"). It is deliberately a
// DIFFERENT shape from verify-desk's — verify always routes to TierLocal or a human; batch routes
// across TierCheap and TierSession and never emits TierLocal — which is exactly what makes it the
// contract-validating second consumer: the same Tier enum, a genuinely different policy.
//
// Precedence:
//
//  1. exec-tier: strong  -> TierSession (session-tier ONLY, never an economy tier). The dispatch
//     prompt additionally carries the cheap-pickup STOP text (see renderDispatchPrompt), so a
//     fast/cheap model that picks the work up hands it back.
//  2. orphan resume       -> TierCheap. A resume is bounded rework against a PR's stated findings,
//     not a fresh effort-sized implementation, so it takes the economy tier by default. (A resume of
//     an `exec:strong` brief is already caught by rule 1 when its payload carries the marker.)
//  3. effort S            -> TierSession (may run at the session tier).
//  4. effort M / L        -> TierCheap (economy tier, behind the verify/review gates — executing M/L
//     inline at a strong tier is the SDD cost leak the system exists to avoid).
//  5. effort unspecified  -> TierCheap (never silently promote unknown work to a strong tier).
//
// gate:human and risk-flagged briefs DISPATCH NORMALLY — there is NO TierHuman branch here. The
// human gate binds APPROVAL (review, merge, verify sign-off), not implementation: a worker in a
// worktree behind a draft PR is inert, and every human control point sits downstream. The prompt
// tells such a worker to stop at `implemented` and report BLOCKED-ON-IAN at any documented cutover
// step (see renderDispatchPrompt). This is the exact inverse of verify-desk, whose risk-flagged
// work routes to TierHuman — and the divergence is the contract-generalization evidence.
func (f *FanoutLoop) TierPolicy(it loopengine.Item) (loopengine.Tier, error) {
	if strings.EqualFold(strings.TrimSpace(it.ExecTier), "strong") {
		return loopengine.TierSession, nil
	}
	if it.Payload["kind"] == kindOrphan {
		return loopengine.TierCheap, nil
	}
	switch strings.ToUpper(strings.TrimSpace(it.Effort)) {
	case "S":
		return loopengine.TierSession, nil
	default: // M, L, or unspecified — economy tier
		return loopengine.TierCheap, nil
	}
}

// reachableTiers is the set of DISPATCHABLE tiers TierPolicy can actually emit — the set a
// runner table would be validated against at boot. Batch reaches TierCheap and
// TierSession; it never emits TierLocal (verify-desk's tier) and never TierHuman (non-dispatchable,
// and batch has no human-route branch). This differing set is itself part of the contract
// validation: the tier→runner surface generalizes to a consumer whose reachable set is disjoint
// from verify-desk's on TierLocal.
func (f *FanoutLoop) reachableTiers() []loopengine.Tier {
	return []loopengine.Tier{loopengine.TierCheap, loopengine.TierSession}
}
