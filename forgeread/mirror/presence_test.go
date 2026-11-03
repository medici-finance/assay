package mirror_test

import (
	"context"
	"errors"
	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/mirror/mirrortest"
	"reflect"
	"testing"
	"time"
)

// regression: forge-neutral/50
func TestMirrorFieldPresence(t *testing.T) {
	for _, known := range []bool{false, true} {
		scope, a, row := fixture()
		row.BodyKnown = known
		row.Body = ""
		row.Labels = nil
		row.RequestedReviewers = nil
		row.Reviews = mirror.ReviewObservations{State: mirror.NeverRead}
		if known {
			row.Labels = []string{}
			row.RequestedReviewers = []mirror.Actor{}
			row.Reviews.GitHub = []mirror.GitHubReview{}
		}
		store := mirrortest.NewStore()
		old, _ := store.Read(context.Background(), a, scope)
		p, _ := mirror.PlanRefresh(old, instant, policy())
		u, e := mirror.PlanScan(old, p, mirror.ScanResult{Complete: true, ObservedUpTo: instant, Records: []mirror.Observation{row}}, instant)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = store.Apply(context.Background(), a, u); e != nil {
			t.Fatal(e)
		}
		got, e := mirror.QueryScope(context.Background(), store, a, mirror.Query{Scope: scope, Now: instant, MaxAge: time.Minute})
		if e != nil || !got.Available || len(got.Snapshot.Records) != 1 || !reflect.DeepEqual(got.Snapshot.Records[0], row) {
			t.Fatalf("presence lost known=%v: %+v %v", known, got, e)
		}
	}
	scope, a, row := fixture()
	scope.Kind = mirror.Changes
	row.ID.Kind = "pull-request"
	row.BodyKnown = true
	row.Body = "body"
	row.RequestedReviewers = []mirror.Actor{{Login: "reviewer"}}
	row.Reviews = mirror.ReviewObservations{State: mirror.Partial, GitHub: []mirror.GitHubReview{{ID: "review", Author: mirror.Actor{Login: "author"}}}}
	store := mirrortest.NewStore()
	old, _ := store.Read(context.Background(), a, scope)
	p, _ := mirror.PlanRefresh(old, instant, policy())
	u, err := mirror.PlanScan(old, p, mirror.ScanResult{Complete: true, ObservedUpTo: instant, Records: []mirror.Observation{row}}, instant)
	if err != nil {
		t.Fatal(err)
	}
	applied, e := store.Apply(context.Background(), a, u)
	if e != nil {
		t.Fatal(e)
	}
	u.Upsert[0].Labels[0] = "changed-input"
	applied.Records[0].RequestedReviewers[0].Login = "changed-return"
	applied.Records[0].Reviews.GitHub[0].Author.Login = "changed-review"
	got, e := store.Read(context.Background(), a, scope)
	if e != nil || !reflect.DeepEqual(got.Records[0], row) {
		t.Fatalf("mutable collections aliased: %+v %v", got, e)
	}
}

// regression: forge-neutral/50
func TestMirrorPlanBaseline(t *testing.T) {
	_, scope, _, s := seed(t, true)
	now := instant.Add(2 * time.Hour)
	p, e := mirror.PlanRefresh(s, now, policy())
	if e != nil || !p.PriorObservedUpTo.Equal(s.Coverage.ObservedUpTo) || p.ExpectedVersion != s.Version || p.Scope != scope {
		t.Fatalf("baseline not bound: %+v %v", p, e)
	}
	scan := mirror.ScanResult{Complete: true, ObservedUpTo: instant}
	for _, b := range []time.Time{{}, instant.Add(-time.Minute), instant.Add(time.Minute)} {
		bad := p
		bad.PriorObservedUpTo = b
		if _, e := mirror.PlanScan(s, bad, scan, now); !errors.Is(e, mirror.ErrConflict) {
			t.Fatalf("forged baseline accepted: %v %v", b, e)
		}
	}
	bad := p
	bad.ExpectedVersion = "another-version"
	if _, e := mirror.PlanScan(s, bad, scan, now); !errors.Is(e, mirror.ErrConflict) {
		t.Fatal("stale version accepted", e)
	}
	bad = p
	bad.Scope.AdmissionID = "another-admission"
	if _, e := mirror.PlanScan(s, bad, scan, now); !errors.Is(e, mirror.ErrConflict) {
		t.Fatal("foreign admission plan accepted", e)
	}
	scan.ObservedUpTo = instant.Add(-time.Minute)
	if _, e := mirror.PlanScan(s, p, scan, now); !errors.Is(e, mirror.ErrInvalid) {
		t.Fatal("raw regression control weakened", e)
	}
}
