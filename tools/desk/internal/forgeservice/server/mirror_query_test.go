package server

import (
	"bytes"
	"crypto/ed25519"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/refresh"
	"github.com/medici-finance/assay/forgeread/service"
	"github.com/medici-finance/assay/tools/desk/internal/forgeservice/auth"
)

func mirrorRequest() service.Request {
	repo := mirror.RepositoryID{Provider: mirror.GitHub, Host: "github.com", Path: "example/project"}
	return service.Request{
		Schema: service.Schema, ID: "mirror-query-fixture", Operation: service.MirrorQuery, Repository: repo,
		Scope: service.Scope{
			Repositories: []mirror.RepositoryID{repo}, AdmissionID: "example-admission",
			AccessEpoch: "example-epoch", CallerID: "example-reader", RepresentedActor: "example-actor",
		},
		Mirror: &service.MirrorRequest{Kind: mirror.Changes, MaxAgeMS: 60_000},
	}
}

func TestMirrorQueryRejectsClaimsBeforeStore(t *testing.T) {
	q := mirrorRequest()
	handler := MirrorQueryHandler{policy: auth.GrantScope{
		Actor:  q.Scope.RepresentedActor,
		Scope:  mirror.Scope{Repository: q.Repository, AdmissionID: q.Scope.AdmissionID, Kind: q.Mirror.Kind},
		Caller: refresh.Caller{ID: q.Scope.CallerID, AccessEpoch: q.Scope.AccessEpoch, Limits: refresh.Limits{WaitTimeout: time.Second}},
	}, verifier: new(auth.Verifier)}
	for _, test := range []struct {
		name   string
		change func(*service.Request)
	}{
		{"caller", func(q *service.Request) { q.Scope.CallerID = "foreign-reader" }},
		{"actor", func(q *service.Request) { q.Scope.RepresentedActor = "foreign-actor" }},
		{"epoch", func(q *service.Request) { q.Scope.AccessEpoch = "foreign-epoch" }},
		{"kind", func(q *service.Request) { q.Mirror = &service.MirrorRequest{Kind: mirror.Issues, MaxAgeMS: 60_000} }},
		{"missing grant", func(*service.Request) {}},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := q
			test.change(&request)
			raw, err := service.EncodeRequest(request)
			if err != nil {
				t.Fatal(err)
			}
			httpRequest := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(raw))
			httpRequest.Header.Set("Content-Type", "application/json")
			var grant auth.SignedGrant
			if test.name != "missing grant" {
				grant = auth.SignedGrant{Payload: []byte("fixture"), Signature: make([]byte, ed25519.SignatureSize)}
			}
			data, err := handler.Handle(httpRequest, grant)
			if err != nil {
				t.Fatal(err)
			}
			got, err := service.DecodeResponse(data, request)
			if err != nil || got.Metadata.State != service.Permission || got.Metadata.Cached != nil ||
				got.Metadata.RetryAt != nil || got.Metadata.Source != nil || got.Mirror != nil {
				t.Fatalf("expected metadata-free refusal: %v, %#v", err, got.Metadata)
			}
		})
	}
}

func TestMirrorReplyPreservesWireCeilingAndObservationTime(t *testing.T) {
	q := mirrorRequest()
	now := time.Date(2026, 1, 1, 0, 1, 0, 0, time.UTC)
	stamp := now.Add(-time.Second)
	snapshot := mirror.Snapshot{
		Scope: mirror.Scope{Repository: q.Repository, AdmissionID: q.Scope.AdmissionID, Kind: q.Mirror.Kind},
		Coverage: mirror.Coverage{
			State: mirror.Complete, LastAttempted: stamp, LastComplete: stamp, LastInventory: stamp, ObservedUpTo: stamp,
		},
		Version: "observed-version",
		Records: []mirror.Observation{},
	}
	result := refresh.Result{Result: mirror.Result{Snapshot: snapshot, Available: true}, Fulfilled: true}
	got := mirrorReply(q, result, true, now)
	if got.Mirror == nil || got.Metadata.State != service.Confirmed || got.Metadata.Source == nil ||
		!got.Metadata.Source.ObservedAt.Equal(stamp) || got.Metadata.Observation != nil {
		t.Fatalf("expected an empty, complete observation at its original time: %#v", got.Metadata)
	}
	if _, err := service.EncodeResponse(got, q); err != nil {
		t.Fatal(err)
	}

	for n := 1; n <= service.MaxItems+1; n++ {
		result.Snapshot.Records = append(result.Snapshot.Records, mirror.Observation{
			ID:    mirror.RecordID{Repository: q.Repository, Kind: "pull-request", Number: int64(n)},
			State: "open", HeadRevision: strings.Repeat("a", 40), Title: "fixture",
			UpdatedAt: stamp, ObservedAt: stamp, Reviews: mirror.ReviewObservations{State: mirror.NeverRead},
		})
	}
	got = mirrorReply(q, result, true, now)
	if got.Metadata.State != service.Unavailable || got.Metadata.Reason != mirror.Truncated || got.Mirror != nil || got.Metadata.Cached == nil {
		t.Fatalf("over-cap snapshot was not held with its coverage: %#v", got.Metadata)
	}
	if _, err := service.EncodeResponse(got, q); err != nil {
		t.Fatal(err)
	}

	result.Reason, result.RetryAt = mirror.Permission, now.Add(time.Minute)
	got = mirrorReply(q, result, false, now)
	if got.Metadata.State != service.Permission || got.Metadata.RetryAt != nil || got.Metadata.Cached != nil || got.Mirror != nil {
		t.Fatalf("permission response leaked state: %#v", got.Metadata)
	}
}
