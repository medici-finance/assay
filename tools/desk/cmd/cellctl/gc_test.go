package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellcadence"
)

// regression: #5371
func TestGCInterval(t *testing.T) {
	for raw, want := range map[string]time.Duration{"": time.Hour, "0": 0, "2h": 2 * time.Hour, "1m": time.Minute, "30s": -1, "-1m": -1, "soon": -1} {
		c := &Cell{Name: "fixture", Dir: t.TempDir(), Env: &Env{vals: map[string]string{"CELL_GC_INTERVAL": raw}}}
		got, err := c.gcInterval()
		if want < 0 {
			if _, prepErr := c.prepareLaunch(); err == nil || prepErr == nil {
				t.Fatalf("%q accepted: %v %v", raw, err, prepErr)
			}
			continue
		}
		if err != nil || got != want {
			t.Fatalf("%q = %v %v, want %v", raw, got, err, want)
		}
	}
}

// regression: #5371
func TestGCDeskLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run", "gc", "worker-desk.stamp")
	now := time.Now()
	first, outcome, _ := acquireGCLock(path, time.Hour, now)
	if outcome != gcLockAcquired {
		t.Fatalf("first gc did not take the lock: %v", outcome)
	}
	// A second gc for the same desk, scheduled or manual, never runs beside a live one.
	for _, ttl := range []time.Duration{time.Hour, 0} {
		if l, o, detail := acquireGCLock(path, ttl, now); o != gcLockHeld || l != nil || !strings.Contains(detail, "pid") {
			t.Fatalf("second gc ran beside a live one (ttl %s): %v %q", ttl, o, detail)
		}
	}
	first.release(true, now)
	if _, o, _ := acquireGCLock(path, time.Hour, now.Add(30*time.Minute)); o != gcLockRecent {
		t.Fatalf("scheduled gc inside the interval was not skipped: %v", o)
	}
	l, o, _ := acquireGCLock(path, 0, now.Add(30*time.Minute))
	if o != gcLockAcquired {
		t.Fatalf("manual gc was debounced: %v", o)
	}
	l.release(false, now)
	if l, o, _ = acquireGCLock(path, time.Hour, now.Add(time.Hour)); o != gcLockAcquired {
		t.Fatalf("gc due after the interval was skipped: %v", o)
	}
	l.release(true, now)
	// The recency half fails open: a damaged or future stamp is no stamp.
	for _, body := range []string{"not json", `{"schema":"other","finishedAt":"` + now.UTC().Format(time.RFC3339Nano) + `"}`,
		`{"schema":"` + gcStampSchema + `","finishedAt":"` + now.Add(time.Hour).UTC().Format(time.RFC3339Nano) + `"}`} {
		must(t, os.WriteFile(path, []byte(body), 0o600))
		l, o, _ := acquireGCLock(path, time.Hour, now)
		if o != gcLockAcquired {
			t.Fatalf("unusable stamp %q held gc: %v", body, o)
		}
		l.release(true, now)
	}
}

// regression: #5371
func TestGCTargetFailureIsolated(t *testing.T) {
	ran := 0
	targets := []gcTarget{
		{name: "a", run: func(context.Context) []gcLine {
			ran++
			return []gcLine{{Target: "a", Outcome: "failed", Detail: "boom"}}
		}},
		{name: "b", run: func(context.Context) []gcLine { ran++; die("refused"); return nil }},
		{name: "c", run: func(context.Context) []gcLine { ran++; panic(errors.New("crash")) }},
		{name: "d", run: func(context.Context) []gcLine { ran++; return []gcLine{{Target: "d", Outcome: "applied"}} }},
	}
	lines := runGC(context.Background(), targets)
	if ran != 4 || len(lines) != 4 {
		t.Fatalf("a failing target stopped the rest: ran=%d lines=%+v", ran, lines)
	}
	for i, want := range []string{"failed", "failed", "failed", "applied"} {
		if lines[i].Outcome != want || lines[i].Target != targets[i].name {
			t.Fatalf("line %d = %+v, want %s", i, lines[i], want)
		}
	}
}

