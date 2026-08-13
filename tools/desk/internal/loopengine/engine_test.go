package loopengine

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// runUntil runs the engine in a goroutine and plants a STOP flag once stopWhen() holds,
// so the (never-self-exiting) engine terminates deterministically. It returns the Run
// error. deskDir is where the STOP flag is written (from setupDeskHome).
func runUntil(t *testing.T, cfg Config, loop Loop, deskDir string, stopWhen func() bool) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- Run(cfg, loop) }()

	deadline := time.After(5 * time.Second)
	planted := false
	tick := time.NewTicker(2 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case err := <-done:
			return err
		case <-tick.C:
			if !planted && stopWhen() {
				planted = true
				if err := os.MkdirAll(deskDir, 0o700); err != nil {
					t.Fatalf("mkdir deskdir: %v", err)
				}
				if err := os.WriteFile(filepath.Join(deskDir, "STOP"), []byte("test stop\n"), 0o600); err != nil {
					t.Fatalf("write STOP: %v", err)
				}
			}
		case <-deadline:
			t.Fatal("engine did not stop within deadline (possible wedge / no-exit bug)")
		}
	}
}

func TestRun_PoolRefillAfterCompletion(t *testing.T) {
	deskDir := setupDeskHome(t, "refill")
	loop := &fakeLoop{name: "refill"}
	for i := 0; i < 5; i++ {
		loop.remaining = append(loop.remaining, Item{ID: fmt.Sprintf("brief-%d", i)})
	}
	cfg := Config{PoolSize: 2, IdlePoll: 5 * time.Millisecond, ClaimsDir: t.TempDir(), StaleClaim: time.Hour}

	err := runUntil(t, cfg, loop, deskDir, func() bool { return len(loop.landedIDs()) == 5 })
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := len(loop.landedIDs()); got != 5 {
		t.Fatalf("landed %d; want 5 (pool of 2 must refill 5 items through)", got)
	}
	if got := len(loop.dispatchedIDs()); got != 5 {
		t.Fatalf("dispatched %d; want 5", got)
	}
}

