package drainloop

import (
	"errors"
	"testing"
	"time"
)

// The author-not-runner guard trips only when a non-empty Implementer equals the runner, and
// is inert otherwise (empty runner, empty implementer, or a mismatch).
func TestCheckAuthorRunner(t *testing.T) {
	if err := CheckAuthorRunner(Item{ID: "a", Implementer: "alice"}, "alice"); err == nil {
		t.Fatal("author==runner must trip the guard")
	}
	if err := CheckAuthorRunner(Item{ID: "a", Implementer: "alice"}, "bob"); err != nil {
		t.Fatalf("author!=runner must not trip: %v", err)
	}
	if err := CheckAuthorRunner(Item{ID: "a", Implementer: "alice"}, ""); err != nil {
		t.Fatalf("empty runnerID disables the guard: %v", err)
	}
	if err := CheckAuthorRunner(Item{ID: "a"}, "alice"); err != nil {
		t.Fatalf("empty Implementer disables the guard: %v", err)
	}
}

// An item authored by the configured runner is routed to Land as held, never dispatched.
func TestEngineAuthorRunnerGuard(t *testing.T) {
	q := &recordDispatch{MemoryQueue: NewMemoryQueue("t", []Item{{ID: "self", Implementer: "runner-1"}})}
	c := mustFileClaim(t, t.TempDir())
	if err := Run(Config{Loop: q, Claimer: c, PoolSize: 1, StopWhenIdle: true, RunnerID: "runner-1"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if q.dispatched {
		t.Fatal("an item authored by the runner must not be dispatched")
	}
	if r, ok := q.Landed("self"); !ok || r.Verdict != VerdictHold {
		t.Fatalf("guarded item must land VerdictHold, got ok=%v verdict=%v", ok, r.Verdict)
	}
}

// A WorkEvidence probe that reports the work already done lands the item without dispatch.
func TestEngineWorkEvidenceSkipsDispatch(t *testing.T) {
	q := &recordDispatch{MemoryQueue: NewMemoryQueue("t", []Item{{ID: "done"}})}
	c := mustFileClaim(t, t.TempDir())
	probe := WorkEvidence(func(it Item) (bool, string, error) {
		return true, "merged in PR #123", nil
	})
	if err := Run(Config{Loop: q, Claimer: c, PoolSize: 1, StopWhenIdle: true, Evidence: probe}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if q.dispatched {
		t.Fatal("an item with prior work-evidence must not be dispatched")
	}
	r, ok := q.Landed("done")
	if !ok || r.Verdict != VerdictPass {
		t.Fatalf("evidenced item must land VerdictPass, got ok=%v verdict=%v", ok, r.Verdict)
	}
}

// A could-not-check WorkEvidence probe SKIPS the item rather than dispatch a possible
// duplicate.
func TestEngineWorkEvidenceCouldNotCheckSkips(t *testing.T) {
	q := &recordDispatch{MemoryQueue: NewMemoryQueue("t", []Item{{ID: "maybe"}})}
	c := mustFileClaim(t, t.TempDir())
	probe := WorkEvidence(func(it Item) (bool, string, error) {
		return false, "", errors.New("board unreachable")
	})
	if err := Run(Config{Loop: q, Claimer: c, PoolSize: 1, StopWhenIdle: true, Evidence: probe}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if q.dispatched {
		t.Fatal("a could-not-check evidence probe must not dispatch")
	}
	if _, ok := q.Landed("maybe"); ok {
		t.Fatal("a skipped item must not be landed")
	}
}

// The Timeouts taxonomy's Exceeded helper: a zero timer never breaches; a set timer breaches
// only past its bound.
func TestTimeoutsExceeded(t *testing.T) {
	if Exceeded(0, time.Hour) {
		t.Fatal("a zero timeout must never breach")
	}
	if Exceeded(time.Minute, 30*time.Second) {
		t.Fatal("30s must not breach a 1m timeout")
	}
	if !Exceeded(time.Minute, 90*time.Second) {
		t.Fatal("90s must breach a 1m timeout")
	}
}

type recordDispatch struct {
	*MemoryQueue
	dispatched bool
}

func (r *recordDispatch) Dispatch(it Item, tier Tier) (Handle, error) {
	r.dispatched = true
	return r.MemoryQueue.Dispatch(it, tier)
}
