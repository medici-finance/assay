package mirrortest

import (
	"context"
	"errors"
	"github.com/medici-finance/assay/forgeread/mirror"
	"reflect"
	"testing"
	"time"
)

// TestAdmission is a synthetic, mutable fixture capability; never use it to admit
// a real caller or deserialize it from a request.
type TestAdmission struct {
	Name    string
	Allowed bool
}

func (a *TestAdmission) ID() string {
	if a == nil {
		return ""
	}
	return a.Name
}
func (a *TestAdmission) Authorize(context.Context, mirror.RepositoryID) error {
	if a == nil || !a.Allowed {
		return mirror.ErrPermission
	}
	return nil
}

// Check exercises the Store directly, bypassing QueryScope and PlanScan. A later
// durable adapter can reuse these controls; this asserts no SQL/restart behavior.
func Check(t *testing.T, newStore func() mirror.Store) {
	CheckPermissionHold(t, newStore)
	CheckHoldRecovery(t, newStore)
	CheckPartialAbsence(t, newStore)
	t.Helper()
	ctx := context.Background()
	store := newStore()
	a := &TestAdmission{Name: "admission-a", Allowed: true}
	repo := mirror.RepositoryID{Provider: mirror.GitHub, Host: "forge.example", Path: "example/project"}
	scope := mirror.Scope{Repository: repo, Kind: mirror.Issues, AdmissionID: a.Name}
	now := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	first := mirror.ScanUpdate{Scope: scope, Coverage: mirror.Coverage{State: mirror.Complete, LastAttempted: now, LastComplete: now, LastInventory: now}, CompleteScan: true, FullInventory: true, Upsert: []mirror.Observation{{ID: mirror.RecordID{Repository: repo, Kind: "issue", Number: 1}, Title: "canary", InInventory: true}}}
	snap, err := store.Apply(ctx, a, first)
	if err != nil || len(snap.Records) != 1 {
		t.Fatalf("authorized complete update: %+v %v", snap, err)
	}
	for _, bad := range []mirror.Admission{nil, &TestAdmission{Name: "", Allowed: true}, &TestAdmission{Name: "admission-b", Allowed: true}, &TestAdmission{Name: a.Name, Allowed: false}} {
		if _, err := store.Read(ctx, bad, scope); !errors.Is(err, mirror.ErrPermission) {
			t.Fatalf("wrong/revoked direct read admitted: %v", err)
		}
		if _, err := store.Apply(ctx, bad, mirror.ScanUpdate{Scope: scope, ExpectedVersion: snap.Version, Coverage: snap.Coverage}); !errors.Is(err, mirror.ErrPermission) {
			t.Fatalf("wrong/revoked direct apply admitted: %v", err)
		}
	}
	bad := mirror.ScanUpdate{Scope: scope, ExpectedVersion: snap.Version, Coverage: snap.Coverage}
	bad.Coverage.LastAttempted = now.Add(time.Minute)
	bad.Coverage.LastComplete = bad.Coverage.LastAttempted
	if _, err := store.Apply(ctx, a, bad); !errors.Is(err, mirror.ErrInvalid) {
		t.Fatalf("incomplete watermark advanced: %v", err)
	}
	conflict := first
	conflict.Upsert[0].Title = "changed"
	if _, err := store.Apply(ctx, a, conflict); !errors.Is(err, mirror.ErrConflict) {
		t.Fatalf("stale update accepted: %v", err)
	}
	unchanged, err := store.Read(ctx, a, scope)
	if err != nil || unchanged.Version != snap.Version || unchanged.Records[0].Title != "canary" || !unchanged.Coverage.LastComplete.Equal(now) {
		t.Fatalf("refused update changed snapshot: %+v %v", unchanged, err)
	}
	a.Allowed = false
	if _, err := store.Read(ctx, a, scope); !errors.Is(err, mirror.ErrPermission) {
		t.Fatalf("withdrawal ignored: %v", err)
	}
}

