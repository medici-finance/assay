//go:build darwin || linux

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellop"
)

// The cycle rows (assay#3859), driven in process on the lifecycle world (lifecycle_test.go) with
// the sync, restart, heartbeat and confirm effects faked.

type cycleFakes struct {
	w         *lcWorld
	restarts  []string
	restartAt []time.Time
	syncs     []string
	sync      map[string]syncReport
	staleBeat bool
	confirms  int
}

func newCycleFakes(w *lcWorld) *cycleFakes {
	return &cycleFakes{w: w, sync: map[string]syncReport{}}
}

func (f *cycleFakes) ops() cycleOps {
	return cycleOps{
		confirm: func(actions []cellop.Action) cellop.Result {
			f.confirms++
			r := cellop.NewResult("cycle", f.w.c.Name, f.w.clock)
			for _, a := range actions {
				d, err := f.w.lc.runAction(a)
				if err != nil {
					r.Actions = append(r.Actions, actionResult(f.w.c.Name, a.ID, cellop.ActionFailed, err.Error()))
					r.Finish(cellop.Failed, cellop.ReasonActionFailed, err.Error(), f.w.clock)
					return r
				}
				r.Actions = append(r.Actions, actionResult(f.w.c.Name, a.ID, cellop.ActionOK, d))
			}
			r.Finish(cellop.OK, "", "", f.w.clock)
			return r
		},
		sync: func(role string) syncReport {
			f.syncs = append(f.syncs, role)
			if s, ok := f.sync[role]; ok {
				return s
			}
			return syncReport{Harness: "claude", Pin: "1.0.45", Old: "1.0.30", New: "1.0.45", Outcome: syncOK, Detail: "moved"}
		},
		restart: func(role string) error {
			f.restarts = append(f.restarts, role)
			f.restartAt = append(f.restartAt, f.w.clock)
			return nil
		},
		heartbeat: func(role string) (time.Time, error) {
			if f.staleBeat {
				return f.w.clock.Add(-time.Hour), nil
			}
			return f.w.clock.Add(time.Second), nil
		},
	}
}

func cycleOptsFor(roles ...string) cycleOpts {
	return cycleOpts{Roles: roles, DrainTimeout: time.Minute, HandoffWait: 0, HeartbeatWait: 30 * time.Second}
}

func reportLine(t *testing.T, r cellop.Result, role string) string {
	t.Helper()
	for _, a := range r.Actions {
		if a.ID == "report:"+role && a.Detail != nil {
			return *a.Detail
		}
	}
	t.Fatalf("no report line for %s: %+v", role, r.Actions)
	return ""
}

// regression: cycle-waits-claim
func TestCycleWaitsForClaim(t *testing.T) {
	w := newLCWorld(t)
	w.cadenceRole("worker-desk", "tmux")
	w.claim("worker-desk", "example--brief-1")
	w.onSleep = func(n int) {
		if n == 2 {
			w.claims = nil
		}
	}
	f := newCycleFakes(w)
	start := w.clock
	r := w.lc.cycle(cycleOptsFor("worker-desk"), f.ops())
	if r.Outcome != cellop.OK {
		t.Fatalf("outcome %s: %v", r.Outcome, deref(r.Detail))
	}
	if len(w.sleeps) < 2 || len(f.restarts) != 1 {
		t.Fatalf("the cycle did not wait for the claim: sleeps=%v restarts=%v", w.sleeps, f.restarts)
	}
	if len(w.stopAt) != 1 || f.restartAt[0].Before(w.stopAt[0]) || !w.stopAt[0].After(start) {
		t.Fatalf("stopped before the claim was released: stops at %v", w.stopAt)
	}
	line := reportLine(t, r, "worker-desk")
	for _, want := range []string{"drained", "1.0.30 -> 1.0.45", "heartbeat 20"} {
		if !strings.Contains(line, want) {
			t.Errorf("report line %q lacks %q", line, want)
		}
	}
	if p, _ := w.lc.Paused("worker-desk"); p {
		t.Error("the cycle left its own stop behind")
	}
}

