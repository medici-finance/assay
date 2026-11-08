package store

import (
	"context"
	"testing"
	"time"

	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/refresh"
)

type refreshDataClock struct{ now time.Time }

func (c *refreshDataClock) Now() time.Time { return c.now }

type refreshClockAuthority struct {
	clock  *refreshDataClock
	calls  int
	zeroAt int
}

func (a *refreshClockAuthority) Authorize(ctx context.Context, c refresh.Caller, s mirror.Scope) error {
	a.calls++
	if a.calls == a.zeroAt {
		a.clock.now = time.Time{}
	}
	return (refreshTestAuthority{}).Authorize(ctx, c, s)
}

// regression: F-refresh-trusted-clock-required
func TestPGRefreshCompletionClockRefusal(t *testing.T) {
	t.Run("constructor", func(t *testing.T) {
		r := sqlRefresh(t)
		var typedNil *refreshDataClock
		for _, clock := range []refresh.Clock{nil, typedNil, &refreshDataClock{}} {
			if _, err := r.db.Refresh(r.s, r.cfg, refreshTestAuthority{}, clock); err != refresh.ErrPolicy {
				t.Fatal("missing/typed-nil/zero data clock accepted", err)
			}
		}
		var profiles int
		if err := r.raw.QueryRow(`SELECT count(*) FROM public.refresh_fills`).Scan(&profiles); err != nil || profiles != 0 {
			t.Fatal("refused clock created ownership", err)
		}
	})
	for _, position := range []struct {
		name string
		call int
	}{{"before-io", 0}, {"after-io", 1}, {"before-commit", 4}} {
		t.Run(position.name, func(t *testing.T) {
			r := sqlRefresh(t)
			a, u := mirrorInput()
			ctx := context.Background()
			clock := &refreshDataClock{now: time.Now().UTC()}
			authority := &refreshClockAuthority{clock: clock}
			gate, err := r.db.Refresh(r.s, r.cfg, authority, clock)
			if err != nil {
				t.Fatal(err)
			}
			r.g = gate
			ticket := acquireRefresh(t, r, r.demand(u.Scope, "caller-a"))
			receipt := knownRefreshAttempt(t, r, ticket)
			clock.now = time.Now().UTC()
			authority.calls, authority.zeroAt = 0, position.call
			if position.call == 0 {
				clock.now = time.Time{}
			}
			c := completionRefresh(r, ticket, &u)
			c.Attempts = receipt
			if err = r.g.Complete(ctx, ticket, r.s, a, c); err == nil {
				t.Fatal("invalid later clock committed")
			}
			if authority.calls != position.call {
				t.Fatal("clock refusal did not exercise the intended boundary")
			}
			snap := readMirror(t, r.s, a, u.Scope)
			if snap.Version != "" {
				t.Fatal("invalid later clock committed observations")
			}
			var pending int
			if err = r.raw.QueryRow(`SELECT count(*) FROM public.refresh_fills WHERE state='pending'`).Scan(&pending); err != nil || pending != 1 {
				t.Fatal("invalid clock changed ownership", err)
			}
		})
	}
}

