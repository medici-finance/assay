package main

// installidcache_test.go — desk-tools brief 25, Verify rows 8 to 14.
//
// The three assertions #1036 asks for are rows 8, 9 and 11; rows 10, 12, 13 and 14 are the
// fall-through, bypass and self-heal routes that make the fast path safe to have.
//
// Every row here is about a cache in front of a CREDENTIAL, so the negative cases matter as
// much as the positive one: row 12 is four ways of NOT matching, each asserted to fall
// through to the resolution that ran before this cache existed.

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// refusingTransport fails EVERY request and records what was attempted. It is how "zero
// network calls" is asserted as a property rather than hoped for: if any code path reaches
// the network the run fails and the test names the request.
type refusingTransport struct {
	mu   sync.Mutex
	reqs []string
}

func (rt *refusingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	rt.mu.Lock()
	rt.reqs = append(rt.reqs, r.Method+" "+r.URL.Path)
	rt.mu.Unlock()
	return nil, fmt.Errorf("network call attempted: %s %s", r.Method, r.URL.Path)
}

func (rt *refusingTransport) seen() []string {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return append([]string(nil), rt.reqs...)
}

// countingTransport records every request it forwards, so a row can assert the exact number
// AND order of calls rather than "some requests happened".
type countingTransport struct {
	inner http.RoundTripper
	mu    sync.Mutex
	reqs  []string
}

func (ct *countingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	ct.mu.Lock()
	ct.reqs = append(ct.reqs, r.Method+" "+r.URL.Path)
	ct.mu.Unlock()
	return ct.inner.RoundTrip(r)
}

func (ct *countingTransport) seen() []string {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	return append([]string(nil), ct.reqs...)
}

// --- fixture helpers ---------------------------------------------------------------------

const testInstallID = "100000004"

func exampleInstalls(id int64, login string) []installationInfo {
	inst := installationInfo{ID: id}
	inst.Account.Login = login
	return []installationInfo{inst}
}

// configDir is where setupTest's HOME puts the App-credential plane.
func configDir(home string) string { return filepath.Join(home, ".config", "assay") }

// seedKeyAndAppID plants the App private key and App ID for the default reviewer binding.
func seedKeyAndAppID(t *testing.T, home string) {
	t.Helper()
	t.Setenv("REVIEWER_APP_ID", "12345")
	writeFileMode(t, filepath.Join(configDir(home), "reviewer-app.pem"), makePEM(t), 0o600)
}

// seedTokenCache plants a token cache file of the given age, with the owner sidecar that
// makes it recognisable to the probe. ageMinutes < 50 is "fresh".
func seedTokenCache(t *testing.T, home, installID string, ageMinutes int, sidecar string) string {
	t.Helper()
	p := filepath.Join(configDir(home), "reviewer-token-"+installID)
	writeTokenCache(t, p, "ghs_cached_token")
	mt := time.Now().Add(-time.Duration(ageMinutes) * time.Minute)
	if err := os.Chtimes(p, mt, mt); err != nil {
		t.Fatalf("chtimes %s: %v", p, err)
	}
	if sidecar != "" {
		writeFileMode(t, ownerSidecarPath(p), sidecar, 0o600)
	}
	return p
}

// seedInstallIDCache plants the per-(App, account) install-id cache with the given age.
func seedInstallIDCache(t *testing.T, home, appName, owner, id string, age time.Duration) string {
	t.Helper()
	p := filepath.Join(configDir(home), appName+"-install-"+owner)
	writeFileMode(t, p, id+"\n", 0o600)
	mt := time.Now().Add(-age)
	if err := os.Chtimes(p, mt, mt); err != nil {
		t.Fatalf("chtimes %s: %v", p, err)
	}
	return p
}

