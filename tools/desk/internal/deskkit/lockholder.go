package deskkit

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// Lock-holder identification for advisory locks (TryLockExclusive).
//
// flock and LockFileEx cannot say WHO holds a lock, so the acquirer writes an
// owner record next to the lock file (<name>.holder) once it holds the lock. A
// waiter that meets ErrLockBusy reads that record and checks it against the live
// system before it believes a word of it: the recorded pid must be running, its
// start time must match (a reused pid fails here), its command must match, and it
// must actually have the lock file open. A record that fails any check means the
// real holder is UNKNOWN; it is never a reason to act on the recorded pid.
//
// Only a verified holder whose recorded role child has exited is STALE, and only a
// stale holder that is the caller's own cellctl for this exact cell, role and lock
// may be ended — and then only under the caller's explicit opt-in, with SIGTERM, a
// bounded wait and a re-check of the lock. SIGKILL is never sent.

const (
	lockHolderSchema = "deskkit-lock-holder-v1"
	lockHolderSuffix = ".holder"
	lockHolderLimit  = 4096
)

// Holder states a diagnosis can reach.
const (
	HolderUnknown    = "unknown"    // no trustworthy record matches the real holder
	HolderUnverified = "unverified" // the record could not be checked on this platform
	HolderCurrent    = "current"    // verified holder that still owns live work
	HolderStale      = "stale"      // verified holder whose recorded role child has exited
)

// LockHolder is the owner record written beside a held lock. It is diagnostic
// evidence only: every field is re-checked against the live system before use.
type LockHolder struct {
	Schema     string    `json:"schema"`
	PID        int       `json:"pid"`
	Start      string    `json:"start"`
	Command    string    `json:"command"`
	Lock       string    `json:"lock"`
	Cell       string    `json:"cell,omitempty"`
	Role       string    `json:"role,omitempty"`
	Acquired   time.Time `json:"acquired"`
	ChildPID   int       `json:"child_pid,omitempty"`
	ChildStart string    `json:"child_start,omitempty"`
}

// Diagnosis is a waiter's verdict on a busy lock's holder.
type Diagnosis struct {
	Record   *LockHolder // nil when no well-formed record was readable
	State    string      // one of the Holder* states
	Reason   string      // the evidence behind State, in words
	Eligible bool        // stale AND provably the caller's own cellctl for this lock
}

// LockSpec says how a caller waits for, identifies and recovers one lock.
type LockSpec struct {
	Name string // operator-facing lock name, e.g. "cell scratch sweep"
	Cell string // cell that owns the lock; empty means "never eligible for recovery"
	Role string // role that owns the lock, when the lock is per role
	// Wait bounds the retry-with-backoff loop for transient collisions.
	Wait time.Duration
	// Probe returns ErrLockBusy at the first busy read, without waiting or
	// diagnosing. Status probes use it; launches never do.
	Probe bool
	// EndStale is the caller's explicit opt-in to SIGTERM a stale, eligible holder.
	EndStale bool
	// OptIn names the flag that sets EndStale, quoted in the "action available" text.
	OptIn string
	// Grace separates the two stale readings required before a signal (default 2s).
	Grace time.Duration
	// TermWait bounds the wait for a signalled holder to exit (default 10s).
	TermWait time.Duration
	// Command is the only process name eligible for recovery (default "cellctl").
	Command string
	// Log receives one line per wait that succeeded late and per recovery step.
	Log io.Writer
}

func (s LockSpec) command() string {
	if s.Command == "" {
		return "cellctl"
	}
	return s.Command
}

// BusyError is returned when a lock stays busy. It names the lock, its path, the
// bounded wait, the holder (or "unknown"), whether the holder is current and the
// action taken or available. errors.Is(err, ErrLockBusy) holds.
type BusyError struct {
	Lock      string
	Path      string
	Waited    time.Duration
	Diagnosis Diagnosis
	Action    string
}

func (e *BusyError) Unwrap() error { return ErrLockBusy }

