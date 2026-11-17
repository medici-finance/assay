package main

// stall_test.go — ghAPI gives up on a stalled forge within its bound, and neither a body cut
// off after a 200 nor an empty 200 body reads as a successful answer.

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// stallBound is the shortened deadline the stall tests run under; stallGrace is how long a
// test waits before it calls ghAPI hung.
const (
	stallBound = 200 * time.Millisecond
	stallGrace = 5 * time.Second
)

// stallAPI points githubAPIBase at a server that stalls every request: before any header
// when partial is "", else after a 200 and partial, flushed. Cleanup unblocks the handler
// before the server closes, so a hung call (the mutant) still drains.
func stallAPI(t *testing.T, partial string) {
	t.Helper()
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if partial != "" {
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, partial)
			w.(http.Flusher).Flush()
		}
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(release) })
	prevBase, prevTimeout := githubAPIBase, ghAPITimeout
	githubAPIBase, ghAPITimeout = srv.URL+"/", stallBound
	t.Cleanup(func() { githubAPIBase, ghAPITimeout = prevBase, prevTimeout })
}

// ghAPIWithin calls ghAPI and fails the test if it has not returned within stallGrace.
func ghAPIWithin(t *testing.T) ([]byte, error) {
	t.Helper()
	type result struct {
		body []byte
		err  error
	}
	done := make(chan result, 1)
	go func() {
		b, err := ghAPI("repos/example-org/example-repo")
		done <- result{b, err}
	}()
	select {
	case r := <-done:
		return r.body, r.err
	case <-time.After(stallGrace):
		t.Fatalf("ghAPI still blocked after %s against a stalled forge (bound %s)", stallGrace, stallBound)
		return nil, nil
	}
}

// regression: #6721
func TestGhAPIStallBeforeHeaders(t *testing.T) {
	stallAPI(t, "")
	if b, err := ghAPIWithin(t); err == nil {
		t.Fatalf("stalled forge read as success: %q", b)
	}
}

// A 200 whose body stalls after a complete-looking JSON document must not read as success.
//
// regression: #6721
func TestGhAPIStallAfterHeaders(t *testing.T) {
	stallAPI(t, `{"private":true}`)
	if b, err := ghAPIWithin(t); err == nil {
		t.Fatalf("a body cut off after 200 read as success: %q", b)
	}
}

// regression: #6721
func TestGhAPIEmptyBodyRefused(t *testing.T) {
	withMockAPI(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	if b, err := ghAPI("repos/example-org/example-repo"); err == nil {
		t.Fatalf("an empty 200 body read as success: %q", b)
	}
}

// regression: #6721
func TestGhAPIClientIsBounded(t *testing.T) {
	if ghAPIRequestTimeout <= 0 || ghAPITimeout != ghAPIRequestTimeout {
		t.Fatalf("ghAPI bound = %s (default %s); want a positive default in force", ghAPITimeout, ghAPIRequestTimeout)
	}
	if got := ghAPIClient().Timeout; got != ghAPIRequestTimeout {
		t.Fatalf("ghAPIClient().Timeout = %s, want %s", got, ghAPIRequestTimeout)
	}
}
