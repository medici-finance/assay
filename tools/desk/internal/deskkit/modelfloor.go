package deskkit

// modelfloor.go — the model-capability floor on AUTHORITY-BEARING writes.
//
// WHAT THIS IS. A ready-flip and a review verdict are authority-bearing: they move a PR out
// of the review lane and record a correctness judgement. The fleet's convention has been to
// dispatch such work to a strong-tier session and cheap implementers to everything else, but
// nothing REFUSED an authority-bearing write from a below-tier session — a session that
// self-selected or was mis-dispatched cheap succeeded identically to a strong one. This file
// is the write-side backstop: it turns that convention into a guard that FAILS CLOSED at the
// moment of the write, keyed on a DIFFERENT signal than the dispatch-side default — the
// dispatcher's ATTESTED tier stamp (modelstamp.go), not the session's self-report.
//
// WHY IT READS THE ATTESTATION, NOT A SELF-REPORT. The decision consumes the applier-aware
// reader (AttestedModelStampOf): a dispatched-tier label counts only when the DISPATCHER
// applied it. A stamp a session could apply to itself is the honor-system with extra steps,
// so a self-applied or non-dispatcher stamp reads Indeterminate and does NOT clear the floor.
// This raises the bar from self-report to dispatcher attestation; it does not close custody
// questions (a session that swapped models mid-run is outside what any dispatch stamp sees).
//
// THE FOUR OUTCOMES, and which way each fails:
//
//	attested at/above the floor   -> proceed (FloorAllow).
//	attested BELOW the floor      -> REFUSE with remediation (FloorRefuse). The talk-33 rule:
//	                                 say why AND what instead — escalate to a strong-tier
//	                                 session; delegation downward is fine.
//	NO attestation at all         -> proceed with a NOTICE (FloorNoticeAllow). The floor
//	                                 targets attested below-tier sessions; a human-driven
//	                                 session and a pre-attestation dispatch carry no stamp and
//	                                 must not be bricked. Absent is NOT the same as below-tier.
//	attestation PRESENT-BUT-       -> REFUSE (FloorRefuse). A conflicting, incomplete, or
//	  UNREADABLE (Indeterminate)      non-dispatcher-applied stamp cannot PROVE a strong tier,
//	                                  and the floor fails closed: an unprovable tier is refused,
//	                                  not admitted. This is distinct from "absent" — someone
//	                                  applied a stamp this verb cannot trust, which is exactly
//	                                  the forged-self-report case the floor exists to stop.
//
// THE OVERRIDE, and why it is loud. Incident recovery needs a way past the floor; a floor
// with no escape can brick the review lane when the attestation pipeline itself is broken.
// So an explicit env override exists — but a SILENT override would nullify the layer, so the
// bypass is always logged with a stable, grep-able marker (ModelFloorOverrideMarker). The
// override is an ENV toggle, deliberately not a CLI flag: a flag on the write verb is a gate
// a routine caller waves past, which is the property these verbs are built NOT to have.
//
// ONE HOME FOR THE DECISION. Both verdict/flip verbs call ModelCapabilityFloor, so the floor
// semantics and the remediation wording live here once rather than drifting between two verbs.
// Each verb owns only its own forge read of the target PR's label EVENTS (the applier is what
// AttestedModelStampOf needs) and the transport of the decision's Message to its output.

import (
	"fmt"
	"os"
	"strings"
)

// ModelFloorTier is the strength tier an authority-bearing write requires. The tier
// vocabulary is DispatchTiers() = {any, strong} and "strong" is its top rung, so the floor
// is "strong": an attested "any"-tier dispatch is BELOW it. It is declared against that one
// vocabulary rather than as a fresh literal, so a vocabulary change is caught here too.
const ModelFloorTier = "strong"

// ModelFloorOverrideEnv, when set to a non-empty value, bypasses the floor for incident
// recovery. The bypass is never silent (see ModelFloorOverrideMarker). It is an env toggle,
// not a CLI flag, on purpose: an authority-bearing write verb must not advertise a
// wave-past-me flag in its own synopsis.
const ModelFloorOverrideEnv = "DESK_MODEL_FLOOR_OVERRIDE"

// ModelFloorOverrideMarker is the stable token the loud override line carries, so an
// operator — and a test — can positively confirm a bypass happened rather than infer it.
const ModelFloorOverrideMarker = "MODEL-FLOOR-OVERRIDE"

// FloorOutcome is the decision the floor reaches for one authority-bearing write.
type FloorOutcome int

