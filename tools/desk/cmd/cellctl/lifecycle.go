package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellcadence"
	"github.com/medici-finance/assay/tools/desk/internal/cellop"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The runner-lifecycle transitions behind `pause`, `resume`, `drain` and the per-role `down`
// (desk-containers/18). The VOCABULARY here is the design: the role states, the transitions and
// their invariants are amendment 21's (docs/proposals/non-ci-architecture/21-cell-operator-surface.md
// §Lifecycle states and transitions). The MECHANISM behind the lifecycle interface is INTERIM:
// today's cell-scoped stop flag (desk-containers/16), the cadence checkpoint, the per-role
// surface receipt and a handoff file under the cell. When the runner lifecycle contract ships,
// interimLifecycle is replaced by an implementation over it and the verbs do not change.

// lifeState is a role's lifecycle state.
type lifeState string

const (
	stateRunning  lifeState = "running"
	statePausing  lifeState = "pausing"
	statePaused   lifeState = "paused"
	stateDraining lifeState = "draining"
	stateDrained  lifeState = "drained"
	stateStopped  lifeState = "stopped"
)

// lifecycleEvent moves a role between states. pause, resume, drain, stop and start are the
// transitions; boundary (the loop reached its next iteration boundary) and idle (no in-flight
// work remains) are the observations that complete pausing and draining.
type lifecycleEvent string

const (
	evPause    lifecycleEvent = "pause"
	evBoundary lifecycleEvent = "boundary"
	evResume   lifecycleEvent = "resume"
	evDrain    lifecycleEvent = "drain"
	evIdle     lifecycleEvent = "idle"
	evStop     lifecycleEvent = "stop"
	evStart    lifecycleEvent = "start"
)

// errIllegalTransition is returned for a transition amendment 21 does not define from a state.
var errIllegalTransition = errors.New("illegal lifecycle transition")

// lifecycleStep is the state machine. A transition it does not list is refused and the state is
// unchanged: resume from stopped is a start, never a resume, and stop needs a paused or drained
// role.
func lifecycleStep(from lifeState, ev lifecycleEvent) (lifeState, error) {
	switch {
	case ev == evPause && from == stateRunning:
		return statePausing, nil
	case ev == evPause && (from == statePausing || from == statePaused):
		return from, nil // idempotent
	case ev == evBoundary && from == statePausing:
		return statePaused, nil
	case ev == evResume && (from == statePausing || from == statePaused || from == stateDraining || from == stateDrained):
		return stateRunning, nil
	case ev == evResume && from == stateRunning:
		return stateRunning, nil // idempotent
	case ev == evDrain && (from == stateRunning || from == statePausing || from == statePaused):
		return stateDraining, nil
	case ev == evIdle && from == stateDraining:
		return stateDrained, nil
	case ev == evStop && (from == statePaused || from == stateDrained):
		return stateStopped, nil
	case ev == evStart && from == stateStopped:
		return stateRunning, nil
	}
	return from, fmt.Errorf("%w: %s from %s", errIllegalTransition, ev, from)
}

// lifecycleRefusal is a transition the interface refused, with the cell-op reason it maps to.
type lifecycleRefusal struct {
	Reason string
	Detail string
}

func (e *lifecycleRefusal) Error() string { return e.Reason + ": " + e.Detail }

func refuseWith(reason, format string, args ...any) error {
	return &lifecycleRefusal{Reason: reason, Detail: fmt.Sprintf(format, args...)}
}

// refusalReason is err's cell-op reason when it is a lifecycle refusal.
func refusalReason(err error) (string, bool) {
	var r *lifecycleRefusal
	if errors.As(err, &r) {
		return r.Reason, true
	}
	return "", false
}

// roleClaim is one dispatch claim a role's session holds.
type roleClaim struct {
	Key, Repo, Item, State, Holder string
}

// idleReading is a role's idle signal, measured per mode. It is a work-safety precondition, not
// a security boundary.
type idleReading struct {
	Role   string
	Mode   string // cadence | interactive | down (no role process holds the lease)
	Busy   []string
	Claims []roleClaim
	// HarnessUnmeasured is true for an interactive role: whether the harness itself has a tool
	// call in flight cannot be measured from outside it today. It is could-not-check, never idle.
	HarnessUnmeasured bool
	Err               error // the reading could not be taken; never idle
}

// measuredIdle: every part that can be measured reads idle.
func (r idleReading) measuredIdle() bool { return r.Err == nil && len(r.Busy) == 0 }

