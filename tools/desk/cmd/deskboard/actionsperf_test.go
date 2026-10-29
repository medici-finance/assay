package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestActionsCostFixture runs complete actions sweeps through the real typed
// GitHub transport against a frozen ten-repository/two-PR fixture. It records
// counts and semantic output; production network latency and quota points are
// deliberately outside this local fixture's claims.
func TestActionsCostFixture(t *testing.T) {
	const repos, prs = 10, 2
	ff := newEtagForge(t, prs)
	ff.snapshotVisibility = snapshotVisibility("PRIVATE")
	ff.reset = 1893456000 // fixed fixture reset, independent of measurement time
	names := make([]string, repos)
	for i := range names {
		names[i] = fmt.Sprintf("example-org/cost-%02d", i)
	}
	rosterPath := filepath.Join(os.Getenv("HOME"), ".config", "assay", "roster.env")
	raw, err := os.ReadFile(rosterPath)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Replace(string(raw), "ASSAY_ALLOWED_REPOS="+fakeRepo+":ci:private", "ASSAY_ALLOWED_REPOS="+strings.Join(names, ":ci:private,")+":ci:private", 1)
	text = strings.Replace(text, "ASSAY_REPO_FORGES="+fakeRepo+"=github", "ASSAY_REPO_FORGES="+strings.Join(names, "=github,")+"=github", 1)
	if err := os.WriteFile(rosterPath, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	deskkit.ReloadConfig()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/repos/example-org/cost-") {
			clone := r.Clone(r.Context())
			u := *r.URL
			parts := strings.Split(strings.TrimPrefix(u.Path, "/repos/"), "/")
			u.Path = "/repos/" + fakeRepo
			if len(parts) > 2 {
				u.Path += "/" + strings.Join(parts[2:], "/")
			}
			clone.URL = &u
			ff.serve(w, clone)
			return
		}
		ff.serve(w, r)
	}))
	defer srv.Close()
	forgeAPIBase = srv.URL
	for _, name := range []string{"cold", "unchanged"} {
		start := time.Now()
		res := ff.sweep(t)
		elapsed := time.Since(start).Nanoseconds()
		if res.code != 0 {
			t.Fatalf("actions: exit %d: %s", res.code, res.stderr)
		}
		var decoded map[string]any
		if err := json.Unmarshal(res.stdout, &decoded); err != nil {
			t.Fatal(err)
		}
		rows, ok := decoded["rows"].([]any)
		if !ok || len(rows) != repos*prs {
			t.Fatalf("coverage: have %d rows, want %d", len(rows), repos*prs)
		}
		// The clock and read-cost report are intentionally different across
		// compared invocations. Preserve all other classification/output fields.
		delete(decoded, "asOf")
		delete(decoded, "auditFirstTS")
		delete(decoded, "cost")
		for _, row := range rows {
			if fields, ok := row.(map[string]any); ok {
				delete(fields, "openAge")
				delete(fields, "approvedAge")
			}
		}
		semantic, err := json.Marshal(decoded)
		if err != nil {
			t.Fatal(err)
		}
		result := map[string]any{"fixture": name, "repos": repos, "prsPerRepo": prs, "http": len(res.all), "non304": len(res.charged), "elapsedNs": elapsed, "exit": res.code, "semantic": json.RawMessage(semantic)}
		line, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Printf("ACTIONS_COST_FIXTURE %s\n", line)
	}
}
