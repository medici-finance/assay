package deskkit

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// The desk audit lock: ONE advisory lock (<state dir>/audit.lock) every desk write takes so a
// read-check-append over the audit ledger and its sibling records cannot interleave.
//
// AcquireAuditLock is the single choke point for taking it. Every acquirer goes through
// AcquireHeld, so the holder's pid, process start and acquisition time are recorded beside the
// lock (audit.lock.holder) and a waiter that times out can say WHO holds it, not just that it is
// busy. A lock is a kernel advisory lock: it is released when its holder exits, however it
// exits, so a holder that is gone never leaves a stale lock to clear — the next acquirer simply
// takes it and overwrites the leftover owner record. A lock that stays busy therefore has a live
// holder; the message says which one when it can be told and how to find it when it cannot.

// AuditLockName is the lock file's name inside the desk state dir.
const AuditLockName = "audit.lock"

// AuditLockWait is how long a writer waits for a contended audit lock before refusing.
const AuditLockWait = 60 * time.Second

const auditLockLabel = "desk audit lock"

// AuditLock is a held audit lock. Release it exactly once, after the audit append.
type AuditLock struct {
	held *HeldLock
	root *os.Root
}

// Release drops the lock and removes its owner record. It is safe on a nil receiver.
func (l *AuditLock) Release() {
	if l == nil {
		return
	}
	_ = l.held.Release()
	_ = l.root.Close()
}

// AcquireAuditLock takes the audit lock in dir (created 0700 when absent), waiting up to wait
// for a contended one. It FAILS CLOSED: a lock it cannot take is exit 6 (Unverifiable), never
// permission to proceed. The refusal for a lock that stayed busy names the lock file, the
// recorded holder's pid, process start and age when there is one, and the action that clears it.
func AcquireAuditLock(dir string, wait time.Duration) (*AuditLock, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, Unverifiable("cannot create the desk state dir", err)
	}
	return acquireAudit(dir, LockSpec{Name: auditLockLabel, Wait: wait})
}

// tryAcquireAuditLock is ONE non-blocking attempt, for optional work (log rotation) that must
// never make a writer wait. ok=false means "someone else holds it" or "could not open it": it is
// permission to do nothing, never to proceed without the lock.
func tryAcquireAuditLock(dir string) (*AuditLock, bool) {
	l, err := acquireAudit(dir, LockSpec{Name: auditLockLabel, Probe: true})
	return l, err == nil
}

func acquireAudit(dir string, spec LockSpec) (*AuditLock, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, Unverifiable("cannot open the desk state dir", err)
	}
	f, err := root.OpenFile(AuditLockName, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		root.Close()
		return nil, Unverifiable("cannot open the audit lock", err)
	}
	held, err := AcquireHeld(root, AuditLockName, f, spec)
	if err != nil {
		root.Close()
		var busy *BusyError
		if errors.As(err, &busy) {
			return nil, Unverifiable(AuditBusyMessage(busy), nil)
		}
		if errors.Is(err, ErrLockBusy) { // a probe: the caller reads only ok
			return nil, err
		}
		return nil, Unverifiable("cannot acquire the audit lock", err)
	}
	return &AuditLock{held: held, root: root}, nil
}

// AuditBusyMessage renders a lasting-busy audit lock as one actionable sentence: which file, who
// holds it (pid, process start, age) when the owner record checks out, and what clears it. It
// never tells anyone to delete the lock file — deleting a flock file does not release the lock a
// live holder has open and lets a second writer in beside it.
func AuditBusyMessage(b *BusyError) string {
	d := b.Diagnosis
	head := fmt.Sprintf("the desk audit lock %s stayed busy for the bounded %s wait (retryable)",
		b.Path, b.Waited.Round(time.Second))
	inspect := "list the processes holding it with `lsof " + b.Path + "`"
	switch {
	case d.Record != nil && (d.State == HolderCurrent || d.State == HolderStale):
		return head + "; held by " + auditHolder(d.Record) + ", which is running and has the lock file open — " +
			"a live writer, not a stale lock. Action: wait and retry; if that process is stuck, a human ends pid " +
			fmt.Sprint(d.Record.PID) + " (nothing here signals it) and the lock frees at once"
	case d.Record != nil && d.State == HolderUnverified:
		return head + "; the owner record names " + auditHolder(d.Record) + " but whether it is alive and holds the lock " +
			"could not be checked here (" + d.Reason + "). Action: " + inspect + "; a human ends the holder if it is stuck"
	case d.Record != nil:
		return head + "; the owner record names " + auditHolder(d.Record) + " but it does not check out (" + d.Reason +
			"). The lock is still held by an unrecorded process, such as a child that inherited the lock file or a writer " +
			"from a build that records no owner. A lock whose holder has exited is released by the operating system, so a " +
			"stale lock file never needs deleting. Action: " + inspect + "; a human ends the right process"
	default:
		return head + "; no owner record is readable (" + d.Reason + "), so the holder is unknown. It is a live process: " +
			"a lock whose holder has exited is released by the operating system, so a stale lock file never needs deleting. " +
			"Action: " + inspect + "; a human ends the right process if it is stuck"
	}
}

func auditHolder(r *LockHolder) string {
	s := fmt.Sprintf("pid %d (%s", r.PID, r.Command)
	if r.Start != "" {
		s += ", process start " + processStartText(r.Start)
	}
	if !r.Acquired.IsZero() {
		s += ", holding since " + r.Acquired.UTC().Format(time.RFC3339) + ", age " + time.Since(r.Acquired).Round(time.Second).String()
	}
	return s + ")"
}

// processStartText renders the platform's start identity: a wall-clock time where the platform
// records one ("<unix seconds>.<micros>"), otherwise the opaque value as recorded.
func processStartText(start string) string {
	sec, _, ok := strings.Cut(start, ".")
	if n, err := strconv.ParseInt(sec, 10, 64); ok && err == nil && n > 1e9 {
		return time.Unix(n, 0).UTC().Format(time.RFC3339)
	}
	return start
}
