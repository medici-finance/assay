package service_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/forgeread/client"
	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/service"
)

// regression: class service-review-wire
func TestReviewServiceMapping(t *testing.T) {
	t.Run("issue-projection", checkIssueProjectionMapping)
	var cases []struct {
		Case        string          `json:"case"`
		Observation json.RawMessage `json:"observation"`
	}
	if err := json.Unmarshal(fixture(t, "review-observations.json"), &cases); err != nil || len(cases) != 21 {
		t.Fatal("qualified producer literals unavailable", err)
	}
	requests := make([]service.Request, len(cases))
	responses := make([][]byte, len(cases))
	for i, tc := range cases {
		var observation service.ReviewEnvelope
		if err := json.Unmarshal(tc.Observation, &observation); err != nil {
			t.Fatal(err)
		}
		q := literalRequest(t)
		r := literalResponse(t, q)
		q.ID, q.Operation, q.Source, q.Repository = "review-"+strconv.Itoa(i), service.ReviewObservation, nil, mirror.RepositoryID{}
		q.Scope.Repositories = nil
		for _, repo := range observation.Inputs.Requested {
			q.Scope.Repositories = append(q.Scope.Repositories, mirror.RepositoryID{Provider: mirror.Provider(*repo.Provider), Host: *repo.Host, Path: *repo.Repository})
		}
		r.ID, r.Operation, r.Source, r.Repository, r.Scope, r.Review = q.ID, q.Operation, nil, q.Repository, q.Scope, &observation
		b, err := service.EncodeResponse(r, q)
		if err != nil {
			t.Fatalf("producer case %s not transported: %v", tc.Case, err)
		}
		requests[i], responses[i] = q, b
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		b, _ := io.ReadAll(io.LimitReader(req.Body, service.MaxMessageBytes+1))
		q, err := service.DecodeRequest(b)
		i, numberErr := strconv.Atoi(strings.TrimPrefix(q.ID, "review-"))
		if err != nil || numberErr != nil || i < 0 || i >= len(requests) || !service.SameScope(q.Scope, requests[i].Scope) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(responses[i])
	}))
	defer server.Close()
	seenWait, seenSurface, seenNoOp, seenFull, seenEmpty, seenHosts, seenProviders := false, false, false, false, false, false, false
	for i, tc := range cases {
		t.Run(tc.Case, func(t *testing.T) {
			q := requests[i]
			c, err := client.New(client.Config{Endpoint: server.URL, Transport: server.Client().Transport, Scope: q.Scope, Timeout: 5 * time.Second})
			if err != nil {
				t.Fatal(err)
			}
			out, report, err := c.Do(context.Background(), q)
			if err != nil || out.Review == nil || report.LogicalCalls != 1 || report.TransportAttempts != 1 {
				t.Fatal("typed client/producer flow failed", err)
			}
			raw, _ := json.Marshal(out.Review)
			sameJSON(t, raw, tc.Observation)
			if len(out.Review.Result.Rows) == 151 {
				seenFull = *out.Review.Result.Depths.Total == 151
			}
			if len(out.Review.Result.Rows) == 0 {
				seenEmpty = out.Review.Result.Rows != nil && *out.Review.Result.Depths.Total == 0
			}
			if len(q.Scope.Repositories) == 2 {
				seenHosts = seenHosts || q.Scope.Repositories[0].Host != q.Scope.Repositories[1].Host
				seenProviders = seenProviders || q.Scope.Repositories[0].Provider != q.Scope.Repositories[1].Provider
			}
			for _, row := range out.Review.Result.Rows {
				seenWait = seenWait || *row.Disposition == "WAIT"
				seenSurface = seenSurface || *row.Disposition == "SURFACE"
				seenNoOp = seenNoOp || *row.Disposition == "NO-OP"
			}
		})
	}
	if !seenWait || !seenSurface || !seenNoOp || !seenFull || !seenEmpty || !seenHosts || !seenProviders {
		t.Fatal("full producer population/action/scope/empty coverage lost")
	}
	t.Run("malformed", func(t *testing.T) {
		q, raw := requests[0], string(responses[0])
		at := strings.Index(raw, `"review":`)
		partial := raw[:at] + strings.Replace(raw[at:], `"complete":true`, `"complete":false`, 1)
		for name, bad := range map[string]string{
			"schema":           strings.Replace(raw, "review-observation-v1", "review-observation-v9", 1),
			"missing-false":    strings.Replace(raw, `"AtHead":false,`, "", 1),
			"null-false":       strings.Replace(raw, `"AtHead":false`, `"AtHead":null`, 1),
			"unknown-required": strings.Replace(raw, `"schema":"review-observation-v1"`, `"schema":"review-observation-v1","internalImport":true`, 1),
			"body-revision":    strings.Replace(raw, `"body":"set-1"`, `"body":"wrong-revision"`, 1),
			"depth":            strings.Replace(raw, `"total":1`, `"total":0`, 1),
			"partial":          partial,
		} {
			t.Run(name, func(t *testing.T) {
				if bad == raw {
					t.Fatal("negative control did not change fixture")
				}
				if _, err := service.DecodeResponse([]byte(bad), q); err == nil {
					t.Fatal("malformed producer envelope accepted")
				}
			})
		}
	})
}

