package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/medici-finance/assay/perfrec"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestAdmissionPerfV3 is a repeatable offline cohort: the actual admission path,
// native GitHub reads and perf-v3 writer/reader, with only the remote forge replaced.
// Its injected 8ms/request is a latency model, not a production speed measurement.
// regression: medici-finance/assay#3373
func TestAdmissionPerfV3(t *testing.T) {
	installPerfLeafRoster(t)
	t.Setenv("ASSAY_TELEMETRY", "1")
	const count = 12
	briefs := map[string]string{}
	var nums []string
	for i := 1; i <= count; i++ {
		n := fmt.Sprintf("%02d", i)
		briefs[n] = "gate: human\n"
		nums = append(nums, n)
	}
	root := admissionRoot(t, briefs)
	var requests atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		time.Sleep(8 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/search/issues":
			var items []map[string]any
			q := r.URL.Query().Get("q")
			for i := 1; i <= count; i++ {
				if strings.Contains(q, "s/") && !strings.Contains(q, fmt.Sprintf("s/%02d", i)) {
					continue
				}
				items = append(items, map[string]any{"number": i, "body": fmt.Sprintf("<!-- decision-gate: s/%02d -->", i)})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"total_count": len(items), "incomplete_results": false, "items": items})
		case r.URL.Path == "/graphql":
			fmt.Fprint(w, `{"data":{"repository":{"issue":{"comments":{"nodes":[],"pageInfo":{"hasNextPage":false}},"timelineItems":{"nodes":[],"pageInfo":{"hasNextPage":false}}}}}}`)
		case strings.HasSuffix(r.URL.Path, "/timeline"):
			fmt.Fprint(w, `[{"event":"labeled","created_at":"2026-10-01T10:00:00Z","label":{"name":"needs-decision"}}]`)
		case strings.HasPrefix(r.URL.Path, "/repos/example-org/tracker/issues/"):
			n, err := strconv.Atoi(filepath.Base(r.URL.Path))
			if err != nil || n < 1 || n > count {
				t.Errorf("unexpected issue route %s", r.URL.Path)
				w.WriteHeader(404)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"number": n, "state": "open", "body": fmt.Sprintf("<!-- decision-gate: s/%02d -->", n), "labels": []map[string]string{{"name": "needs-decision"}}})
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	oldForge, oldRatifier := decisionForgeFor, decisionRatifierFn
	t.Cleanup(func() { decisionForgeFor, decisionRatifierFn = oldForge, oldRatifier })
	decisionRatifierFn = func() (func(string, int64) bool, bool) {
		return func(login string, id int64) bool { return login == "example-driver" && id == 2001 }, true
	}
	ctx := perfrec.ResolveContext(perfrec.ContextInput{Profile: perfrec.Value{Value: "tool", Present: true}})
	rep := admissionReport(root, nums...)
	code := deskkit.PerfRunMeasured("deskboard", perfVerbs, []string{"dispatch"}, func(_ []string, session *perfrec.Session) (int, perfrec.Context) {
		session.SelectContext(ctx)
		session.ObserveRequests() // this fixture witnesses every request; no custody I/O
		decisionForgeFor = func(string) (deskkit.DecisionForge, deskkit.ForgeRepo, error) {
			return &deskkit.GitHubForge{Token: "fixture-token", BaseURL: srv.URL, Client: srv.Client(), Observer: deskkit.RequestObserverFor(ctx, session, perfrec.OriginDirect)}, deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}, nil
		}
		applyBriefAdmission(rep, "strong")
		return 0, ctx
	})
	if code != 0 || rep.HeldByDecision != count || len(rep.Rows) != 0 || len(rep.AdmissionUnknown) != 0 {
		t.Fatalf("cohort outcome changed: code=%d held=%d rows=%d unknown=%+v", code, rep.HeldByDecision, len(rep.Rows), rep.AdmissionUnknown)
	}
	history, err := deskkit.LoadPerfHistory(1)
	if err != nil || history.Skipped != 0 || len(history.Records) != 1 || history.Records[0].Schema != perfrec.SchemaV3 {
		t.Fatalf("perf-v3 history: %v %+v", err, history)
	}
	rec := history.Records[0]
	var observed int64
	for _, r := range rec.V3.Requests {
		observed += r.Attempts
	}
	if observed != requests.Load() {
		t.Fatalf("HTTP witness=%d perf-v3 attempts=%d", requests.Load(), observed)
	}
	// Emit only the schema-qualified record. No paths, fixture bodies or tokens.
	b, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("ADMISSION_PERF_V3 %s\n", b)
}
