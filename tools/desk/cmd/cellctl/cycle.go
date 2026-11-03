package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/medici-finance/assay/tools/desk/internal/cellcadence"
	"github.com/medici-finance/assay/tools/desk/internal/cellop"
	"github.com/medici-finance/assay/tools/desk/internal/cli"
)

// `cellctl cycle <cell> [role...]`: drain each role window, bring its harness's plugin placement
// to the marketplace pin, check it, reopen the window as `up` does and confirm a fresh heartbeat.
// It is built on the lifecycle interface and its drain (drain.go): the stop of new work, the
// wait for idle, the handoff and the per-role stop are drain's, unchanged. What cycle adds is
// the sync, the restart, the heartbeat and the per-role report.

const (
	cycleDefaultDrainTimeout  = 30 * time.Minute
	cycleDefaultHeartbeatWait = 2 * time.Minute
)

// Sync outcomes. could-not-check is never reported as a success.
const (
	syncOK            = "ok"
	syncFailed        = "failed"
	syncCouldNotCheck = "could-not-check"
)

type cycleOpts struct {
	Roles         []string
	DrainTimeout  time.Duration
	HandoffWait   time.Duration
	HeartbeatWait time.Duration
	Force         bool // --force-after-timeout
	AllAtOnce     bool
}

// syncReport is one role's plugin placement before and after the sync. Old and New are empty
// when the record could not be read.
type syncReport struct {
	Harness, Pin, Old, New string
	Outcome                string
	Detail                 string
}

// cycleOps are cycle's effects past the drain, injectable so the tests drive it without a
// terminal, a harness CLI or a cockpit.
type cycleOps struct {
	// confirm applies planned stops on the operator's confirm and returns that apply's result.
	confirm func(actions []cellop.Action) cellop.Result
	sync    func(role string) syncReport
	restart func(role string) error
	// heartbeat is the role's last recorded heartbeat.
	heartbeat func(role string) (time.Time, error)
}

// cycleRole is what cycle learned about one role.
type cycleRole struct {
	role      string
	state     lifeState
	drained   string // drained | forced | skipped
	sync      *syncReport
	heartbeat time.Time
	note      string
}

func (cr cycleRole) line() string {
	parts := []string{cr.drained}
	if cr.sync != nil {
		s := cr.sync
		parts = append(parts, fmt.Sprintf("%s plugin %s -> %s (%s)", s.Harness, orUnmeasured(s.Old), orUnmeasured(s.New), s.Outcome))
	}
	if !cr.heartbeat.IsZero() {
		parts = append(parts, "heartbeat "+cellop.Stamp(cr.heartbeat))
	} else if cr.drained != "skipped" {
		parts = append(parts, "heartbeat unconfirmed")
	}
	if cr.note != "" {
		parts = append(parts, cr.note)
	}
	return strings.Join(parts, "; ")
}

func (cr cycleRole) entry() cellop.Role {
	f := map[string]string{"drained": cr.drained}
	if cr.sync != nil {
		f["harness"], f["sync"] = cr.sync.Harness, cr.sync.Outcome
		for k, v := range map[string]string{"pin": cr.sync.Pin, "plugin_old": cr.sync.Old, "plugin_new": cr.sync.New} {
			if v != "" {
				f[k] = v
			}
		}
	}
	if !cr.heartbeat.IsZero() {
		f["heartbeat"] = cellop.Stamp(cr.heartbeat)
	}
	return roleEntry(cr.role, cr.state, f)
}

func orUnmeasured(s string) string {
	if s == "" {
		return "unmeasured"
	}
	return s
}

