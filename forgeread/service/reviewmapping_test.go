package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
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
	t.Run("schema-vocabulary", func(t *testing.T) { checkReviewSchemaVocabulary(t, requests[0], responses[0]) })
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

type reviewFixtureTrip func(*http.Request) (*http.Response, error)

func (f reviewFixtureTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func checkReviewSchemaVocabulary(t *testing.T, q service.Request, baseline []byte) {
	var schema struct {
		Defs map[string]struct {
			Properties map[string]json.RawMessage `json:"properties"`
		} `json:"$defs"`
	}
	raw, e := os.ReadFile(filepath.Join("..", "..", "schemas", "review-observation-v1.json"))
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(raw, &schema); e != nil {
		t.Fatal(e)
	}
	type constraint struct {
		Enum     []string `json:"enum"`
		Minimum  *int     `json:"minimum"`
		MinItems *int     `json:"minItems"`
		MaxItems *int     `json:"maxItems"`
		Pattern  string   `json:"pattern"`
	}
	get := func(def, field string) constraint {
		t.Helper()
		var c constraint
		b := schema.Defs[def].Properties[field]
		if len(b) == 0 {
			t.Fatal("canonical constraint missing", def, field)
		}
		if e := json.Unmarshal(b, &c); e != nil {
			t.Fatal(e)
		}
		return c
	}
	clone := func() service.Response {
		t.Helper()
		var r service.Response
		if e := json.Unmarshal(baseline, &r); e != nil {
			t.Fatal(e)
		}
		return r
	}
	attempt := func(t *testing.T, query service.Request, r service.Response, valid bool) {
		t.Helper()
		_, enc := service.EncodeResponse(r, query)
		wire, e := json.Marshal(r)
		if e != nil {
			t.Fatal(e)
		}
		_, dec := service.DecodeResponse(wire, query)
		c, e := client.New(client.Config{Endpoint: "https://fixture.example/forge", Scope: query.Scope, Timeout: time.Second, Transport: reviewFixtureTrip(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(bytes.NewReader(wire))}, nil
		})})
		if e != nil {
			t.Fatal(e)
		}
		out, report, ce := c.Do(context.Background(), query)
		if valid {
			if enc != nil || dec != nil || ce != nil || out.Review == nil {
				t.Fatalf("canonical structural value refused: encode=%v decode=%v client=%v", enc, dec, ce)
			}
		} else if enc == nil || dec == nil || ce == nil || out.Review != nil {
			t.Fatalf("schema-invalid review escaped: encode=%v decode=%v client=%v", enc, dec, ce)
		}
		if report.TransportAttempts != 1 {
			t.Fatal("fixture attempt count changed")
		}
	}
	for _, tc := range []struct {
		name, def, field string
		set              func(*service.ReviewEnvelope, string)
	}{
		{"action", "ObservationRow", "action", func(o *service.ReviewEnvelope, v string) { o.Result.Rows[0].Action = &v }},
		{"disposition", "ObservationRow", "disposition", func(o *service.ReviewEnvelope, v string) { o.Result.Rows[0].Disposition = &v }},
		{"lane-name", "LaneConclusion", "name", func(o *service.ReviewEnvelope, v string) { o.Inputs.Changes[0].Lanes[0].Name = &v }},
		{"lane-state", "LaneConclusion", "state", func(o *service.ReviewEnvelope, v string) { o.Inputs.Changes[0].Lanes[0].State = &v }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			values := get(tc.def, tc.field).Enum
			if len(values) == 0 {
				t.Fatal("canonical enum instrument did not look")
			}
			for _, v := range values {
				t.Run(v, func(t *testing.T) { r := clone(); tc.set(r.Review, v); attempt(t, q, r, true) })
			}
			t.Run("unsupported", func(t *testing.T) { r := clone(); tc.set(r.Review, "UNSUPPORTED-WIRE-VALUE"); attempt(t, q, r, false) })
		})
	}
	rank := get("ObservationRow", "rank").Minimum
	if rank == nil {
		t.Fatal("canonical rank minimum missing")
	}
	for _, valid := range []bool{true, false} {
		t.Run("rank-"+strconv.FormatBool(valid), func(t *testing.T) {
			r := clone()
			n := *rank
			if !valid {
				n--
			}
			r.Review.Result.Rows[0].Rank = &n
			attempt(t, q, r, valid)
		})
	}
	lanes := get("ObservedChange", "lanes")
	if lanes.MinItems == nil || lanes.MaxItems == nil {
		t.Fatal("canonical lane bounds missing")
	}
	for _, n := range []int{*lanes.MinItems - 1, *lanes.MinItems, *lanes.MaxItems + 1} {
		t.Run("lanes-"+strconv.Itoa(n), func(t *testing.T) {
			r := clone()
			prior := r.Review.Inputs.Changes[0].Lanes
			rows := make([]service.ReviewLaneConclusion, n)
			for i := range rows {
				rows[i] = prior[i%len(prior)]
			}
			r.Review.Inputs.Changes[0].Lanes = rows
			attempt(t, q, r, n >= *lanes.MinItems && n <= *lanes.MaxItems)
		})
	}
	for _, tc := range []struct {
		name string
		set  func(*service.ReviewEnvelope)
	}{
		{"unknown-evaluator", func(o *service.ReviewEnvelope) { v := "UNSUPPORTED-EVALUATOR"; o.Inputs.Evaluator = &v }},
		{"negative-population", func(o *service.ReviewEnvelope) { v := -1; o.Inputs.Population[0].Count = &v }},
		{"empty-provenance", func(o *service.ReviewEnvelope) { v := ""; o.Inputs.Population[0].Input.Version = &v }},
		{"negative-total", func(o *service.ReviewEnvelope) { v := -1; o.Result.Depths.Total = &v }},
		{"absent-rank", func(o *service.ReviewEnvelope) { o.Result.Rows[0].Rank = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) { r := clone(); tc.set(r.Review); attempt(t, q, r, false) })
	}
	t.Run("legacy-repository-pattern", func(t *testing.T) {
		pattern := get("Repository", "repository").Pattern
		re, e := regexp.Compile(pattern)
		if e != nil || pattern == "" || re.MatchString("Example/project") {
			t.Fatal("canonical pattern control invalid", e)
		}
		r := clone()
		query := q
		query.Scope.Repositories = append([]mirror.RepositoryID(nil), q.Scope.Repositories...)
		query.Scope.Repositories[0].Path = "Example/project"
		r.Scope = query.Scope
		set := func(repo *service.ReviewRepository) { v := "Example/project"; repo.Repository = &v }
		for i := range r.Review.Inputs.Requested {
			set(&r.Review.Inputs.Requested[i])
		}
		for i := range r.Review.Inputs.Admitted {
			set(&r.Review.Inputs.Admitted[i])
		}
		for i := range r.Review.Inputs.Population {
			set(&r.Review.Inputs.Population[i].Repository)
		}
		for i := range r.Review.Inputs.Changes {
			set(&r.Review.Inputs.Changes[i].Identity.Repository)
		}
		for i := range r.Review.Result.Rows {
			set(&r.Review.Result.Rows[i].Identity.Repository)
		}
		attempt(t, query, r, false)
	})
}
