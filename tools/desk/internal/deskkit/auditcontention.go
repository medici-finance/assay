package deskkit

import "errors"

// AuditLockContended is the refusal for a desk audit lock that stayed busy for the whole bounded
// wait (exit 6, fail closed: never "assume free"). msg is the actionable sentence for the caller
// (AuditBusyMessage). It is the ONE place a lock-wait refusal is marked as such, and its only
// caller is AcquireAuditLock, the single choke point every acquirer goes through, so every
// audit line a lock-wait refusal produces can be classified without parsing the message.
//
// The wait is the first step of an outward write, before any audit read, forge read or forge
// write, so this refusal says nothing about the caller's input (#6227). LockStepResult turns it
// into ResultContended, which neither the budget nor the circuit breaker counts.
func AuditLockContended(msg string) *DeskError {
	return &DeskError{Code: ExitUnverifiable, Msg: msg, lockContended: true}
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