// idle: the whole signal reads idle, harness included. A recorded request needs this.
func (r idleReading) idle() bool { return r.measuredIdle() && !r.HarnessUnmeasured }

func (r idleReading) String() string {
	switch {
	case r.Err != nil:
		return "could-not-check: " + r.Err.Error()
	case len(r.Busy) > 0:
		return "busy: " + strings.Join(r.Busy, "; ")
	case r.HarnessUnmeasured:
		return "no claim and no extra harness child; harness idleness could-not-check (not measured)"
	}
	return "idle"
}

// lifecycle is the one internal transition interface the verbs bind to. Its implementation today
// is interimLifecycle; the runner lifecycle contract replaces it.
type lifecycle interface {
	// Pause stops new work for roles (none: the whole cell) from their next iteration boundary.
	// It only ever adds a stop, and is idempotent.
	Pause(roles []string) (created []string, err error)
	// Resume lifts exactly the stops Pause set whose identity still matches, and no other.
	Resume() (lifted, kept []string, err error)
	// ForeignStop validates a stop Pause did not set, for an operator plan naming it.
	ForeignStop(flag string) (string, error)
	// LiftForeign lifts that one named stop; it is reached only from an applied plan.
	LiftForeign(flag string) error
	// Paused reports whether a stop covers role.
	Paused(role string) (bool, error)
	// Idle measures role's idle signal per its mode.
	Idle(role string) idleReading
	// Stop is the per-role stop of a paused or drained role: stop its cadence, re-measure idle,
	// re-verify its surface identity, close that one surface. confirmed is true only on an
	// operator's applied plan, which is what may accept an interactive role's could-not-check
	// harness idleness. force (drain --now, applied) skips the idle re-measure: the plan already
	// listed what is abandoned.
	Stop(role string, confirmed, force bool) (string, error)
	// Handoff collects role's handoff record: the desk's own when it writes one by the deadline,
	// else a fallback naming what cellctl measured.
	Handoff(role string, since time.Time, wait time.Duration, claims []roleClaim, abandoned bool) (string, error)
}

// lifecycleProbes are the interim mechanism's outside readings and effects, injectable so tests
// drive the transitions without a live cockpit, forge or process table.
type lifecycleProbes struct {
	claims      func() ([]roleClaim, error)
	cadence     func(role string) (cadenceReading, error)
	stopCadence func(role string) error
	procs       procTable
	cockpit     surfaceCockpit
	now         func() time.Time
	sleep       func(time.Duration)
}

// cadenceReading is what the role's cadence directory says.
type cadenceReading struct {
	Held    bool   // a live process holds the role lease
	Mode    string // cadence | interactive | "" when no owner mode is recorded
	Running bool   // the checkpoint says a pass (or an interactive harness) is running
	Child   int    // the interactive harness child the lease holder recorded
}

// interimLifecycle is the INTERIM mechanism (amendment 21 §Interim mapping):
//   - pause / resume: the cell-scoped stop flag STOP.cell.<cell>[.<role>] in the cell's config
//     home (desk-containers/16), with every flag pause created recorded in
//     run/lifecycle/pause-record.json;
//   - drain: pause, then the idle signal per mode (the cadence checkpoint; the role's claims and
//     its harness's children);
//   - stop: the per-role `down <cell> <role...>`;
//   - the handoff record: run/handoff/<role>.md under the cell.
type interimLifecycle struct {
	c      *Cell
	probes lifecycleProbes
}

var _ lifecycle = (*interimLifecycle)(nil)

func (c *Cell) lifecycle() *interimLifecycle {
	return &interimLifecycle{c: c, probes: c.liveLifecycleProbes()}
}

func (c *Cell) liveLifecycleProbes() lifecycleProbes {
	return lifecycleProbes{
		claims:      c.liveClaims,
		cadence:     c.liveCadence,
		stopCadence: func(role string) error { return c.stopRoleCadence(role, cadenceStopWait) },
		procs:       psTable{},
		cockpit:     liveCockpit{},
		now:         time.Now,
		sleep:       time.Sleep,
	}
}

func (l *interimLifecycle) now() time.Time { return l.probes.now() }

// --- pause / resume: the cell-scoped stop flag ---------------------------------------------

