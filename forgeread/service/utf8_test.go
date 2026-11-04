package service_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/service"
)

// Wire strings must round-trip exactly; opaque source bytes remain base64 data.
func checkWireUTF8(t *testing.T) {
	bad := string([]byte{255})
	unicode := "café 日本語 �"
	for _, provider := range []mirror.Provider{mirror.GitHub, mirror.GitLab} {
		t.Run(string(provider), func(t *testing.T) {
			q := literalRequest(t)
			q.Operation, q.Source = service.MirrorQuery, nil
			q.Repository.Provider = provider
			q.Scope.Repositories = []mirror.RepositoryID{q.Repository}
			q.Mirror = &service.MirrorRequest{Kind: mirror.Changes, MaxAgeMS: 60000}
			r := literalResponse(t, literalRequest(t))
			r.Operation, r.Repository, r.Scope, r.Source = q.Operation, q.Repository, q.Scope, nil
			now := r.Metadata.Source.ObservedAt
			actor := mirror.Actor{ID: unicode, Login: unicode, Type: unicode}
			rowKind := "pull-request"
			reviews := mirror.ReviewObservations{State: mirror.Complete, GitHub: []mirror.GitHubReview{{ID: unicode, Author: actor, State: unicode, Commit: unicode, SubmittedAt: now}}}
			if provider == mirror.GitLab {
				rowKind = "merge-request"
				reviews = mirror.ReviewObservations{State: mirror.Complete, GitLab: []mirror.GitLabApproval{{User: actor, Approved: true, ObservedAt: now}}}
			}
			r.Metadata.Executor = &actor
			r.Mirror = &mirror.Snapshot{Scope: mirror.Scope{Repository: q.Repository, Kind: mirror.Changes, AdmissionID: q.Scope.AdmissionID},
				Coverage: mirror.Coverage{State: mirror.Complete, Validator: unicode, Revision: unicode, LastAttempted: now, LastComplete: now, LastInventory: now}, Version: unicode,
				Records: []mirror.Observation{{ID: mirror.RecordID{Repository: q.Repository, Kind: rowKind, Number: 1}, State: unicode, ClosureReason: unicode, HeadRevision: unicode, Title: unicode, Body: unicode, BodyKnown: true, Labels: []string{unicode}, Author: actor, RequestedReviewers: []mirror.Actor{actor}, UpdatedAt: now, ObservedAt: now, Reviews: reviews, InInventory: true}}}
			wire, err := service.EncodeResponse(r, q)
			if err != nil {
				t.Fatal("valid Unicode refused", err)
			}
			out, err := service.DecodeResponse(wire, q)
			if err != nil || !reflect.DeepEqual(out, r) {
				t.Fatal("valid Unicode changed", err)
			}
			leaves := wireStringLeaves(reflect.ValueOf(&r).Elem(), "response")
			if len(leaves) < 40 {
				t.Fatal("nested fixture lost string coverage", len(leaves))
			}
			for name := range leaves {
				t.Run(name, func(t *testing.T) {
					copy, err := service.DecodeResponse(wire, q)
					if err != nil {
						t.Fatal(err)
					}
					target := wireStringLeaves(reflect.ValueOf(&copy).Elem(), "response")[name]
					target.SetString(bad)
					if encoded, err := service.EncodeResponse(copy, q); err == nil || len(encoded) != 0 {
						t.Fatal("invalid UTF-8 silently encoded", name)
					}
					if target.String() != bad {
						t.Fatal("caller string was normalized", name)
					}
				})
			}
		})
	}
	t.Run("cached-coverage", func(t *testing.T) {
		q, r := cachedRead(t, 2*time.Minute, service.Unavailable)
		for _, field := range []string{"Validator", "Revision"} {
			t.Run(field, func(t *testing.T) {
				copy := r
				cached := *r.Metadata.Cached
				copy.Metadata.Cached = &cached
				reflect.ValueOf(&cached.Coverage).Elem().FieldByName(field).SetString(bad)
				if b, err := service.EncodeResponse(copy, q); err == nil || len(b) != 0 {
					t.Fatal("invalid cached coverage encoded")
				}
			})
		}
	})
	t.Run("opaque-content", func(t *testing.T) {
		q := literalRequest(t)
		r := literalResponse(t, q)
		r.Source.Content = []byte{0, 255, 254}
		wire, err := service.EncodeResponse(r, q)
		if err != nil {
			t.Fatal("opaque source content refused", err)
		}
		out, err := service.DecodeResponse(wire, q)
		if err != nil || !reflect.DeepEqual(out.Source.Content, r.Source.Content) {
			t.Fatal("opaque bytes changed", err)
		}
	})
	t.Run("review-map-key", func(t *testing.T) {
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
		if _, err := service.EncodeResponse(r, q); err != nil {
			t.Fatal("producer review refused", err)
		}
		count := 0
		observation.Result.Depths.Actions[bad] = &count
		if b, err := service.EncodeResponse(r, q); err == nil || len(b) != 0 {
			t.Fatal("invalid map key silently encoded")
		}
	})
}

// Enumerate actual exported DTO string leaves, independently of codec validation.
// time.Time internals and opaque []byte have no exported string leaves.
func wireStringLeaves(v reflect.Value, path string) map[string]reflect.Value {
	leaves := map[string]reflect.Value{}
	var visit func(reflect.Value, string)
	visit = func(v reflect.Value, name string) {
		if !v.IsValid() {
			return
		}
		switch v.Kind() {
		case reflect.Pointer, reflect.Interface:
			if !v.IsNil() {
				visit(v.Elem(), name)
			}
		case reflect.String:
			leaves[name] = v
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				if v.Type().Field(i).PkgPath == "" {
					visit(v.Field(i), name+"."+v.Type().Field(i).Name)
				}
			}
		case reflect.Slice, reflect.Array:
			for i := 0; i < v.Len(); i++ {
				visit(v.Index(i), fmt.Sprintf("%s[%d]", name, i))
			}
		}
	}
	visit(v, path)
	return leaves
}
