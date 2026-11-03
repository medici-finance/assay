package identity

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ── Capability ladder + derivation (Verify 6) ─────────────────────────────

func TestCapabilityLadder(t *testing.T) {
	if !Answerer.IsAnswerer() || !Answerer.AtLeastCommenter() {
		t.Fatal("answerer must imply commenter")
	}
	if Commenter.IsAnswerer() {
		t.Fatal("commenter is not an answerer")
	}
	if Anonymous.AtLeastCommenter() {
		t.Fatal("anonymous is not a commenter")
	}
	for tier, want := range map[Capability]string{Anonymous: "anonymous", Commenter: "commenter", Answerer: "answerer"} {
		if tier.String() != want {
			t.Fatalf("%d.String()=%q want %q", tier, tier.String(), want)
		}
	}
	// An out-of-ladder value renders anonymous (fail-closed).
	if Capability(99).String() != "anonymous" {
		t.Fatal("an unknown tier must render anonymous")
	}
}

// fakeQuerier answers a fixed permission and counts calls. The `calls` counter
// proves the cache behaviour; `perm`/`ok` drive the tier.
type fakeQuerier struct {
	perm  string
	ok    bool
	calls int
}

func (f *fakeQuerier) Permission(_ context.Context, _, _, _ string) (string, bool) {
	f.calls++
	return f.perm, f.ok
}

func sess(login string) *Session {
	return &Session{ID: "s", Login: login, ExpiresAt: time.Now().Add(time.Hour), token: "t"}
}

// TestDeriverTierMapping walks the base tier mapping and the answerer step.
func TestDeriverTierMapping(t *testing.T) {
	cases := []struct {
		perm     string
		ok       bool
		login    string
		answerAs string
		want     Capability
	}{
		{"none", true, "bob", "operator", Anonymous},
		{"read", true, "bob", "operator", Commenter},
		{"write", true, "bob", "operator", Commenter},
		{"admin", true, "bob", "operator", Commenter},
		{"", false, "bob", "operator", Anonymous},        // fail closed on error
		{"none", true, "operator", "operator", Answerer}, // answerer even with no repo grant
		{"read", true, "operator", "operator", Answerer}, // login compare is case-insensitive
	}
	for _, c := range cases {
		d := NewDeriver(&fakeQuerier{perm: c.perm, ok: c.ok}, nil)
		got := d.Capability(context.Background(), sess(c.login), "o", "r", c.answerAs)
		if got != c.want {
			t.Fatalf("perm=%q ok=%v login=%q answerAs=%q → %q, want %q", c.perm, c.ok, c.login, c.answerAs, got, c.want)
		}
	}
}

// TestDeriverNilSessionAnonymous: no session, no tier.
func TestDeriverNilSessionAnonymous(t *testing.T) {
	d := NewDeriver(&fakeQuerier{perm: "admin", ok: true}, nil)
	if got := d.Capability(context.Background(), nil, "o", "r", "operator"); got != Anonymous {
		t.Fatalf("nil session got %q, want anonymous", got)
	}
}

// TestDeriverIndependence (Verify 6): the tier is decided ONLY by the deskd-side
// permission query — the session (which stands in for the viewer's own token /
// any client-asserted claim) cannot raise it. A viewer with pull permission is
// a commenter no matter what identity they present.
func TestDeriverIndependence(t *testing.T) {
	q := &fakeQuerier{perm: "read", ok: true} // deskd-side truth: pull only
	d := NewDeriver(q, nil)
	// Even though the caller "claims" to be admin by any means, the querier is
	// the only input: commenter.
	if got := d.Capability(context.Background(), sess("bob"), "o", "r", "operator"); got != Commenter {
		t.Fatalf("independence violated: got %q, want commenter (the client cannot raise its own tier)", got)
	}
}

