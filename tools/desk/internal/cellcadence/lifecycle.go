package cellcadence

import (
	"errors"
	"fmt"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// This file is the pure local lifecycle model of a cell role: the judgments up, reconcile, down
// and settle make about whether a recorded role owner is the process now running, whether a
// checkpoint is the one that owner wrote, and whether a stale role can be planned. Every input
// is an explicit observation the caller measured; every output is a typed decision. Nothing
// here reads a file, a process table or a lock, signals a process, or writes a checkpoint: those
// effects stay at their adapters (cmd/cellctl, cellstop, and Lease.RecordStopped for the one
// checkpoint writer a teardown uses), and each adapter keeps its own check at its own boundary.
//
// An unobservable input is never a clean one: an unreadable process start, an unprobed lease or
// an unreadable checkpoint is could-not-check and is never read as dead, free or owned.

// The classes a role record is judged into. They are the values the reconcile report prints and
// the values an `up --plan` plan records as the role's class precondition.
const (
	ClassOwned         = "owned"           // a live process holds the lease: a running window
	ClassDead          = "dead"            // every recorded process is gone: the record can be cleared
	ClassAlive         = "alive"           // a recorded process still runs: it is never signalled
	ClassCouldNotCheck = "could-not-check" // an observation was absent or unreadable
)

// ProcessProbe is one observation of a pid in the process table: its start time as
// deskkit.ProcessStart reports it, whether the pid names no process at all, or the error that made
// the observation impossible.
type ProcessProbe struct {
	Start string
	Gone  bool
	Err   error
}

// ProcessVerdict is what one probe says about one recorded process.
type ProcessVerdict int

const (
	// ProcessGone: the pid names no running process.
	ProcessGone ProcessVerdict = iota + 1
	// ProcessUnreadable: the process table could not be read; could-not-check, never gone.
	ProcessUnreadable
	// ProcessNoRecordedStart: a process runs at the pid, but the record holds no start time to
	// tell it from a reused pid; could-not-check, never the same process and never gone.
	ProcessNoRecordedStart
	// ProcessReused: the pid now belongs to a process with a different start time. The recorded
	// process is gone, and the pid is never signalled.
	ProcessReused
	// ProcessSame: the pid and the start time both match the record.
	ProcessSame
)

// JudgeProcess compares a recorded start time with a probe of the same pid. A pid alone never
// identifies a process: only the pair does. The order is the order evidence is trusted in: gone,
// unreadable, no recorded start, reused, same.
// semantic: S-cell-lifecycle
func JudgeProcess(recordedStart string, p ProcessProbe) ProcessVerdict {
	switch {
	case p.Gone:
		return ProcessGone
	case p.Err != nil:
		return ProcessUnreadable
	case recordedStart == "":
		return ProcessNoRecordedStart
	case p.Start != recordedStart:
		return ProcessReused
	}
	return ProcessSame
}

// Alive reports that the recorded process itself is still running.
func (v ProcessVerdict) Alive() bool { return v == ProcessSame }

// Unknown reports that the verdict is could-not-check.
func (v ProcessVerdict) Unknown() bool { return v == ProcessUnreadable || v == ProcessNoRecordedStart }

// Departed reports that the recorded process is proven not running: gone, or its pid reused.
func (v ProcessVerdict) Departed() bool { return v == ProcessGone || v == ProcessReused }

// ClassifyProcesses is the class of a record whose processes were judged: any live process makes
// it alive, otherwise any unknown one makes it could-not-check, otherwise it is dead.
func ClassifyProcesses(vs ...ProcessVerdict) string {
	unknown := false
	for _, v := range vs {
		switch {
		case v.Alive():
			return ClassAlive
		case v.Unknown():
			unknown = true
		}
	}
	if unknown {
		return ClassCouldNotCheck
	}
	return ClassDead
}

// OwnsCheckpointPID reports whether the lease holder record is the one the checkpoint's pid
// wrote: the same pid, and either an unscoped legacy record or the same cell and role.
func OwnsCheckpointPID(holder *deskkit.LockHolder, s State, cell, role string) bool {
	return holder != nil && holder.PID == s.PID &&
		(holder.Cell == "" && holder.Role == "" || holder.Cell == cell && holder.Role == role)
}

// Generation names one owner generation of a role: the pid and start time the lease record held,
// and the checkpoint (Sequence and LastStart) that owner was running, both read before anything
// was signalled. A clean exit releases the lease, which removes the owner record and the start
// time in it (#6251), so the checkpoint is what binds that owner to the record it left. Zero
// Sequence is a checkpoint that was not read. A Generation naming nothing is not a generation.
type Generation struct {
	PID       int
	Start     string
	Sequence  uint64
	LastStart time.Time
}

// Named reports that g identifies an owner: a pid, and either its start time or its checkpoint.
func (g Generation) Named() bool { return g.PID > 0 && (g.Start != "" || g.Sequence != 0) }

// Attributes reports whether the checkpoint s, judged dead by a record whose surviving owner
// record carried start (empty when none survived), is the one g's owner wrote. While the owner
// record survives that is its start time matching. Once it is gone, the checkpoint down read
// before it signalled anyone: the same interactive record (sequence and start) written by the
// same pid. A replacement owner's record (another sequence) or an unread one (zero) never
// matches.
func (g Generation) Attributes(s State, start string) bool {
	if start != "" {
		return start == g.Start
	}
	return g.Sequence != 0 && s.Sequence == g.Sequence && s.PID == g.PID && s.LastStart.Equal(g.LastStart)
}

// OwnsUnfinished reports whether s is the unfinished interactive checkpoint of g's owner, and is
// the check at the one checkpoint writer a teardown uses. The generation must be named, the pid
// must match, and when the checkpoint was read (Sequence set) the sequence and start must match
// too, so a replacement owner's later record is never finished by a stale reading.
func (g Generation) OwnsUnfinished(s State) bool {
	if !g.Named() || s.Mode != "interactive" || !s.Running || s.PID != g.PID {
		return false
	}
	return g.Sequence == 0 || s.Sequence == g.Sequence && s.LastStart.Equal(g.LastStart)
}

// SettleRecord is the stale record a settle reading found for the role.
type SettleRecord struct {
	HasState bool
	State    State
	Start    string // the owner record's start time, empty when none survived
	Class    string
}

// SettleInput is one reading of an interactive owner a teardown signalled.
type SettleInput struct {
	Owner     Generation
	LeaseBusy bool
	LeaseErr  error // the lease could not be probed
	// Record is nil when no stale record remains for the role.
	Record *SettleRecord
	// ScanErr is the error that kept the cell's processes from being listed.
	ScanErr error
	// MarkedRunning: a process carrying the role's marker (orphaned or live) is still running.
	MarkedRunning bool
}

// SettleAction is what to do with a settle reading.
type SettleAction int

const (
	// SettleDone: the role is settled and nothing is written.
	SettleDone SettleAction = iota + 1
	// SettleRetry: the evidence is not yet conclusive; read again before the deadline.
	SettleRetry
	// SettleRefuse: no later reading can change this; leave the record unfinished.
	SettleRefuse
	// SettleRecordStopped: every proof holds; the caller may take the lease and record the stop.
	SettleRecordStopped
)

// SettleDecision is a settle action with the reason that is shown for any but SettleDone.
type SettleDecision struct {
	Action SettleAction
	Reason string
}

const notTheOwnersRecord = "the record is not the one the signalled owner wrote"

// JudgeSettle decides one reading of settleInteractiveOwner. The record may be finished as
// stopped only when the lease is free, the record is the very owner's (by start time or, once
// the owner record is gone, by checkpoint generation), the record is proven dead, and no process
// carrying the role's marker survives. A record some other owner wrote is refused at once; a
// record that is only unproven is read again.
func JudgeSettle(in SettleInput) SettleDecision {
	retry := func(why string) SettleDecision { return SettleDecision{SettleRetry, why} }
	refuse := func(why string) SettleDecision { return SettleDecision{SettleRefuse, why} }
	if in.LeaseErr != nil {
		return retry("the role lease could not be probed: " + in.LeaseErr.Error())
	}
	if in.LeaseBusy {
		return retry("the interactive owner has not exited")
	}
	e := in.Record
	switch {
	case e == nil:
		return SettleDecision{Action: SettleDone} // the owner finished its own cleanup and recorded it
	case in.Owner.PID == 0:
		return refuse("the owner record did not identify the owner down signalled")
	case e.HasState && (e.State.Mode != "interactive" || e.State.PID != in.Owner.PID) ||
		e.Start != "" && e.Start != in.Owner.Start:
		return refuse(notTheOwnersRecord)
	case in.ScanErr != nil:
		return retry("the cell's processes could not be listed: " + in.ScanErr.Error())
	case in.MarkedRunning:
		return retry("a process carrying the role's marker is still running")
	case e.Class != ClassDead:
		return retry("the record is " + e.Class + ", not proven dead")
	case !e.HasState || !in.Owner.Attributes(e.State, e.Start):
		return refuse(notTheOwnersRecord)
	}
	return SettleDecision{Action: SettleRecordStopped}
}

// The kinds a role is probed into by `up`.
const (
	KindUp     = "up"
	KindStale  = "stale"
	KindAbsent = "absent"
)

// RoleObservation is what `up` measured for one role.
type RoleObservation struct {
	Kind        string // KindUp, KindStale or KindAbsent
	HasRecord   bool   // a stale record was found
	RecordClass string // the stale record's class, when HasRecord
	Orphan      bool   // a marked orphan of a free role was found
}

// Class is the class that decides whether a stale role can be planned: only a proven-dead record
// (or orphans alone) is. "-" is a role with neither.
func (o RoleObservation) Class() string {
	switch {
	case o.HasRecord:
		return o.RecordClass
	case o.Orphan:
		return ClassDead
	}
	return "-"
}

// Plannable reports whether `up --plan` may plan this role's reconcile action.
func (o RoleObservation) Plannable() bool { return o.Kind == KindStale && o.Class() == ClassDead }

// Preconditions are the (state, class) pair a plan records for the role, and that `up --apply`
// compares against a fresh measurement before it acts.
func (o RoleObservation) Preconditions() (state, class string) { return o.Kind, o.Class() }

// ErrNotOwnersCheckpoint is the refusal of the checkpoint writer for a record the stopped owner
// did not write.
var ErrNotOwnersCheckpoint = errors.New("checkpoint is not the unfinished interactive record of the stopped owner")

// checkOwnsUnfinished is the writer-side refusal, with the generation named in the error.
func checkOwnsUnfinished(g Generation, s State) error {
	if g.OwnsUnfinished(s) {
		return nil
	}
	if !g.Named() {
		return fmt.Errorf("%w: the stopped owner's generation is not named", ErrNotOwnersCheckpoint)
	}
	return ErrNotOwnersCheckpoint
}
