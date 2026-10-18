package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/medici-finance/assay/tools/desk/internal/cellcadence"
	"github.com/medici-finance/assay/tools/desk/internal/cellop"
)

// `cellctl up` converges the cell (desk-containers/17). Before it composes any window it classifies
// each targeted role from the same probes the reconcile preflight (#2880) already trusts:
//
//   - up:     a live process holds the role's cadence lease. The role is left alone: no second launch,
//     no window closed, no window re-created.
//   - stale:  the lease is free but the checkpoint still says running, or a marked orphan is left. Plain
//     `up` never reconciles it; `up --plan` writes ONE cell-op plan for the whole cell and
//     `up --apply <plan-id>` clears it on a terminal, after the typed confirm.
//   - absent: nothing is running and nothing is left. The role is launched.
//
// Launches are serialized: the next role's window opens only once the previous role's launch has
// taken its lease and passed the scratch sweep (the lease record then names the role child), or
// has failed.

// The final per-role states of the up table.
const (
	upStateUp           = "up"
	upStateAlreadyUp    = "already-up"
	upStateReconciledUp = "reconciled+up"
	upStateFailed       = "failed"
)

// The classes a role is probed into.
const (
	probeUp     = "up"
	probeStale  = "stale"
	probeAbsent = "absent"
)

// upPoll is how often a launch waits on the lease record of the role it just opened.
var upPoll = 100 * time.Millisecond

// upDefaultLeaseWait bounds the wait for one role's launch to take its lease and start its child.
const upDefaultLeaseWait = 2 * time.Minute

// upProbe is one role's classification.
type upProbe struct {
	Role   string
	Kind   string // probeUp, probeStale or probeAbsent
	PID    int    // the live holder, when Kind is probeUp and a holder record names one
	Since  time.Time
	Entry  *reconcileEntry // the stale evidence, when there is a record
	Orphan bool            // a marked orphan of a free role was found
}

// class is the reconcile class that decides whether a stale role can be planned: only a proven-dead
// record (or orphans alone) is.
func (p upProbe) class() string {
	switch {
	case p.Entry != nil:
		return p.Entry.Class
	case p.Orphan:
		return reconcileDead
	}
	return "-"
}

func (p upProbe) planned() bool { return p.Kind == probeStale && p.class() == reconcileDead }

// probeUpRoles classifies roles. comms, when configured, is inventoried too: a stale comms record
// is reported through the returned plan, not as a role.
func (c *Cell) probeUpRoles(roles []string, comms *deskCommsConfig) ([]upProbe, *reconcilePlan) {
	plan := c.reconcileInventory(roles, comms)
	entries := map[string]*reconcileEntry{}
	for _, e := range plan.Entries {
		entries[e.Role] = e
	}
	orphans := map[string]bool{}
	for _, o := range plan.Orphans {
		orphans[o.Role] = true
	}
	var out []upProbe
	for _, role := range roles {
		p := upProbe{Role: role}
		busy, err := cellcadence.Probe(c.cadenceDir(role))
		switch {
		case err != nil:
			p.Kind = probeStale
			p.Entry = &reconcileEntry{Role: role, Class: reconcileCNC, Evidence: []string{"the lease could not be probed: " + err.Error()}}
		case busy:
			p.Kind = probeUp
			if h, herr := cellcadence.Holder(c.cadenceDir(role)); herr == nil && h != nil {
				p.PID, p.Since = h.PID, h.Acquired
			}
		case entries[role] != nil && entries[role].Class != reconcileOwned:
			p.Kind, p.Entry = probeStale, entries[role]
		case orphans[role]:
			p.Kind, p.Orphan = probeStale, true
		default:
			p.Kind = probeAbsent
		}
		out = append(out, p)
	}
	return out, plan
}

// commsStale reports a stale comms record in plan.
func commsStale(plan *reconcilePlan) *reconcileEntry {
	for _, e := range plan.Entries {
		if e.Role == "comms" && e.Class != reconcileOwned {
			return e
		}
	}
	return nil
}

// upRow is one line of the final table.
type upRow struct {
	Role   string
	State  string
	Detail string
	PID    int
}

