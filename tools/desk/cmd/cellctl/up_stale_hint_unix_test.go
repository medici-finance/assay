//go:build darwin || linux

package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellcadence"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The recovery verbs a stale hint may name, and what each one accepts.
const (
	verbPlan    = "plan"    // `cellctl up <cell> --plan`: plans a proven-dead record only
	verbRecover = "recover" // `cellctl cadence <cell> recover <role> --confirm-stopped`: any record whose lease is free
	verbNone    = "none"    // a lease that cannot be probed: both of the above take the lease and refuse
)

// writeCheckpoint writes a role's checkpoint.json as given, with an empty lease file beside it.
func writeCheckpoint(t *testing.T, dir string, body []byte) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "owner.lock"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "checkpoint.json"), body, 0o600); err != nil {
		t.Fatal(err)
	}
}

// listedRunning is the pid list a hint names in its "still running: pid ..." phrase, or nil.
func listedRunning(text string) []string {
	const phrase = "still running: pid "
	i := strings.Index(text, phrase)
	if i < 0 {
		return nil
	}
	rest := text[i+len(phrase):]
	if j := strings.Index(rest, ")"); j >= 0 {
		rest = rest[:j]
	}
	return strings.Split(rest, ", ")
}

// assertListsRunning fails unless text's "still running" phrase names every pid in pids.
func assertListsRunning(t *testing.T, text string, pids []int) {
	t.Helper()
	listed := listedRunning(text)
	for _, pid := range pids {
		found := false
		for _, l := range listed {
			found = found || l == strconv.Itoa(pid)
		}
		if !found {
			t.Fatalf("hint does not list running pid %d in its still-running phrase:\n%s", pid, text)
		}
	}
}

// stopOrphans kills the fixture's re-parented orphans and waits until each is gone: the
// operator's stop that the hint asks for before recover.
func stopOrphans(t *testing.T, pids []int) {
	t.Helper()
	for _, pid := range pids {
		syscall.Kill(pid, syscall.SIGKILL)
		for deadline := time.Now().Add(5 * time.Second); alive(pid); time.Sleep(20 * time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatalf("orphan pid %d did not stop", pid)
			}
		}
	}
}

func runningState(cell, role string, pid int) []byte {
	now := time.Now().UTC()
	b, _ := json.Marshal(cellcadence.State{Schema: cellcadence.Schema, Mode: "interactive", Cell: cell, Role: role, PID: pid, Running: true, Sequence: 1, NextDue: now, Heartbeat: now, LastStart: now, Outcome: "running"})
	return b
}

