package identity

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// ── the service contract Task 9 — the runtime half of the ROLLOUT GATE ─────────
//
// Environment-variable presence must never enable per-user identity globally.
// These tests enforce the per-cell behavior:
//
//	2. an OFF cell is LOGIN-GATED WITH NO SIGN-IN ROUTE — the capability
//	   ladder middleware stays, the OAuth login/callback routes are not
//	   served, so nobody rises above anonymous and nothing renders pre-auth.
//	   No single-persona mode returns.
//	5. deskd fails CLOSED to OFF when the gate cannot be resolved, and two
//	   cells in ONE process may differ (Verify row 12).

// fakeRollout is a per-repo gate fixture.
type fakeRollout struct {
	enabled map[string]bool
	signIn  bool
}

func (f fakeRollout) EnabledForRepo(owner, name string) bool { return f.enabled[owner+"/"+name] }
func (f fakeRollout) SignInServed() bool                     { return f.signIn }

// allOn is the "every cell opted in" fixture the pre-existing ladder tests
// run under, so those tests keep testing the ladder rather than the gate.
type allOn struct{}

func (allOn) EnabledForRepo(string, string) bool { return true }
func (allOn) SignInServed() bool                 { return true }

// mapRepos resolves several item ids to DIFFERENT repos, so one guard can be
// asked about two cells in one process.
type mapRepos map[string][2]string

func (m mapRepos) RepoForItem(id string) (string, string, bool) {
	r, ok := m[id]
	if !ok {
		return "", "", false
	}
	return r[0], r[1], true
}

// An OFF cell admits nobody above anonymous, even a viewer whose GitHub
// permission is admin and whose session is perfectly valid. This is ruling 2:
// the ladder middleware stays and the answer is "anonymous", NOT a
// single-persona fallback.
func TestRolloutOffCellKeepsEveryViewerAnonymous(t *testing.T) {
	ran := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { ran = true; w.WriteHeader(200) })
	g, store := guardWith("admin", true, fakeRepos{owner: "o", name: "off"}, fakePolicy{answerAs: "operator"})
	g.Rollout = fakeRollout{enabled: map[string]bool{}} // nothing opted in

	h := g.Handler(Route{Pattern: "/item/", Min: Commenter, ItemScoped: true, Handler: inner})
	req := httptest.NewRequest(http.MethodGet, "/item/id-1", nil)
	req.AddCookie(withSessionCookie(store, "bob", "t"))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("an admin-permission session on an OFF cell got %d, want 403 — the per-user path is not live there", rr.Code)
	}
	if ran {
		t.Fatal("the data handler ran on an OFF cell — the rollout gate is not enforcing")
	}
}

// Pre-auth on an OFF cell is still 401 with a login affordance and no data:
// OFF means login-gated, never anonymous-serving.
func TestRolloutOffCellIsStillLoginGatedPreAuth(t *testing.T) {
	ran := false
	inner := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { ran = true })
	g, _ := guardWith("admin", true, fakeRepos{owner: "o", name: "off"}, fakePolicy{loginGated: true})
	g.Rollout = fakeRollout{enabled: map[string]bool{}}

	h := g.Handler(Route{Pattern: "/inbox", Min: Commenter, ItemScoped: false, Handler: inner})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/inbox", nil))
	if rr.Code != http.StatusUnauthorized || ran {
		t.Fatalf("OFF surface pre-auth got %d ran=%v, want 401 with no handler run", rr.Code, ran)
	}
}

// A nil rollout is an UNRESOLVED gate, and an unresolved gate is OFF. This is
// the positive control for "fails closed to OFF" (Verify 12a): a wiring that
// forgot to pass the gate must lose capability, never gain it.
func TestRolloutNilFailsClosedToOff(t *testing.T) {
	g, store := guardWith("admin", true, fakeRepos{owner: "o", name: "r"}, fakePolicy{answerAs: "operator"})
	g.Rollout = nil

	req := httptest.NewRequest(http.MethodGet, "/item/id-1", nil)
	req.AddCookie(withSessionCookie(store, "bob", "t"))
	if _, cap := g.CapabilityFor(req, "o", "r"); cap != Anonymous {
		t.Fatalf("an unresolved rollout gate derived %q — it must fail CLOSED to anonymous", cap)
	}
}

