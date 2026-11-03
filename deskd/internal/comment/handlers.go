package comment

import (
	"encoding/json"
	"errors"
	"net/http"
)

// This file is the HTTP surface of the user comment path: POST /comment/{id}.
// Two properties are load-bearing and enforced HERE:
//
//  1. The AUTHOR is the request's authenticated session, resolved via
//     [SessionSource] from the HttpOnly session cookie — NEVER a field, header,
//     or parameter in the request. A body that carries `actor=someone` or an
//     `X-Actor` header changes nothing (Verify 5): this handler never reads an
//     identity from the request at all.
//  2. The ref is resolved server-side from the id against the trusted index
//     (Service.Comment → RefLookup), never from the request body.

// SessionSource resolves the authenticated viewer for a request. It is
// implemented in the deskd wiring over the identity session store; keeping it
// an interface means this package imports neither identity nor any GitHub code.
// ok=false means no authenticated session — the handler returns 401.
type SessionSource interface {
	Authenticated(r *http.Request) (login, token string, ok bool)
}

// Handlers is the comment path's HTTP surface.
type Handlers struct {
	Service  *Service
	Sessions SessionSource
	// PathID extracts the escalation id from the request path. deskd supplies
	// the mux-appropriate extractor so this package holds no routing assumption.
	PathID func(r *http.Request) string
}

// Handler registers POST /comment/{id}.
func (h *Handlers) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /comment/{id}", h.handleComment)
	return mux
}

func (h *Handlers) handleComment(w http.ResponseWriter, r *http.Request) {
	login, token, ok := h.authenticated(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": ErrNoAuthor.Error()})
		return
	}
	id := r.PathValue("id")
	if id == "" && h.PathID != nil {
		id = h.PathID(r)
	}
	if err := r.ParseForm(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "form did not parse: " + err.Error()})
		return
	}
	body := r.FormValue("text")
	if body == "" {
		body = r.FormValue("body")
	}
	// NOTE: any `actor`/`author`/`login` field or `X-Actor` header on the
	// request is deliberately NOT read. The author is `login`, resolved from the
	// session cookie above.
	if err := h.Service.Comment(r.Context(), login, token, id, body); err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, ErrNoAuthor):
			status = http.StatusUnauthorized
		case errors.Is(err, ErrEmptyComment), errors.Is(err, ErrBadRef):
			status = http.StatusBadRequest
		case errors.Is(err, ErrUnknownItem):
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"commented": true, "as": login, "still_open": true})
}

func (h *Handlers) authenticated(r *http.Request) (string, string, bool) {
	if h.Sessions == nil {
		return "", "", false
	}
	return h.Sessions.Authenticated(r)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