// useCountingServer points httpClient at a counting transport in front of the standard
// install+token server, and returns the recorder.
func useCountingServer(t *testing.T, installs []installationInfo, token string) *countingTransport {
	t.Helper()
	srv, _ := makeInstallTokenServer(t, installs, token, "2124-01-01T00:00:00Z")
	t.Cleanup(srv.Close)
	ct := &countingTransport{inner: &rewriteTransport{orig: srv.URL}}
	old := httpClient
	httpClient = &http.Client{Transport: ct}
	t.Cleanup(func() { httpClient = old })
	return ct
}

// --- row 8 -------------------------------------------------------------------------------

// TestWarmCacheMakesNoNetworkCall — Verify row 8, and #1036's first Verify assertion.
// A warm token cache plus a matching owner sidecar, no <PREFIX>_INSTALL_ID: the run must
// make NO network call at all. Before this change the same run signed a JWT and called
// GET /app/installations to discover the id the cache path is keyed by.
func TestWarmCacheMakesNoNetworkCall(t *testing.T) {
	home := setupTest(t)
	seedKeyAndAppID(t, home)
	tokenPath := seedTokenCache(t, home, testInstallID, 5, "reviewer-app example-org\n")

	rt := &refusingTransport{}
	old := httpClient
	httpClient = &http.Client{Transport: rt}
	defer func() { httpClient = old }()

	rc, stdout, stderr := runCap(t, []string{"reviewer", "--repo", "example-org/tracker"})
	if rc != deskkit.ExitOK {
		t.Fatalf("warm-cache run rc = %d, want 0; stderr: %s", rc, stderr)
	}
	if !strings.Contains(stdout, tokenPath) {
		t.Fatalf("stdout should name the cached token path %s; got: %s", tokenPath, stdout)
	}
	if strings.Contains(stdout, "ghs_cached_token") {
		t.Fatalf("token value leaked to stdout: %s", stdout)
	}
	if reqs := rt.seen(); len(reqs) != 0 {
		t.Fatalf("a warm cache hit made %d network call(s): %v — the whole point of the row is that it makes none", len(reqs), reqs)
	}
	entries := auditEntries(t)
	if len(entries) == 0 || !strings.Contains(entries[len(entries)-1].Detail, "reused cached") {
		t.Fatalf("expected a cache-reuse audit row; got %+v", entries)
	}
}

// --- row 9 -------------------------------------------------------------------------------

// TestStaleCacheMintsExactlyOnce — Verify row 9, and #1036's second Verify assertion.
func TestStaleCacheMintsExactlyOnce(t *testing.T) {
	t.Run("stale token cache, warm install-id cache: one call, the exchange", func(t *testing.T) {
		home := setupTest(t)
		seedKeyAndAppID(t, home)
		seedTokenCache(t, home, testInstallID, 51, "reviewer-app example-org\n")
		seedInstallIDCache(t, home, "reviewer-app", "example-org", testInstallID, time.Hour)

		ct := useCountingServer(t, exampleInstalls(100000004, "example-org"), "ghs_minted")

		rc, _, stderr := runCap(t, []string{"reviewer", "--repo", "example-org/tracker"})
		if rc != deskkit.ExitOK {
			t.Fatalf("stale-cache run rc = %d, want 0; stderr: %s", rc, stderr)
		}
		reqs := ct.seen()
		if len(reqs) != 1 {
			t.Fatalf("a stale cache with a warm install-id cache made %d call(s): %v — want exactly 1 (the exchange)", len(reqs), reqs)
		}
		if !strings.Contains(reqs[0], "/app/installations/"+testInstallID+"/access_tokens") {
			t.Fatalf("the one call was %q, want the access-tokens exchange for install %s", reqs[0], testInstallID)
		}
	})

	t.Run("everything cold: two calls, installations then exchange", func(t *testing.T) {
		home := setupTest(t)
		seedKeyAndAppID(t, home)

		ct := useCountingServer(t, exampleInstalls(100000004, "example-org"), "ghs_minted")

		rc, _, stderr := runCap(t, []string{"reviewer", "--repo", "example-org/tracker"})
		if rc != deskkit.ExitOK {
			t.Fatalf("cold run rc = %d, want 0; stderr: %s", rc, stderr)
		}
		reqs := ct.seen()
		if len(reqs) != 2 {
			t.Fatalf("a cold run made %d call(s): %v — want exactly 2", len(reqs), reqs)
		}
		if !strings.Contains(reqs[0], "/app/installations") || strings.Contains(reqs[0], "access_tokens") {
			t.Fatalf("first call was %q, want the installations lookup", reqs[0])
		}
		if !strings.Contains(reqs[1], "access_tokens") {
			t.Fatalf("second call was %q, want the access-tokens exchange", reqs[1])
		}
		// The resolution must be recorded, so the NEXT cold token cache costs no lookup.
		if _, ok := readInstallIDCache("reviewer-app", "example-org"); !ok {
			t.Fatal("a resolved installation id was not written to the install-id cache")
		}
		// And the minted token must carry the sidecar that makes it findable.
		p := filepath.Join(configDir(home), "reviewer-token-"+testInstallID)
		if !sidecarMatches(p, "reviewer-app", "example-org") {
			t.Fatalf("the minted token at %s has no matching .owner sidecar", p)
		}
	})
}

