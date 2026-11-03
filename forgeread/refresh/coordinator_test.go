package refresh

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/mirror/mirrortest"
)

type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Now() time.Time       { c.mu.Lock(); defer c.mu.Unlock(); return c.now }
func (c *testClock) step(d time.Duration) { c.mu.Lock(); c.now = c.now.Add(d); c.mu.Unlock() }

type testAdmission struct {
	name    string
	allowed atomic.Bool
}

func (a *testAdmission) ID() string { return a.name }
func (a *testAdmission) Authorize(context.Context, mirror.RepositoryID) error {
	if !a.allowed.Load() {
		return mirror.ErrPermission
	}
	return nil
}

type accountKey struct{ admission, caller string }
type testAuthority struct {
	mu      sync.Mutex
	current map[accountKey]Caller
}

func (a *testAuthority) Authorize(_ context.Context, c Caller, s mirror.Scope) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.current[accountKey{c.AdmissionID, c.ID}] != c || c.AdmissionID != s.AdmissionID {
		return mirror.ErrPermission
	}
	return nil
}
func (a *testAuthority) set(c Caller) {
	a.mu.Lock()
	a.current[accountKey{c.AdmissionID, c.ID}] = c
	a.mu.Unlock()
}
func (a *testAuthority) revoke(id string) {
	a.mu.Lock()
	for k := range a.current {
		if k.caller == id {
			delete(a.current, k)
		}
	}
	a.mu.Unlock()
}

type testSource struct {
	calls atomic.Int64
	fn    func(context.Context, mirror.Admission, mirror.Scope, mirror.ScanPlan) (mirror.ScanResult, error)
}

func (s *testSource) Scan(c context.Context, a mirror.Admission, q mirror.Scope, p mirror.ScanPlan) (mirror.ScanResult, error) {
	s.calls.Add(1)
	return s.fn(c, a, q, p)
}

type gateEntry struct {
	key        FillKey
	generation string
	ready      chan struct{}
	state      State
	expiry     time.Time
	limits     Limits
	owner      string
	waiters    int
}
type gateWaiter struct {
	entry  *gateEntry
	caller string
	left   bool
}

// fakeGate is stateful contract support only. It is deliberately in a test file:
// its mutex is no PostgreSQL transaction, process lease or durability proof.
type fillCounter struct {
	window int64
	used   int
}
type fakeGate struct {
	windows         map[accountKey]fillCounter
	mu              sync.Mutex
	entries         map[FillKey]*gateEntry
	waiters         map[string]*gateWaiter
	serial          int
	down, leaveDown bool
	acquired        chan struct{}
	completions     chan error
	active, queued  map[accountKey]int
}