// pauseFlag is the interim stop flag for role ("" for the whole cell).
func (l *interimLifecycle) pauseFlag(role string) string {
	loop := ""
	if role != "" {
		loop = role
		if names, known := deskkit.LoopFlagNames(role); known && len(names) > 0 {
			loop = names[0]
		}
	}
	return filepath.Join(l.c.Config, deskkit.CellStopFlagName(l.c.Name, loop))
}

func (l *interimLifecycle) pauseRecordPath() string {
	return filepath.Join(l.c.Dir, "run", "lifecycle", "pause-record.json")
}

func (l *interimLifecycle) handoffPath(role string) string {
	return filepath.Join(l.c.Dir, "run", "handoff", role+".md")
}

// pauseRecord is every flag pause created, with the identity it had when created.
type pauseRecord struct {
	Schema string        `json:"schema"`
	Cell   string        `json:"cell"`
	Flags  []createdFlag `json:"flags"`
}

type createdFlag struct {
	Path    string `json:"path"`
	Inode   uint64 `json:"inode"`
	Ctime   string `json:"ctime"`
	Created string `json:"created"`
}

const pauseRecordSchema = "cellctl-pause-record-v1"

func (l *interimLifecycle) readPauseRecord() (pauseRecord, error) {
	rec := pauseRecord{Schema: pauseRecordSchema, Cell: l.c.Name}
	b, err := os.ReadFile(l.pauseRecordPath())
	if errors.Is(err, os.ErrNotExist) {
		return rec, nil
	}
	if err != nil {
		return rec, err
	}
	if err := json.Unmarshal(b, &rec); err != nil {
		return rec, fmt.Errorf("pause record: %w", err)
	}
	if rec.Schema != pauseRecordSchema || rec.Cell != l.c.Name {
		return rec, fmt.Errorf("pause record: not this cell's (%s, %s)", rec.Schema, rec.Cell)
	}
	return rec, nil
}

func (l *interimLifecycle) writePauseRecord(rec pauseRecord) error {
	raw, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return writePrivateFile(l.pauseRecordPath(), append(raw, '\n'))
}

// Pause creates each flag only if it is absent, and records each one it created.
func (l *interimLifecycle) Pause(roles []string) ([]string, error) {
	targets := []string{l.pauseFlag("")}
	if len(roles) > 0 {
		targets = targets[:0]
		for _, r := range roles {
			targets = append(targets, l.pauseFlag(r))
		}
	}
	rec, err := l.readPauseRecord()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(l.c.Config, 0o700); err != nil {
		return nil, err
	}
	var created []string
	for _, p := range targets {
		f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, os.ErrExist) {
			continue // a stop already in place, ours or anyone's: left exactly as it is
		}
		if err != nil {
			return created, err
		}
		_, werr := fmt.Fprintf(f, "cellctl pause %s at %s\n", l.c.Name, cellop.Stamp(l.now()))
		cerr := f.Close()
		if werr != nil || cerr != nil {
			return created, errors.Join(werr, cerr)
		}
		ino, ctime, err := fileIdentity(p)
		if err != nil {
			// Unrecordable: resume will not be able to prove this flag is ours, so it stays
			// until the operator lifts it (resume --foreign). Never a silent loss of a stop.
			fmt.Fprintf(os.Stderr, "NOTICE: pause created %s but cannot record its identity (%v); resume will leave it\n", p, err)
			continue
		}
		rec.Flags = append(rec.Flags, createdFlag{Path: p, Inode: ino, Ctime: ctime, Created: cellop.Stamp(l.now())})
		created = append(created, p)
	}
	if len(created) > 0 {
		if err := l.writePauseRecord(rec); err != nil {
			return created, err
		}
	}
	return created, nil
}

// Resume removes exactly the recorded flags whose (inode, ctime) still match; a flag that was
// replaced since (someone else's stop now) is kept and reported.
func (l *interimLifecycle) Resume() ([]string, []string, error) { return l.resumeOnly(nil) }