// --- row 10 ------------------------------------------------------------------------------

// TestInstallIDCacheHitAndExpiry — Verify row 10.
func TestInstallIDCacheHitAndExpiry(t *testing.T) {
	t.Run("23 hours old: used, no installations lookup", func(t *testing.T) {
		home := setupTest(t)
		seedKeyAndAppID(t, home)
		seedInstallIDCache(t, home, "reviewer-app", "example-org", testInstallID, 23*time.Hour)
		ct := useCountingServer(t, exampleInstalls(100000004, "example-org"), "ghs_minted")

		rc, _, stderr := runCap(t, []string{"reviewer", "--repo", "example-org/tracker"})
		if rc != deskkit.ExitOK {
			t.Fatalf("rc = %d, want 0; stderr: %s", rc, stderr)
		}
		for _, r := range ct.seen() {
			if strings.HasSuffix(r, "/app/installations") {
				t.Fatalf("a 23-hour-old install-id cache still made an installations lookup: %v", ct.seen())
			}
		}
	})

	t.Run("25 hours old: not used, and rewritten", func(t *testing.T) {
		home := setupTest(t)
		seedKeyAndAppID(t, home)
		p := seedInstallIDCache(t, home, "reviewer-app", "example-org", testInstallID, 25*time.Hour)
		ct := useCountingServer(t, exampleInstalls(100000004, "example-org"), "ghs_minted")

		rc, _, stderr := runCap(t, []string{"reviewer", "--repo", "example-org/tracker"})
		if rc != deskkit.ExitOK {
			t.Fatalf("rc = %d, want 0; stderr: %s", rc, stderr)
		}
		var lookups int
		for _, r := range ct.seen() {
			if strings.HasSuffix(r, "/app/installations") {
				lookups++
			}
		}
		if lookups != 1 {
			t.Fatalf("an expired install-id cache produced %d installations lookup(s), want exactly 1: %v", lookups, ct.seen())
		}
		fi, err := os.Stat(p)
		if err != nil {
			t.Fatalf("stat rewritten cache: %v", err)
		}
		if time.Since(fi.ModTime()) > time.Minute {
			t.Fatalf("the install-id cache was not rewritten after re-resolution (mtime %v)", fi.ModTime())
		}
	})

	t.Run("non-digit content is ignored, never parsed", func(t *testing.T) {
		home := setupTest(t)
		writeFileMode(t, filepath.Join(configDir(home), "reviewer-app-install-example-org"), "not-an-id\n", 0o600)
		if id, ok := readInstallIDCache("reviewer-app", "example-org"); ok {
			t.Fatalf("a non-digit install-id cache was accepted as %q", id)
		}
	})

	t.Run("a world-readable install-id cache is refused", func(t *testing.T) {
		home := setupTest(t)
		writeFileMode(t, filepath.Join(configDir(home), "reviewer-app-install-example-org"), testInstallID+"\n", 0o644)
		if _, ok := readInstallIDCache("reviewer-app", "example-org"); ok {
			t.Fatal("a 0644 install-id cache was accepted; the custody bar is 0600")
		}
	})
}

