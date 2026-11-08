package client_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/medici-finance/assay/forgeread/client"
	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/service"
)

type trip func(*http.Request) (*http.Response, error)

func (f trip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func literal(t *testing.T) (service.Request, service.Response, []byte) {
	t.Helper()
	read := func(name string) []byte {
		t.Helper()
		b, err := os.ReadFile(filepath.Join("..", "testdata", "service", name))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	q, err := service.DecodeRequest(read("request-source.json"))
	if err != nil {
		t.Fatal(err)
	}
	b := read("response-source.json")
	r, err := service.DecodeResponse(b, q)
	if err != nil {
		t.Fatal(err)
	}
	return q, r, b
}
func configured(t *testing.T, q service.Request, endpoint string, rt http.RoundTripper, consent bool) *client.Client {
	t.Helper()
	c, err := client.New(client.Config{Endpoint: endpoint, Transport: rt, Scope: q.Scope, Timeout: time.Second, DiagnosticConsent: consent})
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func noData(r service.Response) bool {
	return r.Source == nil && r.Mirror == nil && r.Issues == nil && r.Effect == nil && r.Review == nil && r.Projection == nil
}

// regression: class service-client
func TestForgeServiceClientFlow(t *testing.T) {
	t.Run("server-cache-advice", checkServerCacheAdvice)
	t.Run("kind-mismatch", checkKindMismatch)
	t.Run("actual-http", func(t *testing.T) {
		q, r, _ := literal(t)
		one := int64(1)
		r.Measurements = []service.Measurement{{Boundary: "server-handler", Unit: "handler", Count: &one}}
		var handlers atomic.Int64
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			handlers.Add(1)
			b, _ := io.ReadAll(io.LimitReader(req.Body, service.MaxMessageBytes+1))
			got, err := service.DecodeRequest(b)
			if err != nil || req.URL.Path != "/forge/v1" || req.Header.Get("X-Fixture-Subject") != "reader" || !service.SameScope(got.Scope, q.Scope) {
				w.WriteHeader(403)
				return
			}
			if req.Header.Get("X-Diagnostic-Link") != "" {
				t.Error("automatic diagnostic header")
			}
			wire, err := service.EncodeResponse(r, got)
			if err != nil {
				t.Error(err)
				w.WriteHeader(500)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write(wire)
		}))
		defer s.Close()
		rt := trip(func(req *http.Request) (*http.Response, error) {
			req.Header.Set("X-Fixture-Subject", "reader")
			return s.Client().Transport.RoundTrip(req)
		})
		c := configured(t, q, s.URL+"/forge/v1", rt, false)
		out, report, err := c.Do(context.Background(), q)
		if err != nil || string(out.Source.Content) != "fixture\n" || handlers.Load() != 1 || report.LogicalCalls != 1 || report.TransportAttempts != 1 || report.RequestBodyBytes == nil || report.ResponseBodyBytes == nil || *report.ResponseBodyBytes < 1 || report.Duration <= 0 || len(out.Measurements) != 1 || *out.Measurements[0].Count != 1 {
			t.Fatal("typed HTTP flow/own accounting lost", err, report)
		}
		// A server handler and a client invocation remain separate units.
		if out.Measurements[0].Unit == "call" || report.LogicalCalls != 1 {
			t.Fatal("descendant work copied as logical calls")
		}
	})
	t.Run("states", func(t *testing.T) {
		for _, state := range []service.State{service.Confirmed, service.Newer, service.Older, service.ProjectionPending} {
			t.Run(string(state), func(t *testing.T) {
				q, r, _ := literal(t)
				r.Metadata.State = state
				if state == service.Older || state == service.Newer {
					q.Operation, q.Source = service.ReadObservation, nil
					q.Minimum = &service.Token{Scope: q.Scope, Repository: q.Repository, Kind: mirror.Issues, Value: "wanted"}
					r.Operation, r.Source = q.Operation, nil
					if state == service.Older {
						f := false
						r.Metadata.Complete = &f
						r.Metadata.Freshness = "stale"
					}
					r.Metadata.Observation = &service.Token{Scope: q.Scope, Repository: q.Repository, Kind: mirror.Issues, Value: string(state)}
				}
				if state == service.ProjectionPending {
					q.Operation, q.Source = service.CreateIssue, nil
					q.Effect = &service.EffectRequest{ID: service.OperationID{Key: "fixture", PayloadDigest: strings.Repeat("a", 64)}, Title: "fixture", Body: "fixture"}
					r.Operation, r.Source = q.Operation, nil
					f := false
					r.Metadata.Complete = &f
					r.Metadata.Freshness = "unknown"
					r.Effect = &service.EffectResult{ID: q.Effect.ID, Outcome: service.Unknown, Projection: service.NoProjection}
				}
				wire, err := service.EncodeResponse(r, q)
				if err != nil {
					t.Fatal(err)
				}
				var calls atomic.Int64
				c := configured(t, q, "https://service.example/forge", trip(func(req *http.Request) (*http.Response, error) {
					calls.Add(1)
					return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(bytes.NewReader(wire))}, nil
				}), false)
				out, report, err := c.Do(context.Background(), q)
				if err != nil || out.Metadata.State != state || calls.Load() != 1 || report.TransportAttempts != 1 {
					t.Fatal("state changed", err)
				}
				if state == service.Older && !noData(out) {
					t.Fatal("older minimum read released data")
				}
				if state == service.ProjectionPending && (out.Effect.Outcome != service.Unknown || out.Effect.Object != nil) {
					t.Fatal("unknown effect became provider success")
				}
			})
		}
	})
	t.Run("denials", func(t *testing.T) {
		for _, status := range []int{401, 403, 404, 429, 503, 409} {
			q, _, _ := literal(t)
			var calls atomic.Int64
			var closed atomic.Int64
			body := &closeBody{Reader: strings.NewReader("private-object-canary"), close: func() { closed.Add(1) }}
			c := configured(t, q, "https://service.example/forge", trip(func(*http.Request) (*http.Response, error) {
				calls.Add(1)
				return &http.Response{StatusCode: status, Header: http.Header{}, Body: body}, nil
			}), false)
			out, report, err := c.Do(context.Background(), q)
			if err == nil || !noData(out) || calls.Load() != 1 || closed.Load() != 1 || report.TransportAttempts != 1 || report.ResponseBodyBytes != nil || strings.Contains(err.Error(), "private-object") {
				t.Fatal("denial leaked data, retried or lost close", status, err)
			}
			if (status == 401 || status == 403 || status == 404) && out.Metadata.State != service.Permission {
				t.Fatal("permission/existence distinguished")
			}
			if status == 409 && !errors.Is(err, service.ErrConflict) {
				t.Fatal("payload conflict lost")
			}
		}
	})
	t.Run("scope-and-constructor", func(t *testing.T) {
		q, _, _ := literal(t)
		var calls atomic.Int64
		rt := trip(func(*http.Request) (*http.Response, error) {
			calls.Add(1)
			return nil, errors.New("transport-sensitive-canary")
		})
		for _, endpoint := range []string{"http://service.example/forge", "https://service.example/forge?credential=fixture", "https://service.example/forge#fragment", "https://service.example/%2fforge"} {
			if _, err := client.New(client.Config{Endpoint: endpoint, Transport: rt, Scope: q.Scope, Timeout: time.Second}); err == nil {
				t.Fatal("untrusted endpoint form accepted")
			}
		}
		if _, err := client.New(client.Config{Endpoint: "https://service.example/forge", Scope: q.Scope, Timeout: time.Second}); err == nil {
			t.Fatal("default transport discovered")
		}
		var nilRT *http.Transport
		if _, err := client.New(client.Config{Endpoint: "https://service.example/forge", Transport: nilRT, Scope: q.Scope, Timeout: time.Second}); err == nil {
			t.Fatal("typed nil transport accepted")
		}
		c := configured(t, q, "https://service.example/forge", rt, false)
		q.Scope.Repositories[0].Path = "other/private"
		q.Repository = q.Scope.Repositories[0]
		out, report, err := c.Do(context.Background(), q)
		if !errors.Is(err, service.ErrScope) || !noData(out) || calls.Load() != 0 || report.TransportAttempts != 0 {
			t.Fatal("caller mutation widened constructor binding", err)
		}
	})
	t.Run("redirect", func(t *testing.T) {
		q, _, _ := literal(t)
		var escaped atomic.Int64
		other := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { escaped.Add(1) }))
		defer other.Close()
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Location", other.URL)
			w.WriteHeader(302)
		}))
		defer s.Close()
		c := configured(t, q, s.URL, s.Client().Transport, false)
		out, report, err := c.Do(context.Background(), q)
		if err == nil || !noData(out) || escaped.Load() != 0 || report.TransportAttempts != 1 {
			t.Fatal("redirect escaped admitted endpoint")
		}
	})
	t.Run("links", func(t *testing.T) {
		q, r, _ := literal(t)
		q.Diagnostic = &service.DiagnosticLink{Parent: "parent-a", Link: "link-a"}
		r.Diagnostic = q.Diagnostic
		wire, err := service.EncodeResponse(r, q)
		if err != nil {
			t.Fatal(err)
		}
		var calls atomic.Int64
		rt := trip(func(req *http.Request) (*http.Response, error) {
			calls.Add(1)
			if req.Header.Get("X-Diagnostic-Link") != "" {
				t.Error("automatic header")
			}
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(bytes.NewReader(wire))}, nil
		})
		c := configured(t, q, "https://service.example/forge", rt, false)
		if out, report, err := c.Do(context.Background(), q); err == nil || !noData(out) || report.TransportAttempts != 0 || calls.Load() != 0 {
			t.Fatal("links propagated without both-end consent")
		}
		c = configured(t, q, "https://service.example/forge", rt, true)
		if out, _, err := c.Do(context.Background(), q); err != nil || out.Diagnostic == nil || *out.Diagnostic != *q.Diagnostic || calls.Load() != 1 {
			t.Fatal("admitted link lost", err)
		}
	})
	t.Run("last-io", func(t *testing.T) {
		q, _, wire := literal(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var closed atomic.Int64
		c := configured(t, q, "https://service.example/forge", trip(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: &closeBody{Reader: bytes.NewReader(wire), close: func() { closed.Add(1); cancel() }}}, nil
		}), false)
		out, _, err := c.Do(ctx, q)
		if !errors.Is(err, context.Canceled) || !noData(out) || closed.Load() != 1 {
			t.Fatal("canceled final body close released private rows", err)
		}
	})
	t.Run("response-plus-error", func(t *testing.T) {
		q, _, wire := literal(t)
		var closed atomic.Int64
		c := configured(t, q, "https://service.example/forge", trip(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: &closeBody{Reader: bytes.NewReader(wire), close: func() { closed.Add(1) }}}, errors.New("transport-sensitive-canary")
		}), false)
		out, report, err := c.Do(context.Background(), q)
		if err == nil || !noData(out) || closed.Load() != 1 || report.TransportAttempts != 1 || strings.Contains(err.Error(), "transport-sensitive") {
			t.Fatal("response/error branch leaked or lost resource close", err)
		}
	})
	t.Run("bad-wire", func(t *testing.T) {
		q, r, wire := literal(t)
		wrongScope := r
		wrongScope.Scope.AccessEpoch = "other-epoch"
		wrong, _ := json.Marshal(wrongScope)
		for name, raw := range map[string][]byte{"scope": wrong, "oversize": append(append([]byte{}, wire...), bytes.Repeat([]byte{' '}, service.MaxMessageBytes)...), "trailing": append(append([]byte{}, wire...), []byte("{}")...)} {
			t.Run(name, func(t *testing.T) {
				c := configured(t, q, "https://service.example/forge", trip(func(*http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(bytes.NewReader(raw))}, nil
				}), false)
				if out, _, err := c.Do(context.Background(), q); err == nil || !noData(out) {
					t.Fatal("unqualified response became data")
				}
			})
		}
	})
}

