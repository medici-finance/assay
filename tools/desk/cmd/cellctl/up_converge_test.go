//go:build darwin || linux

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellcadence"
	"github.com/medici-finance/assay/tools/desk/internal/cellop"
	"github.com/medici-finance/assay/tools/desk/internal/cellop/schemacheck"
)

// The window a stub tmux "opens" is this test binary: the init hook below takes the role lease the
// way `cellctl desk` does, waits, records its role child, and holds the lease until told to stop.
// The events file is the timeline the ordering checks read.
func init() {
	role := os.Getenv("CELLCTL_UP_HOLDER")
	if role == "" {
		return
	}
	ev := os.Getenv("CELLCTL_UP_EVENTS")
	logEvent := func(kind string) {
		if f, err := os.OpenFile(ev, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
			fmt.Fprintf(f, "%s %s\n", kind, role)
			f.Close()
		}
	}
	dir := filepath.Join(os.Getenv("CELLCTL_UP_CELLDIR"), "run", "cadence", role)
	lease, err := cellcadence.AcquireRole(dir, cellcadence.RoleLock{Cell: "example", Role: role})
	if err != nil {
		logEvent("lease-failed")
		os.Exit(1)
	}
	logEvent("lease")
	if os.Getenv("CELLCTL_UP_IDLE") == "1" { // a cadenced role resting until its next pass: lease, checkpoint, no child
		now := time.Now().UTC()
		b, _ := json.Marshal(cellcadence.State{Schema: cellcadence.Schema, Cell: "example", Role: role, PID: os.Getpid(),
			Interval: time.Hour, Budget: time.Minute, NextDue: now.Add(40 * time.Minute), Heartbeat: now,
			LastStart: now.Add(-20 * time.Minute), LastFinish: now.Add(-19 * time.Minute), Sequence: 1, Outcome: "ok"})
		if os.WriteFile(filepath.Join(dir, "checkpoint.json"), b, 0o600) != nil {
			logEvent("checkpoint-failed")
			os.Exit(1)
		}
	}
	d, _ := time.ParseDuration(os.Getenv("CELLCTL_UP_DELAY"))
	time.Sleep(d)
	if os.Getenv("CELLCTL_UP_NOCHILD") != "1" {
		_ = lease.RecordChild(os.Getpid())
		logEvent("ready")
	}
	for i := 0; i < 2400; i++ { // never outlives the test by more than two minutes
		if _, err := os.Stat(ev + ".stop"); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = lease.Close()
	os.Exit(0)
}

// upWorld is a house cell whose tmux is a recording stub that "starts" each role window.
type upWorld struct {
	*cliWorld
	events string
}

func newUpWorld(t *testing.T, roles string, delay time.Duration, extra ...string) *upWorld {
	t.Helper()
	w := &upWorld{cliWorld: newCLIWorld(t, "ROLES=\""+roles+"\"\n")}
	w.events = filepath.Join(w.root, "events")
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	holder := "CELLCTL_UP_HOLDER=\"$role\" CELLCTL_UP_EVENTS=" + cockpitQuote(w.events) + " CELLCTL_UP_CELLDIR=" + cockpitQuote(w.cellDir) +
		" CELLCTL_UP_DELAY=" + delay.String()
	for _, e := range extra {
		holder += " " + e
	}
	script := "#!/bin/sh\necho \"$*\" >> " + cockpitQuote(w.events+".calls") + "\n" +
		"case \"$1\" in\n" +
		"has-session) [ -f " + cockpitQuote(w.events+".session") + " ]; exit $? ;;\n" +
		"new-session|new-window)\n" +
		"  for a; do last=\"$a\"; done\n" +
		"  role=$(printf '%s' \"$last\" | sed -n \"s/.*desk 'example' '\\([a-z-]*\\)'.*/\\1/p\")\n" +
		"  : > " + cockpitQuote(w.events+".session") + "\n" +
		"  echo \"start $role\" >> " + cockpitQuote(w.events) + "\n" +
		"  env " + holder + " " + cockpitQuote(self) + " </dev/null >/dev/null 2>&1 &\n" +
		"  ;;\nesac\nexit 0\n"
	if err := os.WriteFile(filepath.Join(w.binDir, "tmux"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.WriteFile(w.events+".stop", nil, 0o600) })
	return w
}

// markSession makes the stub report an existing cockpit session.
func (w *upWorld) markSession(t *testing.T) {
	t.Helper()
	if err := os.WriteFile(w.events+".session", nil, 0o600); err != nil {
		t.Fatal(err)
	}
}

func (w *upWorld) up(t *testing.T, args ...string) cliRun {
	t.Helper()
	return w.run(t, nil, append([]string{"up", "example", "--cockpit", "tmux", "--no-attach"}, args...)...)
}

func (w *upWorld) timeline(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile(w.events)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(raw)), "\n")
}

