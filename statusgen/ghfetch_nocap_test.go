package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// pagedPullsServer serves `full` pages of 100 pulls, then one page of `tail`
// pulls (0 = an empty page), and counts the pages requested.
func pagedPullsServer(t *testing.T, full, tail int) (*ghClient, *int) {
	t.Helper()
	pages := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pages++
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		n := 0
		switch {
		case page <= full:
			n = 100
		case page == full+1:
			n = tail
		}
		batch := make([]map[string]any, n)
		for i := range batch {
			batch[i] = map[string]any{"number": (page-1)*100 + i + 1, "state": "closed"}
		}
		_ = json.NewEncoder(w).Encode(batch)
	}))
	t.Cleanup(srv.Close)
	return &ghClient{doer: srv.Client(), base: srv.URL}, &pages
}

// TestFetchAllPullsNoSilentCap pins #1969: a repository past the old 20-page
// (2,000-pull) bound is read to its last page, and a walk that hits the safety
// bound is a could-not-check, never a short list reported as complete.
func TestFetchAllPullsNoSilentCap(t *testing.T) {
	c, _ := pagedPullsServer(t, 25, 37)
	pulls, lookedAt, reason := c.fetchAllPulls("example-org/board")
	if !lookedAt || len(pulls) != 2537 {
		t.Fatalf("25 full pages + a 37-pull page must read all 2,537 pulls as complete; got %d lookedAt=%v reason=%q", len(pulls), lookedAt, reason)
	}

	c, _ = pagedPullsServer(t, 20, 0)
	if pulls, lookedAt, _ := c.fetchAllPulls("example-org/board"); !lookedAt || len(pulls) != 2000 {
		t.Fatalf("exactly 2,000 pulls (an empty 21st page) must read as complete; got %d lookedAt=%v", len(pulls), lookedAt)
	}

	prev := fetchAllPullsMaxPages
	fetchAllPullsMaxPages = 5
	t.Cleanup(func() { fetchAllPullsMaxPages = prev })
	c, pages := pagedPullsServer(t, 50, 0)
	pulls, lookedAt, reason = c.fetchAllPulls("example-org/board")
	if lookedAt || pulls != nil || !strings.Contains(reason, "not provably complete") {
		t.Fatalf("a walk stopped by the bound must be a could-not-check, not %d pulls read as complete; lookedAt=%v reason=%q", len(pulls), lookedAt, reason)
	}
	if *pages != 5 {
		t.Errorf("the bound must stop the walk at 5 pages; it requested %d", *pages)
	}
}
