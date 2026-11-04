package service_test

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/service"
)

// These are wire fixtures. They establish no source, gate or current authorization.
func cachedRead(t *testing.T, age time.Duration, state service.State) (service.Request, service.Response) {
	t.Helper()
	q := literalRequest(t)
	r := literalResponse(t, q)
	q.Operation, q.Source = service.MirrorQuery, nil
	q.Mirror = &service.MirrorRequest{Kind: mirror.Issues, MaxAgeMS: 60000}
	r.Operation, r.Source = q.Operation, nil
	now := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
	observed := now.Add(-age)
	n := age.Milliseconds()
	no := false
	r.Metadata = service.Metadata{State: state, Reason: mirror.Transport, Complete: &no, Freshness: "stale",
		Cached: &service.CachedObservation{Coverage: mirror.Coverage{State: mirror.Complete, LastAttempted: observed, LastComplete: observed, LastInventory: observed, ObservedUpTo: observed, Revision: "original-revision", Validator: "original-validator"}, EvaluatedAt: now, AgeMS: &n},
		Source: &service.Provenance{Source: "admitted-source", Version: "source-v1", Revision: "original-revision", ObservedAt: observed}}
	if age <= time.Minute {
		r.Metadata.Freshness = "fresh"
	}
	if state == service.Older {
		q.Mirror.Minimum = &service.Token{Scope: q.Scope, Repository: q.Repository, Kind: mirror.Issues, Value: "required-observation"}
		r.Metadata.Observation = &service.Token{Scope: q.Scope, Repository: q.Repository, Kind: mirror.Issues, Value: "older-observation"}
	}
	return q, r
}

func cacheRoundTrip(t *testing.T, q service.Request, r service.Response) {
	t.Helper()
	wire, err := service.EncodeResponse(r, q)
	if err != nil {
		t.Fatal("qualified cache/advice refused", err)
	}
	out, err := service.DecodeResponse(wire, q)
	if err != nil || !reflect.DeepEqual(out.Metadata, r.Metadata) {
		t.Fatalf("original cache/advice changed: err=%v got=%+v want=%+v", err, out.Metadata, r.Metadata)
	}
}

func cacheRefusal(t *testing.T, q service.Request, r service.Response) {
	t.Helper()
	if _, err := service.EncodeResponse(r, q); err == nil {
		t.Error("invalid cache/advice encoded")
	}
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.DecodeResponse(raw, q); err == nil {
		t.Error("invalid cache/advice decoded")
	}
}