func (w *upWorld) calls(t *testing.T) string {
	t.Helper()
	raw, _ := os.ReadFile(w.events + ".calls")
	return string(raw)
}

// hold takes the role's lease in this process, as a live `cellctl desk` window does.
func (w *upWorld) hold(t *testing.T, role string) *cellcadence.Lease {
	t.Helper()
	l, err := cellcadence.AcquireRole(w.cell().cadenceDir(role), cellcadence.RoleLock{Cell: "example", Role: role})
	if err != nil {
		t.Fatal(err)
	}
	if err := l.RecordChild(os.Getpid()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	return l
}

func (w *upWorld) cell() *Cell { return &Cell{Name: "example", Dir: w.cellDir, Kind: "house"} }

// upTable reads the `[up] ROLE STATE DETAIL` table: role -> state, and role -> detail.
func upTable(t *testing.T, out string) (states, details map[string]string, order []string) {
	t.Helper()
	states, details = map[string]string{}, map[string]string{}
	for _, l := range strings.Split(out, "\n") {
		rest, ok := strings.CutPrefix(l, "[up] ")
		f := strings.Fields(rest)
		if !ok || len(f) < 2 || f[0] == "ROLE" {
			continue
		}
		states[f[0]] = f[1]
		details[f[0]] = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(rest)[len(f[0]):], " "+f[1]))
		order = append(order, f[0])
	}
	return states, details, order
}

func startedRoles(timeline []string) []string {
	var out []string
	for _, e := range timeline {
		if r, ok := strings.CutPrefix(e, "start "); ok {
			out = append(out, r)
		}
	}
	return out
}

// serialViolations names every place a role's window started before the previous role's launch had
// passed its lease and scratch sweep (the holder's "ready" event). A launch that opens all roles at
// once violates it for every role after the first.
func serialViolations(timeline, roles []string) []string {
	at := map[string]int{}
	for i, e := range timeline {
		at[e] = i
	}
	var bad []string
	for i := 1; i < len(roles); i++ {
		s, hasStart := at["start "+roles[i]]
		r, hasReady := at["ready "+roles[i-1]]
		if !hasStart || !hasReady || s < r {
			bad = append(bad, fmt.Sprintf("%s started before %s was ready", roles[i], roles[i-1]))
		}
	}
	return bad
}

const upFive = "worker-desk verify-desk intake-desk pr-review-desk"

var upFiveRoles = strings.Fields("the-desk " + upFive)