func (e *BusyError) Error() string {
	current := map[string]string{
		HolderCurrent:    "yes",
		HolderStale:      "no (stale)",
		HolderUnverified: "could not check",
		HolderUnknown:    "unknown",
	}[e.Diagnosis.State]
	return fmt.Sprintf("lock %q (%s) is still busy after a bounded %s wait (retryable); holder: %s; holder current: %s — %s; action: %s",
		e.Lock, e.Path, e.Waited.Round(time.Millisecond), e.Diagnosis.holder(), current, e.Diagnosis.Reason, e.Action)
}

func (d Diagnosis) holder() string {
	r := d.Record
	if r == nil || d.State == HolderUnknown {
		return "unknown"
	}
	s := fmt.Sprintf("pid %d (%s", r.PID, r.Command)
	if r.Cell != "" {
		s += ", cell " + strconv.Quote(r.Cell)
	}
	if r.Role != "" {
		s += ", role " + strconv.Quote(r.Role)
	}
	if !r.Acquired.IsZero() {
		s += ", holding since " + r.Acquired.UTC().Format(time.RFC3339)
	}
	return s + ")"
}

// HeldLock is a lock this process holds, together with its owner record.
type HeldLock struct {
	mu   sync.Mutex
	root *os.Root
	name string
	f    *os.File
	rec  LockHolder
}

// File is the locked file.
func (h *HeldLock) File() *os.File { return h.f }

// RecordChild names the role child this holder is serving (0 clears it). A
// verified holder whose recorded child has exited is what "stale" means.
func (h *HeldLock) RecordChild(pid int) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.f == nil {
		return errors.New("lock already released")
	}
	h.rec.ChildPID, h.rec.ChildStart = 0, ""
	if pid > 0 {
		start, err := procStart(pid)
		if err != nil {
			return fmt.Errorf("child pid %d: %w", pid, err)
		}
		h.rec.ChildPID, h.rec.ChildStart = pid, start
	}
	return writeLockHolder(h.root, h.name, h.rec)
}

// Release removes the owner record, then unlocks and closes the file.
func (h *HeldLock) Release() error {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.f == nil {
		return nil
	}
	var rmErr error
	if err := h.root.Remove(h.name + lockHolderSuffix); err != nil && !os.IsNotExist(err) {
		rmErr = err
	}
	err := errors.Join(rmErr, UnlockFile(h.f), h.f.Close())
	h.f = nil
	return err
}

// AcquireHeld takes the advisory lock on f (opened by the caller as root/name),
// retrying a busy lock with exponential backoff for spec.Wait. On success it
// writes the owner record and returns the held lock; f then belongs to it. On a
// lasting busy read it diagnoses the holder and returns a *BusyError, recovering
// a stale holder first only under spec.EndStale. f is closed on every failure.
func AcquireHeld(root *os.Root, name string, f *os.File, spec LockSpec) (*HeldLock, error) {
	began := time.Now()
	deadline := began.Add(spec.Wait)
	delay := 10 * time.Millisecond
	var observed *LockHolder
	for {
		err := TryLockExclusive(f)
		if err == nil {
			if observed != nil && spec.Log != nil && time.Since(began) >= time.Second {
				fmt.Fprintf(spec.Log, "deskkit: lock %q (%s) taken after a %s wait; it was held by pid %d (%s)\n",
					spec.Name, lockPath(f), time.Since(began).Round(time.Millisecond), observed.PID, observed.Command)
			}
			return hold(root, name, f, spec), nil
		}
		if !errors.Is(err, ErrLockBusy) {
			f.Close()
			return nil, err
		}
		if spec.Probe {
			f.Close()
			return nil, ErrLockBusy
		}
		if observed == nil {
			observed, _ = readLockHolder(root, name)
		}
		now := time.Now()
		if !now.Before(deadline) {
			break
		}
		sleep := delay
		if rest := deadline.Sub(now); sleep > rest {
			sleep = rest
		}
		time.Sleep(sleep)
		if delay *= 2; delay > 500*time.Millisecond {
			delay = 500 * time.Millisecond
		}
	}
	busy := &BusyError{Lock: spec.Name, Path: lockPath(f), Waited: time.Since(began)}
	busy.Diagnosis = Diagnose(root, name, f, spec)
	busy.Action = actionAvailable(busy.Diagnosis, spec)
	if busy.Diagnosis.Eligible && spec.EndStale {
		if h, err := recoverStale(root, name, f, spec, busy); h != nil || err != nil {
			return h, err
		}
	}
	f.Close()
	return nil, busy
}

