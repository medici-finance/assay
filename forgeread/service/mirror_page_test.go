package service_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/service"
)

func pageResponseFixture(t *testing.T) (service.MirrorPageRequest, service.MirrorPageResponse) {
	t.Helper()
	q := literalRequest(t)
	q.Operation, q.Source = service.MirrorQuery, nil
	q.Mirror = &service.MirrorRequest{Kind: mirror.Changes, MaxAgeMS: 60_000}
	req := service.MirrorPageRequest{Schema: service.MirrorPageSchema, Query: q, Limit: 2}
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	snapshot := mirror.Snapshot{
		Scope:    mirror.Scope{Repository: q.Repository, Kind: q.Mirror.Kind, AdmissionID: q.Scope.AdmissionID},
		Coverage: mirror.Coverage{State: mirror.Complete, LastAttempted: now, LastComplete: now, LastInventory: now},
		Version:  "fixture-version",
	}
	for _, n := range []int64{2, 3} {
		snapshot.Records = append(snapshot.Records, mirror.Observation{
			ID:    mirror.RecordID{Repository: q.Repository, Kind: "pull-request", Number: n},
			Title: "fixture row", UpdatedAt: now, ObservedAt: now,
		})
	}
	complete := true
	resp := service.MirrorPageResponse{
		Schema: service.MirrorPageSchema, ID: q.ID, Scope: q.Scope, Repository: q.Repository, Kind: q.Mirror.Kind,
		Metadata: service.Metadata{State: service.Confirmed, Complete: &complete, Freshness: "fresh",
			Source: &service.Provenance{Source: "fixture-store", Version: "1", Revision: snapshot.Version, ObservedAt: now}},
		Page: &service.MirrorPage{Snapshot: snapshot, Next: &service.MirrorPageCursor{
			Scope: q.Scope, Repository: q.Repository, Kind: q.Mirror.Kind, Version: snapshot.Version, ObservedAt: now, After: 3,
		}},
	}
	return req, resp
}

func TestMirrorPageDecoderRejectsUnboundResponse(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*service.MirrorPageRequest, *service.MirrorPageResponse)
		err    error
	}{
		{"descending rows", func(_ *service.MirrorPageRequest, r *service.MirrorPageResponse) {
			rows := r.Page.Snapshot.Records
			rows[0], rows[1] = rows[1], rows[0]
			r.Page.Next.After = rows[1].ID.Number
		}, service.ErrInvalid},
		{"cursor key", func(_ *service.MirrorPageRequest, r *service.MirrorPageResponse) {
			r.Page.Next.After++
		}, service.ErrScope},
		{"cursor version", func(_ *service.MirrorPageRequest, r *service.MirrorPageResponse) {
			r.Page.Next.Version = "different-version"
		}, service.ErrScope},
		{"cursor observation", func(_ *service.MirrorPageRequest, r *service.MirrorPageResponse) {
			r.Page.Next.ObservedAt = r.Page.Next.ObservedAt.Add(time.Second)
		}, service.ErrScope},
		{"unsolicited restart", func(q *service.MirrorPageRequest, r *service.MirrorPageResponse) {
			r.Page = nil
			r.Metadata = service.Held(q.Query, mirror.Unknown).Metadata
			r.Restart = true
		}, service.ErrInvalid},
		{"requested row limit", func(q *service.MirrorPageRequest, _ *service.MirrorPageResponse) {
			q.Limit = 1
		}, service.ErrInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, resp := pageResponseFixture(t)
			valid, err := service.EncodeMirrorPageResponse(resp, req)
			if err != nil {
				t.Fatalf("valid control refused: %v", err)
			}
			decoded, err := service.DecodeMirrorPageResponse(valid, req)
			if err != nil || decoded.Page == nil || decoded.Page.Next == nil {
				t.Fatalf("valid page did not round trip: %v", err)
			}
			tc.change(&req, &resp)
			raw, err := json.Marshal(resp)
			if err != nil {
				t.Fatal(err)
			}
			result, err := service.DecodeMirrorPageResponse(raw, req)
			if !errors.Is(err, tc.err) || result.Page != nil {
				t.Fatalf("want %v and no released page, got %v", tc.err, err)
			}
		})
	}
}