func TestUpConverge(t *testing.T) {
	t.Run("all absent", func(t *testing.T) {
		w := newUpWorld(t, upFive, 50*time.Millisecond)
		r := w.up(t)
		states, _, order := upTable(t, r.Stdout)
		if r.Code != 0 || strings.Join(order, " ") != strings.Join(upFiveRoles, " ") {
			t.Fatalf("exit %d, table order %v\n%s%s", r.Code, order, r.Stdout, r.Stderr)
		}
		for _, role := range upFiveRoles {
			if states[role] != "up" {
				t.Errorf("%s: state %q, want up", role, states[role])
			}
		}
		if got := startedRoles(w.timeline(t)); strings.Join(got, " ") != strings.Join(upFiveRoles, " ") {
			t.Errorf("windows opened: %v", got)
		}
	})

	t.Run("two up, one stale, two absent", func(t *testing.T) {
		w := newUpWorld(t, upFive, 50*time.Millisecond)
		w.markSession(t)
		w.hold(t, "worker-desk")
		w.hold(t, "verify-desk")
		writeStale(t, w.cell(), "intake-desk", deadPID(t), "", 0, "")
		r := w.up(t)
		states, details, _ := upTable(t, r.Stdout)
		want := map[string]string{"the-desk": "up", "worker-desk": "already-up", "verify-desk": "already-up", "intake-desk": "failed", "pr-review-desk": "up"}
		for role, st := range want {
			if states[role] != st {
				t.Errorf("%s: state %q, want %q\n%s", role, states[role], st, r.Stdout)
			}
		}
		if r.Code != 1 {
			t.Errorf("a failed row must exit 1, got %d", r.Code)
		}
		if !strings.Contains(details["intake-desk"], "up example --plan") {
			t.Errorf("the stale row does not name the recovery: %q", details["intake-desk"])
		}
		if got := startedRoles(w.timeline(t)); strings.Join(got, " ") != "the-desk pr-review-desk" {
			t.Errorf("only the absent roles open a window, got %v", got)
		}
		if strings.Contains(w.calls(t), "new-session") {
			t.Errorf("an existing session was re-created:\n%s", w.calls(t))
		}
		if _, err := os.Stat(filepath.Join(w.cellDir, "run", "cadence", "intake-desk", "checkpoint.json")); err != nil {
			t.Errorf("plain up cleared the stale checkpoint: %v", err)
		}
	})

	t.Run("role subset", func(t *testing.T) {
		w := newUpWorld(t, upFive, 50*time.Millisecond)
		r := w.up(t, "--role", "verify-desk", "--role", "the-desk")
		states, _, order := upTable(t, r.Stdout)
		if r.Code != 0 || strings.Join(order, " ") != "the-desk verify-desk" || states["verify-desk"] != "up" {
			t.Fatalf("exit %d, rows %v\n%s%s", r.Code, order, r.Stdout, r.Stderr)
		}
		if got := startedRoles(w.timeline(t)); strings.Join(got, " ") != "the-desk verify-desk" {
			t.Errorf("windows opened: %v", got)
		}
		bad := w.up(t, "--role", "nonesuch")
		if bad.Code == 0 || !strings.Contains(bad.Stderr, "nonesuch") {
			t.Errorf("an unknown --role was accepted: %d %s", bad.Code, bad.Stderr)
		}
	})

	t.Run("a launch that never takes its lease fails its row", func(t *testing.T) {
		w := newUpWorld(t, "worker-desk", 0, "CELLCTL_UP_NOCHILD=1")
		r := w.up(t, "--role", "worker-desk", "--lease-wait", "1s")
		states, details, _ := upTable(t, r.Stdout)
		if r.Code != 1 || states["worker-desk"] != "failed" || details["worker-desk"] == "" {
			t.Fatalf("exit %d, rows %v %v\n%s", r.Code, states, details, r.Stdout)
		}
	})

	t.Run("a cadenced role resting until its next pass is scheduled, not failed", func(t *testing.T) {
		w := newUpWorld(t, "worker-desk", 0, "CELLCTL_UP_NOCHILD=1", "CELLCTL_UP_IDLE=1")
		r := w.up(t, "--role", "worker-desk", "--lease-wait", "20s")
		states, details, _ := upTable(t, r.Stdout)
		if r.Code != 0 || states["worker-desk"] != "scheduled" || !strings.Contains(details["worker-desk"], "next pass due") {
			t.Fatalf("exit %d, rows %v %v\n%s%s", r.Code, states, details, r.Stdout, r.Stderr)
		}
		again := w.up(t, "--role", "worker-desk")
		states, _, _ = upTable(t, again.Stdout)
		if again.Code != 0 || states["worker-desk"] != "already-up" {
			t.Errorf("an immediate re-run: exit %d, rows %v\n%s", again.Code, states, again.Stdout)
		}
	})

	t.Run("dry run launches nothing", func(t *testing.T) {
		w := newUpWorld(t, upFive, 50*time.Millisecond)
		w.hold(t, "worker-desk")
		r := w.run(t, []string{"DRY_RUN=1"}, "up", "example", "--cockpit", "tmux", "--no-attach")
		if r.Code != 0 || !strings.Contains(r.Stdout, "[dry-run] worker-desk: already-up") ||
			!strings.Contains(r.Stdout, "[dry-run] verify-desk: ") || strings.Contains(r.Stdout, "[dry-run] verify-desk: already-up") {
			t.Fatalf("dry run: %d\n%s", r.Code, r.Stdout)
		}
		if len(w.timeline(t)) != 0 || w.calls(t) != "" {
			t.Errorf("a dry run reached tmux: %v %q", w.timeline(t), w.calls(t))
		}
	})
}

