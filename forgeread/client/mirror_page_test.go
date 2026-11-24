package client_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/service"
)

func TestMirrorPageClientRestartDoesNotRetry(t *testing.T) {
	q, _, _ := literal(t)
	q.Operation, q.Source = service.MirrorQuery, nil
	q.Mirror = &service.MirrorRequest{Kind: mirror.Changes, MaxAgeMS: 60_000}
	cursor := &service.MirrorPageCursor{Scope: q.Scope, Repository: q.Repository, Kind: mirror.Changes,
		After: 256, Version: "snapshot-1", ObservedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	request := service.MirrorPageRequest{Schema: service.MirrorPageSchema, Query: q, Limit: 100, Cursor: cursor}
	calls := 0
	transport := trip(func(r *http.Request) (*http.Response, error) {
		calls++
		data, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		q, err := service.DecodeMirrorPageRequest(data)
		if err != nil || q.Cursor == nil || q.Cursor.After != 256 {
			t.Fatalf("bad page request: %v", err)
		}
		reply := service.MirrorPageResponse{Schema: service.MirrorPageSchema, ID: q.Query.ID, Scope: q.Query.Scope,
			Repository: q.Query.Repository, Kind: q.Query.Mirror.Kind, Restart: true,
			Metadata: service.Held(q.Query, mirror.Unknown).Metadata}
		wire, err := service.EncodeMirrorPageResponse(reply, q)
		if err != nil {
			return nil, err
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}},
			Body: io.NopCloser(bytes.NewReader(wire))}, nil
	})
	c := configured(t, q, "https://example.test/mirror-pages", transport, false)
	got, report, err := c.DoMirrorPage(context.Background(), request)
	if !errors.Is(err, service.ErrConflict) || !got.Restart || got.Page != nil || calls != 1 || report.TransportAttempts != 1 {
		t.Fatalf("restart was hidden or retried: %v, calls %d", err, calls)
	}
	cursor.Scope.CallerID = "different-caller"
	if _, report, err := c.DoMirrorPage(context.Background(), request); !errors.Is(err, service.ErrScope) || report.TransportAttempts != 0 || calls != 1 {
		t.Fatalf("foreign cursor was sent: %v, calls %d", err, calls)
	}
}

func TestMirrorPageClientCapsResponse(t *testing.T) {
	q, _, _ := literal(t)
	q.Operation, q.Source = service.MirrorQuery, nil
	q.Mirror = &service.MirrorRequest{Kind: mirror.Changes, MaxAgeMS: 60_000}
	request := service.MirrorPageRequest{Schema: service.MirrorPageSchema, Query: q, Limit: 100}
	body := bytes.NewReader(make([]byte, service.MaxMessageBytes+2))
	closes := 0
	transport := trip(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}},
			Body: &closeBody{Reader: body, close: func() { closes++ }}}, nil
	})
	c := configured(t, q, "https://example.test/mirror-pages", transport, false)
	got, report, err := c.DoMirrorPage(context.Background(), request)
	if !errors.Is(err, service.ErrInvalid) || got.Page != nil || report.TransportAttempts != 1 ||
		report.ResponseBodyBytes == nil || *report.ResponseBodyBytes != service.MaxMessageBytes+1 ||
		body.Len() != 1 || closes != 1 {
		t.Fatalf("expected an oversized response to be refused after reading limit+1 bytes: %v, report %+v, unread %d, closes %d",
			err, report, body.Len(), closes)
	}
}