func newGate() *fakeGate {
	return &fakeGate{windows: map[accountKey]fillCounter{}, entries: map[FillKey]*gateEntry{}, waiters: map[string]*gateWaiter{}, active: map[accountKey]int{}, queued: map[accountKey]int{}, acquired: make(chan struct{}, 256), completions: make(chan error, 256)}
}
func (g *fakeGate) Acquire(ctx context.Context, d Demand) (Ticket, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return Ticket{}, err
	}
	if g.down {
		return Ticket{}, ErrHeld
	}
	for k, e := range g.entries {
		if !e.state.Pending && e.state.Reason != "" && !d.Now.Before(e.expiry) {
			delete(g.entries, k)
		}
	}
	if g.queued[accountKey{d.Key.Scope.AdmissionID, d.CallerID}] >= d.Limits.QueuedCallers {
		return Ticket{}, ErrQuota
	}
	admQueued, admActive, failed := 0, 0, 0
	for _, w := range g.waiters {
		if !w.left && w.entry.key.Scope.AdmissionID == d.Key.Scope.AdmissionID {
			admQueued++
		}
	}
	for _, e := range g.entries {
		if e.key.Scope.AdmissionID == d.Key.Scope.AdmissionID {
			if e.state.Pending {
				admActive++
			}
			if e.state.Reason != "" {
				failed++
			}
		}
	}
	if admQueued >= d.Limits.AdmissionWaiters {
		return Ticket{}, ErrQuota
	}
	e := g.entries[d.Key]
	owner := false
	if e != nil && !e.state.Pending && e.state.Reason != "" && !d.Now.Before(e.state.RetryAt) {
		delete(g.entries, d.Key)
		e = nil
	}
	if e == nil {
		window := d.Now.UnixNano() / int64(d.Limits.FillWindow)
		callerKey := accountKey{d.Key.Scope.AdmissionID, d.CallerID}
		admissionKey := accountKey{admission: d.Key.Scope.AdmissionID}
		cc, ac := g.windows[callerKey], g.windows[admissionKey]
		if cc.window != window {
			cc = fillCounter{window: window}
		}
		if ac.window != window {
			ac = fillCounter{window: window}
		}
		if cc.used >= d.Limits.CallerFillsPerWindow || ac.used >= d.Limits.AdmissionFillsPerWindow {
			return Ticket{}, ErrQuota
		}
		if g.active[accountKey{d.Key.Scope.AdmissionID, d.CallerID}] >= d.Limits.ActiveFills || admActive >= d.Limits.AdmissionFills {
			return Ticket{}, ErrQuota
		}
		if failed >= d.Limits.FailureEntries {
			return Ticket{}, ErrHeld
		}
		cc.used++
		ac.used++
		g.windows[callerKey] = cc
		g.windows[admissionKey] = ac
		g.serial++
		e = &gateEntry{key: d.Key, generation: fmt.Sprint(g.serial), ready: make(chan struct{}), state: State{Pending: true}, limits: d.Limits, owner: d.CallerID}
		g.entries[d.Key] = e
		g.active[accountKey{d.Key.Scope.AdmissionID, d.CallerID}]++
		owner = true
	}
	g.serial++
	id := fmt.Sprint(g.serial)
	g.waiters[id] = &gateWaiter{entry: e, caller: d.CallerID}
	e.waiters++
	g.queued[accountKey{d.Key.Scope.AdmissionID, d.CallerID}]++
	g.acquired <- struct{}{}
	return Ticket{ID: id, Generation: e.generation, Owner: owner, Joined: !owner && e.state.Pending}, nil
}
func (g *fakeGate) Wait(ctx context.Context, t Ticket) (State, error) {
	g.mu.Lock()
	w := g.waiters[t.ID]
	if w == nil || g.down {
		g.mu.Unlock()
		return State{}, ErrHeld
	}
	ready := w.entry.ready
	g.mu.Unlock()
	select {
	case <-ctx.Done():
		return State{}, ctx.Err()
	case <-ready:
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.entries[w.entry.key] != w.entry || w.entry.generation != t.Generation {
		return State{}, ErrStale
	}
	return w.entry.state, nil
}
func (g *fakeGate) Complete(ctx context.Context, t Ticket, s mirror.Store, a mirror.Admission, c Completion) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	var err error
	defer func() { g.completions <- err }()
	w := g.waiters[t.ID]
	if g.down || w == nil {
		err = ErrHeld
		return err
	}
	e := w.entry
	if !t.Owner || g.entries[e.key] != e || t.Generation != e.generation || !e.state.Pending {
		err = ErrStale
		return err
	}
	// The same test Store is the bound backend. Real R2 must enforce this binding
	// and use its actual SQL transaction, not this fake's mutex.
	result, applyErr := ApplyCompletion(ctx, s, a, c)
	e.state = State{}
	g.active[accountKey{e.key.Scope.AdmissionID, e.owner}]--
	if c.AdmissionDenied || errors.Is(applyErr, mirror.ErrPermission) {
		e.state = State{Reason: mirror.Unknown}
		e.expiry = c.Now
	} else if applyErr != nil || !result.Fulfilled {
		e.state.Reason = result.Reason
		if e.state.Reason == "" {
			e.state.Reason = mirror.Unknown
		}
		e.state.RetryAt = c.Now.Add(e.limits.Cooldown)
		e.expiry = c.Now.Add(e.limits.FailureTTL)
	}
	close(e.ready)
	err = applyErr
	return err
}
func (g *fakeGate) Leave(_ context.Context, t Ticket) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.leaveDown {
		return ErrHeld
	}
	if w := g.waiters[t.ID]; w != nil && !w.left {
		w.left = true
		w.entry.waiters--
		g.queued[accountKey{w.entry.key.Scope.AdmissionID, w.caller}]--
		if w.entry.waiters == 0 && !w.entry.state.Pending && w.entry.state.Reason == "" && g.entries[w.entry.key] == w.entry {
			delete(g.entries, w.entry.key)
		}
	}
	return nil
}
func (g *fakeGate) takeOver() {
	g.mu.Lock()
	defer g.mu.Unlock()
	for k, e := range g.entries {
		if e.state.Pending {
			delete(g.entries, k)
			g.active[accountKey{e.key.Scope.AdmissionID, e.owner}]--
			close(e.ready)
		}
	}
}

type fixture struct {
	clock  *testClock
	store  *mirrortest.ReferenceStore
	gate   *fakeGate
	source *testSource
	auth   *testAuthority
	a      *testAdmission
	caller Caller
	q      Query
	c      [2]*Coordinator
}

