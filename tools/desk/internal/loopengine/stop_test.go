package loopengine

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestStopFlagMidDrain proves the mid-drain stop contract (Verify item 4): when a stop flag
// arms while an item is in flight, the STARTED land completes and NO new dispatch happens,
// then the engine exits cleanly. It uses the per-loop STOP.<DESK_LOOP> flag (scoped to
// testLoopName) to also exercise the DISABLED > STOP > STOP.<loop> precedence path.
func TestStopFlagMidDrain(t *testing.T) {
	deskDir := setupDeskHome(t, testLoopName)

	dispatched := make(chan string, 8)
	handles := map[string]*fakeHandle{}

	loop := &fakeLoop{
		name:      "drilltest",
		remaining: []Item{{ID: "in-flight-1"}, {ID: "queued-2"}, {ID: "queued-3"}},
	}
	loop.dispatchFn = func(l *fakeLoop, it Item, tier Tier) (Handle, error) {
		h := &fakeHandle{item: it, done: make(chan Result, 1)}
		l.mu.Lock()
		handles[it.ID] = h
		l.mu.Unlock()
		dispatched <- it.ID
		return h, nil // completion is fed by the test, not automatically
	}

	// PoolSize 1 so exactly ONE item is in flight when we arm the stop.
	cfg := Config{PoolSize: 1, IdlePoll: 3 * time.Millisecond, ClaimsDir: t.TempDir(), StaleClaim: time.Hour}

	done := make(chan error, 1)
	go func() { done <- Run(cfg, loop) }()

	// Wait for the first (only) dispatch.
	var first string
	select {
	case first = <-dispatched:
	case <-time.After(3 * time.Second):
		t.Fatal("no dispatch within deadline")
	}
	if first != "in-flight-1" {
		t.Fatalf("first dispatch = %q; want in-flight-1", first)
	}

	// Arm the per-loop stop flag WHILE the item is in flight.
	if err := os.MkdirAll(deskDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(deskDir, "STOP."+testLoopName), []byte("mid-drain stop\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Give the engine a moment to observe the stop at its next boundary, then let the
	// in-flight land complete — a STARTED land must finish.
	time.Sleep(20 * time.Millisecond)
	loop.mu.Lock()
	h := handles["in-flight-1"]
	loop.mu.Unlock()
	h.done <- Result{Item: Item{ID: "in-flight-1"}, Verdict: VerdictPass, RunnerID: "local:test"}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error on clean stop: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("engine did not exit after stop flag + in-flight land completed")
	}

	// The started land completed.
	if !contains(loop.landedIDs(), "in-flight-1") {
		t.Fatal("the in-flight item's started land did not complete before exit")
	}
	// No NEW dispatch after the stop armed.
	if contains(loop.dispatchedIDs(), "queued-2") || contains(loop.dispatchedIDs(), "queued-3") {
		t.Fatalf("engine dispatched new work after stop armed: %v", loop.dispatchedIDs())
	}
}
