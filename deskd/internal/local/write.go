// Package local is deskd's LOCAL-ONLY write surface: the read/ack setter
// endpoints (decision-model spec §1.1.2, ruling D7). It is deliberately a
// separate package from internal/serve — serve is structurally read-only and
// stays that way, its no-write property untouched — because the write these
// endpoints perform is a write to deskd's OWN SQLite (local_attention) and
// NOTHING on the platform.
//
// # Why this is not a §1.2 hole
//
// Read and Acked record HUMAN attention on the human-authed surface. No MCP
// tool writes them, no agent action reaches them, and — the hard rule — they
// never feed the attention derivation of any automation (spec §1.1.2). An
// agent cannot look alive, urgent, or acknowledged through them.
//
// # Why no platform write can leak from here
//
// This package holds no GitHub client and imports no transport: it depends on
// internal/index (the SQLite store) and the standard library, and that is the
// whole of its reach. The setters it calls touch one local table. The
// write-through to a `operator` 👀 reaction is the answer flow's, where the
// authenticated write path actually exists — these are the endpoints the answer flow
// will call.
package local

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/medici-finance/assay/deskd/internal/domain"
	"github.com/medici-finance/assay/deskd/internal/index"
)

// Writer serves the two local write endpoints over the index.
type Writer struct {
	DB *index.DB
	// Now is injectable so a test can pin the write timestamp.
	Now func() time.Time
	// Actor is the human identity writes are attributed to. Empty falls back
	// to index.DefaultActor (`operator`).
	Actor string
}

// New builds a Writer with sane defaults.
func New(db *index.DB) *Writer {
	return &Writer{DB: db, Now: func() time.Time { return time.Now().UTC() }, Actor: index.DefaultActor()}
}

// Handler registers the two POST endpoints the answer flow will call:
//
//	POST /escalations/{id}/read   — set the local read timestamp
//	POST /escalations/{id}/ack    — set the local ack timestamp
//
// The method is pinned in the pattern (Go 1.22+ routing), so a GET to either
// path is a 405 from the mux, and no other verb is registered. Nothing here
// writes GitHub.
func (wr *Writer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /escalations/{id}/read", wr.handle((*index.DB).SetRead))
	mux.HandleFunc("POST /escalations/{id}/ack", wr.handle((*index.DB).SetAck))
	return mux
}

// MarkRead records the local read marker for one escalation — the same D7
// write POST /escalations/{id}/read performs, exposed as a method so the
// versioned seam (POST /v1/escalations/{id}/read, internal/serve) records it
// through THIS package rather than growing a second writer of the local table.
// It writes deskd's own SQLite and nothing on the platform.
func (wr *Writer) MarkRead(id domain.EscalationID) error {
	actor := wr.Actor
	if actor == "" {
		actor = index.DefaultActor()
	}
	now := time.Now().UTC()
	if wr.Now != nil {
		now = wr.Now()
	}
	return wr.DB.SetRead(id, actor, now)
}

// setter is the shape of DB.SetRead / DB.SetAck.
type setter func(*index.DB, domain.EscalationID, string, time.Time) error

func (wr *Writer) handle(set setter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := domain.EscalationID(r.PathValue("id"))
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "escalation id is required in the path"})
			return
		}
		actor := wr.Actor
		if actor == "" {
			actor = index.DefaultActor()
		}
		now := time.Now().UTC()
		if wr.Now != nil {
			now = wr.Now()
		}
		if err := set(wr.DB, id, actor, now); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "the local read/ack write failed: " + err.Error(),
			})
			return
		}
		read, acked, err := wr.DB.Attention(id)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"id":    string(id),
			"actor": actor,
			"read":  read,
			"acked": acked,
			// A standing disclosure: this endpoint is local-only by ruling D7.
			"platform_write": "none — read/ack are index-local this brief (spec §1.1.2, D7); the operator write-through is the answer flow",
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
