package adapter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// fakePermAPI records the Authorization header it saw, so a test can prove the
// query used the OWNER's own read token, never another org's.
func fakePermAPI(t *testing.T, permission string, status int) (*httptest.Server, *string) {
	t.Helper()
	var sawAuth string
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/{owner}/{name}/collaborators/{user}/permission", func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization")
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"permission":"` + permission + `"}`))
	})
	mux.HandleFunc("GET /repos/{owner}/{name}", func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization")
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		private := "false"
		if permission == "PRIVATE" {
			private = "true"
		}
		_, _ = w.Write([]byte(`{"private":` + private + `}`))
	})
	srv := httptest.NewServer(mux)
	return srv, &sawAuth
}

func TestPermissionUsesOwnerToken(t *testing.T) {
	srv, sawAuth := fakePermAPI(t, "write", http.StatusOK)
	defer srv.Close()
	pr := permRouterFor(srv.URL, map[string]string{"acme": "acme-read-token"})

	perm, ok := pr.Permission(context.Background(), "acme", "widgets", "bob")
	if !ok || perm != "write" {
		t.Fatalf("got %q ok=%v, want write true", perm, ok)
	}
	if *sawAuth != "Bearer acme-read-token" {
		t.Fatalf("query used %q — must use the OWNER's own read token", *sawAuth)
	}
}

func TestPermissionFailsClosed(t *testing.T) {
	// Unmapped owner: never read with another org's token.
	srv, _ := fakePermAPI(t, "admin", http.StatusOK)
	defer srv.Close()
	pr := permRouterFor(srv.URL, map[string]string{"acme": "acme-token"})
	if _, ok := pr.Permission(context.Background(), "other-org", "repo", "bob"); ok {
		t.Fatal("an unmapped owner must fail closed (ok=false), never cross-read")
	}

	// 404 (user has no access) → fail closed.
	srv404, _ := fakePermAPI(t, "", http.StatusNotFound)
	defer srv404.Close()
	pr404 := permRouterFor(srv404.URL, map[string]string{"acme": "t"})
	if _, ok := pr404.Permission(context.Background(), "acme", "repo", "bob"); ok {
		t.Fatal("a 404 must fail closed")
	}
}

func TestRepoPrivateFailsClosed(t *testing.T) {
	// A 404 (unverifiable) is treated as PRIVATE and not-ok — never served
	// anonymously.
	srv, _ := fakePermAPI(t, "", http.StatusNotFound)
	defer srv.Close()
	pr := permRouterFor(srv.URL, map[string]string{"acme": "t"})
	private, ok := pr.RepoPrivate(context.Background(), "acme", "repo")
	if ok || !private {
		t.Fatalf("unverifiable repo got private=%v ok=%v, want private=true ok=false (fail closed)", private, ok)
	}

	// A confirmed public repo.
	pub, _ := fakePermAPI(t, "public", http.StatusOK)
	defer pub.Close()
	prp := permRouterFor(pub.URL, map[string]string{"acme": "t"})
	private, ok = prp.RepoPrivate(context.Background(), "acme", "repo")
	if !ok || private {
		t.Fatalf("public repo got private=%v ok=%v, want private=false ok=true", private, ok)
	}

	// A confirmed private repo.
	priv, _ := fakePermAPI(t, "PRIVATE", http.StatusOK)
	defer priv.Close()
	prpv := permRouterFor(priv.URL, map[string]string{"acme": "t"})
	private, ok = prpv.RepoPrivate(context.Background(), "acme", "repo")
	if !ok || !private {
		t.Fatalf("private repo got private=%v ok=%v, want private=true ok=true", private, ok)
	}
}

// mapSource is a test-only [TokenSource] over a fixed owner->token map. It
// exists only in tests: production code gets a TokenSource from the env or the
// token-file directory, both re-read on every Resolve.
type mapSource map[string]string

func (m mapSource) Resolve(owners []string) (map[string]string, []MissingToken) {
	out := map[string]string{}
	var missing []MissingToken
	for _, o := range owners {
		if t, ok := m[o]; ok && t != "" {
			out[o] = t
		} else {
			missing = append(missing, MissingToken{Owner: o, Detail: "test map"})
		}
	}
	return out, missing
}

// permRouterFor builds a router configured for exactly the map's owners.
func permRouterFor(apiBase string, tokens map[string]string) *PermissionRouter {
	owners := make([]string, 0, len(tokens))
	for o := range tokens {
		owners = append(owners, o)
	}
	return NewPermissionRouter(apiBase, mapSource(tokens), owners, nil)
}
