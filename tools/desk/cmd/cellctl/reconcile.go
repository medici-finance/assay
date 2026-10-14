package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellcadence"
	"github.com/medici-finance/assay/tools/desk/internal/cellstop"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The reconcile preflight (`cellctl up`, and `cellctl desk` for its one role) turns the stale
// state a hand-closed or crashed session leaves behind into ONE inventory and, on a terminal,
// ONE question — instead of a refusal per role and a by-hand recovery.
//
// Evidence, never a pid alone:
//   - A role lease that a live process holds is "owned": a running window. It is listed for
//     context and never touched.
//   - With the lease free, the supervisor and role child named by the lease's surviving owner
//     record are each checked by pid AND start time. A pid that is gone, or that now belongs to a
//     process with a different start time (pid reuse), is evidence of death; the reused pid is
//     never signalled. A match is "alive". Anything unmeasured is "could-not-check", which is
//     never treated as dead.
//   - A process is an orphan candidate only when it carries this cell's marker (ASSAY_CELL,
//     ASSAY_CELL_DIR, ASSAY_CELL_ROLE, stamped on every child cellctl launches) for a role whose
//     lease is free, and it was re-parented to pid 1 (or descends, through marked parents, from
//     one that was). Paths and command lines are never ownership proof.
//
// Any alive or could-not-check record, a non-terminal stdin, or any answer but yes refuses as
// before — with the full inventory and the exact commands, rather than the first refusal hit.
// There is deliberately no --yes flag: clearing stays a human act, as --confirm-stopped intends.

const (
	reconcileOwned = "owned"
	reconcileDead  = "dead"
	reconcileAlive = "alive"
	reconcileCNC   = "could-not-check"
)

// orphanGrace is how long a SIGTERMed orphan has before SIGKILL.
var orphanGrace = 5 * time.Second

type reconcileEntry struct {
	Role     string // a role name, or "comms"
	Class    string
	State    cellcadence.State
	HasState bool
	Start    string // the recorded supervisor start time, when an owner record survived
	PIDs     []int  // recorded processes that are still alive: never signalled
	Evidence []string
	Surface  *commsSurface
}

type reconcilePlan struct {
	Entries   []*reconcileEntry
	Orphans   []cellstop.Marked
	Live      []cellstop.Marked // marked processes that are not orphans: an owned child is alive
	ScanErr   error             // the orphan scan could not run: could-not-check
	scanned   bool
	commsCfg  *deskCommsConfig
	protected []int
}

type reconcileIO struct {
	DryRun bool
	TTY    bool
	In     io.Reader
	Out    io.Writer
}

func (c *Cell) cellMarker() cellstop.Marker { return cellstop.Marker{Cell: c.Name, Dir: c.Dir} }

// stampCellMarker sets this cell's marker for role on a child environment, replacing any marker
// the launching shell carried (a cellctl run from inside another role inherits that role's).
func (c *Cell) stampCellMarker(env []string, role string) []string {
	for _, kv := range c.cellMarker().Env(role) {
		k, v, _ := strings.Cut(kv, "=")
		env = envSet(env, k, v)
	}
	return env
}

// reconcilePreflight runs the preflight for a launch and dies on refusal. verb names the
// refusal ("up" or "desk"); refusal is the line the launch has always died with.
func (c *Cell) reconcilePreflight(verb string, roles []string, comms *deskCommsConfig, dryRun bool, refusal string) {
	err := c.reconcile(roles, comms, reconcileIO{DryRun: dryRun, TTY: isTTY(os.Stdin) && isTTY(os.Stdout), In: os.Stdin, Out: os.Stdout})
	if err != nil {
		if refusal != "" {
			die("%s", refusal)
		}
		die("%s: %v", verb, err)
	}
}

var errReconcileRefused = errors.New("stale cell state; reconcile before restarting (inventory and commands above)")

