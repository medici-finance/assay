package answer

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/internal/domain"
	"github.com/medici-finance/assay/deskd/internal/httpsecurity"
)

type fakeCred struct{ present bool }

func (f fakeCred) Token() (string, error) { return "tok", nil }
func (f fakeCred) TokenActor() string     { return humanActor() }
func (f fakeCred) Present() bool          { return f.present }

type fakeLookup struct {
	ref   Ref
	found bool
}

func (f fakeLookup) Ref(domain.EscalationID) (Ref, bool, error) { return f.ref, f.found, nil }

func newTestHandlers(t *testing.T, w platformWriter) (*Handlers, *SessionStore) {
	t.Helper()
	sessions := NewSessionStore(func() time.Time { return time.Now().UTC() }, func() bool { return true })
	h := &Handlers{
		Service:    NewService(w, NewMemoryDraftStore(), &fakeAcks{}, time.Now),
		Sessions:   sessions,
		Lookup:     fakeLookup{ref: testRef, found: true},
		Credential: fakeCred{present: true},
	}
	return h, sessions
}

// TestLoopbackOnly proves the write surface refuses a non-loopback origin, even
// though deskd already binds localhost — defence-in-depth against a mis-binding.
func TestLoopbackOnly(t *testing.T) {
	h, _ := newTestHandlers(t, &fakeWriter{})
	srv := h.Handler()

	req := httptest.NewRequest(http.MethodPost, "/session/begin", nil)
	req.RemoteAddr = "8.8.8.8:44321" // not loopback
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("non-loopback origin got %d, want 403", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "localhost-only") {
		t.Fatalf("expected a localhost-only refusal, got %s", rr.Body.String())
	}
}

// TestSessionCookieRequired proves a write with no session cookie is refused,
// and one behind a live-session cookie is accepted — the gate is the cookie the
// SessionStore minted, not a header a caller can set.
func TestSessionCookieRequired(t *testing.T) {
	w := &fakeWriter{}
	h, sessions := newTestHandlers(t, w)
	srv := h.Handler()

	// no cookie -> 401, no write
	req := httptest.NewRequest(http.MethodPost, "/answer/01ID", nil)
	req.RemoteAddr = "127.0.0.1:5000"
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("no-cookie write got %d, want 401", rr.Code)
	}
	if w.comments != 0 {
		t.Fatal("a cookieless request wrote")
	}

	// live cookie -> answer commits
	sess, err := sessions.Begin()
	if err != nil {
		t.Fatal(err)
	}
	form := url.Values{"option": {"keep-it"}, "text": {"ok"}}
	req2 := httptest.NewRequest(http.MethodPost, "/answer/01ID", strings.NewReader(form.Encode()))
	req2.RemoteAddr = "127.0.0.1:5000"
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req2.AddCookie(&http.Cookie{Name: SessionCookie, Value: sess.ID})
	rr2 := httptest.NewRecorder()
	srv.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("live-session write got %d, want 200 (%s)", rr2.Code, rr2.Body.String())
	}
	if w.comments != 1 || w.closes != 1 {
		t.Fatalf("expected the write to happen: comments=%d closes=%d", w.comments, w.closes)
	}
}

// TestRefResolvedServerSide proves the write acts on the ref the trusted index
// returns, and that an unknown id is a 404 rather than a write against a guessed
// ref — a caller cannot point an answer at an arbitrary issue.
func TestRefResolvedServerSide(t *testing.T) {
	w := &fakeWriter{}
	sessions := NewSessionStore(func() time.Time { return time.Now().UTC() }, func() bool { return true })
	h := &Handlers{
		Service:    NewService(w, NewMemoryDraftStore(), &fakeAcks{}, time.Now),
		Sessions:   sessions,
		Lookup:     fakeLookup{found: false}, // id not in the index
		Credential: fakeCred{present: true},
	}
	srv := h.Handler()
	sess, _ := sessions.Begin()

	req := httptest.NewRequest(http.MethodPost, "/answer/unknown-id", strings.NewReader("option=x"))
	req.RemoteAddr = "127.0.0.1:5000"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: SessionCookie, Value: sess.ID})
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("unknown id got %d, want 404", rr.Code)
	}
	if w.comments != 0 {
		t.Fatal("wrote against an unresolved ref")
	}
}

// TestBeginRefusedWithoutCredential proves the HTTP session-begin fails closed
// when no credential is held.
func TestBeginRefusedWithoutCredential(t *testing.T) {
	sessions := NewSessionStore(func() time.Time { return time.Now().UTC() }, func() bool { return false })
	h := &Handlers{Sessions: sessions, Credential: fakeCred{present: false}}
	srv := h.Handler()
	req := httptest.NewRequest(http.MethodPost, "/session/begin", nil)
	req.RemoteAddr = "127.0.0.1:5000"
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("begin with no credential got %d, want 403", rr.Code)
	}
}

func TestSessionCookieAttributes(t *testing.T) {
	for _, local := range []bool{false, true} {
		h, sessions := newTestHandlers(t, &fakeWriter{})
		target := "http://console.example.invalid"
		if local {
			target = "http://127.0.0.1:8787"
			h.Cookies = httpsecurity.CookiesForURL(target)
		}
		var session *http.Cookie
		for _, path := range []string{"/session/begin", "/session/end"} {
			req := httptest.NewRequest(http.MethodPost, target+path, nil)
			req.RemoteAddr = "127.0.0.1:5000"
			if session != nil {
				req.AddCookie(session)
			}
			rr := httptest.NewRecorder()
			h.Handler().ServeHTTP(rr, req)
			if rr.Code != http.StatusOK {
				t.Fatalf("%s status=%d", path, rr.Code)
			}
			cookies := rr.Result().Cookies()
			if len(cookies) != 1 {
				t.Fatalf("%s cookies=%d", path, len(cookies))
			}
			c := cookies[0]
			if c.Name != SessionCookie || c.Secure == local || !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Path != "/" || c.Domain != "" {
				t.Fatalf("%s unexpected attributes: %+v", path, c)
			}
			if session == nil {
				session = c
				if _, err := sessions.Validate(c.Value); err != nil {
					t.Fatalf("new session invalid: %v", err)
				}
			} else {
				if c.Value != "" || c.MaxAge != -1 {
					t.Fatal("session cookie was not expired")
				}
				if _, err := sessions.Validate(session.Value); err == nil {
					t.Fatal("ended session remains valid")
				}
			}
		}
	}
}