// regression: F-refresh-data-clock-domain
func TestPGRefreshCompletionDataClock(t *testing.T) {
	r := sqlRefresh(t)
	a, u := mirrorInput()
	ctx := context.Background()
	ticket := acquireRefresh(t, r, r.demand(u.Scope, "caller-a"))
	reservation, err := r.g.BeforeAttempt(ctx, ticket.AttemptGrant)
	if err != nil {
		t.Fatal(err)
	}
	// Controlled application time is only 500ms ahead of the SQL authority.
	// This is a data-clock fixture, not a lease/floor tolerance or policy value.
	dataNow := time.Now().UTC().Add(500 * time.Millisecond)
	clock := &refreshDataClock{now: dataNow}
	gate, err := r.db.Refresh(r.s, r.cfg, refreshTestAuthority{}, clock)
	if err != nil {
		t.Fatal(err)
	}
	r.g = gate
	u.Coverage.LastAttempted = dataNow
	u.Coverage.LastComplete = dataNow
	u.Coverage.LastInventory = dataNow
	u.Coverage.ObservedUpTo = dataNow
	u.Upsert[0].ObservedAt = dataNow
	u.Upsert[0].UpdatedAt = dataNow
	if err = r.g.ObserveAttempt(ctx, reservation, mirror.AttemptObservation{HTTP: mirror.WorkCount{Value: 1, Known: true}, DecodedBytes: mirror.WorkCount{Known: true}, Status: 200, ResponseComplete: true, ObservedAt: dataNow, Retry: mirror.RetryEvidence{Knowledge: mirror.RetryNoConstraint}}); err != nil {
		t.Fatal(err)
	}
	var databaseNow time.Time
	if err = r.raw.QueryRow(`SELECT clock_timestamp()`).Scan(&databaseNow); err != nil || !dataNow.After(databaseNow) {
		t.Fatal("fixture did not establish positive application-clock skew", err)
	}
	c := completionRefresh(r, ticket, &u)
	c.Now, c.Query.Now = dataNow, dataNow
	c.Attempts = mirror.AttemptReceipt{Grant: ticket.AttemptGrant, ObservedAt: dataNow, HTTP: mirror.WorkCount{Value: 1, Known: true}, Retry: mirror.RetryEvidence{Knowledge: mirror.RetryNoConstraint}}
	if err = r.g.Complete(ctx, ticket, r.s, a, c); err != nil {
		t.Fatal(err)
	}
	state, err := r.g.Wait(ctx, ticket)
	if err != nil || state.Pending || state.AttemptHeld {
		t.Fatalf("qualified application data time permanently held: %+v %v", state, err)
	}
	snap := readMirror(t, r.s, a, u.Scope)
	if snap.Version != "1" || !snap.Coverage.LastComplete.Equal(dataNow) {
		t.Fatal("qualified source data time was dropped or restamped")
	}
}

// regression: F-refresh-denied-completion-version
func TestPGRefreshHeldCompletionPopulatedScope(t *testing.T) {
	for _, previous := range []bool{false, true} {
		for _, hold := range []mirror.AttemptHold{"", mirror.AttemptCallerHeld, mirror.AttemptAdmissionHeld} {
			name := string(hold)
			if name == "" {
				name = "admission-denied"
			}
			t.Run(name+"/previous="+map[bool]string{false: "false", true: "true"}[previous], func(t *testing.T) {
				r := sqlRefresh(t)
				a, u := mirrorInput()
				before := applyMirror(t, r.s, a, u)
				ctx := context.Background()
				first := acquireRefresh(t, r, r.demand(u.Scope, "caller-a"))
				owner := first
				if previous {
					if err := r.g.Complete(ctx, first, r.s, a, completionRefresh(r, first, nil)); err != nil {
						t.Fatal(err)
					}
					if _, err := r.g.Leave(ctx, first, refresh.Outcome{}); err != nil {
						t.Fatal(err)
					}
					owner = acquireRefresh(t, r, r.demand(u.Scope, "caller-a"))
					u.ExpectedVersion = before.Version
					u.Upsert[0].Title = "new independently committed observation"
					before = applyMirror(t, r.s, a, u)
				}
				c := completionRefresh(r, owner, nil)
				c.AdmissionDenied = hold == ""
				c.Attempts.Held = hold
				if err := r.g.Complete(ctx, owner, r.s, a, c); err != nil {
					t.Fatalf("held completion failed on populated scope: %v", err)
				}
				state, err := r.g.Wait(ctx, owner)
				if err != nil || state.Pending || !state.AttemptHeld || state.Reason != "" || !state.RetryAt.IsZero() {
					t.Fatalf("denial left pending ownership or shared failure: %+v %v", state, err)
				}
				sameMirror(t, before, readMirror(t, r.s, a, u.Scope))
				if _, err := r.g.Leave(ctx, owner, refresh.Outcome{}); err != nil {
					t.Fatal(err)
				}
				next := acquireRefresh(t, r, r.demand(u.Scope, "caller-b"))
				if !next.Owner {
					t.Fatal("denied owner poisoned the next eligible caller")
				}
			})
		}
	}
}
