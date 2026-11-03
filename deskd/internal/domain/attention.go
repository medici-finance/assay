package domain

import "encoding/json"

// AttentionState is the derived, per-automation attention vocabulary
// (decision-model spec §1.2). The vocabulary is Xirp's; the SENSOR is not —
// every state is derived from an automation's own artifacts plus the clock,
// never from anything the automation asserts about itself. There is no field,
// endpoint, tool parameter, header, file, beacon, or config key by which an
// automation can set or nudge its own state; `Read`/`Acked` never feed it
// (spec §1.2, §1.1.2 hard rule). The derivation lives in internal/index and
// its input set is closed and authority-tested; this package holds only the
// vocabulary and its render, so a consumer that must NOT reach the derivation
// can still name a state.
type AttentionState string

const (
	// FailedToStart — dispatched, no first artifact within N (spec §1.3).
	// Born-dead outranks everything: the only state with no artifact to
	// notice.
	FailedToStart AttentionState = "failed-to-start"

	// Waiting — ≥ 1 open Escalation whose Automation is this automation, and
	// nothing else (spec §1.2). Outranks working because a loop files-and-
	// continues: it may be emitting artifacts AND blocked on a human. The
	// human-actionable fact wins the rollup.
	Waiting AttentionState = "waiting"

	// Working — newest artifact younger than the activity window W. F-08:
	// artifacts are agent-writable, so this means "left artifacts", not "is
	// healthy".
	Working AttentionState = "working"

	// Idle — ran, found nothing to do, left a no-op artifact saying so.
	// Reachable ONLY for automation classes whose no-op tick leaves an
	// artifact (spec §1.2 idle-honesty rule, binding); where a class leaves
	// nothing, an idle automation derives Silent, never Idle.
	Idle AttentionState = "idle"

	// Silent — none of the above is known in flight. Rendered as a question,
	// never green (spec §1.2): the board admits UNKNOWN rather than inventing
	// a healthier state it cannot honestly derive.
	Silent AttentionState = "silent"
)

// Valid reports whether s is one of the five derived states.
func (s AttentionState) Valid() bool {
	switch s {
	case FailedToStart, Waiting, Working, Idle, Silent:
		return true
	}
	return false
}

// Glyph is the state's mark (spec §2.6). It pairs with Word and never stands
// as colour alone: strip colour entirely and glyph + word still fully
// determine the state, which is how the no-colour-only rule is satisfied by
// construction (§2.6, §2.9).
func (s AttentionState) Glyph() string {
	switch s {
	case FailedToStart:
		return "✕"
	case Waiting:
		return "⟡" // gold is the reserved human-gate semantic (spec §2.6)
	case Working:
		return "▸"
	case Idle:
		return "·"
	case Silent:
		return "?" // silent renders as a question, never green (spec §2.6)
	}
	return "?"
}

// Word is the state's spelled label (spec §2.6). "waiting on you" carries the
// direction the gold gate points; the others are the bare state word.
func (s AttentionState) Word() string {
	switch s {
	case FailedToStart:
		return "failed-to-start"
	case Waiting:
		return "waiting on you"
	case Working:
		return "working"
	case Idle:
		return "idle"
	case Silent:
		return "silent"
	}
	return "silent"
}

// AttentionRender is the wire shape of a derived state: the state word, its
// glyph, and its spelled label together, so a consumer NEVER has to encode a
// state as colour alone (spec §2.6, §2.9). It is what a Console row carries.
type AttentionRender struct {
	State AttentionState `json:"state"`
	Glyph string         `json:"glyph"`
	Word  string         `json:"word"`
}

// Render packages a state for serving.
func Render(s AttentionState) AttentionRender {
	return AttentionRender{State: s, Glyph: s.Glyph(), Word: s.Word()}
}

// MarshalJSON renders the state as its word. A bare enum on the wire is fine
// here because AttentionRender carries the glyph beside it; the state string
// alone is never the only encoding a consumer sees.
func (s AttentionState) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(s))
}