// --- row 11 ------------------------------------------------------------------------------

// TestTwoAppsOnOneOwnerNeverShareACacheFile — Verify row 11, and #1036's third Verify
// assertion. Two App bindings for ONE role on ONE account: the files are distinct, and a
// candidate whose sidecar names the OTHER App is rejected by the probe.
func TestTwoAppsOnOneOwnerNeverShareACacheFile(t *testing.T) {
	home := setupTest(t)

	// App one (the default binding) mints install 100000004 and owns that cache file.
	first := seedTokenCache(t, home, testInstallID, 5, "reviewer-app example-org\n")

	// App two (bound as x-act) mints a DIFFERENT installation on the same account.
	const secondID = "100000009"
	second := filepath.Join(configDir(home), "reviewer-token-"+secondID)
	writeTokenCache(t, second, "ghs_other_app_token")
	writeFileMode(t, ownerSidecarPath(second), "x-act example-org\n", 0o600)

	if first == second {
		t.Fatal("two installations must not share a cache file name")
	}

	// The probe for App one must find App one's file and only App one's file.
	got, ok := probeCachedInstallID("reviewer", "reviewer-app", "example-org")
	if !ok || got != testInstallID {
		t.Fatalf("probe for reviewer-app returned (%q, %v), want (%q, true)", got, ok, testInstallID)
	}
	// And the probe for App two must find App two's.
	got, ok = probeCachedInstallID("reviewer", "x-act", "example-org")
	if !ok || got != secondID {
		t.Fatalf("probe for x-act returned (%q, %v), want (%q, true)", got, ok, secondID)
	}
	// Neither App's sidecar may satisfy the other.
	if sidecarMatches(first, "x-act", "example-org") {
		t.Fatal("App x-act matched a cache file minted by reviewer-app — two Apps must never share a cache")
	}
	if sidecarMatches(second, "reviewer-app", "example-org") {
		t.Fatal("App reviewer-app matched a cache file minted by x-act — two Apps must never share a cache")
	}

	// End to end: a run bound to x-act, with only reviewer-app's file fresh, must NOT read
	// it — it falls through and resolves.
	home2 := setupTest(t)
	t.Setenv("REVIEWER_APP", "x-act")
	t.Setenv("X_ACT_APP_ID", "424242")
	writeFileMode(t, filepath.Join(configDir(home2), "x-act.pem"), makePEM(t), 0o600)
	seedTokenCache(t, home2, testInstallID, 5, "reviewer-app example-org\n")
	ct := useCountingServer(t, exampleInstalls(100000009, "example-org"), "ghs_minted_by_x_act")

	rc, stdout, stderr := runCap(t, []string{"reviewer", "--repo", "example-org/tracker"})
	if rc != deskkit.ExitOK {
		t.Fatalf("x-act run rc = %d, want 0; stderr: %s", rc, stderr)
	}
	if strings.Contains(stdout, "reviewer-token-"+testInstallID) {
		t.Fatalf("the x-act run reused reviewer-app's cache file: %s", stdout)
	}
	var lookups int
	for _, r := range ct.seen() {
		if strings.HasSuffix(r, "/app/installations") {
			lookups++
		}
	}
	if lookups != 1 {
		t.Fatalf("the x-act run made %d installations lookup(s), want 1 — it must not have trusted another App's cache: %v", lookups, ct.seen())
	}
}

// --- row 12 ------------------------------------------------------------------------------

