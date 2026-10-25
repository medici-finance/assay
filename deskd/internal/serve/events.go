// events.go — GET /v1/events, the SSE stream (the service contract increment 3,
// task 2), and the refresh nudge, POST /v1/escalations/{id}/refresh.
//
// Both of increment 2's audit findings for these two routes were the same
// shape: `internal/serve/v1.go` and `cmd/deskd/main.go` both named them "the
// remaining increment(s)" — declared on the wire (contract.Routes,
// contract.Client.Events/Refresh) with nothing on the service mounting them.
// This file is the mount.
package serve

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/medici-finance/assay/deskd/internal/contract"
	"github.com/medici-finance/assay/deskd/internal/domain"
	"github.com/medici-finance/assay/deskd/internal/index"
)

// ErrRefreshUnknownEscalation is the sentinel a [Refresher] returns when the
// path names no escalation the index knows about. handleV1Refresh renders it
// as 404; any other error renders as 500 (the re-derive itself failed, which
// is a different fact from "there is nothing there").
var ErrRefreshUnknownEscalation = errors.New("no such escalation in the index")

// eventRingSize is the ring buffer's depth (facts: "the service's ring buffer
// (last 1000 events)"). A client that presents a Last-Event-ID older than the
// oldest event still in the ring has fallen off the buffer and is told to
// resync rather than served a silently-truncated replay.
const eventRingSize = 1000

// eventChanDepth bounds one subscriber's outbound buffer. A slow reader (a
// stalled connection, a client that stopped pumping its event loop) must not
// block the publisher — see [EventBus.Publish] — so a full channel drops the
// event for that one subscriber rather than backing up every other one; the
// dropped subscriber's next successful read still carries a monotonic id, so
// its OWN next reconnect-with-Last-Event-ID (or a resync if it fell too far
// behind) is what recovers it, never the publisher waiting on it.
const eventChanDepth = 64

// EventBus is the /v1/events producer and ring buffer. It is the ONE place
// that mints monotonic event ids, so the ring buffer and every subscriber's
// Last-Event-ID resume agree on ordering. A nil *EventBus is a valid,
// deliberately inert value: [NewV1] mounts the route only when one is
// supplied, and [index.Poller] no-ops its publish calls when its Events field
// is nil — a build with no producer wired serves no event stream, never a
// stream with no events flowing through it.
type EventBus struct {
	mu   sync.Mutex
	seq  uint64
	ring []contract.Event
	subs map[chan contract.Event]struct{}
}

// NewEventBus builds an empty bus.
func NewEventBus() *EventBus {
	return &EventBus{subs: map[chan contract.Event]struct{}{}}
}

// Publish mints the next monotonic id, appends the event to the ring
// (trimming the oldest entry once the ring is full), and fans it out to every
// live subscriber. A subscriber whose channel is full is skipped for this one
// event rather than blocking every other subscriber and the publisher behind
// it — see [eventChanDepth].
func (b *EventBus) Publish(typ contract.EventType, data any) {
	b.mu.Lock()
	b.seq++
	id := strconv.FormatUint(b.seq, 10)
	evt, err := contract.NewEvent(id, time.Now().UTC(), typ, data)
	if err != nil {
		// A data value that cannot marshal is a caller bug, not a stream
		// fault: drop this one event rather than corrupt the ring with a
		// zero-value Event that would decode as something it is not.
		b.mu.Unlock()
		return
	}
	b.ring = append(b.ring, evt)
	if len(b.ring) > eventRingSize {
		b.ring = b.ring[len(b.ring)-eventRingSize:]
	}
	subs := make([]chan contract.Event, 0, len(b.subs))
	for ch := range b.subs {
		subs = append(subs, ch)
	}
	b.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- evt:
		default:
		}
	}
}

// Subscribe registers a live listener and computes its REPLAY set (facts:
// "Last-Event-ID resumes from the service's ring buffer … a client that falls
// off the buffer gets a resync event and re-reads /v1/inbox"):
//
//   - lastEventID == "": no replay — the caller wants only the live tail.
//   - lastEventID found in the ring: every event strictly AFTER it — the one
//     named by lastEventID is never replayed a second time (no duplicates).
//   - lastEventID non-empty but not found (fallen off the ring, or never
//     issued): the sole replay element is a synthetic [contract.EventResync],
//     which is the client's signal to re-read /v1/inbox rather than trust a
//     replay this bus cannot honestly reconstruct.
//
// cancel MUST be called when the caller is done (typically deferred) — it is
// the only way an EventBus stops holding the channel and delivering to it.
func (b *EventBus) Subscribe(lastEventID string) (replay []contract.Event, ch chan contract.Event, cancel func()) {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch = make(chan contract.Event, eventChanDepth)
	b.subs[ch] = struct{}{}
	cancel = func() {
		b.mu.Lock()
		delete(b.subs, ch)
		b.mu.Unlock()
	}

	if lastEventID == "" {
		return nil, ch, cancel
	}
	for i, e := range b.ring {
		if e.ID == lastEventID {
			return append([]contract.Event(nil), b.ring[i+1:]...), ch, cancel
		}
	}
	resyncID := "0"
	if n := len(b.ring); n > 0 {
		resyncID = b.ring[n-1].ID
	}
	resync, err := contract.NewEvent(resyncID, time.Now().UTC(), contract.EventResync, nil)
	if err != nil {
		return nil, ch, cancel
	}
	return []contract.Event{resync}, ch, cancel
}

