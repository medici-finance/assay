package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/medici-finance/assay/tools/desk/internal/cellop"
	"github.com/medici-finance/assay/tools/desk/internal/cli"
)

// The `pause`, `resume` and `drain` verbs and the per-role `down <cell> <role...>`
// are adapters over the lifecycle interface (lifecycle.go) that adopt the
// cell-op v1 result and plan contract (spec/cell-op-v1.md). They name states and
// transitions; the interim mechanism behind them is lifecycle.go's alone.

const (
	drainPollMax            = 10 * time.Second
	drainDefaultTimeout     = 20 * time.Minute
	drainDefaultHandoffWait = 60 * time.Second
)

// drainPollCap is the drain's poll interval: a tenth of the bound, at most 10s.
func drainPollCap(bound time.Duration) time.Duration {
	p := bound / 10
	if p > drainPollMax {
		p = drainPollMax
	}
	if p < 10*time.Millisecond {
		p = 10 * time.Millisecond
	}
	return p
}

// lifecycleTargets is the roles a verb names, or every known role.
func lifecycleTargets(roles []string) []string {
	if len(roles) == 0 {
		return append([]string(nil), knownRoles...)
	}
	return roles
}

func checkRoles(verb string, roles []string) {
	for _, r := range roles {
		if !valueIn(r, knownRoles) {
			die("%s: unknown role %q (one of: %s)", verb, r, strings.Join(knownRoles, " "))
		}
	}
}

func roleEntry(role string, state lifeState, fields map[string]string) cellop.Role {
	s := string(state)
	f := map[string]*string{}
	for k, v := range fields {
		v := v
		f[k] = &v
	}
	return cellop.Role{Role: role, State: &s, Fields: f}
}

func actionResult(cell, id, outcome, detail string) cellop.ActionResult {
	r := cellop.ActionResult{ID: id, Cell: cell, Outcome: outcome}
	if detail != "" {
		r.Detail = &detail
	}
	return r
}

// finishErr folds a lifecycle error into a result: a refusal keeps its reason, anything else is
// a failure (partial when some action already ran).
func finishErr(r *cellop.Result, err error, now time.Time) {
	if reason, ok := refusalReason(err); ok {
		o := cellop.Refused
		if reason == cellop.ReasonUnmeasured {
			o = cellop.CouldNotCheck
		}
		if ranAny(r) {
			o = cellop.Partial
		}
		r.Finish(o, reason, err.Error(), now)
		return
	}
	o := cellop.Failed
	if ranAny(r) {
		o = cellop.Partial
	}
	r.Finish(o, cellop.ReasonActionFailed, err.Error(), now)
}

func ranAny(r *cellop.Result) bool {
	for _, a := range r.Actions {
		if a.Outcome == cellop.ActionOK {
			return true
		}
	}
	return false
}

// --- the drain, independent of the terminal ------------------------------------------------

type drainOpts struct {
	Roles       []string
	Timeout     time.Duration
	HandoffWait time.Duration
	// Owner, when set, stamps the drain's own pause flags with that invocation id and leaves
	// lifting them to the caller, which lifts only its own (a cycle). Empty: drain lifts the stops
	// of the roles it stopped itself, as `cellctl drain` does.
	Owner string
}

// drainReport is a drain's outcome before any operator confirm: the result so far, and the
// stop actions that still need the operator's confirm (an interactive role's could-not-check
// harness idleness is never accepted without it).
type drainReport struct {
	Result  cellop.Result
	Confirm []cellop.Action
	Since   time.Time
}

