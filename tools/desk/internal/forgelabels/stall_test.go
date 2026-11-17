package forgelabels

// stall_test.go — the HTTP Transport gives up on a stalled forge within its bound when the
// caller's client sets no deadline, keeps a deadline the caller did set, and a body cut off
// after a 200 is an error, never an empty label list.

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// stallBound is the shortened default deadline the stall tests run under; stallGrace is how
// long a test waits before it calls the Transport hung.
const (
	stallBound = 200 * time.Millisecond
	stallGrace = 5 * time.Second
)

// stallServer stalls every request: before any header when partial is "", else after a 200
// and partial, flushed. Cleanup unblocks the handler before the server closes, so a hung
// call (the mutant) still drains.
func stallServer(t *testing.T, partial string) *httptest.Server {
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
	prev := defaultTimeout
	defaultTimeout = stallBound
	t.Cleanup(func() { defaultTimeout = prev })
	return srv
}

// listWithin lists labels through the github Backend over tr, failing the test if the call
// has not returned within stallGrace.
func listWithin(t *testing.T, tr Transport) ([]string, error) {
	t.Helper()
	type result struct {
		names []string
		err   error
	}
	done := make(chan result, 1)
	go func() {
		names, err := GitHub(tr, "example-org/example-repo").List()
		done <- result{names, err}
	}()
	select {
	case r := <-done:
		return r.names, r.err
	case <-time.After(stallGrace):
		t.Fatalf("label list still blocked after %s against a stalled forge (bound %s)", stallGrace, stallBound)
		return nil, nil
	}
}

// Both a nil client and an injected client with no Timeout get the default bound.
//
// regression: #6721
func TestTransportStallBeforeHeaders(t *testing.T) {
	srv := stallServer(t, "")
	for name, hc := range map[string]*http.Client{"nil client": nil, "zero-Timeout client": srv.Client()} {
		tr, err := NewHTTPTransport("github", srv.URL, hc, "example-credential")
		if err != nil {
			t.Fatal(err)
		}
		if names, err := listWithin(t, tr); err == nil {
			t.Errorf("%s: stalled forge read as success: %v", name, names)
		}
	}
}

// A 200 whose body stalls after a complete-looking "[]" must not read as "no labels".
//
// regression: #6721
func TestTransportStallAfterHeaders(t *testing.T) {
	srv := stallServer(t, "[]")
	tr, err := NewHTTPTransport("github", srv.URL, nil, "example-credential")
	if err != nil {
		t.Fatal(err)
	}
	if names, err := listWithin(t, tr); err == nil {
		t.Fatalf("a body cut off after 200 read as an empty success: %v", names)
	}
}

// regression: #6721
func TestTransportEmptyBodyRefused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	tr, err := NewHTTPTransport("github", srv.URL, nil, "example-credential")
	if err != nil {
		t.Fatal(err)
	}
	if names, err := listWithin(t, tr); err == nil {
		t.Fatalf("an empty 200 body read as an empty label list: %v", names)
	}
}

// regression: #6721
func TestTransportKeepsOwnTimeout(t *testing.T) {
	own := 7 * time.Second
	for name, tc := range map[string]struct {
		hc   *http.Client
		want time.Duration
	}{
		"nil client":          {nil, requestTimeout},
		"zero-Timeout client": {&http.Client{}, requestTimeout},
		"own-Timeout client":  {&http.Client{Timeout: own}, own},
	} {
		tr, err := NewHTTPTransport("github", "https://forge.example.com", tc.hc, "example-credential")
		if err != nil {
			t.Fatal(err)
		}
		if got := tr.(*httpTransport).hc.Timeout; got != tc.want {
			t.Errorf("%s: Timeout = %s, want %s", name, got, tc.want)
		}
	}
}
