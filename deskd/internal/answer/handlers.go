package answer

import (
	"encoding/json"
	"errors"
	"github.com/medici-finance/assay/deskd/internal/humanid"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/medici-finance/assay/deskd/internal/domain"
	"github.com/medici-finance/assay/deskd/internal/httpsecurity"
)

// This file is the HTTP surface of the write path. Three properties are load-
// bearing and are enforced HERE, not left to the caller:
//
//  1. The write ref is resolved SERVER-SIDE from the escalation id against the
//     trusted index ([EscalationLookup]), never taken from the request body — a
//     caller cannot point an answer at an arbitrary issue.
//  2. Every write requires a live session, read from an HttpOnly cookie the
//     [SessionStore] minted; there is no header or parameter that stands in.
//  3. Every request must originate from loopback. deskd already binds localhost
//     (§13.1); this is defence-in-depth so a future mis-binding cannot expose
//     the write path to the network.

// SessionCookie is the name of the live-answer session cookie. HttpOnly so page
// script cannot read it, SameSite=Strict so a cross-site form cannot ride it.
// It is exported so the `/v1` named-action handlers (internal/serve) validate
// the SAME session this package's handlers do, rather than growing a second
// notion of "a live human is present".
const SessionCookie = "deskd_session"

// EscalationLookup resolves an escalation id to its platform ref against the
// trusted index. deskd backs it with the derived index; the ref NEVER comes
// from the request.
type EscalationLookup interface {
	// Ref returns the platform ref for an escalation id, and whether it was
	// found. A not-found id is a 404, never a write against a guessed ref.
	Ref(id domain.EscalationID) (Ref, bool, error)
}

// Handlers is the write path's HTTP surface.
type Handlers struct {
	// Cookies uses the same browser URL policy as the OAuth session.
	Cookies  httpsecurity.CookiePolicy
	Service  *Service
	Sessions *SessionStore
	Lookup   EscalationLookup
	// Credential is an AMBIENT credential the process owns for its own
	// lifetime. Under custody model (b) the service leaves it nil and wires
	// CredentialFor instead; a client host that legitimately owns the person's
	// session credential (the service contract) may still use it.
	Credential Credential
	// CredentialFor resolves the PER-REQUEST human credential (custody model
	// (b), #1819): the person supplies it at the moment of the action and the
	// service stores nothing between requests. When it is set it takes
	// precedence over Credential, it decides whether a live session may begin,
	// and it is what every write handler puts on the request context for the
	// per-request writer to read.
	CredentialFor func(r *http.Request) (Credential, bool)
	// AuthLogin resolves the AUTHENTICATED login of the request's OAuth identity
	// session. When set, a live-answer session may be begun ONLY as
	// that login — the live-answer session inherits whoever actually signed in,
	// so its Actor is a real authenticated identity, not a fixed persona. When
	// nil (the localhost-default and the existing test suite), begin falls back
	// to the default answerer identity.
	AuthLogin func(r *http.Request) (login string, ok bool)
	// Now is injectable for tests.
	Now func() time.Time
}

// Handler registers the write endpoints. Every verb is pinned in the pattern
// (Go 1.22+ routing), so a wrong method is a 405 from the mux.
func (h *Handlers) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /session", h.loopback(h.handleSessionStatus))
	mux.HandleFunc("POST /session/begin", h.loopback(h.handleSessionBegin))
	mux.HandleFunc("POST /session/end", h.loopback(h.handleSessionEnd))
	mux.HandleFunc("POST /answer/{id}", h.loopback(h.requireSession(h.handleAnswer)))
	mux.HandleFunc("GET /answer/{id}/draft", h.loopback(h.requireSession(h.handleLoadDraft)))
	mux.HandleFunc("POST /answer/{id}/draft", h.loopback(h.requireSession(h.handleSaveDraft)))
	mux.HandleFunc("POST /answer/{id}/ack", h.loopback(h.requireSession(h.handleAck)))
	return mux
}

// loopback refuses any request that did not originate from 127.0.0.1/::1.
func (h *Handlers) loopback(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			writeJSON(w, http.StatusForbidden, map[string]string{
				"error": "the answer path is localhost-only (§13.1): this request did not originate from loopback",
			})
			return
		}
		next(w, r)
	}
}

// sessionCtx carries the validated session into a handler.
type withSession func(w http.ResponseWriter, r *http.Request, sess *Session)