// resumeOnly is Resume limited to the recorded flags in allowed (nil: every recorded flag);
// a recorded flag outside allowed stays recorded and in place.
func (l *interimLifecycle) resumeOnly(allowed map[string]bool) ([]string, []string, error) {
	rec, err := l.readPauseRecord()
	if err != nil {
		return nil, nil, err
	}
	var lifted, kept []string
	var remain []createdFlag
	for _, f := range rec.Flags {
		if allowed != nil && !allowed[f.Path] {
			remain = append(remain, f)
			continue
		}
		if !l.ownFlagPath(f.Path) {
			kept = append(kept, f.Path+" (not this cell's pause flag; left)")
			continue
		}
		ino, ctime, err := fileIdentity(f.Path)
		if errors.Is(err, os.ErrNotExist) {
			continue // already gone: idempotent
		}
		if err != nil {
			kept = append(kept, f.Path+" (identity could not be read; left)")
			remain = append(remain, f)
			continue
		}
		if ino != f.Inode || ctime != f.Ctime {
			kept = append(kept, f.Path+" (replaced since pause; not ours, left)")
			continue
		}
		if err := os.Remove(f.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			remain = append(remain, f)
			kept = append(kept, f.Path+" ("+err.Error()+")")
			continue
		}
		lifted = append(lifted, f.Path)
	}
	rec.Flags = remain
	if err := l.writePauseRecord(rec); err != nil {
		return lifted, kept, err
	}
	return lifted, kept, nil
}

// ownFlagPath: p is one of the flags this cell's pause can create — the record is a file any
// same-account process can write, so a path in it is checked before anything is removed.
func (l *interimLifecycle) ownFlagPath(p string) bool {
	if p == l.pauseFlag("") {
		return true
	}
	for _, r := range knownRoles {
		if p == l.pauseFlag(r) {
			return true
		}
	}
	return false
}

// ForeignStop resolves the flag the operator named to a stop file in this cell's config home.
func (l *interimLifecycle) ForeignStop(flag string) (string, error) {
	name := filepath.Base(flag)
	if name != flag && filepath.Clean(flag) != filepath.Join(l.c.Config, name) {
		return "", fmt.Errorf("--foreign names a flag in %s, by name", l.c.Config)
	}
	if name != "DISABLED" && name != "STOP" && !strings.HasPrefix(name, "STOP.") {
		return "", fmt.Errorf("%s is not a stop flag", name)
	}
	p := filepath.Join(l.c.Config, name)
	if _, err := os.Lstat(p); err != nil {
		return "", err
	}
	return p, nil
}

func (l *interimLifecycle) LiftForeign(flag string) error {
	p, err := l.ForeignStop(flag)
	if err != nil {
		return err
	}
	return os.Remove(p)
}

// Paused: a stop flag covers role — the same set the cadence guard reads.
func (l *interimLifecycle) Paused(role string) (bool, error) {
	err := l.c.cadenceGuard(role)
	if err == nil {
		return false, nil
	}
	if errors.Is(err, errStoppedByFlag) {
		return true, nil
	}
	return false, err
}

// --- the idle signal, per mode -------------------------------------------------------------

func (l *interimLifecycle) Idle(role string) idleReading {
	r := idleReading{Role: role}
	cr, err := l.probes.cadence(role)
	if err != nil {
		r.Err = fmt.Errorf("cadence checkpoint: %w", err)
		return r
	}
	claims, err := l.probes.claims()
	if err != nil {
		r.Err = fmt.Errorf("dispatch claims: %w", err)
		return r
	}
	for _, cl := range claims {
		if claimRole(l.c.Name, cl.Holder) == role {
			r.Claims = append(r.Claims, cl)
			r.Busy = append(r.Busy, "holds claim "+cl.Key)
		}
	}
	switch {
	case !cr.Held:
		r.Mode = "down"
		if cr.Running {
			r.Busy = append(r.Busy, "the checkpoint records an unfinished pass")
		}
	case cr.Mode == "cadence":
		r.Mode = "cadence"
		if cr.Running {
			r.Busy = append(r.Busy, "a cadence pass is running")
		}
	case cr.Mode == "interactive":
		// The checkpoint holds Running=true until the harness exits, so it says nothing about
		// idleness here: the claims, the harness's children and the harness itself do.
		r.Mode = "interactive"
		r.HarnessUnmeasured = true
		extra, err := l.extraChildren(role, cr.Child)
		if err != nil {
			r.Err = fmt.Errorf("harness children: %w", err)
			return r
		}
		for _, pid := range extra {
			r.Busy = append(r.Busy, fmt.Sprintf("harness child pid %d is running", pid))
		}
	default:
		r.Err = fmt.Errorf("the role lease is held but its owner mode %q is not recorded", cr.Mode)
	}
	return r
}

