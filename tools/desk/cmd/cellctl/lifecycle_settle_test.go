package main

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// The per-role `down <cell> <role>` of an interactive role waits for the owner its close signals,
// named before the close, as the cell-wide down does (#6116).

// regression: #6116
func TestPerRoleDownSettlesInteractiveOwner(t *testing.T) {
	w := newLCWorld(t)
	w.interactiveRole("worker-desk", "tmux")
	var order []string
	w.lc.probes.interactiveOwner = func(role string) (interactiveOwner, bool, error) {
		order = append(order, "named "+role)
		return interactiveOwner{Role: role, PID: harnessPID, Start: "start-100"}, true, nil
	}
	var settled []interactiveOwner
	w.lc.probes.settle = func(o interactiveOwner, deadline time.Time) error {
		order = append(order, "settled after "+strings.Join(w.closes(), ","))
		settled = append(settled, o)
		if time.Until(deadline) <= 0 || time.Until(deadline) > downOwnerWait {
			t.Errorf("settle deadline %v is not bounded by %v", time.Until(deadline), downOwnerWait)
		}
		return nil
	}
	out, err := w.down("worker-desk")
	if err != nil {
		t.Fatal(err)
	}
	if len(settled) != 1 || settled[0].PID != harnessPID || settled[0].Start != "start-100" {
		t.Fatalf("the owner named before the close was not settled: %+v", settled)
	}
	if len(order) != 2 || order[0] != "named worker-desk" || !strings.Contains(order[1], "kill-window") {
		t.Fatalf("the owner must be named before the close and settled after it: %q", order)
	}
	if !strings.Contains(out, "closed tmux") {
		t.Fatalf("result: %s", out)
	}

	// An owner the wait cannot settle is named in the result; the close already happened.
	w2 := newLCWorld(t)
	w2.interactiveRole("worker-desk", "tmux")
	w2.lc.probes.interactiveOwner = func(role string) (interactiveOwner, bool, error) {
		return interactiveOwner{Role: role, PID: harnessPID, Start: "start-100"}, true, nil
	}
	w2.lc.probes.settle = func(o interactiveOwner, _ time.Time) error {
		return errors.New(o.Role + ": left unfinished (the interactive owner has not exited); cellctl up sample --plan shows the evidence")
	}
	if _, err := w2.down("worker-desk"); err == nil || !strings.Contains(err.Error(), "up sample --plan") {
		t.Fatalf("an unsettled owner was not reported: %v", err)
	}

	// A cadence role is not waited for here: its supervisor's stop already was.
	w3 := newLCWorld(t)
	w3.cadenceRole("worker-desk", "tmux")
	w3.lc.probes.interactiveOwner = func(string) (interactiveOwner, bool, error) {
		t.Fatal("a cadence role's owner was probed as interactive")
		return interactiveOwner{}, false, nil
	}
	w3.lc.probes.settle = func(interactiveOwner, time.Time) error { t.Fatal("a cadence role was settled"); return nil }
	if _, err := w3.down("worker-desk"); err != nil {
		t.Fatal(err)
	}
}
