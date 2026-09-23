package deskkit

// repairadmission.go — the reservation DECISION, made an EFFECT (example-stream/18).
//
// THE DEFECT THIS CLOSES. example-stream/05 gave worker-desk a per-class concurrency
// RESERVATION (a floor of slots held for resume/rework so a full pool of fresh briefs cannot
// crowd them out) and example-stream/17 made a failed verification a durable worker REPAIR
// obligation in the rework class. But the reservation was only ever PRINTED: the planner's
// `classes: … (fresh capped at k by reservation)` line is advice a caller can ignore, and a
// busy dispatcher that ignored it filled every reserved slot with fresh work while a repair
// waited. This file turns the printed advice into a decision the dispatch boundary makes.
//
// WHAT THIS FILE IS. The PURE evaluator (EvaluateAdmission) plus the opt-in switch and the
// recorded policy version. It performs NO I/O and reads NO clock: every input is read from an
// authoritative source by the caller (cmd/deskdispatch) BEFORE the gate runs, so the decision
// is identical on every host and is unit-testable without a forge. The SERIALIZATION that makes
// the count→decide→claim window atomic across dispatchers, and the wiring to the live claim
// backend and the repair-obligation source, live in the caller — this file is only the rule.
//
// WHAT IT IS NOT. It is not a new width, not a new default reservation, and not a scheduler.
// The reservation values, their expiry and the pool ceiling stay exactly where width.go /
// widthstore.go hold them; this reads them. The safe direction to be wrong here is to HOLD a
// fresh dispatch that could have run (it costs a tick of throughput and says why), never to
// admit one that steals a reserved repair slot (which is the failure the reservation exists to
// prevent).

import (
	"fmt"
	"os"
	"strings"
)

// EnvRepairAdmission opts the dispatch-boundary reservation ENFORCEMENT in. It ships OFF: unset
// (or any value but the on-token) leaves the behaviour example-stream/05 shipped unchanged —
// the planner's reservation line stays ADVISORY and no dispatch is ever held. This is the
// rollback control named in the brief's ground rules: clear it (or pin the prior binary) and
// the gate is inert, with no state to unwind. There is no companion default-on width or hidden
// global that could enable it for a deployment that did not set this.
const EnvRepairAdmission = "ASSAY_REPAIR_ADMISSION"

// RepairAdmissionPolicyVersion is the recorded version of the enforcement CONTRACT an adopter
// validates a baseline against before activating it (Task 2: "a recorded policy/version so an
// adopter can validate its baseline before activation"). It is bumped when the admission
// decision's INPUTS or SEMANTICS change, never for an unrelated edit, so a deployment can pin
// "I validated v1" and a later behavioural change is detectable rather than silent.
const RepairAdmissionPolicyVersion = "repair-admission-v1"

// repairAdmissionOn is the SINGLE token that turns enforcement on. Anything else — unset, empty,
// "off", "0", "true", a typo — is OFF, because the safe reading of an unrecognised value is the
// shipped advisory behaviour, never an enforcement an operator did not deliberately ask for.
const repairAdmissionOn = "on"

// RepairAdmissionEnabled reports whether the dispatch-boundary enforcement is active, and the
// policy version to record either way. It reads the env opt-in ONLY.
func RepairAdmissionEnabled() (on bool, version string) {
	return strings.TrimSpace(os.Getenv(EnvRepairAdmission)) == repairAdmissionOn, RepairAdmissionPolicyVersion
}

// AdmissionClass is the reservation class a dispatch candidate resolves to. Its values are the
// same strings KnownReserveClasses and the planner's classOf use ("resume"/"rework"/"fresh"), so
// a reserve map keyed by KnownReserveClasses indexes directly by class. It is RESOLVED from the
// authoritative work source (the repair-obligation sidecar, the orphan-resume source), never
// from a caller-supplied label — a caller cannot relabel fresh work as a repair to jump the
// floor (interface contract; Verify row 3, "forged repair class is refused").
type AdmissionClass string

const (
	// AdmissionFresh is an ordinary Next-up brief — the class the floor holds slots AGAINST.
	AdmissionFresh AdmissionClass = "fresh"
	// AdmissionResume is an orphan-PR resume (example-stream/05's reserved class).
	AdmissionResume AdmissionClass = "resume"
	// AdmissionRework is a repair obligation (example-stream/17) — the reserved class this
	// brief exists to protect.
	AdmissionRework AdmissionClass = "rework"
)