// cycle runs the whole cycle and returns its result. One role at a time by default: a role that
// does not come back stops the cycle there, so it never takes down more than that one role.
func (l *interimLifecycle) cycle(o cycleOpts, ops cycleOps) cellop.Result {
	cell := l.c.Name
	r := cellop.NewResult("cycle", cell, l.now())
	groups := make([][]string, 0, len(o.Roles))
	if o.AllAtOnce {
		groups = append(groups, o.Roles)
	} else {
		for _, role := range o.Roles {
			groups = append(groups, []string{role})
		}
	}
	var rows []cycleRole
	unchecked := false
	finish := func(out cellop.Outcome, reason, detail string, next ...string) cellop.Result {
		for _, cr := range rows {
			r.Roles = append(r.Roles, cr.entry())
			r.Actions = append(r.Actions, actionResult(cell, "report:"+cr.role, cellop.ActionOK, cr.line()))
		}
		r.Finish(out, reason, detail, l.now())
		r.Next = append([]string{}, next...)
		return r
	}
	for gi, group := range groups {
		// A role that is down before the cycle is not cycled: cycle reopens what was open.
		var live []string
		for _, role := range group {
			cr, err := l.probes.cadence(role)
			if err != nil {
				return finish(cellop.CouldNotCheck, cellop.ReasonUnmeasured, fmt.Sprintf("%s: cadence checkpoint: %v", role, err))
			}
			if !cr.Held {
				rows = append(rows, cycleRole{role: role, state: stateStopped, drained: "skipped", note: "down before the cycle; not cycled"})
				continue
			}
			live = append(live, role)
		}
		if len(live) == 0 {
			continue
		}
		drained, res, ok := l.cycleDrain(live, o, ops)
		r.Actions = append(r.Actions, res.Actions...)
		if !ok {
			for _, role := range live {
				rows = append(rows, cycleRole{role: role, state: stateRunning, drained: "not drained", note: "left running"})
			}
			reason := ""
			if res.Reason != nil {
				reason = *res.Reason
			}
			detail := ""
			if res.Detail != nil {
				detail = *res.Detail
			}
			return finish(res.Outcome, reason, detail, cycleRemaining(cell, groups[gi+1:])...)
		}
		for _, role := range live {
			cr := cycleRole{role: role, state: stateStopped, drained: drained}
			s := ops.sync(role)
			cr.sync = &s
			switch s.Outcome {
			case syncOK:
				r.Actions = append(r.Actions, actionResult(cell, "sync:"+role, cellop.ActionOK, s.Detail))
			case syncCouldNotCheck:
				unchecked = true
				r.Actions = append(r.Actions, actionResult(cell, "sync:"+role, cellop.ActionNotRun, "could-not-check: "+s.Detail))
			default:
				r.Actions = append(r.Actions, actionResult(cell, "sync:"+role, cellop.ActionFailed, s.Detail))
				cr.note = "left down: the placement check failed"
				rows = append(rows, cr)
				return finish(cellop.Partial, cellop.ReasonActionFailed, fmt.Sprintf("%s: the %s placement check failed, so the role is left down: %s", role, s.Harness, s.Detail),
					append([]string{"cellctl up " + cell + " --role " + role}, cycleRemaining(cell, groups[gi+1:])...)...)
			}
			since := l.now()
			if err := ops.restart(role); err != nil {
				r.Actions = append(r.Actions, actionResult(cell, "restart:"+role, cellop.ActionFailed, err.Error()))
				cr.note = "left down: the restart failed"
				rows = append(rows, cr)
				return finish(cellop.Partial, cellop.ReasonActionFailed, fmt.Sprintf("%s: the restart failed: %v", role, err),
					append([]string{"cellctl up " + cell + " --role " + role}, cycleRemaining(cell, groups[gi+1:])...)...)
			}
			cr.state, _ = lifecycleStep(stateStopped, evStart)
			r.Actions = append(r.Actions, actionResult(cell, "restart:"+role, cellop.ActionOK, "reopened as up does"))
			hb, err := l.awaitHeartbeat(role, since, o.HeartbeatWait, ops)
			if err != nil {
				r.Actions = append(r.Actions, actionResult(cell, "heartbeat:"+role, cellop.ActionFailed, err.Error()))
				cr.note = "no fresh heartbeat"
				rows = append(rows, cr)
				return finish(cellop.Partial, cellop.ReasonActionFailed, fmt.Sprintf("%s: %v", role, err),
					append([]string{"cellctl status " + cell}, cycleRemaining(cell, groups[gi+1:])...)...)
			}
			cr.heartbeat = hb
			r.Actions = append(r.Actions, actionResult(cell, "heartbeat:"+role, cellop.ActionOK, "fresh: "+cellop.Stamp(hb)))
			rows = append(rows, cr)
		}
	}
	if unchecked {
		return finish(cellop.CouldNotCheck, cellop.ReasonUnmeasured, "every role was cycled, but a harness placement could not be checked against the pin")
	}
	return finish(cellop.OK, "", "")
}

