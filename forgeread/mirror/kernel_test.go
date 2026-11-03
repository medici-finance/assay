package mirror_test

import (
	"context"
	"errors"
	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/mirror/mirrortest"
	"testing"
	"time"
)

var instant = time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)

func fixture() (mirror.Scope, *mirrortest.TestAdmission, mirror.Observation) {
	repo := mirror.RepositoryID{Provider: mirror.GitHub, Host: "forge.example", Path: "example/project"}
	a := &mirrortest.TestAdmission{Name: "admission-a", Allowed: true}
	scope := mirror.Scope{Repository: repo, Kind: mirror.Issues, AdmissionID: a.Name}
	row := mirror.Observation{ID: mirror.RecordID{Repository: repo, Kind: "issue", Number: 1}, State: "open", Title: "canary", Labels: []string{"retained"}, ObservedAt: instant, UpdatedAt: instant, InInventory: true}
	return scope, a, row
}
func blank(scope mirror.Scope) mirror.Snapshot {
	return mirror.Snapshot{Scope: scope, Coverage: mirror.Coverage{State: mirror.NeverRead}}
}
func policy() mirror.RefreshPolicy {
	return mirror.RefreshPolicy{Overlap: time.Minute, FullInventoryInterval: time.Hour, FailureBackoff: 5 * time.Minute}
}
func seed(t *testing.T, rows bool) (*mirrortest.ReferenceStore, mirror.Scope, *mirrortest.TestAdmission, mirror.Snapshot) {
	t.Helper()
	scope, a, row := fixture()
	s := blank(scope)
	p, err := mirror.PlanRefresh(s, instant, policy())
	if err != nil {
		t.Fatal(err)
	}
	scan := mirror.ScanResult{Complete: true, ObservedUpTo: instant, Revision: "revision-1", Validator: "validator-1"}
	if rows {
		scan.Records = []mirror.Observation{row}
	}
	u, err := mirror.PlanScan(s, p, scan, instant)
	if err != nil {
		t.Fatal(err)
	}
	store := mirrortest.NewStore()
	s, err = store.Apply(context.Background(), a, u)
	if err != nil {
		t.Fatal(err)
	}
	return store, scope, a, s
}

type spyStore struct {
	mirror.Store
	reads    int
	override *mirror.Snapshot
	failure  error
}

func (s *spyStore) Read(ctx context.Context, a mirror.Admission, scope mirror.Scope) (mirror.Snapshot, error) {
	s.reads++
	if s.failure != nil {
		return mirror.Snapshot{}, s.failure
	}
	if s.override != nil {
		return *s.override, nil
	}
	return s.Store.Read(ctx, a, scope)
}

// regression: forge-neutral/46
func TestMirrorKernelAdmission(t *testing.T) {
	store, scope, a, s := seed(t, true)
	ctx := context.Background()
	q := mirror.Query{Scope: scope, Now: instant, MaxAge: time.Hour}
	spy := &spyStore{Store: store}
	good, err := mirror.QueryScope(ctx, spy, a, q)
	if err != nil || !good.Available || good.Snapshot.Records[0].Title != "canary" {
		t.Fatalf("positive query: %+v %v", good, err)
	}
	var typedNil *mirrortest.TestAdmission
	for _, bad := range []mirror.Admission{nil, typedNil, &mirrortest.TestAdmission{Name: "", Allowed: true}, &mirrortest.TestAdmission{Name: "admission-b", Allowed: true}, &mirrortest.TestAdmission{Name: a.Name, Allowed: false}} {
		before := spy.reads
		r, err := mirror.QueryScope(ctx, spy, bad, q)
		if err != nil || r.Available || r.Snapshot.Coverage.Reason != mirror.Permission || len(r.Snapshot.Records) != 0 || spy.reads != before {
			t.Fatalf("library queried wrong admission: %+v %v reads=%d", r, err, spy.reads-before)
		}
	}
	permissive := &spyStore{override: &s}
	r, err := mirror.QueryScope(ctx, permissive, &mirrortest.TestAdmission{Name: "admission-b", Allowed: true}, q)
	if err != nil || r.Available || permissive.reads != 0 {
		t.Fatal("permissive store bypassed library")
	}
	foreign := s
	foreign.Scope.AdmissionID = "admission-b"
	permissive.override = &foreign
	if _, err := mirror.QueryScope(ctx, permissive, a, q); !errors.Is(err, mirror.ErrInvalid) {
		t.Fatalf("wrong returned scope accepted: %v", err)
	}
	mirrortest.Check(t, func() mirror.Store { return mirrortest.NewStore() })
	a.Allowed = false
	r, err = mirror.QueryScope(ctx, store, a, q)
	if err != nil || r.Available || len(r.Snapshot.Records) > 0 {
		t.Fatal("revocation ignored")
	}
}

