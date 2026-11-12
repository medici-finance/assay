package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellcadence"
)

// `down` then `up` is a round trip (#6116). What down signals it also waits for, and what down
// writes the next launch can tell apart from what an operator or another tool wrote:
//   - an interactive owner holds its lease until its harness exits on the cockpit's hangup; down
//     names each one BEFORE it closes the cockpit (the lease holder's pid and start time), waits
//     for it with a bound, and, when the owner's cleanup could not prove itself finished, records
//     the clean stop only on the reconcile preflight's own dead evidence;
//   - a cadence supervisor is stopped with a STOP file whose body is downStopReason, and the
//     role's next launch removes exactly that body under the role lease.

// downStopReason is the one STOP body `cellctl down` writes. A STOP with any other body (the
// operator's `cadence stop`, another tool's empty marker) is never removed by a launch.
const downStopReason = "cellctl down\n"

// downStoppedSummary is the summary down records on an interactive checkpoint it finished.
const downStoppedSummary = "stopped by cellctl down"

// interactiveOwner is an interactive-mode role owner a teardown is about to signal. PID and
// Start come from the lease's owner record, read before the cockpit is closed; a zero PID is an
// owner whose record did not identify it: it is waited for, and never recorded as stopped.
type interactiveOwner struct {
	Role  string
	PID   int
	Start string
	// Sequence and LastStart name the checkpoint the owner was running when down read its lease
	// record, before anything was signalled. They are what binds an owner whose record is gone
	// to the checkpoint it left: a clean exit releases the lease, and that removes the owner
	// record and the start time in it (#6251). Zero is a checkpoint down did not read.
	Sequence  uint64
	LastStart time.Time
}

// interactiveOwners lists the interactive owners holding this cell's role leases right now.
func (c *Cell) interactiveOwners() ([]interactiveOwner, error) {
	if c.Kind != "house" {
		return nil, nil
	}
	var out []interactiveOwner
	for _, role := range knownRoles {
		o, ok, err := c.interactiveOwner(role)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, o)
		}
	}
	return out, nil
}

// interactiveOwner reads one role's owner record and probes its lease without taking it, so a
// crashed owner's evidence survives the read.
func (c *Cell) interactiveOwner(role string) (interactiveOwner, bool, error) {
	dir := c.cadenceDir(role)
	if _, err := os.Lstat(dir); errors.Is(err, os.ErrNotExist) {
		return interactiveOwner{}, false, nil
	}
	holder, holderErr := cellcadence.Holder(dir)
	busy, err := cellcadence.Probe(dir)
	if err != nil {
		return interactiveOwner{}, false, fmt.Errorf("%s: the role lease could not be probed: %w", role, err)
	}
	if !busy {
		return interactiveOwner{}, false, nil
	}
	mode, err := readRoleMode(dir)
	if err != nil {
		return interactiveOwner{}, false, fmt.Errorf("%s: role owner mode: %w", role, err)
	}
	if mode != "interactive" {
		return interactiveOwner{}, false, nil
	}
	o := interactiveOwner{Role: role}
	if holderErr == nil && holder != nil && holder.PID > 0 && holder.Start != "" &&
		holder.Cell == c.Name && holder.Role == role {
		o.PID, o.Start = holder.PID, holder.Start
		if s, err := cellcadence.Read(dir); err == nil && s.Mode == "interactive" && s.Running &&
			s.Cell == c.Name && s.Role == role && s.PID == holder.PID {
			o.Sequence, o.LastStart = s.Sequence, s.LastStart
		}
	}
	return o, true, nil
}

