//go:build darwin || linux

package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/cellcadence"
	"github.com/medici-finance/assay/tools/desk/internal/cellop"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestCellLifecyclePlanFlow drives the adapters that feed cellcadence's lifecycle model with real
// checkpoint and holder files and real processes: `up --plan` records the model's preconditions,
// `up --apply`'s measurement reproduces them from a fresh read, and `down`'s settle finishes a
// checkpoint only for the owner generation it signalled.
func TestCellLifecyclePlanFlow(t *testing.T) {
	const role = "worker-desk"
	verb := upVerb(nil, false)
	action := cellop.Action{ID: reconcilePrefix + role, Cell: "example"}

	// plan returns the one reconcile action `up --plan` would store for the role, or nil.
	plan := func(t *testing.T, c *Cell) (probe upProbe, planned []string) {
		t.Helper()
		probes, _ := c.probeUpRoles([]string{role}, nil)
		actions, _ := c.upActions(probes, nil)
		for _, a := range actions {
			for _, p := range a.Preconditions {
				planned = append(planned, p.Key+"="+p.Value)
			}
		}
		return probes[0], planned
	}
	measured := func(t *testing.T, c *Cell) []string {
		t.Helper()
		got, err := verb.Measure(c, action)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, p := range got {
			out = append(out, p.Key+"="+p.Value)
		}
		return out
	}

	t.Run("plan preconditions are re-measured identically", func(t *testing.T) {
		c := reconcileCell(t)
		writeStale(t, c, role, deadPID(t), "recorded-start", 0, "")
		p, planned := plan(t, c)
		if !p.planned() || p.class() != cellcadence.ClassDead {
			t.Fatalf("a record whose owner is gone must be plannable: %s %s", p.Kind, p.class())
		}
		want := []string{"role/" + role + "/state=" + cellcadence.KindStale, "role/" + role + "/class=" + cellcadence.ClassDead}
		if !reflect.DeepEqual(planned, want) {
			t.Fatalf("planned preconditions %v, want %v", planned, want)
		}
		if got := measured(t, c); !reflect.DeepEqual(got, planned) {
			t.Fatalf("apply-time measurement %v differs from the plan %v with nothing changed", got, planned)
		}
	})

	t.Run("a pid reused by another process reads dead and is never signalled", func(t *testing.T) {
		c := reconcileCell(t)
		live := liveChild(t)
		writeStale(t, c, role, live.Process.Pid, "start-of-the-previous-owner", 0, "")
		p, planned := plan(t, c)
		if p.class() != cellcadence.ClassDead || len(planned) == 0 {
			t.Fatalf("a reused pid is not the recorded owner: class %s planned %v", p.class(), planned)
		}
		if _, err := verb.Run(c, action); err != nil {
			t.Fatal(err)
		}
		if !alive(live.Process.Pid) {
			t.Fatal("applying the plan signalled a process that only reused the recorded pid")
		}
		if _, err := os.Stat(filepath.Join(c.cadenceDir(role), "checkpoint.json")); !os.IsNotExist(err) {
			t.Fatalf("the stale checkpoint was not cleared: %v", err)
		}
	})

	t.Run("a plan goes stale when the record's owner is running again", func(t *testing.T) {
		c := reconcileCell(t)
		writeStale(t, c, role, deadPID(t), "recorded-start", 0, "")
		_, planned := plan(t, c)
		live := liveChild(t)
		start, gone, err := deskkit.ProcessStart(live.Process.Pid)
		if err != nil || gone {
			t.Fatal(err)
		}
		writeStale(t, c, role, live.Process.Pid, start, 0, "")
		got := measured(t, c)
		if reflect.DeepEqual(got, planned) {
			t.Fatalf("a record whose owner now runs measured as the planned one: %v", got)
		}
		if want := "role/" + role + "/class=" + cellcadence.ClassAlive; got[1] != want {
			t.Fatalf("measured %v, want %s", got, want)
		}
	})

	t.Run("settle finishes only the owner generation down signalled", func(t *testing.T) {
		c := reconcileCell(t)
		dead := deadPID(t)
		writeStale(t, c, role, dead, "recorded-start", 0, "")
		dir := c.cadenceDir(role)
		before, err := cellcadence.Read(dir)
		if err != nil {
			t.Fatal(err)
		}
		owner := interactiveOwner{Role: role, PID: dead, Start: "recorded-start", Sequence: before.Sequence, LastStart: before.LastStart}
		unchanged := func(why string) {
			t.Helper()
			if s, err := cellcadence.Read(dir); err != nil || s != before {
				t.Fatalf("%s: the checkpoint changed: %+v %v", why, s, err)
			}
		}

		// The pid was reused: the owner record that survives names another start time.
		reused := owner
		reused.Start = "another-process"
		if _, err := c.settleOnce(reused, dir); err == nil {
			t.Fatal("settled a record whose owner record carries another start time")
		}
		unchanged("reused pid")

		// The #6251 shape: the owner record is gone with the lease, so the checkpoint read before
		// the signal is the only binding. A replacement owner's sequence is not the signalled one.
		if err := os.Remove(filepath.Join(dir, "owner.lock.holder")); err != nil {
			t.Fatal(err)
		}
		stale := owner
		stale.Sequence++
		if _, err := c.settleOnce(stale, dir); err == nil {
			t.Fatal("settled a checkpoint of another generation")
		}
		unchanged("stale generation")
		if _, err := c.settleOnce(interactiveOwner{Role: role, PID: dead}, dir); err == nil {
			t.Fatal("settled with no owner generation named")
		}
		unchanged("unnamed generation")

		if why, err := c.settleOnce(owner, dir); why != "" || err != nil {
			t.Fatalf("the signalled owner's own checkpoint was not settled: %q %v", why, err)
		}
		done, err := cellcadence.Read(dir)
		if err != nil || done.Running || done.Summary != downStoppedSummary {
			t.Fatalf("settled checkpoint: %+v %v", done, err)
		}

		// Repeated down: nothing is left to finish and nothing is invented.
		if why, err := c.settleOnce(owner, dir); why != "" || err != nil {
			t.Fatalf("a repeated settle: %q %v", why, err)
		}
		if again, err := cellcadence.Read(dir); err != nil || again != done {
			t.Fatalf("a repeated settle rewrote the checkpoint: %+v %v", again, err)
		}
	})
}
