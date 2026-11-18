package drainloop

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// fakeCapacityObserver is the fake shared-observation boundary: the readings a
// desk would take from claim summaries, heartbeats and supervisor process facts.
type fakeCapacityObserver struct {
	obs []CapacityObservation
	err error
}

func (f *fakeCapacityObserver) ObserveCapacity(context.Context) ([]CapacityObservation, error) {
	return f.obs, f.err
}

type capClock struct{ t time.Time }

func (c *capClock) now() time.Time { return c.t }

func capDesk(t *testing.T, clock *capClock, instance string, target int) *RoleCapacity {
	t.Helper()
	d, err := NewRoleCapacity(CapacityConfig{
		Cell: "cell-a", Role: "worker", Instance: instance, Target: target,
		MaxAge: time.Minute, Now: clock.now,
	})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func capObs(clock *capClock, source, instance string, active int) CapacityObservation {
	return CapacityObservation{
		Source: source, Cell: "cell-a", Role: "worker", Instance: instance,
		Generation: "g1", Active: active, ObservedAt: clock.t,
	}
}

// regression: F-ge26-role-capacity-gauge
func TestDeskRoleCapacity(t *testing.T) {
	t0 := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)

	t.Run("two desks fill the last gap with different items", func(t *testing.T) {
		clock := &capClock{t: t0}
		claims, err := NewFileClaim(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		a, b := capDesk(t, clock, "w1", 8), capDesk(t, clock, "w2", 8)
		// Both desks read the same lagging picture: seven active, target eight.
		for _, d := range []*RoleCapacity{a, b} {
			if err := d.Observe(capObs(clock, "claims", "w1", 4)); err != nil {
				t.Fatal(err)
			}
			if err := d.Observe(capObs(clock, "heartbeat-w2", "w2", 3)); err != nil {
				t.Fatal(err)
			}
		}
		if got := a.Estimate(); got.Active != 7 || got.Gap != 1 || !got.Known {
			t.Fatalf("desk a estimate = %+v, want active 7 gap 1 known", got)
		}
		na, nb := a.Refill(4, 0), b.Refill(3, 0)
		if na != 1 || nb != 1 {
			t.Fatalf("both desks must see the gap: refill a=%d b=%d, want 1 and 1", na, nb)
		}
		okA, errA := claims.Claim("item-a")
		okB, errB := claims.Claim("item-b")
		if errA != nil || errB != nil || !okA || !okB {
			t.Fatalf("distinct items must both claim: %v %v %v %v", okA, okB, errA, errB)
		}
		// Per-item exactness is independent of the gauge: a duplicate still refuses.
		if dup, err := claims.Claim("item-a"); err != nil || dup {
			t.Fatalf("duplicate item claim = %v, %v; want refused", dup, err)
		}
		// Actual work is now nine. Both desks observe it and stop refilling.
		for _, d := range []*RoleCapacity{a, b} {
			if err := d.Observe(capObs(clock, "claims", "w1", 5)); err != nil {
				t.Fatal(err)
			}
			if err := d.Observe(capObs(clock, "heartbeat-w2", "w2", 4)); err != nil {
				t.Fatal(err)
			}
			if got := d.Estimate(); got.Active != 9 || got.Gap != -1 {
				t.Fatalf("estimate above target = %+v, want active 9 gap -1", got)
			}
			if n := d.Refill(4, 0); n != 0 {
				t.Fatalf("refill above target = %d, want 0", n)
			}
		}
		// Completions bring it down; at target still nothing, one below resumes.
		clock.t = t0.Add(10 * time.Second)
		for _, d := range []*RoleCapacity{a, b} {
			_ = d.Observe(capObs(clock, "claims", "w1", 4))
			_ = d.Observe(capObs(clock, "heartbeat-w2", "w2", 4))
			if n := d.Refill(4, 0); n != 0 {
				t.Fatalf("refill at target = %d, want 0", n)
			}
		}
		clock.t = t0.Add(20 * time.Second)
		for _, d := range []*RoleCapacity{a, b} {
			_ = d.Observe(capObs(clock, "claims", "w1", 3))
			if n := d.Refill(3, 0); n != 1 {
				t.Fatalf("refill after completions = %d, want 1", n)
			}
		}
	})

	t.Run("counts every instance and draining work", func(t *testing.T) {
		clock := &capClock{t: t0}
		d := capDesk(t, clock, "w1", 8)
		old := capObs(clock, "supervisor-w0", "w0", 2)
		old.Draining = true
		for _, o := range []CapacityObservation{capObs(clock, "claims-w1", "w1", 3), capObs(clock, "claims-w2", "w2", 2), old} {
			if err := d.Observe(o); err != nil {
				t.Fatal(err)
			}
		}
		got := d.Estimate()
		if got.Active != 7 || got.Own != 3 || got.Draining != 2 || got.Fresh != 3 {
			t.Fatalf("estimate = %+v, want active 7 own 3 draining 2 fresh 3", got)
		}
		// The role target is shared, not multiplied: a third desk adds no slots.
		if got.Target != 8 {
			t.Fatalf("target = %d, want the one role target 8", got.Target)
		}
	})

	t.Run("an overestimate is corrected by refresh and by expiry", func(t *testing.T) {
		clock := &capClock{t: t0}
		d := capDesk(t, clock, "w1", 8)
		if err := d.Observe(capObs(clock, "claims", "w1", 10)); err != nil {
			t.Fatal(err)
		}
		if n := d.Refill(0, 0); n != 0 {
			t.Fatalf("refill at overestimate = %d, want 0", n)
		}
		// Fresher facts: only six remain. The newer, smaller reading replaces it.
		clock.t = t0.Add(5 * time.Second)
		if err := d.Observe(capObs(clock, "claims", "w1", 6)); err != nil {
			t.Fatal(err)
		}
		if got := d.Estimate(); got.Active != 6 {
			t.Fatalf("after refresh active = %d, want 6", got.Active)
		}
		if n := d.Refill(0, 0); n != 1 {
			t.Fatalf("refill after correction = %d, want 1", n)
		}
		// Expiry: no refresh at all. The old reading ages out instead of starving the role.
		e := capDesk(t, clock, "w1", 8)
		if err := e.Observe(capObs(clock, "claims", "w1", 10)); err != nil {
			t.Fatal(err)
		}
		clock.t = clock.t.Add(2 * time.Minute)
		got := e.Estimate()
		if got.Known || got.Stale != 1 || got.Fresh != 0 {
			t.Fatalf("expired estimate = %+v, want unknown with one stale source", got)
		}
		if n := e.Refill(0, 0); n != 1 {
			t.Fatalf("refill with an aged-out overestimate = %d, want one bounded burst", n)
		}
	})

	t.Run("missing data is unknown and bounded", func(t *testing.T) {
		clock := &capClock{t: t0}
		d, err := NewRoleCapacity(CapacityConfig{Cell: "cell-a", Role: "worker", Target: 8, MaxAge: time.Minute, Burst: 3, Now: clock.now})
		if err != nil {
			t.Fatal(err)
		}
		got := d.Estimate()
		if got.Known || got.Active != 0 || got.Fresh != 0 {
			t.Fatalf("empty estimate = %+v, want unknown, not a measured zero", got)
		}
		if n := d.Refill(0, 0); n != 3 {
			t.Fatalf("unknown refill = %d, want one burst (3), not the whole target", n)
		}
	})

	t.Run("refill is bounded by the burst and the gap", func(t *testing.T) {
		clock := &capClock{t: t0}
		d, err := NewRoleCapacity(CapacityConfig{Cell: "cell-a", Role: "worker", Target: 8, MaxAge: time.Minute, Burst: 3, Now: clock.now})
		if err != nil {
			t.Fatal(err)
		}
		_ = d.Observe(capObs(clock, "claims", "w1", 1))
		if n := d.Refill(0, 0); n != 3 {
			t.Fatalf("gap 7 burst 3 = %d, want 3", n)
		}
		_ = d.Observe(capObs(clock, "claims", "w1", 6))
		if n := d.Refill(0, 0); n != 2 {
			t.Fatalf("gap 2 burst 3 = %d, want 2", n)
		}
	})

	t.Run("an instance target caps this desk only", func(t *testing.T) {
		clock := &capClock{t: t0}
		d := capDesk(t, clock, "w1", 8)
		_ = d.Observe(capObs(clock, "claims", "w1", 1))
		if n := d.Refill(2, 2); n != 0 {
			t.Fatalf("desk at its own target = %d, want 0 though the role has a gap", n)
		}
		if n := d.Refill(1, 2); n != 1 {
			t.Fatalf("desk below its own target = %d, want 1", n)
		}
		if n := d.Refill(5, 0); n != 1 {
			t.Fatalf("no instance target must not cap = %d, want 1", n)
		}
	})

	t.Run("a failed refresh never blocks refill", func(t *testing.T) {
		clock := &capClock{t: t0}
		d := capDesk(t, clock, "w1", 8)
		if err := d.Observe(capObs(clock, "claims", "w1", 4)); err != nil {
			t.Fatal(err)
		}
		err := d.Refresh(context.Background(), &fakeCapacityObserver{err: errors.New("counter write failed")})
		if err == nil || !strings.Contains(err.Error(), "non-fatal") {
			t.Fatalf("refresh error = %v, want a reported non-fatal could-not-check", err)
		}
		got := d.Estimate()
		if got.RefreshErr == "" || got.Active != 4 {
			t.Fatalf("estimate = %+v, want the failure visible and retained reading kept", got)
		}
		if n := d.Refill(0, 0); n != 1 {
			t.Fatalf("refill after a refresh failure = %d, want 1 from the retained reading", n)
		}
		// With nothing retained at all it still does not hold the role.
		empty := capDesk(t, clock, "w1", 8)
		_ = empty.Refresh(context.Background(), &fakeCapacityObserver{err: errors.New("down")})
		if n := empty.Refill(0, 0); n != 1 {
			t.Fatalf("refill with no data after a failure = %d, want 1", n)
		}
		// Recovery clears the visible failure.
		ok := &fakeCapacityObserver{obs: []CapacityObservation{capObs(clock, "claims", "w1", 5)}}
		if err := d.Refresh(context.Background(), ok); err != nil {
			t.Fatal(err)
		}
		if got := d.Estimate(); got.RefreshErr != "" || got.Active != 5 {
			t.Fatalf("estimate after recovery = %+v", got)
		}
	})

	t.Run("foreign and malformed readings count for nothing", func(t *testing.T) {
		clock := &capClock{t: t0}
		d := capDesk(t, clock, "w1", 8)
		foreignCell := capObs(clock, "x1", "w9", 6)
		foreignCell.Cell = "cell-b"
		foreignRole := capObs(clock, "x2", "w9", 6)
		foreignRole.Role = "reviewer"
		negative := capObs(clock, "x3", "w9", -1)
		untimed := capObs(clock, "x4", "w9", 1)
		untimed.ObservedAt = time.Time{}
		anonymous := capObs(clock, "", "w9", 1)
		for name, o := range map[string]CapacityObservation{"cell": foreignCell, "role": foreignRole, "negative": negative, "untimed": untimed, "anonymous": anonymous} {
			if err := d.Observe(o); err == nil {
				t.Fatalf("%s reading was accepted", name)
			}
		}
		if got := d.Estimate(); got.Fresh != 0 || got.Active != 0 {
			t.Fatalf("estimate = %+v, want nothing counted", got)
		}
		if _, err := NewRoleCapacity(CapacityConfig{Cell: "c", Role: "r", Target: 0, MaxAge: time.Minute}); err == nil {
			t.Fatal("target 0 was accepted")
		}
	})

	t.Run("an out-of-order older reading is dropped", func(t *testing.T) {
		clock := &capClock{t: t0}
		d := capDesk(t, clock, "w1", 8)
		newer := capObs(clock, "claims", "w1", 3)
		older := capObs(clock, "claims", "w1", 9)
		older.ObservedAt = t0.Add(-time.Second)
		_ = d.Observe(newer)
		if err := d.Observe(older); err != nil {
			t.Fatal(err)
		}
		if got := d.Estimate(); got.Active != 3 {
			t.Fatalf("active = %d, want the newer reading 3", got.Active)
		}
	})
}
