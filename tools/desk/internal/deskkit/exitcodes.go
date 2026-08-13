package deskkit

import (
	"errors"
	"fmt"
	"time"
)

// Canonical exit codes — the shared contract every desk tool maps its typed errors
// to (README "Shared conventions"). A tool's main() computes its exit
// status with ExitCodeOf(err) and calls os.Exit with the result.
//
// Exit 0 is reserved for a POSITIVE success or an idempotent no-op; an unexpected
// error must never collapse to 0 (that is the silent-default the fail-closed contract forbids).
const (
	// ExitOK — positive success OR an idempotent no-op.
	ExitOK = 0
	// ExitDisabled — kill switch armed: ~/.config/assay/DISABLED exists or
	// DESK_TOOLS_DISABLED=1.
	ExitDisabled = 3
	// ExitRateLimited — the caller must wait. Either the outward-write budget is spent
	// (>=10 charged writes per tool per rolling hour) or the non-progress circuit
	// breaker is open. Both carry a RetryAfter; see ratelimit.go.
	ExitRateLimited = 4
	// ExitRefused — refused by a compiled-in constraint (repo outside the fixed
	// set, secret-scan hit, head mismatch, …).
	ExitRefused = 5
	// ExitUnverifiable — a precondition could not be POSITIVELY verified (audit
	// file unreadable/malformed, HOME missing, API error mid-check). Fail closed;
	// never guess.
	ExitUnverifiable = 6
)

// DeskError carries a canonical exit code so callers never re-derive the mapping.
type DeskError struct {
	Code int
	Msg  string
	Err  error
	// RetryAfter is how long the caller must wait before the same call could succeed.
	// It is set only on ExitRateLimited errors and is zero everywhere else. A bare
	// exit 4 with no retry-after is what turned four review agents into a retry
	// livelock (#209): each guessed, each guessed short,
	// and their retries kept one another out. Callers read this — or the retry-after
	// stated in Msg — and sleep ONCE for the stated duration instead of polling.
	RetryAfter time.Duration
}

func (e *DeskError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Msg, e.Err)
	}
	return e.Msg
}

// Unwrap exposes the wrapped cause for errors.Is / errors.As.
func (e *DeskError) Unwrap() error { return e.Err }

// ExitCode returns the canonical exit code this error maps to.
func (e *DeskError) ExitCode() int { return e.Code }

// Disabled builds a kill-switch refusal (exit 3).
func Disabled(msg string) *DeskError { return &DeskError{Code: ExitDisabled, Msg: msg} }

// RateLimited builds a wait-and-retry refusal (exit 4) with NO retry-after.
// Prefer RateLimitedAfter: a caller that cannot learn WHEN to retry retry-loops, and a
// retry loop against this limiter is what produced the #209 livelock.
func RateLimited(msg string) *DeskError { return &DeskError{Code: ExitRateLimited, Msg: msg} }

// RateLimitedAfter builds a wait-and-retry refusal (exit 4) that tells the caller
// how long to wait before the same call could succeed (#209).
func RateLimitedAfter(msg string, retryAfter time.Duration) *DeskError {
	return &DeskError{Code: ExitRateLimited, Msg: msg, RetryAfter: retryAfter}
}

// RetryAfterOf returns the retry-after carried by a rate-limit error, or 0 when err is
// nil, is not a *DeskError, or carries none. This is the programmatic read of the same
// value the refusal message states, for callers that schedule their own back-off.
func RetryAfterOf(err error) time.Duration {
	var de *DeskError
	if errors.As(err, &de) {
		return de.RetryAfter
	}
	return 0
}

// Refused builds a constraint refusal (exit 5).
func Refused(msg string) *DeskError { return &DeskError{Code: ExitRefused, Msg: msg} }

// Unverifiable builds a fail-closed refusal for state that could not be positively
// verified (exit 6). Pass the underlying cause as err so it survives Unwrap.
func Unverifiable(msg string, err error) *DeskError {
	return &DeskError{Code: ExitUnverifiable, Msg: msg, Err: err}
}

// ExitCodeOf maps an error to a process exit code, failing CLOSED:
//   - nil                → ExitOK (0)
//   - a *DeskError       → its Code
//   - any OTHER non-nil  → ExitUnverifiable (6)
//
// The last rule is the load-bearing one: an unexpected error is treated as a
// precondition we could not verify, NOT as success. No error path collapses to 0.
func ExitCodeOf(err error) int {
	if err == nil {
		return ExitOK
	}
	var de *DeskError
	if errors.As(err, &de) {
		return de.Code
	}
	return ExitUnverifiable
}

// IsDisabled reports whether err maps to the kill-switch exit code.
func IsDisabled(err error) bool { return codeIs(err, ExitDisabled) }

// IsRateLimited reports whether err maps to the rate-limit exit code.
func IsRateLimited(err error) bool { return codeIs(err, ExitRateLimited) }

// IsRefused reports whether err maps to the constraint-refusal exit code.
func IsRefused(err error) bool { return codeIs(err, ExitRefused) }

// IsUnverifiable reports whether err maps to the fail-closed exit code.
func IsUnverifiable(err error) bool { return codeIs(err, ExitUnverifiable) }

func codeIs(err error, code int) bool {
	var de *DeskError
	if errors.As(err, &de) {
		return de.Code == code
	}
	return false
}
