package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/refresh"
)

type refreshTestAuthority struct{}

func (refreshTestAuthority) Authorize(ctx context.Context, c refresh.Caller, s mirror.Scope) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if c.ID == "" || c.AdmissionID != s.AdmissionID {
		return mirror.ErrPermission
	}
	return nil
}

type refreshRig struct {
	mirrorRig
	g   *RefreshGate
	cfg RefreshConfig
}

func testRefreshConfig() RefreshConfig {
	return RefreshConfig{SourceID: "synthetic-source", AdmissionID: "synthetic-a", AccessEpoch: "access-1", ConfigEpoch: "config-1", PolicyClass: "synthetic", PolicyEpoch: "policy-1", Limits: refresh.Limits{CallerFillsPerWindow: 100, AdmissionFillsPerWindow: 1000, FillWindow: time.Hour, MinFreshness: time.Millisecond, MaxFreshness: time.Hour, ActiveFills: 10, QueuedCallers: 100, AdmissionFills: 50, AdmissionWaiters: 1000, FailureEntries: 100, WaitTimeout: 10 * time.Second, FillTimeout: 5 * time.Second, Cooldown: time.Second, FailureTTL: time.Minute, Refresh: mirror.RefreshPolicy{FullInventoryInterval: time.Hour}}, Native: NativeLimits{CallerPerWindow: 100, AdmissionPerWindow: 1000, SourcePerWindow: 1000, PerFill: 20, Concurrent: 5, Reservations: 1000, Window: time.Hour}, PollInterval: time.Millisecond}
}
func seedRefresh(t *testing.T, db *sql.DB, g *RefreshGate, callers ...string) {
	t.Helper()
	c := g.config
	if _, e := db.Exec(`INSERT INTO public.refresh_budgets(source_id,profile,known,window_at,window_ns,http_used,http_limit,inflight_limit,reservation_limit) VALUES($1,$2,true,clock_timestamp(),$3,0,$4,$5,$6)`, []byte(c.SourceID), g.budgetProfile, int64(c.Native.Window), c.Native.SourcePerWindow, c.Native.Concurrent, c.Native.Reservations); e != nil {
		t.Fatal(e)
	}
	if _, e := db.Exec(`INSERT INTO public.refresh_admissions(admission,source_id,profile,known,window_at,window_ns,fills_used,fill_limit,http_used,http_limit,active_limit,waiter_limit,failure_limit) VALUES($1,$2,$3,true,clock_timestamp(),$4,0,$5,0,$6,$7,$8,$9)`, []byte(c.AdmissionID), []byte(c.SourceID), g.profile, int64(c.Limits.FillWindow), c.Limits.AdmissionFillsPerWindow, c.Native.AdmissionPerWindow, c.Limits.AdmissionFills, c.Limits.AdmissionWaiters, c.Limits.FailureEntries); e != nil {
		t.Fatal(e)
	}
	for _, id := range callers {
		if _, e := db.Exec(`INSERT INTO public.refresh_callers(admission,caller_id,known,window_at,fills_used,fill_limit,http_used,http_limit,active_limit,waiter_limit) VALUES($1,$2,true,clock_timestamp(),0,$3,0,$4,$5,$6)`, []byte(c.AdmissionID), []byte(id), c.Limits.CallerFillsPerWindow, c.Native.CallerPerWindow, c.Limits.ActiveFills, c.Limits.QueuedCallers); e != nil {
			t.Fatal(e)
		}
	}
}
func sqlRefresh(t *testing.T) refreshRig { return sqlRefreshConfig(t, testRefreshConfig()) }
func sqlRefreshConfig(t *testing.T, c RefreshConfig) refreshRig {
	t.Helper()
	r := sqlMirror(t)
	g, e := r.db.Refresh(r.s, c, refreshTestAuthority{})
	if e != nil {
		t.Fatal(e)
	}
	seedRefresh(t, r.raw, g, "caller-a", "caller-b")
	return refreshRig{r, g, c}
}
func (r refreshRig) demand(scope mirror.Scope, id string) refresh.Demand {
	c := r.cfg
	return refresh.Demand{Key: refresh.FillKey{Scope: scope, SourceID: c.SourceID, AccessEpoch: c.AccessEpoch, ConfigEpoch: c.ConfigEpoch, PolicyClass: c.PolicyClass, PolicyEpoch: c.PolicyEpoch}, CallerID: id, Limits: c.Limits, Now: time.Now()}
}
func acquireRefresh(t *testing.T, r refreshRig, d refresh.Demand) refresh.Ticket {
	t.Helper()
	ticket, e := r.g.Acquire(context.Background(), d)
	if e != nil {
		t.Fatal(e)
	}
	return ticket
}
func knownRefreshAttempt(t *testing.T, r refreshRig, ticket refresh.Ticket) mirror.AttemptReceipt {
	t.Helper()
	ctx := context.Background()
	res, e := r.g.BeforeAttempt(ctx, ticket.AttemptGrant)
	if e != nil {
		t.Fatal(e)
	}
	if e = r.g.ObserveAttempt(ctx, res, mirror.AttemptObservation{HTTP: mirror.WorkCount{Value: 1, Known: true}, DecodedBytes: mirror.WorkCount{Known: true}, Status: 200, ResponseComplete: true, Retry: mirror.RetryEvidence{Knowledge: mirror.RetryNoConstraint}, ObservedAt: time.Now()}); e != nil {
		t.Fatal(e)
	}
	return mirror.AttemptReceipt{Grant: ticket.AttemptGrant, HTTP: mirror.WorkCount{Value: 1, Known: true}, Retry: mirror.RetryEvidence{Knowledge: mirror.RetryNoConstraint}}
}
func completionRefresh(r refreshRig, ticket refresh.Ticket, u *mirror.ScanUpdate) refresh.Completion {
	q := refresh.Query{Query: mirror.Query{Scope: ticket.AttemptGrant.Scope, MaxAge: time.Hour, Now: time.Now()}}
	return refresh.Completion{Caller: r.g.caller(ticket.AttemptGrant.CallerID), Authority: refreshTestAuthority{}, Query: q, Update: u, Now: time.Now()}
}