func (c *Cell) reconcile(roles []string, comms *deskCommsConfig, rio reconcileIO) error {
	if c.Kind != "house" {
		return nil
	}
	plan := c.reconcileInventory(roles, comms)
	if !plan.actionable() {
		return nil
	}
	plan.print(rio.Out, c)
	if rio.DryRun {
		fmt.Fprintln(rio.Out, "[reconcile] dry run: nothing was changed")
		return nil
	}
	// Orphans alone never blocked a launch before this preflight existed, so declining to stop
	// them (or having no terminal to ask on) keeps launching, as it always did.
	refusal := errReconcileRefused
	if !plan.staleRecords() {
		refusal = nil
	}
	if why := plan.blocked(); why != "" {
		fmt.Fprintf(rio.Out, "[reconcile] refusing to clear anything: %s\n", why)
		plan.printCommands(rio.Out, c)
		return errReconcileRefused
	}
	if !rio.TTY {
		fmt.Fprintln(rio.Out, "[reconcile] not a terminal: nothing is cleared without a human answer")
		plan.printCommands(rio.Out, c)
		return refusal
	}
	fmt.Fprintf(rio.Out, "Clear stale state and stop %d orphaned processes? [y/N] ", len(plan.Orphans))
	if !readYes(rio.In) {
		fmt.Fprintln(rio.Out, "[reconcile] not confirmed: nothing was changed")
		plan.printCommands(rio.Out, c)
		return refusal
	}
	if err := c.reconcileApply(plan, roles, comms); err != nil {
		fmt.Fprintf(rio.Out, "[reconcile] stopped: %v\n", err)
		after := c.reconcileInventory(roles, comms)
		if after.actionable() {
			after.print(rio.Out, c)
			after.printCommands(rio.Out, c)
		}
		return errReconcileRefused
	}
	fmt.Fprintln(rio.Out, "[reconcile] stale state cleared; continuing")
	return nil
}

// readYes reads one answer line byte by byte, so nothing past the newline is consumed from a
// stdin that the launch may hand on (a tmux attach).
func readYes(in io.Reader) bool {
	if in == nil {
		return false
	}
	var line []byte
	b := make([]byte, 1)
	for len(line) < 64 {
		n, err := in.Read(b)
		if n == 1 {
			if b[0] == '\n' {
				break
			}
			line = append(line, b[0])
		}
		if err != nil {
			break
		}
	}
	a := strings.ToLower(strings.TrimSpace(string(line)))
	return a == "y" || a == "yes"
}

func (p *reconcilePlan) actionable() bool {
	if len(p.Orphans) > 0 {
		return true
	}
	for _, e := range p.Entries {
		if e.Class != reconcileOwned {
			return true
		}
	}
	return false
}

// staleRecords reports a record a launch would refuse on (anything but a live owner).
func (p *reconcilePlan) staleRecords() bool {
	for _, e := range p.Entries {
		if e.Class != reconcileOwned {
			return true
		}
	}
	return false
}

// blocked names why nothing may be cleared, or "" when every stale record is proven dead.
func (p *reconcilePlan) blocked() string {
	var why []string
	for _, e := range p.Entries {
		switch e.Class {
		case reconcileAlive:
			why = append(why, e.Role+" is still running (alive means refuse, never kill)")
		case reconcileCNC:
			why = append(why, e.Role+" could not be checked (never treated as dead)")
		}
	}
	if p.ScanErr != nil {
		why = append(why, "the cell's processes could not be listed: "+p.ScanErr.Error())
	}
	return strings.Join(why, "; ")
}