// requireSession validates the session cookie before the handler runs, mapping
// each session error to a status so the web layer can prompt correctly.
func (h *Handlers) requireSession(next withSession) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(SessionCookie)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": ErrNoSession.Error()})
			return
		}
		sess, err := h.Sessions.Validate(c.Value)
		if err != nil {
			status := http.StatusUnauthorized
			if errors.Is(err, ErrSessionExpired) {
				status = http.StatusUnauthorized
			}
			writeJSON(w, status, map[string]string{"error": err.Error()})
			return
		}
		// Custody model (b): attach the request's own human credential so the
		// per-request writer can author with it, and DROP it when the request
		// returns. A write handler reached without one still runs — and fails
		// with ErrNoCredential from the writer — because "no credential" must be
		// a loud refusal, never a silent no-op.
		if cred, ok := h.credentialFor(r); ok {
			r = r.WithContext(WithCredential(r.Context(), cred))
		}
		next(w, r, sess)
	}
}

// credentialFor resolves the credential for THIS request, preferring the
// per-request resolver (custody model (b)) over any ambient one.
func (h *Handlers) credentialFor(r *http.Request) (Credential, bool) {
	if h.CredentialFor != nil {
		return h.CredentialFor(r)
	}
	if h.Credential != nil && h.Credential.Present() {
		return h.Credential, true
	}
	return nil, false
}

func (h *Handlers) handleSessionStatus(w http.ResponseWriter, r *http.Request) {
	_, present := h.credentialFor(r)
	live := false
	login := ""
	if c, err := r.Cookie(SessionCookie); err == nil {
		if sess, verr := h.Sessions.Validate(c.Value); verr == nil {
			live = true
			login = sess.Actor
		}
	}
	// The AUTHENTICATED login is the OAuth identity's login (server-derived from
	// GET /user), never a client- or config-supplied name (Verify 5). Prefer it
	// over the live-answer session's Actor so /session reports the real viewer
	// even before a live-answer session is begun.
	if h.AuthLogin != nil {
		if l, ok := h.AuthLogin(r); ok {
			login = l
		}
	}
	// Session-level capability summary. The real gate is per-item (the
	// middleware); this is the chrome's hint: anonymous when unauthenticated,
	// answerer when the authenticated login is the cell's answer_as, else
	// commenter.
	capability := "anonymous"
	if login != "" {
		capability = "commenter"
		if humanid.Enabled() && h.answerAs() != "" && strings.EqualFold(login, h.answerAs()) {
			capability = "answerer"
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"credential_present": present,
		"live":               live,
		"login":              login,
		"capability":         capability,
	})
}

// answerAs returns the cell's authorized-answerer login for the /session
// summary, defaulting through the Service.
func (h *Handlers) answerAs() string {
	if h.Service != nil {
		return h.Service.AnswerAs()
	}
	return humanActor()
}

func (h *Handlers) handleSessionBegin(w http.ResponseWriter, r *http.Request) {
	sess, err := h.beginSession(r)
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}
	c := h.Cookies.New(r, SessionCookie, sess.ID, http.SameSiteStrictMode)
	c.Expires = sess.ExpiresAt
	http.SetCookie(w, c)
	writeJSON(w, http.StatusOK, map[string]any{"live": true, "actor": sess.Actor, "expires_at": sess.ExpiresAt})
}

// beginSession mints the live-answer session. When AuthLogin is wired,
// it binds the session to the request's AUTHENTICATED login — a
// live-answer session cannot be begun without an OAuth identity, and it carries
// whoever signed in. Without AuthLogin (localhost-default / tests) it falls back
// to the default answerer identity via Begin.
func (h *Handlers) beginSession(r *http.Request) (*Session, error) {
	if h.CredentialFor != nil {
		// Custody model (b): credential presence is a fact of THIS request.
		_, present := h.CredentialFor(r)
		login := humanActor()
		if h.AuthLogin != nil {
			l, ok := h.AuthLogin(r)
			if !ok || l == "" {
				return nil, ErrNoSession
			}
			login = l
		}
		return h.Sessions.BeginAsPresent(login, present)
	}
	if h.AuthLogin != nil {
		login, ok := h.AuthLogin(r)
		if !ok || login == "" {
			return nil, ErrNoSession
		}
		return h.Sessions.BeginAs(login)
	}
	return h.Sessions.Begin()
}