// gcStubDir writes an executable cellctl stub that appends a tag and its argv to record.
func gcStubDir(t *testing.T, record, tag string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell stub")
	}
	dir := t.TempDir()
	body := "#!/bin/sh\nprintf '%s %s\\n' '" + tag + "' \"$*\" >> '" + record + "'\n"
	must(t, os.WriteFile(filepath.Join(dir, "cellctl"), []byte(body), 0o755))
	return dir
}

func gcCell(t *testing.T, tools string) *Cell {
	t.Helper()
	dir := t.TempDir()
	must(t, os.MkdirAll(filepath.Join(dir, "run"), 0o700))
	return &Cell{Name: "fixture", Dir: dir, Config: filepath.Join(dir, "config"),
		Env: &Env{vals: map[string]string{"DESK_TOOLS_BIN": tools, "CELL_GC_INTERVAL": "1h"}}}
}

// The supervisor executes whatever cellctl is installed at each tick, never its own image.
//
// regression: #5371
func TestGCScheduleResolvesEachTick(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // no fallback cellctl from the host's PATH
	record := filepath.Join(t.TempDir(), "record")
	tools := gcStubDir(t, record, "v1")
	c := gcCell(t, tools)
	ctx := context.Background()
	if !c.scheduledGCTick(ctx, "worker-desk", time.Hour, nil) {
		t.Fatal("due tick did not start a child")
	}
	// A re-pin replaces the installed binary; the next tick runs the new one.
	must(t, os.WriteFile(filepath.Join(tools, "cellctl"), []byte("#!/bin/sh\nprintf 'v2 %s\\n' \"$*\" >> '"+record+"'\n"), 0o755))
	if !c.scheduledGCTick(ctx, "worker-desk", time.Hour, nil) {
		t.Fatal("second due tick did not start a child")
	}
	b, err := os.ReadFile(record)
	must(t, err)
	if string(b) != "v1 gc fixture --role worker-desk --apply --scheduled\nv2 gc fixture --role worker-desk --apply --scheduled\n" {
		t.Fatalf("child argv/resolution = %q", b)
	}
	// Missing install: nothing runs in-process; the tick is logged could-not-check.
	must(t, os.Remove(filepath.Join(tools, "cellctl")))
	if c.scheduledGCTick(ctx, "worker-desk", time.Hour, nil) {
		t.Fatal("tick started with no installed cellctl")
	}
	log, err := os.ReadFile(filepath.Join(c.Dir, "run", "gc.log"))
	if err != nil || !strings.Contains(string(log), "could-not-check: no installed cellctl") {
		t.Fatalf("missing install not logged: %s %v", log, err)
	}
}

// regression: #5371
func TestGCScheduleSkips(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	record := filepath.Join(t.TempDir(), "record")
	c := gcCell(t, gcStubDir(t, record, "x"))
	now := time.Now()
	// A fresh stamp from this desk's own last pass: no child.
	l, _, _ := acquireGCLock(c.gcStampPath("worker-desk"), 0, now)
	l.release(true, now)
	if c.scheduledGCTick(context.Background(), "worker-desk", time.Hour, nil) {
		t.Fatal("tick ran inside the interval")
	}
	must(t, os.Remove(c.gcStampPath("worker-desk")))
	// A stopping supervisor starts nothing: cancelled context, or a stop flag on its role.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if c.scheduledGCTick(ctx, "worker-desk", time.Hour, nil) {
		t.Fatal("cancelled supervisor started a gc")
	}
	must(t, os.MkdirAll(c.Config, 0o700))
	must(t, os.WriteFile(filepath.Join(c.Config, "STOP"), nil, 0o600))
	if c.scheduledGCTick(context.Background(), "worker-desk", time.Hour, nil) {
		t.Fatal("stopped supervisor started a gc")
	}
	if _, err := os.Stat(record); !os.IsNotExist(err) {
		t.Fatal("a skipped tick ran the child", err)
	}
}