func TestRun_IsDoneCallsOnIdleNeverExits(t *testing.T) {
	deskDir := setupDeskHome(t, "idle")
	loop := &fakeLoop{name: "idle"} // empty queue from the start
	cfg := Config{PoolSize: 3, IdlePoll: 3 * time.Millisecond, ClaimsDir: t.TempDir(), StaleClaim: time.Hour}

	// Stop only after OnIdle has fired several times — proving the engine idle-polled on an
	// empty queue rather than exiting on is_done().
	err := runUntil(t, cfg, loop, deskDir, func() bool {
		loop.mu.Lock()
		defer loop.mu.Unlock()
		return loop.onIdleN >= 3
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if loop.onIdleN < 3 {
		t.Fatalf("OnIdle fired %d times; want >=3 (engine must NOT exit on empty queue)", loop.onIdleN)
	}
}

func TestRun_ClaimCollisionSkipsItem(t *testing.T) {
	deskDir := setupDeskHome(t, "collide")
	claims := t.TempDir()
	// Pre-plant a LIVE claim for "b" as if another loop owns it.
	if err := os.MkdirAll(claims, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := Config{PoolSize: 3, IdlePoll: 3 * time.Millisecond, ClaimsDir: claims, StaleClaim: time.Hour}
	preclaimed := Item{ID: "b"}
	if err := os.WriteFile(claimPath(cfg, preclaimed), []byte(`{"itemID":"b","runner":"other","claimed":"2999-01-01T00:00:00Z"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	loop := &fakeLoop{name: "collide", remaining: []Item{{ID: "a"}, {ID: "b"}, {ID: "c"}}}
	// "b" can never land (claim held elsewhere), so stop once a and c have landed.
	err := runUntil(t, cfg, loop, deskDir, func() bool {
		ids := loop.landedIDs()
		return contains(ids, "a") && contains(ids, "c")
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if contains(loop.dispatchedIDs(), "b") {
		t.Fatal("item b was dispatched despite a live claim held elsewhere (double-dispatch)")
	}
}

func TestRun_AuthorEqualsRunnerRefused(t *testing.T) {
	deskDir := setupDeskHome(t, "author")
	loop := &fakeLoop{name: "author", remaining: []Item{
		{ID: "own", Implementer: "session-me", BriefPath: "docs/streams/x/brief-own.md"},
		{ID: "other", Implementer: "someone-else"},
	}}
	cfg := Config{PoolSize: 2, IdlePoll: 3 * time.Millisecond, ClaimsDir: t.TempDir(), StaleClaim: time.Hour, RunnerID: "session-me"}

	err := runUntil(t, cfg, loop, deskDir, func() bool { return contains(loop.landedIDs(), "other") })
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if contains(loop.dispatchedIDs(), "own") {
		t.Fatal("engine dispatched an item to its own implementer (author==runner not refused)")
	}
	if !contains(loop.dispatchedIDs(), "other") {
		t.Fatal("engine failed to dispatch the other-authored item")
	}
}

func TestRun_LandFailureFilesAndContinues(t *testing.T) {
	deskDir := setupDeskHome(t, "landfail")
	var mu sync.Mutex
	landAttempts := map[string]int{}
	loop := &fakeLoop{
		name:      "landfail",
		remaining: []Item{{ID: "ok-1"}, {ID: "bad"}, {ID: "ok-2"}},
		landFn: func(l *fakeLoop, r Result) error {
			mu.Lock()
			landAttempts[r.Item.ID]++
			mu.Unlock()
			if r.Item.ID == "bad" {
				// Unlandable — Land exhausted its retry and could not succeed.
				return fmt.Errorf("push race unresolved")
			}
			l.recordLandAndDrain(r)
			return nil
		},
	}
	cfg := Config{PoolSize: 3, IdlePoll: 3 * time.Millisecond, ClaimsDir: t.TempDir(), StaleClaim: time.Hour}

	err := runUntil(t, cfg, loop, deskDir, func() bool {
		ids := loop.landedIDs()
		return contains(ids, "ok-1") && contains(ids, "ok-2")
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// The good items drained; the bad one did NOT wedge the loop.
	if !contains(loop.landedIDs(), "ok-1") || !contains(loop.landedIDs(), "ok-2") {
		t.Fatal("good items did not drain past the unlandable one")
	}
	// The unlandable item is filed-and-continued (parked), not retried forever.
	mu.Lock()
	defer mu.Unlock()
	if landAttempts["bad"] == 0 {
		t.Fatal("unlandable item never attempted")
	}
	if landAttempts["bad"] > 3 {
		t.Fatalf("unlandable item retried %d times — must file-and-continue (park), not respin", landAttempts["bad"])
	}
}

func TestRun_TierHumanRoutedNotDispatched(t *testing.T) {
	deskDir := setupDeskHome(t, "human")
	var routed []Result
	var rmu sync.Mutex
	loop := &fakeLoop{
		name:      "human",
		remaining: []Item{{ID: "risky", Gate: "human", Risk: RiskFlags{Regulatory: true}}, {ID: "normal"}},
		tierFn: func(it Item) (Tier, error) {
			if it.Risk.Flagged(it.Gate) {
				return TierHuman, nil
			}
			return TierLocal, nil
		},
		landFn: func(l *fakeLoop, r Result) error {
			if r.Verdict == VerdictRouteHuman {
				rmu.Lock()
				routed = append(routed, r)
				rmu.Unlock()
				return nil
			}
			l.recordLandAndDrain(r)
			return nil
		},
	}
	cfg := Config{PoolSize: 2, IdlePoll: 3 * time.Millisecond, ClaimsDir: t.TempDir(), StaleClaim: time.Hour}

	err := runUntil(t, cfg, loop, deskDir, func() bool {
		rmu.Lock()
		defer rmu.Unlock()
		return contains(loop.landedIDs(), "normal") && len(routed) >= 1
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if contains(loop.dispatchedIDs(), "risky") {
		t.Fatal("TierHuman item was dispatched to a model — must route to human, not dispatch")
	}
	rmu.Lock()
	defer rmu.Unlock()
	if len(routed) == 0 || routed[0].Item.ID != "risky" {
		t.Fatal("risky item was not routed to the human path via Land")
	}
}

func TestRun_RejectsBadPoolSize(t *testing.T) {
	setupDeskHome(t, "bad")
	if err := Run(Config{PoolSize: 0}, &fakeLoop{name: "bad"}); err == nil {
		t.Fatal("PoolSize 0 should be rejected")
	}
}