// regression: F-refresh-atomic-generation
func TestPGRefreshGateAtomicCompletion(t *testing.T) {
	r := sqlRefresh(t)
	a, u := mirrorInput()
	now := time.Now().UTC()
	u.Coverage.LastAttempted = now
	u.Coverage.LastComplete = now
	u.Coverage.LastInventory = now
	u.Coverage.ObservedUpTo = now
	u.Upsert[0].UpdatedAt = now
	u.Upsert[0].ObservedAt = now
	ctx := context.Background()
	ticket := acquireRefresh(t, r, r.demand(u.Scope, "caller-a"))
	other := acquireRefresh(t, r, r.demand(u.Scope, "caller-b"))
	if !ticket.Owner || other.Owner || !other.Joined || other.Generation != ticket.Generation {
		t.Fatal("SQL did not coalesce")
	}
	receipt := knownRefreshAttempt(t, r, ticket)
	c := completionRefresh(r, ticket, &u)
	c.Attempts = receipt
	foreign, _ := r.db.Mirror(r.s.limits)
	if e := r.g.Complete(ctx, ticket, foreign, a, c); !errors.Is(e, refresh.ErrStale) {
		t.Fatal("foreign Store accepted")
	}
	if e := r.g.Complete(ctx, ticket, r.s, a, c); e != nil {
		t.Fatal(e)
	}
	snap := readMirror(t, r.s, a, u.Scope)
	if snap.Version != "1" || len(snap.Records) != 1 {
		t.Fatal("atomic observation absent")
	}
	state, e := r.g.Wait(ctx, other)
	if e != nil || state.Pending || state.AttemptHeld {
		t.Fatalf("completion state: %+v %v", state, e)
	}
	if e := r.g.Complete(ctx, ticket, r.s, a, c); !errors.Is(e, refresh.ErrStale) {
		t.Fatal("completion replay accepted")
	}
	if _, e = r.g.Leave(ctx, ticket, refresh.Outcome{Checked: true, Fulfilled: true, SnapshotVersion: snap.Version}); e != nil {
		t.Fatal(e)
	}
	if _, e = r.g.Leave(ctx, other, refresh.Outcome{}); e != nil {
		t.Fatal(e)
	}
	// Failure after row staging must roll back the observation and completion.
	next := acquireRefresh(t, r, r.demand(u.Scope, "caller-b"))
	u.ExpectedVersion = snap.Version
	u.Upsert[0].Title = "must roll back"
	u.Coverage.LastAttempted = u.Coverage.LastAttempted.Add(time.Second)
	u.Coverage.LastComplete = u.Coverage.LastAttempted
	u.Coverage.LastInventory = u.Coverage.LastAttempted
	if _, e = r.raw.Exec(`CREATE FUNCTION public.fixture_refresh_failure() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.state='done' THEN RAISE EXCEPTION 'synthetic completion failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER fixture_refresh_failure BEFORE UPDATE ON public.refresh_fills FOR EACH ROW EXECUTE FUNCTION public.fixture_refresh_failure()`); e != nil {
		t.Fatal(e)
	}
	c = completionRefresh(r, next, &u)
	c.Attempts = knownRefreshAttempt(t, r, next)
	if e = r.g.Complete(ctx, next, r.s, a, c); e == nil {
		t.Fatal("injected failure accepted")
	}
	sameMirror(t, snap, readMirror(t, r.s, a, u.Scope))
}

