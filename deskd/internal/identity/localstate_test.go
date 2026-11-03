package identity

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestLocalStateAllowedIsNeverAnonymous pins the gate POST
// /v1/escalations/{id}/read runs (the service contract increment 4) to the gate the
// un-versioned POST /escalations/{id}/read marker is mounted behind: item-scoped,
// Min=Commenter, no anonymous shortcut. The case that separates it from
// [Guard.SurfaceAuthorizer] is the verified anonymous-read repo — a READ is open
// to everyone there, a local-state WRITE is not.
func TestLocalStateAllowedIsNeverAnonymous(t *testing.T) {
	req := func(store *SessionStore, login string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/v1/escalations/E1/read", nil)
		if login != "" {
			r.AddCookie(withSessionCookie(store, login, "t"))
		}
		return r
	}

	// Anonymous on a verified anonymous-read (public) repo: the read predicate
	// admits it, the local-state gate must not.
	g, _ := guardWith("none", true, fakeRepos{}, fakePolicy{anonOK: true})
	if !g.SurfaceAuthorizer(req(nil, ""))("o", "r") {
		t.Fatal("fixture: the anonymous-read repo should be readable by everyone")
	}
	if authn, ok := g.LocalStateAllowed(req(nil, ""), "o", "r"); authn || ok {
		t.Fatalf("LocalStateAllowed(anonymous, anon-opt-in repo) = (%v, %v), want (false, false) — a local-state write is never anonymous, even where reads are", authn, ok)
	}

	// Authenticated, no read capability on the repo → authenticated, refused.
	g, store := guardWith("none", true, fakeRepos{}, fakePolicy{answerAs: "operator"})
	if authn, ok := g.LocalStateAllowed(req(store, "bob"), "o", "r"); !authn || ok {
		t.Fatalf("LocalStateAllowed(no-permission viewer) = (%v, %v), want (true, false)", authn, ok)
	}

	// Authenticated commenter → allowed.
	g, store = guardWith("read", true, fakeRepos{}, fakePolicy{answerAs: "operator"})
	if authn, ok := g.LocalStateAllowed(req(store, "bob"), "o", "r"); !authn || !ok {
		t.Fatalf("LocalStateAllowed(commenter) = (%v, %v), want (true, true)", authn, ok)
	}

	// Unresolvable repo → authenticated, refused (fail closed).
	if authn, ok := g.LocalStateAllowed(req(store, "bob"), "", ""); !authn || ok {
		t.Fatalf("LocalStateAllowed(unresolvable repo) = (%v, %v), want (true, false)", authn, ok)
	}

	// Rollout gate OFF on the cell: the one derivation yields anonymous for
	// every viewer, so even a permissioned session is refused.
	g, store = guardWith("admin", true, fakeRepos{}, fakePolicy{answerAs: "operator"})
	g.Rollout = nil
	if authn, ok := g.LocalStateAllowed(req(store, "bob"), "o", "r"); !authn || ok {
		t.Fatalf("LocalStateAllowed(rollout OFF) = (%v, %v), want (true, false) — an OFF cell derives nobody above anonymous", authn, ok)
	}
}