func (h *Handlers) handleSessionEnd(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(SessionCookie); err == nil {
		h.Sessions.End(c.Value)
	}
	c := h.Cookies.New(r, SessionCookie, "", http.SameSiteStrictMode)
	c.MaxAge = -1
	http.SetCookie(w, c)
	writeJSON(w, http.StatusOK, map[string]string{"live": "false"})
}

func (h *Handlers) handleAnswer(w http.ResponseWriter, r *http.Request, sess *Session) {
	id := domain.EscalationID(r.PathValue("id"))
	ref, ok, err := h.resolveRef(w, id)
	if !ok {
		return
	}
	_ = err
	if err := r.ParseForm(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "form did not parse: " + err.Error()})
		return
	}
	req := AnswerRequest{
		ID:         id,
		Ref:        ref,
		Option:     domain.OptionID(r.FormValue("option")),
		OptionsRev: r.FormValue("options_rev"),
		Text:       r.FormValue("text"),
	}
	if lastSeen := r.FormValue("last_seen_updated_at"); lastSeen != "" {
		if t, perr := time.Parse(time.RFC3339, lastSeen); perr == nil {
			req.LastSeenUpdatedAt = t
		}
	}
	res, err := h.Service.Answer(r.Context(), sess, req)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, ErrRaced):
			status = http.StatusConflict
		case errors.Is(err, ErrEmptyAnswer), errors.Is(err, ErrBadRef):
			status = http.StatusBadRequest
		case errors.Is(err, ErrNoCredential):
			status = http.StatusForbidden
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *Handlers) handleSaveDraft(w http.ResponseWriter, r *http.Request, sess *Session) {
	id := domain.EscalationID(r.PathValue("id"))
	if err := r.ParseForm(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "form did not parse: " + err.Error()})
		return
	}
	d := Draft{EscalationID: id, Option: r.FormValue("option"), Text: r.FormValue("text")}
	if err := h.Service.SaveDraft(sess, d); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"saved": "true"})
}

func (h *Handlers) handleLoadDraft(w http.ResponseWriter, r *http.Request, sess *Session) {
	id := domain.EscalationID(r.PathValue("id"))
	d, ok, err := h.Service.LoadDraft(sess, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"exists": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"exists": true, "draft": d})
}

func (h *Handlers) handleAck(w http.ResponseWriter, r *http.Request, sess *Session) {
	id := domain.EscalationID(r.PathValue("id"))
	ref, ok, _ := h.resolveRef(w, id)
	if !ok {
		return
	}
	if err := h.Service.Ack(r.Context(), sess, id, ref); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrBadRef) {
			status = http.StatusBadRequest
		} else if errors.Is(err, ErrNoCredential) {
			status = http.StatusForbidden
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"acked": true, "reaction": AckContent, "still_open": true})
}

// resolveRef turns an id into a trusted ref, writing the error response itself
// and returning ok=false when it cannot. The ref comes from the index, never
// the request.
func (h *Handlers) resolveRef(w http.ResponseWriter, id domain.EscalationID) (Ref, bool, error) {
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "escalation id is required in the path"})
		return Ref{}, false, nil
	}
	if h.Lookup == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no escalation lookup configured"})
		return Ref{}, false, nil
	}
	ref, found, err := h.Lookup.Ref(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "ref lookup failed: " + err.Error()})
		return Ref{}, false, err
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no open escalation with that id in the index"})
		return Ref{}, false, nil
	}
	return ref, true, nil
}

// StatusForError maps a write-path error to the HTTP status the surface
// answers with. It lives here, beside the sentinel errors it switches on, so
// every mount of the write path — this package's own handlers and the `/v1`
// named-action routes in internal/serve — reports the SAME condition with the
// same status. A second mapping kept in a second package is how a race quietly
// becomes a 500 on one seam and a 409 on the other.
func StatusForError(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, ErrRaced):
		return http.StatusConflict
	case errors.Is(err, ErrEmptyAnswer), errors.Is(err, ErrBadRef):
		return http.StatusBadRequest
	case errors.Is(err, ErrNoCredential), errors.Is(err, ErrNotLiveHuman), errors.Is(err, ErrIntentRefused):
		return http.StatusForbidden
	case errors.Is(err, ErrNoSession), errors.Is(err, ErrSessionExpired):
		return http.StatusUnauthorized
	default:
		return http.StatusInternalServerError
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
