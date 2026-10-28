package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/perfrec"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: class perf-v3-composed-board-leaf
func TestPerfLeafV3Report(t *testing.T) {
	installPerfLeafRoster(t)
	t.Setenv("ASSAY_TELEMETRY", "1")
	cache := t.TempDir()
	oldCache := boardCacheDir
	boardCacheDir = func() (string, error) { return cache, nil }
	t.Cleanup(func() { boardCacheDir = oldCache; boardMeter.end() })
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
		if r.Header.Get("If-None-Match") != "" {
			w.WriteHeader(304)
			return
		}
		w.Header().Set("ETag", `"fixture-v1"`)
		fmt.Fprint(w, `{"number":1,"state":"open","title":"fixture","user":{"login":"example-driver","id":2001}}`)
	}))
	defer srv.Close()
	oldMint, oldBase := mintTokenFn, forgeAPIBase
	mintTokenFn = func(string, string) (string, string, error) { return "fixture-token", "", nil }
	forgeAPIBase = srv.URL
	t.Cleanup(func() { mintTokenFn, forgeAPIBase = oldMint, oldBase })
	repo := deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}
	newForge := func() (deskkit.Forge, *perfrec.Session) {
		ctx := perfrec.ResolveContext(perfrec.ContextInput{Profile: perfrec.Value{Value: "tool", Present: true}})
		p := perfrec.SystemProbe()
		session := perfrec.BeginSelected(p, p.Mark(), nil, ctx)
		f, _, err := deskkit.ResolveForge(repo, "worker", deskkit.RequestObserverFor(ctx, session, perfrec.OriginDirect))
		if err != nil {
			t.Fatal(err)
		}
		// This fixture owns every network leaf; custody is the no-I/O stub above.
		session.ObserveRequests()
		return f, session
	}
	state, err := deskkit.StateDir()
	if err != nil {
		t.Fatal(err)
	}
	writer := perfrec.Writer{}
	f, session := newForge()
	boardMeter.begin(0)
	if _, err := f.GetIssue(repo, 1); err == nil {
		t.Fatal("503 hidden")
	}
	for i := 0; i < 2; i++ {
		issue, err := f.GetIssue(repo, 1)
		if err != nil || issue.Number != 1 {
			t.Fatal("cache response changed", err)
		}
	}
	if requests != 3 || boardMeter.snapshot().NotModified != 1 {
		t.Fatal("fixture sends/cache", requests, boardMeter.snapshot())
	}
	if err := writer.Write(state, session.Finish("deskboard", "actions", "", 0, "")); err != nil {
		t.Fatal(err)
	}
	boardMeter.begin(500)
	boardMeter.observe(http.Header{"X-Ratelimit-Resource": []string{"core"}, "X-Ratelimit-Remaining": []string{"1"}})
	f, session = newForge()
	if _, err := f.GetIssue(repo, 1); err == nil {
		t.Fatal("floor refusal hidden")
	}
	if requests != 3 {
		t.Fatal("local refusal sent HTTP")
	}
	if err := writer.Write(state, session.Finish("deskboard", "actions", "", 4, "")); err != nil {
		t.Fatal(err)
	}
	history, err := deskkit.LoadPerfHistory(1)
	if err != nil || history.Skipped != 0 {
		t.Fatal("reader", err)
	}
	sum, err := perfrec.ReduceV3(history.Records, perfrec.ProfileTool, "tool", perfrec.V3Filter{})
	if err != nil {
		t.Fatal(err)
	}
	var attempts, notModified int64
	for _, request := range sum.Requests {
		attempts += request.Attempts
		if request.Outcome == "304" {
			notModified += request.Attempts
		}
	}
	if sum.Records != 2 || attempts != 3 || notModified != 1 || sum.RequestCoverage[perfrec.CoverageMeasured] != 2 {
		t.Fatalf("actual=%d reported=%d 304=%d records=%d coverage=%v", requests, attempts, notModified, sum.Records, sum.RequestCoverage)
	}
}

func TestPerfLeafTransport(t *testing.T) {
	installPerfLeafRoster(t)
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

// The isolated mutation workspace intentionally has no forge remote. Native
// resolver fixtures must supply their own private roster instead of borrowing it.
func installPerfLeafRoster(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	config := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(config, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(config, "roster.env"), []byte(fixtureRoster+"\nASSAY_REPO_FORGES=example-org/tracker=github\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("ASSAY_CONFIG_HOME", config)
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
}
