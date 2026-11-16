package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// The same provider population exercises complete production sweeps on old and
// new source. Success rows must match; every actual HTTP attempt is counted.
func TestStalledActivityCost(t *testing.T) {
	for _, size := range []int{5, 20, 60} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			ff := newEtagForge(t, size)
			var mu sync.Mutex
			attempts, commentReads, headReads, compares := 0, 0, 0, 0
			recent := false
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				attempts++
				w.Header().Set("Content-Type", "application/json")
				switch {
				case r.URL.Path == "/graphql":
					var req struct {
						Query     string         `json:"query"`
						Variables map[string]any `json:"variables"`
					}
					if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
						t.Error(err)
						return
					}
					if strings.Contains(req.Query, "pullRequests(states:OPEN") {
						ff.mu.Lock()
						payload := ff.snapshotLocked()
						ff.mu.Unlock()
						nodes := payload.(map[string]any)["data"].(map[string]any)["repository"].(map[string]any)["pullRequests"].(map[string]any)["nodes"].([]any)
						for _, raw := range nodes {
							p := raw.(map[string]any)
							p["isDraft"] = true
							rv := p["reviews"].(map[string]any)["nodes"].([]any)[0].(map[string]any)
							rv["state"], rv["submittedAt"] = "CHANGES_REQUESTED", "2026-08-01T00:00:00Z"
						}
						json.NewEncoder(w).Encode(payload)
						return
					}
					commentReads++
					comments := func(n int) []any {
						date := "2026-08-01T00:00:00Z"
						if recent && n%5 == 0 {
							date = "2099-01-01T00:00:00Z"
						}
						return []any{map[string]any{"id": fmt.Sprint(n), "databaseId": n, "createdAt": date, "body": "reply", "author": map[string]any{"login": "ada", "__typename": "User", "databaseId": 2001}}}
					}
					if n, ok := req.Variables["number"].(float64); ok {
						json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"repository": map[string]any{"pullRequest": map[string]any{"comments": map[string]any{"nodes": comments(int(n))}}}}})
						return
					}
					aliases := map[string]any{}
					for i := 0; i < 5; i++ {
						if req.Variables[fmt.Sprintf("use%d", i)] != true {
							continue
						}
						n, ok := req.Variables[fmt.Sprintf("n%d", i)].(float64)
						if !ok || n < 1 || int(n) > size {
							t.Error("batch lost requested identity")
							return
						}
						aliases[fmt.Sprintf("a%d", i)] = map[string]any{"number": n, "comments": map[string]any{"nodes": comments(int(n))}}
					}
					json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"repository": aliases}})
				case strings.Contains(r.URL.Path, "/commits/"):
					headReads++
					json.NewEncoder(w).Encode(map[string]any{"sha": strings.Split(r.URL.Path, "/commits/")[1], "committer": map[string]any{"login": "ada"}, "commit": map[string]any{"committer": map[string]any{"date": "2026-08-01T00:00:00Z"}}, "files": []any{}})
				case strings.Contains(r.URL.Path, "/compare/"):
					compares++
					fmt.Fprint(w, `{"status":"behind","behind_by":5,"files":[]}`)
				default:
					t.Errorf("unexpected request %s", r.URL.Path)
					http.Error(w, "not served", 404)
				}
			}))
			defer srv.Close()
			forgeAPIBase = srv.URL
			for _, phase := range []string{"cold", "unchanged", "author-replied"} {
				mu.Lock()
				attempts, commentReads, headReads, compares = 0, 0, 0, 0
				recent = phase == "author-replied"
				mu.Unlock()
				var out, errs bytes.Buffer
				code := run([]string{"stalled", "--json"}, &out, &errs)
				if code != 0 {
					t.Fatalf("exit %d: %s", code, errs.String())
				}
				var report map[string]any
				if err := json.Unmarshal(out.Bytes(), &report); err != nil {
					t.Fatal(err)
				}
				rows := report["stalled"].([]any)
				want := size
				if recent {
					want -= size / 5
				}
				if len(rows) != want || len(report["unassessable"].([]any)) != 0 {
					t.Fatalf("activity clock changed: %s", out.String())
				}
				delete(report, "asOf")
				delete(report, "auditFirstTS")
				for _, r := range rows {
					delete(r.(map[string]any), "daysStalled")
				}
				mu.Lock()
				a, c, h, b := attempts, commentReads, headReads, compares
				mu.Unlock()
				record, _ := json.Marshal(map[string]any{"size": size, "phase": phase, "http": a, "comments": c, "headReads": h, "compares": b, "semantic": report})
				fmt.Printf("STALLED_ACTIVITY_COST %s\n", record)
				if h != size || b != want {
					t.Errorf("lost head/attribution/behind-main reads: head=%d compare=%d", h, b)
				}
				if c != (size+4)/5 {
					t.Errorf("comment HTTP attempts=%d, want %d", c, (size+4)/5)
				}
			}
		})
	}
}
