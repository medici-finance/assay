package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestWebMCPDevFlagOff is Verify row 2's negative path: with the dev flag off
// (the deployed default), the inbox page must not register a single WebMCP
// tool. We cannot enumerate a browser's live tool list from a Go test, but
// the tools are registered exclusively from web/static/webmcp-diagnostics.js,
// which is included in the page ONLY when Deps.WebMCPDev is true — so
// asserting its absence from the rendered HTML is the exact proxy: no script,
// no registerTool calls, zero tools.
func TestWebMCPDevFlagOff(t *testing.T) {
	db := seed(t, nil, nil)
	srv, err := New(Deps{DB: db, Now: func() time.Time { return time.Now().UTC() }, WebMCPDev: false})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/inbox", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d", rr.Code)
	}
	body := rr.Body.String()
	if strings.Contains(body, "webmcp-diagnostics.js") {
		t.Fatal("dev flag off but webmcp-diagnostics.js was included — exposure creep (Verify row 2)")
	}
	if strings.Contains(body, "__webmcpDiagnostics") {
		t.Fatal("dev flag off but the WebMCP error-buffer bootstrap was rendered — exposure creep (Verify row 2)")
	}
}

// TestWebMCPDevFlagOn is the positive counterpart: with the dev flag on, the
// inbox page (and ONLY the inbox page) carries the error-buffer bootstrap and
// the tools script.
func TestWebMCPDevFlagOn(t *testing.T) {
	db := seed(t, nil, nil)
	srv, err := New(Deps{DB: db, Now: func() time.Time { return time.Now().UTC() }, WebMCPDev: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/inbox", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	body := rr.Body.String()
	if !strings.Contains(body, "webmcp-diagnostics.js") {
		t.Fatal("dev flag on but webmcp-diagnostics.js was not included")
	}
	if !strings.Contains(body, "__webmcpDiagnostics") {
		t.Fatal("dev flag on but the error-buffer bootstrap was not rendered")
	}

	// The experiment is scoped to ONE page state (the inbox/board view). The
	// roadmap pane must never carry the include, even with the flag on.
	req2 := httptest.NewRequest(http.MethodGet, "/roadmap", nil)
	rr2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr2, req2)
	if strings.Contains(rr2.Body.String(), "webmcp-diagnostics.js") {
		t.Fatal("webmcp-diagnostics.js leaked onto /roadmap — the experiment must stay scoped to the one inbox page state")
	}
}

// TestWebMCPToolsReadOnlyByConstruction is Verify row 3, automated: a static
// scan of the tool implementation source for any mutating call OR any
// network-exfiltration vector. No tool may issue a fetch with a method other
// than GET (none should fetch at all), no tool may invoke a state setter this
// app already defines (app.js's postForm/commitAnswer/ackItem, or the
// browser's storage setters), and no tool may reach the network or dynamic
// code paths by any of the other channels a browser offers beyond fetch/XHR
// (beacon, socket, image-ping, dynamic property write, eval-family, dynamic
// import). This denylist is necessarily incomplete against a determined
// adversary editing this file directly — it backstops review, it does not
// replace it — but it is deliberately kept broader than "no fetch" alone so
// the guard actually covers the exfil surface the PR claims read-only-by-
// construction enforces (pr-review-desk finding, PR #41).
func TestWebMCPToolsReadOnlyByConstruction(t *testing.T) {
	src, err := assets.ReadFile("static/webmcp-diagnostics.js")
	if err != nil {
		t.Fatalf("reading webmcp-diagnostics.js: %v", err)
	}
	body := string(src)
	forbidden := []string{
		// mutating network calls
		"fetch(",
		"XMLHttpRequest",
		// this app's own state-setting helpers
		"postForm(",
		"commitAnswer(",
		"ackItem(",
		"beginSession(",
		// browser storage / DOM / navigation setters
		".setItem(",
		".removeItem(",
		"document.cookie =",
		"innerHTML =",
		"location.href =",
		"location.assign(",
		"location.replace(",
		"\"method\":",
		"method:",
		// network-exfil vectors beyond fetch/XHR
		"sendBeacon(",
		"WebSocket(",
		"EventSource(",
		"Image(",
		".src =",
		// dynamic property writes / dynamic code — can smuggle any of the above
		"window[",
		"globalThis[",
		"eval(",
		"Function(",
		"import(",
	}
	for _, f := range forbidden {
		if strings.Contains(body, f) {
			t.Fatalf("webmcp-diagnostics.js contains %q — a diagnostic tool must be read-only by construction (Verify row 3)", f)
		}
	}
}