// regression: F-refresh-native-reservations
func TestPGRefreshAttemptAuthority(t *testing.T) {
	r := sqlRefresh(t)
	_, u := mirrorInput()
	ctx := context.Background()
	ticket := acquireRefresh(t, r, r.demand(u.Scope, "caller-a"))
	grant := ticket.AttemptGrant
	foreign := grant
	foreign.Scope.Repository.Path = "foreign/project"
	if _, e := r.g.BeforeAttempt(ctx, foreign); e == nil {
		t.Fatal("foreign grant accepted")
	}
	first, e := r.g.BeforeAttempt(ctx, grant)
	if e != nil {
		t.Fatal(e)
	}
	second, e := r.g.BeforeAttempt(ctx, grant)
	if e != nil {
		t.Fatal(e)
	}
	// A skewed source clock cannot shorten a qualified relative provider floor.
	skewed := time.Now().Add(-24 * time.Hour)
	observation := mirror.AttemptObservation{HTTP: mirror.WorkCount{Value: 1, Known: true}, Status: 429, Retry: mirror.RetryEvidence{Knowledge: mirror.RetryNotBefore, NotBefore: skewed.Add(time.Second), Delay: time.Second, DelayKnown: true}, ObservedAt: skewed}
	before := time.Now()
	if e = r.g.ObserveAttempt(ctx, first, observation); e != nil {
		t.Fatal(e)
	}
	var floor time.Time
	if e = r.raw.QueryRow(`SELECT retry_at FROM public.refresh_budgets WHERE source_id=$1`, []byte(r.cfg.SourceID)).Scan(&floor); e != nil || floor.Before(before.Add(time.Second)) {
		t.Fatal("relative floor shortened by source clock")
	}
	if _, e = r.g.BeforeAttempt(ctx, grant); e == nil {
		t.Fatal("known shared floor ignored")
	}
	if e = r.g.ObserveAttempt(ctx, first, observation); e == nil {
		t.Fatal("duplicate observation accepted")
	}
	observation.Retry = mirror.RetryEvidence{Knowledge: mirror.RetryUnknown}
	if e = r.g.ObserveAttempt(ctx, second, observation); e == nil {
		t.Fatal("unknown report not held")
	}
	var used int
	var held bool
	if e = r.raw.QueryRow(`SELECT http_used,unknown_hold FROM public.refresh_budgets WHERE source_id=$1`, []byte(r.cfg.SourceID)).Scan(&used, &held); e != nil || used != 2 || !held {
		t.Fatal("unknown work refunded")
	}
}

