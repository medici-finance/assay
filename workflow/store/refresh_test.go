package store

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/mirror/mirrortest"
	"github.com/medici-finance/assay/forgeread/refresh"
)

type refreshTestClock struct{}

func (refreshTestClock) Now() time.Time { return time.Now().UTC() }

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
	if _, e := db.Exec(`INSERT INTO public.refresh_budgets(source_id,profile,known,window_at,window_ns,http_used,http_limit,inflight_limit,reservation_limit) VALUES($1,$2,true,clock_timestamp(),$3,0,$4,$5,$6) ON CONFLICT(source_id) DO NOTHING`, []byte(c.SourceID), g.budgetProfile, int64(c.Native.Window), c.Native.SourcePerWindow, c.Native.Concurrent, c.Native.Reservations); e != nil {
		t.Fatal(e)
	}
	var existing []byte
	var known bool
	if db.QueryRow(`SELECT profile,known FROM public.refresh_budgets WHERE source_id=$1`, []byte(c.SourceID)).Scan(&existing, &known) != nil || !known || !bytes.Equal(existing, g.budgetProfile) {
		t.Fatal("synthetic shared source profile mismatch")
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
	g, e := r.db.Refresh(r.s, c, refreshTestAuthority{}, refreshTestClock{})
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
	decodedAt := time.Now().UTC()
	if e = r.g.ObserveAttempt(ctx, res, mirror.AttemptObservation{HTTP: mirror.WorkCount{Value: 1, Known: true}, DecodedBytes: mirror.WorkCount{Known: true}, Status: 200, ResponseComplete: true, Retry: mirror.RetryEvidence{Knowledge: mirror.RetryNoConstraint}, ObservedAt: time.Now()}); e != nil {
		t.Fatal(e)
	}
	return mirror.AttemptReceipt{Grant: ticket.AttemptGrant, ObservedAt: decodedAt, HTTP: mirror.WorkCount{Value: 1, Known: true}, Retry: mirror.RetryEvidence{Knowledge: mirror.RetryNoConstraint}}
}
func completionRefresh(r refreshRig, ticket refresh.Ticket, u *mirror.ScanUpdate) refresh.Completion {
	q := refresh.Query{Query: mirror.Query{Scope: ticket.AttemptGrant.Scope, MaxAge: time.Hour, Now: time.Now()}}
	return refresh.Completion{Caller: r.g.caller(ticket.AttemptGrant.CallerID), Authority: refreshTestAuthority{}, Query: q, Update: u, Now: time.Now()}
}

