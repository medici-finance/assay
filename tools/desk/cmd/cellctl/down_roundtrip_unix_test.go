//go:build darwin || linux

package main

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellcadence"
	"github.com/medici-finance/assay/tools/desk/internal/cellrunner"
)

// The round trip `up`, `down`, `up` (#6116), with real compiled owners: each owner is this test
// binary re-executed, admitted by the launch's own admitRole, holding the real role lease. "down"
// is the cell-wide downRoles with a cockpit close that delivers the hangup a closed tmux window
// does. Nothing reconciles between the down and the second up.

const roundTripRole = "worker-desk"

// TestDownRoundTripFixture is the owner and child fixture; it does nothing in an ordinary run.
func TestDownRoundTripFixture(t *testing.T) {
	mode := os.Getenv("CELL_ROUNDTRIP_FIXTURE")
	if mode == "" {
		return
	}
	dir, cfg := os.Getenv("CELL_ROUNDTRIP_DIR"), os.Getenv("CELL_ROUNDTRIP_CONFIG")
	if mode == "child" {
		_ = os.WriteFile(filepath.Join(dir, "child.pid"), []byte(strconv.Itoa(os.Getpid())), 0600)
		for {
			time.Sleep(time.Hour)
		}
	}
	c := &Cell{Name: "sample", Dir: dir, Config: cfg, Kind: "house"}
	if mode == "cadence" {
		c.Cadence = &cadenceOptions{Interval: time.Second, Budget: 61 * time.Second}
	}
	lease, err := c.admitRole(roundTripRole, false)
	if err != nil {
		_ = os.WriteFile(filepath.Join(dir, "admit.err"), []byte(err.Error()), 0600)
		os.Exit(92)
	}
	c.cadenceLease = lease
	child := []string{os.Args[0], "-test.run=^TestDownRoundTripFixture$"}
	childEnv := fixtureProcessEnv("CELL_ROUNDTRIP_FIXTURE=child", "CELL_ROUNDTRIP_DIR="+dir)
	switch mode {
	case "interactive":
		// The launch's own interactive owner.
		c.runInteractiveHarness(roundTripRole, child, childEnv, dir)
	case "uncertain":
		// An owner whose cleanup cannot prove itself finished after the hangup, as when the
		// terminal's foreground cannot be restored once tmux has gone: it reaps its child and
		// still reports Uncertain, so RunInteractive leaves the checkpoint Running.
		ctx, stop := cellrunner.NotifyContext(context.Background())
		defer stop()
		_ = lease.RunInteractive(ctx, c.Name, roundTripRole, func(pass context.Context) cellcadence.Result {
			cmd := exec.Command(child[0], child[1:]...)
			cmd.Env = childEnv
			if err := cmd.Start(); err != nil {
				return cellcadence.Result{ExitCode: -1, Err: err}
			}
			_ = lease.RecordChild(cmd.Process.Pid)
			<-pass.Done()
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			_ = lease.RecordChild(0)
			return cellcadence.Result{ExitCode: -1, Uncertain: true}
		})
		os.Exit(1)
	case "cadence":
		config := c.cadenceConfig(roundTripRole)
		config.Heartbeat = 50 * time.Millisecond
		config.BetweenPasses = nil
		_ = lease.Run(context.Background(), config, func(context.Context) cellcadence.Result {
			f, err := os.OpenFile(filepath.Join(dir, "passes"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
			if err == nil {
				_, _ = f.WriteString("pass\n")
				_ = f.Close()
			}
			return cellcadence.Result{Outcome: "noop", Summary: "fixture pass"}
		})
		_ = lease.Close()
		os.Exit(0)
	}
}

// roundTripOwner is one running fixture owner; it is reaped in the background, as the cockpit's
// shell reaps a window's process, so a down that waits for it sees it gone.
type roundTripOwner struct {
	cmd  *exec.Cmd
	done chan error
}

func startRoundTripOwner(t *testing.T, c *Cell, mode string) *roundTripOwner {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestDownRoundTripFixture$")
	cmd.Env = fixtureProcessEnv("CELL_ROUNDTRIP_FIXTURE="+mode, "CELL_ROUNDTRIP_DIR="+c.Dir, "CELL_ROUNDTRIP_CONFIG="+c.Config)
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	cmd.WaitDelay = 200 * time.Millisecond
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	o := &roundTripOwner{cmd: cmd, done: make(chan error, 1)}
	go func() { o.done <- cmd.Wait() }()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		<-o.done
		if b, err := os.ReadFile(filepath.Join(c.Dir, "child.pid")); err == nil {
			if pid, _ := strconv.Atoi(string(b)); pid > 0 {
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
		}
	})
	return o
}

// exited reports whether the owner has exited within d.
func (o *roundTripOwner) exited(d time.Duration) bool {
	select {
	case err := <-o.done:
		o.done <- err
		return true
	case <-time.After(d):
		return false
	}
}

// waitFor polls cond until it holds; an owner that exits first fails the test with what its
// admission said.
func waitFor(t *testing.T, c *Cell, o *roundTripOwner, what string, cond func() bool) {
	t.Helper()
	until := time.Now().Add(15 * time.Second)
	for !cond() {
		if o.exited(0) {
			b, _ := os.ReadFile(filepath.Join(c.Dir, "admit.err"))
			t.Fatalf("%s: the owner exited first; admission said %q", what, b)
		}
		if time.Now().After(until) {
			t.Fatalf("%s: timed out", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func childStarted(c *Cell) func() bool {
	return func() bool {
		_, err := os.Stat(filepath.Join(c.Dir, "child.pid"))
		return err == nil
	}
}

// hangup is a cockpit close that delivers what a closed tmux window delivers to its process.
func hangup(t *testing.T, o *roundTripOwner) func() bool {
	return func() bool {
		if err := o.cmd.Process.Signal(syscall.SIGHUP); err != nil {
			t.Fatal(err)
		}
		return true
	}
}

func roundTripCell(t *testing.T) *Cell {
	t.Helper()
	root := t.TempDir()
	return &Cell{Name: "sample", Dir: filepath.Join(root, "sample"), Config: t.TempDir(), Kind: "house"}
}

// assertNoReconcile is the second `up`'s preflight: nothing for an operator to clear.
func assertNoReconcile(t *testing.T, c *Cell) {
	t.Helper()
	probes, plan := c.probeUpRoles([]string{roundTripRole}, nil)
	for _, p := range probes {
		if p.Role == roundTripRole && p.Kind != probeAbsent {
			t.Fatalf("after down the role needs reconciling: probe %+v, plan entries %d", p, len(plan.Entries))
		}
	}
	if plan != nil && len(plan.Entries) > 0 {
		t.Fatalf("after down the preflight found stale state: %+v", plan.Entries[0])
	}
}

// regression: #6116
func TestDownRoundTripInteractiveUncertainCleanup(t *testing.T) {
	c := roundTripCell(t)
	first := startRoundTripOwner(t, c, "uncertain")
	waitFor(t, c, first, "first up", childStarted(c))
	unsettled, err := c.downRoles(hangup(t, first))
	if err != nil || len(unsettled) > 0 {
		t.Fatalf("down did not settle the interactive owner it stopped: %v %q", err, unsettled)
	}
	if !first.exited(5 * time.Second) {
		t.Fatal("the owner did not exit on the hangup")
	}
	s, err := cellcadence.Read(c.cadenceDir(roundTripRole))
	if err != nil || s.Running || s.Summary != downStoppedSummary || s.Outcome != "could-not-check" {
		t.Fatalf("down left the checkpoint unfinished or misrecorded: %+v %v", s, err)
	}
	assertNoReconcile(t, c)
	must(t, os.Remove(filepath.Join(c.Dir, "child.pid")))
	second := startRoundTripOwner(t, c, "uncertain")
	waitFor(t, c, second, "second up", childStarted(c))
}

// regression: #6116
func TestDownRoundTripInteractive(t *testing.T) {
	c := roundTripCell(t)
	first := startRoundTripOwner(t, c, "interactive")
	waitFor(t, c, first, "first up", childStarted(c))
	unsettled, err := c.downRoles(hangup(t, first))
	if err != nil || len(unsettled) > 0 {
		t.Fatalf("down did not settle the interactive owner it stopped: %v %q", err, unsettled)
	}
	// down returned only after the owner let go: the next launch does not race it.
	if busy, err := cellcadence.Probe(c.cadenceDir(roundTripRole)); err != nil || busy {
		t.Fatalf("down returned with the role lease still held: busy=%v %v", busy, err)
	}
	if s, err := cellcadence.Read(c.cadenceDir(roundTripRole)); err != nil || s.Running {
		t.Fatalf("down left the checkpoint unfinished: %+v %v", s, err)
	}
	assertNoReconcile(t, c)
	must(t, os.Remove(filepath.Join(c.Dir, "child.pid")))
	second := startRoundTripOwner(t, c, "interactive")
	waitFor(t, c, second, "second up", childStarted(c))
}

// regression: #6116
func TestDownRoundTripCadence(t *testing.T) {
	c := roundTripCell(t)
	passes := func() int {
		b, _ := os.ReadFile(filepath.Join(c.Dir, "passes"))
		return strings.Count(string(b), "pass\n")
	}
	first := startRoundTripOwner(t, c, "cadence")
	waitFor(t, c, first, "first up", func() bool { return passes() > 0 })
	unsettled, err := c.downRoles(func() bool { return true })
	if err != nil || len(unsettled) > 0 {
		t.Fatalf("down: %v %q", err, unsettled)
	}
	if !first.exited(5 * time.Second) {
		t.Fatal("the supervisor did not stop on down's STOP")
	}
	stop := filepath.Join(c.cadenceDir(roundTripRole), "STOP")
	if b, err := os.ReadFile(stop); err != nil || string(b) != downStopReason {
		t.Fatalf("down's STOP: %q %v", b, err)
	}
	assertNoReconcile(t, c)
	before := passes()
	second := startRoundTripOwner(t, c, "cadence")
	waitFor(t, c, second, "second up runs a pass", func() bool { return passes() > before })
	if _, err := os.Lstat(stop); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the second up kept the STOP down wrote: %v", err)
	}
}

// A crashed interactive owner with no down behind it keeps its unfinished record: down's settle
// refuses an owner it did not identify, or one whose start time is not the record's, and the
// next launch still refuses until reconcile or recover. It holds before and after #6116: the
// safety the fix keeps.
func TestDownSettleRefusesUnidentifiedOwner(t *testing.T) {
	c := roundTripCell(t)
	first := startRoundTripOwner(t, c, "uncertain")
	waitFor(t, c, first, "first up", childStarted(c))
	dir := c.cadenceDir(roundTripRole)
	o, ok, err := c.interactiveOwner(roundTripRole)
	if err != nil || !ok || o.PID != first.cmd.Process.Pid || o.Start == "" {
		t.Fatalf("the live owner was not identified: %+v %v %v", o, ok, err)
	}
	must(t, first.cmd.Process.Kill())
	if !first.exited(5 * time.Second) {
		t.Fatal("the owner did not exit")
	}
	b, _ := os.ReadFile(filepath.Join(c.Dir, "child.pid"))
	if pid, _ := strconv.Atoi(string(b)); pid > 0 {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
	soon := time.Now().Add(time.Second)
	for name, who := range map[string]interactiveOwner{
		"unidentified":   {Role: roundTripRole},
		"another start":  {Role: roundTripRole, PID: o.PID, Start: o.Start + "-other"},
		"another holder": {Role: roundTripRole, PID: o.PID + 1, Start: o.Start},
	} {
		if err := c.settleInteractiveOwner(who, soon); err == nil {
			t.Fatalf("%s: settle finished a record down cannot prove it stopped", name)
		}
		if s, err := cellcadence.Read(dir); err != nil || !s.Running {
			t.Fatalf("%s: the crashed owner's record was cleared: %+v %v", name, s, err)
		}
	}
	if _, err := c.admitRole(roundTripRole, false); err == nil || !strings.Contains(err.Error(), "unfinished") {
		t.Fatalf("a crashed owner's record admitted a launch: %v", err)
	}
}