// regression: F-refresh-requirement-generation
func TestPGRefreshRequirementBackoff(t *testing.T) {
	r := sqlRefresh(t)
	a, u := mirrorInput()
	ctx := context.Background()
	u.Scope.Kind = mirror.Changes
	u.Upsert[0].ID.Kind = "pull-request"
	now := time.Now().UTC()
	u.Coverage.LastAttempted = now
	u.Coverage.LastComplete = now
	u.Coverage.LastInventory = now
	u.Coverage.ObservedUpTo = now
	u.Upsert[0].UpdatedAt = now
	u.Upsert[0].ObservedAt = now
	base := r.demand(u.Scope, "caller-a")
	strong := r.demand(u.Scope, "caller-b")
	strong.Requirements.RequireReviews = true
	owner := acquireRefresh(t, r, base)
	waiter := acquireRefresh(t, r, strong)
	c := completionRefresh(r, owner, &u)
	c.Attempts = knownRefreshAttempt(t, r, owner)
	if e := r.g.Complete(ctx, owner, r.s, a, c); e != nil {
		t.Fatal(e)
	}
	snap := readMirror(t, r.s, a, u.Scope)
	state, e := r.g.Leave(ctx, waiter, refresh.Outcome{Checked: true, Fulfilled: false, SnapshotVersion: snap.Version, Reason: mirror.Unknown})
	if e != nil || state.RetryAt.IsZero() {
		t.Fatal("strong requirement failure not shared")
	}
	if _, e = r.g.Acquire(ctx, strong); e == nil {
		t.Fatal("strong repeat bypassed shared backoff")
	}
	next := acquireRefresh(t, r, base)
	if next.Generation == owner.Generation {
		t.Fatal("new base generation absent")
	}
	// A fresh base generation must not erase the stronger requirement cooldown.
	if _, e = r.g.Acquire(ctx, strong); e == nil {
		t.Fatal("base fill erased review requirement failure")
	}
	oldState, e := r.g.Leave(ctx, owner, refresh.Outcome{Checked: true, Fulfilled: false, SnapshotVersion: snap.Version})
	if e != nil || !oldState.RetryAt.IsZero() {
		t.Fatal("stale outcome poisoned recovery")
	}
	// Unchecked cancellation creates no failure; only its waiter debit is released.
	if _, e = r.g.Leave(ctx, next, refresh.Outcome{}); e != nil {
		t.Fatal(e)
	}
}

// regression: F-refresh-capacity-known-accounting
func TestPGRefreshCapacityCancellation(t *testing.T) {
	r := sqlRefresh(t)
	_, u := mirrorInput()
	ctx := context.Background()
	first := acquireRefresh(t, r, r.demand(u.Scope, "caller-a"))
	if _, e := r.g.Acquire(ctx, r.demand(u.Scope, "missing-caller")); e == nil {
		t.Fatal("missing caller usage treated as zero")
	}
	// A queued cancellation releases precisely its own waiter, retaining the owner.
	other := acquireRefresh(t, r, r.demand(u.Scope, "caller-b"))
	if _, e := r.g.Leave(ctx, other, refresh.Outcome{}); e != nil {
		t.Fatal(e)
	}
	var live int
	if e := r.raw.QueryRow(`SELECT count(*) FROM public.refresh_waiters WHERE NOT released`).Scan(&live); e != nil || live != 1 {
		t.Fatal("waiter cancellation lost another demand")
	}
	if _, e := r.g.BeforeAttempt(ctx, first.AttemptGrant); e != nil {
		t.Fatal(e)
	}
	// Unknown native work survives a new client and a caller policy epoch change.
	reopened, e := Open(ctx, r.config)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	s, _ := reopened.Mirror(r.s.limits)
	cfg := r.cfg
	cfg.PolicyEpoch = "new-policy"
	g, _ := reopened.Refresh(s, cfg, refreshTestAuthority{})
	d := r.demand(u.Scope, "caller-b")
	d.Key.PolicyEpoch = cfg.PolicyEpoch
	if _, e = g.Acquire(ctx, d); e == nil {
		t.Fatal("new epoch reset canonical usage")
	}
	if _, e = r.raw.Exec(`UPDATE public.refresh_budgets SET http_used=0,window_at=clock_timestamp() WHERE source_id=$1`, []byte(r.cfg.SourceID)); e == nil {
		t.Fatal("raw SQL reset unresolved debit")
	}
	if _, e = r.raw.Exec(`DELETE FROM public.refresh_budgets WHERE source_id=$1`, []byte(r.cfg.SourceID)); e == nil {
		t.Fatal("budget eviction accepted")
	}
}