// TestDeriverCache: a definite answer is cached ≤120s; an error is not.
func TestDeriverCache(t *testing.T) {
	clock := time.Now()
	q := &fakeQuerier{perm: "read", ok: true}
	d := NewDeriver(q, func() time.Time { return clock })

	d.Capability(context.Background(), sess("bob"), "o", "r", "operator")
	d.Capability(context.Background(), sess("bob"), "o", "r", "operator")
	if q.calls != 1 {
		t.Fatalf("expected one query within the cache window, got %d", q.calls)
	}
	clock = clock.Add(PermissionCacheTTL + time.Second)
	d.Capability(context.Background(), sess("bob"), "o", "r", "operator")
	if q.calls != 2 {
		t.Fatalf("expected a re-query after the TTL, got %d", q.calls)
	}

	// A failing query is NOT cached — the next call retries.
	qe := &fakeQuerier{ok: false}
	de := NewDeriver(qe, func() time.Time { return clock })
	de.Capability(context.Background(), sess("carol"), "o", "r", "operator")
	de.Capability(context.Background(), sess("carol"), "o", "r", "operator")
	if qe.calls != 2 {
		t.Fatalf("a fail-closed result must not be cached; got %d calls, want 2", qe.calls)
	}
}

// ── Route table (Verify 9) ────────────────────────────────────────────────

// TestValidateRoutesRejectsUngatedRoute is the route-table walk (Verify 9): a
// data/write route registered with a tier below Commenter — i.e. WITHOUT the
// capability gate — makes ValidateRoutes fail. This is what the mutation test
// re-runs: register the comment route with Min=Anonymous and it fails.
func TestValidateRoutesRejectsUngatedRoute(t *testing.T) {
	good := []Route{
		{Pattern: "/comment/", Min: Commenter, ItemScoped: true, Handler: http.NotFoundHandler()},
		{Pattern: "/answer/", Min: Answerer, ItemScoped: true, Handler: http.NotFoundHandler()},
	}
	if err := ValidateRoutes(good); err != nil {
		t.Fatalf("a fully gated table was rejected: %v", err)
	}
	// The mutation: the comment route WITHOUT the gate.
	mutated := []Route{
		{Pattern: "/comment/", Min: Anonymous, ItemScoped: true, Handler: http.NotFoundHandler()},
		{Pattern: "/answer/", Min: Answerer, ItemScoped: true, Handler: http.NotFoundHandler()},
	}
	if err := ValidateRoutes(mutated); err == nil {
		t.Fatal("a data/write route with no capability gate passed validation — the walk is not load-bearing")
	}
	// An anonymous-read shortcut on a write route is also rejected.
	badAnon := []Route{{Pattern: "/answer/", Min: Answerer, ItemScoped: true, AllowAnonymousRead: true, Handler: http.NotFoundHandler()}}
	if err := ValidateRoutes(badAnon); err == nil {
		t.Fatal("anonymous read on a write route must be rejected")
	}
}

// ── Middleware gates (Verify 2, 4) ────────────────────────────────────────

type fakeRepos struct{ owner, name string }

func (f fakeRepos) RepoForItem(string) (string, string, bool) {
	if f.owner == "" {
		return "", "", false
	}
	return f.owner, f.name, true
}

type fakePolicy struct {
	answerAs   string
	anonOK     bool
	loginGated bool
}

func (f fakePolicy) AnswerAsForRepo(string, string) string    { return f.answerAs }
func (f fakePolicy) AnonymousReadAllowed(string, string) bool { return f.anonOK }
func (f fakePolicy) SurfaceLoginGated() bool                  { return f.loginGated }

func guardWith(perm string, permOK bool, repos RepoResolver, pol CellPolicy) (*Guard, *SessionStore) {
	store := NewSessionStore(nil)
	return &Guard{
		Sessions: store,
		Deriver:  NewDeriver(&fakeQuerier{perm: perm, ok: permOK}, nil),
		Repos:    repos,
		Policy:   pol,
		// These tests exercise the capability LADDER, so they run with the
		// the service contract Task 9 rollout gate open on every cell. The gate's
		// own behaviour — and its fail-closed default — is in rollout_test.go.
		Rollout: allOn{},
	}, store
}

func withSessionCookie(store *SessionStore, login, token string) *http.Cookie {
	s, _ := store.Begin(login, token)
	return &http.Cookie{Name: SessionCookie, Value: s.ID}
}