// settleInteractiveOwner waits, until deadline, for an interactive owner this teardown signalled
// to release its lease, then leaves its checkpoint finished. An owner whose harness finished its
// own cleanup has already recorded that. Otherwise the clean stop is recorded only when every
// one of these holds, and the record is left Running for `up --plan` / `cadence recover` when
// any does not by the deadline:
//   - the lease is free and the checkpoint is the Running interactive record of the very owner
//     (pid and start time) that was holding the lease before the cockpit was closed;
//   - the reconcile preflight classifies the record dead: the owner and its recorded role child
//     are gone by pid and start time, nothing is left in the child's process group, and no
//     process carrying the role's marker is still running, orphaned or not.
//
// A process that is still exiting (or not yet reaped) reads alive, so an unproven record is
// re-read until the deadline; a record some other owner wrote is refused at once.
func (c *Cell) settleInteractiveOwner(o interactiveOwner, deadline time.Time) error {
	dir := c.cadenceDir(o.Role)
	unproven := func(why string) error {
		return fmt.Errorf("%s: left unfinished (%s); cellctl up %s --plan shows the evidence", o.Role, why, c.Name)
	}
	for {
		why, err := c.settleOnce(o, dir)
		if err != nil {
			return unproven(err.Error())
		}
		if why == "" {
			return nil
		}
		if !time.Now().Before(deadline) {
			return unproven(why)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// settleOnce is one reading of settleInteractiveOwner. It returns "" when the role is settled, a
// reason to read again before the deadline, or an error that no later reading can change.
func (c *Cell) settleOnce(o interactiveOwner, dir string) (string, error) {
	busy, err := cellcadence.Probe(dir)
	if err != nil {
		return "the role lease could not be probed: " + err.Error(), nil
	}
	if busy {
		return "the interactive owner has not exited", nil
	}
	plan := c.reconcileInventory([]string{o.Role}, nil)
	var e *reconcileEntry
	for _, x := range plan.Entries {
		if x.Role == o.Role {
			e = x
		}
	}
	switch {
	case e == nil:
		return "", nil // the owner finished its own cleanup and recorded it
	case o.PID == 0:
		return "", errors.New("the owner record did not identify the owner down signalled")
	case e.HasState && (e.State.Mode != "interactive" || e.State.PID != o.PID) || e.Start != "" && e.Start != o.Start:
		return "", errors.New("the record is not the one the signalled owner wrote")
	case plan.ScanErr != nil:
		return "the cell's processes could not be listed: " + plan.ScanErr.Error(), nil
	case len(plan.Orphans) > 0 || len(plan.Live) > 0:
		return "a process carrying the role's marker is still running", nil
	case e.Class != reconcileDead:
		return "the record is " + e.Class + ", not proven dead", nil
	case !e.HasState || !c.recordIsOwners(o, e):
		return "", errors.New("the record is not the one the signalled owner wrote")
	}
	lease, err := cellcadence.Acquire(dir)
	if errors.Is(err, cellcadence.ErrBusy) {
		return "", errors.New("another owner took the role lease")
	}
	if err != nil {
		return "the role lease could not be taken: " + err.Error(), nil
	}
	defer lease.Close()
	if err := lease.RecordStopped(o.PID, downStoppedSummary); err != nil {
		return "", err
	}
	return "", nil
}

// recordIsOwners reports whether the role's checkpoint is the one o's owner wrote. While the owner
// record survives, that is its start time matching the one down read. A launch that exits
// releases its lease, and the release removes the owner record: nothing then carries a start
// time, and requiring one refused every owner that exited on the hangup (#6251). That owner is
// attributed by the checkpoint down read before it signalled anyone: the same interactive record
// (sequence and start), written by the same pid. The caller has already established that the
// checkpoint is Running, that the pid is gone (an unreadable start time is never dead), and that
// the lease is free, so a replacement owner's record (another sequence) or an unread one (zero)
// is never matched.
func (c *Cell) recordIsOwners(o interactiveOwner, e *reconcileEntry) bool {
	if e.Start != "" {
		return e.Start == o.Start
	}
	s := e.State
	return o.Sequence != 0 && s.Sequence == o.Sequence && s.PID == o.PID && s.LastStart.Equal(o.LastStart)
}

// settleInteractiveOwners settles each owner against one shared deadline, so the whole wait is
// bounded by wait however many interactive roles the cell runs. It returns one line per owner
// that is not settled.
func (c *Cell) settleInteractiveOwners(owners []interactiveOwner, wait time.Duration) []string {
	deadline := time.Now().Add(wait)
	var unsettled []string
	for _, o := range owners {
		if err := c.settleInteractiveOwner(o, deadline); err != nil {
			unsettled = append(unsettled, err.Error())
			continue
		}
		fmt.Printf("[cell] %s: interactive owner stopped\n", o.Role)
	}
	return unsettled
}

// writeDownStop asks a cadence supervisor to stop with downStopReason. A STOP that is already
// there is kept as it is: it stops the supervisor just the same, and overwriting it would turn an
// operator's stop into one the next launch removes.
func writeDownStop(dir string) error {
	f, err := os.OpenFile(filepath.Join(dir, "STOP"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if errors.Is(err, os.ErrExist) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = io.WriteString(f, downStopReason)
	return errors.Join(err, f.Close())
}

// clearDownStop removes the role's STOP file when, and only when, its body is downStopReason.
// The caller holds the role lease, so no supervisor of the role is reading it. The file is read
// through a no-follow open, and removed only while the path still names that same, unchanged
// file. It reports whether a STOP was removed.
func clearDownStop(dir string) (bool, error) {
	p := filepath.Join(dir, "STOP")
	st, err := os.Lstat(p)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !st.Mode().IsRegular() || st.Size() != int64(len(downStopReason)) {
		return false, nil
	}
	f, err := os.OpenFile(p, os.O_RDONLY|oNoFollow, 0)
	if err != nil {
		return false, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil {
		return false, err
	}
	b, err := io.ReadAll(io.LimitReader(f, int64(len(downStopReason))+1))
	if err != nil {
		return false, err
	}
	if string(b) != downStopReason || !os.SameFile(st, opened) {
		return false, nil
	}
	again, err := os.Lstat(p)
	if err != nil || !os.SameFile(again, opened) || again.Size() != opened.Size() || !again.ModTime().Equal(opened.ModTime()) {
		return false, nil
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	return true, nil
}