// (f) TWO CELLS IN ONE PROCESS DIFFER — Verify row 12's load-bearing claim,
// the one that proves the gate is per-cell and not a global build flag. One
// guard, one session, two items: the opted-in cell serves, the OFF cell does
// not.
func TestRolloutTwoCellsInOneProcessDiffer(t *testing.T) {
	served := map[string]bool{}
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		served[r.URL.Path] = true
		w.WriteHeader(200)
	})
	g, store := guardWith("admin", true, mapRepos{
		"on-item":  {"o", "proving"},
		"off-item": {"o", "waiting"},
	}, fakePolicy{answerAs: "operator"})
	g.Rollout = fakeRollout{enabled: map[string]bool{"o/proving": true}, signIn: true}

	h := g.Handler(Route{Pattern: "/item/", Min: Commenter, ItemScoped: true, Handler: inner})
	code := func(id string) int {
		req := httptest.NewRequest(http.MethodGet, "/item/"+id, nil)
		req.AddCookie(withSessionCookie(store, "bob", "t"))
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr.Code
	}

	if got := code("on-item"); got != 200 {
		t.Fatalf("the opted-in cell got %d, want 200 — the gate must not be global-OFF", got)
	}
	if got := code("off-item"); got != http.StatusForbidden {
		t.Fatalf("the OFF cell got %d, want 403 — the gate must not be global-ON", got)
	}
	if served["/item/off-item"] {
		t.Fatal("the OFF cell's handler ran in the same process that serves the ON cell")
	}
	if !served["/item/on-item"] {
		t.Fatal("the ON cell's handler did not run — the two cells are not actually differing")
	}
}

// The surface listing follows the same gate per row: an OFF cell's rows are
// not visible to an authenticated viewer who would otherwise be a commenter.
func TestRolloutSurfaceAuthorizerDropsOffCellRows(t *testing.T) {
	g, store := guardWith("read", true, fakeRepos{}, fakePolicy{answerAs: "operator"})
	g.Rollout = fakeRollout{enabled: map[string]bool{"o/proving": true}, signIn: true}

	req := httptest.NewRequest(http.MethodGet, "/inbox", nil)
	req.AddCookie(withSessionCookie(store, "bob", "t"))
	auth := g.SurfaceAuthorizer(req)
	if !auth("o", "proving") {
		t.Fatal("the opted-in cell's rows must stay visible to a commenter")
	}
	if auth("o", "waiting") {
		t.Fatal("an OFF cell's rows were listed — the rollout gate must apply per row on the surface too")
	}
}

// The chrome the web layer renders follows the gate as well: on an OFF cell a
// signed-in viewer is shown no comment or answer affordance. (Not a control —
// the route gate above is — but the page must not offer what the server will
// refuse.)
func TestRolloutViewerForOffCellOffersNothing(t *testing.T) {
	g, store := guardWith("admin", true, fakeRepos{owner: "o", name: "off"}, fakePolicy{answerAs: "bob"})
	g.Rollout = fakeRollout{enabled: map[string]bool{}}

	req := httptest.NewRequest(http.MethodGet, "/item/id-1", nil)
	req.AddCookie(withSessionCookie(store, "bob", "t"))
	v := g.ViewerFor(req, "o", "off")
	if v.Capability != "anonymous" || v.CanComment || v.CanAnswer {
		t.Fatalf("OFF-cell viewer = %+v, want anonymous with no affordances", v)
	}
}

// (e) NO SIGN-IN ROUTE when no cell is opted in: /auth/login and
// /auth/callback refuse, so no viewer can even begin to rise above anonymous.
// Logout stays reachable — ending a session is always safe.
func TestRolloutSignInRouteNotServedWhenNoCellEnabled(t *testing.T) {
	a := NewAuth(Config{ClientID: "cid", ClientSecret: "sec", RedirectURL: "http://127.0.0.1:8787/auth/callback"},
		NewSessionStore(nil), nil)
	a.Rollout = fakeRollout{signIn: false}
	h := a.Handler()

	for _, path := range []string{"/auth/login", "/auth/callback?code=x&state=y"} {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("%s got %d, want 503 — an OFF deployment must serve no sign-in route", path, rr.Code)
		}
		if loc := rr.Header().Get("Location"); loc != "" {
			t.Errorf("%s redirected to %q — it must not begin an OAuth flow at all", path, loc)
		}
	}

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/auth/logout", nil))
	if rr.Code == http.StatusServiceUnavailable {
		t.Error("logout must stay reachable — ending a session is always safe")
	}
}

// A nil rollout on the Auth is also OFF: the same fail-closed default as the
// guard, so a forgotten wiring cannot serve sign-in.
func TestRolloutSignInNilGateFailsClosed(t *testing.T) {
	a := NewAuth(Config{ClientID: "cid", ClientSecret: "sec", RedirectURL: "http://x/auth/callback"},
		NewSessionStore(nil), nil)
	rr := httptest.NewRecorder()
	a.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/auth/login", nil))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("an unwired rollout gate served sign-in (%d) — it must fail closed", rr.Code)
	}
}

// ...and it IS served once a cell is opted in, so the gate is a gate and not
// a permanent off switch.
func TestRolloutSignInRouteServedWhenACellIsEnabled(t *testing.T) {
	a := NewAuth(Config{ClientID: "cid", ClientSecret: "sec", RedirectURL: "http://127.0.0.1:8787/auth/callback",
		AuthBase: "https://github.example"}, NewSessionStore(nil), nil)
	a.Rollout = fakeRollout{signIn: true}
	rr := httptest.NewRecorder()
	a.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/auth/login", nil))
	if rr.Code != http.StatusFound {
		t.Fatalf("sign-in on an opted-in deployment got %d, want a redirect to the OAuth authorize URL", rr.Code)
	}
}