// TestSurfaceGateBlocksUnauth (Verify 4): a surface route on a login-gated cell
// returns 401 to an unauthenticated request, and the wrapped handler never runs
// (no data leaks pre-auth).
func TestSurfaceGateBlocksUnauth(t *testing.T) {
	ran := false
	inner := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { ran = true })
	g, _ := guardWith("", false, fakeRepos{}, fakePolicy{loginGated: true})
	h := g.Handler(Route{Pattern: "/inbox", Min: Commenter, ItemScoped: false, Handler: inner})

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/inbox", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauth surface got %d, want 401", rr.Code)
	}
	if ran {
		t.Fatal("the data handler ran pre-auth — item data would leak")
	}
}

// TestSurfaceGateOpenWhenNotLoginGated: an anonymous-opt-in verified public cell
// serves the surface anonymously.
func TestSurfaceGateOpenWhenNotLoginGated(t *testing.T) {
	ran := false
	inner := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { ran = true })
	g, _ := guardWith("", false, fakeRepos{}, fakePolicy{loginGated: false})
	h := g.Handler(Route{Pattern: "/inbox", Min: Commenter, Handler: inner})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/inbox", nil))
	if !ran || rr.Code == http.StatusUnauthorized {
		t.Fatalf("a non-login-gated surface should serve anonymously (code=%d ran=%v)", rr.Code, ran)
	}
}

// TestAnswerRouteRefusesCommenter (Verify 2): a commenter session hitting an
// answerer route is refused 403 and the handler never runs — the bypassed-UI
// row, at the middleware layer.
func TestAnswerRouteRefusesCommenter(t *testing.T) {
	ran := false
	inner := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { ran = true })
	g, store := guardWith("read", true, fakeRepos{owner: "o", name: "r"}, fakePolicy{answerAs: "operator"})
	h := g.Handler(Route{Pattern: "/answer/", Min: Answerer, ItemScoped: true, Handler: inner})

	req := httptest.NewRequest(http.MethodPost, "/answer/id-1", nil)
	req.AddCookie(withSessionCookie(store, "bob", "bob-token")) // bob: pull perm → commenter
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("commenter hitting the answer route got %d, want 403", rr.Code)
	}
	if ran {
		t.Fatal("the answer handler ran for a commenter — the gate is not enforcing")
	}
}

// TestAnswerRouteAllowsAnswerer: the answerer (login == answer_as) reaches the
// handler.
func TestAnswerRouteAllowsAnswerer(t *testing.T) {
	ran := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { ran = true; w.WriteHeader(200) })
	g, store := guardWith("none", true, fakeRepos{owner: "o", name: "r"}, fakePolicy{answerAs: "operator"})
	h := g.Handler(Route{Pattern: "/answer/", Min: Answerer, ItemScoped: true, Handler: inner})

	req := httptest.NewRequest(http.MethodPost, "/answer/id-1", nil)
	req.AddCookie(withSessionCookie(store, "operator", "k-token"))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if !ran || rr.Code != 200 {
		t.Fatalf("the answerer was blocked (code=%d ran=%v)", rr.Code, ran)
	}
}

// TestItemDataRequiresCommenter: an item data route refuses anonymous (401),
// refuses a no-permission session (403), and serves a commenter.
func TestItemDataRequiresCommenter(t *testing.T) {
	makeH := func(perm string) (http.Handler, *SessionStore, *bool) {
		ran := false
		inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { ran = true; w.WriteHeader(200) })
		g, store := guardWith(perm, true, fakeRepos{owner: "o", name: "r"}, fakePolicy{answerAs: "operator"})
		return g.Handler(Route{Pattern: "/item/", Min: Commenter, ItemScoped: true, Handler: inner}), store, &ran
	}

	// anonymous → 401
	h, _, ran := makeH("read")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/item/id-1", nil))
	if rr.Code != http.StatusUnauthorized || *ran {
		t.Fatalf("anon item got %d ran=%v, want 401 no-run", rr.Code, *ran)
	}

	// no permission → 403
	h, store, ran := makeH("none")
	req := httptest.NewRequest(http.MethodGet, "/item/id-1", nil)
	req.AddCookie(withSessionCookie(store, "bob", "t"))
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden || *ran {
		t.Fatalf("no-perm item got %d ran=%v, want 403 no-run", rr.Code, *ran)
	}

	// commenter → served
	h, store, ran = makeH("read")
	req = httptest.NewRequest(http.MethodGet, "/item/id-1", nil)
	req.AddCookie(withSessionCookie(store, "bob", "t"))
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if !*ran || rr.Code != 200 {
		t.Fatalf("commenter item got %d ran=%v, want served", rr.Code, *ran)
	}
}

