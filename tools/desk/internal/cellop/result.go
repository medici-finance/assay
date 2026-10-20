// Package cellop is the cell-operation contract, cell-op-v1 (spec/cell-op-v1.md): the one result
// document and the one plan document that every cell-level operator verb emits, whichever adapter
// carries it (the cellctl CLI now, a read/plan-only Cell API later), plus the plan store, the
// atomic consume and the confirm-time re-measure harness that applying a plan goes through.
//
// Applying a plan is an interactive-terminal confirmation only. The confirm, the re-measure and the
// atomic consume guard against operator mistakes and concurrent races; they are NOT an
// authorization control, because a process running under the operator's own account can drive a
// terminal and can delete a consumed marker. There is no approval, no non-interactive apply and no
// API apply in this package, by design (the spec's §Deferred).
package cellop

import (
	"encoding/json"
	"time"
)

// The contract's fixed markers. Schema names the contract version every document carries; Kind
// tells a result from a plan.
const (
	Schema     = "cell-op-v1"
	KindResult = "cell-op-result"
	KindPlan   = "cell-op-plan"
)

// Outcome is a result's one-word verdict.
type Outcome string

const (
	// OK: the verb did everything it set out to do. A read verb's unmeasured fields are nulls
	// inside an ok result, never a different outcome.
	OK Outcome = "ok"
	// Partial: at least one action ran to completion and a later one failed or was stopped.
	Partial Outcome = "partial"
	// Refused: nothing was changed; Reason says why.
	Refused Outcome = "refused"
	// Failed: an action failed before any action completed.
	Failed Outcome = "failed"
	// CouldNotCheck: a measurement the verb needed could not be taken; nothing was changed.
	CouldNotCheck Outcome = "could-not-check"
)

// ExitCode is the process exit code the contract assigns an outcome: 0 ok, 1 failed or partial,
// 5 refused, 6 could-not-check. An outcome outside the contract exits 1, never 0.
func (o Outcome) ExitCode() int {
	switch o {
	case OK:
		return 0
	case Refused:
		return 5
	case CouldNotCheck:
		return 6
	}
	return 1
}

// Refusal and failure reasons, the closed vocabulary a result's reason field draws from.
const (
	ReasonNotInteractive  = "not-interactive"  // a state-changing invocation without a terminal on stdin and stdout
	ReasonDeskSession     = "desk-session"     // a desk-session marker is set in the invoking environment
	ReasonPlanNotFound    = "plan-not-found"   // no stored plan has this id
	ReasonPlanInvalid     = "plan-invalid"     // the stored plan does not parse, or its digest does not recompute
	ReasonWrongVerb       = "wrong-verb"       // the plan was made by a different verb
	ReasonForeignCell     = "foreign-cell"     // the invoking cell is not among the plan's cells
	ReasonPlanExpired     = "plan-expired"     // the plan (or its batch) is past its expiry
	ReasonAlreadyConsumed = "already-consumed" // the plan's consumed marker exists
	ReasonConfirmMismatch = "confirm-mismatch" // the operator did not type the confirm code
	ReasonPlanStale       = "plan-stale"       // a precondition no longer holds its recorded value
	ReasonUnmeasured      = "unmeasured"       // a precondition could not be measured (could-not-check)
	ReasonActionFailed    = "action-failed"    // an action failed after the plan was consumed
)

// Role is one role's line in a result. A nil State or field value is unmeasured, never a default.
type Role struct {
	Role   string             `json:"role"`
	State  *string            `json:"state"`
	Fields map[string]*string `json:"fields"`
}

// Action outcomes inside a result.
const (
	ActionOK     = "ok"
	ActionFailed = "failed"
	ActionNotRun = "not-run"
)

// ActionResult is what happened to one planned action.
type ActionResult struct {
	ID      string  `json:"id"`
	Cell    string  `json:"cell"`
	Outcome string  `json:"outcome"`
	Detail  *string `json:"detail"`
}

// Result is the cell-op-result document.
type Result struct {
	Schema   string         `json:"schema"`
	Kind     string         `json:"kind"`
	Verb     string         `json:"verb"`
	Cell     string         `json:"cell"`
	Started  string         `json:"started"`
	Finished string         `json:"finished"`
	Outcome  Outcome        `json:"outcome"`
	Reason   *string        `json:"reason"`
	Detail   *string        `json:"detail"`
	Roles    []Role         `json:"roles"`
	Actions  []ActionResult `json:"actions"`
	PlanID   *string        `json:"plan_id"`
	Next     []string       `json:"next"`
}

// Stamp renders a time the way every cell-op-v1 document does: RFC 3339, UTC, whole seconds.
func Stamp(t time.Time) string { return t.UTC().Truncate(time.Second).Format(time.RFC3339) }

// NewResult starts a result for verb on cell. Its lists are empty, never null, and its outcome is
// could-not-check until Finish says otherwise, so a result that is never finished cannot read ok.
func NewResult(verb, cell string, started time.Time) Result {
	s := Stamp(started)
	return Result{
		Schema: Schema, Kind: KindResult, Verb: verb, Cell: cell,
		Started: s, Finished: s, Outcome: CouldNotCheck,
		Roles: []Role{}, Actions: []ActionResult{}, Next: []string{},
	}
}

// Finish records the outcome, its reason (empty for none) and an optional detail line.
func (r *Result) Finish(o Outcome, reason, detail string, finished time.Time) {
	r.Outcome = o
	r.Finished = Stamp(finished)
	r.Reason, r.Detail = optional(reason), optional(detail)
}

// ExitCode is the result's process exit code.
func (r Result) ExitCode() int { return r.Outcome.ExitCode() }

// JSON is the result as one line of JSON.
func (r Result) JSON() ([]byte, error) { return json.Marshal(r) }

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