// regression: F-refresh-atomic-generation
func TestPGRefreshGateAtomicCompletion(t *testing.T) {
	t.Run("superseded-ticket-release", checkRefreshSupersededTicketRelease)
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
	u.Coverage.LastAttempted = time.Now().UTC()
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
	t.Run("cross-admission-source-lock", checkRefreshCrossAdmissionLock)
	t.Run("known-zero-entry", checkRefreshKnownZeroEntry)
	t.Run("unknown-decoded-prefix", checkRefreshUnknownDecodedPrefix)
	t.Run("native-data-bound", checkRefreshNativeDataBound)
	t.Run("shared-ceilings", checkRefreshSharedHTTPBudget)
	t.Run("no-aggregate-reconciliation", checkRefreshAggregateCannotReconcile)
	t.Run("provider-floor", checkRefreshProviderRetryFloor)
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
	t.Run("strong-owner-base-join", checkRefreshStrongOwnerIsolation)
	t.Run("snapshot-version-fence", checkRefreshOutcomeVersion)
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
	t.Run("postcommit-caller-refusal", checkRefreshPostcommitRefusal)
	t.Run("direct-deadline", checkRefreshDirectDeadline)
	t.Run("caller-fairness", checkRefreshFairBudget)
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
	g, _ := reopened.Refresh(s, cfg, refreshTestAuthority{}, refreshTestClock{})
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
func checkRefreshExpiry(t *testing.T) {
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
			newer, e := r.g.Acquire(ctx, r.demand(u.Scope, "caller-a"))
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
func checkRefreshFairBudget(t *testing.T) {
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
func checkRefreshSharedHTTPBudget(t *testing.T) {
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
func checkRefreshAggregateCannotReconcile(t *testing.T) {
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
func checkRefreshProviderRetryFloor(t *testing.T) {
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

func checkRefreshOutcomeVersion(t *testing.T) {
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
	d := r.demand(u.Scope, "caller-a")
	owner := acquireRefresh(t, r, d)
	waiter := acquireRefresh(t, r, r.demand(u.Scope, "caller-b"))
	c := completionRefresh(r, owner, &u)
	c.Attempts = knownRefreshAttempt(t, r, owner)
	if e := r.g.Complete(ctx, owner, r.s, a, c); e != nil {
		t.Fatal(e)
	}
	if _, e := r.g.Leave(ctx, waiter, refresh.Outcome{Checked: true, Fulfilled: false, SnapshotVersion: "999"}); e != nil {
		t.Fatal(e)
	}
	if next, e := r.g.Acquire(ctx, r.demand(u.Scope, "caller-b")); e != nil || !next.Owner {
		t.Fatal("foreign snapshot outcome poisoned current generation")
	}
}

func checkRefreshDirectDeadline(t *testing.T) {
	// Nil contexts refuse before context.WithTimeout; direct callers cannot panic
	// into an unbounded operation or erase an earlier deadline.
	cfg := testRefreshConfig()
	cfg.Limits.WaitTimeout = time.Second
	r := sqlRefreshConfig(t, cfg)
	_, u := mirrorInput()
	ticket := acquireRefresh(t, r, r.demand(u.Scope, "caller-a"))
	if _, e := r.g.Acquire(nil, r.demand(u.Scope, "caller-a")); e == nil {
		t.Fatal("nil Acquire context")
	}
	if _, e := r.g.Wait(nil, ticket); e == nil {
		t.Fatal("nil Wait context")
	}
	if e := r.g.Complete(nil, ticket, r.s, nil, refresh.Completion{}); e == nil {
		t.Fatal("nil Complete context")
	}
	if _, e := r.g.Leave(nil, ticket, refresh.Outcome{}); e == nil {
		t.Fatal("nil Leave context")
	}
	if _, e := r.g.BeforeAttempt(nil, ticket.AttemptGrant); e == nil {
		t.Fatal("nil attempt context")
	}
	if e := r.g.ObserveAttempt(nil, mirror.AttemptReservation{}, mirror.AttemptObservation{}); e == nil {
		t.Fatal("nil observation context")
	}
	ctx := context.Background()
	lock, e := r.raw.Begin()
	if e != nil {
		t.Fatal(e)
	}
	defer lock.Rollback()
	var source []byte
	if e = lock.QueryRow(`SELECT source_id FROM public.refresh_budgets WHERE source_id=$1 FOR UPDATE`, []byte(r.cfg.SourceID)).Scan(&source); e != nil {
		t.Fatal(e)
	}
	started := time.Now()
	if _, e = r.g.BeforeAttempt(ctx, ticket.AttemptGrant); e == nil {
		t.Fatal("blocked native authority granted an attempt")
	}
	if time.Since(started) > 2*time.Second {
		t.Fatal("direct authority ignored configured deadline")
	}
	lock.Rollback()
	var used int
	if e = r.raw.QueryRow(`SELECT http_used FROM public.refresh_budgets WHERE source_id=$1`, source).Scan(&used); e != nil || used != 0 {
		t.Fatal("timed-out authorization fabricated work allowance")
	}
	// The first waiter's own deadline elapsed while SQL was blocked. Renew only
	// demand (not the still-current fill or its budget) before the positive probe.
	renewed := acquireRefresh(t, r, r.demand(u.Scope, "caller-a"))
	if _, e = r.g.BeforeAttempt(ctx, renewed.AttemptGrant); e != nil {
		t.Fatal("expired SQL operation leaked its locks")
	}

}

func checkRefreshNativeDataBound(t *testing.T) {
	for _, future := range []bool{false, true} {
		t.Run(fmt.Sprint("future=", future), func(t *testing.T) {
			r := sqlRefresh(t)
			a, u := mirrorInput()
			ctx := context.Background()
			ticket := acquireRefresh(t, r, r.demand(u.Scope, "caller-a"))
			receipt := knownRefreshAttempt(t, r, ticket)
			receipt.ObservedAt = time.Time{}
			if future {
				receipt.ObservedAt = time.Now().Add(time.Hour)
			}
			c := completionRefresh(r, ticket, &u)
			c.Attempts = receipt
			if e := r.g.Complete(ctx, ticket, r.s, a, c); e != nil {
				t.Fatal(e)
			}
			state, e := r.g.Wait(ctx, ticket)
			if e != nil || !state.AttemptHeld {
				t.Fatal("unqualified native data time accepted")
			}
			if snap := readMirror(t, r.s, a, u.Scope); snap.Version != "" {
				t.Fatal("missing/future native bound promoted coverage")
			}
		})
	}
}

func checkRefreshStrongOwnerIsolation(t *testing.T) {
	r := sqlRefresh(t)
	a, u := mirrorInput()
	ctx := context.Background()
	u.Scope.Kind = mirror.Changes
	u.Upsert[0].ID.Kind = "pull-request"
	now := time.Now().UTC()
	u.Upsert[0].ObservedAt = now
	u.Upsert[0].UpdatedAt = now
	strong := r.demand(u.Scope, "caller-a")
	strong.Requirements.RequireReviews = true
	base := r.demand(u.Scope, "caller-b")
	owner := acquireRefresh(t, r, strong)
	waiter := acquireRefresh(t, r, base)
	baseline := readMirror(t, r.s, a, u.Scope)
	plan, e := mirror.PlanRefresh(baseline, now, r.cfg.Limits.Refresh)
	if e != nil {
		t.Fatal(e)
	}
	partial, e := mirror.PlanScan(baseline, plan, mirror.ScanResult{Records: u.Upsert}, now)
	if e != nil {
		t.Fatal(e)
	}
	c := completionRefresh(r, owner, &partial)
	c.Query.RequireReviews = true
	c.Attempts = knownRefreshAttempt(t, r, owner)
	if !c.ReviewOnlyPartial() {
		t.Fatal("fixture lacks canonical owner-review class")
	}
	if e = r.g.Complete(ctx, owner, r.s, a, c); e != nil {
		t.Fatal(e)
	}
	snapshot := readMirror(t, r.s, a, u.Scope)
	if snapshot.Coverage.State != mirror.Partial || !snapshot.Coverage.LastComplete.IsZero() {
		t.Fatal("review-only partial upgraded scope coverage")
	}
	state, e := r.g.Leave(ctx, waiter, refresh.Outcome{Checked: true, Fulfilled: false, SnapshotVersion: snapshot.Version, Reason: mirror.Truncated})
	if e != nil || !state.RetryAt.IsZero() {
		t.Fatal("strong owner review gap poisoned joined base demand")
	}
	state, e = r.g.Leave(ctx, owner, refresh.Outcome{Checked: true, Fulfilled: false, SnapshotVersion: snapshot.Version, Reason: mirror.Truncated})
	if e != nil || state.RetryAt.IsZero() {
		t.Fatal("strong requirement gap lost shared backoff")
	}
	next := acquireRefresh(t, r, base)
	if !next.Owner {
		t.Fatal("base demand lost its one legitimate refetch")
	}
	if _, e = r.g.Acquire(ctx, strong); e == nil {
		t.Fatal("base refetch erased stronger requirement failure")
	}
	u.ExpectedVersion = snapshot.Version
	now = time.Now().UTC()
	u.Coverage = mirror.Coverage{State: mirror.Complete, LastAttempted: now, LastComplete: now, LastInventory: now, ObservedUpTo: u.Upsert[0].UpdatedAt}
	c = completionRefresh(r, next, &u)
	c.Attempts = knownRefreshAttempt(t, r, next)
	if e = r.g.Complete(ctx, next, r.s, a, c); e != nil {
		t.Fatal(e)
	}
	fresh := readMirror(t, r.s, a, u.Scope)
	if fresh.Coverage.State != mirror.Complete || fresh.Coverage.LastComplete.IsZero() {
		t.Fatal("base refetch failed to recover qualified inventory")
	}
}

func checkRefreshUnknownDecodedPrefix(t *testing.T) {
	r := sqlRefresh(t)
	_, u := mirrorInput()
	ctx := context.Background()
	ticket := acquireRefresh(t, r, r.demand(u.Scope, "caller-a"))
	res, e := r.g.BeforeAttempt(ctx, ticket.AttemptGrant)
	if e != nil {
		t.Fatal(e)
	}
	o := mirror.AttemptObservation{HTTP: mirror.WorkCount{Value: 1, Known: true}, DecodedBytes: mirror.WorkCount{Value: 17, Known: false}, Status: 200, ResponseComplete: false, Retry: mirror.RetryEvidence{Knowledge: mirror.RetryNoConstraint}, ObservedAt: time.Now()}
	if e = r.g.ObserveAttempt(ctx, res, o); e != nil {
		t.Fatal("byte-prefix uncertainty poisoned qualified HTTP/retry observation")
	}
	var value int64
	var known, unknown bool
	if e = r.raw.QueryRow(`SELECT decoded_bytes,decoded_known,unknown_hold FROM public.refresh_attempts WHERE reservation_id=$1`, []byte(res.ID)).Scan(&value, &known, &unknown); e != nil || value != 17 || known || unknown {
		t.Fatal("unknown decoded prefix was dropped or promoted to a total")
	}
	if _, e = r.g.BeforeAttempt(ctx, ticket.AttemptGrant); e != nil {
		t.Fatal("unknown byte total blocked otherwise eligible HTTP work")
	}
}

type postcommitRefusalAuthority struct{ calls atomic.Int32 }

func (a *postcommitRefusalAuthority) Authorize(ctx context.Context, c refresh.Caller, s mirror.Scope) error {
	if e := ctx.Err(); e != nil {
		return e
	}
	if c.ID == "caller-a" && a.calls.Add(1) >= 3 {
		return mirror.ErrPermission
	}
	return nil
}
func checkRefreshPostcommitRefusal(t *testing.T) {
	cfg := testRefreshConfig()
	cfg.Limits.FillTimeout = 2 * time.Second
	r := sqlRefreshConfig(t, cfg)
	_, u := mirrorInput()
	ctx := context.Background()
	owner := acquireRefresh(t, r, r.demand(u.Scope, "caller-a"))
	r.g.authority = &postcommitRefusalAuthority{}
	if permit, e := r.g.BeforeAttempt(ctx, owner.AttemptGrant); e == nil || permit.ID != "" {
		t.Fatal("revoked caller received native permit")
	}
	var charged int
	if e := r.raw.QueryRow(`SELECT http_used FROM public.refresh_budgets WHERE source_id=$1`, []byte(r.cfg.SourceID)).Scan(&charged); e != nil || charged != 1 {
		t.Fatal("postcommit refusal refunded its charged reservation")
	}
	if wait := time.Until(owner.AttemptGrant.ExpiresAt) + 50*time.Millisecond; wait > 0 {
		time.Sleep(wait)
	}
	other, e := r.g.Acquire(ctx, r.demand(u.Scope, "caller-b"))
	if e != nil || !other.Owner {
		t.Fatal("definite no-permit caller refusal poisoned eligible B after expiry")
	}
	var observed, known, unknown bool
	var count int64
	if e = r.raw.QueryRow(`SELECT observed,http_count,http_known,unknown_hold FROM public.refresh_attempts WHERE generation=$1`, []byte(owner.Generation)).Scan(&observed, &count, &known, &unknown); e != nil || !observed || !known || count != 0 || unknown {
		t.Fatal("certain zero-entry work was not distinguished from unknown completion")
	}
}

func checkRefreshKnownZeroEntry(t *testing.T) {
	r := sqlRefresh(t)
	_, u := mirrorInput()
	ctx := context.Background()
	ticket := acquireRefresh(t, r, r.demand(u.Scope, "caller-a"))
	res, e := r.g.BeforeAttempt(ctx, ticket.AttemptGrant)
	if e != nil {
		t.Fatal(e)
	}
	zero := mirror.AttemptObservation{HTTP: mirror.WorkCount{Known: true}, DecodedBytes: mirror.WorkCount{Known: true}, Status: 0, ResponseComplete: false, Retry: mirror.RetryEvidence{Knowledge: mirror.RetryNoConstraint}, ObservedAt: time.Now()}
	if e = r.g.ObserveAttempt(ctx, res, zero); e != nil {
		t.Fatal("qualified lower pre-base refusal held as unknown HTTP")
	}
	var charged, http int
	var known bool
	if e = r.raw.QueryRow(`SELECT b.http_used,a.http_count,a.http_known FROM public.refresh_budgets b JOIN public.refresh_attempts a USING(source_id) WHERE reservation_id=$1`, []byte(res.ID)).Scan(&charged, &http, &known); e != nil || charged != 1 || http != 0 || !known {
		t.Fatal("zero entry lost reservation charge or measured-zero distinction")
	}
	if _, e = r.g.BeforeAttempt(ctx, ticket.AttemptGrant); e != nil {
		t.Fatal("known zero-entry slot poisoned eligible future work")
	}
}

func checkRefreshCrossAdmissionLock(t *testing.T) {
	cfg := testRefreshConfig()
	cfg.Native.Concurrent = 1
	cfg.Limits.FillTimeout = 20 * time.Second
	r := sqlRefreshConfig(t, cfg)
	_, u := mirrorInput()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	bcfg := cfg
	bcfg.AdmissionID = "synthetic-b"
	otherDB, e := Open(ctx, r.config)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { otherDB.Close() })
	otherStore, _ := otherDB.Mirror(r.s.limits)
	other, e := otherDB.Refresh(otherStore, bcfg, refreshTestAuthority{}, refreshTestClock{})
	if e != nil {
		t.Fatal(e)
	}
	seedRefresh(t, r.raw, other, "caller-b")
	aTicket := acquireRefresh(t, r, r.demand(u.Scope, "caller-a"))
	bScope := u.Scope
	bScope.AdmissionID = bcfg.AdmissionID
	bRig := refreshRig{mirrorRig: r.mirrorRig, g: other, cfg: bcfg}
	bTicket := acquireRefresh(t, bRig, bRig.demand(bScope, "caller-b"))
	block, e := r.raw.BeginTx(ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer block.Rollback()
	var source []byte
	if e = block.QueryRowContext(ctx, `SELECT source_id FROM public.refresh_budgets WHERE source_id=$1 FOR UPDATE`, []byte(cfg.SourceID)).Scan(&source); e != nil {
		t.Fatal(e)
	}
	type answer struct {
		gate        *RefreshGate
		grant       mirror.AttemptGrant
		reservation mirror.AttemptReservation
		err         error
	}
	answers := make(chan answer, 2)
	var workers sync.WaitGroup
	for _, input := range []struct {
		gate  *RefreshGate
		grant mirror.AttemptGrant
	}{{r.g, aTicket.AttemptGrant}, {other, bTicket.AttemptGrant}} {
		workers.Add(1)
		go func(g *RefreshGate, grant mirror.AttemptGrant) {
			defer workers.Done()
			res, e := g.BeforeAttempt(ctx, grant)
			answers <- answer{g, grant, res, e}
		}(input.gate, input.grant)
	}
	t.Cleanup(func() { cancel(); block.Rollback(); workers.Wait() })
	deadline := time.Now().Add(2 * time.Second)
	for {
		var waiting int
		if e = r.raw.QueryRowContext(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%public.refresh_budgets%FOR UPDATE%'`).Scan(&waiting); e != nil {
			t.Fatal(e)
		}
		if waiting == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("both independent admissions did not block on source anchor")
		}
		time.Sleep(time.Millisecond)
	}
	if e = block.Rollback(); e != nil {
		t.Fatal(e)
	}
	one, two := <-answers, <-answers
	winner, loser := one, two
	if winner.err != nil {
		winner, loser = loser, winner
	}
	if winner.err != nil || loser.err == nil {
		t.Fatal("two admissions overspent one canonical in-flight slot")
	}
	if e = winner.gate.ObserveAttempt(ctx, winner.reservation, mirror.AttemptObservation{HTTP: mirror.WorkCount{Value: 1, Known: true}, Status: 200, ResponseComplete: true, Retry: mirror.RetryEvidence{Knowledge: mirror.RetryNoConstraint}, ObservedAt: time.Now()}); e != nil {
		t.Fatal(e)
	}
	if _, e = loser.gate.BeforeAttempt(ctx, loser.grant); e != nil {
		t.Fatal("qualified shared slot release did not admit waiting domain")
	}
}

// regression: F-refresh-superseded-waiter-accounting
func checkRefreshSupersededTicketRelease(t *testing.T) {
	r := sqlRefresh(t)
	a, u := mirrorInput()
	ctx := context.Background()
	first := acquireRefresh(t, r, r.demand(u.Scope, "caller-a"))
	c := completionRefresh(r, first, &u)
	c.Attempts = knownRefreshAttempt(t, r, first)
	if e := r.g.Complete(ctx, first, r.s, a, c); e != nil {
		t.Fatal(e)
	}
	next := acquireRefresh(t, r, r.demand(u.Scope, "caller-b"))
	if !next.Owner || next.Generation == first.Generation {
		t.Fatal("completed generation blocked a new freshness demand")
	}
	if _, e := r.g.Wait(ctx, first); !errors.Is(e, refresh.ErrStale) {
		t.Fatal("superseded wait unexpectedly current", e)
	}
	// Old tickets cannot poison the next generation, but their exact waiter debit
	// must release rather than consuming capacity until the old ticket expires.
	if _, e := r.g.Leave(ctx, first, refresh.Outcome{Checked: true, Fulfilled: false, SnapshotVersion: "1"}); e != nil {
		t.Fatal(e)
	}
	var released bool
	if e := r.raw.QueryRow(`SELECT released FROM public.refresh_waiters WHERE ticket_id=$1`, []byte(first.ID)).Scan(&released); e != nil || !released {
		t.Fatal("superseded waiter debit was not released", e)
	}
	var generation []byte
	var base, reviews *time.Time
	if e := r.raw.QueryRow(`SELECT generation,base_retry_at,review_retry_at FROM public.refresh_fills`).Scan(&generation, &base, &reviews); e != nil || string(generation) != next.Generation || base != nil || reviews != nil {
		t.Fatal("superseded outcome poisoned the current generation", e)
	}
}

// regression: F-refresh-absence-proof
// Reuse the canonical nine-case Store witness through actual Gate.Complete, not
// just direct Apply: a tx-bound Store must retain the same no-op absence guard.
func TestPGRefreshGateAbsence(t *testing.T) {
	r := sqlMirror(t)
	adapter := &refreshAbsenceStore{rig: r, t: t, gates: map[string]*RefreshGate{}}
	mirrortest.CheckPartialAbsence(t, func() mirror.Store { return adapter })
}

type refreshAbsenceStore struct {
	rig   mirrorRig
	t     *testing.T
	gates map[string]*RefreshGate
}

func (s *refreshAbsenceStore) Read(ctx context.Context, a mirror.Admission, scope mirror.Scope) (mirror.Snapshot, error) {
	return s.rig.s.Read(ctx, a, scope)
}
func (s *refreshAbsenceStore) Apply(ctx context.Context, a mirror.Admission, u mirror.ScanUpdate) (mirror.Snapshot, error) {
	g := s.gates[u.Scope.AdmissionID]
	if g == nil {
		cfg := testRefreshConfig()
		cfg.AdmissionID = u.Scope.AdmissionID
		var err error
		g, err = s.rig.db.Refresh(s.rig.s, cfg, refreshTestAuthority{}, refreshTestClock{})
		if err != nil {
			return mirror.Snapshot{}, err
		}
		seedRefresh(s.t, s.rig.raw, g, "caller-a")
		s.gates[u.Scope.AdmissionID] = g
	}
	r := refreshRig{mirrorRig: s.rig, g: g, cfg: g.config}
	ticket, err := g.Acquire(ctx, r.demand(u.Scope, "caller-a"))
	if err != nil {
		return mirror.Snapshot{}, err
	}
	if !ticket.Owner {
		return mirror.Snapshot{}, refresh.ErrHeld
	}
	defer func() {
		if _, err := g.Leave(context.Background(), ticket, refresh.Outcome{}); err != nil {
			s.t.Errorf("owned absence ticket cleanup: %v", err)
		}
	}()
	c := completionRefresh(r, ticket, &u)
	c.Attempts = knownRefreshAttempt(s.t, r, ticket)
	if err = g.Complete(ctx, ticket, s.rig.s, a, c); err != nil {
		return mirror.Snapshot{}, err
	}
	return s.rig.s.Read(ctx, a, u.Scope)
}