// Every stale classification `up` prints a hint for must name a verb that actually accepts it: the
// hint (the `up` row) and the --plan refusal both. Each case builds the state on disk, classifies
// it as `up` does, then RUNS the verb the hint names (and the one it must not name) against it.
//
// regression: #6192
func TestStaleHintNamesAcceptingVerb(t *testing.T) {
	const role = "worker-desk"
	planCmd := "cellctl up example --plan"
	recoverCmd := "cellctl cadence example recover " + role + " --confirm-stopped"
	cases := []struct {
		name     string
		setup    func(t *testing.T, c *Cell) (pids []int)
		class    string
		verb     string
		evidence string
		orphans  bool // the pids are re-parented orphans the test stops before recover
		// unproven: the pids run with no recorded start time, so they may be reused pids. The
		// evidence names them with that caveat; the still-running list (what to stop) does not.
		unproven bool
	}{
		{
			name:  "dead/owner-record",
			setup: func(t *testing.T, c *Cell) []int { writeStale(t, c, role, deadPID(t), "1.000000", 0, ""); return nil },
			class: reconcileDead, verb: verbPlan, evidence: "has exited",
		},
		{
			name: "dead/no-owner-record",
			setup: func(t *testing.T, c *Cell) []int {
				writeStale(t, c, role, deadPID(t), "1.000000", 0, "")
				if err := os.Remove(filepath.Join(c.cadenceDir(role), "owner.lock.holder")); err != nil {
					t.Fatal(err)
				}
				return nil
			},
			class: reconcileDead, verb: verbPlan, evidence: "no owner record survived",
		},
		{
			// The issue's state: no owner record, so nothing tells the running pid from a reused one.
			name: "could-not-check/no-owner-record",
			setup: func(t *testing.T, c *Cell) []int {
				live := liveChild(t).Process.Pid
				writeCheckpoint(t, c.cadenceDir(role), runningState(c.Name, role, live))
				return []int{live}
			},
			class: reconcileCNC, verb: verbRecover, evidence: "no owner record survived", unproven: true,
		},
		{
			name: "alive/role-child-running",
			setup: func(t *testing.T, c *Cell) []int {
				child := liveChild(t).Process.Pid
				start, gone, err := deskkit.ProcessStart(child)
				if err != nil || gone {
					t.Fatalf("child start: %v %v", gone, err)
				}
				writeStale(t, c, role, deadPID(t), "1.000000", child, start)
				return []int{child}
			},
			class: reconcileAlive, verb: verbRecover, evidence: "is still running",
		},
		{
			name: "could-not-check/unreadable-checkpoint",
			setup: func(t *testing.T, c *Cell) []int {
				writeCheckpoint(t, c.cadenceDir(role), []byte("{"))
				return nil
			},
			class: reconcileCNC, verb: verbRecover, evidence: "the checkpoint is unreadable",
		},
		{
			name: "could-not-check/foreign-checkpoint",
			setup: func(t *testing.T, c *Cell) []int {
				writeCheckpoint(t, c.cadenceDir(role), runningState("another-cell", role, deadPID(t)))
				return nil
			},
			class: reconcileCNC, verb: verbRecover, evidence: "belongs to another cell or role",
		},
		{
			// A marked orphan beside a record --plan refuses: no evidence line names its pid, so
			// only the running list does.
			name: "could-not-check/marked-orphan",
			setup: func(t *testing.T, c *Cell) []int {
				writeCheckpoint(t, c.cadenceDir(role), runningState("another-cell", role, deadPID(t)))
				return []int{startOrphan(t, c, role)}
			},
			class: reconcileCNC, verb: verbRecover, evidence: "belongs to another cell or role", orphans: true,
		},
		{
			name: "could-not-check/lease-unprobed",
			setup: func(t *testing.T, c *Cell) []int {
				dir := c.cadenceDir(role)
				if err := os.MkdirAll(filepath.Join(dir, "owner.lock"), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "checkpoint.json"), runningState(c.Name, role, deadPID(t)), 0o600); err != nil {
					t.Fatal(err)
				}
				return nil
			},
			class: reconcileCNC, verb: verbNone, evidence: "the lease could not be probed",
		},
	}
	covered := map[string]bool{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := reconcileCell(t)
			pids := tc.setup(t, c)
			probes, _ := c.probeUpRoles([]string{role}, nil)
			p := probes[0]
			if p.Kind != probeStale || p.class() != tc.class {
				t.Fatalf("classified %s/%s, want stale/%s: %+v", p.Kind, p.class(), tc.class, p.Entry)
			}
			covered[tc.class] = true
			hint := c.staleHint(p)
			actions, notPlanned := c.upActions(probes, nil)
			t.Logf("up row: %s", hint)
			if !strings.Contains(hint, tc.evidence) {
				t.Fatalf("hint does not carry the evidence %q:\n%s", tc.evidence, hint)
			}
			running := pids
			if tc.unproven {
				for _, pid := range pids {
					if !strings.Contains(hint, "pid "+strconv.Itoa(pid)+" is running and no start time was recorded") {
						t.Fatalf("hint does not name unproven pid %d with its caveat:\n%s", pid, hint)
					}
				}
				running = nil
			}
			assertListsRunning(t, hint, running)
			switch tc.verb {
			case verbPlan:
				if !strings.Contains(hint, planCmd) || strings.Contains(hint, recoverCmd) {
					t.Fatalf("hint must name %q only:\n%s", planCmd, hint)
				}
				if len(actions) != 1 || actions[0].ID != reconcilePrefix+role || len(notPlanned) != 0 {
					t.Fatalf("--plan did not accept a hint that names it: actions=%+v notPlanned=%q", actions, notPlanned)
				}
			case verbRecover, verbNone:
				// The --plan refusal carries the same per-role pointer as the up row.
				if len(actions) != 0 || len(notPlanned) != 1 {
					t.Fatalf("--plan planned a record it must refuse: actions=%+v notPlanned=%q", actions, notPlanned)
				}
				for _, text := range []string{hint, notPlanned[0]} {
					if strings.Contains(text, planCmd) {
						t.Fatalf("names %q, which refuses this class:\n%s", planCmd, text)
					}
					names := strings.Contains(text, recoverCmd)
					if names != (tc.verb == verbRecover) {
						t.Fatalf("names %q = %v, want %v:\n%s", recoverCmd, names, tc.verb == verbRecover, text)
					}
					if tc.verb == verbRecover && !strings.Contains(text, "cellctl up example --role "+role) {
						t.Fatalf("does not name the relaunch after recover:\n%s", text)
					}
					if tc.verb == verbNone && !strings.Contains(text, "cellctl cadence example status "+role) {
						t.Fatalf("does not name the re-check:\n%s", text)
					}
					if !strings.Contains(text, "once the prior harness and all its children have stopped") {
						t.Fatalf("does not make recovery wait for the prior harness to stop:\n%s", text)
					}
					assertListsRunning(t, text, running)
				}
				if tc.orphans {
					stopOrphans(t, pids)
				}
				// Run the recover verb's one code path. The operator's --confirm-stopped is the test's
				// word here: the fixture's processes are not the prior harness.
				err := c.clearCadence(role, true)
				if tc.verb == verbNone {
					if err == nil {
						t.Fatal("cadence recover accepted a lease that could not be probed; the hint must name it")
					}
					return
				}
				if err != nil {
					t.Fatalf("cadence recover refused the class its hint names it for: %v", err)
				}
				if again, _ := c.probeUpRoles([]string{role}, nil); again[0].Kind != probeAbsent {
					t.Fatalf("after cadence recover the role is %s, not absent: up --role cannot start it", again[0].Kind)
				}
			}
		})
	}
	// A marked orphan alone (no record) is planned: --plan stops it.
	c := reconcileCell(t)
	p := upProbe{Role: role, Kind: probeStale, Orphan: true}
	if hint := c.staleHint(p); !strings.Contains(hint, planCmd) {
		t.Fatalf("orphan-only hint does not name %q: %s", planCmd, hint)
	}
	if actions, _ := c.upActions([]upProbe{p}, nil); len(actions) != 1 {
		t.Fatalf("--plan did not accept an orphan-only role: %+v", actions)
	}
	for _, class := range []string{reconcileDead, reconcileAlive, reconcileCNC} {
		if !covered[class] {
			t.Fatalf("class sweep: no case classifies a stale role %s", class)
		}
	}
}