// regression: cycle-timeout-no-force
func TestCycleTimeoutLeavesRunning(t *testing.T) {
	w := newLCWorld(t)
	w.cadenceRole("worker-desk", "tmux")
	w.claim("worker-desk", "example--brief-2")
	f := newCycleFakes(w)
	r := w.lc.cycle(cycleOptsFor("worker-desk"), f.ops())
	if r.ExitCode() == 0 || r.Outcome != cellop.Partial {
		t.Fatalf("a drain timeout exited %d (%s)", r.ExitCode(), r.Outcome)
	}
	if r.Detail == nil || !strings.Contains(*r.Detail, "example--brief-2") {
		t.Fatalf("the held claim is not named: %v", r.Detail)
	}
	if len(f.restarts) != 0 || len(f.syncs) != 0 || len(w.stops) != 0 || len(w.closes()) != 0 {
		t.Fatalf("a timed-out role was touched: restarts=%v syncs=%v stops=%v closes=%v", f.restarts, f.syncs, w.stops, w.closes())
	}
	if p, _ := w.lc.Paused("worker-desk"); p {
		t.Error("a timed-out role was left paused, not running")
	}
}

// regression: cycle-force-after-timeout
func TestCycleForceAfterTimeout(t *testing.T) {
	w := newLCWorld(t)
	w.cadenceRole("worker-desk", "tmux")
	w.claim("worker-desk", "example--brief-3")
	f := newCycleFakes(w)
	o := cycleOptsFor("worker-desk")
	o.Force = true
	r := w.lc.cycle(o, f.ops())
	if r.Outcome != cellop.OK || f.confirms != 1 || len(f.restarts) != 1 {
		t.Fatalf("outcome %s confirms=%d restarts=%v", r.Outcome, f.confirms, f.restarts)
	}
	if line := reportLine(t, r, "worker-desk"); !strings.HasPrefix(line, "forced") {
		t.Errorf("report line %q does not say forced", line)
	}
}

// regression: cycle-sync-fail-down
func TestCycleSyncFailLeavesDown(t *testing.T) {
	w := newLCWorld(t)
	w.cadenceRole("worker-desk", "tmux")
	w.cadenceRole("pr-review-desk", "tmux")
	w.receipt("pr-review-desk", "tmux", "@9")
	w.procs.kids[92] = []int{harnessPID}
	f := newCycleFakes(w)
	f.sync["worker-desk"] = syncReport{Harness: "claude", Pin: "1.0.45", Old: "1.0.30", New: "1.0.30", Outcome: syncFailed, Detail: "record 1.0.30, pin 1.0.45"}
	r := w.lc.cycle(cycleOptsFor("worker-desk", "pr-review-desk"), f.ops())
	if r.Outcome != cellop.Partial || len(f.restarts) != 0 {
		t.Fatalf("a failed check restarted: %s %v", r.Outcome, f.restarts)
	}
	if r.Detail == nil || !strings.Contains(*r.Detail, "left down") {
		t.Fatalf("the detail does not say why: %v", r.Detail)
	}
	if len(w.stops) != 1 || w.stops[0] != "worker-desk" {
		t.Fatalf("the cycle went on past the failed role: stops=%v", w.stops)
	}
}

// regression: cycle-unchecked-not-ok
func TestCycleUncheckedNotOK(t *testing.T) {
	w := newLCWorld(t)
	w.cadenceRole("worker-desk", "tmux")
	f := newCycleFakes(w)
	f.sync["worker-desk"] = syncReport{Harness: "codex", Outcome: syncCouldNotCheck, Detail: "codex has no placement mode"}
	r := w.lc.cycle(cycleOptsFor("worker-desk"), f.ops())
	if r.Outcome != cellop.CouldNotCheck || r.ExitCode() == 0 {
		t.Fatalf("an unchecked placement read %s", r.Outcome)
	}
	if len(f.restarts) != 1 {
		t.Fatalf("an unchecked placement was not restarted: %v", f.restarts)
	}
	if line := reportLine(t, r, "worker-desk"); !strings.Contains(line, "could-not-check") {
		t.Errorf("report line %q hides the could-not-check", line)
	}
}

// regression: cycle-heartbeat-fresh
func TestCycleNeedsFreshBeat(t *testing.T) {
	w := newLCWorld(t)
	w.cadenceRole("worker-desk", "tmux")
	f := newCycleFakes(w)
	f.staleBeat = true
	r := w.lc.cycle(cycleOptsFor("worker-desk"), f.ops())
	if r.Outcome == cellop.OK {
		t.Fatal("a stale heartbeat read ok")
	}
	if line := reportLine(t, r, "worker-desk"); !strings.Contains(line, "heartbeat unconfirmed") {
		t.Errorf("report line %q", line)
	}
}