func checkCachedReadAdvice(t *testing.T) {
	for _, state := range []service.State{service.Older, service.Unavailable} {
		for _, age := range []time.Duration{30 * time.Second, 2 * time.Minute} {
			t.Run(string(state)+"/"+age.String(), func(t *testing.T) {
				q, r := cachedRead(t, age, state)
				retry := r.Metadata.Cached.EvaluatedAt.Add(30 * time.Second)
				r.Metadata.RetryAt = &retry
				cacheRoundTrip(t, q, r)
			})
		}
	}
	t.Run("older-unknown-provenance", func(t *testing.T) {
		q, r := cachedRead(t, 30*time.Second, service.Older)
		r.Metadata.Source = nil
		cacheRoundTrip(t, q, r)
	})
	t.Run("unknown-age", func(t *testing.T) {
		q, r := cachedRead(t, 2*time.Minute, service.Unavailable)
		r.Metadata.Cached.Coverage = mirror.Coverage{State: mirror.NeverRead}
		r.Metadata.Cached.AgeMS = nil
		r.Metadata.Source = nil
		cacheRoundTrip(t, q, r)
	})
	t.Run("elapsed-overflow-is-unknown", func(t *testing.T) {
		q, r := cachedRead(t, 2*time.Minute, service.Unavailable)
		start := time.Date(100, 1, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)
		r.Metadata.Cached.Coverage = mirror.Coverage{State: mirror.Complete, LastAttempted: start, LastComplete: start}
		r.Metadata.Cached.EvaluatedAt = end
		r.Metadata.Cached.AgeMS = nil
		r.Metadata.Source.ObservedAt = start
		cacheRoundTrip(t, q, r)
	})
	t.Run("plain-read-retry", func(t *testing.T) {
		q := literalRequest(t)
		r := service.Held(q, mirror.RateLimited)
		retry := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
		r.Metadata.RetryAt = &retry
		cacheRoundTrip(t, q, r)
	})
	for _, tc := range []struct {
		name   string
		mutate func(*service.Request, *service.Response)
	}{
		{"negative-age", func(_ *service.Request, r *service.Response) { n := int64(-1); r.Metadata.Cached.AgeMS = &n }},
		{"wrong-age", func(_ *service.Request, r *service.Response) { n := int64(1); r.Metadata.Cached.AgeMS = &n }},
		{"missing-known-age", func(_ *service.Request, r *service.Response) { r.Metadata.Cached.AgeMS = nil }},
		{"future-coverage", func(_ *service.Request, r *service.Response) {
			r.Metadata.Cached.Coverage.ObservedUpTo = r.Metadata.Cached.EvaluatedAt.Add(time.Second)
		}},
		{"future-provenance", func(_ *service.Request, r *service.Response) {
			r.Metadata.Source.ObservedAt = r.Metadata.Cached.EvaluatedAt.Add(time.Second)
		}},
		{"missing-evaluation", func(_ *service.Request, r *service.Response) { r.Metadata.Cached.EvaluatedAt = time.Time{} }},
		{"unknown-state", func(_ *service.Request, r *service.Response) {
			r.Metadata.Cached.Coverage.State = mirror.CoverageState("invalid")
		}},
		{"permission-coverage", func(_ *service.Request, r *service.Response) {
			r.Metadata.Cached.Coverage.State = mirror.Unavailable
			r.Metadata.Cached.Coverage.Reason = mirror.Permission
		}},
		{"false-freshness", func(_ *service.Request, r *service.Response) { r.Metadata.Freshness = "fresh" }},
		{"false-completeness", func(_ *service.Request, r *service.Response) { yes := true; r.Metadata.Complete = &yes }},
		{"freshness-not-token-order", func(q *service.Request, r *service.Response) { r.Metadata.State = service.Older }},
		{"zero-retry", func(_ *service.Request, r *service.Response) { r.Metadata.RetryAt = &time.Time{} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q, r := cachedRead(t, 2*time.Minute, service.Unavailable)
			tc.mutate(&q, &r)
			cacheRefusal(t, q, r)
		})
	}
	t.Run("older-permission-reason", func(t *testing.T) {
		q, r := cachedRead(t, 2*time.Minute, service.Older)
		r.Metadata.Cached = nil
		r.Metadata.Reason = mirror.Permission
		cacheRefusal(t, q, r)
	})
	t.Run("unknown-age-is-not-zero", func(t *testing.T) {
		q, r := cachedRead(t, 2*time.Minute, service.Unavailable)
		r.Metadata.Cached.Coverage = mirror.Coverage{State: mirror.NeverRead}
		n := int64(0)
		r.Metadata.Cached.AgeMS = &n
		cacheRefusal(t, q, r)
	})
	t.Run("permission-advice", func(t *testing.T) {
		q, r := cachedRead(t, 2*time.Minute, service.Unavailable)
		cached := r.Metadata.Cached
		r = service.Held(q, mirror.Permission)
		r.Metadata.Cached = cached
		cacheRefusal(t, q, r)
		r.Metadata.Cached = nil
		retry := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
		r.Metadata.RetryAt = &retry
		cacheRefusal(t, q, r)
	})
	t.Run("fulfilled-advice", func(t *testing.T) {
		q := literalRequest(t)
		r := literalResponse(t, q)
		retry := r.Metadata.Source.ObservedAt
		r.Metadata.RetryAt = &retry
		cacheRefusal(t, q, r)
		r.Metadata.RetryAt = nil
		_, cached := cachedRead(t, time.Minute, service.Unavailable)
		r.Metadata.Cached = cached.Metadata.Cached
		cacheRefusal(t, q, r)
	})
	t.Run("effect-retry", func(t *testing.T) {
		for _, tc := range operationCases(t) {
			q, err := service.DecodeRequest(tc.Request)
			if err != nil {
				t.Fatal(err)
			}
			if q.Operation != service.CreateIssue && q.Operation != service.CreateComment {
				continue
			}
			r := service.Held(q, mirror.Transport)
			retry := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
			r.Metadata.RetryAt = &retry
			cacheRefusal(t, q, r)
		}
	})
}
