package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellcadence"
	"github.com/medici-finance/assay/tools/desk/internal/cellscratch"
)

// plantAbandoned leaves what an abandoned agent session leaves: a finished,
// reaped, never-acknowledged task, a month old, holding compiler output beside
// ordinary work. It returns the build directory, the record and the other work.
func plantAbandoned(t *testing.T, root string) (build, record, kept string) {
	t.Helper()
	s, err := cellscratch.Open(root)
	must(t, err)
	r, err := s.Begin("abandoned", "worker-desk", "fixture-revision")
	must(t, err)
	build = filepath.Join(r.Work(), "go-build7")
	must(t, os.MkdirAll(filepath.Join(build, "b001"), 0700))
	must(t, os.WriteFile(filepath.Join(build, "b001", "x.a"), []byte(strings.Repeat("b", 4096)), 0600))
	kept = filepath.Join(r.Work(), "notes.txt")
	must(t, os.WriteFile(kept, []byte("unacknowledged work"), 0600))
	must(t, r.Finish(1, true))
	must(t, r.Close())
	must(t, s.Close())
	record = filepath.Join(root, r.Record.ID, "record.json")
	b, err := os.ReadFile(record)
	must(t, err)
	var rec cellscratch.Record
	must(t, json.Unmarshal(b, &rec))
	rec.Created = rec.Created.Add(-30 * 24 * time.Hour)
	rec.Finished = rec.Finished.Add(-30 * 24 * time.Hour)
	b, err = json.Marshal(rec)
	must(t, err)
	must(t, os.WriteFile(record, b, 0600))
	return build, record, kept
}

func scratchGone(path string, within time.Duration) bool {
	until := time.Now().Add(within)
	for time.Now().Before(until) {
		if _, err := os.Lstat(path); os.IsNotExist(err) {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

func scratchExists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Lstat(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return err == nil
}

func fastScratchMaintenance(t *testing.T) {
	t.Helper()
	delay, floor := scratchSweepDelay, scratchSweepIntervalFloor
	scratchSweepDelay, scratchSweepIntervalFloor = 20*time.Millisecond, time.Millisecond
	t.Cleanup(func() { scratchSweepDelay, scratchSweepIntervalFloor = delay, floor })
}

// regression: #5353
func TestScratchSweepInterval(t *testing.T) {
	for raw, want := range map[string]time.Duration{"": scratchSweepIntervalDefault, "0": 0, "2h": 2 * time.Hour, "1m": time.Minute, "30s": -1, "-1m": -1, "soon": -1} {
		c := &Cell{Name: "fixture", Dir: t.TempDir(), Env: &Env{vals: map[string]string{"CELL_SCRATCH_SWEEP_INTERVAL": raw}}}
		got, err := c.scratchSweepInterval()
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

// The launch path admits a task without walking history; the supervisor's
// periodic sweep, started beside the child, reclaims abandoned build output.
//
// regression: #5353
func TestScratchDeferredSweep(t *testing.T) {
	fastScratchMaintenance(t)
	c := &Cell{Name: "fixture", Dir: t.TempDir(), Config: t.TempDir(), Env: &Env{vals: map[string]string{"CELL_SCRATCH_SWEEP_INTERVAL": "10ms"}}}
	root := filepath.Join(c.Dir, "run", "scratch")
	must(t, os.MkdirAll(root, 0700))
	build, record, kept := plantAbandoned(t, root)
	launch, env, err := c.beginScratch("worker-desk", []string{"ASSAY_SOURCE_REVISION=fixture-revision"})
	must(t, err)
	if !scratchExists(t, build) {
		t.Fatal("launch path swept history")
	}
	work := ""
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, "TMPDIR="); ok {
			work = v
		}
	}
	must(t, os.WriteFile(filepath.Join(work, "go-build9"), []byte("live"), 0600))
	stop := c.startScratchMaintenance(context.Background(), "worker-desk", nil)
	if !scratchGone(build, 3*time.Second) {
		stop()
		t.Fatal("running supervisor never reclaimed abandoned build output")
	}
	stop()
	for _, path := range []string{record, kept, filepath.Join(work, "go-build9")} {
		if !scratchExists(t, path) {
			t.Fatal("maintenance removed more than abandoned build output", path)
		}
	}
	later, _, _ := plantAbandoned(t, root)
	_ = os.Remove(filepath.Join(root, "maintenance.stamp"))
	time.Sleep(200 * time.Millisecond)
	if !scratchExists(t, later) {
		t.Fatal("maintenance outlived its supervisor")
	}
	result := cellcadence.Result{}
	launch.finish(&result)
	must(t, result.Err)
	b, err := os.ReadFile(filepath.Join(c.Dir, "run", "scratch-maintenance.log"))
	if err != nil || !strings.Contains(string(b), `"ran":true`) || !strings.Contains(string(b), `"reclaim-build":1`) {
		t.Fatalf("maintenance summary missing: %s %v", b, err)
	}
}

// TestScratchMaintainFixture is a compiled subprocess: an interactive or cadenced
// supervisor whose child succeeds only once periodic maintenance has run beside it.
func TestScratchMaintainFixture(t *testing.T) {
	mode := os.Getenv("CELL_MAINT_FIXTURE")
	if mode == "" {
		return
	}
	if mode == "child" {
		if scratchGone(os.Getenv("CELL_MAINT_TARGET"), 3*time.Second) {
			os.Exit(0)
		}
		os.Exit(3)
	}
	scratchSweepDelay = 20 * time.Millisecond
	dir := os.Getenv("CELL_MAINT_DIR")
	c := &Cell{Name: "fixture", Dir: dir, Config: filepath.Join(dir, "config"), Env: &Env{vals: map[string]string{"CELL_SCRATCH_SWEEP_INTERVAL": "1m"}}}
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
	env := fixtureProcessEnv("CELL_MAINT_FIXTURE=child", "ASSAY_SOURCE_REVISION=fixture-revision")
	c.runInteractiveHarness("worker-desk", []string{os.Args[0], "-test.run=^TestScratchMaintainFixture$"}, env, dir)
}

// regression: #5353
func TestScratchMaintainWiring(t *testing.T) {
	for _, mode := range []string{"interactive", "cadence"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			root := filepath.Join(dir, "run", "scratch")
			must(t, os.MkdirAll(root, 0700))
			must(t, os.MkdirAll(filepath.Join(dir, "config"), 0700))
			build, record, kept := plantAbandoned(t, root)
			cmd := exec.Command(os.Args[0], "-test.run=^TestScratchMaintainFixture$")
			cmd.Env = fixtureProcessEnv("CELL_MAINT_FIXTURE="+mode, "CELL_MAINT_DIR="+dir, "CELL_MAINT_TARGET="+build)
			cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
			cmd.WaitDelay = 200 * time.Millisecond
			must(t, cmd.Start())
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			t.Cleanup(func() {
				// Stop the supervisor the way an operator does, so it reaps its pass's
				// own process group and finishes its sweep before TempDir removal. A bare
				// kill leaves both writing into the tree being removed.
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
						t.Fatal("interactive child never saw maintenance:", err)
					}
				case <-time.After(10 * time.Second):
					t.Fatal("interactive supervisor hung")
				}
			} else if !scratchGone(build, 3*time.Second) {
				t.Fatal("cadenced supervisor never ran maintenance")
			}
			if scratchExists(t, build) || !scratchExists(t, record) || !scratchExists(t, kept) {
				t.Fatal("supervisor maintenance reclaimed the wrong scope")
			}
		})
	}
}