// regression: cycle-skips-down-role
func TestCycleSkipsDownRole(t *testing.T) {
	w := newLCWorld(t)
	f := newCycleFakes(w)
	r := w.lc.cycle(cycleOptsFor("verify-desk"), f.ops())
	if r.Outcome != cellop.OK || len(f.restarts) != 0 {
		t.Fatalf("a down role was cycled: %s %v", r.Outcome, f.restarts)
	}
}

// regression: cycle-all-at-once
func TestCycleAllAtOnce(t *testing.T) {
	w := newLCWorld(t)
	w.cadenceRole("worker-desk", "tmux")
	w.cadenceRole("pr-review-desk", "tmux")
	w.receipt("pr-review-desk", "tmux", "@9")
	w.procs.kids[92] = []int{harnessPID}
	f := newCycleFakes(w)
	var order []string
	w.onStop = func(role string) { order = append(order, "stop:"+role) }
	o := cycleOptsFor("worker-desk", "pr-review-desk")
	o.AllAtOnce = true
	ops := f.ops()
	restart := ops.restart
	ops.restart = func(role string) error { order = append(order, "restart:"+role); return restart(role) }
	if r := w.lc.cycle(o, ops); r.Outcome != cellop.OK {
		t.Fatalf("outcome %s: %v", r.Outcome, deref(r.Detail))
	}
	want := "stop:worker-desk stop:pr-review-desk restart:worker-desk restart:pr-review-desk"
	if got := strings.Join(order, " "); got != want {
		t.Fatalf("order %q, want %q", got, want)
	}
}

// Under --all-at-once a drain timeout may already have stopped the roles that reached idle. Those
// are reported down with their own stops lifted and `up` named, never "left running".
// regression: cycle-all-at-once-timeout
func TestCycleAllAtOnceTimeoutStopped(t *testing.T) {
	w := newLCWorld(t)
	w.cadenceRole("worker-desk", "tmux")
	w.cadenceRole("pr-review-desk", "tmux")
	w.receipt("pr-review-desk", "tmux", "@9")
	w.procs.kids[92] = []int{harnessPID}
	w.claim("pr-review-desk", "example--brief-4")
	f := newCycleFakes(w)
	o := cycleOptsFor("worker-desk", "pr-review-desk")
	o.AllAtOnce = true
	r := w.lc.cycle(o, f.ops())
	if r.Outcome != cellop.Partial || r.ExitCode() == 0 || len(f.restarts) != 0 {
		t.Fatalf("outcome %s exit %d restarts %v", r.Outcome, r.ExitCode(), f.restarts)
	}
	if len(w.stops) != 1 || w.stops[0] != "worker-desk" {
		t.Fatalf("the fixture did not stop exactly the idle role: stops=%v", w.stops)
	}
	if p, _ := w.lc.Paused("worker-desk"); p {
		t.Error("the stopped role kept the cycle's stop, so a later up meets a paused role")
	}
	if p, _ := w.lc.Paused("pr-review-desk"); p {
		t.Error("the timed-out role was left paused, not running")
	}
	if line := reportLine(t, r, "worker-desk"); !strings.Contains(line, "left down") || strings.Contains(line, "left running") {
		t.Errorf("the stopped role reads %q", line)
	}
	if line := reportLine(t, r, "pr-review-desk"); !strings.Contains(line, "left running") {
		t.Errorf("the timed-out role reads %q", line)
	}
	if !valueIn("cellctl up "+w.c.Name+" --role worker-desk", r.Next) {
		t.Errorf("Next does not bring the stopped role back: %v", r.Next)
	}
	for _, e := range r.Roles {
		if e.Role == "worker-desk" && (e.State == nil || *e.State != string(stateStopped)) {
			t.Errorf("the stopped role's entry reads %v", deref(e.State))
		}
	}
}

// A pause the operator set before the cycle is not the cycle's to lift on a timeout.
// regression: cycle-timeout-keeps-operator-pause
func TestCycleTimeoutKeepsOperatorPause(t *testing.T) {
	w := newLCWorld(t)
	w.cadenceRole("worker-desk", "tmux")
	w.claim("worker-desk", "example--brief-5")
	if _, err := w.lc.Pause([]string{"worker-desk"}); err != nil {
		t.Fatal(err)
	}
	f := newCycleFakes(w)
	r := w.lc.cycle(cycleOptsFor("worker-desk"), f.ops())
	if r.Outcome != cellop.Partial {
		t.Fatalf("outcome %s", r.Outcome)
	}
	if p, _ := w.lc.Paused("worker-desk"); !p {
		t.Error("the timeout lifted the operator's own earlier pause")
	}
}

