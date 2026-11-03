package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/medici-finance/assay/perfrec"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func paginationSHA(n int) string { return fmt.Sprintf("%040x", n+1) }

// A large compare includes main-side commits; only head lies on the PR's first-parent
// chain. Serve only that commit detail so any accidental second-parent traversal fails.
func paginationServer(t *testing.T, commits, files int, mode *string, delay time.Duration) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	requests := new(atomic.Int64)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		time.Sleep(delay)
		w.Header().Set("Content-Type", "application/json")
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page == 0 {
			page = 1
		}
		perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
		switch {
		case strings.Contains(r.URL.Path, "/compare/"):
			limit := perPage
			if limit == 0 {
				limit = 250
			}
			start := (page - 1) * limit
			end := start + limit
			if end > commits {
				end = commits
			}
			chunk := []map[string]any{}
			for i := start; i < end; i++ {
				parents := []map[string]string{{"sha": paginationSHA(i)}}
				if i == commits-1 {
					parents = []map[string]string{{"sha": paginationSHA(0)}}
					if commits > 1 {
						parents = append(parents, map[string]string{"sha": paginationSHA(i)})
					}
				}
				chunk = append(chunk, map[string]any{"sha": paginationSHA(i + 1), "parents": parents})
			}
			if *mode == "short history" {
				chunk = []map[string]any{}
				w.Header().Del("Link")
			} else if perPage > 0 && end < commits {
				w.Header().Set("Link", `<https://untrusted.invalid>; rel="next"`)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"total_commits": commits, "status": "ahead", "ahead_by": commits, "commits": chunk, "files": []map[string]string{{"filename": "unrelated.go"}}})
		case strings.HasSuffix(r.URL.Path, "/commits/"+paginationSHA(commits)):
			limit := perPage
			if limit == 0 {
				limit = 300
			}
			start := (page - 1) * limit
			end := start + limit
			if end > files {
				end = files
			}
			names := []map[string]string{}
			for i := start; i < end; i++ {
				f := map[string]string{"filename": fmt.Sprintf("unrelated%d.go", i)}
				if i == files-1 && *mode == "own edit" {
					f["filename"] = "own.go"
				}
				if i == files-1 && *mode == "rename" {
					f["previous_filename"] = "own.go"
				}
				names = append(names, f)
			}
			if end < files {
				w.Header().Set("Link", `<https://untrusted.invalid>; rel="next"`)
			}
			if page > 1 && *mode == "later error" {
				w.WriteHeader(502)
				return
			}
			parents := []map[string]string{{"sha": paginationSHA(0)}}
			if commits > 1 {
				parents = append(parents, map[string]string{"sha": paginationSHA(commits - 1)})
			}
			payload := map[string]any{"sha": paginationSHA(commits), "parents": parents, "files": names}
			if page > 1 && *mode == "missing files" {
				delete(payload, "files")
			}
			_ = json.NewEncoder(w).Encode(payload)
		default:
			t.Errorf("unexpected request outside first-parent chain: %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, requests
}

func nativePaginationForge(t *testing.T, srv *httptest.Server, observer deskkit.RequestObserver) {
	t.Helper()
	prev := forgeFor
	t.Cleanup(func() { forgeFor = prev })
	g := &deskkit.GitHubForge{Token: "fixture-token", BaseURL: srv.URL, Client: srv.Client(), Observer: observer}
	forgeFor = func(string) (deskkit.Forge, deskkit.ForgeRepo, error) {
		return g, deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}, nil
	}
}

func TestHistoryPaginationBoard(t *testing.T) {
	for _, tc := range []struct {
		name           string
		commits, files int
		mode           string
		want           string
		requests       int
	}{
		{"small", 1, 2, "clean", actMergeCurr, 2},
		{"large history", 251, 2, "clean", actMergeCurr, 4},
		{"large files", 1, 301, "clean", actMergeCurr, 5},
		{"both", 251, 301, "clean", actMergeCurr, 7},
		{"late own edit", 251, 301, "own edit", actReReview, 7},
		{"late rename", 251, 301, "rename", actReReview, 7},
		{"history incomplete", 251, 301, "short history", actReReview, 1},
		{"file incomplete", 251, 301, "missing files", actReReview, 5},
		{"later page error", 251, 301, "later error", actReReview, 5},
		{"history cap", 1001, 2, "clean", actReReview, 1},
		{"file cap", 1, 3001, "clean", actReReview, 31},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mode := tc.mode
			srv, requests := paginationServer(t, tc.commits, tc.files, &mode, 0)
			nativePaginationForge(t, srv, deskkit.RequestObserver{})
			changed, err := changedFilesBetween("example-org/tracker", paginationSHA(0), paginationSHA(tc.commits))
			ownChanged := true
			if err == nil {
				ownChanged = ownFilesChanged(map[string]bool{"own.go": true}, true, changed)
			}
			action, _ := classify(classifyInput{ever: true, ownFilesChanged: ownChanged})
			if action != tc.want {
				t.Fatalf("action=%s want %s; err=%v", action, tc.want, err)
			}
			if requests.Load() != int64(tc.requests) {
				t.Fatalf("requests=%d want %d", requests.Load(), tc.requests)
			}
		})
	}
}