func (c *Cell) reconcileInventory(roles []string, comms *deskCommsConfig) *reconcilePlan {
	plan := &reconcilePlan{commsCfg: comms}
	eligible := map[string]*reconcileEntry{} // roles whose lease is free
	inventoried := map[string]bool{}
	add := func(role, dir string, isComms bool) {
		inventoried[role] = true
		e, free := c.inventoryRecord(role, dir, isComms)
		if e != nil {
			plan.Entries = append(plan.Entries, e)
		}
		if free {
			eligible[role] = e
		}
	}
	for _, role := range roles {
		add(role, c.cadenceDir(role), false)
	}
	if comms != nil {
		add("comms", c.commsDir(), true)
	}
	plan.protected = c.liveLeaseHolders()
	marked, err := cellstop.MarkedProcesses(c.cellMarker(), plan.protected)
	if err != nil {
		// Could-not-check matters only when there is stale state to clear: a clean launch was
		// never gated on a process listing and is not now.
		for _, e := range plan.Entries {
			if e.Class != reconcileOwned {
				plan.ScanErr = err
				break
			}
		}
		return plan
	}
	plan.scanned = true
	recorded := map[int]bool{}
	for _, e := range plan.Entries {
		for _, pid := range e.PIDs {
			recorded[pid] = true
		}
	}
	for _, m := range marked {
		if !inventoried[m.Role] || recorded[m.PID] {
			continue
		}
		e, free := eligible[m.Role]
		if !free {
			continue // a live window owns the role: its processes are its own business
		}
		if m.Orphan {
			plan.Orphans = append(plan.Orphans, m)
			continue
		}
		// A marked process whose parent is alive and not a cell orphan is an owned child still
		// running under something: the record cannot be dead, and it is never signalled.
		plan.Live = append(plan.Live, m)
		if e == nil {
			e = &reconcileEntry{Role: m.Role, Class: reconcileAlive}
			plan.Entries = append(plan.Entries, e)
			eligible[m.Role] = e
		}
		e.Class = reconcileAlive
		e.Evidence = append(e.Evidence, fmt.Sprintf("marked process pid %d (parent %d, %s) is still running and was not orphaned", m.PID, m.Parent, m.Identity))
	}
	return plan
}

// inventoryRecord classifies one role (or comms) record. free reports that no live process
// holds the lease. A nil entry means there is nothing stale to report.
func (c *Cell) inventoryRecord(role, dir string, isComms bool) (e *reconcileEntry, free bool) {
	if _, err := os.Lstat(dir); errors.Is(err, os.ErrNotExist) {
		return nil, true
	}
	// The owner record first: taking or even probing the lease the ordinary way rewrites it.
	holder, _ := cellcadence.Holder(dir)
	busy, probeErr := cellcadence.Probe(dir)
	state, stateErr := cellcadence.Read(dir)
	e = &reconcileEntry{Role: role}
	if stateErr == nil {
		e.State, e.HasState = state, true
	}
	switch {
	case probeErr != nil:
		e.Class = reconcileCNC
		e.Evidence = append(e.Evidence, "the lease could not be probed: "+probeErr.Error())
		return e, false
	case busy:
		if stateErr != nil {
			return nil, false
		}
		e.Class = reconcileOwned
		e.Evidence = append(e.Evidence, "a live process holds the lease (a running window); left alone")
		return e, false
	}
	if errors.Is(stateErr, cellcadence.ErrNoState) {
		return nil, true
	}
	if stateErr != nil {
		e.Class = reconcileCNC
		e.Evidence = append(e.Evidence, "the checkpoint is unreadable: "+stateErr.Error())
		return e, true
	}
	if state.Cell != c.Name || state.Role != role {
		e.Class = reconcileCNC
		e.Evidence = append(e.Evidence, "the checkpoint belongs to another cell or role")
		return e, true
	}
	stale := state.Running || isComms && state.Outcome != "ok"
	if !stale {
		return nil, true
	}
	if _, err := os.Lstat(c.commsLaunchDir()); isComms && err == nil {
		s, err := c.readCommsSurface()
		if err != nil {
			e.Class = reconcileCNC
			e.Evidence = append(e.Evidence, "the comms launch receipt is unreadable: "+err.Error())
			return e, true
		}
		e.Surface = s
		if s != nil && s.Handle == "" {
			e.Class = reconcileCNC
			e.Evidence = append(e.Evidence, "the comms surface "+s.Label+" has no recorded terminal identity, so it cannot be proven closed; close it by hand")
			return e, true
		}
	}
	alive, unknown := false, false
	check := func(what string, pid int, start string) {
		now, gone, err := deskkit.ProcessStart(pid)
		switch {
		case gone:
			e.Evidence = append(e.Evidence, fmt.Sprintf("%s pid %d has exited", what, pid))
		case err != nil:
			unknown = true
			e.Evidence = append(e.Evidence, fmt.Sprintf("%s pid %d could not be checked: %v", what, pid, err))
		case start == "":
			unknown = true
			e.Evidence = append(e.Evidence, fmt.Sprintf("%s pid %d is running and no start time was recorded to tell it from a reused pid", what, pid))
		case now != start:
			e.Evidence = append(e.Evidence, fmt.Sprintf("%s pid %d now belongs to a different process (start time differs: pid reuse); it is never signalled", what, pid))
		default:
			alive = true
			e.PIDs = append(e.PIDs, pid)
			e.Evidence = append(e.Evidence, fmt.Sprintf("%s pid %d is still running (pid and start time match)", what, pid))
		}
	}
	ownRecord := holder != nil && holder.PID == state.PID &&
		(holder.Cell == "" && holder.Role == "" || holder.Cell == c.Name && holder.Role == role)
	if ownRecord {
		e.Start = holder.Start
		check("supervisor", holder.PID, holder.Start)
		if holder.ChildPID > 0 {
			check("role child", holder.ChildPID, holder.ChildStart)
		}
	} else {
		e.Evidence = append(e.Evidence, "no owner record survived with a start time for the checkpoint's pid")
		check("supervisor", state.PID, "")
	}
	switch {
	case alive:
		e.Class = reconcileAlive
	case unknown:
		e.Class = reconcileCNC
	default:
		e.Class = reconcileDead
	}
	return e, true
}