const (
	// FloorRefuse is the ZERO VALUE and means the write is refused — the attested tier is
	// below the floor, or the attestation is present but unreadable. The zero value refuses
	// so a consumer that forgets to handle a state fails closed rather than open.
	FloorRefuse FloorOutcome = iota
	// FloorAllow means the dispatch was attested at or above the floor: proceed.
	FloorAllow
	// FloorNoticeAllow means NO attestation was found: proceed, but the verb must SAY so —
	// absent is not the same as cleared.
	FloorNoticeAllow
	// FloorOverrideAllow means the incident-recovery override was engaged: proceed, LOUDLY.
	FloorOverrideAllow
)

func (o FloorOutcome) String() string {
	switch o {
	case FloorAllow:
		return "allow"
	case FloorNoticeAllow:
		return "notice-allow"
	case FloorOverrideAllow:
		return "override-allow"
	default:
		return "refuse"
	}
}

// Proceeds reports whether the outcome lets the write happen. A consumer writes
// `if !d.Outcome.Proceeds()` and refuses, rather than enumerating the allow states and
// forgetting one.
func (o FloorOutcome) Proceeds() bool { return o != FloorRefuse }

// FloorDecision is the whole answer: the outcome, the attested state and stamp it was
// reached from (for the verb's report), and the Message the verb prints — remediation on a
// refuse, the NOTICE line on notice, the loud override line on override, an OK detail on a
// clean allow.
type FloorDecision struct {
	Outcome FloorOutcome
	State   ModelState
	Stamp   ModelStamp
	Message string
}

// tierRank orders the tier vocabulary so the floor is a comparison rather than a special
// case for each value. An unknown tier ranks below everything (-1), so it can never satisfy
// a floor — the fail-closed direction.
func tierRank(tier string) int {
	switch strings.ToLower(strings.TrimSpace(tier)) {
	case "strong":
		return 1
	case "any":
		return 0
	default:
		return -1
	}
}

// tierMeetsFloor reports whether an attested tier is at or above ModelFloorTier.
func tierMeetsFloor(tier string) bool {
	return tierRank(tier) >= tierRank(ModelFloorTier)
}

// ModelFloorOverrideEngaged reports whether the incident-recovery override env is set to a
// non-empty value. Both verbs read it through this one helper so "engaged" has one meaning.
func ModelFloorOverrideEngaged() bool {
	return strings.TrimSpace(os.Getenv(ModelFloorOverrideEnv)) != ""
}

// ModelCapabilityFloor decides whether an authority-bearing write may proceed, given the
// target PR's label EVENTS (name + applying login), the dispatcher predicate, and whether
// the override is engaged. It is the ONE decision both verdict/flip verbs share.
//
// isDispatcher is the applier-aware predicate (inject IsDispatcherLogin against the live
// roster, or a test stub). A nil predicate vouches for no one, so any dispatched-* label
// then reads Indeterminate and the floor refuses — an unconfigured deployment fails closed.
func ModelCapabilityFloor(tl StampTimeline, isDispatcher func(applier string) bool, override bool) FloorDecision {
	stamp, state := AttestedModelStampOf(tl, isDispatcher)

	if override {
		return FloorDecision{
			Outcome: FloorOverrideAllow,
			State:   state,
			Stamp:   stamp,
			Message: fmt.Sprintf(
				"%s: the model-capability floor was BYPASSED via %s for incident recovery "+
					"(attested state: %s; tier: %s). This override is logged loudly and is not a routine "+
					"path — a normal authority-bearing write must clear the floor, not override it.",
				ModelFloorOverrideMarker, ModelFloorOverrideEnv, state, tierOrNone(stamp.Tier)),
		}
	}

	switch state {
	case ModelStamped:
		if tierMeetsFloor(stamp.Tier) {
			return FloorDecision{
				Outcome: FloorAllow,
				State:   state,
				Stamp:   stamp,
				Message: fmt.Sprintf(
					"model-capability floor: OK — the dispatch attested for this PR is tier %q (model %q), "+
						"at or above the %s floor.", stamp.Tier, stamp.Model, ModelFloorTier),
			}
		}
		return FloorDecision{
			Outcome: FloorRefuse,
			State:   state,
			Stamp:   stamp,
			Message: fmt.Sprintf(
				"model-capability floor: this authority-bearing write requires a %s-tier session, but the "+
					"dispatch attested for this PR is tier %q — BELOW the floor. Escalate to a strong-tier "+
					"session to perform this write; delegation downward is fine, escalating an "+
					"authority-bearing write upward is not. (Incident-recovery override: set %s=1; it is "+
					"logged loudly.)",
				ModelFloorTier, stamp.Tier, ModelFloorOverrideEnv),
		}
	case ModelUnknown:
		return FloorDecision{
			Outcome: FloorNoticeAllow,
			State:   state,
			Stamp:   stamp,
			Message: "model-capability floor: NOTICE — no dispatch attestation on this PR (a human-driven " +
				"session or a pre-attestation dispatch). The floor targets attested below-tier sessions and " +
				"does not brick unattested lanes, so this write proceeds.",
		}
	default: // ModelIndeterminate
		return FloorDecision{
			Outcome: FloorRefuse,
			State:   state,
			Stamp:   stamp,
			Message: unreadableStampMessage(tl, isDispatcher),
		}
	}
}