// TestGCScheduleFixture is a compiled subprocess: an interactive or cadenced supervisor with
// the scheduled gc on; the interactive child succeeds only once the stub cellctl has run.
func TestGCScheduleFixture(t *testing.T) {
	mode := os.Getenv("CELL_GC_FIXTURE")
	if mode == "" {
		return
	}
	if mode == "child" {
		until := time.Now().Add(3 * time.Second)
		for time.Now().Before(until) {
			if _, err := os.Stat(os.Getenv("CELL_GC_RECORD")); err == nil {
				os.Exit(0)
			}
			time.Sleep(10 * time.Millisecond)
		}
		os.Exit(3)
	}
	gcFirstDelay = 20 * time.Millisecond
	dir := os.Getenv("CELL_GC_DIR")
	c := &Cell{Name: "fixture", Dir: dir, Config: filepath.Join(dir, "config"),
		Env: &Env{vals: map[string]string{"CELL_GC_INTERVAL": "1m", "DESK_TOOLS_BIN": os.Getenv("CELL_GC_TOOLS")}}}
	l, err := cellcadence.Acquire(c.cadenceDir("worker-desk"))
	if err != nil {
		os.Exit(91)
	}
	c.cadenceLease = l
	if mode == "cadence" {
		c.Cadence = &cadenceOptions{Interval: time.Hour, Budget: 30 * time.Second}
		env := fixtureProcessEnv("CELL_SCRATCH_FIXTURE=1", "ASSAY_SOURCE_REVISION=fixture-revision", "DESK_LOOP=worker-desk", "SCRATCH_EVIDENCE="+filepath.Join(dir, "result"))
		c.runCadencedHarness("worker-desk", "claude", []string{os.Args[0], "fixture prompt"}, env, dir)
		os.Exit(0)
	}
	defer func() {
		if code, ok := recover().(exitCode); ok {
			os.Exit(code.code)
		}
		os.Exit(92)
	}()
	env := fixtureProcessEnv("CELL_GC_FIXTURE=child", "ASSAY_SOURCE_REVISION=fixture-revision")
	c.runInteractiveHarness("worker-desk", []string{os.Args[0], "-test.run=^TestGCScheduleFixture$"}, env, dir)
}

// regression: #5371
func TestGCScheduleWiring(t *testing.T) {
	for _, mode := range []string{"interactive", "cadence"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			must(t, os.MkdirAll(filepath.Join(dir, "run", "scratch"), 0o700))
			must(t, os.MkdirAll(filepath.Join(dir, "config"), 0o700))
			record := filepath.Join(t.TempDir(), "record")
			tools := gcStubDir(t, record, "installed")
			cmd := exec.Command(os.Args[0], "-test.run=^TestGCScheduleFixture$")
			cmd.Env = fixtureProcessEnv("CELL_GC_FIXTURE="+mode, "CELL_GC_DIR="+dir, "CELL_GC_TOOLS="+tools, "CELL_GC_RECORD="+record)
			cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
			cmd.WaitDelay = 200 * time.Millisecond
			must(t, cmd.Start())
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			t.Cleanup(func() {
				if cmd.Process.Signal(os.Interrupt) == nil {
					select {
					case err := <-done:
						done <- err
					case <-time.After(10 * time.Second):
					}
				}
				_ = cmd.Process.Kill()
				<-done
			})
			if mode == "interactive" {
				select {
				case err := <-done:
					done <- err
					if err != nil {
						t.Fatal("interactive child never saw the scheduled gc:", err)
					}
				case <-time.After(10 * time.Second):
					t.Fatal("interactive supervisor hung")
				}
			} else {
				until := time.Now().Add(5 * time.Second)
				for {
					if _, err := os.Stat(record); err == nil {
						break
					}
					if time.Now().After(until) {
						t.Fatal("cadenced supervisor never ran the scheduled gc")
					}
					time.Sleep(10 * time.Millisecond)
				}
			}
			b, err := os.ReadFile(record)
			must(t, err)
			if !strings.HasPrefix(string(b), "installed gc fixture --role worker-desk --apply --scheduled\n") {
				t.Fatalf("supervisor ran %q", b)
			}
		})
	}
}