// CheckPartialAbsence requires full completed inventory proof even when an ID
// has no row or is already absent, so SQL no-ops cannot acknowledge bad updates.
func CheckPartialAbsence(t *testing.T, newStore func() mirror.Store) {
	t.Helper()
	ctx := context.Background()
	store := newStore()
	repo := mirror.RepositoryID{Provider: mirror.GitHub, Host: "forge.example", Path: "example/project"}
	now := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	present := mirror.RecordID{Repository: repo, Kind: "issue", Number: 1}
	absent := mirror.RecordID{Repository: repo, Kind: "issue", Number: 2}
	for _, mode := range []struct {
		name           string
		complete, full bool
	}{{"partial-inventory", false, true}, {"incremental", true, false}, {"partial-incremental", false, false}} {
		for name, id := range map[string]mirror.RecordID{"present": present, "already-absent": absent,
			"missing": {Repository: repo, Kind: "issue", Number: 999}} {
			t.Run(mode.name+"/"+name, func(t *testing.T) {
				a := &TestAdmission{Name: "absence-" + mode.name + "-" + name, Allowed: true}
				scope := mirror.Scope{Repository: repo, Kind: mirror.Issues, AdmissionID: a.Name}
				initial := mirror.ScanUpdate{Scope: scope, CompleteScan: true, FullInventory: true,
					Coverage: mirror.Coverage{State: mirror.Complete, LastAttempted: now, LastComplete: now, LastInventory: now},
					Upsert:   []mirror.Observation{{ID: present, Title: "present", InInventory: true}, {ID: absent, Title: "absent", InInventory: true}}}
				snap, err := store.Apply(ctx, a, initial)
				if err != nil {
					t.Fatal(err)
				}
				full := mirror.ScanUpdate{Scope: scope, ExpectedVersion: snap.Version, Coverage: snap.Coverage,
					CompleteScan: true, FullInventory: true, NotInInventory: []mirror.RecordID{absent}}
				before, err := store.Apply(ctx, a, full)
				if err != nil {
					t.Fatalf("complete inventory absence refused: %v", err)
				}
				bad := mirror.ScanUpdate{Scope: scope, ExpectedVersion: before.Version, Coverage: before.Coverage,
					CompleteScan: mode.complete, FullInventory: mode.full, NotInInventory: []mirror.RecordID{id}}
				bad.Coverage.State, bad.Coverage.Reason = mirror.Partial, mirror.Truncated
				bad.Coverage.LastAttempted = now.Add(time.Minute)
				if err := bad.Validate(); err != nil {
					t.Fatalf("control must reach independent Store check: %v", err)
				}
				if _, err := store.Apply(ctx, a, bad); !errors.Is(err, mirror.ErrInvalid) {
					t.Fatalf("absence without complete inventory accepted: %v", err)
				}
				after, err := store.Read(ctx, a, scope)
				if err != nil || !reflect.DeepEqual(before, after) {
					t.Fatalf("refused absence changed rows, coverage or version: %v", err)
				}
			})
		}
	}
}

// CheckPermissionHold bypasses PlanScan: failed/partial updates cannot clear a
// persisted permission hold even when they pass the structural update validator.
func CheckPermissionHold(t *testing.T, newStore func() mirror.Store) {
	t.Helper()
	for _, failure := range []mirror.Reason{mirror.Transport, mirror.RateLimited, mirror.Unknown, mirror.Truncated} {
		t.Run("permission-hold-"+string(failure), func(t *testing.T) {
			ctx := context.Background()
			store := newStore()
			a := &TestAdmission{Name: "admission-a", Allowed: true}
			repo := mirror.RepositoryID{Provider: mirror.GitHub, Host: "forge.example", Path: "example/project"}
			scope := mirror.Scope{Repository: repo, Kind: mirror.Issues, AdmissionID: a.Name}
			now := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
			initial := mirror.ScanUpdate{Scope: scope, Coverage: mirror.Coverage{State: mirror.Complete, LastAttempted: now, LastComplete: now, LastInventory: now}, CompleteScan: true, FullInventory: true, Upsert: []mirror.Observation{{ID: mirror.RecordID{Repository: repo, Kind: "issue", Number: 1}, Title: "held-canary", InInventory: true}}}
			snap, err := store.Apply(ctx, a, initial)
			if err != nil {
				t.Fatal(err)
			}
			loss := mirror.ScanUpdate{Scope: scope, ExpectedVersion: snap.Version, Coverage: snap.Coverage, RetentionDecisionRequired: true}
			loss.Coverage.State = mirror.Unavailable
			loss.Coverage.Reason = mirror.Permission
			loss.Coverage.LastAttempted = now.Add(time.Minute)
			held, err := store.Apply(ctx, a, loss)
			if err != nil {
				t.Fatal(err)
			}
			incomplete := mirror.ScanUpdate{Scope: scope, ExpectedVersion: held.Version, Coverage: held.Coverage}
			incomplete.Coverage.Reason = failure
			incomplete.Coverage.LastAttempted = now.Add(2 * time.Minute)
			if failure == mirror.Truncated {
				incomplete.Coverage.State = mirror.Partial
			}
			if err := incomplete.Validate(); err != nil {
				t.Fatalf("control must reach independent store transition: %v", err)
			}
			if _, err := store.Apply(ctx, a, incomplete); !errors.Is(err, mirror.ErrInvalid) {
				t.Fatalf("store cleared hold without a complete observation: %v", err)
			}
			stillHeld, err := store.Read(ctx, a, scope)
			if err != nil || stillHeld.Version != held.Version || stillHeld.Coverage.Reason != mirror.Permission {
				t.Fatalf("refused update changed hold: %+v %v", stillHeld, err)
			}
			r, err := mirror.QueryScope(ctx, store, a, mirror.Query{Scope: scope, Now: now.Add(2 * time.Minute), MaxAge: time.Hour})
			if err != nil || len(r.Snapshot.Records) != 0 || r.Snapshot.Coverage.Reason != mirror.Permission {
				t.Fatalf("bypassed planner disclosed content: %+v %v", r, err)
			}
		})
	}
}

