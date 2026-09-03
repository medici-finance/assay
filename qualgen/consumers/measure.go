package consumers

import (
	"encoding/json"
	"fmt"
	"strings"
)

// measure.go — a package-local copy of qualgen's frozen three-state instrument
// wrapper (spec §3.2, qualgen/measure.go), kept identical in shape to the
// qualgen/reflex, qualgen/dorajoin, and qualgen/telemetry copies. It is
// duplicated, not imported: Go forbids importing package main as a library, and
// this package must stay self-contained (no dependency edge back into the
// qualgen command package), exactly the self-containment those siblings already
// establish. The three states, their invariants, and the wire shape are kept
// identical so a consumer that already knows qualgen's three-state JSON shape
// reads a consumers artifact for free.
//
// It is used here for the ONE place quality/14 must be honestly three-state: a
// budget over a stream with fewer than two measured windows is
// could-not-measure, never a budget armed at zero (spec §9.6).

// State is the three-state instrument enum — the on-disk contract. The string
// values match qualgen's own.
type State string

const (
	// StateMeasured: a real value was read. Value is meaningful.
	StateMeasured State = "measured"
	// StateMeasuredZero: the instrument ran and the genuine answer is zero.
	// Distinct from could-not-measure.
	StateMeasuredZero State = "measured-zero"
	// StateCouldNotMeasure: the instrument could not read the value. Reason is
	// required and names why.
	StateCouldNotMeasure State = "could-not-measure"
)

// Measure[T] is the three-state instrument wrapper every value this package
// reports as three-state is carried in — never a silent zero, never a
// could-not-measure rounded up to a pass (spec §3.2).
type Measure[T any] struct {
	State  State
	Value  T
	Reason string
}

// Measured wraps a real, read value.
func Measured[T any](v T) Measure[T] { return Measure[T]{State: StateMeasured, Value: v} }

// MeasuredZero records a genuine zero — the instrument ran, the answer is zero.
func MeasuredZero[T any]() Measure[T] { return Measure[T]{State: StateMeasuredZero} }

// CouldNotMeasure records that the value could not be read, with a required
// non-empty reason.
func CouldNotMeasure[T any](reason string) Measure[T] {
	return Measure[T]{State: StateCouldNotMeasure, Reason: reason}
}

// IsMeasured reports whether the value participates in arithmetic: measured and
// measured-zero both do; could-not-measure and the unset zero-value Measure do
// not.
func (m Measure[T]) IsMeasured() bool {
	return m.State == StateMeasured || m.State == StateMeasuredZero
}

// validate enforces the three-state invariants, identical to qualgen's own.
func (m Measure[T]) validate() error {
	switch m.State {
	case StateMeasured, StateMeasuredZero:
		if m.Reason != "" {
			return fmt.Errorf("consumers: a %q measure must not carry a reason (reason is could-not-measure only)", m.State)
		}
	case StateCouldNotMeasure:
		if strings.TrimSpace(m.Reason) == "" {
			return fmt.Errorf("consumers: a could-not-measure requires a non-empty reason")
		}
	case "":
		return fmt.Errorf("consumers: measure has no state (one of measured / measured-zero / could-not-measure required)")
	default:
		return fmt.Errorf("consumers: unknown measure state %q", m.State)
	}
	return nil
}

type measureJSON[T any] struct {
	State  State  `json:"state"`
	Value  *T     `json:"value,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// MarshalJSON emits the frozen three-state shape, validating first so an invalid
// measure never reaches an artifact.
func (m Measure[T]) MarshalJSON() ([]byte, error) {
	if err := m.validate(); err != nil {
		return nil, err
	}
	out := measureJSON[T]{State: m.State}
	switch m.State {
	case StateMeasured, StateMeasuredZero:
		v := m.Value
		out.Value = &v
	case StateCouldNotMeasure:
		out.Reason = m.Reason
	}
	return json.Marshal(out)
}

// UnmarshalJSON parses the three-state shape and re-validates the invariants.
func (m *Measure[T]) UnmarshalJSON(data []byte) error {
	var in measureJSON[T]
	if err := json.Unmarshal(data, &in); err != nil {
		return err
	}
	m.State = in.State
	m.Reason = in.Reason
	var zero T
	m.Value = zero
	if in.Value != nil {
		m.Value = *in.Value
	}
	return m.validate()
}
