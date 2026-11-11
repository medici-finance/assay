package deskkit

import (
	"errors"
	"fmt"
	"time"
)

// AuditLockContended is the refusal for a desk audit lock that stayed busy for the whole bounded
// wait (exit 6, fail closed: never "assume free"). It is the ONE place that message is built, and
// the one place a lock-wait refusal is marked as such, so every acquirer reports contention the
// same way and every audit line it writes can be classified without parsing the message.
//
// The wait is the first step of an outward write, before any audit read, forge read or forge
// write, so this refusal says nothing about the caller's input (#6227). LockStepResult turns it
// into ResultContended, which neither the budget nor the circuit breaker counts.
func AuditLockContended(wait time.Duration) *DeskError {
	w := wait.String()
	if wait > 0 && wait%time.Second == 0 {
		w = fmt.Sprintf("%ds", int(wait/time.Second))
	}
	return &DeskError{
		Code: ExitUnverifiable,
		Msg: fmt.Sprintf(
			"audit lock held >%s (stale lock?) — another desk write may be stuck; retry, or a human clears it", w),
		lockContended: true,
	}
}

// IsAuditLockContended reports whether err is (or wraps) an AuditLockContended refusal.
func IsAuditLockContended(err error) bool {
	var de *DeskError
	return errors.As(err, &de) && de.lockContended
}

// LockStepResult is the audit result for a failure at the audit-lock step: ResultContended for
// a lock-wait refusal, otherwise `other` unchanged. A lock that could not be opened or created is
// a real fault the caller repeats identically, so it keeps the class the caller already gave it
// and keeps counting; only contention is exempt.
func LockStepResult(err error, other string) string {
	if IsAuditLockContended(err) {
		return ResultContended
	}
	return other
}
