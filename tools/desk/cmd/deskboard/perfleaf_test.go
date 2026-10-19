package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestPerfLeafTransport(t *testing.T) {
	cache := t.TempDir()
	old := boardCacheDir
	boardCacheDir = func() (string, error) { return cache, nil }
	t.Cleanup(func() { boardCacheDir = old; boardMeter.end() })
	t.Setenv("ASSAY_TELEMETRY", "1")
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			w.WriteHeader(503)
			fmt.Fprint(w, `{"message":"fixture retry"}`)
			return
		}
		w.Header().Set("X-RateLimit-Resource", "core")
		w.Header().Set("X-RateLimit-Remaining", "1")
		w.Header().Set("X-RateLimit-Limit", "5000")
		if r.Header.Get("If-None-Match") != "" {
			w.WriteHeader(304)
			return
		}
		w.Header().Set("ETag", `"fixture-v1"`)
		fmt.Fprint(w, `{"number":1,"state":"open","title":"fixture","user":{"login":"example-driver","id":2001}}`)
	}))
	defer srv.Close()
	oldTransport := http.DefaultTransport
	http.DefaultTransport = srv.Client().Transport
	if http.DefaultTransport == nil {
		http.DefaultTransport = oldTransport
	}
	t.Cleanup(func() { http.DefaultTransport = oldTransport })
	// ResolveForge uses this exact seam; use its resolver construction rather
	// than a separately composed transport that could omit/double-wrap counters.
	oldMint, oldBase := mintTokenFn, forgeAPIBase
	mintTokenFn = func(string, string) (string, string, error) { return "fixture-token", "", nil }
	forgeAPIBase = srv.URL
	t.Cleanup(func() { mintTokenFn, forgeAPIBase = oldMint, oldBase })
	repo := deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}
	f, _, err := deskkit.ResolveForge(repo, "worker")
	if err != nil {
		t.Fatal(err)
	}
	boardMeter.begin(0)
	deskkit.PerfRun("deskboard", perfVerbs, []string{"actions"}, func([]string) int {
		// Caller retries the failed typed read. Both attempts use the production
		// resolver/REST/cache stack; no request is synthesized by the perf test.
		if _, err := f.GetIssue(repo, 1); err == nil {
			t.Fatal("503 was hidden")
		}
		if _, err := f.GetIssue(repo, 1); err != nil {
			t.Fatal(err)
		}
		if _, err := f.GetIssue(repo, 1); err != nil {
			t.Fatal(err)
		}
		_, _, calls := deskkit.PerfCounters()
		if requests != 3 || calls != 3 || boardMeter.snapshot().NotModified != 1 {
			t.Fatalf("attempts server=%d perf=%d meter=%+v", requests, calls, boardMeter.snapshot())
		}
		return 0
	})
	boardMeter.begin(500)
	// Seed a pre-existing breach, then ask the actual typed provider to read.
	boardMeter.observe(http.Header{"X-Ratelimit-Resource": []string{"core"}, "X-Ratelimit-Remaining": []string{"1"}})
	before := requests
	deskkit.PerfRun("deskboard", perfVerbs, []string{"actions"}, func([]string) int {
		_, err := f.GetIssue(repo, 1)
		if err == nil || !strings.Contains(err.Error(), "request not sent — core quota is below the --rate-floor (1 < 500)") {
			t.Fatalf("floor error lost: %v", err)
		}
		if deskkit.ExitCodeOf(boardMeter.snapshot().floorError(time.Now())) != deskkit.ExitRateLimited {
			t.Fatal("floor exit changed")
		}
		_, _, calls := deskkit.PerfCounters()
		if calls != 0 || requests != before {
			t.Fatalf("local refusal became HTTP work: calls=%d server=%d/%d", calls, requests, before)
		}
		return deskkit.ExitRateLimited
	})
}
