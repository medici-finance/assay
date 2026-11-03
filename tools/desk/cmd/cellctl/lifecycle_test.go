//go:build darwin || linux

package main

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellop"
	"github.com/medici-finance/assay/tools/desk/internal/cellop/schemacheck"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The lifecycle rows of desk-containers/18, driven in process against the one transition
// interface with fake probes: claims, cadence readings, the process table and stub cockpit CLIs
// (tmux, herdr, orca) whose argv is recorded. Nothing here names the interim mechanism's own
// paths; TestInterimMappingPinned is the one test that does.

type fakeProcs struct {
	start    map[int]string
	gone     map[int]bool
	startErr map[int]error
	kids     map[int][]int
}

func (p *fakeProcs) Start(pid int) (string, bool, error) {
	if err := p.startErr[pid]; err != nil {
		return "", false, err
	}
	if p.gone[pid] {
		return "", true, nil
	}
	s, ok := p.start[pid]
	if !ok {
		return "", true, nil
	}
	return s, false, nil
}

func (p *fakeProcs) Descendants(pid int) ([]int, error) { return p.kids[pid], nil }

// lcWorld is one cell "sample" with fake probes.
type lcWorld struct {
	t       *testing.T
	c       *Cell
	lc      *interimLifecycle
	claims  []roleClaim
	claimEr error
	cad     map[string]cadenceReading
	procs   *fakeProcs
	stops   []string    // roles whose cadence was stopped, in order
	stopAt  []time.Time // the fake clock at each
	onStop  func(role string)
	calls   []string // every cockpit argv
	noHelp  map[string]bool
	paneErr bool
	clock   time.Time
	sleeps  []time.Duration
	onSleep func(n int)
}

const (
	tmuxList   = "@7\tsample-cell\tsample-worker-desk\t90\n@8\tother-cell\tother-worker-desk\t91\n@9\tsample-cell\tsample-pr-review-desk\t92\n"
	herdrTabs  = `{"result":{"tabs":[{"tab_id":"tab7","label":"sample-worker-desk"},{"tab_id":"tab9","label":"sample-pr-review-desk"}]}}`
	herdrPane  = `{"result":{"shell_pid":90,"foreground_processes":[{"pid":100}]}}`
	orcaShow   = `{"ok":true,"result":{"terminal":{"handle":"term7","title":"sample-worker-desk","pid":90}},"_meta":{"runtimeId":"rt1"}}`
	harnessPID = 100
)

func newLCWorld(t *testing.T) *lcWorld {
	t.Helper()
	w := &lcWorld{
		t:      t,
		c:      &Cell{Name: "sample", Dir: t.TempDir(), Config: t.TempDir(), Kind: "house", Session: "sample-cell"},
		cad:    map[string]cadenceReading{},
		noHelp: map[string]bool{},
		clock:  time.Now(),
		procs: &fakeProcs{
			start:    map[int]string{harnessPID: "start-100", 102: "start-102"},
			gone:     map[int]bool{},
			startErr: map[int]error{},
			kids:     map[int][]int{90: {harnessPID, 101}, 92: {200}, harnessPID: {101}},
		},
	}
	w.lc = &interimLifecycle{c: w.c, probes: lifecycleProbes{
		claims: func() ([]roleClaim, error) {
			return append([]roleClaim(nil), w.claims...), w.claimEr
		},
		cadence: func(role string) (cadenceReading, error) { return w.cad[role], nil },
		stopCadence: func(role string) error {
			w.stops = append(w.stops, role)
			w.stopAt = append(w.stopAt, w.clock)
			if w.onStop != nil {
				w.onStop(role)
			}
			return nil
		},
		procs:   w.procs,
		cockpit: liveCockpit{run: w.run, help: func(tok string, _ ...string) bool { return !w.noHelp[tok] }},
		now:     func() time.Time { return w.clock },
		sleep: func(d time.Duration) {
			w.sleeps = append(w.sleeps, d)
			w.clock = w.clock.Add(d)
			if w.onSleep != nil {
				w.onSleep(len(w.sleeps))
			}
		},
	}}
	return w
}

func (w *lcWorld) run(name string, args ...string) ([]byte, error) {
	line := strings.Join(append([]string{name}, args...), " ")
	w.calls = append(w.calls, line)
	switch {
	case strings.HasPrefix(line, "tmux list-windows"):
		return []byte(tmuxList), nil
	case strings.HasPrefix(line, "herdr tab list"):
		return []byte(herdrTabs), nil
	case strings.HasPrefix(line, "herdr pane process-info"):
		if w.paneErr {
			return nil, errors.New("exit status 1")
		}
		return []byte(herdrPane), nil
	case strings.HasPrefix(line, "orca terminal show"):
		return []byte(orcaShow), nil
	}
	return nil, nil
}