// unreadableStampMessage writes the present-but-UNREADABLE refusal, naming the CAUSE it
// actually found rather than listing every cause it might have found.
//
// WHY THE CAUSE, NOT THE MENU. The two causes have DIFFERENT remedies. An untrusted
// applier is fixed on the PR (re-stamp it from the dispatcher); unreadable CONTENT is
// fixed by correcting the labels themselves. A refusal that reads "conflicting, incomplete,
// or applied by a non-dispatcher identity" makes the operator diagnose which one from the
// timeline API by hand — which is what a field report of this floor had to do before it
// could tell a genuinely mis-stamped PR from a correctly dispatched one. So the applier
// case NAMES the login that applied the stamp and the identity the floor would have
// accepted, and the content case names which half is wrong.
func unreadableStampMessage(tl StampTimeline, isDispatcher func(applier string) bool) string {
	var cause string
	if untrusted := NonDispatcherStampAppliers(tl, isDispatcher); len(untrusted) > 0 {
		cause = fmt.Sprintf(
			"The dispatched-* labels this PR currently carries were applied by %s, and this floor accepts a "+
				"stamp only from the bound dispatcher identity %s (roster role %q). Re-stamp the PR from the "+
				"dispatcher — the dispatch verb REMOVES a foreign stamp and re-applies it under that App, "+
				"which is the only repair an append-only timeline allows — or escalate this write "+
				"to a strong-tier session.",
			StripControl(strings.Join(untrusted, ", ")), RoleAppLoginOrEmpty(DispatcherRole), DispatcherRole)
	} else if unattributed := UnattributedStampLabels(tl); len(unattributed) > 0 {
		// A DIFFERENT remedy again: the stamp may be perfectly good and the timeline read
		// short. Sending this operator to re-stamp a correct PR is the wrong next move, so
		// the message names the labels and the read rather than the appliers.
		cause = fmt.Sprintf(
			"This PR carries %s, but the label timeline read contains no standing `labeled` event for %s — "+
				"so who applied the stamp could not be established. That is could-not-check, not a cleared "+
				"floor and not an unstamped PR. Re-run this write against a complete timeline read; if the "+
				"labels really have no applying event, re-stamp the PR from the dispatcher.",
			StripControl(strings.Join(unattributed, ", ")),
			plural(len(unattributed), "it", "them"))
	} else {
		cause = stampContentCause(tl) +
			" Re-stamp the PR with exactly one dispatched-model label and one dispatched-tier label, or " +
			"escalate this write to a strong-tier session."
	}
	return fmt.Sprintf(
		"model-capability floor: the dispatch attestation on this PR is present but UNREADABLE, so it "+
			"cannot PROVE a %s tier and does not clear the floor — a stamp anyone could self-apply is not "+
			"attestation. %s (Incident-recovery override: set %s=1; it is logged loudly.)",
		ModelFloorTier, cause, ModelFloorOverrideEnv)
}

// stampContentCause names which way the stamp's CONTENT failed to resolve, for the branch
// where every applier was the dispatcher. It reports the INCOMPLETE case separately
// because that one is a transport failure between the two label writes rather than a
// malformed input, and the operator's next look is different.
func stampContentCause(tl StampTimeline) string {
	models, tiers := 0, 0
	for _, l := range tl.Present {
		name := normLabel(l)
		switch {
		case strings.HasPrefix(name, DispatchedModelPrefix):
			models++
		case strings.HasPrefix(name, DispatchedTierPrefix):
			tiers++
		}
	}
	switch {
	case models == 0:
		return "The stamp is incomplete: a dispatched-tier label is present with no dispatched-model half."
	case tiers == 0:
		return "The stamp is incomplete: a dispatched-model label is present with no dispatched-tier half."
	default:
		return "The stamp's content does not resolve to one (model, tier): the labels are conflicting, " +
			"empty, or name a tier outside the vocabulary (" + strings.Join(DispatchTiers(), " | ") + ")."
	}
}

// plural picks between two words for a count, so a refusal reads as a sentence rather than
// as "label(s)".
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// tierOrNone renders a tier for the override line, naming the empty tier explicitly rather
// than printing a bare "" that reads as a formatting bug.
func tierOrNone(tier string) string {
	if strings.TrimSpace(tier) == "" {
		return "none"
	}
	return tier
}
