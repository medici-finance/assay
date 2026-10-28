package identity

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// terminal_gate_test.go — layer (c) of the service contract in isolation: who is
// asking (viewer identity only; a write credential is never an identity) and
// whether they are the answerer on the desk's cell (rollout ON for every repo,
// login == answer_as). Every refusal is fail-closed.

type tgAllOn struct{}

func (tgAllOn) EnabledForRepo(owner, name string) bool { return owner != "" && name != "" }
func (tgAllOn) SignInServed() bool                     { return true }

// viewerApp stands in for the viewer OAuth App's token-validation endpoint:
// it recognises exactly one token, and 404s everything else — a GitHub write
// token included.
func viewerApp(t *testing.T) *Validator {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			AccessToken string `json:"access_token"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.AccessToken != "viewer-token-operator" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"scopes": []string{"read:user"}, "user": map[string]string{"login": "operator"}})
	}))
	t.Cleanup(srv.Close)
	return NewValidator("viewer-client", "viewer-secret", srv.URL)
}

func gateFixture(t *testing.T, v *Validator) (*TerminalGate, *SessionStore, *fakeClockTG) {
	t.Helper()
	clk := &fakeClockTG{t: time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)}
	store := NewSessionStore(clk.now)
	g := &TerminalGate{
		Sessions:  store,
		Validator: v,
		Rollout:   tgAllOn{},
		Cell: func(name string) (TerminalCell, bool) {
			switch name {
			case "assay":
				return TerminalCell{Name: "assay", Repos: []string{"example-org/console"}, AnswerAs: "operator"}, true
			case "tracker":
				return TerminalCell{Name: "tracker", Repos: []string{"example-org/tracker-restricted"}, AnswerAs: "someone-else"}, true
			}
			return TerminalCell{}, false
		},
	}
	return g, store, clk
}

type fakeClockTG struct{ t time.Time }

func (c *fakeClockTG) now() time.Time { return c.t }

func reqWith(cookie string, authz string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/v1/desks/assay%2Fdispatch/terminal/send", nil)
	if cookie != "" {
		r.AddCookie(&http.Cookie{Name: SessionCookie, Value: cookie})
	}
	if authz != "" {
		r.Header.Set("Authorization", authz)
	}
	return r
}

func status(err error) int {
	if ge, ok := IsGateError(err); ok {
		return ge.Status
	}
	if err == nil {
		return 200
	}
	return -1
}

func TestTerminalGateAnswererOnly(t *testing.T) {
	g, store, _ := gateFixture(t, nil)
	answerer, _ := store.Begin("operator", "oauth-session-token")
	commenter, _ := store.Begin("some-reader", "oauth-session-token-2")

	if login, err := g.Authorize(reqWith(answerer.ID, ""), "assay"); err != nil || login != "operator" {
		t.Fatalf("the answerer on assay: login=%q err=%v, want operator / nil", login, err)
	}
	for _, c := range []struct {
		name   string
		r      *http.Request
		cell   string
		status int
	}{
		{"no identity", reqWith("", ""), "assay", 401},
		{"unknown session cookie", reqWith("not-a-session", ""), "assay", 401},
		{"a commenter may watch, never type", reqWith(commenter.ID, ""), "assay", 403},
		{"answerer of assay on a cell whose answer_as is someone else", reqWith(answerer.ID, ""), "tracker", 403},
		{"unknown cell", reqWith(answerer.ID, ""), "nope", 403},
		{"no cell", reqWith(answerer.ID, ""), "", 403},
	} {
		if _, err := g.Authorize(c.r, c.cell); status(err) != c.status {
			t.Errorf("%s: err = %v (status %d), want %d", c.name, err, status(err), c.status)
		}
	}
}

// TestTerminalGateRolloutOffIsNobody — an OFF cell derives nobody above
// anonymous, the configured answerer included (the Guard.derive rule).
func TestTerminalGateRolloutOffIsNobody(t *testing.T) {
	g, store, _ := gateFixture(t, nil)
	g.Rollout = nil
	answerer, _ := store.Begin("operator", "tok")
	if _, err := g.Authorize(reqWith(answerer.ID, ""), "assay"); status(err) != 403 {
		t.Fatalf("rollout OFF: err = %v, want 403", err)
	}
}

// TestTerminalGateNeverTakesAWriteCredential — row 8's layer: any
// Authorization header that is not a bearer the VIEWER App validates is a 400,
// even alongside a perfectly good answerer session cookie. It is never tried as
// a write credential and never silently ignored in favour of the cookie.
func TestTerminalGateNeverTakesAWriteCredential(t *testing.T) {
	g, store, _ := gateFixture(t, viewerApp(t))
	answerer, _ := store.Begin("operator", "tok")
	for _, h := range []string{
		"Bearer answerer-repo-scoped-write-credential",
		"token answerer-repo-scoped-write-credential",
		"Basic a3J5dG9uOnB3",
		"Bearer ",
	} {
		if _, err := g.Authorize(reqWith(answerer.ID, h), "assay"); status(err) != 400 {
			t.Errorf("Authorization %q: err = %v (status %d), want 400", h, err, status(err))
		}
	}
	// With NO viewer App configured, every Authorization header is refused.
	g2, store2, _ := gateFixture(t, nil)
	s2, _ := store2.Begin("operator", "tok")
	if _, err := g2.Authorize(reqWith(s2.ID, "Bearer viewer-token-operator"), "assay"); status(err) != 400 {
		t.Errorf("no viewer App configured: err = %v, want 400", err)
	}
	// The viewer bearer itself is accepted, alone or agreeing with the cookie.
	if login, err := g.Authorize(reqWith("", "Bearer viewer-token-operator"), "assay"); err != nil || login != "operator" {
		t.Errorf("viewer bearer: login=%q err=%v, want operator / nil", login, err)
	}
	if _, err := g.Authorize(reqWith(answerer.ID, "Bearer viewer-token-operator"), "assay"); err != nil {
		t.Errorf("agreeing cookie + bearer: %v", err)
	}
	other, _ := store.Begin("someone-else", "tok")
	if _, err := g.Authorize(reqWith(other.ID, "Bearer viewer-token-operator"), "assay"); status(err) != 400 {
		t.Errorf("conflicting cookie + bearer: err = %v, want 400", err)
	}
}

// TestTerminalGateSessionExpiryAndRevocation — the per-request identity is
// re-derived every time: an ended or expired session stops authorizing at once.
func TestTerminalGateSessionExpiryAndRevocation(t *testing.T) {
	g, store, clk := gateFixture(t, nil)
	s, _ := store.Begin("operator", "tok")
	if _, err := g.Authorize(reqWith(s.ID, ""), "assay"); err != nil {
		t.Fatal(err)
	}
	store.End(s.ID)
	if _, err := g.Authorize(reqWith(s.ID, ""), "assay"); status(err) != 401 {
		t.Errorf("ended session: err = %v, want 401", err)
	}
	s2, _ := store.Begin("operator", "tok")
	clk.t = clk.t.Add(SessionTTL + time.Minute)
	if _, err := g.Authorize(reqWith(s2.ID, ""), "assay"); status(err) != 401 || !strings.Contains(err.Error(), "expired") {
		t.Errorf("expired session: err = %v, want 401 expired", err)
	}
}