type closeBody struct {
	io.Reader
	close func()
}

func (b *closeBody) Close() error { b.close(); return nil }

// The malformed server body is independent of the production response encoder.
func checkKindMismatch(t *testing.T) {
	for _, kind := range []mirror.RecordKind{mirror.Issues, mirror.Changes} {
		for _, state := range []service.State{service.Confirmed, service.Newer, service.Older} {
			t.Run(string(kind)+"/"+string(state), func(t *testing.T) {
				q, r, _ := literal(t)
				q.Operation, q.Source = service.ReadObservation, nil
				q.Minimum = &service.Token{Scope: q.Scope, Repository: q.Repository, Kind: kind, Value: "want"}
				r.Operation, r.Source = q.Operation, nil
				r.Metadata.State = state
				value := "want"
				if state != service.Confirmed {
					value = "have"
				}
				r.Metadata.Observation = &service.Token{Scope: q.Scope, Repository: q.Repository, Kind: kind, Value: value}
				if state == service.Older {
					no := false
					r.Metadata.Complete = &no
					r.Metadata.Freshness = "stale"
				}
				wire, err := service.EncodeResponse(r, q)
				if err != nil {
					t.Fatal("valid kind baseline refused", err)
				}
				var calls atomic.Int64
				c := configured(t, q, "https://service.example/forge", trip(func(*http.Request) (*http.Response, error) {
					calls.Add(1)
					return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(bytes.NewReader(wire))}, nil
				}), false)
				if out, report, err := c.Do(context.Background(), q); err != nil || out.Metadata.State != state || report.TransportAttempts != 1 || calls.Load() != 1 {
					t.Fatal("valid kind client baseline changed", err)
				}
				other := mirror.Changes
				if kind == mirror.Changes {
					other = mirror.Issues
				}
				r.Metadata.Observation.Kind = other
				wire, err = json.Marshal(r)
				if err != nil {
					t.Fatal(err)
				}
				out, report, err := c.Do(context.Background(), q)
				if err == nil || !noData(out) || out.Metadata.Source != nil || out.Metadata.Observation != nil || report.TransportAttempts != 1 || calls.Load() != 2 {
					t.Fatal("client accepted foreign observation kind", err, report)
				}
			})
		}
	}
}
