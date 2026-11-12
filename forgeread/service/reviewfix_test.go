package service_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/medici-finance/assay/forgeread"
	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/service"
)

func progressFixture(t *testing.T, op service.Operation, provider mirror.Provider, kind mirror.RecordKind, state service.State) (service.Request, service.Response) {
	t.Helper()
	q := literalRequest(t)
	r := literalResponse(t, q)
	q.Operation, q.Source = op, nil
	q.Repository.Provider = provider
	q.Scope.Repositories = []mirror.RepositoryID{q.Repository}
	minimum := &service.Token{Scope: q.Scope, Repository: q.Repository, Kind: kind, Value: "want"}
	if op == service.MirrorQuery {
		q.Mirror = &service.MirrorRequest{Kind: kind, MaxAgeMS: 60000, Minimum: minimum}
	} else {
		q.Minimum = minimum
	}
	r.Operation, r.Repository, r.Scope, r.Source = q.Operation, q.Repository, q.Scope, nil
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
	} else if op == service.MirrorQuery {
		now := r.Metadata.Source.ObservedAt
		r.Mirror = &mirror.Snapshot{Scope: mirror.Scope{Repository: q.Repository, Kind: kind, AdmissionID: q.Scope.AdmissionID}, Coverage: mirror.Coverage{State: mirror.Complete, LastAttempted: now, LastComplete: now, LastInventory: now}, Version: "fixture", Records: []mirror.Observation{}}
	}
	return q, r
}

func checkObservationKinds(t *testing.T) {
	for _, op := range []service.Operation{service.ReadObservation, service.MirrorQuery} {
		for _, provider := range []mirror.Provider{mirror.GitHub, mirror.GitLab} {
			for _, kind := range []mirror.RecordKind{mirror.Issues, mirror.Changes} {
				for _, state := range []service.State{service.Confirmed, service.Newer, service.Older} {
					t.Run(fmt.Sprintf("%s/%s/%s/%s", op, provider, kind, state), func(t *testing.T) {
						q, r := progressFixture(t, op, provider, kind, state)
						wire, err := service.EncodeResponse(r, q)
						if err != nil {
							t.Fatal("valid progress baseline refused", err)
						}
						got, err := service.DecodeResponse(wire, q)
						if err != nil || got.Metadata.Observation.Kind != kind || got.Metadata.State != state {
							t.Fatal("valid progress baseline changed", err)
						}
						other := mirror.Changes
						if kind == mirror.Changes {
							other = mirror.Issues
						}
						r.Metadata.Observation.Kind = other
						if b, err := service.EncodeResponse(r, q); !errors.Is(err, service.ErrScope) || len(b) != 0 {
							t.Error("foreign record kind encoded", err)
						}
						// Independent malformed server wire bypasses the encoder under test.
						raw, err := json.Marshal(r)
						if err != nil {
							t.Fatal(err)
						}
						if _, err := service.DecodeResponse(raw, q); !errors.Is(err, service.ErrScope) {
							t.Fatal("foreign record kind decoded", err)
						}
					})
				}
			}
		}
	}
}

func reviewFixture(t *testing.T) (service.Request, service.Response) {
	t.Helper()
	var cases []struct {
		Observation json.RawMessage `json:"observation"`
	}
	if err := json.Unmarshal(fixture(t, "review-observations.json"), &cases); err != nil || len(cases) == 0 {
		t.Fatal(err)
	}
	var observation service.ReviewEnvelope
	if err := json.Unmarshal(cases[0].Observation, &observation); err != nil {
		t.Fatal(err)
	}
	q := literalRequest(t)
	r := literalResponse(t, q)
	q.Operation, q.Source, q.Repository = service.ReviewObservation, nil, mirror.RepositoryID{}
	q.Scope.Repositories = nil
	for _, repo := range observation.Inputs.Requested {
		q.Scope.Repositories = append(q.Scope.Repositories, mirror.RepositoryID{Provider: mirror.Provider(*repo.Provider), Host: *repo.Host, Path: *repo.Repository})
	}
	r.Operation, r.Source, r.Repository, r.Scope, r.Review = q.Operation, nil, q.Repository, q.Scope, &observation
	return q, r
}

