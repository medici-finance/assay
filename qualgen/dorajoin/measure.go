// Package dorajoin joins two already-mined layers — the M1 line-operation +
// churn aggregates (brief-02) and the M2 SZZ defect-trace metrics (brief-07)
// — with an external delivery-metrics feed, on the keys the board already
// uses (PR number, merge SHA, stream/task ID). It contributes a quality
// denominator (durable-change volume) and a traced-CFR refinement into the
// existing DORA collection (spec §8).
//
// This package is deliberately self-contained — no dependency on the qualgen
// command package — for the same reason the sibling adapters package is
// (qualgen/adapters/githublabels.go): a subpackage cannot import the `main`
// package qualgen's own types (Measure[T], MetricRecord, DefectTrace) live
// in, so wiring the join's raw inputs from qualgen's own aggregates is the
// CALLER's job (a thin translation layer in the qualgen command, mirroring
// GithubLabelsLinkage's translation of adapters.GithubLabels into the
// LinkageAdapter interface), never a dependency edge from here back into
// qualgen.
package dorajoin

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Measure[T] mirrors qualgen's frozen three-state instrument wrapper
// (spec §3.2, qualgen/measure.go) byte-for-byte — same states, same JSON wire
// shape — so a dorajoin artifact round-trips identically to a qualgen one and
// a downstream reader never needs a translation table between the two. It is
// a SEPARATE type (not a re-export) purely because a subpackage cannot import
// package main; the contract it enforces is identical, not merely similar.
//
// Every value this package computes or reports is wrapped in it: a failed
// read, an unmeasured component, or an unresolved join key is a DISTINCT
// could-not-measure/could-not-join state — never silently zero, never
// conflated with a genuine measured-zero.
type Measure[T any] struct {
	State  State
	Value  T
	Reason string
}

// State is the three-state instrument enum — the on-disk contract. The
// string values match qualgen's own (qualgen/measure.go) so a reader that
// already knows qualgen's contract needs no new vocabulary for this package's
// artifacts.
type State string

const (
	// StateMeasured: a real value was read. Value is meaningful.
	StateMeasured State = "measured"
	// StateMeasuredZero: the instrument ran and the genuine answer is zero.
	// Distinct from could-not-measure.
	StateMeasuredZero State = "measured-zero"
	// StateCouldNotMeasure: the instrument could not read the value. Reason
	// is required and names why.
	StateCouldNotMeasure State = "could-not-measure"
)

// Measured wraps a real, read value.
func Measured[T any](v T) Measure[T] {
	return Measure[T]{State: StateMeasured, Value: v}
}

// MeasuredZero records a genuine zero — the instrument ran, the answer is
// zero.
func MeasuredZero[T any]() Measure[T] {
	return Measure[T]{State: StateMeasuredZero}
}

// CouldNotMeasure records that the value could not be read, with a required
// non-empty reason.
func CouldNotMeasure[T any](reason string) Measure[T] {
	return Measure[T]{State: StateCouldNotMeasure, Reason: reason}
}

// IsMeasured reports whether the value participates in arithmetic: measured
// and measured-zero both do (a measured-zero's Value is the type's zero
// value); could-not-measure and the unset zero-value Measure do not.
func (m Measure[T]) IsMeasured() bool {
	return m.State == StateMeasured || m.State == StateMeasuredZero
}

// validate enforces the three-state invariants, identical to qualgen's own.
func (m Measure[T]) validate() error {
	switch m.State {
	case StateMeasured, StateMeasuredZero:
		if m.Reason != "" {
			return fmt.Errorf("dorajoin: a %q measure must not carry a reason (reason is could-not-measure only)", m.State)
		}
	case StateCouldNotMeasure:
		if strings.TrimSpace(m.Reason) == "" {
			return fmt.Errorf("dorajoin: a could-not-measure requires a non-empty reason")
		}
	case "":
		return fmt.Errorf("dorajoin: measure has no state (one of measured / measured-zero / could-not-measure required)")
	default:
		return fmt.Errorf("dorajoin: unknown measure state %q", m.State)
	}
	return nil
}

type measureJSON[T any] struct {
	State  State  `json:"state"`
	Value  *T     `json:"value,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// MarshalJSON emits the frozen three-state shape, validating first so an
// invalid measure never reaches an artifact.
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