// cycleRemaining names the cycle of the roles not reached.
func cycleRemaining(cell string, groups [][]string) []string {
	var roles []string
	for _, g := range groups {
		roles = append(roles, g...)
	}
	if len(roles) == 0 {
		return nil
	}
	return []string{"cellctl cycle " + cell + " " + strings.Join(roles, " ")}
}

// cycleDrain is drain over roles, plus what cycle decides at its edges: interactive stops go to
// the operator's confirm; on timeout without force the cycle's own stops are lifted so the roles
// keep running, and with force the held claims are planned abandoned for the confirm. ok is true
// when every role is stopped and its stops lifted.
func (l *interimLifecycle) cycleDrain(roles []string, o cycleOpts, ops cycleOps) (string, cellop.Result, bool) {
	rep := l.drain(drainOpts{Roles: roles, Timeout: o.DrainTimeout, HandoffWait: o.HandoffWait})
	res := rep.Result
	timedOut := res.Outcome == cellop.Partial && res.Reason != nil && *res.Reason == cellop.ReasonRoleBusy
	switch {
	case res.Outcome == cellop.OK && len(rep.Confirm) == 0:
		return "drained", res, true
	case res.Outcome == cellop.OK:
		c := ops.confirm(rep.Confirm)
		res.Actions = append(res.Actions, c.Actions...)
		if c.Outcome != cellop.OK {
			return "", foldInto(res, c), false
		}
		l.liftAfterStop(roles)
		return "drained", res, true
	case !timedOut:
		return "", res, false
	}
	var held []string
	var left []string
	for _, role := range roles {
		if !stoppedIn(res, role) {
			left = append(left, role)
		}
	}
	claims, cerr := l.probes.claims()
	for _, role := range left {
		for _, cl := range claimsOf(l.c.Name, role, claims) {
			held = append(held, fmt.Sprintf("%s holds claim %s (%s)", role, cl.Key, cl.Repo))
		}
	}
	heldText := strings.Join(held, "; ")
	if cerr != nil {
		heldText = "the claims could not be read: " + cerr.Error()
	} else if heldText == "" {
		heldText = "no claim is held; the harness is still busy"
	}
	if !o.Force {
		allowed := map[string]bool{}
		for _, role := range left {
			allowed[l.pauseFlag(role)] = true
		}
		if _, _, err := l.resumeOnly(allowed); err != nil {
			res.Actions = append(res.Actions, actionResult(l.c.Name, "resume", cellop.ActionFailed, err.Error()))
		} else {
			res.Actions = append(res.Actions, actionResult(l.c.Name, "resume", cellop.ActionOK, "lifted the cycle's own stops of "+strings.Join(left, ", ")+"; left running"))
		}
		res.Finish(cellop.Partial, cellop.ReasonRoleBusy, fmt.Sprintf("the drain timed out after %s; not restarted, left running: %s", o.DrainTimeout, heldText), l.now())
		return "", res, false
	}
	actions, err := l.drainNowPlan(left, l.now(), o.HandoffWait)
	if err != nil {
		finishErr(&res, err, l.now())
		return "", res, false
	}
	c := ops.confirm(actions)
	res.Actions = append(res.Actions, c.Actions...)
	if c.Outcome != cellop.OK {
		return "", foldInto(res, c), false
	}
	l.liftAfterStop(roles)
	res.Actions = append(res.Actions, actionResult(l.c.Name, "force:"+strings.Join(left, ","), cellop.ActionOK, "stopped after the timeout: "+heldText))
	return "forced", res, true
}

func stoppedIn(r cellop.Result, role string) bool {
	for _, a := range r.Actions {
		if a.ID == "stop:"+role && a.Outcome == cellop.ActionOK {
			return true
		}
	}
	return false
}