func (r upRow) ok() bool { return r.State != upStateFailed }

// firstOutLine is the first non-empty line of s, verbatim.
func firstOutLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(l); t != "" {
			return t
		}
	}
	return ""
}

// upTargets is the role set `up` converges: every --role given (in the cell's own order), else the
// ROLES of cell.env as --no-the-desk / --with-the-desk shape them. A role the cell does not run is
// refused by name.
func (c *Cell) upTargets(requested []string, noTheDesk bool) []string {
	all := c.upRoles(noTheDesk)
	if len(requested) == 0 {
		return all
	}
	want := map[string]bool{}
	for _, r := range requested {
		if !valueIn(r, all) {
			die("up: --role %s is not a role of cell '%s' (roles: %s)", r, c.Name, strings.Join(all, " "))
		}
		want[r] = true
	}
	var out []string
	for _, r := range all {
		if want[r] {
			out = append(out, r)
		}
	}
	return out
}

// staleHint is the row detail for a stale role: the evidence, then the way out.
func (c *Cell) staleHint(p upProbe) string {
	why := "stale state from a previous session"
	if p.Entry != nil && len(p.Entry.Evidence) > 0 {
		why = p.Entry.Evidence[0]
	} else if p.Orphan {
		why = "a marked process of this role survived its window"
	}
	cmd := "cellctl up " + c.Name + " --plan"
	if p.class() != reconcileDead {
		cmd = "inspect it first (" + p.class() + "): cellctl up " + c.Name + " --plan names what can be cleared"
	}
	return "stale (" + why + "); not reconciled by up: " + cmd
}

// awaitRoleLease waits for the window just opened for role to take the role lease and start its
// child. The lease record names the child only after the launch passed lease acquisition and the
// scratch sweep, so that is the gate the next window waits on. A launch that took the lease and
// released it without a child failed; one that never took it within wait failed too.
func (c *Cell) awaitRoleLease(role string, wait time.Duration) error {
	dir := c.cadenceDir(role)
	deadline := time.Now().Add(wait)
	sawLease := false
	for {
		busy, err := cellcadence.Probe(dir)
		if err != nil {
			return fmt.Errorf("the role lease could not be probed: %v", err)
		}
		if busy {
			sawLease = true
			if h, herr := cellcadence.Holder(dir); herr == nil && h != nil && h.ChildPID > 0 {
				return nil
			}
		} else if sawLease {
			return errors.New("the role lease was released before a role child started: the launch failed after taking it")
		}
		if !time.Now().Before(deadline) {
			if sawLease {
				return fmt.Errorf("the role lease is held but no role child started within %s", wait)
			}
			return fmt.Errorf("no role lease was taken within %s: the window did not start the role", wait)
		}
		time.Sleep(upPoll)
	}
}

// convergeRoles brings every probed role to its final state, one launch at a time. open starts the
// role's window and returns the window's first error line. reconciled names the roles whose stale
// state was just cleared by an applied plan.
func (c *Cell) convergeRoles(probes []upProbe, reconciled map[string]bool, gate func(role string) error, open func(role string) error) []upRow {
	var rows []upRow
	for _, p := range probes {
		switch p.Kind {
		case probeUp:
			d := "a live process holds the role lease; left alone"
			if p.PID > 0 {
				d = "pid " + strconv.Itoa(p.PID)
				if !p.Since.IsZero() {
					d += " since " + p.Since.UTC().Format(time.RFC3339)
				}
			}
			rows = append(rows, upRow{Role: p.Role, State: upStateAlreadyUp, Detail: d, PID: p.PID})
		case probeStale:
			rows = append(rows, upRow{Role: p.Role, State: upStateFailed, Detail: c.staleHint(p)})
		default:
			if err := open(p.Role); err != nil {
				rows = append(rows, upRow{Role: p.Role, State: upStateFailed, Detail: firstOutLine(err.Error())})
				continue
			}
			if err := gate(p.Role); err != nil {
				rows = append(rows, upRow{Role: p.Role, State: upStateFailed, Detail: firstOutLine(err.Error())})
				continue
			}
			state := upStateUp
			if reconciled[p.Role] {
				state = upStateReconciledUp
			}
			rows = append(rows, upRow{Role: p.Role, State: state})
		}
	}
	return rows
}

