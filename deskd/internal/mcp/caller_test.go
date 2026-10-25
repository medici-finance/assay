package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// This file pins the CALLER-AUTHENTICATION half of the service contract P3.
//
// The route-boundary review found /mcp mounted outside both gates
// (cmd/deskd/main.go:341, ahead of the capability loop at :347-349) and
// performing no authentication of its own: the caller login came from the
// X-Desk-Caller header, which the code comments describe as something "the
// cluster's per-pod auth sets from the pod's App identity". With --addr
// 0.0.0.0 and no such proxy in front, POST /mcp was reachable unauthenticated
// and desk__escalate / desk__report_file / desk__claim would write as
// assay-desk-app[bot] with a caller login the requester supplied.
//
// Two properties close it, and both are asserted here:
//
//  1. an unauthenticated caller is refused, and
//  2. the X-Desk-Caller header is never trusted from one.

const rpcInitialize = `{"jsonrpc":"2.0","id":1,"method":"initialize"}`

// TestMCPRefusesUnauthenticatedCaller: with no caller authenticator wired, the
// surface trusts the MACHINE boundary only — a request that did not originate
// from loopback is refused with 401 before any tool is reached.
func TestMCPRefusesUnauthenticatedCaller(t *testing.T) {
	s := NewServer(Options{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(rpcInitialize))
	req.RemoteAddr = "203.0.113.9:52344" // off-loopback: the --addr 0.0.0.0 case
	req.Header.Set("X-Desk-Caller", "assay-worker-app[bot]")
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("POST /mcp from %s returned %d, want 401 — an unauthenticated caller must not reach the MCP surface (the service contract P3)",
			req.RemoteAddr, rec.Code)
	}
	if strings.Contains(rec.Body.String(), "serverInfo") {
		t.Fatalf("the refused request was answered with a real initialize result — the gate must refuse BEFORE the RPC runs:\n%s", rec.Body.String())
	}
}

// TestMCPServesLoopbackCaller is the paired positive control: the loopback
// caller the in-cluster and laptop deployments both use is unaffected, so the
// gate cannot be satisfied by refusing everything.
func TestMCPServesLoopbackCaller(t *testing.T) {
	s := NewServer(Options{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(rpcInitialize))
	req.RemoteAddr = "127.0.0.1:41111"
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /mcp from loopback returned %d, want 200 — the machine boundary is the loopback deployment's authentication", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "serverInfo") {
		t.Fatalf("loopback initialize did not return a result:\n%s", rec.Body.String())
	}
}

// TestMCPIgnoresForgedCallerHeader is the second half of P3: once a caller is
// AUTHENTICATED, the X-Desk-Caller header stops being consulted at all. The
// audit's counter-example was "desk__escalate would write as the desk App with
// a caller login the requester supplies"; an authenticated identity is never
// asked who it is.
func TestMCPIgnoresForgedCallerHeader(t *testing.T) {
	var seen string
	s := NewServer(Options{
		Caller: CallerAuthFunc(func(r *http.Request) (string, bool) {
			// A bearer-validating authenticator: it names the caller itself.
			return "assay-verifier-app[bot]", true
		}),
	})
	// Intercept the caller the RPC layer was handed by registering a probe tool.
	s.register(append(s.Tools(), Tool{
		Name:        "probe__caller",
		Description: "test probe",
		Scope:       ScopeNone, // probes the transport, not a cell
		Handler: func(ctx context.Context, srv *Server, call Call) (any, error) {
			seen = call.CallerLogin
			return map[string]any{"ok": true}, nil
		},
	}))

	rec := httptest.NewRecorder()
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"probe__caller","arguments":{}}}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.RemoteAddr = "203.0.113.9:52344"
	req.Header.Set("X-Desk-Caller", "assay-desk-app[bot]") // the forgery
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("an authenticated caller was refused: %d %s", rec.Code, rec.Body.String())
	}
	if seen != "assay-verifier-app[bot]" {
		t.Fatalf("the tool saw caller %q — the AUTHENTICATED identity must win over the X-Desk-Caller header (the service contract P3)", seen)
	}
}

// TestMCPRefusesWhenTheAuthenticatorRefuses: a bearer-mode caller with no valid
// token is refused, and no tool runs, even from loopback — the authenticator is
// the gate, not the interface the request arrived on.
func TestMCPRefusesWhenTheAuthenticatorRefuses(t *testing.T) {
	s := NewServer(Options{
		Caller: CallerAuthFunc(func(r *http.Request) (string, bool) { return "", false }),
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(rpcInitialize))
	req.RemoteAddr = "127.0.0.1:41111"
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("a caller the authenticator refused was served %d, want 401", rec.Code)
	}
}