func setup(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{clock: &testClock{now: time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)}, store: mirrortest.NewStore(), gate: newGate(), auth: &testAuthority{current: map[accountKey]Caller{}}}
	f.a = &testAdmission{name: "admission-a"}
	f.a.allowed.Store(true)
	f.caller = Caller{ID: "caller-a", AdmissionID: f.a.name, AccessEpoch: "access-1", ConfigEpoch: "config-1", PolicyClass: "read", PolicyEpoch: "policy-1", Limits: Limits{CallerFillsPerWindow: 64, AdmissionFillsPerWindow: 128, FillWindow: time.Minute, MinFreshness: time.Second, MaxFreshness: time.Hour, ActiveFills: 1, QueuedCallers: 64, AdmissionFills: 2, AdmissionWaiters: 128, FailureEntries: 4, WaitTimeout: 2 * time.Second, FillTimeout: time.Second, Cooldown: time.Minute, FailureTTL: 2 * time.Minute, Refresh: mirror.RefreshPolicy{FullInventoryInterval: time.Hour}}}
	f.auth.set(f.caller)
	f.q = Query{Query: mirror.Query{Scope: mirror.Scope{Repository: mirror.RepositoryID{Provider: mirror.GitHub, Host: "forge.example", Path: "example/project"}, Kind: mirror.Issues, AdmissionID: f.a.name}, Now: f.clock.Now(), MaxAge: time.Minute}}
	f.source = &testSource{}
	f.source.fn = func(_ context.Context, _ mirror.Admission, s mirror.Scope, _ mirror.ScanPlan) (mirror.ScanResult, error) {
		return f.scan(s, "observed"), nil
	}
	life, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	for i := range f.c {
		var err error
		f.c[i], err = New(life, f.store, f.source, f.gate, f.clock, "issue-inventory", f.auth)
		if err != nil {
			t.Fatal(err)
		}
	}
	return f
}
func (f *fixture) scan(s mirror.Scope, title string) mirror.ScanResult {
	return mirror.ScanResult{Complete: true, ObservedUpTo: f.clock.Now(), Records: []mirror.Observation{{ID: mirror.RecordID{Repository: s.Repository, Kind: "issue", Number: 1}, Title: title, InInventory: true}}}
}
func (f *fixture) seed(t *testing.T) mirror.Snapshot {
	t.Helper()
	now := f.clock.Now()
	s, err := f.store.Apply(context.Background(), f.a, mirror.ScanUpdate{Scope: f.q.Scope, CompleteScan: true, FullInventory: true, Coverage: mirror.Coverage{State: mirror.Complete, LastAttempted: now, LastComplete: now, LastInventory: now, ObservedUpTo: now}, Upsert: f.scan(f.q.Scope, "prior").Records})
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func await(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatal("fixture synchronization timed out")
	}
}
func result(t *testing.T, ch <-chan Result) Result {
	t.Helper()
	select {
	case r := <-ch:
		return r
	case <-time.After(3 * time.Second):
		t.Fatal("result timed out")
		return Result{}
	}
}

// regression: #5399
func TestRefreshJoinedFlow(t *testing.T) {
	checkCompletionClock(t)
	for _, empty := range []bool{false, true} {
		t.Run(fmt.Sprint(empty), func(t *testing.T) {
			f := setup(t)
			if empty {
				f.source.fn = func(context.Context, mirror.Admission, mirror.Scope, mirror.ScanPlan) (mirror.ScanResult, error) {
					return mirror.ScanResult{Complete: true, ObservedUpTo: f.clock.Now()}, nil
				}
			}
			for i := range f.c {
				r, err := f.c[i].Read(context.Background(), f.a, f.q, f.caller)
				if err != nil || !r.Fulfilled || r.Stale || (len(r.Snapshot.Records) == 0) != empty {
					t.Fatalf("joined result: %+v %v", r, err)
				}
			}
			if f.source.calls.Load() != 1 {
				t.Fatal("fresh query caused another source walk")
			}
		})
	}
}

// regression: #5399
func TestRefreshSharedGate(t *testing.T) {
	f := setup(t)
	release := make(chan struct{})
	f.source.fn = func(ctx context.Context, _ mirror.Admission, s mirror.Scope, _ mirror.ScanPlan) (mirror.ScanResult, error) {
		select {
		case <-release:
			return f.scan(s, "shared"), nil
		case <-ctx.Done():
			return mirror.ScanResult{}, ctx.Err()
		}
	}
	out := make(chan Result, 50)
	for i := 0; i < 50; i++ {
		go func(i int) {
			q := f.q
			if i%2 == 1 {
				q.MaxAge = 2 * time.Minute
			}
			r, _ := f.c[i%2].Read(context.Background(), f.a, q, f.caller)
			out <- r
		}(i)
	}
	for i := 0; i < 50; i++ {
		await(t, f.gate.acquired)
	}
	close(release)
	for i := 0; i < 50; i++ {
		if !result(t, out).Fulfilled {
			t.Fatal("joined caller unfulfilled")
		}
	}
	if f.source.calls.Load() != 1 {
		t.Fatalf("two coordinators walked %d times", f.source.calls.Load())
	}
}

// regression: #5399
func TestRefreshFailureBackoff(t *testing.T) {
	checkPermissionRecovery(t)
	checkNotModifiedReviews(t)
	checkReviews(t)
	for _, partial := range []bool{false, true} {
		t.Run(fmt.Sprint(partial), func(t *testing.T) {
			f := setup(t)
			old := f.seed(t)
			f.clock.step(2 * time.Minute)
			f.source.fn = func(context.Context, mirror.Admission, mirror.Scope, mirror.ScanPlan) (mirror.ScanResult, error) {
				if partial {
					return mirror.ScanResult{Complete: false}, nil
				}
				return mirror.ScanResult{}, errors.New("synthetic source failure")
			}
			for i := range f.c {
				r, e := f.c[i].Read(context.Background(), f.a, f.q, f.caller)
				if e != nil || r.Fulfilled || !r.Stale || r.Reason == "" {
					t.Fatalf("failure became fulfilment: %+v %v", r, e)
				}
			}
			if f.source.calls.Load() != 1 {
				t.Fatal("failure cache is process-local")
			}
			s, _ := f.store.Read(context.Background(), f.a, f.q.Scope)
			if !s.Coverage.LastComplete.Equal(old.Coverage.LastComplete) || len(s.Records) != 1 {
				t.Fatal("failed fill advanced watermark or lost rows")
			}
			f.clock.step(time.Minute)
			f.source.fn = func(_ context.Context, _ mirror.Admission, s mirror.Scope, _ mirror.ScanPlan) (mirror.ScanResult, error) {
				return f.scan(s, "recovered"), nil
			}
			r, e := f.c[1].Read(context.Background(), f.a, f.q, f.caller)
			if e != nil || !r.Fulfilled || f.source.calls.Load() != 2 {
				t.Fatalf("bounded recovery: %+v %v", r, e)
			}
		})
	}
}