// liveLeaseHolders is every verified live holder of any of this cell's leases (every role, the
// comms service and its launch receipt). Their process trees are never orphan candidates, even
// when they inherited another role's marker (a cellctl launched from inside a role window).
func (c *Cell) liveLeaseHolders() []int {
	dirs := []string{c.commsDir(), c.commsLaunchDir()}
	for _, r := range knownRoles {
		dirs = append(dirs, c.cadenceDir(r))
	}
	var pids []int
	for _, d := range dirs {
		busy, err := cellcadence.Probe(d)
		if err != nil || !busy {
			continue
		}
		if h, err := cellcadence.Holder(d); err == nil {
			if start, gone, err := deskkit.ProcessStart(h.PID); err == nil && !gone && start == h.Start {
				pids = append(pids, h.PID)
			}
		}
	}
	return pids
}

func (c *Cell) reconcileApply(plan *reconcilePlan, roles []string, comms *deskCommsConfig) error {
	m := c.cellMarker()
	for round := 0; ; round++ {
		if len(plan.Orphans) > 0 {
			// Re-prove each orphan's role lease is still free right before any signal.
			for _, o := range plan.Orphans {
				dir := c.cadenceDir(o.Role)
				if o.Role == "comms" {
					dir = c.commsDir()
				}
				if busy, err := cellcadence.Probe(dir); err != nil || busy {
					return fmt.Errorf("role %s gained a live owner; nothing further was signalled", o.Role)
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), orphanGrace+15*time.Second)
			err := cellstop.StopMarked(ctx, m, plan.Orphans, orphanGrace)
			cancel()
			if err != nil {
				return err
			}
		}
		next := c.reconcileInventory(roles, comms)
		if why := next.blocked(); why != "" {
			return fmt.Errorf("after stopping orphans: %s", why)
		}
		if len(next.Orphans) == 0 {
			plan = next
			break
		}
		if round >= 1 {
			return fmt.Errorf("%d orphaned processes reappeared after two rounds", len(next.Orphans))
		}
		plan = next
	}
	for _, e := range plan.Entries {
		if e.Class != reconcileDead {
			continue
		}
		if e.Role == "comms" {
			if e.Surface != nil && e.Surface.Handle != "" {
				// The existing rule: a recorded surface is closed only if its identity still matches.
				if err := c.closeCommsSurface(e.Surface); err != nil {
					return fmt.Errorf("comms surface close: %w", err)
				}
			}
			if err := c.recoverComms(comms); err != nil {
				return err
			}
			fmt.Println("[reconcile] comms: recovery recorded")
			continue
		}
		if err := c.clearCadence(e.Role, true); err != nil {
			return err
		}
		fmt.Printf("[reconcile] %s: recovery recorded\n", e.Role)
	}
	if after := c.reconcileInventory(roles, comms); after.actionable() {
		return fmt.Errorf("stale state remains after recovery")
	}
	return nil
}

