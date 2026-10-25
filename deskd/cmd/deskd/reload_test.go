package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/internal/adapter"
	"github.com/medici-finance/assay/deskd/internal/fleet"
)

// authLog is a fake GitHub API that answers the permission, repo and GraphQL
// reads the permission querier and the fleet map make, and records the
// Authorization header of every request that reaches it.
type authLog struct {
	mu   sync.Mutex
	seen []string
}

func (a *authLog) since(n int) []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.seen[n:]...)
}

func (a *authLog) count() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.seen)
}

func newAuthLogAPI(t *testing.T) (*httptest.Server, *authLog) {
	t.Helper()
	log := &authLog{}
	record := func(r *http.Request) {
		log.mu.Lock()
		log.seen = append(log.seen, r.Header.Get("Authorization"))
		log.mu.Unlock()
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/{owner}/{name}/collaborators/{user}/permission", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"permission":"write"}`))
	})
	mux.HandleFunc("GET /repos/{owner}/{name}", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"private":false}`))
	})
	mux.HandleFunc("POST /graphql", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		w.Header().Set("Content-Type", "application/json")
		// One body satisfies both fleet reads: the claim-ref and open-PR
		// queries each find their own field and an empty, uncapped page.
		_, _ = w.Write([]byte(`{"data":{"repository":{"refs":{"totalCount":0,"nodes":[]},"pullRequests":{"totalCount":0,"nodes":[]}}}}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, log
}

func writeToken(t *testing.T, path, tok string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(tok+"\n"), 0o600); err != nil {
		t.Fatalf("write token file: %v", err)
	}
}

// wantOnly fails unless every header in got is exactly want, and got is
// non-empty (a read that made no request proves nothing about its token).
func wantOnly(t *testing.T, what string, got []string, want string) {
	t.Helper()
	if len(got) == 0 {
		t.Fatalf("%s made no request through the fake API", what)
	}
	for _, h := range got {
		if h != want {
			t.Fatalf("%s sent Authorization %q, want %q", what, h, want)
		}
	}
}