func TestHistoryPaginationSweepReuse(t *testing.T) {
	mode := "missing files"
	srv, requests := paginationServer(t, 251, 301, &mode, 0)
	nativePaginationForge(t, srv, deskkit.RequestObserver{})
	oldDir := boardCacheDir
	boardCacheDir = func() (string, error) { return t.TempDir(), nil }
	t.Cleanup(func() { boardCacheDir = oldDir })
	store := &prFactStore{repo: "example-org/tracker", prior: map[int]prFacts{}, next: map[int]prFacts{}}
	p := prBase{Number: 1, HeadRefOid: paginationSHA(251), BaseRefName: "main", UpdatedAt: "2026-10-21T00:00:00Z"}
	if _, err := store.interval(store.repo, p, paginationSHA(0)); err == nil {
		t.Fatal("partial files were cached as complete")
	}
	before := requests.Load()
	mode = "clean"
	if _, err := store.interval(store.repo, p, paginationSHA(0)); err != nil {
		t.Fatal(err)
	}
	if requests.Load() == before {
		t.Fatal("incomplete read was reused")
	}
	before = requests.Load()
	if _, err := store.interval(store.repo, p, paginationSHA(0)); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != before {
		t.Fatal("complete interval not reused")
	}
	p.UpdatedAt = "2026-10-21T00:01:00Z"
	if _, err := store.interval(store.repo, p, paginationSHA(0)); err != nil {
		t.Fatal(err)
	}
	if requests.Load() == before {
		t.Fatal("changed key reused old interval")
	}
}

// Matched offline source comparison. This records the ACTUAL selected schema; it must
// migrate with the shared producer when perf-v4 exists, never relabel historical v3.
func TestHistoryPaginationPerf(t *testing.T) {
	if testing.Short() {
		t.Skip("opt-in measurement")
	}
	installPerfLeafRoster(t)
	t.Setenv("ASSAY_TELEMETRY", "1")
	commits, files, sweeps := 1, 2, 1
	cohort := strings.TrimSpace(os.Getenv("ASSAY_PAGINATION_FIXTURE"))
	if cohort == "large" || cohort == "repeat8" {
		commits, files = 251, 301
	}
	if cohort == "repeat8" {
		sweeps = 8
	}
	if cohort == "overbudget" {
		commits, files = 1001, 2
	}
	mode := "clean"
	srv, requests := paginationServer(t, commits, files, &mode, 8*time.Millisecond)
	ctx := perfrec.ResolveContext(perfrec.ContextInput{Profile: perfrec.Value{Value: "tool", Present: true}})
	var comparisonErr error
	completeSweeps := 0
	code := deskkit.PerfRunMeasured("deskboard", perfVerbs, []string{"actions"}, func(_ []string, session *perfrec.Session) (int, perfrec.Context) {
		session.SelectContext(ctx)
		session.ObserveRequests()
		nativePaginationForge(t, srv, deskkit.RequestObserverFor(ctx, session, perfrec.OriginDirect))
		p := prBase{Number: 1, HeadRefOid: paginationSHA(commits), BaseRefName: "main", UpdatedAt: "2026-10-21T00:00:00Z"}
		// Replay the same externally observed key. Repo snapshot/check/review reads are
		// deliberately outside this isolated interval-cost measurement.
		for i := 0; i < sweeps; i++ {
			store := loadPRFacts("example-org/tracker")
			_, comparisonErr = store.interval("example-org/tracker", p, paginationSHA(0))
			if comparisonErr == nil {
				completeSweeps++
			}
			store.save()
		}
		return 0, ctx
	})
	if code != 0 {
		t.Fatalf("changed recorder outcome: %d", code)
	}
	history, err := deskkit.LoadPerfHistory(1)
	if err != nil || history.Skipped != 0 || len(history.Records) != 1 {
		t.Fatalf("history=%+v err=%v", history, err)
	}
	rec := history.Records[0]
	var attempts int64
	for _, r := range rec.V3.Requests {
		attempts += r.Attempts
	}
	if attempts != requests.Load() {
		t.Fatalf("perf attempts=%d HTTP witness=%d", attempts, requests.Load())
	}
	raw, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("PAGINATION_PERF %s\n", raw)
	fmt.Printf("PAGINATION_OUTCOME complete=%v requests=%d completeSweeps=%d sweeps=%d\n", comparisonErr == nil, requests.Load(), completeSweeps, sweeps)
}