func (p *reconcilePlan) print(w io.Writer, c *Cell) {
	fmt.Fprintf(w, "[reconcile] cell=%s: state left by a previous session\n", c.Name)
	for _, e := range p.Entries {
		line := fmt.Sprintf("[reconcile] %s class=%s", e.Role, e.Class)
		if e.HasState {
			s := e.State
			rec := "unfinished"
			if !s.Running {
				rec = "failed(" + s.Outcome + ")"
			}
			start := e.Start
			if start == "" {
				start = "unrecorded"
			}
			line += fmt.Sprintf(" record=%s pid=%d start=%s heartbeat=%s", rec, s.PID, start, s.Heartbeat.UTC().Format(time.RFC3339))
		}
		if e.Surface != nil {
			line += fmt.Sprintf(" surface=%s:%s(%s)", e.Surface.Cockpit, orDefault(e.Surface.Handle, "unknown"), e.Surface.Label)
		}
		fmt.Fprintln(w, line)
		for _, ev := range e.Evidence {
			fmt.Fprintf(w, "[reconcile]   %s\n", ev)
		}
	}
	for _, o := range p.Orphans {
		fmt.Fprintf(w, "[reconcile] orphan pid=%d role=%s parent=%d started=%s\n", o.PID, o.Role, o.Parent, o.Identity)
	}
	if p.ScanErr != nil {
		fmt.Fprintf(w, "[reconcile] process listing could-not-check: %v\n", p.ScanErr)
	}
	var clear []string
	surfaces := 0
	for _, e := range p.Entries {
		if e.Class == reconcileDead {
			clear = append(clear, e.Role)
			if e.Surface != nil && e.Surface.Handle != "" {
				surfaces++
			}
		}
	}
	fmt.Fprintf(w, "[reconcile] plan: stop %d orphaned processes (SIGTERM, %s grace, SIGKILL, re-checked by pid and start time); close %d recorded surfaces; clear %d records (%s)\n",
		len(p.Orphans), orphanGrace, surfaces, len(clear), orDefault(strings.Join(clear, " "), "none"))
	fmt.Fprintln(w, "[reconcile] untouched: live lease holders, comms messages, keys, configuration, worktrees, branches and the Go cache")
	if len(p.Orphans) > 0 || len(clear) > 0 {
		root, name := c.reenter()
		sh := operatorShell()
		fmt.Fprintf(w, "[reconcile] Go cache consumer records are never cleared here: if `cellctl --cells-root %s cache %s status` names a process this run proved dead, recover it with `cache %s recover --confirm-stopped`\n",
			sh.quote(root), sh.quote(name), sh.quote(name))
	}
}

// printCommands is the by-hand recovery: every command the preflight would have run, exact.
func (p *reconcilePlan) printCommands(w io.Writer, c *Cell) {
	root, name := c.reenter()
	sh := operatorShell()
	prefix := "cellctl --cells-root " + sh.quote(root)
	fmt.Fprintln(w, "[reconcile] to recover by hand, first confirm every process named above has stopped, then run:")
	for _, o := range p.Orphans {
		fmt.Fprintf(w, "  kill -TERM %d    # orphan role=%s started %s — check it is still that process first\n", o.PID, o.Role, o.Identity)
	}
	for _, e := range p.Entries {
		if e.Class == reconcileOwned {
			continue
		}
		note := ""
		if len(e.PIDs) > 0 {
			note = fmt.Sprintf("    # only after pid %s has exited", joinInts(e.PIDs))
		}
		if e.Role == "comms" {
			fmt.Fprintf(w, "  %s comms %s recover --confirm-stopped%s\n", prefix, sh.quote(name), note)
			continue
		}
		fmt.Fprintf(w, "  %s cadence %s recover %s --confirm-stopped%s\n", prefix, sh.quote(name), e.Role, note)
	}
}

// operatorShell is the shell the operator reading a refusal types the recovery commands into.
func operatorShell() paneShell {
	if runtime.GOOS == "windows" {
		return shellPowerShell
	}
	return shellPOSIX
}

func joinInts(v []int) string {
	s := make([]string, len(v))
	for i, n := range v {
		s[i] = fmt.Sprint(n)
	}
	return strings.Join(s, ", ")
}