// regression: forge-neutral/46
func TestMirrorKernelCoverage(t *testing.T) {
	checkPermissionHold(t)
	store, scope, a, s := seed(t, true)
	ctx := context.Background()
	q := mirror.Query{Scope: scope, Now: instant, MaxAge: time.Hour}
	for _, state := range []mirror.CoverageState{mirror.NeverRead, mirror.Partial, mirror.Unavailable} {
		snap := s
		snap.Coverage.State = state
		if state == mirror.NeverRead {
			snap = blank(scope)
		}
		if state == mirror.Unavailable {
			snap.Coverage.Reason = mirror.Transport
		}
		r, err := mirror.QueryScope(ctx, &spyStore{override: &snap}, a, q)
		if err != nil || r.Available || !r.Stale {
			t.Fatalf("%s became present: %+v %v", state, r, err)
		}
	}
	emptyStore, _, _, _ := seed(t, false)
	r, err := mirror.QueryScope(ctx, emptyStore, a, q)
	if err != nil || !r.Available || r.Snapshot.Records == nil || len(r.Snapshot.Records) != 0 {
		t.Fatalf("known empty missing: %+v %v", r, err)
	}
	q.Now = instant.Add(2 * time.Hour)
	r, err = mirror.QueryScope(ctx, store, a, q)
	if err != nil || r.Available || !r.Stale || len(r.Snapshot.Records) != 1 {
		t.Fatalf("stale not explicit: %+v %v", r, err)
	}
	for _, failure := range []mirror.Reason{"", mirror.Transport, mirror.RateLimited} {
		p, _ := mirror.PlanRefresh(s, instant.Add(time.Minute), policy())
		u, err := mirror.PlanScan(s, p, mirror.ScanResult{Failure: failure}, instant.Add(time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		if !u.Coverage.LastComplete.Equal(s.Coverage.LastComplete) || u.Coverage.State == mirror.Complete || len(u.NotInInventory) > 0 {
			t.Fatalf("partial/failed erased facts: %+v", u)
		}
		next, err := store.Apply(ctx, a, u)
		if err != nil || len(next.Records) != 1 || !next.Records[0].InInventory {
			t.Fatal("unseen row lost", err)
		}
		store, _, _, s = seed(t, true)
	}
	p, _ := mirror.PlanRefresh(s, instant.Add(time.Minute), policy())
	u, err := mirror.PlanScan(s, p, mirror.ScanResult{Failure: mirror.Permission}, instant.Add(time.Minute))
	if err != nil || !u.RetentionDecisionRequired || len(u.Upsert) > 0 || len(u.NotInInventory) > 0 || u.Coverage.Reason != mirror.Permission {
		t.Fatalf("permission selected retention: %+v %v", u, err)
	}
	if _, err := store.Apply(ctx, a, u); err != nil {
		t.Fatal(err)
	}
	q.Now = instant.Add(time.Minute)
	r, err = mirror.QueryScope(ctx, store, a, q)
	if err != nil || len(r.Snapshot.Records) > 0 || r.Snapshot.Coverage.Reason != mirror.Permission {
		t.Fatal("permission-lost content served")
	}
	_, _, row := fixture()
	foreign := row
	foreign.ID.Repository.Path = "another/project"
	for _, records := range [][]mirror.Observation{{foreign}, {row, row}} {
		if _, err := mirror.PlanScan(s, p, mirror.ScanResult{Complete: true, Records: records}, instant.Add(time.Minute)); !errors.Is(err, mirror.ErrInvalid) {
			t.Fatalf("foreign/duplicate accepted: %v", err)
		}
	}
	if _, err := mirror.PlanScan(s, p, mirror.ScanResult{Complete: true, ObservedUpTo: instant.Add(-time.Minute)}, instant.Add(time.Minute)); !errors.Is(err, mirror.ErrInvalid) {
		t.Fatal("watermark regression accepted")
	}
	older := row
	older.UpdatedAt = instant.Add(-time.Minute)
	older.ObservedAt = instant.Add(time.Minute)
	if _, err := mirror.PlanScan(s, p, mirror.ScanResult{Complete: true, Records: []mirror.Observation{older}}, instant.Add(time.Minute)); !errors.Is(err, mirror.ErrInvalid) {
		t.Fatal("older record overwrote newer observation")
	}
	bad := s
	bad.Coverage.LastComplete = instant.Add(time.Hour)
	if _, err := mirror.QueryScope(ctx, &spyStore{override: &bad}, a, mirror.Query{Scope: scope, Now: instant, MaxAge: time.Hour}); err == nil {
		t.Fatal("future timestamp accepted")
	}
}

// regression: forge-neutral/46
func TestMirrorKernelScanPlanning(t *testing.T) {
	scope, _, _ := fixture()
	p, err := mirror.PlanRefresh(blank(scope), instant, policy())
	if err != nil || !p.FullInventory || !p.Since.IsZero() {
		t.Fatalf("initial plan %+v %v", p, err)
	}
	_, _, _, s := seed(t, true)
	p, err = mirror.PlanRefresh(s, instant.Add(time.Minute), policy())
	if err != nil || p.FullInventory || !p.Since.Equal(instant.Add(-time.Minute)) {
		t.Fatalf("overlap plan %+v %v", p, err)
	}
	full, err := mirror.PlanRefresh(s, instant.Add(2*time.Hour), policy())
	if err != nil || !full.FullInventory {
		t.Fatal("periodic repair missing")
	}
	u, err := mirror.PlanScan(s, full, mirror.ScanResult{Complete: true, ObservedUpTo: instant.Add(2 * time.Hour)}, instant.Add(2*time.Hour))
	if err != nil || len(u.NotInInventory) != 1 || len(u.Upsert) != 0 {
		t.Fatalf("full absence %+v %v", u, err)
	}
	for _, complete := range []bool{false, true} {
		u, err := mirror.PlanScan(s, p, mirror.ScanResult{Complete: complete}, instant.Add(time.Minute))
		if err != nil || len(u.NotInInventory) > 0 {
			t.Fatal("incremental absence inferred deletion")
		}
	}
	partial := s
	partial.Coverage.State = mirror.Partial
	partial.Coverage.LastAttempted = instant.Add(time.Minute)
	backoff, err := mirror.PlanRefresh(partial, instant.Add(2*time.Minute), policy())
	if err != nil || !backoff.NotBefore.Equal(instant.Add(6*time.Minute)) {
		t.Fatalf("backoff %+v %v", backoff, err)
	}
	if _, err := mirror.PlanScan(partial, backoff, mirror.ScanResult{Complete: true}, instant.Add(2*time.Minute)); err == nil {
		t.Fatal("backoff ignored")
	}
	nm := mirror.ScanResult{Complete: true, NotModified: true, ConditionalSupported: true, Revision: "revision-1", Validator: "validator-1"}
	conditional := p
	conditional.FullInventory = true
	conditional.Since = time.Time{}
	conditional.Validator = "validator-1"
	if _, err := mirror.PlanScan(partial, conditional, nm, instant.Add(7*time.Minute)); err == nil {
		t.Fatal("not-modified promoted partial")
	}
	good, err := mirror.PlanScan(s, conditional, nm, instant.Add(time.Minute))
	if err != nil || !good.CompleteScan || !good.Coverage.LastComplete.Equal(instant.Add(time.Minute)) {
		t.Fatalf("conditional %+v %v", good, err)
	}
	changed := conditional
	changed.Since = instant.Add(-time.Minute)
	if _, err := mirror.PlanScan(s, changed, nm, instant.Add(time.Minute)); err == nil {
		t.Fatal("validator reused for different scan window")
	}
	changed = conditional
	changed.FullInventory = false
	if _, err := mirror.PlanScan(s, changed, nm, instant.Add(time.Minute)); err == nil {
		t.Fatal("validator reused across inventory modes")
	}
	if p.Validator != "" {
		t.Fatal("incremental plan inherited full-inventory validator")
	}
	incrementalBaseline := s
	incrementalBaseline.Coverage.ValidatorFullInventory = false
	incrementalBaseline.Coverage.ValidatorSince = instant.Add(-2 * time.Minute)
	incrementalPlan := conditional
	incrementalPlan.FullInventory = false
	incrementalPlan.Since = instant.Add(-2 * time.Minute)
	if _, err := mirror.PlanScan(incrementalBaseline, incrementalPlan, nm, instant.Add(time.Minute)); err != nil {
		t.Fatal("matching incremental validator rejected", err)
	}
	incrementalPlan.Since = instant.Add(-time.Minute)
	if _, err := mirror.PlanScan(incrementalBaseline, incrementalPlan, nm, instant.Add(time.Minute)); err == nil {
		t.Fatal("validator reused for changed incremental bound")
	}
	recent := s
	recent.Coverage.LastAttempted = instant.Add(2 * time.Hour)
	recent.Coverage.LastComplete = recent.Coverage.LastAttempted
	periodic, err := mirror.PlanRefresh(recent, instant.Add(2*time.Hour), policy())
	if err != nil || !periodic.FullInventory {
		t.Fatal("incrementals suppressed inventory")
	}
	store, _, a, s := seed(t, true)
	full, _ = mirror.PlanRefresh(s, instant.Add(2*time.Hour), policy())
	u, err = mirror.PlanScan(s, full, mirror.ScanResult{Complete: true}, instant.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	next, err := store.Apply(context.Background(), a, u)
	if err != nil || next.Records[0].InInventory || next.Records[0].State != "open" {
		t.Fatal("absence became closure or deletion")
	}
}

// regression: forge-neutral/46
func TestMirrorStoreConformance(t *testing.T) {
	mirrortest.Check(t, func() mirror.Store { return mirrortest.NewStore() })
}

// regression: forge-neutral/46
func TestMirrorKernelFlow(t *testing.T) {
	store, scope, a, s := seed(t, true)
	ctx := context.Background()
	q := mirror.Query{Scope: scope, Now: instant, MaxAge: time.Hour}
	s.Records[0].Title = "caller changed"
	s.Records[0].Labels[0] = "caller changed"
	first, err := mirror.QueryScope(ctx, store, a, q)
	if err != nil || !first.Available || first.Snapshot.Records[0].Title != "canary" {
		t.Fatalf("source clone %+v %v", first, err)
	}
	first.Snapshot.Records[0].Labels[0] = "consumer changed"
	again, err := mirror.QueryScope(ctx, store, a, q)
	if err != nil || again.Snapshot.Records[0].Labels[0] != "retained" {
		t.Fatal("caller mutated stored data")
	}
	other := scope
	other.Repository.Path = "example/empty"
	initial, err := store.Read(ctx, a, other)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := mirror.PlanRefresh(initial, instant, policy())
	if err != nil {
		t.Fatal(err)
	}
	u, err := mirror.PlanScan(initial, plan, mirror.ScanResult{Complete: true}, instant)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Apply(ctx, a, u); err != nil {
		t.Fatal(err)
	}
	q.Scope = other
	empty, err := mirror.QueryScope(ctx, store, a, q)
	if err != nil || !empty.Available || empty.Snapshot.Records == nil || len(empty.Snapshot.Records) > 0 {
		t.Fatal("external empty flow failed")
	}
	nested := scope.Repository
	nested.Provider = mirror.GitLab
	nested.Path = "example/group/nested/project"
	if err := nested.Validate(); err != nil {
		t.Fatal(err)
	}
}

// regression: sec-permission-loss-not-sticky
func TestMirrorPermissionHold(t *testing.T) { checkPermissionHold(t) }

func checkPermissionHold(t *testing.T) {
	t.Helper()
	for _, failure := range []mirror.Reason{mirror.Transport, mirror.RateLimited, mirror.Unknown, mirror.Truncated, ""} {
		name := string(failure)
		if name == "" {
			name = "incomplete"
		}
		t.Run(name, func(t *testing.T) {
			store, scope, a, s := seed(t, true)
			ctx := context.Background()
			lossTime := instant.Add(time.Minute)
			plan, err := mirror.PlanRefresh(s, lossTime, policy())
			if err != nil {
				t.Fatal(err)
			}
			loss, err := mirror.PlanScan(s, plan, mirror.ScanResult{Failure: mirror.Permission}, lossTime)
			if err != nil {
				t.Fatal(err)
			}
			held, err := store.Apply(ctx, a, loss)
			if err != nil {
				t.Fatal(err)
			}
			assertHidden := func(now time.Time) {
				t.Helper()
				r, err := mirror.QueryScope(ctx, store, a, mirror.Query{Scope: scope, Now: now, MaxAge: time.Hour})
				if err != nil || r.Available || len(r.Snapshot.Records) != 0 || r.Snapshot.Coverage.Reason != mirror.Permission {
					t.Fatalf("permission hold disclosed prior rows: %+v %v", r, err)
				}
			}
			assertHidden(lossTime)
			attemptTime := instant.Add(7 * time.Minute)
			plan, err = mirror.PlanRefresh(held, attemptTime, policy())
			if err != nil {
				t.Fatal(err)
			}
			_, _, partialRow := fixture()
			partialRow.ID.Number = 2
			partialRow.ObservedAt = attemptTime
			partialRow.UpdatedAt = attemptTime
			incomplete, err := mirror.PlanScan(held, plan, mirror.ScanResult{Failure: failure, Records: []mirror.Observation{partialRow}}, attemptTime)
			if err != nil {
				t.Fatal(err)
			}
			if incomplete.Coverage.State != mirror.Unavailable || incomplete.Coverage.Reason != mirror.Permission || !incomplete.RetentionDecisionRequired || len(incomplete.Upsert) != 0 || len(incomplete.NotInInventory) != 0 {
				t.Fatalf("non-complete scan cleared permission hold: %+v", incomplete)
			}
			held, err = store.Apply(ctx, a, incomplete)
			if err != nil {
				t.Fatal(err)
			}
			assertHidden(attemptTime)
			recoveryTime := instant.Add(13 * time.Minute)
			plan, err = mirror.PlanRefresh(held, recoveryTime, policy())
			if err != nil {
				t.Fatal(err)
			}
			if !plan.FullInventory || !plan.Since.IsZero() {
				t.Fatal("permission recovery did not request full inventory")
			}
			incremental := plan
			incremental.FullInventory = false
			incremental.Since = instant
			if _, err := mirror.PlanScan(held, incremental, mirror.ScanResult{Complete: true}, recoveryTime); err == nil {
				t.Fatal("incremental observation cleared permission hold")
			}
			// Bypass backoff planning to isolate the strict recovery watermark.
			replay := plan
			replay.NotBefore = time.Time{}
			if _, err := mirror.PlanScan(held, replay, mirror.ScanResult{Complete: true}, held.Coverage.LastAttempted); err == nil {
				t.Fatal("same-attempt observation cleared permission hold")
			}
			// An unchanged validator is not a complete observation from an unavailable baseline.
			if _, err := mirror.PlanScan(held, plan, mirror.ScanResult{Complete: true, NotModified: true, ConditionalSupported: true, Validator: "validator-1", Revision: "revision-1"}, recoveryTime); err == nil {
				t.Fatal("not-modified cleared permission hold")
			}
			_, _, row := fixture()
			row.ObservedAt = recoveryTime
			recovered, err := mirror.PlanScan(held, plan, mirror.ScanResult{Complete: true, Records: []mirror.Observation{row}, ObservedUpTo: recoveryTime, Revision: "recovered"}, recoveryTime)
			if err != nil {
				t.Fatal(err)
			}
			if recovered.RetentionDecisionRequired || recovered.Coverage.Reason != "" || !recovered.CompleteScan || !recovered.FullInventory || !recovered.Coverage.LastComplete.After(held.Coverage.LastAttempted) || !recovered.Coverage.LastInventory.After(held.Coverage.LastAttempted) {
				t.Fatalf("complete recovery did not clear hold: %+v", recovered)
			}
			if _, err := store.Apply(ctx, a, recovered); err != nil {
				t.Fatal(err)
			}
			r, err := mirror.QueryScope(ctx, store, a, mirror.Query{Scope: scope, Now: recoveryTime, MaxAge: time.Hour})
			if err != nil || !r.Available || len(r.Snapshot.Records) != 1 || r.Snapshot.Records[0].Title != "canary" {
				t.Fatalf("complete recovery failed: %+v %v", r, err)
			}
		})
	}
}