// upDryRunLine is a role's line of the dry-run plan from the same probe a real run acts on. An
// absent role keeps the line it always had: the command its window would run.
func (c *Cell) upDryRunLine(p upProbe, cmd string) string {
	switch p.Kind {
	case probeUp:
		d := "already-up"
		if p.PID > 0 {
			d += " (pid " + strconv.Itoa(p.PID) + ")"
		}
		return "[dry-run] " + p.Role + ": " + d + "; left alone, no window opened"
	case probeStale:
		return "[dry-run] " + p.Role + ": " + c.staleHint(p)
	}
	return "[dry-run] " + p.Role + ": " + cmd
}

// printUpTable writes the final table, one row per targeted role.
func printUpTable(w io.Writer, rows []upRow) {
	width := len("ROLE")
	for _, r := range rows {
		if len(r.Role) > width {
			width = len(r.Role)
		}
	}
	fmt.Fprintf(w, "[up] %-*s  %-13s  %s\n", width, "ROLE", "STATE", "DETAIL")
	for _, r := range rows {
		fmt.Fprintf(w, "[up] %-*s  %-13s  %s\n", width, r.Role, r.State, r.Detail)
	}
}

func upRowsOK(rows []upRow) bool {
	for _, r := range rows {
		if !r.ok() {
			return false
		}
	}
	return true
}

// upResult is the cell-op-result for a launch run: one entry per targeted role.
func upResult(cell string, started time.Time, rows []upRow) cellop.Result {
	r := cellop.NewResult("up", cell, started)
	r.Finish(cellop.OK, "", "", time.Now())
	withRoles(&r, rows)
	return r
}

// --- the machine seam: up --plan, up --apply ---------------------------------------------------

func roleKey(role, what string) string { return "role/" + role + "/" + what }

const reconcilePrefix = "reconcile-"

// upActions is the ONE reconcile plan's action list for the whole cell: one action per stale role
// (and the comms service) whose record is proven dead. Alive and could-not-check records are never
// planned; they are named so the operator sees why.
func (c *Cell) upActions(probes []upProbe, comms *reconcileEntry) (actions []cellop.Action, notPlanned []string) {
	for _, p := range probes {
		if p.Kind != probeStale {
			continue
		}
		if !p.planned() {
			notPlanned = append(notPlanned, p.Role+": "+c.staleHint(p))
			continue
		}
		actions = append(actions, cellop.Action{
			ID: reconcilePrefix + p.Role, Cell: c.Name,
			Summary: "clear " + p.Role + "'s stale checkpoint and stop its marked orphans (never a live process)",
			Preconditions: []cellop.Precondition{
				{Key: roleKey(p.Role, "state"), Value: probeStale},
				{Key: roleKey(p.Role, "class"), Value: p.class()},
			},
		})
	}
	if comms != nil {
		if comms.Class == reconcileDead {
			actions = append(actions, cellop.Action{
				ID: reconcilePrefix + "comms", Cell: c.Name,
				Summary: "clear the comms service's stale checkpoint (never a live process)",
				Preconditions: []cellop.Precondition{
					{Key: roleKey("comms", "state"), Value: probeStale},
					{Key: roleKey("comms", "class"), Value: comms.Class},
				},
			})
		} else {
			notPlanned = append(notPlanned, "comms: "+comms.Class+": inspect it first")
		}
	}
	return actions, notPlanned
}

// upPlan is `up --plan [--json]`: one plan for the whole cell, or an ok result saying there is
// nothing stale to plan. It changes nothing but the plan store.
func upPlan(cmd *cobra.Command, c *Cell, roles []string) {
	if c.Kind != "house" {
		die("up: --plan is for house cells; '%s' is a %s cell", c.Name, c.Kind)
	}
	commsCfg, err := c.loadDeskComms()
	if err != nil {
		commsCfg = nil
	}
	probes, plan := c.probeUpRoles(roles, commsCfg)
	actions, notPlanned := c.upActions(probes, commsStale(plan))
	for _, n := range notPlanned {
		fmt.Fprintf(os.Stderr, "[up] not planned: %s\n", n)
	}
	if len(actions) == 0 {
		r := cellop.NewResult("up", c.Name, time.Now())
		detail := "no stale role to reconcile; cellctl up " + c.Name + " starts the roles that are not running"
		outcome := cellop.OK
		reason := ""
		if len(notPlanned) > 0 {
			outcome, reason = cellop.CouldNotCheck, cellop.ReasonUnmeasured
			detail = "nothing can be planned: " + strings.Join(notPlanned, "; ")
		}
		r.Finish(outcome, reason, detail, time.Now())
		emitResult(cmd, r)
		exitWith(r.ExitCode())
	}
	cellopPlan(cmd, c, "up", actions)
}

