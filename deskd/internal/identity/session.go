package identity

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

// SessionTTL bounds how long one authenticated session stays valid without a
// fresh login. A viewer's presence must be re-proven after it elapses.
const SessionTTL = 12 * time.Hour

// Sentinel session errors, first-class so a handler can map each to a status.
var (
	// ErrNoSession — no session cookie was presented, or it names no session.
	ErrNoSession = errors.New("no authenticated session: sign in with GitHub to continue")
	// ErrSessionExpired — the session TTL elapsed.
	ErrSessionExpired = errors.New("session expired: sign in with GitHub again")
)

// Session is one authenticated viewer's presence.
//
// The token is UNEXPORTED and never serialised: it lives only here, in memory,
// for the lifetime of the process. A deskd restart drops the whole map, which
// logs everyone out — correct, because presence must be re-proven and no user
// token is ever written at rest (the capability model gate-why (5), §6.4).
type Session struct {
	// ID is the opaque cookie value. crypto/rand, so it is unguessable.
	ID string
	// Login is the AUTHENTICATED GitHub login, resolved server-side from the
	// token via GET /user. It is the only identity source — no request field,
	// header, or config entry ever names the viewer.
	Login     string
	StartedAt time.Time
	ExpiresAt time.Time
	// token is the viewer's own GitHub OAuth token. It authors the viewer's
	// comments (internal/comment). It is UNEXPORTED so no other package can read
	// it off the struct, and it is never marshalled — a Session has no JSON tags
	// and is never handed to a template or a response body.
	token string
}

// Token returns the viewer's in-memory GitHub token. Only the comment write
// path calls it, at the moment of a write. It is never cached elsewhere.
func (s *Session) Token() string {
	if s == nil {
		return ""
	}
	return s.token
}

// SessionStore mints and validates authenticated sessions. It is in-memory by
// design: a deskd restart ends every session (gate-why (5)).
type SessionStore struct {
	// Now is injectable so tests can pin session age.
	Now func() time.Time

	mu       sync.Mutex
	sessions map[string]*Session
}

// NewSessionStore builds an empty in-memory session store.
func NewSessionStore(now func() time.Time) *SessionStore {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &SessionStore{Now: now, sessions: map[string]*Session{}}
}

// Begin mints a session for an already-authenticated viewer. login MUST have
// been resolved server-side from token via GET /user by the caller ([Auth]);
// this store never derives a login from anything a request supplied.
func (st *SessionStore) Begin(login, token string) (*Session, error) {
	if login == "" {
		return nil, fmt.Errorf("refusing to mint a session with no authenticated login")
	}
	if token == "" {
		return nil, fmt.Errorf("refusing to mint a session with no token to author with")
	}
	id, err := newSessionID()
	if err != nil {
		return nil, err
	}
	now := st.Now()
	s := &Session{
		ID:        id,
		Login:     login,
		StartedAt: now,
		ExpiresAt: now.Add(SessionTTL),
		token:     token,
	}
	st.mu.Lock()
	st.sessions[id] = s
	st.mu.Unlock()
	return s, nil
}

// Validate returns the unexpired session for id, or an error. It is what the
// middleware calls on every request that presents a session cookie.
func (st *SessionStore) Validate(id string) (*Session, error) {
	if id == "" {
		return nil, ErrNoSession
	}
	st.mu.Lock()
	s, ok := st.sessions[id]
	st.mu.Unlock()
	if !ok {
		return nil, ErrNoSession
	}
	if st.Now().After(s.ExpiresAt) {
		return nil, ErrSessionExpired
	}
	return s, nil
}

// End invalidates a session (logout).
func (st *SessionStore) End(id string) {
	st.mu.Lock()
	delete(st.sessions, id)
	st.mu.Unlock()
}

// newSessionID mints an unguessable session id: crypto/rand, never math/rand —
// a session id is a bearer of the viewer's authenticated identity.
func newSessionID() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("could not mint a session id: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}