// drain is pause → wait (polling at most every 10s, bounded by the timeout) for each targeted
// role to measure idle → collect its handoff → stop it. A cadence role is stopped here; an
// interactive role is returned for the operator's confirm. Work still in flight at the timeout
// is listed and its role stays up and paused: nothing in flight is cancelled.
func (l *interimLifecycle) drain(o drainOpts) drainReport {
	since := l.now()
	cell := l.c.Name
	r := cellop.NewResult("drain", cell, since)
	rep := drainReport{Since: since}
	targets := lifecycleTargets(o.Roles)
	state := map[string]lifeState{}
	for _, role := range targets {
		state[role], _ = lifecycleStep(stateRunning, evDrain)
	}
	if _, err := l.pauseAs(o.Roles, o.Owner); err != nil {
		finishErr(&r, err, l.now())
		rep.Result = r
		return rep
	}
	r.Actions = append(r.Actions, actionResult(cell, "pause", cellop.ActionOK, "no new work from the next iteration boundary"))
	pending := append([]string(nil), targets...)
	last := map[string]idleReading{}
	deadline := since.Add(o.Timeout)
	for {
		var still []string
		for _, role := range pending {
			rd := l.Idle(role)
			last[role] = rd
			if rd.Err != nil {
				// An unreadable idle signal is never idle and does not get better by waiting:
				// the drain ends could-not-check, nothing is stopped, the roles stay paused.
				r.Actions = append(r.Actions, actionResult(cell, "wait:"+role, cellop.ActionNotRun, "idle could not be measured: "+rd.Err.Error()))
				r.Roles = drainRoles(targets, state, last)
				r.Finish(cellop.CouldNotCheck, cellop.ReasonUnmeasured, fmt.Sprintf("%s: idle could not be measured: %v", role, rd.Err), l.now())
				r.Next = []string{"cellctl resume " + cell}
				rep.Result, rep.Confirm = r, nil
				return rep
			}
			if !rd.measuredIdle() {
				still = append(still, role)
				continue
			}
			state[role], _ = lifecycleStep(state[role], evIdle)
			if rd.Mode != "down" {
				where, err := l.Handoff(role, since, o.HandoffWait, rd.Claims, false)
				if err != nil {
					r.Actions = append(r.Actions, actionResult(cell, "handoff:"+role, cellop.ActionFailed, err.Error()))
				} else {
					r.Actions = append(r.Actions, actionResult(cell, "handoff:"+role, cellop.ActionOK, where))
				}
			}
			if rd.HarnessUnmeasured {
				rep.Confirm = append(rep.Confirm, stopAction(cell, role, rd))
				continue
			}
			detail, err := l.Stop(role, false, false)
			if err != nil {
				r.Actions = append(r.Actions, actionResult(cell, "stop:"+role, cellop.ActionFailed, err.Error()))
				finishErr(&r, err, l.now())
				r.Roles = drainRoles(targets, state, last)
				rep.Result, rep.Confirm = r, nil
				return rep
			}
			state[role], _ = lifecycleStep(state[role], evStop)
			r.Actions = append(r.Actions, actionResult(cell, "stop:"+role, cellop.ActionOK, detail))
		}
		pending = still
		if len(pending) == 0 || !l.now().Before(deadline) {
			break
		}
		wait := drainPollCap(o.Timeout)
		if left := deadline.Sub(l.now()); left < wait {
			wait = left
		}
		l.probes.sleep(wait)
	}
	r.Roles = drainRoles(targets, state, last)
	if len(pending) > 0 {
		for _, role := range pending {
			r.Actions = append(r.Actions, actionResult(cell, "wait:"+role, cellop.ActionNotRun, "still in flight at the timeout, left running: "+last[role].String()))
		}
		for _, a := range rep.Confirm {
			r.Actions = append(r.Actions, actionResult(cell, a.ID, cellop.ActionNotRun, "drained; its stop needs the operator's confirm"))
		}
		rep.Confirm = nil
		r.Finish(cellop.Partial, cellop.ReasonRoleBusy, fmt.Sprintf("the drain timed out after %s with %s in flight; nothing was cancelled and those roles stay up, paused", o.Timeout, strings.Join(pending, ", ")), l.now())
		r.Next = []string{"cellctl drain " + cell + " --now", "cellctl resume " + cell}
		rep.Result = r
		return rep
	}
	if len(rep.Confirm) == 0 && o.Owner == "" {
		l.liftAfterStop(stoppedRoles(&r))
	}
	r.Finish(cellop.OK, "", "", l.now())
	rep.Result = r
	return rep
}

func drainRoles(targets []string, state map[string]lifeState, last map[string]idleReading) []cellop.Role {
	var out []cellop.Role
	for _, role := range targets {
		f := map[string]string{}
		if rd, ok := last[role]; ok {
			f["idle"] = rd.String()
			if rd.Mode != "" {
				f["mode"] = rd.Mode
			}
		}
		out = append(out, roleEntry(role, state[role], f))
	}
	return out
}