func TestUpConvergeDryRunJSON(t *testing.T) {
	w := newUpWorld(t, upFive, 50*time.Millisecond)
	w.hold(t, "worker-desk")
	r := w.run(t, []string{"DRY_RUN=1"}, "up", "example", "--cockpit", "tmux", "--no-attach", "--json")
	var doc struct {
		Outcome string `json:"outcome"`
		Roles   []struct {
			Role  string `json:"role"`
			State string `json:"state"`
		} `json:"roles"`
	}
	if err := json.Unmarshal([]byte(r.Stdout), &doc); err != nil {
		t.Fatalf("a dry run under --json wrote no document: %v\n%s", err, r.Stdout)
	}
	got := map[string]string{}
	for _, ro := range doc.Roles {
		got[ro.Role] = ro.State
	}
	if r.Code != 0 || doc.Outcome != "ok" || got["worker-desk"] != "already-up" || got["verify-desk"] != "would-start" || len(w.timeline(t)) != 0 {
		t.Errorf("dry run json: exit %d outcome %q roles %v", r.Code, doc.Outcome, got)
	}
}

func TestUpConvergeNeverTouchesLiveRole(t *testing.T) {
	w := newUpWorld(t, upFive, 50*time.Millisecond)
	w.markSession(t)
	for _, role := range upFiveRoles {
		w.hold(t, role)
	}
	r := w.up(t)
	states, _, _ := upTable(t, r.Stdout)
	if r.Code != 0 {
		t.Fatalf("exit %d\n%s%s", r.Code, r.Stdout, r.Stderr)
	}
	for _, role := range upFiveRoles {
		if states[role] != "already-up" {
			t.Errorf("%s: state %q, want already-up", role, states[role])
		}
	}
	if tl := w.timeline(t); len(tl) != 0 {
		t.Errorf("a live role was launched a second time: %v", tl)
	}
	for _, call := range strings.Split(w.calls(t), "\n") {
		for _, verb := range []string{"new-session", "new-window", "kill-", "respawn", "send-keys"} {
			if strings.HasPrefix(call, verb) {
				t.Errorf("up touched a live cockpit with %q", call)
			}
		}
	}
	for _, role := range upFiveRoles {
		if busy, err := cellcadence.Probe(w.cell().cadenceDir(role)); err != nil || !busy {
			t.Errorf("%s: the lease was disturbed (busy=%v err=%v)", role, busy, err)
		}
	}
}

// regression: F-up-converge-serial — the next window opens only after the previous role passed
// lease acquisition and the scratch sweep, never all at once.
func TestUpConvergeSerialLaunch(t *testing.T) {
	w := newUpWorld(t, upFive, 300*time.Millisecond)
	r := w.up(t)
	if r.Code != 0 {
		t.Fatalf("exit %d\n%s%s", r.Code, r.Stdout, r.Stderr)
	}
	tl := w.timeline(t)
	if got := startedRoles(tl); strings.Join(got, " ") != strings.Join(upFiveRoles, " ") {
		t.Fatalf("windows opened: %v", got)
	}
	if bad := serialViolations(tl, upFiveRoles); len(bad) != 0 {
		t.Fatalf("launches overlapped: %v\ntimeline: %v", bad, tl)
	}

	// The check bites: the timeline of a launcher that opens every window at once is refused.
	var allAtOnce []string
	for _, role := range upFiveRoles {
		allAtOnce = append(allAtOnce, "start "+role)
	}
	for _, role := range upFiveRoles {
		allAtOnce = append(allAtOnce, "lease "+role, "ready "+role)
	}
	if bad := serialViolations(allAtOnce, upFiveRoles); len(bad) != len(upFiveRoles)-1 {
		t.Fatalf("a launch-everything-at-once mutant passed the ordering check: %v", bad)
	}
}

