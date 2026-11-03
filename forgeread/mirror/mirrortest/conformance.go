package mirrortest

import (
	"context"
	"errors"
	"github.com/medici-finance/assay/forgeread/mirror"
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