// stopAction is the planned stop of a drained interactive role. Its text states what was not
// measured, so the operator confirms knowing it.
func stopAction(cell, role string, rd idleReading) cellop.Action {
	return cellop.Action{
		ID:   "stop:" + role,
		Cell: cell,
		Summary: "stop " + role + " (close its one surface): no claim and no extra harness child; " +
			"harness idleness was NOT measured (could-not-check) — confirm only if its harness is idle",
		Preconditions: []cellop.Precondition{{Key: "idle:" + role, Value: rd.String()}},
	}
}

// drainNowPlan is `drain --now`'s plan: every claim still held becomes an abandon action, an
// unknown outcome to reconcile, ahead of the stop of its role.
// owner stamps any pause flag the plan creates, as drainOpts.Owner does ("" for `drain --now`).
func (l *interimLifecycle) drainNowPlan(roles []string, since time.Time, handoffWait time.Duration, owner string) ([]cellop.Action, error) {
	cell := l.c.Name
	if _, err := l.pauseAs(roles, owner); err != nil {
		return nil, err
	}
	claims, err := l.probes.claims()
	if err != nil {
		return nil, refuseWith(cellop.ReasonUnmeasured, "dispatch claims: %v", err)
	}
	deadline := since.Add(handoffWait)
	var actions []cellop.Action
	for _, role := range lifecycleTargets(roles) {
		held := claimsOf(cell, role, claims)
		cr, err := l.probes.cadence(role)
		if err != nil {
			return nil, refuseWith(cellop.ReasonUnmeasured, "%s: cadence checkpoint: %v", role, err)
		}
		if cr.Held || len(held) > 0 {
			wait := deadline.Sub(l.now())
			if wait < 0 {
				wait = 0
			}
			if _, err := l.Handoff(role, since, wait, held, true); err != nil {
				return nil, err
			}
		}
		for _, cl := range held {
			actions = append(actions, cellop.Action{
				ID: "abandon:" + cl.Key, Cell: cell,
				Summary:       fmt.Sprintf("abandon claim %s (%s, %s) held by %s: an unknown outcome to reconcile", cl.Key, cl.Repo, cl.State, role),
				Preconditions: []cellop.Precondition{{Key: "claim:" + cl.Key, Value: "held by " + role}},
			})
		}
		actions = append(actions, cellop.Action{
			ID: "stop-now:" + role, Cell: cell,
			Summary:       "stop " + role + " now, without waiting for idle (close its one surface); harness idleness was NOT measured",
			Preconditions: []cellop.Precondition{{Key: "claims:" + role, Value: claimKeys(held)}},
		})
	}
	return actions, nil
}