// regression: #5399
func TestRefreshScopeEpochs(t *testing.T) {
	checkActualFillKeys(t)
	f := setup(t)
	entered, release := make(chan struct{}), make(chan struct{})
	f.source.fn = func(ctx context.Context, _ mirror.Admission, s mirror.Scope, _ mirror.ScanPlan) (mirror.ScanResult, error) {
		close(entered)
		select {
		case <-release:
			return f.scan(s, "private-canary"), nil
		case <-ctx.Done():
			return mirror.ScanResult{}, ctx.Err()
		}
	}
	out := make(chan Result, 1)
	go func() { r, _ := f.c[0].Read(context.Background(), f.a, f.q, f.caller); out <- r }()
	await(t, entered)
	f.auth.revoke(f.caller.ID)
	close(release)
	r := result(t, out)
	if r.Fulfilled || len(r.Snapshot.Records) != 0 {
		t.Fatalf("revoked caller read private rows: %+v", r)
	}
	s, _ := f.store.Read(context.Background(), f.a, f.q.Scope)
	if len(s.Records) != 0 {
		t.Fatal("revoked fill published")
	}
	// All key epochs and admissions are compared independently, not presentation IDs.
	base := FillKey{Scope: f.q.Scope, SourceID: "issues", AccessEpoch: "a", ConfigEpoch: "c", PolicyClass: "r", PolicyEpoch: "p"}
	keys := []FillKey{base, base, base, base, base}
	keys[1].AccessEpoch = "b"
	keys[2].ConfigEpoch = "d"
	keys[3].PolicyEpoch = "q"
	keys[4].Scope.AdmissionID = "other"
	seen := map[FillKey]bool{}
	for _, k := range keys {
		if seen[k] {
			t.Fatal("epoch/admission collision")
		}
		seen[k] = true
	}
}

// regression: #5399
func TestRefreshFairBudgets(t *testing.T) {
	t.Run("whole-read-deadline", checkReadDeadline)
	checkWindowBudget(t)
	checkFailureCapacity(t)
	f := setup(t)
	f.caller.Limits.QueuedCallers = 1
	f.auth.set(f.caller)
	b := f.caller
	b.ID = "caller-b"
	f.auth.set(b)
	entered, release := make(chan struct{}), make(chan struct{})
	f.source.fn = func(ctx context.Context, _ mirror.Admission, s mirror.Scope, _ mirror.ScanPlan) (mirror.ScanResult, error) {
		if s.Repository.Path == f.q.Scope.Repository.Path {
			close(entered)
			select {
			case <-release:
			case <-ctx.Done():
				return mirror.ScanResult{}, ctx.Err()
			}
		}
		return f.scan(s, "fair"), nil
	}
	out := make(chan Result, 1)
	go func() { r, _ := f.c[0].Read(context.Background(), f.a, f.q, f.caller); out <- r }()
	await(t, entered)
	if _, e := f.c[1].Read(context.Background(), f.a, f.q, f.caller); !errors.Is(e, ErrQuota) {
		t.Fatalf("caller cap missing: %v", e)
	}
	q := f.q
	q.Scope.Repository.Path = "example/second"
	r, e := f.c[1].Read(context.Background(), f.a, q, b)
	if e != nil || !r.Fulfilled {
		t.Fatalf("A denied B reserved progress: %+v %v", r, e)
	}
	close(release)
	if !result(t, out).Fulfilled {
		t.Fatal("A lost its permitted fill")
	}
	f.gate.mu.Lock()
	defer f.gate.mu.Unlock()
	for _, e := range f.gate.entries {
		if e.state.Reason != "" {
			t.Fatal("quota denial poisoned shared failure state")
		}
	}
	for _, n := range f.gate.queued {
		if n != 0 {
			t.Fatal("waiter accounting leaked")
		}
	}
}