// extraChildren is every process under the role's harness (its whole subtree, so the agent's
// own tool calls are included) other than the harness child itself and the long-lived helpers
// its receipt recorded at launch. A helper started after launch reads as busy: the safe side.
func (l *interimLifecycle) extraChildren(role string, child int) ([]int, error) {
	rec, err := readSurfaceReceipt(l.c.cadenceDir(role))
	if err != nil {
		return nil, fmt.Errorf("surface receipt: %w", err)
	}
	if rec == nil {
		return nil, errors.New("no surface receipt names the harness")
	}
	desc, err := l.probes.procs.Descendants(rec.HarnessPID)
	if err != nil {
		return nil, err
	}
	helper := map[int]string{}
	for _, h := range rec.Helpers {
		helper[h.PID] = h.Start
	}
	var extra []int
	for _, pid := range desc {
		if pid == child {
			continue
		}
		if start, ok := helper[pid]; ok {
			now, gone, err := l.probes.procs.Start(pid)
			if err == nil && !gone && now == start {
				continue
			}
		}
		extra = append(extra, pid)
	}
	sort.Ints(extra)
	return extra, nil
}

// claimRole is the role a claim holder session belongs to in cell: sessions are named
// <cell>-<role>-<stamp> (desk.go).
func claimRole(cell, holder string) string {
	for _, r := range knownRoles {
		if strings.HasPrefix(holder, cell+"-"+r+"-") {
			return r
		}
	}
	return ""
}

// --- stop: the per-role interim stop -----------------------------------------------------

// Stop runs the per-role stop for one paused or drained role, in the brief's order.
func (l *interimLifecycle) Stop(role string, confirmed, force bool) (string, error) {
	paused, err := l.Paused(role)
	if err != nil {
		return "", refuseWith(cellop.ReasonUnmeasured, "%s: reading its stop: %v", role, err)
	}
	if !paused {
		return "", refuseWith(cellop.ReasonRoleBusy, "%s is not paused or drained; pause or drain it first", role)
	}
	cr, err := l.probes.cadence(role)
	if err != nil {
		return "", refuseWith(cellop.ReasonUnmeasured, "%s: cadence checkpoint: %v", role, err)
	}
	rec, err := readSurfaceReceipt(l.c.cadenceDir(role))
	if err != nil {
		return "", refuseWith(cellop.ReasonPerRoleCloseUnsupported, "%s: surface receipt unreadable: %v", role, err)
	}
	if !cr.Held && rec == nil {
		return role + ": not running; nothing to close", nil
	}
	if rec == nil {
		return "", refuseWith(cellop.ReasonPerRoleCloseUnsupported, "%s has no surface receipt; only cell-wide `down` can close it", role)
	}
	if why := l.probes.cockpit.CloseUnsupported(*rec); why != "" {
		return "", refuseWith(cellop.ReasonPerRoleCloseUnsupported, "%s: %s", role, why)
	}
	// The identity is verified before anything is stopped, so a stale or foreign receipt stops
	// nothing at all, and again immediately before the close.
	if err := l.verifySurface(l.c, *rec, false); err != nil {
		return "", err
	}
	// 1. that role's cadence alone.
	if err := l.probes.stopCadence(role); err != nil {
		if errors.Is(err, cellcadence.ErrUnfinished) {
			return "", refuseWith(cellop.ReasonRoleBusy, "%s: a cadence pass is unfinished", role)
		}
		return "", refuseWith(cellop.ReasonRoleBusy, "%s: %v", role, err)
	}
	// 2. the idle signal again, after that wait and immediately before the close.
	if !force {
		rd := l.Idle(role)
		if !rd.measuredIdle() || (rd.HarnessUnmeasured && !confirmed) {
			return "", refuseWith(cellop.ReasonRoleBusy, "%s re-measured %s", role, rd)
		}
	}
	// 3. the surface identity against the live cockpit. A cadence-mode supervisor exits on its
	// STOP file in step 1, so for it alone the recorded harness pid may now be gone.
	if err := l.verifySurface(l.c, *rec, cr.Mode == "cadence"); err != nil {
		return "", err
	}
	// 4. exactly that one surface.
	if err := l.probes.cockpit.Close(*rec); err != nil {
		return "", fmt.Errorf("%s: closing %s %s: %w", role, rec.Cockpit, rec.Surface, err)
	}
	// 5. the receipt.
	if err := removeSurfaceReceipt(l.c.cadenceDir(role)); err != nil {
		return "", fmt.Errorf("%s: closed, but removing its receipt: %w", role, err)
	}
	return fmt.Sprintf("%s: closed %s %s %s", role, rec.Cockpit, surfaceKind(rec.Cockpit), rec.Surface), nil
}