func checkWireLimits(t *testing.T) {
	for _, count := range []int{service.MaxItems, service.MaxItems + 1} {
		t.Run(fmt.Sprintf("legacy-labels/%d", count), func(t *testing.T) {
			q := literalRequest(t)
			r := literalResponse(t, q)
			q.Operation, q.Source = service.MirrorQuery, nil
			q.Mirror = &service.MirrorRequest{Kind: mirror.Issues, MaxAgeMS: 60000}
			r.Operation, r.Source = q.Operation, nil
			labels := make([]string, count)
			for i := range labels {
				labels[i] = "fixture"
			}
			r.Issues = &forgeread.IssuesEnvelope{Schema: forgeread.EnvelopeSchema, Kind: forgeread.IssuesKind, Partial: []forgeread.Unavailable{}, Repos: []forgeread.RepoIssues{{Repo: q.Repository.Path, Issues: []forgeread.Issue{{Number: 1, State: "open", Labels: labels}}}}}
			wire, err := service.EncodeResponse(r, q)
			if count > service.MaxItems {
				if err == nil || len(wire) != 0 {
					t.Fatal("oversized nested label array encoded")
				}
				return
			}
			got, e := service.DecodeResponse(wire, q)
			if err != nil || e != nil || len(got.Issues.Repos[0].Issues[0].Labels) != count {
				t.Fatal("bounded labels changed", err, e)
			}
		})
		t.Run(fmt.Sprintf("review-map/%d", count), func(t *testing.T) {
			q, r := reviewFixture(t)
			for len(r.Review.Result.Depths.Actions) < count {
				zero := 0
				r.Review.Result.Depths.Actions[fmt.Sprintf("fixture-%d", len(r.Review.Result.Depths.Actions))] = &zero
			}
			wire, err := service.EncodeResponse(r, q)
			if count > service.MaxItems {
				if err == nil || len(wire) != 0 {
					t.Fatal("oversized nested map encoded")
				}
				return
			}
			got, e := service.DecodeResponse(wire, q)
			if err != nil || e != nil || len(got.Review.Result.Depths.Actions) != count {
				t.Fatal("bounded map changed", err, e)
			}
		})
	}
	// Byte content is a single base64 scalar on this wire, not a JSON array.
	q := literalRequest(t)
	r := literalResponse(t, q)
	r.Source.Content = []byte(strings.Repeat("x", service.MaxItems+1))
	wire, err := service.EncodeResponse(r, q)
	if err != nil {
		t.Fatal("opaque bytes were array-capped", err)
	}
	got, err := service.DecodeResponse(wire, q)
	if err != nil || len(got.Source.Content) != service.MaxItems+1 {
		t.Fatal("opaque bytes changed", err)
	}
}

func checkEffectStates(t *testing.T) {
	var cases []struct {
		Request  service.Request  `json:"request"`
		Response service.Response `json:"response"`
	}
	if err := json.Unmarshal(fixture(t, "operation-cases.json"), &cases); err != nil {
		t.Fatal(err)
	}
	var q service.Request
	var base service.Response
	for _, c := range cases {
		if c.Request.Operation == service.CreateIssue {
			q, base = c.Request, c.Response
			break
		}
	}
	if base.Effect == nil {
		t.Fatal("producer effect fixture missing")
	}
	for _, outcome := range []service.ProviderOutcome{service.Succeeded, service.Unknown, service.Failed} {
		for _, projection := range []service.ProjectionState{service.Current, service.Pending, service.NoProjection} {
			for _, state := range []service.State{service.Confirmed, service.ProjectionPending} {
				t.Run(fmt.Sprintf("%s/%s/%s", outcome, projection, state), func(t *testing.T) {
					r := base
					e := *base.Effect
					r.Effect = &e
					e.Outcome, e.Projection = outcome, projection
					r.Metadata.State = state
					if outcome != service.Succeeded {
						e.Object, e.Provenance = nil, nil
					}
					if state == service.ProjectionPending {
						no := false
						r.Metadata.Complete = &no
						r.Metadata.Freshness = "unknown"
					}
					allowed := false
					if outcome == service.Succeeded {
						allowed = (projection == service.Pending) == (state == service.ProjectionPending)
					} else {
						allowed = projection != service.Current && (projection != service.Pending || state == service.ProjectionPending)
					}
					wire, err := service.EncodeResponse(r, q)
					raw, e2 := json.Marshal(r)
					if e2 != nil {
						t.Fatal(e2)
					}
					_, decodedErr := service.DecodeResponse(raw, q)
					if allowed {
						if err != nil || decodedErr != nil {
							t.Fatal("retained effect state refused", err, decodedErr)
						}
					} else if err == nil || len(wire) != 0 || decodedErr == nil {
						t.Fatal("inconsistent pending state accepted", err, decodedErr)
					}
				})
			}
		}
	}
}