// The comms service's stale record: --plan's refusal and the comms preflight's pointer name
// `comms recover --confirm-stopped`, never --plan, for a record --plan refuses.
//
// regression: #6192
func TestCommsStaleHintAcceptingVerb(t *testing.T) {
	planCmd := "cellctl up example --plan"
	recoverCmd := "cellctl comms example recover --confirm-stopped"
	cases := []struct {
		name   string
		body   func(c *Cell) []byte
		lock   bool // owner.lock is a directory: the lease cannot be probed
		orphan bool // a comms-marked orphan runs beside the record
		class  string
		verb   string
	}{
		{name: "dead", body: func(c *Cell) []byte { return runningState(c.Name, "comms", deadPID(t)) }, class: reconcileDead, verb: verbPlan},
		{name: "could-not-check/foreign", body: func(c *Cell) []byte { return runningState("another-cell", "comms", deadPID(t)) }, class: reconcileCNC, verb: verbRecover},
		{name: "could-not-check/marked-orphan", body: func(c *Cell) []byte { return runningState("another-cell", "comms", deadPID(t)) }, orphan: true, class: reconcileCNC, verb: verbRecover},
		{name: "could-not-check/lease-unprobed", body: func(c *Cell) []byte { return runningState(c.Name, "comms", deadPID(t)) }, lock: true, class: reconcileCNC, verb: verbNone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := reconcileCell(t)
			cfg := &deskCommsConfig{Socket: filepath.Join(t.TempDir(), "gw.sock")}
			dir := c.commsDir()
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			if tc.lock {
				if err := os.MkdirAll(filepath.Join(dir, "owner.lock"), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(dir, "checkpoint.json"), tc.body(c), 0o600); err != nil {
				t.Fatal(err)
			}
			var pids []int
			if tc.orphan {
				pids = []int{startOrphan(t, c, "comms")}
			}
			e := commsStale(c.reconcileInventory(nil, cfg))
			if e == nil || e.Class != tc.class {
				t.Fatalf("comms classified %+v, want %s", e, tc.class)
			}
			actions, notPlanned := c.upActions(nil, e)
			pointer := c.commsStalePointer(e)
			t.Logf("preflight pointer: %s", pointer)
			if tc.verb == verbPlan {
				if len(actions) != 1 || actions[0].ID != reconcilePrefix+"comms" || !strings.Contains(pointer, planCmd) {
					t.Fatalf("dead comms: actions=%+v pointer=%q", actions, pointer)
				}
				return
			}
			if len(actions) != 0 || len(notPlanned) != 1 {
				t.Fatalf("--plan planned a comms record it must refuse: actions=%+v notPlanned=%q", actions, notPlanned)
			}
			for _, text := range []string{notPlanned[0], pointer} {
				if strings.Contains(text, planCmd) || strings.Contains(text, recoverCmd) != (tc.verb == verbRecover) {
					t.Fatalf("pointer names the wrong verb for %s:\n%s", tc.name, text)
				}
				assertListsRunning(t, text, pids)
			}
			stopOrphans(t, pids)
			err := c.recoverComms(cfg)
			if tc.verb == verbNone {
				if err == nil {
					t.Fatal("comms recover accepted a lease that could not be probed")
				}
				return
			}
			if err != nil {
				t.Fatalf("comms recover refused the class its pointer names it for: %v", err)
			}
			if _, err := cellcadence.Read(dir); !errors.Is(err, cellcadence.ErrNoState) {
				t.Fatalf("comms recover left the checkpoint: %v", err)
			}
		})
	}
}
