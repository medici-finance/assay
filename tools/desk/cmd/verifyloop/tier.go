package main

import "github.com/medici-finance/assay/tools/desk/internal/loopengine"

// TierPolicy encodes verify-desk's CURRENT rules, unchanged (arch doc §6 irreducibles).
// Precedence:
//
//  1. irreversible: yes  -> TierLocal. The Verify table IS run by a dispatched local-model
//     verifier so the Evidence is real and recorded; Land then writes Evidence with NO
//     status flip and opens a checkpoint PR for a human (statusgen's brieffile.go human-gate
//     stays authoritative). Dispatched, not human-only, because the mechanical verification
//     is worth recording.
//  2. risk-flagged (gate: human OR any OTHER risk answer yes)  -> TierHuman. Not dispatched:
//     routed to the checkpoint-PR / labeled-issue path, drain continues. A model may not
//     sign off a risk-flagged brief (a maintainer's ruling: verify dispatches the local model only,
//     never opus/external; the risk-keyed floor's upper rung is a HUMAN, full stop).
//  3. otherwise (risk-clear)  -> TierLocal. The normal path, the majority of the queue.
//
// The tier is ALWAYS the local session model when a model runs at all — never opus, never an
// external/paid tier. The adapter maps TierLocal onto the local model; there is no tier that
// maps onto a bigger model.
func (v *VerifyLoop) TierPolicy(it loopengine.Item) (loopengine.Tier, error) {
	if it.Risk.Irreversible {
		return loopengine.TierLocal, nil // dispatched for Evidence; Land does no-flip + checkpoint PR
	}
	if it.Risk.Flagged(it.Gate) {
		// MIDDLE RUNG (arch doc §9.2) — the owner's OPEN decision, left OFF.
		// Flipping v.F16ReversibleRiskToSession to true is the ENTIRE change needed to
		// restore the middle rung: session-tier verification for risk-flagged
		// -but-REVERSIBLE briefs, reserving the human for irreversible only. It is one line
		// and deliberately dormant — enabling it loosens the cost rule and widens
		// the model-sign-off surface on risk-flagged work, which is the owner's trade to make, not
		// the engine's.
		if v.F16ReversibleRiskToSession {
			return loopengine.TierSession, nil
		}
		return loopengine.TierHuman, nil
	}
	return loopengine.TierLocal, nil
}