// upDoc decodes the one document `up --json` printed and validates it against the schema.
func upDoc(t *testing.T, stdout string) cellop.Result {
	t.Helper()
	doc, err := schemacheck.Decode([]byte(stdout))
	if err != nil {
		t.Fatalf("stdout is not exactly one JSON document: %v\n%s", err, stdout)
	}
	sv, err := schemacheck.Load(filepath.Join("..", "..", "..", "..", "schemas", "cell-op-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	if errs := sv.Validate(doc); len(errs) > 0 {
		t.Fatalf("the document fails schemas/cell-op-v1.json:\n  %s\n%s", strings.Join(errs, "\n  "), stdout)
	}
	var res cellop.Result
	dec := json.NewDecoder(strings.NewReader(stdout))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&res); err != nil {
		t.Fatal(err)
	}
	return res
}

func roleState(res cellop.Result, role string) string {
	for _, r := range res.Roles {
		if r.Role == role && r.State != nil {
			return *r.State
		}
	}
	return ""
}

func TestUpEmitsCellOpV1(t *testing.T) {
	w := newUpWorld(t, "worker-desk verify-desk", 50*time.Millisecond)
	w.markSession(t)
	w.hold(t, "the-desk")
	writeStale(t, w.cell(), "worker-desk", deadPID(t), "", 0, "")
	r := w.up(t, "--json")
	res := upDoc(t, r.Stdout)
	if len(res.Roles) != 3 {
		t.Fatalf("want one entry per targeted role, got %+v", res.Roles)
	}
	for role, want := range map[string]string{"the-desk": "already-up", "worker-desk": "failed", "verify-desk": "up"} {
		if got := roleState(res, role); got != want {
			t.Errorf("%s: state %q, want %q", role, got, want)
		}
	}
	if res.Verb != "up" || res.Outcome == cellop.OK || r.Code != 1 {
		t.Errorf("a failed role must show in the outcome and exit 1: verb %q outcome %q exit %d", res.Verb, res.Outcome, r.Code)
	}

	// All up: ok, exit 0, still one document and nothing else on stdout.
	w2 := newUpWorld(t, "worker-desk", 50*time.Millisecond)
	r = w2.up(t, "--json")
	res = upDoc(t, r.Stdout)
	if r.Code != 0 || res.Outcome != cellop.OK || roleState(res, "worker-desk") != "up" {
		t.Errorf("exit %d, outcome %q, rows %+v", r.Code, res.Outcome, res.Roles)
	}
}

