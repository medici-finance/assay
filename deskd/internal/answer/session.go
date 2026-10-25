package answer

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/medici-finance/assay/deskd/internal/humanid"
	"sync"
	"time"
)

// This file is the LIVE-HUMAN SESSION GATE (design §6, the answer flow: "behind their
// live session"). It is the reason a bot or an unauthenticated caller cannot
// reach the write path: every write in this package requires a *Session that
// was minted by [SessionStore.Begin], and Begin is the ONLY way the unexported
// `live` bit is ever set. A caller outside this package cannot construct a live
// Session from a struct literal — the field is unexported — so "a live human is
// present" is a fact the type system carries, not a boolean a request supplies.
//
// # Why the gate is not the credential
//
// Holding the operator token (internal/custody) proves the human authorized once;
// a live session proves the human is present NOW. Both are required: a token at
// rest with no live session must not write (a background process could hold the
// token), and a live session with no token cannot write (nothing to author
// with). The Service checks both.

// SessionTTL bounds how long one interactive session stays live without being
// renewed. The spike is a single human at a localhost console; a short TTL
// keeps a walked-away laptop from holding an open write path.
const SessionTTL = 30 * time.Minute

// Session is one live human presence at the console. The `live` field is
// unexported by design: it is set only by [SessionStore.Begin].
//
// Actor is the AUTHENTICATED login of the human who established the session:
// the fixed "operator" assumption is gone — Actor now carries whoever
// signed in, and the Service asserts it equals the cell's answer_as before any
// write. The write itself is still authored as operator by the custody credential
// (frozen, the answer flow); Actor is the AUTHORIZATION identity, not the write one.
type Session struct {
	ID        string
	Actor     string
	StartedAt time.Time
	ExpiresAt time.Time
	live      bool
}

// Live reports whether this session was established interactively by a human.
// It can only be true for a session Begin minted.
func (s *Session) Live() bool { return s != nil && s.live }

// SessionStore mints and validates live sessions. It is in-memory: a deskd
// restart ends every session, which is the correct behaviour — a session is a
// live presence, and after a restart no one is proven present until they act.
type SessionStore struct {
	// Now is injectable so tests can pin session age.
	Now func() time.Time
	// CredentialPresent reports whether the operator credential is held. Begin
	// refuses to mint a live session with no credential, because a session that
	// cannot lead to a write is a false promise of one.
	CredentialPresent func() bool

	mu       sync.Mutex
	sessions map[string]*Session
}

// NewSessionStore builds a store. credentialPresent is required — a nil one is
// treated as "no credential", so Begin fails closed.
func NewSessionStore(now func() time.Time, credentialPresent func() bool) *SessionStore {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	if credentialPresent == nil {
		credentialPresent = func() bool { return false }
	}
	return &SessionStore{Now: now, CredentialPresent: credentialPresent, sessions: map[string]*Session{}}
}

// ErrNoCredentialForSession is returned when Begin is called with no operator
// credential held: there is nothing a session could author with.
var ErrNoCredentialForSession = fmt.Errorf("cannot begin a live session: no human credential accompanied the request — under custody model (b) the person's credential rides with the action, and a session that could never lead to a write is a false promise of one (#1819)")

// Begin mints a new live-human session for the DEFAULT answerer identity
// (humanActor()). It is retained for the localhost-default path and the existing
// test suite; the per-user path uses [SessionStore.BeginAs] with the
// authenticated login. Both are the ONLY constructors of a live Session.
func (st *SessionStore) Begin() (*Session, error) {
	return st.BeginAs(humanActor())
}

// BeginAs mints a live-human session bound to an AUTHENTICATED login.
// deskd calls it with the login the OAuth identity session resolved, so the
// live-answer session carries whoever actually signed in — the Service then
// asserts that login equals the cell's answer_as before any write. The operator
// credential must still be present (a session that could never lead to a write
// is not a false promise of one), and `live` is set ONLY here and in Begin.
func (st *SessionStore) BeginAs(login string) (*Session, error) {
	return st.BeginAsPresent(login, st.CredentialPresent())
}

// BeginAsPresent is the PER-REQUEST twin of [BeginAs] (custody model (b),
// #1819): the caller has already established, for THIS request, whether a human
// credential accompanied it, because under model (b) presence is a fact of the
// request and not a service-wide state there is any store to ask. Everything
// else is identical — including that `live` is set only here.
func (st *SessionStore) BeginAsPresent(login string, credentialPresent bool) (*Session, error) {
	if !humanid.Enabled() {
		return nil, humanid.ErrDisabled
	}
	if !credentialPresent {
		return nil, ErrNoCredentialForSession
	}
	if login == "" {
		return nil, fmt.Errorf("cannot begin a live session with no authenticated login")
	}
	id, err := newSessionID()
	if err != nil {
		return nil, err
	}
	now := st.Now()
	s := &Session{
		ID:        id,
		Actor:     login,
		StartedAt: now,
		ExpiresAt: now.Add(SessionTTL),
		live:      true,
	}
	st.mu.Lock()
	st.sessions[id] = s
	st.mu.Unlock()
	return s, nil
}

// Validate returns the live, unexpired session for id, or an error. It is what
// every write handler calls before touching the write path.
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
	if !s.Live() {
		return nil, ErrNotLiveHuman
	}
	if st.Now().After(s.ExpiresAt) {
		return nil, ErrSessionExpired
	}
	return s, nil
}

// End invalidates a session (logout / window teardown).
func (st *SessionStore) End(id string) {
	st.mu.Lock()
	delete(st.sessions, id)
	st.mu.Unlock()
}

// newSessionID mints an unguessable session id. crypto/rand, not math/rand: a
// session id is a bearer of the write path and must not be predictable.
func newSessionID() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("could not mint a session id: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}
