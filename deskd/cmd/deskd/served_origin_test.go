package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// originDeps is walkDeps with both route-owned stand-ins answering
// "authenticated", so a refusal on a /mcp or /v1 write can only come from the
// request-origin floor, never from the capability gate.
func originDeps() servedDeps {
	d := walkDeps()
	yes := func(*http.Request) bool { return true }
	d.Routes.MCPAuthenticated = yes
	d.Routes.V1Authenticated = yes
	return d
}

func serveOne(t *testing.T, top http.Handler, method, path, contentType string, hdr map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`))
	req.Host = "127.0.0.1:8787"
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	top.ServeHTTP(rec, req)
	return rec
}

// regression: F-security-surface — only the terminal actions checked the
// request's origin. A page on any other origin could POST a text/plain body
// (a "simple" request, no preflight) to /mcp, the legacy answer and session
// forms, and the /v1 writes of a loopback deskd.
func TestServedWritesRefuseCrossSite(t *testing.T) {
	top, err := buildServedMux(originDeps())
	if err != nil {
		t.Fatal(err)
	}
	crossSite := map[string]string{"Origin": "http://attacker.example", "Sec-Fetch-Site": "cross-site"}
	for _, path := range []string{"/mcp", "/answer/E1", "/session/begin", "/v1/cells/assay/overrides", "/escalations/E1/read"} {
		rec := serveOne(t, top, http.MethodPost, path, "text/plain", crossSite)
		if rec.Code != http.StatusForbidden || rec.Header().Get(reachedHeader) != "" {
			t.Errorf("cross-site POST %s answered %d (reached %q), want 403 before any handler", path, rec.Code, rec.Header().Get(reachedHeader))
		}
	}

	// DNS rebinding: same-origin to the browser, but the Host is not ours.
	rebound := map[string]string{"Origin": "http://rebound.example:8787", "Sec-Fetch-Site": "same-origin"}
	req := httptest.NewRequest(http.MethodGet, "/v1/inbox", nil)
	req.Host = "rebound.example:8787"
	for k, v := range rebound {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	top.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || rec.Header().Get(reachedHeader) != "" {
		t.Errorf("a rebound-host read answered %d (reached %q), want 403", rec.Code, rec.Header().Get(reachedHeader))
	}

	// The agent surface takes JSON only, even with no browser headers.
	if rec := serveOne(t, top, http.MethodPost, "/mcp", "text/plain", nil); rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("text/plain POST /mcp answered %d, want 415", rec.Code)
	}

	// Positive controls: a loop's JSON call and a same-origin browser write
	// both reach their handlers.
	if rec := serveOne(t, top, http.MethodPost, "/mcp", "application/json", nil); rec.Header().Get(reachedHeader) != "mcp" {
		t.Errorf("a loop's JSON POST /mcp answered %d and did not reach the handler", rec.Code)
	}
	same := map[string]string{"Origin": "http://127.0.0.1:8787", "Sec-Fetch-Site": "same-origin"}
	if rec := serveOne(t, top, http.MethodPost, "/session/begin", "application/x-www-form-urlencoded", same); rec.Header().Get(reachedHeader) != "session" {
		t.Errorf("a same-origin session form answered %d and did not reach the handler", rec.Code)
	}
}