func claimsOf(cell, role string, claims []roleClaim) []roleClaim {
	var out []roleClaim
	for _, cl := range claims {
		if claimRole(cell, cl.Holder) == role {
			out = append(out, cl)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

func claimKeys(cs []roleClaim) string {
	var ks []string
	for _, c := range cs {
		ks = append(ks, c.Key)
	}
	return strings.Join(ks, ",")
}

// actionRole checks the role a stored plan's action id names before any path is built from it:
// a role-bearing action must name one of knownRoles. The CLI entry points already check their
// arguments (checkRoles); this repeats it for an id read back from the plan store, which any
// same-account process can write.
func actionRole(kind, arg string) error {
	switch kind {
	case "pause":
		if arg == "" || valueIn(arg, knownRoles) {
			return nil
		}
	case "stop", "stop-now":
		if valueIn(arg, knownRoles) {
			return nil
		}
	default:
		return nil
	}
	return fmt.Errorf("lifecycle action %s:%s names no known role", kind, arg)
}

// measureAction re-measures one planned lifecycle action's preconditions.
func (l *interimLifecycle) measureAction(a cellop.Action) ([]cellop.Precondition, error) {
	kind, arg, _ := strings.Cut(a.ID, ":")
	if err := actionRole(kind, arg); err != nil {
		return nil, err
	}
	switch kind {
	case "pause", "resume":
		return nil, nil
	case "lift-foreign":
		if _, err := l.ForeignStop(arg); err != nil {
			return []cellop.Precondition{{Key: "flag:" + arg, Value: "absent"}}, nil
		}
		return []cellop.Precondition{{Key: "flag:" + arg, Value: "present"}}, nil
	case "stop":
		rd := l.Idle(arg)
		if rd.Err != nil {
			return nil, rd.Err
		}
		return []cellop.Precondition{{Key: "idle:" + arg, Value: rd.String()}}, nil
	case "stop-now":
		claims, err := l.probes.claims()
		if err != nil {
			return nil, err
		}
		return []cellop.Precondition{{Key: "claims:" + arg, Value: claimKeys(claimsOf(l.c.Name, arg, claims))}}, nil
	case "abandon":
		claims, err := l.probes.claims()
		if err != nil {
			return nil, err
		}
		for _, cl := range claims {
			if cl.Key == arg {
				return []cellop.Precondition{{Key: "claim:" + arg, Value: "held by " + claimRole(l.c.Name, cl.Holder)}}, nil
			}
		}
		return []cellop.Precondition{{Key: "claim:" + arg, Value: "released"}}, nil
	}
	return nil, fmt.Errorf("unknown lifecycle action %q", a.ID)
}

// runAction runs one applied lifecycle action.
func (l *interimLifecycle) runAction(a cellop.Action) (string, error) {
	kind, arg, _ := strings.Cut(a.ID, ":")
	if err := actionRole(kind, arg); err != nil {
		return "", err
	}
	switch kind {
	case "pause":
		var roles []string
		if arg != "" {
			roles = []string{arg}
		}
		created, err := l.Pause(roles)
		if err != nil {
			return "", err
		}
		if len(created) == 0 {
			return "already paused", nil
		}
		return "paused", nil
	case "resume":
		lifted, kept, err := l.Resume()
		if err != nil {
			return "", err
		}
		return resumeDetail(lifted, kept), nil
	case "lift-foreign":
		if err := l.LiftForeign(arg); err != nil {
			return "", err
		}
		return "lifted the stop " + arg, nil
	case "stop":
		if _, err := l.Pause([]string{arg}); err != nil {
			return "", err
		}
		return l.Stop(arg, true, false)
	case "stop-now":
		if _, err := l.Pause([]string{arg}); err != nil {
			return "", err
		}
		return l.Stop(arg, true, true)
	case "abandon":
		return "abandoned: an unknown outcome to reconcile (named in the role's handoff record)", nil
	}
	return "", fmt.Errorf("unknown lifecycle action %q", a.ID)
}

func resumeDetail(lifted, kept []string) string {
	d := fmt.Sprintf("lifted %d stop(s) pause set", len(lifted))
	if len(kept) > 0 {
		names := make([]string, len(kept))
		for i, k := range kept {
			names[i] = filepath.Base(k)
		}
		d += "; left " + strings.Join(names, "; ")
	}
	return d
}

// stoppedRoles is the set of roles an applied result actually stopped: the roles of its stop and
// stop-now actions whose outcome is ok. It is read from the applied result, never from the apply
// invocation's arguments, because `--apply <id>` does not tie those arguments to the stored plan.
func stoppedRoles(r *cellop.Result) []string {
	var out []string
	for _, a := range r.Actions {
		kind, role, _ := strings.Cut(a.ID, ":")
		if (kind == "stop" || kind == "stop-now") && a.Outcome == cellop.ActionOK && !valueIn(role, out) {
			out = append(out, role)
		}
	}
	return out
}

// liftAfterStop lifts the stops pause set for roles now stopped, so a following `up` starts
// them: a stopped role is held by its closed surface, not by a flag. stopped is exactly the roles
// stopped (stoppedRoles); an empty set lifts nothing. The cell-wide stop is lifted only when no
// role of the cell outside the stopped set is still up (a role just closed may still read held
// for a moment while its supervisor exits, so the stopped set is not re-probed).
func (l *interimLifecycle) liftAfterStop(stopped []string) {
	if len(stopped) == 0 {
		return
	}
	allowed := map[string]bool{}
	for _, role := range stopped {
		allowed[l.pauseFlag(role)] = true
	}
	up := false
	for _, role := range knownRoles {
		if valueIn(role, stopped) {
			continue
		}
		if cr, err := l.probes.cadence(role); err != nil || cr.Held {
			up = true
		}
	}
	if !up {
		allowed[l.pauseFlag("")] = true
	}
	if _, _, err := l.resumeOnly(allowed); err != nil {
		fmt.Fprintf(os.Stderr, "NOTICE: lifting the drain's own stops: %v\n", err)
	}
}

// --- the cobra verbs -----------------------------------------------------------------------

func lifecycleBindings(extra ...cli.Binding) []cli.Binding {
	return append([]cli.Binding{
		bBool("plan", "write a cell-op plan for this transition and change nothing else (works anywhere)"),
		bBool("json", "print the cell-op-v1 document and nothing else on stdout"),
		bStr("apply", "apply a stored plan of this verb on this terminal"),
	}, extra...)
}

// lifecycleApply applies a stored plan of verb through the one apply path.
func lifecycleApply(cmd *cobra.Command, c *Cell, verb, id string, after func(*Cell, *cellop.Result)) {
	lc := c.lifecycle()
	cellopApply(cmd, c, cellopVerb{
		Verb:    verb,
		Measure: func(_ *Cell, a cellop.Action) ([]cellop.Precondition, error) { return lc.measureAction(a) },
		Run:     func(_ *Cell, a cellop.Action) (string, error) { return lc.runAction(a) },
		After:   after,
	}, id)
}

// planAndApply stores a plan of actions and applies it at once on this terminal (the operator
// sees it and types its confirm code), folding earlier into the result.
func planAndApply(cmd *cobra.Command, c *Cell, verb string, actions []cellop.Action, earlier []cellop.ActionResult, after func(*Cell, *cellop.Result)) {
	p := cellopNewPlan(c, verb, actions)
	lifecycleApply(cmd, c, verb, p.ID, func(c *Cell, r *cellop.Result) {
		r.Actions = append(append([]cellop.ActionResult(nil), earlier...), r.Actions...)
		if after != nil {
			after(c, r)
		}
	})
}

func lifecycleCell(verb string, args []string) *Cell {
	c := loadCell(cellArg(args))
	if c.Kind == "container" || c.Kind == "scrubbed" || c.kubeSelected() {
		die("%s: a %s cell has no per-role lifecycle on this host; use down", verb, c.Kind)
	}
	return c
}

func emitAndExit(cmd *cobra.Command, r cellop.Result) {
	emitResult(cmd, r)
	exitWith(r.ExitCode())
}

func pauseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pause <cell> [role...]",
		Short: "stop new work in the cell (or the named roles) from each loop's next iteration boundary",
		Long: "Move the targeted roles to paused: no new pass, dispatch or sweep starts after each loop's next iteration\n" +
			"boundary; work already in flight runs to its end. pause only ever adds a stop and is idempotent; resume lifts\n" +
			"exactly what it added. The mechanism is interim (docs/cellctl.md §Lifecycle).",
		Annotations: map[string]string{leadingArgsKey: "1"},
		Args:        cobra.MinimumNArgs(1),
	}
	set := declare(cmd, lifecycleBindings()...)
	markStateChanging(cmd)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		v := resolve(set, nil)
		c := lifecycleCell("pause", args)
		roles := args[1:]
		checkRoles("pause", roles)
		targets := roles
		if len(targets) == 0 {
			targets = []string{c.Name}
		}
		// A cell-wide pause is the bare id "pause" and a role pause is "pause:<role>", so the two
		// can never be confused, even for a cell whose name is also a role name.
		var actions []cellop.Action
		for _, t := range targets {
			id := "pause"
			if len(roles) > 0 {
				id = "pause:" + t
			}
			actions = append(actions, cellop.Action{ID: id, Cell: c.Name, Summary: "pause " + t + ": no new work from the next iteration boundary"})
		}
		switch {
		case v.Bool("plan"):
			cellopPlan(cmd, c, "pause", actions)
			return nil
		case cmd.Flags().Changed("apply"):
			lifecycleApply(cmd, c, "pause", v.String("apply"), nil)
			return nil
		}
		lc := c.lifecycle()
		r := cellop.NewResult("pause", c.Name, time.Now())
		for _, a := range actions {
			d, err := lc.runAction(a)
			if err != nil {
				finishErr(&r, err, time.Now())
				emitAndExit(cmd, r)
			}
			r.Actions = append(r.Actions, actionResult(c.Name, a.ID, cellop.ActionOK, d))
		}
		for _, role := range lifecycleTargets(roles) {
			s, _ := lifecycleStep(stateRunning, evPause)
			r.Roles = append(r.Roles, roleEntry(role, s, nil))
		}
		r.Finish(cellop.OK, "", "", time.Now())
		emitAndExit(cmd, r)
		return nil
	}
	return cmd
}

func resumeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resume <cell>",
		Short: "lift the stops pause set, and no other",
		Long: "Lift exactly the stops pause created whose identity still matches, and move those roles back to running.\n" +
			"It never lifts DISABLED, STOP, STOP.<loop>, a stop that existed before the pause or another cell's stop.\n" +
			"--foreign <flag> names one stop pause did not set: it is lifted only through a plan, applied by the\n" +
			"operator's interactive confirm. The mechanism is interim (docs/cellctl.md §Lifecycle).",
		Annotations: map[string]string{leadingArgsKey: "1"},
		Args:        cobra.ExactArgs(1),
	}
	set := declare(cmd, lifecycleBindings(bStr("foreign", "lift this one stop flag that pause did not set (plan + interactive confirm)"))...)
	markStateChanging(cmd)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		v := resolve(set, nil)
		c := lifecycleCell("resume", args)
		lc := c.lifecycle()
		actions := []cellop.Action{{ID: "resume", Cell: c.Name, Summary: "lift the stops pause set whose identity still matches"}}
		foreign := cmd.Flags().Changed("foreign")
		if foreign {
			p, err := lc.ForeignStop(v.String("foreign"))
			if err != nil {
				die("resume --foreign: %v", err)
			}
			name := filepath.Base(p)
			actions = []cellop.Action{{
				ID: "lift-foreign:" + name, Cell: c.Name,
				Summary:       "lift the stop " + name + ", which pause did not set: a deliberate operator act",
				Preconditions: []cellop.Precondition{{Key: "flag:" + name, Value: "present"}},
			}}
		}
		switch {
		case v.Bool("plan"):
			cellopPlan(cmd, c, "resume", actions)
			return nil
		case cmd.Flags().Changed("apply"):
			lifecycleApply(cmd, c, "resume", v.String("apply"), nil)
			return nil
		case foreign:
			planAndApply(cmd, c, "resume", actions, nil, nil)
			return nil
		}
		r := cellop.NewResult("resume", c.Name, time.Now())
		d, err := lc.runAction(actions[0])
		if err != nil {
			finishErr(&r, err, time.Now())
			emitAndExit(cmd, r)
		}
		r.Actions = append(r.Actions, actionResult(c.Name, "resume", cellop.ActionOK, d))
		r.Finish(cellop.OK, "", "", time.Now())
		emitAndExit(cmd, r)
		return nil
	}
	return cmd
}

func drainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "drain <cell> [role...]",
		Short: "pause, wait for in-flight work to finish, hand off, then stop each targeted role",
		Long: "Pause the targeted roles, then wait (polling at most every 10s, bounded by --timeout) for each to be idle per\n" +
			"its mode, collect its handoff record, and stop it with the per-role `down <cell> <role>`. A cadence role is\n" +
			"idle between passes. An interactive role is idle when it holds no claim and its harness has no extra child;\n" +
			"whether the harness itself is mid tool call cannot be measured (could-not-check), so it is stopped only on the\n" +
			"operator's confirm of a plan saying so. Draining never cancels work in flight: on timeout the roles stay up,\n" +
			"the outcome is partial and next names `drain --now` and `resume`.\n\n" +
			"--now waits only --handoff-wait for handoff records, then plans the stop of every targeted role, listing\n" +
			"each claim still held as an abandoned, unknown outcome to reconcile; it is applied only by the confirm.\n" +
			"The mechanism is interim (docs/cellctl.md §Lifecycle).",
		Annotations: map[string]string{leadingArgsKey: "1"},
		Args:        cobra.MinimumNArgs(1),
	}
	set := declare(cmd, lifecycleBindings(
		cli.Binding{Key: "timeout", Kind: cli.Duration, Flag: "timeout", Default: drainDefaultTimeout, Usage: "how long to wait for in-flight work before giving up (the roles stay up)"},
		cli.Binding{Key: "handoff-wait", Kind: cli.Duration, Flag: "handoff-wait", Default: drainDefaultHandoffWait, Usage: "how long to wait for each desk's own handoff record"},
		bBool("now", "do not wait for idle: plan the stop, listing every abandoned claim (interactive confirm only)"),
	)...)
	markStateChanging(cmd)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		v := resolve(set, nil)
		c := lifecycleCell("drain", args)
		roles := args[1:]
		checkRoles("drain", roles)
		lc := c.lifecycle()
		if cmd.Flags().Changed("apply") {
			lifecycleApply(cmd, c, "drain", v.String("apply"), func(c *Cell, r *cellop.Result) {
				if r.Outcome == cellop.OK {
					lc.liftAfterStop(stoppedRoles(r))
				}
			})
			return nil
		}
		timeout, handoffWait := v.Duration("timeout"), v.Duration("handoff-wait")
		if timeout <= 0 || handoffWait < 0 {
			die("drain: --timeout must be positive and --handoff-wait not negative")
		}
		if v.Bool("plan") {
			var actions []cellop.Action
			if v.Bool("now") {
				claims, err := lc.probes.claims()
				if err != nil {
					die("drain --plan: dispatch claims could not be read: %v", err)
				}
				for _, role := range lifecycleTargets(roles) {
					held := claimsOf(c.Name, role, claims)
					for _, cl := range held {
						actions = append(actions, cellop.Action{ID: "abandon:" + cl.Key, Cell: c.Name,
							Summary:       fmt.Sprintf("abandon claim %s (%s) held by %s: an unknown outcome to reconcile", cl.Key, cl.Repo, role),
							Preconditions: []cellop.Precondition{{Key: "claim:" + cl.Key, Value: "held by " + role}}})
					}
					actions = append(actions, cellop.Action{ID: "stop-now:" + role, Cell: c.Name,
						Summary:       "stop " + role + " now, without waiting for idle; harness idleness was NOT measured",
						Preconditions: []cellop.Precondition{{Key: "claims:" + role, Value: claimKeys(held)}}})
				}
			} else {
				for _, role := range lifecycleTargets(roles) {
					rd := lc.Idle(role)
					actions = append(actions, stopAction(c.Name, role, rd))
				}
			}
			cellopPlan(cmd, c, "drain", actions)
			return nil
		}
		if v.Bool("now") {
			actions, err := lc.drainNowPlan(roles, time.Now(), handoffWait, "")
			if err != nil {
				r := cellop.NewResult("drain", c.Name, time.Now())
				finishErr(&r, err, time.Now())
				emitAndExit(cmd, r)
			}
			planAndApply(cmd, c, "drain", actions, nil, func(c *Cell, r *cellop.Result) {
				if r.Outcome == cellop.OK {
					lc.liftAfterStop(stoppedRoles(r))
				}
			})
			return nil
		}
		rep := lc.drain(drainOpts{Roles: roles, Timeout: timeout, HandoffWait: handoffWait})
		if len(rep.Confirm) == 0 {
			emitAndExit(cmd, rep.Result)
		}
		earlier, drained := rep.Result.Actions, rep.Result.Roles
		planAndApply(cmd, c, "drain", rep.Confirm, earlier, func(c *Cell, r *cellop.Result) {
			r.Roles = drained
			if r.Outcome == cellop.OK {
				lc.liftAfterStop(stoppedRoles(r))
			}
		})
		return nil
	}
	return cmd
}