// closes is every cockpit call that is not a read.
func (w *lcWorld) closes() []string {
	var out []string
	for _, c := range w.calls {
		if strings.Contains(c, "list-windows") || strings.Contains(c, "tab list") || strings.Contains(c, "process-info") || strings.Contains(c, "terminal show") {
			continue
		}
		out = append(out, c)
	}
	return out
}

// cadenceRole makes role a cadence-mode role between passes, with a receipt for cockpit.
func (w *lcWorld) cadenceRole(role, cockpit string) {
	w.cad[role] = cadenceReading{Held: true, Mode: "cadence"}
	w.receipt(role, cockpit, "")
}

// interactiveRole makes role an interactive role whose harness has its child 101 only.
func (w *lcWorld) interactiveRole(role, cockpit string) {
	w.cad[role] = cadenceReading{Held: true, Mode: "interactive", Running: true, Child: 101}
	w.receipt(role, cockpit, "")
}

// receipt writes role's surface receipt; surface overrides the cockpit's default id.
func (w *lcWorld) receipt(role, cockpit, surface string) {
	w.t.Helper()
	rec := surfaceReceipt{Cell: w.c.Name, Role: role, Cockpit: cockpit, Name: surfaceName(w.c.Name, role),
		HarnessPID: harnessPID, HarnessStart: "start-100", Written: cellop.Stamp(w.clock)}
	switch cockpit {
	case "tmux":
		rec.Session, rec.Surface = w.c.Session, "@7"
	case "herdr":
		rec.Workspace, rec.Surface, rec.Pane = "ws1", "tab7", "pane7"
	case "orca":
		rec.Runtime, rec.Surface = "rt1", "term7"
	}
	if surface != "" {
		rec.Surface = surface
	}
	if err := writeSurfaceReceipt(w.c.cadenceDir(role), rec); err != nil {
		w.t.Fatal(err)
	}
}

func (w *lcWorld) hasReceipt(role string) bool {
	rec, err := readSurfaceReceipt(w.c.cadenceDir(role))
	if err != nil {
		w.t.Fatal(err)
	}
	return rec != nil
}

func (w *lcWorld) claim(role, key string) {
	w.claims = append(w.claims, roleClaim{Key: key, Repo: "example-org/example-repo", Item: key, State: "held", Holder: w.c.Name + "-" + role + "-20261103T000000Z"})
}

// down is the per-role stop the operator's applied `down <cell> <role>` plan runs.
func (w *lcWorld) down(role string) (string, error) {
	if _, err := w.lc.Pause([]string{role}); err != nil {
		w.t.Fatal(err)
	}
	return w.lc.Stop(role, true, false)
}

