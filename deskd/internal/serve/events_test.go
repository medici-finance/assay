package serve_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/internal/contract"
	"github.com/medici-finance/assay/deskd/internal/domain"
	"github.com/medici-finance/assay/deskd/internal/serve"
)

// TestEventBusLiveDelivery: a live subscriber (no Last-Event-ID) receives an
// event published after it subscribed.
func TestEventBusLiveDelivery(t *testing.T) {
	bus := serve.NewEventBus()
	replay, ch, cancel := bus.Subscribe("")
	defer cancel()
	if len(replay) != 0 {
		t.Fatalf("a fresh live subscribe replayed %d events, want 0", len(replay))
	}

	bus.Publish(contract.EventBadge, contract.BadgeData{Open: 1})

	select {
	case e := <-ch:
		if e.Type != contract.EventBadge {
			t.Fatalf("got event type %q, want %q", e.Type, contract.EventBadge)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the published event")
	}
}

// TestEventBusResumeNoDuplicates is Verify row 9's core mechanical claim:
// reconnecting with Last-Event-ID of an already-seen event replays everything
// AFTER it, and never that event itself again.
func TestEventBusResumeNoDuplicates(t *testing.T) {
	bus := serve.NewEventBus()
	bus.Publish(contract.EventTransition, contract.TransitionData{ID: "e1", From: "", To: "waiting"})
	bus.Publish(contract.EventTransition, contract.TransitionData{ID: "e1", From: "waiting", To: "closed"})
	bus.Publish(contract.EventBadge, contract.BadgeData{Open: 1})

	// Publish mints ids as a monotonic decimal counter starting at 1, so the
	// three events above are ids "1", "2", "3" — resuming from "1" must
	// replay "2" and "3" and never "1" again.
	replay, _, cancel := bus.Subscribe("1")
	defer cancel()
	if len(replay) != 2 {
		t.Fatalf("resume from id 1 replayed %d events, want 2 (ids 2 and 3, never 1 again)", len(replay))
	}
	if replay[0].ID != "2" || replay[1].ID != "3" {
		t.Fatalf("replay ids = [%s, %s], want [2, 3]", replay[0].ID, replay[1].ID)
	}
	for _, e := range replay {
		if e.ID == "1" {
			t.Fatalf("replay duplicated the already-seen event id 1")
		}
	}
}

// TestEventBusFellOffRingGetsResync: a Last-Event-ID the ring does not
// recognise (fallen off the 1000-event buffer, or simply never issued) gets a
// single synthetic resync event instead of a fabricated partial replay.
func TestEventBusFellOffRingGetsResync(t *testing.T) {
	bus := serve.NewEventBus()
	bus.Publish(contract.EventBadge, contract.BadgeData{Open: 1})

	replay, _, cancel := bus.Subscribe("does-not-exist-in-the-ring")
	defer cancel()
	if len(replay) != 1 {
		t.Fatalf("got %d replay events, want exactly 1 (the resync)", len(replay))
	}
	if replay[0].Type != contract.EventResync {
		t.Fatalf("replay[0].Type = %q, want %q", replay[0].Type, contract.EventResync)
	}
}

// TestHandleV1EventsUnwiredIs503: a build with no EventBus wired answers a
// clear could-not-check, never a stream that hangs open delivering nothing.
func TestHandleV1EventsUnwiredIs503(t *testing.T) {
	h := serve.NewV1(nil, nil, nil, nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/events", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET /v1/events with no bus wired answered %d, want 503", rec.Code)
	}
}

// TestHandleV1EventsReplays: a GET with Last-Event-ID resumes from the bus's
// ring, over the real HTTP handler, decoded with the real client-side SSE
// parser — proving the wire format the handler emits and the format
// contract.EventStream expects are the SAME format, not two that happen to
// look alike in each side's own test.
func TestHandleV1EventsReplays(t *testing.T) {
	bus := serve.NewEventBus()
	bus.Publish(contract.EventTransition, contract.TransitionData{ID: "e1", From: "", To: "waiting"})
	bus.Publish(contract.EventTransition, contract.TransitionData{ID: "e1", From: "waiting", To: "closed"})

	h := serve.NewV1(nil, nil, nil, nil, nil, nil, bus, nil)

	// An already-cancelled request context: the handler writes its replay
	// synchronously before ever checking ctx.Done(), so this deterministically
	// exercises "replay, then stop" without an open-ended live stream in a
	// unit test.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodGet, "/v1/events", nil).WithContext(ctx)
	req.Header.Set("Last-Event-ID", "1")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/events answered %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", ct)
	}

	stream := contract.NewEventStream(rec.Body, nil)
	e, err := stream.Next()
	if err != nil {
		t.Fatalf("decoding the replayed frame: %v", err)
	}
	if e.ID != "2" {
		t.Fatalf("replayed event id = %q, want 2 (event 1 must not repeat)", e.ID)
	}
	if _, err := stream.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("a second frame was present or a decode error occurred (only event 2 should replay): %v", err)
	}
}

// TestHandleV1RefreshUnmountedWithNoWiring: the nudge route is ABSENT (404),
// never present-but-refusing, when NewV1 gets no Refresher.
func TestHandleV1RefreshUnmountedWithNoWiring(t *testing.T) {
	h := serve.NewV1(nil, nil, nil, nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/escalations/E1/refresh", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("POST refresh with no Refresher wired answered %d, want 404", rec.Code)
	}
}

// fixtureRefresher is a scripted [serve.Refresher] for the route-level test.
type fixtureRefresher struct {
	gotID domain.EscalationID
	err   error
}

func (f *fixtureRefresher) Refresh(_ context.Context, id domain.EscalationID) error {
	f.gotID = id
	return f.err
}

// TestHandleV1RefreshCallsWiredRefresher: the route parses the path id and
// reports the Refresher's own three-state outcome — 200 on success, 404 for
// the unknown-escalation sentinel, 500 for anything else.
func TestHandleV1RefreshCallsWiredRefresher(t *testing.T) {
	ref := &fixtureRefresher{}
	h := serve.NewV1(nil, nil, nil, nil, nil, nil, nil, ref)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/escalations/E42/refresh", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("POST refresh answered %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if ref.gotID != domain.EscalationID("E42") {
		t.Fatalf("Refresher got id %q, want E42", ref.gotID)
	}

	ref2 := &fixtureRefresher{err: serve.ErrRefreshUnknownEscalation}
	h2 := serve.NewV1(nil, nil, nil, nil, nil, nil, nil, ref2)
	rec2 := httptest.NewRecorder()
	h2.ServeHTTP(rec2, httptest.NewRequest(http.MethodPost, "/v1/escalations/does-not-exist/refresh", nil))
	if rec2.Code != http.StatusNotFound {
		t.Fatalf("POST refresh on an unknown escalation answered %d, want 404", rec2.Code)
	}
}