func hold(root *os.Root, name string, f *os.File, spec LockSpec) *HeldLock {
	rec := LockHolder{Schema: lockHolderSchema, PID: os.Getpid(), Lock: spec.Name, Cell: spec.Cell, Role: spec.Role, Acquired: time.Now().UTC()}
	rec.Start, _ = procStart(rec.PID)
	if c, err := procCommand(rec.PID); err == nil {
		rec.Command = c
	} else {
		rec.Command = commandName(os.Args[0])
	}
	// The record is evidence for waiters, never custody: a failed write leaves a
	// holder that diagnoses as "unknown", which is the safe reading.
	_ = writeLockHolder(root, name, rec)
	return &HeldLock{root: root, name: name, f: f, rec: rec}
}

func lockPath(f *os.File) string {
	if p, err := filepath.Abs(f.Name()); err == nil {
		return p
	}
	return f.Name()
}

func actionAvailable(d Diagnosis, spec LockSpec) string {
	switch {
	case d.State == HolderCurrent:
		return "none — a current holder is a correct refusal; retry after it finishes"
	case d.Eligible && spec.OptIn != "":
		return fmt.Sprintf("none taken — rerun with %s to send SIGTERM to stale pid %d (bounded wait, then the lock is re-checked; SIGKILL is never sent)", spec.OptIn, d.Record.PID)
	case d.Eligible:
		return "none — this caller has no recovery path; retry, or end the stale holder yourself"
	case d.State == HolderStale:
		return "none — the stale holder is not this cell's own cellctl for this lock, so it is reported, never signalled"
	default:
		return "none — an unidentified holder is reported, never signalled; retry, or inspect the lock yourself"
	}
}