// foldInto carries a confirm's verdict into the drain's result.
func foldInto(res, c cellop.Result) cellop.Result {
	reason, detail := "", ""
	if c.Reason != nil {
		reason = *c.Reason
	}
	if c.Detail != nil {
		detail = *c.Detail
	}
	o := c.Outcome
	if o == cellop.OK {
		o = cellop.Partial
	}
	res.Outcome, res.Reason, res.Detail = o, nil, nil
	if reason != "" {
		res.Reason = &reason
	}
	if detail != "" {
		res.Detail = &detail
	}
	res.PlanID = c.PlanID
	return res
}

// awaitHeartbeat waits, bounded, for a heartbeat newer than since.
func (l *interimLifecycle) awaitHeartbeat(role string, since time.Time, wait time.Duration, ops cycleOps) (time.Time, error) {
	deadline := since.Add(wait)
	var lastErr error
	for {
		hb, err := ops.heartbeat(role)
		lastErr = err
		if err == nil && !hb.Before(since) {
			return hb, nil
		}
		if !l.now().Before(deadline) {
			if lastErr != nil {
				return time.Time{}, fmt.Errorf("no fresh heartbeat within %s: %v", wait, lastErr)
			}
			return time.Time{}, fmt.Errorf("no heartbeat newer than the restart within %s", wait)
		}
		step := drainPollCap(wait)
		if left := deadline.Sub(l.now()); left < step {
			step = left
		}
		l.probes.sleep(step)
	}
}

// --- the live effects ---------------------------------------------------------------------

func (c *Cell) liveCycleOps(cmd *cobra.Command) cycleOps {
	return cycleOps{
		confirm:   func(actions []cellop.Action) cellop.Result { return c.cycleConfirm(cmd, actions) },
		sync:      c.cycleSync,
		restart:   c.cycleRestart,
		heartbeat: c.cycleHeartbeat,
	}
}

// cycleConfirm stores the planned stops and applies them on this terminal, through the one
// apply session, returning the result instead of exiting.
func (c *Cell) cycleConfirm(cmd *cobra.Command, actions []cellop.Action) cellop.Result {
	if !stateChangingInvocation(cmd) {
		die("cycle: an apply was reached from an invocation not annotated %s", stateChanging)
	}
	p := cellopNewPlan(c, "cycle", actions)
	lc := c.lifecycle()
	sess := cellop.Session{
		Store:       cellop.StoreFor(c.Dir),
		Cell:        c.Name,
		Verb:        "cycle",
		Interactive: interactive(),
		DeskMarkers: deskMarkers(),
		Confirm: func(p cellop.Plan, children []cellop.Plan) (string, error) {
			return cellop.ReadConfirm(os.Stdin, os.Stderr, cellop.Render(p, children))
		},
		Measure: func(a cellop.Action) ([]cellop.Precondition, error) { return lc.measureAction(a) },
		Run:     func(a cellop.Action) (string, error) { return lc.runAction(a) },
		Now:     time.Now,
	}
	return sess.Apply(p.ID)
}

// cycleHarness is the harness role runs under: the model policy's route when one is set, else
// the cell's CELL_HARNESS.
func (c *Cell) cycleHarness(role string) (string, error) {
	policy, _, err := c.cellModelPolicy()
	if err != nil {
		return "", err
	}
	if policy != nil {
		route, err := policy.Resolve(role, "", "", "")
		if err != nil {
			return "", err
		}
		return route.Harness, nil
	}
	if c.Harness == "" {
		return "claude", nil
	}
	return c.Harness, nil
}