// handleV1Events serves GET /v1/events: an SSE stream over bus. A nil bus (no
// producer wired on this build) is a could-not-check 503, never a stream that
// hangs open and delivers nothing — a caller can tell "not wired" from "wired
// but silent" by the status code alone.
//
// The bus is ONE producer for every viewer, so every event — replayed or live —
// passes through the viewer's [Server.eventFilter] before it is written: a
// viewer never learns of an escalation, a repository's health or a count it
// could not read through /v1/inbox.
func (s *Server) handleV1Events(bus *EventBus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if bus == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"error": "the event stream is not wired on this build",
			})
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming is not supported by this response writer"})
			return
		}

		replay, ch, cancel := bus.Subscribe(r.Header.Get("Last-Event-ID"))
		defer cancel()
		visible := s.eventFilter(r)

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)

		for _, e := range replay {
			e, ok := visible(e)
			if !ok {
				continue
			}
			if err := e.EncodeSSE(w); err != nil {
				return
			}
		}
		flusher.Flush()

		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case e, ok := <-ch:
				if !ok {
					return
				}
				e, ok = visible(e)
				if !ok {
					continue
				}
				if err := e.EncodeSSE(w); err != nil {
					return
				}
				flusher.Flush()
			}
		}
	}
}

// Refresher is the App-identity re-derivation the refresh nudge needs
// (contract.KindNudge, "App-identity re-fetch; authors nothing" — it performs
// no human-authored write and so is not gated by [NamedActions]). cmd/deskd
// wires it over the same per-org read routing the poller uses; a nil
// Refresher leaves the route unmounted (a 404), the same "absent, not
// refusing" shape every optional wiring on this surface takes.
type Refresher interface {
	Refresh(ctx context.Context, id domain.EscalationID) error
}

// handleV1Refresh serves POST /v1/escalations/{id}/refresh: nudge the service
// to re-derive this one issue now. It is idempotent (re-deriving twice leaves
// the same state a single re-derive would) and authors nothing — the read
// uses the service's own App identity, never a human credential, so this
// route is outside contract.NamedActions by construction and needs no
// per-request credential.
//
// It is still gated per item: a viewer with no read capability on the item's
// repository gets the same 404 an unknown id gets (never a permission leak),
// decided before the Refresher spends a platform read on it.
func (s *Server) handleV1Refresh(refresh Refresher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := domain.EscalationID(r.PathValue("id"))
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "escalation id is required in the path"})
			return
		}
		if s.Authz != nil {
			row, found, err := s.lookupRow(id)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "item index read failed (BLIND, not empty): " + err.Error()})
				return
			}
			if !found || !s.itemAuthorized(r, row) {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": ErrRefreshUnknownEscalation.Error()})
				return
			}
		}
		if err := refresh.Refresh(r.Context(), id); err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, ErrRefreshUnknownEscalation) {
				status = http.StatusNotFound
			}
			writeJSON(w, status, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": id, "refreshed": true})
	}
}

// lookupRow finds one escalation in the index, open or closed. found is false
// when the index has no such id or no index is wired.
func (s *Server) lookupRow(id domain.EscalationID) (row index.Row, found bool, err error) {
	if s.DB == nil {
		return index.Row{}, false, nil
	}
	rows, err := s.DB.Inbox("", true)
	if err != nil {
		return index.Row{}, false, err
	}
	for _, r := range rows {
		if r.ID == id {
			return r, true, nil
		}
	}
	return index.Row{}, false, nil
}

// eventFilter is one viewer's view of the shared event bus. With no Authz
// wired the stream is unfiltered, as every other read on this surface is.
// Otherwise:
//
//   - a transition is delivered only for an escalation the viewer can read;
//   - a badge is RECOMPUTED over the viewer's readable rows (the producer's
//     count spans every repository) and sent only when that count changes;
//   - an index-health set is cut to the repositories the viewer can read and
//     sent only when that cut changes;
//   - a resync carries no data and always passes;
//   - an event type this filter does not know is dropped: a new type must be
//     given a per-viewer rule here before any viewer sees it.
func (s *Server) eventFilter(r *http.Request) func(contract.Event) (contract.Event, bool) {
	if s.Authz == nil {
		return func(e contract.Event) (contract.Event, bool) { return e, true }
	}
	authorized := s.Authz.SurfaceAuthorizer(r)
	var lastBadge *contract.BadgeData
	var lastHealth []string
	sentHealth := false
	return func(e contract.Event) (contract.Event, bool) {
		switch e.Type {
		case contract.EventResync:
			return e, true
		case contract.EventTransition:
			var d contract.TransitionData
			if json.Unmarshal(e.Data, &d) != nil {
				return e, false
			}
			row, found, err := s.lookupRow(domain.EscalationID(d.ID))
			if err != nil || !found || !s.itemAuthorized(r, row) {
				return e, false
			}
			return e, true
		case contract.EventBadge:
			if s.DB == nil {
				return e, false
			}
			rows, err := s.DB.Inbox("", true)
			if err != nil {
				return e, false
			}
			b := index.BadgeCounts(s.filterByCapability(r, rows))
			if lastBadge != nil && *lastBadge == b {
				return e, false
			}
			out, err := contract.NewEvent(e.ID, e.At, e.Type, b)
			if err != nil {
				return e, false
			}
			lastBadge = &b
			return out, true
		case contract.EventIndexHealth:
			var d contract.IndexHealthData
			if json.Unmarshal(e.Data, &d) != nil {
				return e, false
			}
			var mine []string
			for _, repo := range d.CouldNotCheck {
				owner, name, ok := strings.Cut(repo, "/")
				if ok && authorized(owner, name) {
					mine = append(mine, repo)
				}
			}
			if sentHealth && slices.Equal(mine, lastHealth) {
				return e, false
			}
			out, err := contract.NewEvent(e.ID, e.At, e.Type, contract.IndexHealthData{CouldNotCheck: mine})
			if err != nil {
				return e, false
			}
			lastHealth, sentHealth = mine, true
			return out, true
		default:
			return e, false
		}
	}
}