// TestProbeFallsThroughOnAmbiguityAndMalformedInput — Verify row 12, the NEGATIVE control.
// Four ways of not matching, each of which must fall through to authoritative resolution.
func TestProbeFallsThroughOnAmbiguityAndMalformedInput(t *testing.T) {
	t.Run("two equally fresh candidates", func(t *testing.T) {
		home := setupTest(t)
		seedTokenCache(t, home, "100000004", 5, "reviewer-app example-org\n")
		seedTokenCache(t, home, "100000009", 5, "reviewer-app example-org\n")
		if id, ok := probeCachedInstallID("reviewer", "reviewer-app", "example-org"); ok {
			t.Fatalf("two distinct fresh candidates resolved to %q; ambiguity must fall through", id)
		}
	})

	t.Run("no sidecar", func(t *testing.T) {
		home := setupTest(t)
		seedTokenCache(t, home, testInstallID, 5, "")
		if id, ok := probeCachedInstallID("reviewer", "reviewer-app", "example-org"); ok {
			t.Fatalf("a cache file with no .owner sidecar was accepted as %q", id)
		}
	})

	t.Run("world-readable candidate", func(t *testing.T) {
		home := setupTest(t)
		p := filepath.Join(configDir(home), "reviewer-token-"+testInstallID)
		writeFileMode(t, p, "ghs_loose", 0o644)
		writeFileMode(t, ownerSidecarPath(p), "reviewer-app example-org\n", 0o600)
		if id, ok := probeCachedInstallID("reviewer", "reviewer-app", "example-org"); ok {
			t.Fatalf("a 0644 cache file was accepted as %q; the custody bar is 0600", id)
		}
	})

	t.Run("stale candidate", func(t *testing.T) {
		home := setupTest(t)
		seedTokenCache(t, home, testInstallID, 51, "reviewer-app example-org\n")
		if id, ok := probeCachedInstallID("reviewer", "reviewer-app", "example-org"); ok {
			t.Fatalf("a 51-minute-old cache file was accepted as %q; the reuse window is 50m", id)
		}
	})

	t.Run("account name outside the accepted set builds no path", func(t *testing.T) {
		home := setupTest(t)
		const badOwner = "bad!owner"
		if _, ok := installIDCacheName("reviewer-app", badOwner); ok {
			t.Fatal("a malformed account name produced a cache file name")
		}
		if _, ok := probeCachedInstallID("reviewer", "reviewer-app", badOwner); ok {
			t.Fatal("a malformed account name was probed for")
		}
		seedKeyAndAppID(t, home)
		ct := useCountingServer(t, exampleInstalls(100000004, badOwner), "ghs_minted")
		rc, _, stderr := runCap(t, []string{"reviewer", "--repo", badOwner + "/tracker"})
		if rc != deskkit.ExitOK {
			t.Fatalf("rc = %d, want 0 (the malformed account must still resolve normally); stderr: %s", rc, stderr)
		}
		var lookups int
		for _, r := range ct.seen() {
			if strings.HasSuffix(r, "/app/installations") {
				lookups++
			}
		}
		if lookups != 1 {
			t.Fatalf("a malformed account made %d installations lookup(s), want 1: %v", lookups, ct.seen())
		}
		// No cache file may have been constructed from the malformed name.
		matches, _ := filepath.Glob(filepath.Join(configDir(home), "*install*"))
		for _, m := range matches {
			if strings.Contains(filepath.Base(m), badOwner) {
				t.Fatalf("a path was built from the malformed account name: %s", m)
			}
		}
	})
}

// --- row 13 ------------------------------------------------------------------------------

