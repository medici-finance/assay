package server

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/refresh"
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

type continuationFixture struct {
	t               *testing.T
	cursor          service.MirrorPageCursor
	page            store.MirrorPage
	readErr         error
	revoked         bool
	revokeOnRead    bool
	expectedRead    bool
	reads, releases int
}

func (f *continuationFixture) authorizeMirrorQuery(ctx context.Context, _ service.Request, _ auth.SignedGrant) (auth.SignedGrant, bool) {
	if ctx.Err() != nil || f.revoked {
		f.t.Fatal("fixture should admit at ingress")
	}
	return auth.SignedGrant{Payload: []byte("admitted-fixture")}, true
}

func (f *continuationFixture) readMirrorQuery(context.Context, auth.SignedGrant, time.Duration) (refresh.Result, error) {
	f.t.Fatal("continuation attempted a read-through query")
	return refresh.Result{}, nil
}

func (f *continuationFixture) readStoredPage(_ context.Context, version string, observed time.Time, after int64, maxRecords, maxBytes int) (store.MirrorPage, error) {
	f.reads++
	if !f.expectedRead || version != f.cursor.Version || !observed.Equal(f.cursor.ObservedAt) ||
		after != f.cursor.After || maxRecords != 2 || maxBytes != 4096 {
		f.t.Fatal("continuation reached storage with an unqualified key or limit")
	}
	f.revoked = f.revokeOnRead
	return f.page, f.readErr
}

func (f *continuationFixture) releaseMirrorQuery(ctx context.Context, grant auth.SignedGrant) bool {
	f.releases++
	if string(grant.Payload) != "admitted-fixture" {
		f.t.Fatal("release used a different proof from admission")
	}
	return ctx.Err() == nil && !f.revoked
}

func TestMirrorPageContinuationRestartAndRelease(t *testing.T) {
	for _, tc := range []struct {
		name        string
		stale       bool
		conflict    bool
		revoke      bool
		want        service.State
		wantRestart bool
	}{
		{name: "stale cursor", stale: true, want: service.Unavailable, wantRestart: true},
		{name: "snapshot conflict", conflict: true, want: service.Unavailable, wantRestart: true},
		{name: "current snapshot", want: service.Confirmed},
		{name: "revoked before page release", revoke: true, want: service.Permission},
		{name: "revoked before restart release", conflict: true, revoke: true, want: service.Permission},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := mirrorRequest()
			scope := mirror.Scope{Repository: q.Repository, AdmissionID: q.Scope.AdmissionID, Kind: q.Mirror.Kind}
			observed := time.Now().UTC().Add(-time.Second)
			if tc.stale {
				observed = observed.Add(-time.Hour)
			}
			cursor := service.MirrorPageCursor{Scope: q.Scope, Repository: q.Repository, Kind: q.Mirror.Kind,
				Version: "fixture-version", After: 4, ObservedAt: observed}
			request := service.MirrorPageRequest{Schema: service.MirrorPageSchema, Query: q, Limit: 10, Cursor: &cursor}
			page := store.MirrorPage{Snapshot: mirror.Snapshot{
				Scope: scope, Version: cursor.Version,
				Coverage: mirror.Coverage{State: mirror.Complete, LastAttempted: observed, LastComplete: observed},
				Records: []mirror.Observation{
					{ID: mirror.RecordID{Repository: q.Repository, Kind: "pull-request", Number: 5}, Title: "fixture row"},
				},
			}}
			f := &continuationFixture{t: t, cursor: cursor, page: page, revokeOnRead: tc.revoke, expectedRead: !tc.stale}
			if tc.conflict {
				f.readErr = mirror.ErrConflict
			}
			h := &MirrorQueryHandler{
				policy: auth.GrantScope{Scope: scope, Caller: refresh.Caller{Limits: refresh.Limits{
					WaitTimeout: time.Second, MinFreshness: time.Second, MaxFreshness: time.Minute,
				}}},
				limits: store.MirrorLimits{MaxRecords: 2, MaxBytes: 4096},
			}
			body, err := service.EncodeMirrorPageRequest(request)
			if err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
			wire, err := h.handlePage(r, auth.SignedGrant{}, f)
			if err != nil {
				t.Fatal(err)
			}
			got, err := service.DecodeMirrorPageResponse(wire, request)
			if err != nil || got.Metadata.State != tc.want || got.Restart != tc.wantRestart {
				t.Fatalf("want %s restart=%t, got %s restart=%t, err=%v", tc.want, tc.wantRestart, got.Metadata.State, got.Restart, err)
			}
			if f.releases != 1 || (tc.stale && f.reads != 0) || (!tc.stale && f.reads != 1) {
				t.Fatalf("unexpected storage/release counts: %d reads, %d releases", f.reads, f.releases)
			}
			if tc.want == service.Confirmed {
				if got.Page == nil || len(got.Page.Snapshot.Records) != 1 || got.Page.Snapshot.Records[0].ID.Number != 5 {
					t.Fatal("authorized continuation lost its row")
				}
				return
			}
			if got.Page != nil || got.Metadata.Cached != nil || got.Metadata.RetryAt != nil || got.Metadata.Source != nil || got.Metadata.Complete != nil {
				t.Fatal("continuation hold exposed snapshot data, provenance, or retry advice")
			}
		})
	}
}