// --- the handoff record --------------------------------------------------------------------

// Handoff waits (bounded) for the desk's own record, written after since, then falls back to
// one naming what cellctl measured. Abandoned claims are named either way.
func (l *interimLifecycle) Handoff(role string, since time.Time, wait time.Duration, claims []roleClaim, abandoned bool) (string, error) {
	p := l.handoffPath(role)
	deadline := l.now().Add(wait)
	for {
		if st, err := os.Stat(p); err == nil && !st.ModTime().Before(since.Truncate(time.Second)) {
			if !abandoned || len(claims) == 0 {
				return p + " (written by the desk)", nil
			}
			return p + " (written by the desk)", appendAbandoned(p, claims)
		}
		if !l.now().Before(deadline) {
			break
		}
		l.probes.sleep(drainPollCap(wait))
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Handoff: %s in cell %s\n\n", role, l.c.Name)
	fmt.Fprintf(&b, "- time: %s\n- written-by: cellctl drain (fallback: the desk wrote no record by the deadline)\n\n", cellop.Stamp(l.now()))
	b.WriteString(handoffClaims(claims, abandoned))
	b.WriteString("## Open PRs\n\nnot recorded (the desk wrote no record)\n\n")
	b.WriteString("## Pending decision\n\nnot recorded (the desk wrote no record)\n\n")
	b.WriteString("## Activity\n\nnot recorded: the desk did not write its handoff before it was stopped. Reconcile the claims above before new work.\n")
	if err := writePrivateFile(p, []byte(b.String())); err != nil {
		return "", err
	}
	return p + " (fallback)", nil
}

func handoffClaims(claims []roleClaim, abandoned bool) string {
	var b strings.Builder
	b.WriteString("## Claims\n\n")
	if len(claims) == 0 {
		b.WriteString("none held at stop\n\n")
		return b.String()
	}
	for _, c := range claims {
		fmt.Fprintf(&b, "- %s (%s) state=%s", c.Key, c.Repo, c.State)
		if abandoned {
			b.WriteString(": abandoned at stop, an unknown outcome to reconcile")
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
	return b.String()
}

func appendAbandoned(p string, claims []roleClaim) error {
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "\n## Abandoned at stop (added by cellctl drain --now)\n\n%s", strings.TrimPrefix(handoffClaims(claims, true), "## Claims\n\n"))
	return err
}

// --- recorded requests: an allowlist with a work-safety precondition -----------------------

// recordedRequest is a transition request read from a file under the cell. Any same-account
// process can write such a file, so it is a request and never authority: its fields say what is
// asked, and nothing it claims about the role's state is believed.
type recordedRequest struct {
	Transition string   `json:"transition"`
	Roles      []string `json:"roles"`
	Timeout    string   `json:"timeout"`
	Now        bool     `json:"now"`
	Foreign    string   `json:"foreign"`
	// State is what the record says about the roles (say "idle"). It is kept for the operator's
	// reading and never believed: the idle signal is always re-measured.
	State string `json:"state,omitempty"`
}

// recordedAllowed is the allowlist: the stop and no-new-work transitions. Anything not on it —
// resume, start, restart, drain --now, a foreign-stop lift, and any future verb — is refused.
func recordedAllowed(q recordedRequest) bool {
	if q.Foreign != "" || q.Now {
		return false
	}
	switch q.Transition {
	case "pause":
		return true
	case "drain":
		return q.Timeout != ""
	case "stop":
		return len(q.Roles) > 0
	}
	return false
}

// executeRecorded is the admission rule for a recorded transition (amendment 21: R3 executes
// from a record only stop / no-new-work transitions). It is the interim form of that rule; no
// cellctl verb reads requests yet, and the runner that does goes through this function.
func executeRecorded(lc lifecycle, raw []byte, drain func(roles []string, timeout time.Duration) error) error {
	var q recordedRequest
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	if err := dec.Decode(&q); err != nil {
		return refuseWith(cellop.ReasonPlanInvalid, "recorded request: %v", err)
	}
	if !recordedAllowed(q) {
		return refuseWith(cellop.ReasonNotInteractive, "a recorded %q is not executed: only pause, a timed drain and a per-role stop are taken from a record; everything else needs the operator's terminal", q.Transition)
	}
	for _, r := range q.Roles {
		if !valueIn(r, knownRoles) {
			return refuseWith(cellop.ReasonPlanInvalid, "recorded request names an unknown role %q", r)
		}
	}
	switch q.Transition {
	case "pause":
		_, err := lc.Pause(q.Roles)
		return err
	case "drain":
		d, err := time.ParseDuration(q.Timeout)
		if err != nil || d <= 0 {
			return refuseWith(cellop.ReasonPlanInvalid, "recorded drain timeout %q", q.Timeout)
		}
		return drain(q.Roles, d)
	}
	// stop: only a role already paused or drained, as pause → re-measure → stop.
	for _, role := range q.Roles {
		paused, err := lc.Paused(role)
		if err != nil {
			return refuseWith(cellop.ReasonUnmeasured, "%s: %v", role, err)
		}
		if !paused {
			return refuseWith(cellop.ReasonRoleBusy, "a recorded stop for %s, which is not paused or drained", role)
		}
		if _, err := lc.Pause([]string{role}); err != nil {
			return err
		}
		if rd := lc.Idle(role); !rd.idle() {
			return refuseWith(cellop.ReasonRoleBusy, "a recorded stop for %s, which re-measured %s", role, rd)
		}
		if _, err := lc.Stop(role, false, false); err != nil {
			return err
		}
	}
	return nil
}

// --- live probes -----------------------------------------------------------------------------

func (c *Cell) liveCadence(role string) (cadenceReading, error) {
	dir := c.cadenceDir(role)
	var r cadenceReading
	busy, err := cellcadence.Probe(dir)
	if err != nil {
		return r, err
	}
	r.Held = busy
	st, err := cellcadence.Read(dir)
	switch {
	case err == nil:
		if st.Cell != "" && (st.Cell != c.Name || st.Role != role) {
			return r, errors.New("foreign cadence checkpoint scope")
		}
		r.Running = st.Running
	case errors.Is(err, cellcadence.ErrNoState), errors.Is(err, os.ErrNotExist):
	default:
		return r, err
	}
	if !busy {
		return r, nil
	}
	mode, err := readRoleMode(dir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return r, err
	}
	r.Mode = mode
	if h, err := cellcadence.Holder(dir); err == nil && h != nil {
		r.Child = h.ChildPID
	}
	return r, nil
}

// liveClaims reads every dispatch claim held by this cell's sessions, through desksupervise
// status, one CELL_ROOTS repository at a time. Any repository that cannot be read makes the
// whole reading could-not-check: a blind claims read is never "no claims".
func (c *Cell) liveClaims() ([]roleClaim, error) {
	bin := filepath.Join(deskToolsBin(c.Env), "desksupervise")
	var out []roleClaim
	entries := rootEntries(c.Env.Get("CELL_ROOTS"))
	if len(entries) == 0 {
		return nil, errors.New("CELL_ROOTS is empty: no repository to read claims from")
	}
	for _, e := range entries {
		args := []string{"status", "--json", "--repo", e[0]}
		if e[1] != "" {
			args = append(args, "--root", e[1])
		}
		cmd := exec.Command(bin, args...)
		cmd.Stderr = nil
		raw, err := cmd.Output()
		var exit *exec.ExitError
		if err != nil && !(errors.As(err, &exit) && exit.ExitCode() == 6) {
			return nil, fmt.Errorf("desksupervise status --repo %s: %v", e[0], err)
		}
		var snap struct {
			Schema string `json:"schema"`
			Claims []struct {
				Key, Repo, Item, State, Holder string
			} `json:"claims"`
		}
		if jerr := json.Unmarshal(raw, &snap); jerr != nil || snap.Schema == "" {
			return nil, fmt.Errorf("desksupervise status --repo %s: unreadable snapshot", e[0])
		}
		for _, cl := range snap.Claims {
			if claimRole(c.Name, cl.Holder) != "" {
				out = append(out, roleClaim{Key: cl.Key, Repo: cl.Repo, Item: cl.Item, State: cl.State, Holder: cl.Holder})
			}
		}
	}
	return out, nil
}

// writePrivateFile writes b to p (0600) through an exclusive temporary name and an atomic
// rename, in a 0700 directory.
func writePrivateFile(p string, b []byte) error {
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp := filepath.Join(dir, fmt.Sprintf(".%s.%d.%d.tmp", filepath.Base(p), os.Getpid(), time.Now().UnixNano()))
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL|oNoFollow, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, p); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