// --- the claude and cursor syncs ---------------------------------------------------------------

func pinnedWorktree(t *testing.T, ref string) string {
	t.Helper()
	wt := t.TempDir()
	if err := os.MkdirAll(filepath.Join(wt, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"extraKnownMarketplaces":{"assay":{"source":{"source":"github","repo":"example-org/example-repo","ref":"` + ref + `"}}}}`
	if err := os.WriteFile(filepath.Join(wt, ".claude", "settings.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return wt
}

func pluginList(wt, version string) []byte {
	return []byte(`[{"id":"assay@assay","version":"9.9.9","scope":"project","projectPath":"/elsewhere"},` +
		`{"id":"assay@assay","version":"` + version + `","scope":"project","projectPath":"` + wt + `"}]`)
}

// regression: cycle-claude-sync
func TestClaudeSyncToPin(t *testing.T) {
	wt := pinnedWorktree(t, "v1.0.45")
	settings := filepath.Join(wt, ".claude", "settings.json")
	before, _ := os.ReadFile(settings)
	rec := "1.0.30"
	updates := 0
	cli := pluginCLI{
		list: func() ([]byte, error) { return pluginList(wt, rec), nil },
		update: func() error {
			updates++
			rec = "1.0.45"
			return os.WriteFile(settings, []byte(`{"rewritten":true}`), 0o644)
		},
	}
	s := claudeSync(wt, cli)
	if s.Outcome != syncOK || s.Old != "1.0.30" || s.New != "1.0.45" || updates != 1 {
		t.Fatalf("sync %+v updates=%d", s, updates)
	}
	if after, _ := os.ReadFile(settings); string(after) != string(before) {
		t.Fatalf("the settings were not restored: %s", after)
	}
	if s := claudeSync(wt, cli); s.Outcome != syncOK || updates != 1 {
		t.Fatalf("a current record was updated again: %+v", s)
	}
}

// regression: cycle-claude-stale
func TestClaudeSyncStaysStale(t *testing.T) {
	wt := pinnedWorktree(t, "v1.0.45")
	cli := pluginCLI{
		list:   func() ([]byte, error) { return pluginList(wt, "1.0.30"), nil },
		update: func() error { return nil },
	}
	s := claudeSync(wt, cli)
	if s.Outcome != syncFailed || !strings.Contains(s.Detail, "uninstall") {
		t.Fatalf("a record still behind the pin: %+v", s)
	}
	cli.list = func() ([]byte, error) { return nil, errors.New("exit status 1") }
	if s := claudeSync(wt, cli); s.Outcome != syncCouldNotCheck {
		t.Fatalf("an unreadable record read %s", s.Outcome)
	}
	bare := pinnedWorktree(t, "")
	cli.list = func() ([]byte, error) { return pluginList(bare, "1.0.30"), nil }
	if s := claudeSync(bare, cli); s.Outcome != syncCouldNotCheck {
		t.Fatalf("an unreadable pin read %s", s.Outcome)
	}
}

// regression: cycle-cursor-sync
func TestCursorSyncChecks(t *testing.T) {
	var calls []string
	codes := []int{1, 0, 0}
	run := func(_, pin string, check bool) (int, string) {
		calls = append(calls, map[bool]string{true: "check", false: "place"}[check])
		c := codes[0]
		codes = codes[1:]
		return c, "out"
	}
	if s := cursorSync("/wt", "1.0.45", run); s.Outcome != syncOK || strings.Join(calls, " ") != "check place check" {
		t.Fatalf("drift then place: %+v %v", s, calls)
	}
	codes = []int{2}
	if s := cursorSync("/wt", "1.0.45", run); s.Outcome != syncCouldNotCheck {
		t.Fatalf("an unrunnable check read %s", s.Outcome)
	}
	codes = []int{1, 0, 1}
	if s := cursorSync("/wt", "1.0.45", run); s.Outcome != syncFailed {
		t.Fatalf("drift after placing read %s", s.Outcome)
	}
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