func recoverStale(root *os.Root, name string, f *os.File, spec LockSpec, busy *BusyError) (*HeldLock, error) {
	grace, termWait := spec.Grace, spec.TermWait
	if grace <= 0 {
		grace = 2 * time.Second
	}
	if termWait <= 0 {
		termWait = 10 * time.Second
	}
	first := busy.Diagnosis
	time.Sleep(grace)
	if TryLockExclusive(f) == nil {
		return hold(root, name, f, spec), nil
	}
	again := Diagnose(root, name, f, spec)
	if !again.Eligible || again.Record.PID != first.Record.PID || again.Record.Start != first.Record.Start || again.Record.ChildPID != first.Record.ChildPID {
		busy.Diagnosis = again
		busy.Action = "none — the holder changed between the two stale readings, so nothing was signalled"
		return nil, nil
	}
	pid := again.Record.PID
	if spec.Log != nil {
		fmt.Fprintf(spec.Log, "deskkit: lock %q (%s): sending SIGTERM to stale holder pid %d — %s\n", spec.Name, busy.Path, pid, again.Reason)
	}
	if err := procTerminate(pid); err != nil {
		busy.Action = fmt.Sprintf("SIGTERM to stale pid %d failed: %v; nothing else was sent", pid, err)
		return nil, nil
	}
	exited := false
	until := time.Now().Add(termWait)
	for {
		if TryLockExclusive(f) == nil {
			if spec.Log != nil {
				fmt.Fprintf(spec.Log, "deskkit: lock %q: stale holder pid %d ended; lock taken\n", spec.Name, pid)
			}
			return hold(root, name, f, spec), nil
		}
		if start, err := procStart(pid); errors.Is(err, errProcessGone) || err == nil && start != again.Record.Start {
			exited = true
		}
		if !time.Now().Before(until) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	busy.Diagnosis = Diagnose(root, name, f, spec)
	if exited {
		busy.Action = fmt.Sprintf("sent SIGTERM to stale pid %d; it exited but the lock is still busy — not escalated", pid)
	} else {
		busy.Action = fmt.Sprintf("sent SIGTERM to stale pid %d; it did not exit within %s — not escalated (SIGKILL is never sent)", pid, termWait)
	}
	return nil, nil
}

// Diagnose reads the owner record of the lock root/name (f is the caller's own
// handle on the same file) and checks it against the live system.
func Diagnose(root *os.Root, name string, f *os.File, spec LockSpec) Diagnosis {
	rec, err := readLockHolder(root, name)
	if err != nil {
		return Diagnosis{State: HolderUnknown, Reason: "no readable owner record (" + err.Error() + ")"}
	}
	d := Diagnosis{Record: rec, State: HolderUnknown}
	start, err := procStart(rec.PID)
	switch {
	case errors.Is(err, errProcessGone):
		d.Reason = fmt.Sprintf("the owner record names pid %d, which is not running — the record is stale, the real holder is unknown, and the recorded pid is never acted on", rec.PID)
		return d
	case err != nil:
		d.State, d.Reason = HolderUnverified, fmt.Sprintf("could not read pid %d's start time: %v", rec.PID, err)
		return d
	case start != rec.Start:
		d.Reason = fmt.Sprintf("pid %d is running but is not the recorded process (start time %s, recorded %s: pid reuse) — the real holder is unknown and pid %d is never acted on", rec.PID, start, rec.Start, rec.PID)
		return d
	}
	cmd, err := procCommand(rec.PID)
	switch {
	case err != nil:
		d.State, d.Reason = HolderUnverified, fmt.Sprintf("could not read pid %d's command: %v", rec.PID, err)
		return d
	case cmd != rec.Command:
		d.Reason = fmt.Sprintf("pid %d now runs %q, not the recorded %q — the real holder is unknown", rec.PID, cmd, rec.Command)
		return d
	}
	holds, err := procHoldsFile(rec.PID, f)
	switch {
	case err != nil:
		d.State, d.Reason = HolderUnverified, fmt.Sprintf("pid %d is alive with a matching start time, but whether it holds the lock file could not be checked: %v", rec.PID, err)
		return d
	case !holds:
		d.Reason = fmt.Sprintf("pid %d is alive but does not have the lock file open — the real holder is unknown", rec.PID)
		return d
	}
	verified := fmt.Sprintf("pid %d is alive, its start time and command match the record and it holds the lock file", rec.PID)
	if rec.ChildPID <= 0 {
		d.State, d.Reason = HolderCurrent, verified+"; no role child is recorded (a short transaction, or between passes)"
		return d
	}
	childStart, err := procStart(rec.ChildPID)
	switch {
	case errors.Is(err, errProcessGone) || err == nil && childStart != rec.ChildStart:
		d.State, d.Reason = HolderStale, verified+fmt.Sprintf("; its recorded role child pid %d has exited, so it owns no live role session", rec.ChildPID)
	case err != nil:
		d.State, d.Reason = HolderUnverified, verified+fmt.Sprintf("; whether its role child pid %d is alive could not be checked: %v", rec.ChildPID, err)
	default:
		d.State, d.Reason = HolderCurrent, verified+fmt.Sprintf("; its role child pid %d is alive", rec.ChildPID)
	}
	d.Eligible = d.State == HolderStale && rec.Command == spec.command() && spec.Cell != "" &&
		rec.Cell == spec.Cell && rec.Role == spec.Role && rec.Lock == spec.Name && rec.PID != os.Getpid()
	return d
}

func readLockHolder(root *os.Root, name string) (*LockHolder, error) {
	p := name + lockHolderSuffix
	st, err := root.Lstat(p)
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() || st.Size() > lockHolderLimit {
		return nil, errors.New("owner record is not a small regular file")
	}
	fh, err := root.Open(p)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	b, err := io.ReadAll(io.LimitReader(fh, lockHolderLimit+1))
	if err != nil {
		return nil, err
	}
	var rec LockHolder
	if len(b) > lockHolderLimit || json.Unmarshal(b, &rec) != nil || rec.Schema != lockHolderSchema || rec.PID <= 0 || rec.Command == "" {
		return nil, errors.New("owner record is malformed")
	}
	return &rec, nil
}

func writeLockHolder(root *os.Root, name string, rec LockHolder) error {
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	tmp := fmt.Sprintf("%s%s.%d.tmp", name, lockHolderSuffix, os.Getpid())
	fh, err := root.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, err = fh.Write(b)
	if err = errors.Join(err, fh.Close()); err == nil {
		err = root.Rename(tmp, name+lockHolderSuffix)
	}
	if err != nil {
		_ = root.Remove(tmp)
	}
	return err
}