// TestReadConsumersReload is the authorization contract: under --tokens-dir the
// permission querier and the fleet map must read their per-org token through
// the SAME reloading [adapter.TokenSource] the escalation poller uses, so a
// token the sidecar rotates is picked up with no deskd restart — and a token
// file that disappears fails CLOSED (anonymous tier, not-known-public, no
// fresh fleet lane), never falls back to the last token it saw.
func TestReadConsumersReload(t *testing.T) {
	const owner, name, slug = "example-org", "console", "example-org/console"
	ctx := context.Background()

	setup := func(t *testing.T) (*adapter.PermissionRouter, *fleet.Cache, *authLog, string) {
		t.Helper()
		srv, log := newAuthLogAPI(t)
		dir := t.TempDir()
		path := filepath.Join(dir, "EXAMPLE_ORG")
		writeToken(t, path, "tok-double-one")
		perm, cache := buildReadConsumers(srv.URL, buildTokenSource(dir, os.Getenv), []string{owner}, time.Minute)
		return perm, cache, log, path
	}

	t.Run("permission querier picks up a rotated token", func(t *testing.T) {
		perm, _, log, path := setup(t)
		if _, ok := perm.Permission(ctx, owner, name, "bob"); !ok {
			t.Fatalf("Permission with a valid token file returned ok=false")
		}
		wantOnly(t, "Permission before rotation", log.since(0), "Bearer tok-double-one")

		n := log.count()
		writeToken(t, path, "tok-double-two")
		if _, ok := perm.Permission(ctx, owner, name, "bob"); !ok {
			t.Fatalf("Permission after rotation returned ok=false")
		}
		if _, ok := perm.RepoPrivate(ctx, owner, name); !ok {
			t.Fatalf("RepoPrivate after rotation returned ok=false")
		}
		wantOnly(t, "Permission/RepoPrivate after rotation", log.since(n), "Bearer tok-double-two")
	})

	t.Run("fleet map picks up a rotated token", func(t *testing.T) {
		_, cache, log, path := setup(t)
		cache.Snapshot(ctx, []string{slug})
		wantOnly(t, "fleet Snapshot before rotation", log.since(0), "Bearer tok-double-one")

		n := log.count()
		writeToken(t, path, "tok-double-two")
		cache.Snapshot(ctx, []string{slug})
		wantOnly(t, "fleet Snapshot after rotation", log.since(n), "Bearer tok-double-two")
	})

	t.Run("removed token file fails closed", func(t *testing.T) {
		perm, cache, log, path := setup(t)
		if _, ok := perm.Permission(ctx, owner, name, "bob"); !ok {
			t.Fatalf("Permission with a valid token file returned ok=false")
		}
		lanes := cache.Snapshot(ctx, []string{slug})
		if len(lanes) != 1 || !lanes[0].OK || lanes[0].Stale {
			t.Fatalf("fleet Snapshot with a valid token file = %+v, want one fresh OK lane", lanes)
		}

		n := log.count()
		if err := os.Remove(path); err != nil {
			t.Fatalf("remove token file: %v", err)
		}
		if p, ok := perm.Permission(ctx, owner, name, "bob"); ok {
			t.Fatalf("Permission with the token file removed = %q ok=true, want ok=false (anonymous tier)", p)
		}
		if private, ok := perm.RepoPrivate(ctx, owner, name); ok || !private {
			t.Fatalf("RepoPrivate with the token file removed = private=%v ok=%v, want private=true ok=false", private, ok)
		}
		lanes = cache.Snapshot(ctx, []string{slug})
		if len(lanes) != 1 || (lanes[0].OK && !lanes[0].Stale) {
			t.Fatalf("fleet Snapshot with the token file removed = %+v, want could-not-check or Stale, never fresh", lanes)
		}
		if got := log.since(n); len(got) != 0 {
			t.Fatalf("with the token file removed, %d request(s) still reached the API (Authorization %v) — a vanished token must stop every read, not reuse the last one", len(got), got)
		}
	})

	// The env source resolves the legacy DESKD_GITHUB_TOKEN default for ANY
	// owner. Resolving per read must not let that default reach an org this
	// deskd was not configured for: the start-time map only ever held the
	// configured owners, and the per-read path keeps that boundary.
	t.Run("unconfigured owner is never resolved", func(t *testing.T) {
		srv, log := newAuthLogAPI(t)
		getenv := func(k string) string {
			if k == tokenEnv {
				return "tok-double-default"
			}
			return ""
		}
		perm, cache := buildReadConsumers(srv.URL, buildTokenSource("", getenv), []string{owner}, time.Minute)

		// Positive control: the configured owner does resolve the default.
		if _, ok := perm.Permission(ctx, owner, name, "bob"); !ok {
			t.Fatalf("Permission for the configured owner returned ok=false")
		}
		wantOnly(t, "Permission for the configured owner", log.since(0), "Bearer tok-double-default")

		n := log.count()
		if _, ok := perm.Permission(ctx, "other-org", "repo", "bob"); ok {
			t.Fatalf("Permission for an unconfigured owner returned ok=true")
		}
		if private, ok := perm.RepoPrivate(ctx, "other-org", "repo"); ok || !private {
			t.Fatalf("RepoPrivate for an unconfigured owner = private=%v ok=%v, want private=true ok=false", private, ok)
		}
		if lanes := cache.Snapshot(ctx, []string{"other-org/repo"}); len(lanes) != 1 || lanes[0].OK {
			t.Fatalf("fleet Snapshot for an unconfigured owner = %+v, want one could-not-check lane", lanes)
		}
		if got := log.since(n); len(got) != 0 {
			t.Fatalf("an unconfigured owner was read %d time(s) with the default token %v", len(got), got)
		}
	})
}
