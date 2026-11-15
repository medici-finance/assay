package client_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/refresh"
	"github.com/medici-finance/assay/forgeread/service"
)

// HTTP fixture only: literal producer state qualifies wire/client behavior, not
// actual shared-gate ownership, SQL persistence, ingestion or serving admission.
func checkServerCacheAdvice(t *testing.T) {
	q, _, _ := literal(t)
	q.Operation, q.Source = service.MirrorQuery, nil
	q.Mirror = &service.MirrorRequest{Kind: mirror.Issues, MaxAgeMS: 60000}
	selected := mirror.Scope{Repository: q.Repository, Kind: mirror.Issues, AdmissionID: q.Scope.AdmissionID}
	start := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
	var mu sync.Mutex
	now := start
	var snapshot *mirror.Snapshot
	var retryAt time.Time
	sourceFailed := false
	down, denied := false, false
	var sourceCalls, handlers, attempts, foreign atomic.Int64
	// This maps an actual typed producer result without changing its observation
	// times or retry floor. The fixture does not supply a production gate backend.
	envelope := func(request service.Request, result refresh.Result) service.Response {
		out := service.Held(request, result.Reason)
		if result.Fulfilled {
			yes := true
			s := mirror.CloneSnapshot(result.Snapshot)
			out.Metadata = service.Metadata{State: service.Confirmed, Complete: &yes, Freshness: "fresh", Source: &service.Provenance{Source: "fixture-source", Version: "source-v1", Revision: s.Coverage.Revision, ObservedAt: s.Coverage.LastComplete}}
			out.Mirror = &s
			return out
		}
		if !result.RetryAt.IsZero() {
			floor := result.RetryAt
			out.Metadata.RetryAt = &floor
		}
		if !result.Snapshot.Coverage.LastComplete.IsZero() {
			no := false
			age := now.Sub(result.Snapshot.Coverage.LastComplete).Milliseconds()
			out.Metadata.Complete = &no
			out.Metadata.Freshness = "stale"
			out.Metadata.Cached = &service.CachedObservation{Coverage: result.Snapshot.Coverage, EvaluatedAt: now, AgeMS: &age}
			out.Metadata.Source = &service.Provenance{Source: "fixture-source", Version: "source-v1", Revision: result.Snapshot.Coverage.Revision, ObservedAt: result.Snapshot.Coverage.LastComplete}
		}
		return out
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		handlers.Add(1)
		raw, err := io.ReadAll(io.LimitReader(req.Body, service.MaxMessageBytes+1))
		request, decodeErr := service.DecodeRequest(raw)
		if err != nil || decodeErr != nil || !service.SameScope(request.Scope, q.Scope) || request.Operation != service.MirrorQuery || request.Repository != q.Repository || req.Header.Get("X-Fixture-Subject") != "reader" {
			w.WriteHeader(403)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if denied {
			w.Header().Set("Retry-After", "30")
			w.WriteHeader(403)
			return
		}
		if down {
			w.Header().Set("Retry-After", "30")
			w.WriteHeader(503)
			return
		}
		var result refresh.Result
		if snapshot != nil {
			evaluated, e := mirror.EvaluateSnapshot(*snapshot, mirror.Query{Scope: selected, Now: now, MaxAge: time.Minute})
			if e != nil {
				t.Error(e)
				w.WriteHeader(500)
				return
			}
			result = refresh.Result{Result: evaluated, Fulfilled: evaluated.Available}
		}
		if !result.Fulfilled {
			if retryAt.IsZero() || !now.Before(retryAt) {
				sourceCalls.Add(1)
				if sourceFailed {
					retryAt = now.Add(30 * time.Second)
				} else {
					s := mirror.Snapshot{Scope: selected, Version: "fixture-version", Records: []mirror.Observation{}, Coverage: mirror.Coverage{State: mirror.Complete, LastAttempted: now, LastComplete: now, LastInventory: now, ObservedUpTo: now, Revision: "observed-revision"}}
					snapshot = &s
					retryAt = time.Time{}
					evaluated, e := mirror.EvaluateSnapshot(s, mirror.Query{Scope: selected, Now: now, MaxAge: time.Minute})
					if e != nil {
						t.Error(e)
						w.WriteHeader(500)
						return
					}
					result = refresh.Result{Result: evaluated, Fulfilled: true}
				}
			}
			if !result.Fulfilled {
				result.Reason = mirror.Transport
				result.RetryAt = retryAt
			}
		}
		wire, e := service.EncodeResponse(envelope(request, result), request)
		if e != nil {
			t.Error(e)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(wire)
	}))
	defer server.Close()
	rt := trip(func(request *http.Request) (*http.Response, error) {
		attempts.Add(1)
		if request.URL.Scheme+"://"+request.URL.Host != server.URL {
			foreign.Add(1)
			return nil, errors.New("fixture refused alternate endpoint")
		}
		request.Header.Set("X-Fixture-Subject", "reader")
		return server.Client().Transport.RoundTrip(request)
	})
	first := configured(t, q, server.URL+"/forge/v1", rt, false)
	second := configured(t, q, server.URL+"/forge/v1", rt, false)
	read := func(which int) (service.Response, error) {
		t.Helper()
		before := attempts.Load()
		c := first
		if which == 1 {
			c = second
		}
		out, report, err := c.Do(context.Background(), q)
		if report.LogicalCalls != 1 || report.TransportAttempts != 1 || attempts.Load() != before+1 {
			t.Fatal("SDK retried or changed its accounting", report)
		}
		return out, err
	}
	out, err := read(0)
	if err != nil || out.Mirror == nil || out.Mirror.Records == nil || len(out.Mirror.Records) != 0 || sourceCalls.Load() != 1 {
		t.Fatal("cold server fill failed", err, out.Metadata)
	}
	original := out.Mirror.Coverage.LastComplete
	out, err = read(1)
	if err != nil || out.Mirror == nil || sourceCalls.Load() != 1 {
		t.Fatal("warm server cache called source", err)
	}
	mu.Lock()
	now = start.Add(2 * time.Minute)
	sourceFailed = true
	mu.Unlock()
	out, err = read(0)
	if !errors.Is(err, service.ErrInvalid) || !noData(out) || out.Metadata.Cached == nil || out.Metadata.RetryAt == nil || sourceCalls.Load() != 2 {
		t.Fatal("failure advice lost", err, out.Metadata)
	}
	floor := *out.Metadata.RetryAt
	for i := 0; i < 8; i++ {
		mu.Lock()
		now = start.Add(2*time.Minute + time.Duration(i+1)*time.Second)
		mu.Unlock()
		out, err = read(i % 2)
		if !errors.Is(err, service.ErrInvalid) || !noData(out) || out.Metadata.Cached == nil || out.Metadata.RetryAt == nil || !out.Metadata.RetryAt.Equal(floor) || out.Metadata.Cached.AgeMS == nil || *out.Metadata.Cached.AgeMS != int64(120+i+1)*1000 || !out.Metadata.Cached.Coverage.LastComplete.Equal(original) || out.Metadata.Source == nil || !out.Metadata.Source.ObservedAt.Equal(original) || sourceCalls.Load() != 2 {
			t.Fatal("shared fixture cooldown changed original observation or attempted source", err, out.Metadata)
		}
	}
	mu.Lock()
	down = true
	mu.Unlock()
	out, err = read(0)
	if err == nil || !noData(out) || out.Metadata.Cached != nil || out.Metadata.RetryAt != nil || out.Metadata.Source != nil || sourceCalls.Load() != 2 {
		t.Fatal("service-down invented cache, provider retry advice or fallback", err, out.Metadata)
	}
	mu.Lock()
	down = false
	denied = true
	mu.Unlock()
	out, err = read(1)
	if !errors.Is(err, service.ErrScope) || out.Metadata.State != service.Permission || !noData(out) || out.Metadata.Cached != nil || out.Metadata.RetryAt != nil || out.Metadata.Source != nil || sourceCalls.Load() != 2 {
		t.Fatal("permission exposed cached observation or eligibility", err, out.Metadata)
	}
	mu.Lock()
	denied = false
	sourceFailed = false
	now = floor.Add(time.Second)
	mu.Unlock()
	out, err = read(0)
	if err != nil || out.Mirror == nil || out.Metadata.Cached != nil || out.Metadata.RetryAt != nil || !out.Mirror.Coverage.LastComplete.After(original) || sourceCalls.Load() != 3 {
		t.Fatal("server recovery failed", err, out.Metadata)
	}
	// A cold failed source has no cached observation; unknown completeness must
	// survive both clients without becoming a successful empty inventory.
	mu.Lock()
	snapshot = nil
	retryAt = time.Time{}
	sourceFailed = true
	mu.Unlock()
	for i := 0; i < 2; i++ {
		out, err = read(i)
		if !errors.Is(err, service.ErrInvalid) || !noData(out) || out.Metadata.Complete != nil || out.Metadata.Freshness != "unknown" || out.Metadata.Cached != nil || out.Metadata.Source != nil || out.Metadata.RetryAt == nil || sourceCalls.Load() != 4 {
			t.Fatal("cold source failure became known-empty or bypassed fixture cooldown", err, out.Metadata)
		}
	}
	if foreign.Load() != 0 || attempts.Load() != handlers.Load() {
		t.Fatal("client bypassed admitted service transport")
	}
}
