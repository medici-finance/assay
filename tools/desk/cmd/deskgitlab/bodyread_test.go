package main

// bodyread_test.go — the API client no longer discards a body read error. An answer whose
// body is cut off, stalls, runs over the bound, or is empty where a GET's result belongs is
// NO answer ("000"), never the 2xx with a partial or empty body, and a run that met one ends
// could-not-check (exit 6).

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// bodyServer serves every request with h and returns an API client pointed at it.
func bodyServer(t *testing.T, h http.HandlerFunc) *apiClient {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return newAPIClient(srv.URL, fakeOwnerToken)
}

// wantNoAnswer asserts a call reported no answer and was noted as could-not-check.
func wantNoAnswer(t *testing.T, c *apiClient, status string, body []byte) {
	t.Helper()
	if status != transportFailed {
		t.Errorf("status %q with a %d-byte body; want %q (no answer)", status, len(body), transportFailed)
	}
	if body != nil {
		t.Errorf("an unreadable answer handed back %d bytes; want none", len(body))
	}
	if c.unreadable == nil {
		t.Errorf("the unreadable answer was not noted for the run's could-not-check exit")
	}
}

// A 200 whose body is cut off after a valid JSON prefix.
//
// regression: #6730
func TestSendTruncatedBody(t *testing.T) {
	c := bodyServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		_, _ = io.WriteString(w, `[]`)
	})
	st, b := c.call(http.MethodGet, "/groups/1/service_accounts", nil)
	wantNoAnswer(t, c, st, b)
}

// A 200 whose body stalls before its first byte, past the client's deadline.
//
// regression: #6730
func TestSendStalledBody(t *testing.T) {
	prev := newHTTPClient
	newHTTPClient = func() *http.Client { return &http.Client{Timeout: 200 * time.Millisecond} }
	t.Cleanup(func() { newHTTPClient = prev })
	release := make(chan struct{})
	c := bodyServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
		}
	})
	t.Cleanup(func() { close(release) })
	st, b := c.call(http.MethodGet, "/groups/1/service_accounts", nil)
	wantNoAnswer(t, c, st, b)
}

// regression: #6730
func TestSendEmptyGetBody(t *testing.T) {
	c := bodyServer(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	st, b := c.call(http.MethodGet, "/groups/1/service_accounts", nil)
	wantNoAnswer(t, c, st, b)
}

// A 204, or an empty body on a write, is not an unreadable answer.
//
// regression: #6730
func TestSendEmptyBodyAllowed(t *testing.T) {
	c := bodyServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	if st, _ := c.call(http.MethodGet, "/x", nil); st != "204" {
		t.Errorf("GET 204: status %q, want 204", st)
	}
	if st, _ := c.call(http.MethodDelete, "/x", nil); st != "200" {
		t.Errorf("DELETE with an empty 200: status %q, want 200", st)
	}
	if c.unreadable != nil {
		t.Errorf("a bodiless answer was noted unreadable: %v", c.unreadable)
	}
}

// An answer over maxResponseBytes was read as its first 4 MiB.
//
// regression: #6730
func TestSendOverBoundBody(t *testing.T) {
	c := bodyServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(append([]byte("[]"), bytes.Repeat([]byte(" "), maxResponseBytes)...))
	})
	st, b := c.call(http.MethodGet, "/groups/1/service_accounts", nil)
	wantNoAnswer(t, c, st, b)
}

// End to end: a run whose group read comes back empty exits 6 (could-not-check), not 1.
//
// regression: #6730
func TestProvisionUnreadableExits6(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	var out, errOut strings.Builder
	code := provision(options{gitlabURL: srv.URL, token: fakeOwnerToken, group: fakeGroup, noAvatars: true}, &out, &errOut)
	if code != 6 {
		t.Fatalf("exit %d, want 6 (could-not-check); stderr: %s", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "could-not-check") {
		t.Errorf("stderr does not say could-not-check: %s", errOut.String())
	}
	if strings.Contains(out.String()+errOut.String(), fakeOwnerToken) {
		t.Errorf("the credential appeared in the output")
	}
}