// CheckHoldRecovery tests the recovery transition with the planner bypassed.
func CheckHoldRecovery(t *testing.T, newStore func() mirror.Store) {
	t.Helper()
	ctx := context.Background()
	store := newStore()
	a := &TestAdmission{Name: "admission-a", Allowed: true}
	repo := mirror.RepositoryID{Provider: mirror.GitHub, Host: "forge.example", Path: "example/project"}
	scope := mirror.Scope{Repository: repo, Kind: mirror.Issues, AdmissionID: a.Name}
	now := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	initial := mirror.ScanUpdate{Scope: scope, Coverage: mirror.Coverage{State: mirror.Complete, LastAttempted: now, LastComplete: now, LastInventory: now}, CompleteScan: true, FullInventory: true, Upsert: []mirror.Observation{{ID: mirror.RecordID{Repository: repo, Kind: "issue", Number: 1}, Title: "held-canary", InInventory: true}}}
	snap, err := store.Apply(ctx, a, initial)
	if err != nil {
		t.Fatal(err)
	}
	loss := mirror.ScanUpdate{Scope: scope, ExpectedVersion: snap.Version, Coverage: snap.Coverage, RetentionDecisionRequired: true}
	loss.Coverage.State, loss.Coverage.Reason = mirror.Unavailable, mirror.Permission
	loss.Coverage.LastAttempted = now.Add(time.Minute)
	held, err := store.Apply(ctx, a, loss)
	if err != nil {
		t.Fatal(err)
	}
	fresh := now.Add(2 * time.Minute)
	recovery := mirror.ScanUpdate{Scope: scope, ExpectedVersion: held.Version, CompleteScan: true, FullInventory: true, Coverage: mirror.Coverage{State: mirror.Complete, LastAttempted: fresh, LastComplete: fresh, LastInventory: fresh}}
	for _, name := range []string{"stale-completeness", "equal-completeness", "stale-inventory", "equal-inventory", "incremental", "incremental-fresh-inventory", "marked-partial", "marked-partial-old-watermark"} {
		t.Run(name, func(t *testing.T) {
			u := recovery
			switch name {
			case "stale-completeness":
				u.Coverage.LastComplete, u.Coverage.LastInventory = now, now
			case "equal-completeness":
				u.Coverage.LastComplete, u.Coverage.LastInventory = loss.Coverage.LastAttempted, now
			case "stale-inventory":
				u.Coverage.LastInventory = now
			case "equal-inventory":
				u.Coverage.LastInventory = loss.Coverage.LastAttempted
			case "incremental":
				u.FullInventory = false
				u.Coverage.LastInventory = now
			case "incremental-fresh-inventory":
				u.FullInventory = false
			case "marked-partial":
				u.Coverage.State, u.Coverage.Reason = mirror.Partial, mirror.Truncated
			case "marked-partial-old-watermark":
				u.Coverage.State, u.Coverage.Reason = mirror.Partial, mirror.Truncated
				u.Coverage.LastComplete, u.Coverage.LastInventory = now, now
			}
			if err := u.Validate(); err != nil {
				t.Fatalf("control did not reach Store: %v", err)
			}
			if _, err := store.Apply(ctx, a, u); !errors.Is(err, mirror.ErrInvalid) {
				t.Fatalf("invalid recovery accepted: %v", err)
			}
			s, err := store.Read(ctx, a, scope)
			if err != nil || s.Version != held.Version || s.Coverage.Reason != mirror.Permission {
				t.Fatalf("refusal changed hold: %+v %v", s, err)
			}
			r, err := mirror.QueryScope(ctx, store, a, mirror.Query{Scope: scope, Now: fresh, MaxAge: time.Hour})
			if err != nil || r.Available || len(r.Snapshot.Records) != 0 {
				t.Fatalf("invalid recovery disclosed rows: %+v %v", r, err)
			}
		})
	}
	for _, bad := range []mirror.Admission{&TestAdmission{Name: "admission-b", Allowed: true}, &TestAdmission{Name: a.Name, Allowed: false}} {
		if _, err := store.Apply(ctx, bad, recovery); !errors.Is(err, mirror.ErrPermission) {
			t.Fatalf("recovery ignored current matching admission: %v", err)
		}
	}
	if _, err := store.Apply(ctx, a, recovery); err != nil {
		t.Fatalf("fresh full recovery refused: %v", err)
	}
	r, err := mirror.QueryScope(ctx, store, a, mirror.Query{Scope: scope, Now: fresh, MaxAge: time.Hour})
	if err != nil || !r.Available || len(r.Snapshot.Records) != 1 {
		t.Fatalf("fresh full recovery failed: %+v %v", r, err)
	}
}
