package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/forgeread"
	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/mirror/mirrortest"
	"github.com/medici-finance/assay/forgeread/service"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "testdata", "service", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func literalRequest(t *testing.T) service.Request {
	t.Helper()
	q, err := service.DecodeRequest(fixture(t, "request-source.json"))
	if err != nil {
		t.Fatal(err)
	}
	return q
}
func literalResponse(t *testing.T, q service.Request) service.Response {
	t.Helper()
	r, err := service.DecodeResponse(fixture(t, "response-source.json"), q)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func sameJSON(t *testing.T, a, b []byte) {
	t.Helper()
	var x, y any
	dx, dy := json.NewDecoder(bytes.NewReader(a)), json.NewDecoder(bytes.NewReader(b))
	dx.UseNumber()
	dy.UseNumber()
	if dx.Decode(&x) != nil || dy.Decode(&y) != nil || !reflect.DeepEqual(x, y) {
		t.Fatal("literal wire value changed")
	}
}

// regression: class service-wire
func TestForgeServiceContractCompatibility(t *testing.T) {
	t.Run("canonical-mirror-kinds", checkCanonicalMirrorKinds)
	t.Run("literal-operations", checkLiteralOperations)
	t.Run("response-refusals", checkResponseRefusals)
	q := literalRequest(t)
	if q.Operation != service.FileAt || q.Repository.Provider != mirror.GitHub || q.Repository.Host != "github.example" || q.Repository.Path != "example/project" || q.Source.Revision != strings.Repeat("a", 40) || q.Source.Path != "readme.txt" {
		t.Fatal("literal request identity/source changed")
	}
	b, err := service.EncodeRequest(q)
	if err != nil {
		t.Fatal(err)
	}
	sameJSON(t, b, fixture(t, "request-source.json"))
	r := literalResponse(t, q)
	if string(r.Source.Content) != "fixture\n" || r.Metadata.State != service.Confirmed || r.Metadata.Complete == nil || !*r.Metadata.Complete || r.Scope.AccessEpoch != "epoch-1" {
		t.Fatal("literal result/provenance changed")
	}
	b, err = service.EncodeResponse(r, q)
	if err != nil {
		t.Fatal(err)
	}
	sameJSON(t, b, fixture(t, "response-source.json"))
	t.Run("closed", func(t *testing.T) {
		raw := string(fixture(t, "request-source.json"))
		for name, bad := range map[string]string{
			"missing-default-scalar": strings.Replace(raw, `,
    "representedActor": ""`, "", 1),
			"missing-nullable":     strings.Replace(raw, `"effect": null,`, "", 1),
			"null-default-scalar":  strings.Replace(raw, `"representedActor": ""`, `"representedActor": null`, 1),
			"null-optional-source": strings.Replace(raw, `"other": ""`, `"other": null`, 1),
			"schema":               strings.Replace(raw, "forge-service-v1", "forge-service-v9", 1),
			"operation":            strings.Replace(raw, "file-at", "arbitrary-command", 1),
			"role":                 strings.Replace(raw, `"operation":`, `"role":"admin","operation":`, 1),
			"url":                  strings.Replace(raw, `"operation":`, `"url":"https://other.example/","operation":`, 1),
			"duplicate":            strings.Replace(raw, `"id":`, `"id":"other","id":`, 1),
			"case":                 strings.Replace(raw, `"schema":`, `"Schema":`, 1),
			"trailing":             raw + "{}",
			"bound":                strings.Repeat(" ", service.MaxMessageBytes) + raw,
		} {
			t.Run(name, func(t *testing.T) {
				if _, err := service.DecodeRequest([]byte(bad)); err == nil {
					t.Fatal("invalid request accepted")
				}
			})
		}
	})
	t.Run("token", func(t *testing.T) {
		q.Operation, q.Source = service.ReadObservation, nil
		q.Minimum = &service.Token{Scope: q.Scope, Repository: q.Repository, Kind: mirror.Issues, Value: "required-observation"}
		r.Operation, r.Source = q.Operation, nil
		r.Metadata.Observation = &service.Token{Scope: q.Scope, Repository: q.Repository, Kind: mirror.Issues, Value: "other-observation"}
		// This is otherwise structurally valid v1. A different confirmed token
		// must not satisfy a minimum-progress read.
		wrong, _ := json.Marshal(r)
		if _, err := service.DecodeResponse(wrong, q); !errors.Is(err, service.ErrScope) {
			t.Fatal("wrong observation token accepted", err)
		}
		r.Metadata.Observation.Value = q.Minimum.Value
		if _, err := service.EncodeResponse(r, q); err != nil {
			t.Fatal("matching token refused", err)
		}
		r.Metadata.Observation.Scope.AccessEpoch = "other-epoch"
		wrong, _ = json.Marshal(r)
		if _, err := service.DecodeResponse(wrong, q); err == nil {
			t.Fatal("cross-epoch observation accepted")
		}
	})
	t.Run("legacy", func(t *testing.T) {
		res, err := forgeread.ParseIssuesEnvelope(fixture(t, "legacy-issues.json"), []string{"example/project"})
		if err != nil || len(res.Unavailable) != 0 || res.Issues["example/project"] == nil || len(res.Issues["example/project"]) != 0 {
			t.Fatal("known-empty legacy result changed", err)
		}
		frozen, err := forgeread.NewFrozenReader(res).OpenIssues(context.Background(), []string{"example/project", "example/missing"})
		if err != nil || len(frozen.Issues) != 1 || len(frozen.Unavailable) != 1 || frozen.Unavailable[0].Class != forgeread.ClassMissing {
			t.Fatal("legacy frozen missing became empty", err)
		}
		q := literalRequest(t)
		r := literalResponse(t, q)
		q.Operation = service.MirrorQuery
		q.Source = nil
		q.Mirror = &service.MirrorRequest{Kind: mirror.Issues, MaxAgeMS: 60000}
		r.Operation = q.Operation
		r.Source = nil
		r.Issues = &forgeread.IssuesEnvelope{Schema: forgeread.EnvelopeSchema, Kind: forgeread.IssuesKind, Partial: []forgeread.Unavailable{}, Repos: []forgeread.RepoIssues{{Repo: q.Repository.Path, Issues: []forgeread.Issue{{Number: 1, State: "open"}}}}}
		wire, e := service.EncodeResponse(r, q)
		if e != nil {
			t.Fatal("minimal legacy row refused", e)
		}
		got, e := service.DecodeResponse(wire, q)
		if e != nil || got.Issues.Repos[0].Issues[0].Number != 1 || got.Issues.Repos[0].Issues[0].AuthorLogin != "" {
			t.Fatal("legacy omitempty behavior changed", e)
		}
		if _, err := forgeread.ParseIssuesEnvelope([]byte(`{"schema":999,"kind":"issues","repos":[],"partial":[]}`), nil); !errors.Is(err, forgeread.ErrUnsupportedSchema) {
			t.Fatal("unknown legacy schema accepted")
		}
	})
	t.Run("held", func(t *testing.T) {
		q := literalRequest(t)
		for _, reason := range []mirror.Reason{mirror.Permission, mirror.Transport, mirror.RateLimited, mirror.Truncated, mirror.Unknown} {
			h := service.Held(q, reason)
			if _, err := service.EncodeResponse(h, q); err != nil {
				t.Fatal("explicit hold refused", err)
			}
			h.Source = literalResponse(t, q).Source
			if _, err := service.EncodeResponse(h, q); err == nil {
				t.Fatal("held result leaked private data")
			}
		}
	})
}

type wireAdmission struct{ name string }

func (a wireAdmission) ID() string                                           { return a.name }
func (a wireAdmission) Authorize(context.Context, mirror.RepositoryID) error { return nil }
func checkCanonicalMirrorKinds(t *testing.T) {
	for _, provider := range []mirror.Provider{mirror.GitHub, mirror.GitLab} {
		for _, kind := range []mirror.RecordKind{mirror.Issues, mirror.Changes} {
			for _, empty := range []bool{false, true} {
				t.Run(string(provider)+"/"+string(kind)+"/"+map[bool]string{false: "nonempty", true: "empty"}[empty], func(t *testing.T) {
					q := literalRequest(t)
					r := literalResponse(t, q)
					q.Operation = service.MirrorQuery
					q.Source = nil
					q.Repository.Provider = provider
					q.Scope.Repositories = []mirror.RepositoryID{q.Repository}
					q.Mirror = &service.MirrorRequest{Kind: kind, MaxAgeMS: 60000}
					scope := mirror.Scope{Repository: q.Repository, Kind: kind, AdmissionID: q.Scope.AdmissionID}
					now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
					rowKind := "issue"
					if kind == mirror.Changes {
						if provider == mirror.GitHub {
							rowKind = "pull-request"
						} else {
							rowKind = "merge-request"
						}
					}
					records := []mirror.Observation{}
					if !empty {
						records = append(records, mirror.Observation{ID: mirror.RecordID{Repository: q.Repository, Kind: rowKind, Number: 1}, Title: "canonical", InInventory: true})
					}
					plan, err := mirror.PlanRefresh(mirror.Snapshot{Scope: scope, Coverage: mirror.Coverage{State: mirror.NeverRead}}, now, mirror.RefreshPolicy{FullInventoryInterval: time.Hour})
					if err != nil {
						t.Fatal(err)
					}
					update, err := mirror.PlanScan(mirror.Snapshot{Scope: scope, Coverage: mirror.Coverage{State: mirror.NeverRead}}, plan, mirror.ScanResult{Complete: true, ObservedUpTo: now, Records: records}, now)
					if err != nil {
						t.Fatal(err)
					}
					snapshot, err := mirrortest.NewStore().Apply(context.Background(), wireAdmission{q.Scope.AdmissionID}, update)
					if err != nil {
						t.Fatal(err)
					}
					r.Operation = q.Operation
					r.Scope = q.Scope
					r.Repository = q.Repository
					r.Source = nil
					r.Mirror = &snapshot
					encoded, err := service.EncodeResponse(r, q)
					if err != nil {
						t.Fatalf("canonical producer snapshot rejected: %v", err)
					}
					decoded, err := service.DecodeResponse(encoded, q)
					if err != nil || !reflect.DeepEqual(decoded.Mirror, &snapshot) {
						t.Fatalf("canonical mirror transport changed: %v", err)
					}
					if !empty {
						if decoded.Mirror.Records[0].BodyKnown || decoded.Mirror.Records[0].Labels != nil {
							t.Fatal("unknown body/collection became known empty")
						}
						known := mirror.CloneSnapshot(snapshot)
						known.Records[0].BodyKnown = true
						known.Records[0].Labels = []string{}
						r.Mirror = &known
						wire, e := service.EncodeResponse(r, q)
						if e != nil {
							t.Fatal(e)
						}
						withEmpty, e := service.DecodeResponse(wire, q)
						if e != nil || !withEmpty.Mirror.Records[0].BodyKnown || withEmpty.Mirror.Records[0].Body != "" || withEmpty.Mirror.Records[0].Labels == nil || len(withEmpty.Mirror.Records[0].Labels) != 0 {
							t.Fatal("known-empty body/collection lost", e)
						}
						for _, badKind := range []string{"change", "issue", "pull-request", "merge-request"} {
							if badKind == rowKind {
								continue
							}
							bad := snapshot
							bad.Records = append([]mirror.Observation(nil), snapshot.Records...)
							bad.Records[0].ID.Kind = badKind
							r.Mirror = &bad
							if _, err := service.EncodeResponse(r, q); err == nil {
								t.Fatalf("foreign provider/object kind accepted: %s", badKind)
							}
						}
					}
				})
			}
		}
	}
}

type operationCase struct {
	Case     string          `json:"case"`
	Request  json.RawMessage `json:"request"`
	Response json.RawMessage `json:"response"`
}

func operationCases(t *testing.T) []operationCase {
	t.Helper()
	var cases []operationCase
	if err := json.Unmarshal(fixture(t, "operation-cases.json"), &cases); err != nil || len(cases) != 8 {
		t.Fatal("independent operation literals missing", err)
	}
	return cases
}
func checkLiteralOperations(t *testing.T) {
	for _, tc := range operationCases(t) {
		t.Run(tc.Case, func(t *testing.T) {
			q, err := service.DecodeRequest(tc.Request)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := service.EncodeRequest(q)
			if err != nil {
				t.Fatal(err)
			}
			sameJSON(t, encoded, tc.Request)
			r, err := service.DecodeResponse(tc.Response, q)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err = service.EncodeResponse(r, q)
			if err != nil {
				t.Fatal(err)
			}
			sameJSON(t, encoded, tc.Response)
			if q.Effect != nil {
				r.Effect.ID.PayloadDigest = strings.Repeat("f", 64)
				if _, err := service.EncodeResponse(r, q); !errors.Is(err, service.ErrConflict) {
					t.Fatal("changed payload masqueraded as same operation", err)
				}
			}
		})
	}
}

func checkResponseRefusals(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*service.Response)
	}{
		{"source-revision", func(r *service.Response) { r.Source.Revision = strings.Repeat("b", 40) }},
		{"source-path", func(r *service.Response) { r.Source.Path = "other.txt" }},
		{"request-id", func(r *service.Response) { r.ID = "other-request" }},
		{"access-epoch", func(r *service.Response) { r.Scope.AccessEpoch = "other-epoch" }},
		{"permission-payload", func(r *service.Response) {
			r.Metadata.State = service.Permission
			r.Metadata.Reason = mirror.Permission
		}},
		{"false-complete", func(r *service.Response) { f := false; r.Metadata.Complete = &f }},
		{"descendant-call-count", func(r *service.Response) {
			one := int64(1)
			r.Measurements = []service.Measurement{{Boundary: "client", Unit: "call", Count: &one}}
		}},
		{"unsolicited-link", func(r *service.Response) { r.Diagnostic = &service.DiagnosticLink{Parent: "parent", Link: "link"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := literalRequest(t)
			var r service.Response
			if e := json.Unmarshal(fixture(t, "response-source.json"), &r); e != nil {
				t.Fatal(e)
			}
			tc.mutate(&r)
			raw, e := json.Marshal(r)
			if e != nil {
				t.Fatal(e)
			}
			if _, e := service.DecodeResponse(raw, q); e == nil {
				t.Fatal("response contract refusal missing")
			}
		})
	}
	q := literalRequest(t)
	q.Source.Path = "../escape"
	if _, err := service.EncodeRequest(q); err == nil {
		t.Fatal("source path escaped repository")
	}
}
