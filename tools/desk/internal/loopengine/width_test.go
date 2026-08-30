package loopengine

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// widthCfg builds a Config whose claims dir is under a fresh temp HOME, with a Width
// resolver read from a pointer the test moves between passes — the whole point being that
// the cap is re-read per pass rather than snapshotted at construction.
func widthCfg(t *testing.T, deskDir string, floor int, width *int, werr *error) Config {
	t.Helper()
	return Config{
		PoolSize:  floor,
		ClaimsDir: filepath.Join(deskDir, "claims"),
		Progress:  &strings.Builder{},
		Width: func() (int, error) {
			if werr != nil && *werr != nil {
				return 0, *werr
			}
			return *width, nil
		},
	}
}

// pendingLoop is a fakeLoop whose dispatched agents NEVER complete. That is what makes the
// pool-occupancy assertions below deterministic: every dispatch stays in flight, so
// len(inflight) after a pass is exactly what that pass's cap admitted.
func pendingLoop(ids ...string) *fakeLoop {
	items := make([]Item, 0, len(ids))
	for _, id := range ids {
		items = append(items, Item{ID: id})
	}
	return &fakeLoop{
		name:      "width",
		remaining: items,
		dispatchFn: func(_ *fakeLoop, it Item, _ Tier) (Handle, error) {
			// An unfed channel: the handle is live forever, so the slot stays occupied.
			return &fakeHandle{item: it, done: make(chan Result)}, nil
		},
	}
}

func fill(t *testing.T, cfg Config, loop Loop, inflight map[string]Handle) {
	t.Helper()
	completions := make(chan completion, 64)
	if err := fillPool(cfg, loop, inflight, map[string]bool{}, newLivenessTracker(), completions); err != nil {
		t.Fatalf("fillPool: %v", err)
	}
}

// TestWidth_GrowFillsFreedSlotsUpToTheNewWidth is the GROW half of the contract: after the
// width rises, the very next pass fills up to the larger number. It fails on the
// pre-change engine, where cfg.PoolSize is a construction-time snapshot and the second
// pass would still stop at 2.
func TestWidth_GrowFillsFreedSlotsUpToTheNewWidth(t *testing.T) {
	deskDir := setupDeskHome(t, testLoopName)
	w := 2
	cfg := widthCfg(t, deskDir, 2, &w, nil)
	loop := pendingLoop("a", "b", "c", "d", "e")
	inflight := map[string]Handle{}

	fill(t, cfg, loop, inflight)
	if len(inflight) != 2 {
		t.Fatalf("first pass at width 2: occupancy %d, want 2", len(inflight))
	}

	w = 4 // the desk widens the pool between ticks
	fill(t, cfg, loop, inflight)
	if len(inflight) != 4 {
		t.Fatalf("after widening to 4: occupancy %d, want 4 — the cap was not re-read on this pass", len(inflight))
	}
}

// TestWidth_ShrinkStopsRefillingAndNeverKills is the SHRINK half, and it asserts BOTH
// halves of the safety property: the narrowed pass dispatches nothing new, and every agent
// already in flight still holds its slot afterwards. A shrink implemented as "terminate
// down to N" would pass the first assertion and fail the second.
func TestWidth_ShrinkStopsRefillingAndNeverKills(t *testing.T) {
	deskDir := setupDeskHome(t, testLoopName)
	w := 4
	cfg := widthCfg(t, deskDir, 4, &w, nil)
	loop := pendingLoop("a", "b", "c", "d", "e", "f")
	inflight := map[string]Handle{}

	fill(t, cfg, loop, inflight)
	if len(inflight) != 4 {
		t.Fatalf("first pass at width 4: occupancy %d, want 4", len(inflight))
	}
	before := loop.dispatchedIDs()

	w = 2 // narrowed below current occupancy
	fill(t, cfg, loop, inflight)

	if len(inflight) != 4 {
		t.Errorf("after narrowing to 2: occupancy %d, want 4 — a shrink must NOT stop work already "+
			"in flight; it converges by declining to refill", len(inflight))
	}
	if after := loop.dispatchedIDs(); len(after) != len(before) {
		t.Errorf("narrowed pass dispatched %d new item(s) (%v -> %v); a pass whose cap is below "+
			"occupancy must dispatch nothing", len(after)-len(before), before, after)
	}
}

// TestWidth_NilResolverIsExactlyPoolSize pins the additive-by-default property: a Config
// that never sets Width behaves precisely as it did before the field existed. Without this,
// "the feature is off by default" is a claim nothing checks.
func TestWidth_NilResolverIsExactlyPoolSize(t *testing.T) {
	deskDir := setupDeskHome(t, testLoopName)
	cfg := Config{PoolSize: 3, ClaimsDir: filepath.Join(deskDir, "claims"), Progress: &strings.Builder{}}
	if cfg.Width != nil {
		t.Fatal("fixture error: Width must be nil for this test")
	}
	loop := pendingLoop("a", "b", "c", "d", "e")
	inflight := map[string]Handle{}
	fill(t, cfg, loop, inflight)
	if len(inflight) != 3 {
		t.Fatalf("nil Width: occupancy %d, want PoolSize 3", len(inflight))
	}
}

// TestWidth_UnreadableWidthHoldsAtTheFloorAndNeverWidens is the could-not-check rule on the
// engine's own knob. A resolver that errors must leave the pool at its configured floor —
// not at "unlimited" (ignorance widening) and not at zero (a config error presenting as a
// drained queue).
func TestWidth_UnreadableWidthHoldsAtTheFloorAndNeverWidens(t *testing.T) {
	deskDir := setupDeskHome(t, testLoopName)
	w := 8
	boom := errors.New("roster unreadable")
	var werr error = boom
	var progress strings.Builder
	cfg := widthCfg(t, deskDir, 2, &w, &werr)
	cfg.Progress = &progress

	loop := pendingLoop("a", "b", "c", "d", "e", "f", "g", "h")
	inflight := map[string]Handle{}
	fill(t, cfg, loop, inflight)

	if len(inflight) != 2 {
		t.Fatalf("unreadable width: occupancy %d, want the configured floor 2 — a width that could "+
			"not be read must never widen the pool (it would have been 8)", len(inflight))
	}
	if !strings.Contains(progress.String(), "could-not-check") {
		t.Errorf("an unreadable width must SAY so on Progress, not degrade silently; got:\n%s", progress.String())
	}
}

// TestWidth_ZeroIsNotAWidth: a resolver answering 0 (or negative) is a broken reading, not
// an instruction to stop dispatching. Treating it as a cap would let a misconfigured knob
// halt a drain while every instrument still reported the loop healthy — the stop flag is
// the control for stopping, and it is one a human can see armed.
func TestWidth_ZeroIsNotAWidth(t *testing.T) {
	deskDir := setupDeskHome(t, testLoopName)
	w := 0
	var progress strings.Builder
	cfg := widthCfg(t, deskDir, 2, &w, nil)
	cfg.Progress = &progress

	loop := pendingLoop("a", "b", "c")
	inflight := map[string]Handle{}
	fill(t, cfg, loop, inflight)

	if len(inflight) != 2 {
		t.Fatalf("width 0: occupancy %d, want the configured floor 2 — 0 is a broken reading, "+
			"not a halt instruction", len(inflight))
	}
	if !strings.Contains(progress.String(), "not a pool width") {
		t.Errorf("a zero width must be NAMED on Progress; got:\n%s", progress.String())
	}
}