// regression: #5399
func TestRefreshWaiterCancel(t *testing.T) {
	f := setup(t)
	entered, release := make(chan struct{}), make(chan struct{})
	f.source.fn = func(ctx context.Context, _ mirror.Admission, s mirror.Scope, _ mirror.ScanPlan) (mirror.ScanResult, error) {
		close(entered)
		select {
		case <-release:
			return f.scan(s, "survived"), nil
		case <-ctx.Done():
			return mirror.ScanResult{}, ctx.Err()
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	first := make(chan error, 1)
	go func() { _, e := f.c[0].Read(ctx, f.a, f.q, f.caller); first <- e }()
	await(t, entered)
	await(t, f.gate.acquired)
	out := make(chan Result, 1)
	go func() { r, _ := f.c[1].Read(context.Background(), f.a, f.q, f.caller); out <- r }()
	await(t, f.gate.acquired)
	cancel()
	if e := <-first; !errors.Is(e, context.Canceled) {
		t.Fatalf("cancelled waiter: %v", e)
	}
	close(release)
	if !result(t, out).Fulfilled || f.source.calls.Load() != 1 {
		t.Fatal("one waiter's context cancelled the shared fill")
	}
}

// regression: #5399
func TestRefreshGateFence(t *testing.T) {
	checkUnknownCompletion(t)
	t.Run("missing", func(t *testing.T) {
		f := setup(t)
		if _, e := New(context.Background(), f.store, f.source, nil, f.clock, "issues", f.auth); !errors.Is(e, ErrHeld) {
			t.Fatal(e)
		}
		f.gate.down = true
		r, e := f.c[0].Read(context.Background(), f.a, f.q, f.caller)
		if e == nil || r.Fulfilled || f.source.calls.Load() != 0 {
			t.Fatal("gate outage used local source fallback")
		}
	})
	t.Run("takeover", func(t *testing.T) {
		f := setup(t)
		old := f.seed(t)
		f.clock.step(2 * time.Minute)
		a, b, ra, rb := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
		var calls atomic.Int64
		f.source.fn = func(ctx context.Context, _ mirror.Admission, s mirror.Scope, _ mirror.ScanPlan) (mirror.ScanResult, error) {
			n := calls.Add(1)
			wait := ra
			title := "old-owner"
			if n == 1 {
				close(a)
			} else {
				wait = rb
				title = "new-owner"
				close(b)
			}
			select {
			case <-wait:
				return f.scan(s, title), nil
			case <-ctx.Done():
				return mirror.ScanResult{}, ctx.Err()
			}
		}
		first := make(chan Result, 1)
		go func() { r, _ := f.c[0].Read(context.Background(), f.a, f.q, f.caller); first <- r }()
		await(t, a)
		f.gate.takeOver()
		second := make(chan Result, 1)
		go func() { r, _ := f.c[1].Read(context.Background(), f.a, f.q, f.caller); second <- r }()
		await(t, b)
		close(ra)
		select {
		case e := <-f.gate.completions:
			if !errors.Is(e, ErrStale) {
				t.Fatalf("old generation completed: %v", e)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("old completion missing")
		}
		snap, _ := f.store.Read(context.Background(), f.a, f.q.Scope)
		if snap.Version != old.Version || snap.Records[0].Title != "prior" {
			t.Fatal("old owner applied despite matching expected version")
		}
		close(rb)
		if !result(t, second).Fulfilled {
			t.Fatal("new owner failed")
		}
		_ = result(t, first)
	})
}

type revokeReadStore struct {
	mirror.Store
	once   sync.Once
	revoke func()
	armed  *atomic.Bool
}

func (s *revokeReadStore) Read(ctx context.Context, a mirror.Admission, scope mirror.Scope) (mirror.Snapshot, error) {
	r, e := s.Store.Read(ctx, a, scope)
	if s.armed == nil || s.armed.Load() {
		s.once.Do(s.revoke)
	}
	return r, e
}

type armAfterWait struct {
	Gate
	arm func()
}

func (g armAfterWait) Wait(ctx context.Context, t Ticket) (State, error) {
	s, e := g.Gate.Wait(ctx, t)
	g.arm()
	return s, e
}

// regression: #5399
func TestRefreshReleaseAuthority(t *testing.T) {
	for _, mode := range []string{"fresh-caller", "fresh-admission", "fresh-epoch", "joined", "completion"} {
		t.Run(mode, func(t *testing.T) {
			f := setup(t)
			var armed atomic.Bool
			revoke := func() { f.auth.revoke(f.caller.ID) }
			if mode == "fresh-admission" {
				revoke = func() { f.a.allowed.Store(false) }
			}
			if mode == "fresh-epoch" {
				revoke = func() { next := f.caller; next.AccessEpoch = "access-2"; f.auth.set(next) }
			}
			store := &revokeReadStore{Store: f.store, revoke: revoke}
			if mode == "joined" {
				store.armed = &armed
				f.c[0].gate = armAfterWait{Gate: f.gate, arm: func() { armed.Store(true) }}
			} else {
				f.seed(t)
			}
			f.c[0].store = store
			var r Result
			var e error
			if mode == "completion" {
				r, e = ApplyCompletion(context.Background(), store, f.a, Completion{Authority: f.auth, Caller: f.caller, Query: f.q, Now: f.clock.Now()})
			} else {
				r, e = f.c[0].Read(context.Background(), f.a, f.q, f.caller)
			}
			if e == nil || r.Fulfilled || len(r.Snapshot.Records) != 0 {
				t.Fatalf("post-read revocation released rows: %+v %v", r, e)
			}
			if mode != "joined" && f.source.calls.Load() != 0 {
				t.Fatal("fresh query attempted a source")
			}
		})
	}
}

func checkActualFillKeys(t *testing.T) {
	for _, mode := range []string{"access", "config", "policy-class", "policy-epoch", "admission", "source"} {
		t.Run(mode, func(t *testing.T) {
			f := setup(t)
			f.source.fn = func(context.Context, mirror.Admission, mirror.Scope, mirror.ScanPlan) (mirror.ScanResult, error) {
				return mirror.ScanResult{}, errors.New("synthetic cached failure")
			}
			r, e := f.c[0].Read(context.Background(), f.a, f.q, f.caller)
			if e != nil || r.Fulfilled {
				t.Fatalf("failure baseline: %+v %v", r, e)
			}
			switch mode {
			case "access":
				f.caller.AccessEpoch = "access-2"
			case "config":
				f.caller.ConfigEpoch = "config-2"
			case "policy-class":
				f.caller.PolicyClass = "other"
			case "policy-epoch":
				f.caller.PolicyEpoch = "policy-2"
			case "admission":
				f.a = &testAdmission{name: "admission-b"}
				f.a.allowed.Store(true)
				f.caller.AdmissionID = f.a.name
				f.q.Scope.AdmissionID = f.a.name
			case "source":
				f.c[1].sourceID = "other-population"
			}
			f.auth.set(f.caller)
			f.source.fn = func(_ context.Context, _ mirror.Admission, s mirror.Scope, _ mirror.ScanPlan) (mirror.ScanResult, error) {
				return f.scan(s, "isolated"), nil
			}
			r, e = f.c[1].Read(context.Background(), f.a, f.q, f.caller)
			if e != nil || !r.Fulfilled || f.source.calls.Load() != 2 {
				t.Fatalf("cached failure crossed actual %s key: %+v %v calls=%d", mode, r, e, f.source.calls.Load())
			}
		})
	}
}
func checkFailureCapacity(t *testing.T) {
	t.Run("bounded-failures", func(t *testing.T) {
		f := setup(t)
		f.caller.Limits.FailureEntries = 1
		f.auth.set(f.caller)
		f.source.fn = func(context.Context, mirror.Admission, mirror.Scope, mirror.ScanPlan) (mirror.ScanResult, error) {
			return mirror.ScanResult{}, errors.New("synthetic capacity failure")
		}
		r, e := f.c[0].Read(context.Background(), f.a, f.q, f.caller)
		if e != nil || r.Fulfilled {
			t.Fatal("failure baseline")
		}
		q := f.q
		q.Scope.Repository.Path = "example/second"
		r, e = f.c[1].Read(context.Background(), f.a, q, f.caller)
		if !errors.Is(e, ErrHeld) || r.Fulfilled || f.source.calls.Load() != 1 {
			t.Fatal("unexpired failure evicted into a fresh allowance")
		}
		f.clock.step(2 * time.Minute)
		_, _ = f.c[1].Read(context.Background(), f.a, q, f.caller)
		if f.source.calls.Load() != 2 {
			t.Fatal("expired failure never released bounded capacity")
		}
	})
}

// regression: #5399
func TestRefreshOwnerDeadline(t *testing.T) {
	f := setup(t)
	f.caller.Limits.FillTimeout = 100 * time.Millisecond
	f.auth.set(f.caller)
	stopped := make(chan struct{})
	f.source.fn = func(ctx context.Context, _ mirror.Admission, _ mirror.Scope, _ mirror.ScanPlan) (mirror.ScanResult, error) {
		<-ctx.Done()
		close(stopped)
		return mirror.ScanResult{}, ctx.Err()
	}
	r, e := f.c[0].Read(context.Background(), f.a, f.q, f.caller)
	if e != nil || r.Fulfilled || r.Reason == "" {
		t.Fatalf("unbounded owner result: %+v %v", r, e)
	}
	await(t, stopped)
	f.gate.mu.Lock()
	defer f.gate.mu.Unlock()
	if f.gate.active[accountKey{f.caller.AdmissionID, f.caller.ID}] != 0 || f.gate.queued[accountKey{f.caller.AdmissionID, f.caller.ID}] != 0 {
		t.Fatal("owner deadline leaked active fill or waiter")
	}
}

// regression: #5399
func TestRefreshAdmissionDenial(t *testing.T) {
	f := setup(t)
	b := f.caller
	b.ID = "caller-b"
	f.auth.set(b)
	f.source.fn = func(context.Context, mirror.Admission, mirror.Scope, mirror.ScanPlan) (mirror.ScanResult, error) {
		return mirror.ScanResult{}, mirror.ErrPermission
	}
	r, e := f.c[0].Read(context.Background(), f.a, f.q, f.caller)
	if e != nil || r.Fulfilled {
		t.Fatalf("admission refusal: %+v %v", r, e)
	}
	s, _ := f.store.Read(context.Background(), f.a, f.q.Scope)
	if s.Coverage.State != mirror.NeverRead {
		t.Fatal("admission refusal installed source coverage")
	}
	f.source.fn = func(_ context.Context, _ mirror.Admission, s mirror.Scope, _ mirror.ScanPlan) (mirror.ScanResult, error) {
		return f.scan(s, "eligible"), nil
	}
	r, e = f.c[1].Read(context.Background(), f.a, f.q, b)
	if e != nil || !r.Fulfilled || f.source.calls.Load() != 2 {
		t.Fatalf("caller denial poisoned eligible caller: %+v %v calls=%d", r, e, f.source.calls.Load())
	}
}

func checkWindowBudget(t *testing.T) {
	t.Run("window-budget", func(t *testing.T) {
		f := setup(t)
		f.caller.Limits.CallerFillsPerWindow = 1
		f.caller.Limits.AdmissionFillsPerWindow = 2
		f.auth.set(f.caller)
		b := f.caller
		b.ID = "caller-b"
		f.auth.set(b)
		if r, e := f.c[0].Read(context.Background(), f.a, f.q, f.caller); e != nil || !r.Fulfilled {
			t.Fatal("positive fill baseline")
		}
		q := f.q
		q.Scope.Repository.Path = "example/second"
		if _, e := f.c[1].Read(context.Background(), f.a, q, f.caller); !errors.Is(e, ErrQuota) {
			t.Fatalf("per-window allowance reset: %v", e)
		}
		if r, e := f.c[1].Read(context.Background(), f.a, q, b); e != nil || !r.Fulfilled {
			t.Fatalf("caller quota denied eligible B: %+v %v", r, e)
		}
		if f.source.calls.Load() != 2 {
			t.Fatal("budget denial reached source")
		}
		f.clock.step(time.Minute)
		q.Scope.Repository.Path = "example/third"
		if r, e := f.c[0].Read(context.Background(), f.a, q, f.caller); e != nil || !r.Fulfilled {
			t.Fatalf("bounded next window failed: %+v %v", r, e)
		}
		q.MaxAge = time.Nanosecond
		if _, e := f.c[0].Read(context.Background(), f.a, q, f.caller); !errors.Is(e, ErrPolicy) {
			t.Fatal("untrusted force-refresh freshness admitted")
		}
	})
}

type unknownCompleteGate struct {
	Gate
	committed bool
	done      chan struct{}
}

func (g unknownCompleteGate) Complete(ctx context.Context, t Ticket, s mirror.Store, a mirror.Admission, c Completion) error {
	if g.committed {
		_ = g.Gate.Complete(ctx, t, s, a, c)
	}
	close(g.done)
	return ErrHeld
}
func (g unknownCompleteGate) Wait(ctx context.Context, t Ticket) (State, error) {
	if g.committed {
		return g.Gate.Wait(ctx, t)
	}
	select {
	case <-g.done:
		return State{}, ErrHeld
	case <-ctx.Done():
		return State{}, ctx.Err()
	}
}
func checkUnknownCompletion(t *testing.T) {
	for _, committed := range []bool{false, true} {
		t.Run(fmt.Sprint(committed), func(t *testing.T) {
			f := setup(t)
			f.c[0].gate = unknownCompleteGate{Gate: f.gate, committed: committed, done: make(chan struct{})}
			r, e := f.c[0].Read(context.Background(), f.a, f.q, f.caller)
			if committed && (e != nil || !r.Fulfilled || len(r.Snapshot.Records) != 1) {
				t.Fatalf("committed unknown response not re-queried: %+v %v", r, e)
			}
			if !committed && (e == nil || r.Fulfilled) {
				t.Fatalf("unknown completion became success: %+v %v", r, e)
			}
			if f.source.calls.Load() != 1 {
				t.Fatal("unknown completion blindly replayed source")
			}
		})
	}
}
func checkReviews(t *testing.T) {
	t.Run("required-reviews", func(t *testing.T) {
		f := setup(t)
		f.q.Scope.Kind = mirror.Changes
		f.q.RequireReviews = true
		record := mirror.Observation{ID: mirror.RecordID{Repository: f.q.Scope.Repository, Kind: "pull-request", Number: 1}, Title: "old", Reviews: mirror.ReviewObservations{State: mirror.Complete}}
		now := f.clock.Now()
		old, e := f.store.Apply(context.Background(), f.a, mirror.ScanUpdate{Scope: f.q.Scope, CompleteScan: true, FullInventory: true, Coverage: mirror.Coverage{State: mirror.Complete, LastComplete: now, LastAttempted: now, LastInventory: now, ObservedUpTo: now}, Upsert: []mirror.Observation{record}})
		if e != nil {
			t.Fatal(e)
		}
		f.clock.step(2 * time.Minute)
		record.Reviews.State = mirror.Partial
		f.source.fn = func(context.Context, mirror.Admission, mirror.Scope, mirror.ScanPlan) (mirror.ScanResult, error) {
			return mirror.ScanResult{Complete: true, ObservedUpTo: f.clock.Now(), Records: []mirror.Observation{record}}, nil
		}
		r, e := f.c[0].Read(context.Background(), f.a, f.q, f.caller)
		s, _ := f.store.Read(context.Background(), f.a, f.q.Scope)
		if e != nil || r.Fulfilled || !s.Coverage.LastComplete.Equal(old.Coverage.LastComplete) {
			t.Fatalf("partial reviews advanced completeness: %+v %+v %v", r, s.Coverage, e)
		}
	})
}

type testObserver struct {
	mu     sync.Mutex
	events []Observation
}

func (o *testObserver) Observe(_ context.Context, e Observation) error {
	o.mu.Lock()
	o.events = append(o.events, e)
	o.mu.Unlock()
	return nil
}

// regression: #5399
func TestRefreshObservations(t *testing.T) {
	f := setup(t)
	o := &testObserver{}
	c, e := New(context.Background(), f.store, f.source, f.gate, f.clock, "issues", f.auth, o)
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		r, e := c.Read(context.Background(), f.a, f.q, f.caller)
		if e != nil || !r.Fulfilled {
			t.Fatal(e)
		}
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	scans, hits := 0, 0
	for _, e := range o.events {
		if !e.Known || e.Count != 1 {
			t.Fatal("unknown instrumentation reported measured zero")
		}
		switch e.Kind {
		case "source-attempt":
			if e.Unit != "scan-call" {
				t.Fatal("scan count became provider work")
			}
			scans++
		case "cache-hit":
			hits++
		}
	}
	if scans != 1 || hits != 1 {
		t.Fatalf("own work double-count: scans=%d hits=%d", scans, hits)
	}
}
func checkCompletionClock(t *testing.T) {
	t.Run("completion-clock", func(t *testing.T) {
		f := setup(t)
		start := f.clock.Now()
		f.source.fn = func(_ context.Context, _ mirror.Admission, s mirror.Scope, _ mirror.ScanPlan) (mirror.ScanResult, error) {
			f.clock.step(2 * time.Second)
			scan := f.scan(s, "after-start")
			scan.Records[0].UpdatedAt = start.Add(time.Second)
			scan.Records[0].ObservedAt = f.clock.Now()
			return scan, nil
		}
		r, e := f.c[0].Read(context.Background(), f.a, f.q, f.caller)
		if e != nil || !r.Fulfilled || !r.Snapshot.Coverage.LastComplete.Equal(f.clock.Now()) || !r.Snapshot.Records[0].ObservedAt.Equal(f.clock.Now()) {
			t.Fatalf("completion-clock facts rejected: %+v %v", r, e)
		}
	})
}

func checkNotModifiedReviews(t *testing.T) {
	t.Run("unfulfilled-not-modified", func(t *testing.T) {
		f := setup(t)
		f.q.Scope.Kind = mirror.Changes
		f.q.RequireReviews = true
		now := f.clock.Now()
		record := mirror.Observation{ID: mirror.RecordID{Repository: f.q.Scope.Repository, Kind: "pull-request", Number: 1}, Reviews: mirror.ReviewObservations{State: mirror.Partial}}
		_, e := f.store.Apply(context.Background(), f.a, mirror.ScanUpdate{Scope: f.q.Scope, CompleteScan: true, FullInventory: true, Coverage: mirror.Coverage{State: mirror.Complete, LastComplete: now, LastAttempted: now, LastInventory: now, ObservedUpTo: now, Validator: "validator", ValidatorSince: now}, Upsert: []mirror.Observation{record}})
		if e != nil {
			t.Fatal(e)
		}
		f.clock.step(2 * time.Minute)
		f.source.fn = func(_ context.Context, _ mirror.Admission, _ mirror.Scope, p mirror.ScanPlan) (mirror.ScanResult, error) {
			if p.Validator != "validator" {
				t.Error("positive conditional plan missing")
			}
			return mirror.ScanResult{Complete: true, NotModified: true, ConditionalSupported: true, Validator: p.Validator}, nil
		}
		r, e := f.c[0].Read(context.Background(), f.a, f.q, f.caller)
		s, _ := f.store.Read(context.Background(), f.a, f.q.Scope)
		if e != nil || r.Fulfilled || !s.Coverage.LastComplete.Equal(now) {
			t.Fatalf("unfulfilled not-modified advanced watermark: %+v %+v %v", r, s.Coverage, e)
		}
	})
}
func checkPermissionRecovery(t *testing.T) {
	t.Run("permission-recovery", func(t *testing.T) {
		f := setup(t)
		f.seed(t)
		f.clock.step(2 * time.Minute)
		f.source.fn = func(context.Context, mirror.Admission, mirror.Scope, mirror.ScanPlan) (mirror.ScanResult, error) {
			return mirror.ScanResult{Failure: mirror.Permission}, nil
		}
		r, e := f.c[0].Read(context.Background(), f.a, f.q, f.caller)
		if e != nil || r.Fulfilled || len(r.Snapshot.Records) != 0 || r.Reason != mirror.Permission {
			t.Fatalf("permission hold exposed rows: %+v %v", r, e)
		}
		f.clock.step(time.Minute)
		f.source.fn = func(_ context.Context, _ mirror.Admission, s mirror.Scope, p mirror.ScanPlan) (mirror.ScanResult, error) {
			if !p.FullInventory {
				t.Error("permission recovery was not full inventory")
			}
			return f.scan(s, "readmitted"), nil
		}
		r, e = f.c[1].Read(context.Background(), f.a, f.q, f.caller)
		if e != nil || !r.Fulfilled || f.source.calls.Load() != 2 {
			t.Fatalf("current admission could not recover through kernel: %+v %v", r, e)
		}
	})
}

type blockingReadStore struct{ mirror.Store }

func (s blockingReadStore) Read(ctx context.Context, _ mirror.Admission, _ mirror.Scope) (mirror.Snapshot, error) {
	<-ctx.Done()
	return mirror.Snapshot{}, ctx.Err()
}

// regression: #5399
func checkReadDeadline(t *testing.T) {
	f := setup(t)
	f.caller.Limits.WaitTimeout = 50 * time.Millisecond
	f.auth.set(f.caller)
	f.c[0].store = blockingReadStore{Store: f.store}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := make(chan error, 1)
	go func() { _, e := f.c[0].Read(ctx, f.a, f.q, f.caller); out <- e }()
	select {
	case e := <-out:
		if !errors.Is(e, context.DeadlineExceeded) {
			t.Fatalf("blocking read deadline: %v", e)
		}
	case <-time.After(500 * time.Millisecond):
		cancel()
		<-out
		t.Fatal("initial store read escaped the finite caller deadline")
	}
	if f.source.calls.Load() != 0 {
		t.Fatal("store outage fell back to provider")
	}
}

// regression: #5399
func TestRefreshReadDeadline(t *testing.T) { checkReadDeadline(t) }
