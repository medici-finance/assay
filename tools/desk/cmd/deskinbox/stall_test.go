package main

// stall_test.go — the comments reader gives up on a stalled forge within its bound, and a
// body cut off after a 200 is could-not-check, never an empty comment list.

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// stallBound is the shortened deadline the stall tests run under; stallGrace is how long a
// test waits before it calls the reader hung. The gap is the margin a mutant that drops the
// deadline cannot cross.
const (
	stallBound = 200 * time.Millisecond
	stallGrace = 5 * time.Second
)

// stallComments serves every request by stalling: before any header when partial is "",
// else after a 200 and partial, flushed. Cleanup unblocks the handler before the server
// closes, so a hung reader (the mutant) still drains.
func stallComments(t *testing.T, partial string) {
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
	prevBase, prevTimeout := forgeAPIBase, commentsTimeout
	forgeAPIBase, commentsTimeout = srv.URL, stallBound
	t.Cleanup(func() { forgeAPIBase, commentsTimeout = prevBase, prevTimeout })
}

// listWithin runs the comments reader and fails the test if it has not returned within
// stallGrace.
func listWithin(t *testing.T) ([]comment, error) {
	t.Helper()
	type result struct {
		cs  []comment
		err error
	}
	done := make(chan result, 1)
	go func() {
		cs, err := listIssueComments("example-credential", deskkit.ForgeRepo{Owner: "example-org", Name: "example-repo"}, 1)
		done <- result{cs, err}
	}()
	select {
	case r := <-done:
		return r.cs, r.err
	case <-time.After(stallGrace):
		t.Fatalf("comments reader still blocked after %s against a stalled forge (bound %s)", stallGrace, stallBound)
		return nil, nil
	}
}

// regression: #6721
func TestCommentsStallBeforeHeaders(t *testing.T) {
	stallComments(t, "")
	cs, err := listWithin(t)
	if err == nil {
		t.Fatalf("stalled forge read as success: %d comments", len(cs))
	}
	if !deskkit.IsUnverifiable(err) {
		t.Errorf("stall must be could-not-check (Unverifiable), got %T: %v", err, err)
	}
}

// A 200 whose body stalls after a complete-looking "[]" must not read as "no comments".
//
// regression: #6721
func TestCommentsStallAfterHeaders(t *testing.T) {
	stallComments(t, "[]")
	cs, err := listWithin(t)
	if err == nil {
		t.Fatalf("a body cut off after 200 read as an empty success: %d comments", len(cs))
	}
	if !deskkit.IsUnverifiable(err) {
		t.Errorf("cut-off body must be could-not-check (Unverifiable), got %T: %v", err, err)
	}
}

// regression: #6721
func TestCommentsClientIsBounded(t *testing.T) {
	if commentsRequestTimeout <= 0 || commentsTimeout != commentsRequestTimeout {
		t.Fatalf("comments bound = %s (default %s); want a positive default in force", commentsTimeout, commentsRequestTimeout)
	}
	if got := commentsClient().Timeout; got != commentsRequestTimeout {
		t.Fatalf("commentsClient().Timeout = %s, want %s", got, commentsRequestTimeout)
	}
}