// regression: F-refresh-expiry-without-invented-receipt
func TestPGRefreshExpiry(t *testing.T) {
	for _, issued := range []bool{false, true} {
		t.Run(fmt.Sprint("issued=", issued), func(t *testing.T) {
			cfg := testRefreshConfig()
			cfg.Limits.FillTimeout = 300 * time.Millisecond
			r := sqlRefreshConfig(t, cfg)
			a, u := mirrorInput()
			ctx := context.Background()
			ticket := acquireRefresh(t, r, r.demand(u.Scope, "caller-a"))
			if issued {
				if _, e := r.g.BeforeAttempt(ctx, ticket.AttemptGrant); e != nil {
					t.Fatal(e)
				}
			}
			time.Sleep(350 * time.Millisecond)
			newer, e := r.g.Acquire(ctx, r.demand(u.Scope, "caller-b"))
			if issued {
				if e == nil {
					t.Fatal("unknown issued attempt recovered as zero")
				}
				return
			}
			if e != nil || !newer.Owner || newer.Generation == ticket.Generation {
				t.Fatal("proved zero-attempt expiry failed recovery")
			}
			c := completionRefresh(r, ticket, &u)
			if e = r.g.Complete(ctx, ticket, r.s, a, c); !errors.Is(e, refresh.ErrStale) {
				t.Fatal("stale owner published matching empty snapshot")
			}
			snap := readMirror(t, r.s, a, u.Scope)
			if snap.Version != "" || snap.Coverage.State != mirror.NeverRead {
				t.Fatal("expired owner advanced coverage")
			}
		})
	}
}

// regression: F-refresh-caller-fair-budget
func TestPGRefreshFairBudget(t *testing.T) {
	cfg := testRefreshConfig()
	cfg.Limits.CallerFillsPerWindow = 1
	cfg.Limits.QueuedCallers = 1
	r := sqlRefreshConfig(t, cfg)
	_, u := mirrorInput()
	ctx := context.Background()
	first := acquireRefresh(t, r, r.demand(u.Scope, "caller-a"))
	different := u.Scope
	different.Repository.Path = "example/other"
	if _, e := r.g.Acquire(ctx, r.demand(different, "caller-a")); !errors.Is(e, refresh.ErrQuota) {
		t.Fatal("caller saturation not enforced")
	}
	second := acquireRefresh(t, r, r.demand(different, "caller-b"))
	if !second.Owner {
		t.Fatal("caller A saturation starved eligible B")
	}
	if _, e := r.g.Leave(ctx, first, refresh.Outcome{}); e != nil {
		t.Fatal(e)
	}
	if _, e := r.g.BeforeAttempt(ctx, first.AttemptGrant); e == nil {
		t.Fatal("abandoned fill spent native budget")
	}
}

// regression: F-refresh-shared-native-ceilings
func TestPGRefreshSharedHTTPBudget(t *testing.T) {
	t.Run("caller", func(t *testing.T) {
		cfg := testRefreshConfig()
		cfg.Native.CallerPerWindow = 1
		cfg.Native.AdmissionPerWindow = 2
		cfg.Native.SourcePerWindow = 2
		r := sqlRefreshConfig(t, cfg)
		_, u := mirrorInput()
		ctx := context.Background()
		first := acquireRefresh(t, r, r.demand(u.Scope, "caller-a"))
		_ = knownRefreshAttempt(t, r, first)
		other := u.Scope
		other.Repository.Path = "example/other"
		second := acquireRefresh(t, r, r.demand(other, "caller-a"))
		if _, e := r.g.BeforeAttempt(ctx, second.AttemptGrant); e == nil {
			t.Fatal("caller HTTP ceiling bypassed")
		}
		thirdScope := u.Scope
		thirdScope.Repository.Path = "example/third"
		third := acquireRefresh(t, r, r.demand(thirdScope, "caller-b"))
		_ = knownRefreshAttempt(t, r, third)
		var used int
		if e := r.raw.QueryRow(`SELECT http_used FROM public.refresh_budgets WHERE source_id=$1`, []byte(r.cfg.SourceID)).Scan(&used); e != nil || used != 2 {
			t.Fatal("caller refusal consumed another caller allowance")
		}
	})
	t.Run("concurrency", func(t *testing.T) {
		cfg := testRefreshConfig()
		cfg.Native.Concurrent = 1
		r := sqlRefreshConfig(t, cfg)
		_, u := mirrorInput()
		ctx := context.Background()
		ticket := acquireRefresh(t, r, r.demand(u.Scope, "caller-a"))
		first, e := r.g.BeforeAttempt(ctx, ticket.AttemptGrant)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = r.g.BeforeAttempt(ctx, ticket.AttemptGrant); e == nil {
			t.Fatal("inflight ceiling bypassed")
		}
		if e = r.g.ObserveAttempt(ctx, first, mirror.AttemptObservation{HTTP: mirror.WorkCount{Value: 1, Known: true}, Status: 200, ResponseComplete: true, ObservedAt: time.Now(), Retry: mirror.RetryEvidence{Knowledge: mirror.RetryNoConstraint}}); e != nil {
			t.Fatal(e)
		}
		if _, e = r.g.BeforeAttempt(ctx, ticket.AttemptGrant); e != nil {
			t.Fatal("qualified own report did not release its finite slot")
		}
	})
}

