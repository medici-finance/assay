package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Measure[T] is the first-class three-state instrument wrapper (spec §3.2).
// Every metric value qualgen ever emits is wrapped in this type, so that a
// failed read, an unreadable blob, or a squash-hidden parent is reported as a
// DISTINCT could-not-measure state — never silently as zero, and never conflated
// with a genuine measured-zero.
//
// This is the seam quality/02, quality/03, quality/04 and quality/06 consume;
// this brief freezes it and later briefs must not redefine it.
//
// Invariants (enforced on marshal/unmarshal by validate):
//   - State is exactly one of the three constants below.
//   - Value is meaningful only when State == StateMeasured. (A measured-zero
//     carries the zero value implicitly; a could-not-measure carries no value.)
//   - Reason is required (non-empty) when State == StateCouldNotMeasure, and
//     must be empty otherwise.
//
// JSON shape:
//
//	{"state":"measured|measured-zero|could-not-measure","value":...,"reason":...}
type Measure[T any] struct {
	State  State
	Value  T
	Reason string
}

// State is the three-state instrument enum. The string values are the ON-DISK
// contract read by every downstream aggregator; do not rename them.
type State string

const (
	// StateMeasured: a real value was read. Value is meaningful.
	StateMeasured State = "measured"
	// StateMeasuredZero: the instrument ran and the genuine answer is zero
	// (e.g. a text file changed by zero lines). Distinct from could-not-measure.
	StateMeasuredZero State = "measured-zero"
	// StateCouldNotMeasure: the instrument could not read the value (failed
	// blame, unreadable/binary blob, unreachable-through-squash parent). Reason
	// is required and names why.
	StateCouldNotMeasure State = "could-not-measure"
)

// Measured wraps a real, read value.
func Measured[T any](v T) Measure[T] {
	return Measure[T]{State: StateMeasured, Value: v}
}

// MeasuredZero records a genuine zero — the instrument ran, the answer is zero.
func MeasuredZero[T any]() Measure[T] {
	return Measure[T]{State: StateMeasuredZero}
}

// CouldNotMeasure records that the value could not be read, with a required
// non-empty reason.
func CouldNotMeasure[T any](reason string) Measure[T] {
	return Measure[T]{State: StateCouldNotMeasure, Reason: reason}
}

// validate enforces the three-state invariants. Called on every marshal and
// unmarshal so a malformed measure can never round-trip through the artifacts.
func (m Measure[T]) validate() error {
	switch m.State {
	case StateMeasured, StateMeasuredZero:
		if m.Reason != "" {
			return fmt.Errorf("qualgen: a %q measure must not carry a reason (reason is could-not-measure only)", m.State)
		}
	case StateCouldNotMeasure:
		if strings.TrimSpace(m.Reason) == "" {
			return fmt.Errorf("qualgen: a could-not-measure requires a non-empty reason")
		}
	case "":
		return fmt.Errorf("qualgen: measure has no state (one of measured / measured-zero / could-not-measure required)")
	default:
		return fmt.Errorf("qualgen: unknown measure state %q", m.State)
	}
	return nil
}

// measureJSON is the wire representation. Value is carried only for the measured
// and measured-zero states; reason only for could-not-measure.
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
	case StateMeasured:
		v := m.Value
		out.Value = &v
	case StateMeasuredZero:
		// The value is the implicit zero; carried explicitly so a consumer that
		// does arithmetic on measured-zero rows reads a real 0, not a missing
		// field. It stays distinct from measured via the state string.
		v := m.Value
		out.Value = &v
	case StateCouldNotMeasure:
		out.Reason = m.Reason
	}
	return json.Marshal(out)
}

// UnmarshalJSON parses the three-state shape and re-validates the invariants, so
// a hand-edited or corrupt artifact line fails loudly rather than silently
// decoding to a zero.
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