// TestItemAnonymousOptIn: with a verified anonymous opt-in, a data-read item
// route serves an anonymous viewer.
func TestItemAnonymousOptIn(t *testing.T) {
	ran := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { ran = true; w.WriteHeader(200) })
	g, _ := guardWith("none", true, fakeRepos{owner: "o", name: "r"}, fakePolicy{anonOK: true})
	h := g.Handler(Route{Pattern: "/item/", Min: Commenter, ItemScoped: true, AllowAnonymousRead: true, Handler: inner})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/item/id-1", nil))
	if !ran || rr.Code != 200 {
		t.Fatalf("verified anon opt-in should serve item read (code=%d ran=%v)", rr.Code, ran)
	}
}

// TestSurfaceAuthorizer (the capability model security fix): the surface/listing per-repo
// gate. It denies an authenticated viewer with no read on a private repo, allows
// a commenter, allows anyone on a verified-public (anonymous-read) repo, denies
// an unauthenticated viewer on a private repo, and fails closed on an
// unresolvable repo — the same derivation the item route uses, lifted to the
// per-row listing.
func TestSurfaceAuthorizer(t *testing.T) {
	req := func(store *SessionStore, login string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/inbox", nil)
		if login != "" {
			r.AddCookie(withSessionCookie(store, login, "t"))
		}
		return r
	}

	// Authenticated, no read on a private (non-anon) repo → DENIED.
	g, store := guardWith("none", true, fakeRepos{}, fakePolicy{answerAs: "operator"})
	if g.SurfaceAuthorizer(req(store, "bob"))("o", "r") {
		t.Fatal("LEAK: an authenticated viewer with no read capability was authorized for a private repo's rows")
	}

	// Authenticated commenter (read) on a private repo → ALLOWED.
	g, store = guardWith("read", true, fakeRepos{}, fakePolicy{answerAs: "operator"})
	if !g.SurfaceAuthorizer(req(store, "bob"))("o", "r") {
		t.Fatal("a commenter must be authorized for the repo's rows")
	}

	// Verified anonymous-read (public) repo → ALLOWED for anyone, even anon.
	g, _ = guardWith("none", true, fakeRepos{}, fakePolicy{anonOK: true})
	if !g.SurfaceAuthorizer(req(nil, ""))("o", "r") {
		t.Fatal("a verified-public repo's rows must be visible to everyone")
	}

	// Unauthenticated viewer on a private repo → DENIED.
	g, _ = guardWith("read", true, fakeRepos{}, fakePolicy{})
	if g.SurfaceAuthorizer(req(nil, ""))("o", "r") {
		t.Fatal("an unauthenticated viewer must not see a private repo's rows")
	}

	// Unresolvable repo (empty owner/name) → DENIED, fail closed.
	g, store = guardWith("read", true, fakeRepos{}, fakePolicy{})
	if g.SurfaceAuthorizer(req(store, "bob"))("", "") {
		t.Fatal("an unresolvable repo must fail closed")
	}
}

func TestItemIDFromPath(t *testing.T) {
	cases := []struct{ prefix, path, want string }{
		{"/answer/", "/answer/abc", "abc"},
		{"/answer/", "/answer/abc/ack", "abc"},
		{"/comment/", "/comment/xyz", "xyz"},
		{"/item/", "/item/id-1", "id-1"},
		{"/answer/", "/answer/", ""},
		{"/answer/", "/item/other", ""}, // prefix mismatch
	}
	for _, c := range cases {
		if got := itemIDFromPath(c.prefix, c.path); got != c.want {
			t.Fatalf("itemIDFromPath(%q,%q)=%q want %q", c.prefix, c.path, got, c.want)
		}
	}
}