// regression: F-refresh-no-invented-individual-observation
func TestPGRefreshAggregateCannotReconcile(t *testing.T) {
	r := sqlRefresh(t)
	a, u := mirrorInput()
	ctx := context.Background()
	ticket := acquireRefresh(t, r, r.demand(u.Scope, "caller-a"))
	receipt := knownRefreshAttempt(t, r, ticket)
	if _, e := r.g.BeforeAttempt(ctx, ticket.AttemptGrant); e != nil {
		t.Fatal(e)
	}
	receipt.HTTP.Value = 2
	c := completionRefresh(r, ticket, &u)
	c.Attempts = receipt
	if e := r.g.Complete(ctx, ticket, r.s, a, c); e != nil {
		t.Fatal(e)
	}
	state, e := r.g.Wait(ctx, ticket)
	if e != nil || !state.AttemptHeld {
		t.Fatal("aggregate invented missing individual report")
	}
	snap := readMirror(t, r.s, a, u.Scope)
	if snap.Version != "" || snap.Coverage.State != mirror.NeverRead {
		t.Fatal("unknown work promoted observation coverage")
	}
}

// regression: F-refresh-canonical-rate-floor
func TestPGRefreshProviderRetryFloor(t *testing.T) {
	r := sqlRefresh(t)
	a, u := mirrorInput()
	ctx := context.Background()
	ticket := acquireRefresh(t, r, r.demand(u.Scope, "caller-a"))
	res, e := r.g.BeforeAttempt(ctx, ticket.AttemptGrant)
	if e != nil {
		t.Fatal(e)
	}
	reported := time.Now().Add(-time.Hour)
	evidence := mirror.RetryEvidence{Knowledge: mirror.RetryNotBefore, NotBefore: reported, Delay: 3 * time.Second, DelayKnown: true}
	if e = r.g.ObserveAttempt(ctx, res, mirror.AttemptObservation{HTTP: mirror.WorkCount{Value: 1, Known: true}, Status: 429, ResponseComplete: true, ObservedAt: reported, Retry: evidence}); e != nil {
		t.Fatal(e)
	}
	c := completionRefresh(r, ticket, nil)
	c.Failure = mirror.RateLimited
	c.Attempts = mirror.AttemptReceipt{Grant: ticket.AttemptGrant, HTTP: mirror.WorkCount{Value: 1, Known: true}, Retry: evidence}
	if e = r.g.Complete(ctx, ticket, r.s, a, c); e != nil {
		t.Fatal(e)
	}
	state, e := r.g.Wait(ctx, ticket)
	if e != nil || state.RetryAt.Before(time.Now().Add(2*time.Second)) {
		t.Fatal("fixed cooldown shortened canonical provider floor")
	}
	if _, e = r.g.Acquire(ctx, r.demand(u.Scope, "caller-b")); e == nil {
		t.Fatal("failure retry floor ignored by another caller")
	}
}
