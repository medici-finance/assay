package drainloop

import (
	"errors"
	"testing"
)

// A drain of the stand-in adapters lands every item exactly once, with a pass verdict.
func TestDrainLandsEveryItem(t *testing.T) {
	items := []Item{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	q := NewMemoryQueue("t", items)
	c := mustFileClaim(t, t.TempDir())
	if err := Run(Config{Loop: q, Claimer: c, PoolSize: 2, StopWhenIdle: true}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, it := range items {
		r, ok := q.Landed(it.ID)
		if !ok || r.Verdict != VerdictPass {
			t.Fatalf("item %s not landed pass: ok=%v verdict=%v", it.ID, ok, r.Verdict)
		}
		if r.Item.ID != it.ID {
			t.Fatalf("item %s: Result.Item not folded in, got %q", it.ID, r.Item.ID)
		}
	}
}

// A claim held by someone else is a SKIP, never a second dispatch. This is the dedupe
// guarantee: an ID in flight at most once.
func TestClaimCollisionSkipsItem(t *testing.T) {
	c := mustFileClaim(t, t.TempDir())
	got, err := c.Claim("x")
	if err != nil || !got {
		t.Fatalf("first claim should win: %v %v", got, err)
	}
	got, err = c.Claim("x")
	if err != nil {
		t.Fatalf("second claim should not error, got %v", err)
	}
	if got {
		t.Fatal("second claim won the same ID — dedupe broken")
	}
}

// Release is idempotent: releasing an unheld claim does not error.
func TestReleaseIsIdempotent(t *testing.T) {
	c := mustFileClaim(t, t.TempDir())
	if err := c.Release("never-held"); err != nil {
		t.Fatalf("releasing an unheld claim must not error: %v", err)
	}
}

// A dispatch that fails is LANDED as VerdictError (not dropped, not retried into a loop) and
// the drain continues to the remaining items.
func TestDispatchFailureLandsAndContinues(t *testing.T) {
	q := &failingDispatch{MemoryQueue: NewMemoryQueue("t", []Item{{ID: "boom"}, {ID: "ok"}})}
	c := mustFileClaim(t, t.TempDir())
	if err := Run(Config{Loop: q, Claimer: c, PoolSize: 1, StopWhenIdle: true}); err != nil {
		t.Fatalf("a failed dispatch must not abort the drain: %v", err)
	}
	if r, ok := q.Landed("boom"); !ok || r.Verdict != VerdictError {
		t.Fatalf("failed item should land VerdictError, got verdict=%v landed=%v", r.Verdict, ok)
	}
	if r, ok := q.Landed("ok"); !ok || r.Verdict != VerdictPass {
		t.Fatalf("the drain must continue past a failure; ok item landed=%v verdict=%v", ok, r.Verdict)
	}
}

// A human-tiered item (TierHuman) is landed as held without dispatch.
func TestHeldItemIsNotDispatched(t *testing.T) {
	q := &holdAll{MemoryQueue: NewMemoryQueue("t", []Item{{ID: "h"}})}
	c := mustFileClaim(t, t.TempDir())
	if err := Run(Config{Loop: q, Claimer: c, PoolSize: 1, StopWhenIdle: true}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if q.dispatched {
		t.Fatal("a held item must never reach Dispatch")
	}
	r, ok := q.Landed("h")
	if !ok || r.Verdict != VerdictHold {
		t.Fatalf("a held item must land VerdictHold, got ok=%v verdict=%v", ok, r.Verdict)
	}
}

// A could-not-check TierPolicy read SKIPS the item — it is neither dispatched nor landed.
func TestTierCouldNotCheckSkips(t *testing.T) {
	q := &tierErrs{MemoryQueue: NewMemoryQueue("t", []Item{{ID: "u"}})}
	c := mustFileClaim(t, t.TempDir())
	// StopWhenIdle so the drain terminates: a skipped item dispatches nothing → idle → stop.
	if err := Run(Config{Loop: q, Claimer: c, PoolSize: 1, StopWhenIdle: true}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if q.dispatched {
		t.Fatal("a could-not-check tier item must never be dispatched")
	}
	if _, ok := q.Landed("u"); ok {
		t.Fatal("a skipped item must not be landed (it is retried a later pass, not guessed)")
	}
}

// PoolSize < 1 and missing Loop/Claimer are rejected before anything runs.
func TestConfigValidation(t *testing.T) {
	c := mustFileClaim(t, t.TempDir())
	q := NewMemoryQueue("t", nil)
	if err := Run(Config{Loop: q, Claimer: c, PoolSize: 0}); err == nil {
		t.Fatal("PoolSize 0 must be rejected")
	}
	if err := Run(Config{Loop: nil, Claimer: c, PoolSize: 1}); err == nil {
		t.Fatal("nil Loop must be rejected")
	}
	if err := Run(Config{Loop: q, Claimer: nil, PoolSize: 1}); err == nil {
		t.Fatal("nil Claimer must be rejected")
	}
}

// NewFileClaim helper that fails the test on error, for terse test bodies.
func mustFileClaim(t *testing.T, dir string) *FileClaim {
	t.Helper()
	c, err := NewFileClaim(dir)
	if err != nil {
		t.Fatalf("NewFileClaim: %v", err)
	}
	return c
}

type failingDispatch struct{ *MemoryQueue }

func (f *failingDispatch) Dispatch(it Item, tier Tier) (Handle, error) {
	if it.ID == "boom" {
		return nil, errors.New("simulated dispatch failure")
	}
	return f.MemoryQueue.Dispatch(it, tier)
}

type holdAll struct {
	*MemoryQueue
	dispatched bool
}

func (h *holdAll) TierPolicy(Item) (Tier, error) { return TierHuman, nil }
func (h *holdAll) Dispatch(it Item, tier Tier) (Handle, error) {
	h.dispatched = true
	return h.MemoryQueue.Dispatch(it, tier)
}

type tierErrs struct {
	*MemoryQueue
	dispatched bool
}

func (te *tierErrs) TierPolicy(Item) (Tier, error) {
	return TierLocal, errors.New("board read failed")
}
func (te *tierErrs) Dispatch(it Item, tier Tier) (Handle, error) {
	te.dispatched = true
	return te.MemoryQueue.Dispatch(it, tier)
}