// stopFiles is every file in the config home, the set the stop flags live in.
func (w *lcWorld) stopFiles() []string {
	w.t.Helper()
	ents, err := os.ReadDir(w.c.Config)
	if err != nil {
		w.t.Fatal(err)
	}
	var out []string
	for _, e := range ents {
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out
}

func touch(t *testing.T, p string) {
	t.Helper()
	if err := os.WriteFile(p, []byte("set by someone else\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func wantReason(t *testing.T, err error, want string) {
	t.Helper()
	got, ok := refusalReason(err)
	if !ok || got != want {
		t.Fatalf("want refusal %s, got %v", want, err)
	}
}

// --- row 1 -----------------------------------------------------------------------------------

// regression: lifecycle-states
func TestLifecycleTransitions(t *testing.T) {
	walk := func(from lifeState, evs ...lifecycleEvent) []lifeState {
		var out []lifeState
		s := from
		for _, ev := range evs {
			next, err := lifecycleStep(s, ev)
			if err != nil {
				t.Fatalf("%s from %s: %v", ev, s, err)
			}
			s = next
			out = append(out, s)
		}
		return out
	}
	got := walk(stateRunning, evPause, evBoundary, evResume)
	if want := []lifeState{statePausing, statePaused, stateRunning}; !equalStates(got, want) {
		t.Errorf("pause/boundary/resume: %v, want %v", got, want)
	}
	got = walk(stateRunning, evDrain, evIdle, evStop, evStart)
	if want := []lifeState{stateDraining, stateDrained, stateStopped, stateRunning}; !equalStates(got, want) {
		t.Errorf("drain/idle/stop/start: %v, want %v", got, want)
	}
	if s, err := lifecycleStep(statePaused, evPause); err != nil || s != statePaused {
		t.Errorf("pause is not idempotent: %s, %v", s, err)
	}
	for _, c := range []struct {
		from lifeState
		ev   lifecycleEvent
	}{
		{stateStopped, evResume}, {stateRunning, evStop}, {stateRunning, evIdle},
		{stateStopped, evDrain}, {stateRunning, evStart}, {statePausing, evStop},
	} {
		s, err := lifecycleStep(c.from, c.ev)
		if !errors.Is(err, errIllegalTransition) || s != c.from {
			t.Errorf("%s from %s: want refused with the state unchanged, got %s, %v", c.ev, c.from, s, err)
		}
	}
}

func equalStates(a, b []lifeState) bool {
	return strings.Join(statesOf(a), ",") == strings.Join(statesOf(b), ",")
}

func statesOf(s []lifeState) []string {
	var out []string
	for _, x := range s {
		out = append(out, string(x))
	}
	return out
}

// --- row 1b ----------------------------------------------------------------------------------

// regression: resume-foreign-stops
func TestResumeLeavesForeignStops(t *testing.T) {
	w := newLCWorld(t)
	for _, name := range []string{"DISABLED", "STOP", "STOP.worker-desk",
		deskkit.CellStopFlagName("sample", ""), deskkit.CellStopFlagName("other", "")} {
		touch(t, filepath.Join(w.c.Config, name))
	}
	before := w.stopFiles()

	if created, err := w.lc.Pause(nil); err != nil || len(created) != 0 {
		t.Fatalf("pausing the whole cell under a pre-existing cell stop must create nothing: %v, %v", created, err)
	}
	created, err := w.lc.Pause([]string{"worker-desk"})
	if err != nil || len(created) != 1 {
		t.Fatalf("pause worker-desk: %v, %v", created, err)
	}
	if paused, err := w.lc.Paused("worker-desk"); err != nil || !paused {
		t.Fatalf("worker-desk is not paused: %v", err)
	}
	if again, err := w.lc.Pause([]string{"worker-desk"}); err != nil || len(again) != 0 {
		t.Fatalf("pause is not idempotent: %v, %v", again, err)
	}
	if len(w.stopFiles()) != len(before)+1 {
		t.Fatalf("pause should add exactly one stop: %v -> %v", before, w.stopFiles())
	}

	lifted, _, err := w.lc.Resume()
	if err != nil || len(lifted) != 1 || lifted[0] != created[0] {
		t.Fatalf("resume lifted %v (%v), want only %v", lifted, err, created)
	}
	if got := w.stopFiles(); strings.Join(got, ",") != strings.Join(before, ",") {
		t.Fatalf("resume changed someone else's stops: %v, want %v", got, before)
	}
	if lifted, _, err := w.lc.Resume(); err != nil || len(lifted) != 0 {
		t.Fatalf("resume is not idempotent: %v, %v", lifted, err)
	}

	// A stop pause created but someone replaced since is theirs now: kept.
	created, err = w.lc.Pause([]string{"verify-desk"})
	if err != nil || len(created) != 1 {
		t.Fatal(created, err)
	}
	if err := os.Remove(created[0]); err != nil {
		t.Fatal(err)
	}
	touch(t, created[0])
	if lifted, kept, err := w.lc.Resume(); err != nil || len(lifted) != 0 || len(kept) != 1 {
		t.Fatalf("a replaced stop: lifted %v kept %v (%v)", lifted, kept, err)
	}
	if _, err := os.Lstat(created[0]); err != nil {
		t.Fatalf("the replaced stop was removed: %v", err)
	}

	// A record naming a stop that is not one of pause's own is never acted on.
	rec, err := w.lc.readPauseRecord()
	if err != nil {
		t.Fatal(err)
	}
	ino, ctime, err := fileIdentity(filepath.Join(w.c.Config, "DISABLED"))
	if err != nil {
		t.Fatal(err)
	}
	rec.Flags = append(rec.Flags, createdFlag{Path: filepath.Join(w.c.Config, "DISABLED"), Inode: ino, Ctime: ctime})
	if err := w.lc.writePauseRecord(rec); err != nil {
		t.Fatal(err)
	}
	if lifted, _, err := w.lc.Resume(); err != nil || len(lifted) != 0 {
		t.Fatalf("a forged record lifted %v (%v)", lifted, err)
	}
	if _, err := os.Lstat(filepath.Join(w.c.Config, "DISABLED")); err != nil {
		t.Fatal("DISABLED was removed")
	}

	// resume --foreign off a terminal: refused before anything is lifted.
	cw := newCLIWorld(t, "")
	stop := filepath.Join(cw.cfgDir, "STOP")
	touch(t, stop)
	r := cw.run(t, nil, "resume", "example", "--foreign", "STOP")
	if r.Code != 5 || !strings.Contains(r.Stdout+r.Stderr, cellop.ReasonNotInteractive) {
		t.Fatalf("resume --foreign off a terminal: exit %d\n%s%s", r.Code, r.Stdout, r.Stderr)
	}
	if _, err := os.Lstat(stop); err != nil {
		t.Fatal("resume --foreign off a terminal lifted the stop")
	}
}

// --- row 1d ----------------------------------------------------------------------------------

// regression: recorded-allowlist
func TestRecordedResumeNotExecuted(t *testing.T) {
	type drainCall struct {
		roles   []string
		timeout time.Duration
	}
	setup := func(t *testing.T) (*lcWorld, *[]drainCall, func(string) error) {
		w := newLCWorld(t)
		var calls []drainCall
		exec := func(raw string) error {
			return executeRecorded(w.lc, []byte(raw), func(roles []string, d time.Duration) error {
				calls = append(calls, drainCall{roles, d})
				return nil
			})
		}
		return w, &calls, exec
	}

	t.Run("never from a record", func(t *testing.T) {
		for _, raw := range []string{
			`{"transition":"resume"}`,
			`{"transition":"start","roles":["worker-desk"]}`,
			`{"transition":"restart","roles":["worker-desk"]}`,
			`{"transition":"drain","timeout":"5m","now":true}`,
			`{"transition":"resume","foreign":"STOP"}`,
			`{"transition":"lift-foreign","foreign":"STOP"}`,
			`{"transition":"stop","roles":["worker-desk"],"foreign":"STOP"}`,
		} {
			w, calls, exec := setup(t)
			w.cadenceRole("worker-desk", "tmux")
			touch(t, filepath.Join(w.c.Config, "STOP"))
			if _, err := w.lc.Pause([]string{"worker-desk"}); err != nil {
				t.Fatal(err)
			}
			before := w.stopFiles()
			err := exec(raw)
			if got, _ := refusalReason(err); got != cellop.ReasonNotInteractive {
				t.Errorf("%s: want refused %s, got %v", raw, cellop.ReasonNotInteractive, err)
			}
			if len(*calls) != 0 || len(w.stops) != 0 || len(w.closes()) != 0 {
				t.Errorf("%s: executed (drain %v, stops %v, closes %v)", raw, *calls, w.stops, w.closes())
			}
			if got := w.stopFiles(); strings.Join(got, ",") != strings.Join(before, ",") {
				t.Errorf("%s: changed the stops %v -> %v", raw, before, got)
			}
		}
	})

	t.Run("pause and a timed drain", func(t *testing.T) {
		w, calls, exec := setup(t)
		if err := exec(`{"transition":"pause","roles":["worker-desk"]}`); err != nil {
			t.Fatal(err)
		}
		if paused, _ := w.lc.Paused("worker-desk"); !paused {
			t.Fatal("a recorded pause was not executed")
		}
		if err := exec(`{"transition":"drain","roles":["worker-desk"],"timeout":"5m"}`); err != nil {
			t.Fatal(err)
		}
		if len(*calls) != 1 || (*calls)[0].timeout != 5*time.Minute || strings.Join((*calls)[0].roles, ",") != "worker-desk" {
			t.Fatalf("a recorded timed drain: %v", *calls)
		}
		if err := exec(`{"transition":"drain"}`); err == nil || len(*calls) != 1 {
			t.Fatal("a recorded drain without a timeout was executed")
		}
	})

	t.Run("stop of a paused idle role", func(t *testing.T) {
		w, _, exec := setup(t)
		w.cadenceRole("worker-desk", "tmux")
		if _, err := w.lc.Pause([]string{"worker-desk"}); err != nil {
			t.Fatal(err)
		}
		if err := exec(`{"transition":"stop","roles":["worker-desk"]}`); err != nil {
			t.Fatal(err)
		}
		if got := w.closes(); len(got) != 1 || got[0] != "tmux kill-window -t @7" {
			t.Fatalf("closes %v", got)
		}
	})

	t.Run("stop of a role not paused", func(t *testing.T) {
		w, _, exec := setup(t)
		w.cadenceRole("worker-desk", "tmux")
		wantReason(t, exec(`{"transition":"stop","roles":["worker-desk"]}`), cellop.ReasonRoleBusy)
		if len(w.stops) != 0 || len(w.closes()) != 0 {
			t.Fatalf("stopped a running role: %v %v", w.stops, w.closes())
		}
	})

	t.Run("stop of a busy role", func(t *testing.T) {
		w, _, exec := setup(t)
		w.cadenceRole("worker-desk", "tmux")
		w.claim("worker-desk", "example-repo--7")
		if _, err := w.lc.Pause([]string{"worker-desk"}); err != nil {
			t.Fatal(err)
		}
		wantReason(t, exec(`{"transition":"stop","roles":["worker-desk"]}`), cellop.ReasonRoleBusy)
		if len(w.stops) != 0 || len(w.closes()) != 0 || !w.hasReceipt("worker-desk") {
			t.Fatalf("a busy role was touched: stops %v closes %v", w.stops, w.closes())
		}
	})

	t.Run("busy sibling left running", func(t *testing.T) {
		w, _, exec := setup(t)
		w.cadenceRole("worker-desk", "tmux")
		w.cad["pr-review-desk"] = cadenceReading{Held: true, Mode: "cadence", Running: true}
		w.receipt("pr-review-desk", "tmux", "@9")
		w.claim("pr-review-desk", "example-repo--9")
		if _, err := w.lc.Pause([]string{"worker-desk"}); err != nil {
			t.Fatal(err)
		}
		if err := exec(`{"transition":"stop","roles":["worker-desk"]}`); err != nil {
			t.Fatal(err)
		}
		if strings.Join(w.stops, ",") != "worker-desk" {
			t.Fatalf("cadences stopped: %v", w.stops)
		}
		for _, c := range w.closes() {
			if strings.Contains(c, "@9") || strings.Contains(c, "kill-session") {
				t.Fatalf("the sibling's window was closed: %v", w.closes())
			}
		}
		if paused, _ := w.lc.Paused("pr-review-desk"); paused || !w.hasReceipt("pr-review-desk") || len(w.claims) != 1 {
			t.Fatal("the busy sibling was paused, lost its receipt or its claim")
		}
	})

	t.Run("the record's idle is not believed", func(t *testing.T) {
		for _, busy := range []string{"claim", "child"} {
			w, _, exec := setup(t)
			if busy == "claim" {
				w.cadenceRole("worker-desk", "tmux")
				w.claim("worker-desk", "example-repo--7")
			} else {
				w.cadenceRole("worker-desk", "tmux")
				w.cad["worker-desk"] = cadenceReading{Held: true, Mode: "interactive", Running: true, Child: 101}
				w.procs.kids[harnessPID] = []int{101, 102}
			}
			if _, err := w.lc.Pause([]string{"worker-desk"}); err != nil {
				t.Fatal(err)
			}
			wantReason(t, exec(`{"transition":"stop","roles":["worker-desk"],"state":"idle"}`), cellop.ReasonRoleBusy)
			if len(w.stops) != 0 || len(w.closes()) != 0 {
				t.Fatalf("%s: the record's idle was believed: %v %v", busy, w.stops, w.closes())
			}
		}
	})
}

// --- row 1e ----------------------------------------------------------------------------------

// regression: per-role-close
func TestPerRoleDownClosesOneSurface(t *testing.T) {
	for _, c := range []struct{ cockpit, close string }{
		{"tmux", "tmux kill-window -t @7"},
		{"herdr", "herdr tab close tab7"},
		{"orca", "orca terminal close --terminal term7"},
	} {
		t.Run(c.cockpit, func(t *testing.T) {
			w := newLCWorld(t)
			w.cadenceRole("worker-desk", c.cockpit)
			w.cadenceRole("pr-review-desk", c.cockpit)
			if _, err := w.down("worker-desk"); err != nil {
				t.Fatal(err)
			}
			if got := w.closes(); len(got) != 1 || got[0] != c.close {
				t.Fatalf("closes %v, want exactly [%s]", got, c.close)
			}
			if w.hasReceipt("worker-desk") || !w.hasReceipt("pr-review-desk") {
				t.Fatal("the role's receipt must go and the sibling's stay")
			}
			if strings.Join(w.stops, ",") != "worker-desk" {
				t.Fatalf("cadences stopped: %v", w.stops)
			}
		})
	}

	refusals := []struct {
		name, reason string
		set          func(w *lcWorld)
	}{
		{"no receipt", cellop.ReasonPerRoleCloseUnsupported, func(w *lcWorld) {
			w.cadenceRole("worker-desk", "tmux")
			if err := removeSurfaceReceipt(w.c.cadenceDir("worker-desk")); err != nil {
				w.t.Fatal(err)
			}
		}},
		{"herdr without tab close", cellop.ReasonPerRoleCloseUnsupported, func(w *lcWorld) {
			w.cadenceRole("worker-desk", "herdr")
			w.noHelp["close"] = true
		}},
		{"orca without --terminal", cellop.ReasonPerRoleCloseUnsupported, func(w *lcWorld) {
			w.cadenceRole("worker-desk", "orca")
			w.noHelp["--terminal"] = true
		}},
		{"another cell's window", cellop.ReasonSurfaceIdentityMismatch, func(w *lcWorld) {
			w.cadenceRole("worker-desk", "tmux")
			w.receipt("worker-desk", "tmux", "@8")
		}},
		{"a sibling's window", cellop.ReasonSurfaceIdentityMismatch, func(w *lcWorld) {
			w.cadenceRole("worker-desk", "tmux")
			w.receipt("worker-desk", "tmux", "@9")
		}},
		{"another cell's tab", cellop.ReasonSurfaceIdentityMismatch, func(w *lcWorld) {
			w.cadenceRole("worker-desk", "herdr")
			w.receipt("worker-desk", "herdr", "tab9")
		}},
		{"harness gone", cellop.ReasonSurfaceStale, func(w *lcWorld) {
			w.cadenceRole("worker-desk", "tmux")
			w.procs.gone[harnessPID] = true
		}},
		{"harness restarted", cellop.ReasonSurfaceStale, func(w *lcWorld) {
			w.cadenceRole("worker-desk", "tmux")
			w.procs.start[harnessPID] = "start-later"
		}},
		{"herdr process-info fails", cellop.ReasonSurfaceIdentityMismatch, func(w *lcWorld) {
			w.cadenceRole("worker-desk", "herdr")
			w.paneErr = true
		}},
		{"start time unreadable", cellop.ReasonSurfaceIdentityMismatch, func(w *lcWorld) {
			w.cadenceRole("worker-desk", "orca")
			w.procs.startErr[harnessPID] = errors.New("ps: no such process table")
		}},
	}
	for _, c := range refusals {
		t.Run(c.name, func(t *testing.T) {
			w := newLCWorld(t)
			c.set(w)
			had := w.hasReceipt("worker-desk")
			_, err := w.down("worker-desk")
			wantReason(t, err, c.reason)
			if got := w.closes(); len(got) != 0 {
				t.Fatalf("closed %v", got)
			}
			if len(w.stops) != 0 {
				t.Fatalf("stopped the cadence before the identity held: %v", w.stops)
			}
			if w.hasReceipt("worker-desk") != had {
				t.Fatal("a refusal changed the receipt")
			}
		})
	}
}

// --- row 1f ----------------------------------------------------------------------------------

// regression: idle-per-mode
func TestIdleSignalPerMode(t *testing.T) {
	w := newLCWorld(t)
	w.cadenceRole("worker-desk", "tmux")
	if rd := w.lc.Idle("worker-desk"); !rd.idle() || rd.Mode != "cadence" {
		t.Fatalf("a cadence role between passes: %+v", rd)
	}
	w.cad["worker-desk"] = cadenceReading{Held: true, Mode: "cadence", Running: true}
	if rd := w.lc.Idle("worker-desk"); rd.measuredIdle() {
		t.Fatalf("a cadence role mid pass read idle: %+v", rd)
	}

	w.interactiveRole("verify-desk", "tmux")
	rd := w.lc.Idle("verify-desk")
	if !rd.measuredIdle() || !rd.HarnessUnmeasured || rd.idle() || !strings.Contains(rd.String(), "could-not-check") {
		t.Fatalf("an interactive role with no claim and no extra child: %+v (%s)", rd, rd)
	}
	w.procs.kids[harnessPID] = []int{101, 102}
	if rd := w.lc.Idle("verify-desk"); rd.measuredIdle() {
		t.Fatalf("an extra harness child read idle: %+v", rd)
	}
	w.procs.kids[harnessPID] = []int{101}
	w.claim("verify-desk", "example-repo--3")
	if rd := w.lc.Idle("verify-desk"); rd.measuredIdle() {
		t.Fatalf("a held claim read idle: %+v", rd)
	}
	w.claims = nil
	w.claimEr = errors.New("forge unreachable")
	if rd := w.lc.Idle("worker-desk"); rd.measuredIdle() || rd.Err == nil {
		t.Fatalf("an unreadable claims probe read idle: %+v", rd)
	}
	w.claimEr = nil

	// A recorded stop for the interactive role: could-not-check is never idle.
	if _, err := w.lc.Pause([]string{"verify-desk"}); err != nil {
		t.Fatal(err)
	}
	wantReason(t, executeRecorded(w.lc, []byte(`{"transition":"stop","roles":["verify-desk"]}`), nil), cellop.ReasonRoleBusy)
	if len(w.stops) != 0 || len(w.closes()) != 0 {
		t.Fatalf("a recorded stop ran for an interactive role: %v %v", w.stops, w.closes())
	}

	// The interactive drain hands the stop to the operator, saying what was not measured.
	rep := w.lc.drain(drainOpts{Roles: []string{"verify-desk"}, Timeout: time.Minute})
	if len(rep.Confirm) != 1 || !strings.Contains(rep.Confirm[0].Summary, "NOT measured") || !strings.Contains(rep.Confirm[0].Summary, "could-not-check") {
		t.Fatalf("the interactive drain plan: %+v", rep.Confirm)
	}
	if len(w.stops) != 0 {
		t.Fatalf("the interactive role was stopped without the confirm: %v", w.stops)
	}

	// The re-measure runs after the cadence wait: a claim taken during it refuses.
	w2 := newLCWorld(t)
	w2.cadenceRole("worker-desk", "tmux")
	w2.onStop = func(role string) { w2.claim(role, "example-repo--8") }
	_, err := w2.down("worker-desk")
	wantReason(t, err, cellop.ReasonRoleBusy)
	if len(w2.closes()) != 0 {
		t.Fatalf("closed after a claim was taken during the wait: %v", w2.closes())
	}
}

// --- rows 2 and 3 ----------------------------------------------------------------------------

// regression: drain-waits-in-flight
func TestDrainWaitsForInFlight(t *testing.T) {
	w := newLCWorld(t)
	w.cadenceRole("worker-desk", "tmux")
	w.claim("worker-desk", "example-repo--7")
	var released time.Time
	w.onSleep = func(n int) {
		if n == 4 {
			w.claims, released = nil, w.clock
		}
	}
	rep := w.lc.drain(drainOpts{Roles: []string{"worker-desk"}, Timeout: 2 * time.Minute})
	if rep.Result.Outcome != cellop.OK || len(rep.Confirm) != 0 {
		t.Fatalf("drain: %+v", rep.Result)
	}
	if released.IsZero() || len(w.stopAt) != 1 || w.stopAt[0].Before(released) {
		t.Fatalf("stop ran at %v, the claim released at %v", w.stopAt, released)
	}
	if got := w.closes(); len(got) != 1 || got[0] != "tmux kill-window -t @7" {
		t.Fatalf("closes %v", got)
	}
	handoff := false
	for _, a := range rep.Result.Actions {
		if a.ID == "handoff:worker-desk" && a.Outcome == cellop.ActionOK {
			handoff = true
		}
	}
	if !handoff {
		t.Fatalf("no handoff record collected: %+v", rep.Result.Actions)
	}
	for _, d := range w.sleeps {
		if d > drainPollMax {
			t.Fatalf("polled every %s, more than %s", d, drainPollMax)
		}
	}
}

// regression: drain-timeout
func TestDrainTimeoutDoesNotStop(t *testing.T) {
	w := newLCWorld(t)
	w.cadenceRole("worker-desk", "tmux")
	w.claim("worker-desk", "example-repo--7")
	rep := w.lc.drain(drainOpts{Roles: []string{"worker-desk"}, Timeout: drainDefaultTimeout})
	r := rep.Result
	if r.Outcome != cellop.Partial || r.ExitCode() != 1 || r.Reason == nil || *r.Reason != cellop.ReasonRoleBusy {
		t.Fatalf("outcome %s reason %v", r.Outcome, r.Reason)
	}
	if next := strings.Join(r.Next, "\n"); !strings.Contains(next, "drain sample --now") || !strings.Contains(next, "resume sample") {
		t.Fatalf("next %v", r.Next)
	}
	if len(w.stops) != 0 || len(w.closes()) != 0 || !w.hasReceipt("worker-desk") {
		t.Fatalf("the timed-out role was stopped: %v %v", w.stops, w.closes())
	}
	if paused, _ := w.lc.Paused("worker-desk"); !paused {
		t.Fatal("the timed-out role should stay up and paused")
	}
	for _, d := range w.sleeps {
		if d > drainPollMax {
			t.Fatalf("polled every %s, more than %s", d, drainPollMax)
		}
	}
	if len(w.claims) != 1 {
		t.Fatal("the in-flight claim was touched")
	}
}

// --- row 5 -----------------------------------------------------------------------------------

// The one test naming the interim mechanism: the cell flags, the pause record, the cadence
// checkpoint and the handoff file. Every other lifecycle test asserts states and transitions.
// regression: interim-mapping
func TestInterimMappingPinned(t *testing.T) {
	w := newLCWorld(t)
	if got, want := w.lc.pauseFlag(""), filepath.Join(w.c.Config, "STOP.cell.sample"); got != want {
		t.Errorf("cell pause flag %s, want %s", got, want)
	}
	if got, want := w.lc.pauseFlag("worker-desk"), filepath.Join(w.c.Config, "STOP.cell.sample.worker-desk"); got != want {
		t.Errorf("role pause flag %s, want %s", got, want)
	}
	if got, want := w.lc.pauseRecordPath(), filepath.Join(w.c.Dir, "run", "lifecycle", "pause-record.json"); got != want {
		t.Errorf("pause record %s, want %s", got, want)
	}
	if got, want := w.lc.handoffPath("worker-desk"), filepath.Join(w.c.Dir, "run", "handoff", "worker-desk.md"); got != want {
		t.Errorf("handoff record %s, want %s", got, want)
	}
	if got, want := surfaceReceiptPath(w.c.cadenceDir("worker-desk")), filepath.Join(w.c.Dir, "run", "cadence", "worker-desk", "surface.json"); got != want {
		t.Errorf("surface receipt %s, want %s", got, want)
	}
	created, err := w.lc.Pause([]string{"worker-desk"})
	if err != nil || len(created) != 1 || created[0] != w.lc.pauseFlag("worker-desk") {
		t.Fatalf("pause created %v (%v)", created, err)
	}
	if _, err := os.Stat(w.lc.pauseRecordPath()); err != nil {
		t.Fatal(err)
	}
	// The flag pause sets is the one the cadence guard reads (desk-containers/16).
	if err := w.c.cadenceGuard("worker-desk"); !errors.Is(err, errStoppedByFlag) {
		t.Fatalf("the cadence guard does not read pause's flag: %v", err)
	}
	if _, err := w.lc.Handoff("worker-desk", w.clock, 0, nil, false); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(w.lc.handoffPath("worker-desk")); err != nil || !strings.Contains(string(b), "## Claims") {
		t.Fatalf("handoff record: %v\n%s", err, b)
	}
	// The cadence checkpoint is the cadence-mode idle signal.
	c := &Cell{Name: "sample", Dir: t.TempDir(), Kind: "house"}
	writeStale(t, c, "worker-desk", deadPID(t), "", 0, "")
	cr, err := c.liveCadence("worker-desk")
	if err != nil || cr.Held || !cr.Running {
		t.Fatalf("the checkpoint's running pass: %+v, %v", cr, err)
	}

	// No other lifecycle test names these paths.
	for _, f := range []string{"lifecycle_test.go", "lifecycle_tty_test.go"} {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		src := string(raw)
		start := strings.Index(src, "func TestInterimMappingPinned(")
		if start >= 0 {
			end := strings.Index(src[start:], "\n}\n")
			src = src[:start] + src[start+end:]
		}
		for _, tok := range []string{"pauseFlag(", "pauseRecordPath(", "handoffPath(", "pause-record", `"handoff"`, "STOP.cell.", "checkpoint.json"} {
			if strings.Contains(src, tok) {
				t.Errorf("%s names the interim mechanism (%s) outside TestInterimMappingPinned", f, tok)
			}
		}
	}
}

// --- row 8j ----------------------------------------------------------------------------------

// regression: drain-cell-op-v1
func TestDrainEmitsCellOpV1(t *testing.T) {
	sv, err := schemacheck.Load(filepath.Join("..", "..", "..", "..", "schemas", "cell-op-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	check := func(name string, r cellop.Result) {
		raw, err := r.JSON()
		if err != nil {
			t.Fatal(err)
		}
		doc, err := schemacheck.Decode(raw)
		if err != nil {
			t.Fatal(err)
		}
		if errs := sv.Validate(doc); len(errs) > 0 {
			t.Errorf("%s: the drain result fails schemas/cell-op-v1.json:\n  %s\n%s", name, strings.Join(errs, "\n  "), raw)
		}
	}
	ok := newLCWorld(t)
	ok.cadenceRole("worker-desk", "tmux")
	check("ok", ok.lc.drain(drainOpts{Roles: []string{"worker-desk"}, Timeout: time.Minute}).Result)
	busy := newLCWorld(t)
	busy.cadenceRole("worker-desk", "tmux")
	busy.claim("worker-desk", "example-repo--7")
	check("partial", busy.lc.drain(drainOpts{Roles: []string{"worker-desk"}, Timeout: time.Minute}).Result)
	blind := newLCWorld(t)
	blind.claimEr = errors.New("forge unreachable")
	r := blind.lc.drain(drainOpts{Roles: []string{"worker-desk"}, Timeout: time.Minute}).Result
	if r.Outcome != cellop.CouldNotCheck {
		t.Errorf("an unreadable idle signal: outcome %s, want could-not-check", r.Outcome)
	}
	check("could-not-check", r)

	// The binary: `drain --json` prints exactly one cell-op-v1 document on stdout.
	cw := newCLIWorld(t, "")
	run := cw.run(t, nil, "drain", "example", "--json")
	res := upDoc(t, run.Stdout)
	if res.Verb != "drain" || run.Code != res.ExitCode() {
		t.Fatalf("drain --json: exit %d, verb %s\n%s", run.Code, res.Verb, run.Stdout)
	}
}
