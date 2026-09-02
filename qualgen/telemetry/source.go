// Package telemetry declares the pluggable M4 session-forensics seam (spec
// §7.3, §3.1 profile-B) and ships its one reference implementation.
//
// The interface is deliberately narrow and repo-agnostic: given the PR a
// harness session (or sessions) produced, return that PR's harness telemetry
// — retries, context length, tool-call churn, interruptions, refusals — OR a
// three-state could-not-measure. No concrete data source is named here; the
// house or another operator wires their own TelemetrySource as configuration,
// never a fork of qualgen/m4.
//
// OSS boundary (load-bearing, quality/13's Context §"OSS boundary"): the CODE
// in this package — the interface and the file-based reference adapter — is
// OSS. The telemetry DATA itself, and any shared-corpus inclusion, retention,
// and audit, are governed by a separate operator privacy ruling this package
// does not assume, name, or require. FileAdapter reads only an
// operator-supplied path; it stays inert until that path is configured.
package telemetry

import (
	"encoding/json"
	"fmt"
	"strings"
)

// --- three-state instrument wrapper -----------------------------------
//
// This is a package-local copy of qualgen's frozen Measure[T] (spec §3.2,
// qualgen/measure.go). It is duplicated, not imported: Go forbids importing
// package main as a library, and this package must stay self-contained (no
// dependency on the qualgen command package) so a telemetry source is pure
// configuration — the same self-containment qualgen/adapters already
// establishes for the fix-linkage reference adapter. The three states, their
// invariants, and the wire shape are kept byte-identical to measure.go's so a
// consumer that already knows qualgen's three-state JSON shape reads this
// one for free.

// State is the three-state instrument enum (spec §3.2).
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

// Measure[T] is the three-state instrument wrapper every value this package
// emits is carried in — never a silent zero, never a could-not-measure
// rounded up to a pass (spec §3.2, common clause C4).
type Measure[T any] struct {
	State  State
	Value  T
	Reason string
}

// Measured wraps a real, read value.
func Measured[T any](v T) Measure[T] { return Measure[T]{State: StateMeasured, Value: v} }

// MeasuredZero records a genuine zero — the instrument ran, the answer is
// zero.
func MeasuredZero[T any]() Measure[T] { return Measure[T]{State: StateMeasuredZero} }

// CouldNotMeasure records that the value could not be read, with a required
// non-empty reason.
func CouldNotMeasure[T any](reason string) Measure[T] {
	return Measure[T]{State: StateCouldNotMeasure, Reason: reason}
}

// validate enforces the three-state invariants on marshal/unmarshal.
func (m Measure[T]) validate() error {
	switch m.State {
	case StateMeasured, StateMeasuredZero:
		if m.Reason != "" {
			return fmt.Errorf("telemetry: a %q measure must not carry a reason (reason is could-not-measure only)", m.State)
		}
	case StateCouldNotMeasure:
		if strings.TrimSpace(m.Reason) == "" {
			return fmt.Errorf("telemetry: a could-not-measure requires a non-empty reason")
		}
	case "":
		return fmt.Errorf("telemetry: measure has no state (one of measured / measured-zero / could-not-measure required)")
	default:
		return fmt.Errorf("telemetry: unknown measure state %q", m.State)
	}
	return nil
}

type measureJSON[T any] struct {
	State  State  `json:"state"`
	Value  *T     `json:"value,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// MarshalJSON emits the frozen three-state shape, validating first.
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

// UnmarshalJSON parses the three-state shape and re-validates.
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

// --- the seam -----------------------------------------------------------

// PRKey is the join key quality's M1/M2 artifacts and the stream board
// already share (spec §8): PR number + merge SHA + stream/task ID. A
// TelemetrySource is keyed by it so the forensics join (qualgen/m4) resolves
// telemetry to the exact PR it produced, not merely "a" session.
type PRKey struct {
	PRNumber     int    `json:"pr_number"`
	MergeSHA     string `json:"merge_sha"`
	StreamTaskID string `json:"stream_task_id"`
}

// String renders key for a diagnostic message (could-not-measure reasons,
// log lines) — never for equality or lookup, where the struct itself is the
// key.
func (k PRKey) String() string {
	return fmt.Sprintf("pr=%d merge_sha=%s stream_task_id=%s", k.PRNumber, k.MergeSHA, k.StreamTaskID)
}

// TelemetryRecord is one PR's harness telemetry (spec §7.3): the scaffolding
// signals the session forensics join correlates against M1/M2 outcomes.
// Meaningful only inside a Measured or MeasuredZero Measure[TelemetryRecord]
// — a could-not-measure Measure carries no TelemetryRecord.
type TelemetryRecord struct {
	// Retries is the count of harness-level tool-call or turn retries
	// (network/rate-limit/parse retries the harness itself performed, not a
	// deliberate user re-prompt).
	Retries int `json:"retries"`
	// ContextLength is the session's peak context length in tokens.
	ContextLength int `json:"context_length"`
	// ToolCallChurn is the count of tool calls whose result was discarded or
	// superseded without contributing to the final diff (a read the agent
	// abandoned, a write it reverted).
	ToolCallChurn int `json:"tool_call_churn"`
	// Interruptions is the count of times a human interrupted or redirected
	// the session mid-run.
	Interruptions int `json:"interruptions"`
	// Refusals is the count of times the model declined or safety-refused a
	// requested action during the session.
	Refusals int `json:"refusals"`
}

// TelemetrySource is the pluggable seam (spec §7.3, §3.1 profile-B): given
// the PR a harness session (or sessions) produced, return that PR's
// telemetry OR a three-state could-not-measure. No concrete data source is
// named by this interface — FileAdapter is the one reference implementation
// this package ships; any other source (a house's own telemetry corpus, once
// an operator privacy ruling authorizes one) is a new adapter satisfying this
// interface, never a fork of the forensics join that consumes it.
//
// Telemetry never returns a Go error: the three-state Measure IS the error
// channel (could-not-measure carries the reason) — mirroring the frozen
// Measure[T] discipline rather than asking every caller to fold a second
// error path into the same "the value is unavailable" fact.
type TelemetrySource interface {
	// Telemetry returns key's harness telemetry, or could-not-measure when
	// the source has no readable record for key — never a silent zero.
	Telemetry(key PRKey) Measure[TelemetryRecord]
}