func (c *Cell) cycleSync(role string) syncReport {
	wt := filepath.Join(c.Dir, "worktrees", role)
	harness, err := c.cycleHarness(role)
	if err != nil {
		return syncReport{Harness: "unknown", Outcome: syncCouldNotCheck, Detail: "the role's harness could not be resolved: " + err.Error()}
	}
	switch harness {
	case "claude":
		cfg, err := claudeConfigDirFor(runtime.GOOS, c.Env)
		if err != nil {
			return syncReport{Harness: harness, Outcome: syncCouldNotCheck, Detail: "the Claude config directory could not be resolved: " + err.Error()}
		}
		return claudeSync(wt, claudePluginCLI(wt, cfg))
	case "cursor":
		pin, err := marketplacePin(wt)
		if err != nil {
			return syncReport{Harness: harness, Outcome: syncCouldNotCheck, Detail: err.Error()}
		}
		return cursorSync(wt, pin, deskinstallCursor)
	}
	return syncReport{Harness: harness, Outcome: syncCouldNotCheck, Detail: harness + " has no placement mode to check against the pin"}
}

// marketplacePin is the plugin pin a role worktree's project settings name:
// extraKnownMarketplaces.assay.source.ref, without a leading v.
func marketplacePin(wt string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(wt, ".claude", "settings.json"))
	if err != nil {
		return "", fmt.Errorf("the marketplace pin could not be read: %v", err)
	}
	var s struct {
		Markets map[string]struct {
			Source struct {
				Ref string `json:"ref"`
			} `json:"source"`
		} `json:"extraKnownMarketplaces"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", fmt.Errorf("the marketplace pin could not be read: %v", err)
	}
	ref := strings.TrimPrefix(strings.TrimSpace(s.Markets["assay"].Source.Ref), "v")
	if ref == "" {
		return "", errors.New("the project settings name no assay marketplace pin (extraKnownMarketplaces.assay.source.ref)")
	}
	return ref, nil
}

// pluginCLI is the slice of the claude plugin CLI the sync uses.
type pluginCLI struct {
	list   func() ([]byte, error)
	update func() error
}

func claudePluginCLI(wt, cfg string) pluginCLI {
	run := func(args ...string) ([]byte, error) {
		cmd := exec.Command("claude", args...)
		cmd.Dir = wt
		cmd.Env = append(os.Environ(), "CLAUDE_CONFIG_DIR="+cfg)
		return cmd.Output()
	}
	return pluginCLI{
		list:   func() ([]byte, error) { return run("plugin", "list", "--json") },
		update: func() error { _, err := run("plugin", "update", "assay@assay", "-s", "project"); return err },
	}
}

// projectRecord is the version of the assay@assay project-scope record for wt.
func projectRecord(raw []byte, wt string) (string, error) {
	var ps []struct {
		ID, Version, Scope, ProjectPath string
	}
	if err := json.Unmarshal(raw, &ps); err != nil {
		return "", fmt.Errorf("the plugin list is not the expected shape: %v", err)
	}
	want := cleanPath(wt)
	for _, p := range ps {
		if p.ID == "assay@assay" && p.Scope == "project" && cleanPath(p.ProjectPath) == want && p.Version != "" {
			return strings.TrimPrefix(p.Version, "v"), nil
		}
	}
	return "", fmt.Errorf("no assay@assay project record for the role worktree")
}

func cleanPath(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return filepath.Clean(p)
}

// claudeSync brings the role worktree's project record to the pin with the plugin CLI's own
// update (restoring the settings bytes it rewrites) and reads it back. It never uninstalls: a
// record still behind the pin fails the check and names the remedy.
func claudeSync(wt string, cli pluginCLI) syncReport {
	s := syncReport{Harness: "claude"}
	pin, err := marketplacePin(wt)
	if err != nil {
		s.Outcome, s.Detail = syncCouldNotCheck, err.Error()
		return s
	}
	s.Pin = pin
	read := func() (string, error) {
		raw, err := cli.list()
		if err != nil {
			return "", fmt.Errorf("claude plugin list: %v", err)
		}
		return projectRecord(raw, wt)
	}
	if s.Old, err = read(); err != nil {
		s.Outcome, s.Detail = syncCouldNotCheck, err.Error()
		return s
	}
	if s.Old == pin {
		s.New, s.Outcome, s.Detail = s.Old, syncOK, "the project record already equals the pin "+pin
		return s
	}
	settings := filepath.Join(wt, ".claude", "settings.json")
	before, err := os.ReadFile(settings)
	if err != nil {
		s.Outcome, s.Detail = syncCouldNotCheck, "the project settings could not be read: "+err.Error()
		return s
	}
	uerr := cli.update()
	if after, rerr := os.ReadFile(settings); rerr != nil || !bytes.Equal(after, before) {
		if werr := os.WriteFile(settings, before, 0o644); werr != nil {
			s.Outcome, s.Detail = syncFailed, "the project settings could not be restored after the update: "+werr.Error()
			return s
		}
	}
	if s.New, err = read(); err != nil {
		s.Outcome, s.Detail = syncCouldNotCheck, err.Error()
		return s
	}
	if s.New == pin {
		s.Outcome, s.Detail = syncOK, fmt.Sprintf("the project record moved %s -> %s", s.Old, s.New)
		return s
	}
	s.Outcome = syncFailed
	s.Detail = fmt.Sprintf("the project record is %s, the pin %s", s.New, pin)
	if uerr != nil {
		s.Detail += fmt.Sprintf(" (the update failed: %v)", uerr)
	}
	s.Detail += "; remedy, in the role worktree: claude plugin uninstall assay@assay -s project --keep-data && claude plugin install assay@assay -s project"
	return s
}

// deskinstallCursor runs deskinstall's Cursor placement against wt; check is its --check.
// It returns deskinstall's exit code (0 clean or placed, 1 drift, 2 could-not-check).
func deskinstallCursor(wt, pin string, check bool) (int, string) {
	args := []string{"--harness", "cursor", "--repo", wt, "--release", "v" + pin}
	if check {
		args = append(args, "--check")
	}
	cmd := exec.Command("deskinstall", args...)
	out, err := cmd.CombinedOutput()
	last := lastLine(out)
	var ee *exec.ExitError
	switch {
	case err == nil:
		return 0, last
	case errors.As(err, &ee):
		return ee.ExitCode(), last
	}
	return 2, err.Error()
}

func lastLine(b []byte) string {
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// cursorSync places the pinned Cursor bundle with deskinstall and proves it with --check.
func cursorSync(wt, pin string, run func(wt, pin string, check bool) (int, string)) syncReport {
	s := syncReport{Harness: "cursor", Pin: pin}
	code, out := run(wt, pin, true)
	switch code {
	case 0:
		s.Old, s.New, s.Outcome, s.Detail = pin, pin, syncOK, "deskinstall --check: the placement already equals "+pin
		return s
	case 1:
		s.Old = "drift"
	default:
		s.Outcome, s.Detail = syncCouldNotCheck, "deskinstall --check could not check: "+out
		return s
	}
	if code, out = run(wt, pin, false); code != 0 {
		s.Outcome, s.Detail = syncFailed, fmt.Sprintf("deskinstall --release v%s failed (exit %d): %s", pin, code, out)
		return s
	}
	switch code, out = run(wt, pin, true); code {
	case 0:
		s.New, s.Outcome, s.Detail = pin, syncOK, "placed "+pin+"; deskinstall --check is clean"
	case 1:
		s.New, s.Outcome, s.Detail = "drift", syncFailed, "deskinstall --check still reports drift after the placement: "+out
	default:
		s.Outcome, s.Detail = syncCouldNotCheck, "deskinstall --check could not check after the placement: "+out
	}
	return s
}

// cycleRestart reopens role's window the way `up --role` does, and returns up's verdict.
func (c *Cell) cycleRestart(role string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(self, "up", c.Name, "--role", role, "--no-attach", "--json")
	cmd.Stderr = os.Stderr
	out, runErr := cmd.Output()
	var doc cellop.Result
	if err := json.Unmarshal(bytes.TrimSpace(out), &doc); err != nil {
		if runErr != nil {
			return fmt.Errorf("up --role %s: %v", role, runErr)
		}
		return fmt.Errorf("up --role %s printed no result document: %v", role, err)
	}
	if doc.Outcome != cellop.OK {
		d := string(doc.Outcome)
		if doc.Detail != nil {
			d += ": " + *doc.Detail
		}
		return fmt.Errorf("up --role %s: %s", role, d)
	}
	return nil
}

// cycleHeartbeat is the heartbeat role's own cadence checkpoint records.
func (c *Cell) cycleHeartbeat(role string) (time.Time, error) {
	st, err := cellcadence.Read(c.cadenceDir(role))
	if err != nil {
		return time.Time{}, err
	}
	if st.Cell != c.Name || st.Role != role {
		return time.Time{}, errors.New("foreign cadence checkpoint scope")
	}
	return st.Heartbeat, nil
}

// --- the cobra verb -----------------------------------------------------------------------

func cycleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cycle <cell> [role...]",
		Short: "drain each role window, sync its plugin placement to the pin, check it, restart it, confirm a heartbeat",
		Long: "For each targeted role (default: the cell's roles), one at a time: drain it (drain's stop of new work, wait\n" +
			"for idle bounded by --drain-timeout, handoff and per-role stop); bring its harness's plugin placement to the\n" +
			"marketplace pin and check it (claude: the worktree's project record against\n" +
			"extraKnownMarketplaces.assay.source.ref; cursor: deskinstall --release <pin>, proved by --check; codex has\n" +
			"no placement mode and reports could-not-check); reopen its window as `up --role` does; and wait for a\n" +
			"heartbeat newer than the restart. One report line per role names drained or forced, the plugin version\n" +
			"before and after, and the heartbeat.\n\n" +
			"On a drain timeout the role is not restarted: its stops are lifted so it keeps running, the held claims\n" +
			"are named, and the exit is non-zero — unless --force-after-timeout, which plans the stop with every held\n" +
			"claim abandoned for the operator's confirm. A failed placement check leaves the role down and says why;\n" +
			"a could-not-check one restarts it and the outcome is could-not-check. A role down before the cycle is\n" +
			"skipped. --all-at-once drains every targeted role together. The mechanism is interim (docs/cellctl.md\n" +
			"§Lifecycle).",
		Annotations: map[string]string{leadingArgsKey: "1"},
		Args:        cobra.MinimumNArgs(1),
	}
	set := declare(cmd,
		bBool("json", "print the cell-op-v1 document and nothing else on stdout"),
		cli.Binding{Key: "drain-timeout", Kind: cli.Duration, Flag: "drain-timeout", Default: cycleDefaultDrainTimeout, Usage: "how long to wait for each role's in-flight work (on timeout the role keeps running)"},
		cli.Binding{Key: "handoff-wait", Kind: cli.Duration, Flag: "handoff-wait", Default: drainDefaultHandoffWait, Usage: "how long to wait for each desk's own handoff record"},
		cli.Binding{Key: "heartbeat-wait", Kind: cli.Duration, Flag: "heartbeat-wait", Default: cycleDefaultHeartbeatWait, Usage: "how long to wait for a heartbeat newer than the restart"},
		bBool("force-after-timeout", "on a drain timeout, plan the stop with every held claim abandoned (interactive confirm only)"),
		bBool("all-at-once", "drain every targeted role together instead of one at a time"),
	)
	markStateChanging(cmd)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		v := resolve(set, nil)
		c := lifecycleCell("cycle", args)
		roles := args[1:]
		checkRoles("cycle", roles)
		if len(roles) == 0 {
			roles = append([]string(nil), c.Roles...)
		}
		if len(roles) == 0 {
			die("cycle: cell %s names no roles", c.Name)
		}
		o := cycleOpts{Roles: roles, DrainTimeout: v.Duration("drain-timeout"), HandoffWait: v.Duration("handoff-wait"),
			HeartbeatWait: v.Duration("heartbeat-wait"), Force: v.Bool("force-after-timeout"), AllAtOnce: v.Bool("all-at-once")}
		if o.DrainTimeout <= 0 || o.HeartbeatWait <= 0 || o.HandoffWait < 0 {
			die("cycle: --drain-timeout and --heartbeat-wait must be positive and --handoff-wait not negative")
		}
		emitAndExit(cmd, c.lifecycle().cycle(o, c.liveCycleOps(cmd)))
		return nil
	}
	return cmd
}
