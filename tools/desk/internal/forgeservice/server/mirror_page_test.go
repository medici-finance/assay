package server

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/service"
	"github.com/medici-finance/assay/tools/desk/internal/forgeservice/auth"
	"github.com/medici-finance/assay/workflow/store"
)

func TestMirrorPageFitsWireBytes(t *testing.T) {
	q := mirrorRequest()
	request := service.MirrorPageRequest{Schema: service.MirrorPageSchema, Query: q, Limit: 5}
	observed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	scope := mirror.Scope{Repository: q.Repository, AdmissionID: q.Scope.AdmissionID, Kind: q.Mirror.Kind}
	snapshot := mirror.Snapshot{
		Scope: scope, Version: "fixture-version",
		Coverage: mirror.Coverage{State: mirror.Complete, LastAttempted: observed, LastComplete: observed},
	}
	for i := int64(1); i <= 5; i++ {
		snapshot.Records = append(snapshot.Records, mirror.Observation{
			ID:        mirror.RecordID{Repository: q.Repository, Kind: "pull-request", Number: i},
			BodyKnown: true, Body: strings.Repeat("x", service.MaxContentBytes), Title: "fixture",
		})
	}
	wire, err := encodeSnapshotPage(request, store.MirrorPage{Snapshot: snapshot})
	if err != nil || len(wire) > service.MaxMessageBytes {
		t.Fatalf("could not encode a bounded prefix: %v", err)
	}
	got, err := service.DecodeMirrorPageResponse(wire, request)
	if err != nil || got.Page == nil || len(got.Page.Snapshot.Records) != 3 ||
		got.Page.Next == nil || got.Page.Next.After != 3 || !got.Page.Next.ObservedAt.Equal(observed) {
		t.Fatalf("unexpected size-derived continuation: %v", err)
	}
	if _, err := service.DecodeResponse(wire, q); err == nil {
		t.Fatal("v1 decoder treated a page as the complete inventory")
	}
	request.Cursor = got.Page.Next
	snapshot.Records = snapshot.Records[3:]
	wire, err = encodeSnapshotPage(request, store.MirrorPage{Snapshot: snapshot})
	if err != nil {
		t.Fatal(err)
	}
	got, err = service.DecodeMirrorPageResponse(wire, request)
	if err != nil || got.Page == nil || len(got.Page.Snapshot.Records) != 2 || got.Page.Next != nil {
		t.Fatalf("expected a terminal page within the same snapshot: %v", err)
	}
	snapshot.Version = "different-version"
	if _, err := encodeSnapshotPage(request, store.MirrorPage{Snapshot: snapshot}); !errors.Is(err, service.ErrInvalid) {
		t.Fatal("encoded a page from a different snapshot", err)
	}
}

func TestMirrorPageDeniesBeforeSQL(t *testing.T) {
	q := mirrorRequest()
	request := service.MirrorPageRequest{Schema: service.MirrorPageSchema, Query: q, Limit: 50}
	data, err := service.EncodeMirrorPageRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(data))
	r.Header.Set("Content-Type", "application/json")
	handler := new(MirrorQueryHandler)
	data, err = handler.HandlePage(r, auth.SignedGrant{})
	if err != nil {
		t.Fatal(err)
	}
	response, err := service.DecodeMirrorPageResponse(data, request)
	if err != nil || response.Metadata.State != service.Permission || response.Page != nil ||
		response.Metadata.Cached != nil || response.Metadata.RetryAt != nil || response.Restart {
		t.Fatal("unadmitted pagination released state", err)
	}
}