// A role that came alive between the plan and the apply is refused plan-stale, naming the role,
// and nothing is launched; a fresh plan then reconciles the stale role and brings the cell up.
func TestUpPlanApply(t *testing.T) {
	w := newUpWorld(t, "worker-desk verify-desk", 50*time.Millisecond)
	w.hold(t, "the-desk")
	stale := w.cell()
	writeStale(t, stale, "worker-desk", deadPID(t), "", 0, "")

	plan := func() cellop.Plan {
		t.Helper()
		r := w.run(t, nil, "up", "example", "--plan", "--json")
		p, err := cellop.DecodePlan([]byte(r.Stdout))
		if r.Code != 0 || err != nil {
			t.Fatalf("up --plan --json: exit %d, %v\n%s%s", r.Code, err, r.Stdout, r.Stderr)
		}
		return p
	}
	p := plan()
	if len(p.Actions) != 1 || p.Actions[0].ID != "reconcile-worker-desk" {
		t.Fatalf("one plan, one action per stale role: %+v", p.Actions)
	}
	if len(w.timeline(t)) != 0 {
		t.Fatalf("--plan launched something: %v", w.timeline(t))
	}
	if _, err := os.Stat(filepath.Join(w.cellDir, "run", "cadence", "worker-desk", "checkpoint.json")); err != nil {
		t.Fatalf("--plan cleared the stale checkpoint: %v", err)
	}

	// worker-desk comes alive between plan and apply.
	live := w.hold(t, "worker-desk")
	args := []string{"up", "example", "--cockpit", "tmux", "--apply", p.ID, "--json", "--lease-wait", "20s"}
	code, out := w.onTTY(t, p.ConfirmCode, args...)
	res := upDocFromTTY(t, out)
	if code != 5 || res.Outcome != cellop.Refused || upReason(res) != cellop.ReasonPlanStale || !strings.Contains(out, "worker-desk") {
		t.Fatalf("apply after the role came alive: exit %d, outcome %q, reason %q\n%s", code, res.Outcome, upReason(res), out)
	}
	if len(w.timeline(t)) != 0 {
		t.Fatalf("a refused apply launched: %v", w.timeline(t))
	}
	live.Close()

	// A fresh plan applies: the stale role is reconciled and launched, the live one left alone.
	writeStale(t, stale, "worker-desk", deadPID(t), "", 0, "")
	p = plan()
	args[5] = p.ID
	code, out = w.onTTY(t, p.ConfirmCode, args...)
	res = upDocFromTTY(t, out)
	want := map[string]string{"the-desk": "already-up", "worker-desk": "reconciled+up", "verify-desk": "up"}
	for role, st := range want {
		if got := roleState(res, role); got != st {
			t.Errorf("%s: state %q, want %q\n%s", role, got, st, out)
		}
	}
	if code != 0 || res.Outcome != cellop.OK {
		t.Errorf("apply: exit %d outcome %q\n%s", code, res.Outcome, out)
	}
	if got := startedRoles(w.timeline(t)); strings.Join(got, " ") != "worker-desk verify-desk" {
		t.Errorf("windows opened: %v", got)
	}
}

func upReason(r cellop.Result) string {
	if r.Reason == nil {
		return ""
	}
	return *r.Reason
}

// upDocFromTTY pulls the cell-op document out of a terminal transcript.
func upDocFromTTY(t *testing.T, out string) cellop.Result {
	t.Helper()
	for _, l := range strings.Split(out, "\n") {
		if l = strings.TrimSpace(l); strings.HasPrefix(l, `{"schema":"cell-op-v1"`) {
			return upDoc(t, l)
		}
	}
	t.Fatalf("no cell-op-v1 document in the transcript:\n%s", out)
	return cellop.Result{}
}

const upConfirmPrompt = "Type the confirm code to apply: "

// onTTY runs cellctl with stdin, stdout and stderr on a pseudo-terminal and types code at the
// confirm prompt.
func (w *upWorld) onTTY(t *testing.T, code string, args ...string) (int, string) {
	t.Helper()
	master, slave, err := openPTY()
	if err != nil {
		t.Skipf("no pseudo-terminal available: %v", err)
	}
	cmd := exec.Command(cellctlBinary(t), args...)
	cmd.Env = w.baseEnv()
	cmd.Dir = w.root
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = slave, slave, slave
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	slave.Close()
	var mu sync.Mutex
	var term bytes.Buffer
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		buf := make([]byte, 4096)
		for {
			n, err := master.Read(buf)
			mu.Lock()
			term.Write(buf[:n])
			mu.Unlock()
			if err != nil {
				return
			}
		}
	}()
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	deadline := time.After(90 * time.Second)
	tick := time.NewTicker(25 * time.Millisecond)
	defer tick.Stop()
	answered := false
	var werr error
loop:
	for {
		select {
		case werr = <-waited:
			break loop
		case <-deadline:
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			werr = <-waited
			t.Errorf("cellctl %v timed out", args)
			break loop
		case <-tick.C:
			mu.Lock()
			seen := term.String()
			mu.Unlock()
			if !answered && strings.Contains(seen, upConfirmPrompt) {
				answered = true
				_, _ = master.Write([]byte(code + "\n"))
			}
		}
	}
	select {
	case <-readDone:
	case <-time.After(time.Second):
	}
	master.Close()
	<-readDone
	exit := 0
	if werr != nil {
		ee, ok := werr.(*exec.ExitError)
		if !ok {
			t.Fatal(werr)
		}
		exit = ee.ExitCode()
	}
	mu.Lock()
	defer mu.Unlock()
	return exit, strings.ReplaceAll(term.String(), "\r", "")
}