func checkIssueProjectionMapping(t *testing.T) {
	var literal operationCase
	for _, tc := range operationCases(t) {
		if tc.Case == "issue-projection" {
			literal = tc
		}
	}
	q, err := service.DecodeRequest(literal.Request)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		b, _ := io.ReadAll(io.LimitReader(req.Body, service.MaxMessageBytes+1))
		got, e := service.DecodeRequest(b)
		if e != nil || got.Operation != service.IssueProjection || !service.SameScope(got.Scope, q.Scope) {
			w.WriteHeader(403)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(literal.Response)
	}))
	defer server.Close()
	c, err := client.New(client.Config{Endpoint: server.URL, Transport: server.Client().Transport, Scope: q.Scope, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	out, report, err := c.Do(context.Background(), q)
	if err != nil || out.Projection == nil || len(out.Projection.Rows) != 6 || len(out.Projection.Quarantined) != 1 || report.TransportAttempts != 1 {
		t.Fatal("literal projection client flow lost", err)
	}
	encoded, err := service.EncodeResponse(out, q)
	if err != nil {
		t.Fatal(err)
	}
	sameJSON(t, encoded, literal.Response)
	for _, tc := range []struct {
		name   string
		mutate func(*service.IssueProjectionEnvelope)
	}{
		{"incomplete", func(p *service.IssueProjectionEnvelope) { *p.Inputs[0].Input.Complete = false }},
		{"missing-population", func(p *service.IssueProjectionEnvelope) { p.Inputs = []service.ReviewPopulation{} }},
		{"negative-population", func(p *service.IssueProjectionEnvelope) { *p.Inputs[0].Count = -1 }},
		{"stale-input", func(p *service.IssueProjectionEnvelope) { *p.Inputs[0].Input.Freshness = "stale" }},
		{"foreign-input", func(p *service.IssueProjectionEnvelope) { *p.Inputs[0].Repository.Host = "other.example" }},
		{"backward-collection", func(p *service.IssueProjectionEnvelope) { *p.Collection.End = p.Collection.Start.Add(-time.Second) }},
		{"future-input", func(p *service.IssueProjectionEnvelope) {
			*p.Inputs[0].Input.ObservedAt = p.Collection.End.Add(time.Second)
		}},
		{"missing-false", func(p *service.IssueProjectionEnvelope) { p.Rows[0].EscalationUnverifiable = nil }},
		{"duplicate-row", func(p *service.IssueProjectionEnvelope) { p.Rows[1].Number = p.Rows[0].Number }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var bad service.Response
			if err := json.Unmarshal(literal.Response, &bad); err != nil {
				t.Fatal(err)
			}
			tc.mutate(bad.Projection)
			raw, err := json.Marshal(bad)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.DecodeResponse(raw, q); err == nil {
				t.Fatal("unqualified projection accepted")
			}
		})
	}
}