// cmdDownRoles is `down <cell> <role...>`: the per-role interim stop. Each named role must
// already be paused or drained; a role that is not is refused role-busy before anything is
// planned or changed, so this verb never pauses a role itself and a declined confirm leaves the
// cell exactly as it was. The stop goes through a plan the operator confirms on this terminal
// (re-measure → stop, in lifecycle.Stop). It never closes the session, a sibling or another
// cell's role. Cell-wide
// `down` is not a state-changing invocation in the cell-op v1 sense and stays as it was;
// only this role form is, so the annotation is set on the invocation that takes it.
func cmdDownRoles(cmd *cobra.Command, c *Cell, roles []string) {
	checkRoles("down", roles)
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[stateChanging] = "1"
	gateStateChanging(cmd, append([]string{c.Name}, roles...))
	lc := c.lifecycle()
	var actions []cellop.Action
	for _, role := range roles {
		paused, err := lc.Paused(role)
		if err != nil {
			err = refuseWith(cellop.ReasonUnmeasured, "%s: reading its stop: %v", role, err)
		} else if !paused {
			err = refuseWith(cellop.ReasonRoleBusy, "%s is not paused or drained; run `cellctl pause %s %s` or `cellctl drain %s %s` first", role, c.Name, role, c.Name, role)
		}
		if err != nil {
			r := cellop.NewResult("down", c.Name, time.Now())
			finishErr(&r, err, time.Now())
			emitAndExit(cmd, r)
		}
		actions = append(actions, stopAction(c.Name, role, lc.Idle(role)))
	}
	planAndApply(cmd, c, "down", actions, nil, func(c *Cell, r *cellop.Result) {
		if r.Outcome == cellop.OK {
			lc.liftAfterStop(stoppedRoles(r))
		}
	})
}