// reservedAdmissionClass reports whether c is a class the reservation floor protects
// (resume/rework), i.e. NOT fresh. A reserved-class candidate is never held by the floor: it is
// the work the floor exists to keep slots for.
func reservedAdmissionClass(c AdmissionClass) bool {
	return c == AdmissionResume || c == AdmissionRework
}

// AdmissionVerdict is the gate's decision.
type AdmissionVerdict int

const (
	// AdmissionAdmit — the candidate may consume a slot now.
	AdmissionAdmit AdmissionVerdict = iota
	// AdmissionHold — admitting the candidate now would steal a reserved slot (or the pool is
	// full); the caller refuses the dispatch and names the waiting reserved work.
	AdmissionHold
)

// AdmissionInputs is everything EvaluateAdmission judges. Each field is read from an
// authoritative source by the caller BEFORE the gate runs; the evaluator is a pure function of
// them, with no I/O and no clock.
type AdmissionInputs struct {
	// Class is the candidate's RESOLVED class (from the work source, not a caller flag).
	Class AdmissionClass
	// Width is the loop's effective pool width (ResolvedWidth, already ceiling-clamped).
	Width int
	// Reserve is the per-class reservation floor (ResolvedReserve), keyed by KnownReserveClasses.
	Reserve map[string]int
	// Occupancy is the count of LIVE admitted claims in the scheduling scope — the slots already
	// taken. It is DERIVED from the live dispatch-claim set, never assumed; an unreadable
	// occupancy is a could-not-check the caller raises before ever reaching here, so this field
	// is only ever a value that was actually observed.
	Occupancy int
	// RunnableDemand is the count of RUNNABLE (assignable-now) items per reserved class, read
	// from the authoritative source. A waiting-external repair contributes 0 here — it is not
	// runnable — which is exactly why an external hold never reserves a slot (Verify row 3,
	// "externally blocked repairs do not idle usable slots").
	RunnableDemand map[string]int
	// WaitingItem names the top runnable reserved item a fresh HOLD is protecting, for the
	// refusal message. "" when nothing reserved is waiting.
	WaitingItem string
}

// EvaluateAdmission decides whether the candidate may consume a slot now, and returns a one-line
// reason either way. It is the ONE place the reservation floor becomes an EFFECT rather than a
// printed advisory.
//
// The rule:
//   - Pool full (no free slot) → HOLD, whatever the class. A slot has to free first.
//   - A reserved-class candidate (resume/rework) → ADMIT on any free slot: it is the work the
//     floor protects, so it fills its own reservation and is never held by it.
//   - A fresh candidate → HOLD when the free slots are at or below the floor held for a reserved
//     class that has RUNNABLE demand right now; ADMIT otherwise. The floor sums the reservation
//     ONLY over reserved classes with runnable demand, so a reservation never idles a slot when
//     nothing reserved is waiting (example-stream/05's strict rule, now enforced not printed).
func EvaluateAdmission(in AdmissionInputs) (AdmissionVerdict, string) {
	free := in.Width - in.Occupancy
	if free <= 0 {
		return AdmissionHold, fmt.Sprintf(
			"pool full: %d/%d slots occupied, no free slot for %s work — a slot must free first",
			in.Occupancy, in.Width, in.Class)
	}

	if reservedAdmissionClass(in.Class) {
		return AdmissionAdmit, fmt.Sprintf(
			"%s admitted: reserved work fills its own reservation (%d/%d slots occupied)",
			in.Class, in.Occupancy, in.Width)
	}

	floor := 0
	var protected []string
	for _, c := range KnownReserveClasses {
		if in.RunnableDemand[c] > 0 && in.Reserve[c] > 0 {
			floor += in.Reserve[c]
			protected = append(protected, fmt.Sprintf("%s=%d", c, in.Reserve[c]))
		}
	}
	if free <= floor {
		waiting := strings.TrimSpace(in.WaitingItem)
		if waiting == "" {
			waiting = "a reserved-class item"
		}
		return AdmissionHold, fmt.Sprintf(
			"fresh dispatch held: %d free slot(s) at or below the reserved floor (%s) while %s is runnable — "+
				"admitting fresh work now would steal a slot reserved for it",
			free, strings.Join(protected, ","), waiting)
	}
	return AdmissionAdmit, fmt.Sprintf(
		"fresh admitted: %d free slot(s) above the reserved floor (%d)", free, floor)
}