// TestFreshBypassesProbeAndInstallIDCache — Verify row 13. --fresh means "the App or its
// permissions may have changed"; a cache that short-circuited it would make it a no-op.
func TestFreshBypassesProbeAndInstallIDCache(t *testing.T) {
	home := setupTest(t)
	seedKeyAndAppID(t, home)
	tokenPath := seedTokenCache(t, home, testInstallID, 5, "reviewer-app example-org\n")
	writeFileMode(t, permsPath(tokenPath), `{"contents":"read"}`, 0o600)
	seedInstallIDCache(t, home, "reviewer-app", "example-org", testInstallID, time.Hour)

	ct := useCountingServer(t, exampleInstalls(100000004, "example-org"), "ghs_fresh_mint")

	rc, _, stderr := runCap(t, []string{"reviewer", "--repo", "example-org/tracker", "--fresh"})
	if rc != deskkit.ExitOK {
		t.Fatalf("--fresh rc = %d, want 0; stderr: %s", rc, stderr)
	}
	reqs := ct.seen()
	if len(reqs) != 2 {
		t.Fatalf("--fresh with every cache warm made %d call(s): %v — want 2 (installations, then exchange)", len(reqs), reqs)
	}
	b, err := os.ReadFile(tokenPath)
	if err != nil {
		t.Fatalf("read re-minted token: %v", err)
	}
	if strings.TrimSpace(string(b)) != "ghs_fresh_mint" {
		t.Fatalf("--fresh did not re-mint: token file holds %q", strings.TrimSpace(string(b)))
	}
	if !sidecarMatches(tokenPath, "reviewer-app", "example-org") {
		t.Fatal("--fresh removed the .owner sidecar and did not write a new one")
	}
}

// --- row 14 ------------------------------------------------------------------------------

// TestExchange404OnCachedInstallIDInvalidatesIt — Verify row 14. A cached id that GitHub no
// longer knows must not fail every call for the rest of its TTL.
func TestExchange404OnCachedInstallIDInvalidatesIt(t *testing.T) {
	// A server whose exchange always 404s, so the cached id is provably rejected.
	newGoneServer := func(t *testing.T) *countingTransport {
		t.Helper()
		mux := http.NewServeMux()
		mux.HandleFunc("GET /app/installations", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
		})
		mux.HandleFunc("POST /app/installations/", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"message":"Not Found"}`, 404)
		})
		srv := httptest.NewServer(mux)
		t.Cleanup(srv.Close)
		ct := &countingTransport{inner: &rewriteTransport{orig: srv.URL}}
		old := httpClient
		httpClient = &http.Client{Transport: ct}
		t.Cleanup(func() { httpClient = old })
		return ct
	}

	t.Run("a cached id is invalidated", func(t *testing.T) {
		home := setupTest(t)
		seedKeyAndAppID(t, home)
		cachePath := seedInstallIDCache(t, home, "reviewer-app", "example-org", testInstallID, time.Hour)
		newGoneServer(t)

		rc, _, stderr := runCap(t, []string{"reviewer", "--repo", "example-org/tracker"})
		if rc == deskkit.ExitOK {
			t.Fatal("a 404 from the exchange must not succeed")
		}
		if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
			t.Fatalf("the stale install-id cache at %s survived a 404 (stat err: %v)", cachePath, err)
		}
		entries := auditEntries(t)
		if len(entries) == 0 || !strings.Contains(entries[len(entries)-1].Detail, "has been removed") {
			t.Fatalf("the refusal must say the cached id was removed; audit detail: %+v", entries)
		}
		_ = stderr
	})

	t.Run("an env-override id is never invalidated", func(t *testing.T) {
		home := setupTest(t)
		seedKeyAndAppID(t, home)
		t.Setenv("REVIEWER_INSTALL_ID", testInstallID)
		cachePath := seedInstallIDCache(t, home, "reviewer-app", "example-org", testInstallID, time.Hour)
		newGoneServer(t)

		rc, _, _ := runCap(t, []string{"reviewer", "--repo", "example-org/tracker"})
		if rc == deskkit.ExitOK {
			t.Fatal("a 404 from the exchange must not succeed")
		}
		if _, err := os.Stat(cachePath); err != nil {
			t.Fatalf("an override-driven 404 removed a cache file it did not use: %v", err)
		}
	})
}