// Plain gc reclaims nothing and calls every target's dry form; --apply acts and stamps.
//
// regression: #5371
func TestGCDryRunDefault(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stub")
	}
	w := newCLIWorld(t, "")
	root := filepath.Join(w.cellDir, "run", "scratch")
	must(t, os.MkdirAll(root, 0o700))
	build, record, kept := plantAbandoned(t, root)
	argvFile := filepath.Join(w.root, "deskwt-argv")
	must(t, os.WriteFile(filepath.Join(w.deskToolsDir(), "deskwt"), []byte("#!/bin/sh\nprintf '%s %s\\n' \"$DESK_LOOP\" \"$*\" >> '"+argvFile+"'\necho 'prune: fixture done'\n"), 0o755))

	run := w.run(t, nil, "gc", "example")
	if run.Code != 0 {
		t.Fatalf("dry run exit %d\n%s\n%s", run.Code, run.Stdout, run.Stderr)
	}
	lines := gcParse(t, run.Stdout)
	if len(lines) != 3 || lines["scratch"].Outcome != "planned" || *lines["scratch"].Items != 1 ||
		lines["go-cache"].Outcome != "skipped" || lines["worktrees:example-org/example-repo"].Outcome != "planned" {
		t.Fatalf("dry-run report = %s", run.Stdout)
	}
	if !scratchExists(t, build) {
		t.Fatal("dry run reclaimed scratch")
	}
	if _, err := os.Stat(filepath.Join(w.cellDir, "run", "gc")); !os.IsNotExist(err) {
		t.Fatal("dry run took the gc lock", err)
	}

	run = w.run(t, nil, "gc", "example", "--role", "worker-desk", "--apply")
	if run.Code != 0 {
		t.Fatalf("apply exit %d\n%s\n%s", run.Code, run.Stdout, run.Stderr)
	}
	lines = gcParse(t, run.Stdout)
	if lines["scratch"].Outcome != "applied" || *lines["scratch"].Reclaimed <= 0 || lines["worktrees:example-org/example-repo"].Outcome != "applied" {
		t.Fatalf("apply report = %s", run.Stdout)
	}
	if scratchExists(t, build) || !scratchExists(t, record) || !scratchExists(t, kept) {
		t.Fatal("apply reclaimed the wrong scope")
	}
	b, err := os.ReadFile(argvFile)
	must(t, err)
	repo := w.repoDir
	if string(b) != " prune --repo "+repo+" --no-cache --dry-run\nworker-desk prune --repo "+repo+" --no-cache\n" {
		t.Fatalf("deskwt argv = %q", b)
	}
	if st := readGCStamp(filepath.Join(w.cellDir, "run", "gc", "worker-desk.stamp")); st == nil || st.FinishedAt == "" || !st.Clean {
		t.Fatalf("apply left no finished stamp: %+v", st)
	}
	// The desk's scheduled form now skips: its own pass above is inside CELL_GC_INTERVAL.
	run = w.run(t, nil, "gc", "example", "--role", "worker-desk", "--apply", "--scheduled")
	if run.Code != 0 || !strings.Contains(run.Stdout, `"outcome":"skipped"`) || strings.Contains(run.Stdout, `"target":"scratch"`) {
		t.Fatalf("scheduled gc inside the interval ran: %d %s", run.Code, run.Stdout)
	}
	// Another desk's pass does not depend on, or wait for, that one: each desk cleans for itself.
	run = w.run(t, nil, "gc", "example", "--role", "the-desk", "--apply", "--scheduled")
	if run.Code != 0 || !strings.Contains(run.Stdout, `"target":"scratch"`) {
		t.Fatalf("a second desk's scheduled gc was held by the first: %d %s", run.Code, run.Stdout)
	}
	// A failing target is reported, the rest still run, and the exit says so.
	must(t, os.WriteFile(filepath.Join(w.deskToolsDir(), "deskwt"), []byte("#!/bin/sh\necho 'prune: refused'\nexit 4\n"), 0o755))
	run = w.run(t, nil, "gc", "example")
	lines = gcParse(t, run.Stdout)
	if run.Code != 1 || lines["scratch"].Outcome != "planned" || lines["worktrees:example-org/example-repo"].Outcome != "failed" {
		t.Fatalf("failing target: %d %s", run.Code, run.Stdout)
	}
}

func gcParse(t *testing.T, out string) map[string]gcLine {
	t.Helper()
	lines := map[string]gcLine{}
	for _, raw := range strings.Split(strings.TrimSpace(out), "\n") {
		var l gcLine
		if err := json.Unmarshal([]byte(raw), &l); err != nil {
			t.Fatalf("not a report line: %q (%v)", raw, err)
		}
		lines[l.Target] = l
	}
	return lines
}