// upVerb is the verb the adapter applies a stored up plan with.
func upVerb(comms *deskCommsConfig, dry bool) cellopVerb {
	return cellopVerb{
		Verb: "up",
		Measure: func(c *Cell, a cellop.Action) ([]cellop.Precondition, error) {
			role := strings.TrimPrefix(a.ID, reconcilePrefix)
			if role == a.ID || role == "" {
				return nil, fmt.Errorf("action %s is not a reconcile action", a.ID)
			}
			if role == "comms" {
				if comms == nil {
					return nil, errors.New("comms is not configured for this cell")
				}
				plan := c.reconcileInventory(nil, comms)
				e := commsStale(plan)
				state, class := probeAbsent, "-"
				if e != nil {
					state, class = probeStale, e.Class
				}
				return []cellop.Precondition{{Key: roleKey(role, "state"), Value: state}, {Key: roleKey(role, "class"), Value: class}}, nil
			}
			probes, _ := c.probeUpRoles([]string{role}, nil)
			p := probes[0]
			return []cellop.Precondition{{Key: roleKey(role, "state"), Value: p.Kind}, {Key: roleKey(role, "class"), Value: p.class()}}, nil
		},
		Run: func(c *Cell, a cellop.Action) (string, error) {
			if dry {
				return "dry run: stale state left in place", nil
			}
			role := strings.TrimPrefix(a.ID, reconcilePrefix)
			var plan *reconcilePlan
			var roles []string
			var cfg *deskCommsConfig
			if role == "comms" {
				cfg = comms
				plan = c.reconcileInventory(nil, cfg)
			} else {
				roles = []string{role}
				plan = c.reconcileInventory(roles, nil)
			}
			if why := plan.blocked(); why != "" {
				return "", errors.New(why)
			}
			if err := c.reconcileApply(plan, roles, cfg); err != nil {
				return "", err
			}
			return "stale state cleared", nil
		},
	}
}

// reconciledRoles are the roles whose reconcile action completed in an applied result.
func reconciledRoles(r cellop.Result) map[string]bool {
	out := map[string]bool{}
	for _, a := range r.Actions {
		if a.Outcome == cellop.ActionOK && strings.HasPrefix(a.ID, reconcilePrefix) {
			out[strings.TrimPrefix(a.ID, reconcilePrefix)] = true
		}
	}
	return out
}

// withRoles folds the table into a result: one entry per targeted role, and the outcome lowered to
// partial when a role failed after the plan was applied.
func withRoles(r *cellop.Result, rows []upRow) {
	for _, row := range rows {
		state := row.State
		fields := map[string]*string{}
		if row.Detail != "" {
			d := row.Detail
			fields["detail"] = &d
		}
		if row.PID > 0 {
			p := strconv.Itoa(row.PID)
			fields["pid"] = &p
		}
		r.Roles = append(r.Roles, cellop.Role{Role: row.Role, State: &state, Fields: fields})
	}
	if !upRowsOK(rows) && r.Outcome == cellop.OK {
		failed := 0
		for _, row := range rows {
			if !row.ok() {
				failed++
			}
		}
		// Partial once anything completed (an applied action, or a role that came up); failed otherwise.
		o := cellop.Failed
		if len(r.Actions) > 0 || failed < len(rows) {
			o = cellop.Partial
		}
		r.Finish(o, cellop.ReasonActionFailed, fmt.Sprintf("%d of %d roles failed after the plan was applied", failed, len(rows)), time.Now())
	}
}
