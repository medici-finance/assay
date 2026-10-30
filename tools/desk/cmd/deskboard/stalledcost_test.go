package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// The real resolver/backend and complete command run against synthetic, fixed
// inputs. This fixture measures HTTP attempts and external command time; its
// headers do not claim to measure GitHub quota points or production latency.
func TestStalledCostFixture(t *testing.T) {
	ff := newEtagForge(t, 5)
	var mu sync.Mutex
	var paths []string
	mode := "cold"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.Method+" "+r.URL.RequestURI())
		currentMode := mode
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "POST" && r.URL.Path == "/graphql":
			var request struct {
				Query     string         `json:"query"`
				Variables map[string]any `json:"variables"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
				return
			}
			if !strings.Contains(request.Query, "pullRequests(states:OPEN") {
				var comments []any
				if request.Variables["number"] == float64(5) {
					comments = []any{map[string]any{"id": "comment5", "databaseId": 5, "author": map[string]any{"login": "ada", "databaseId": 2001, "__typename": "User"}, "createdAt": "2099-01-01T00:00:00Z", "body": "reply"}}
				}
				json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"repository": map[string]any{"pullRequest": map[string]any{"comments": map[string]any{"nodes": comments, "pageInfo": map[string]any{"hasNextPage": false}}}}}})
				return
			}
			ff.mu.Lock()
			payload := ff.snapshotLocked()
			ff.mu.Unlock()
			nodes := payload.(map[string]any)["data"].(map[string]any)["repository"].(map[string]any)["pullRequests"].(map[string]any)["nodes"].([]any)
			for _, raw := range nodes {
				p := raw.(map[string]any)
				n := p["number"].(int)
				p["isDraft"] = true
				reviews := p["reviews"].(map[string]any)
				if n == 3 {
					reviews["nodes"] = []any{}
					continue
				}
				rv := reviews["nodes"].([]any)[0].(map[string]any)
				rv["submittedAt"] = "2026-08-01T00:00:00Z"
				if n == 1 || n == 5 {
					rv["state"] = "CHANGES_REQUESTED"
				}
				if n == 4 {
					reviews["pageInfo"] = map[string]any{"hasNextPage": true}
				}
			}
			json.NewEncoder(w).Encode(payload)
		case strings.Contains(r.URL.Path, "/reviews"):
			parts := strings.Split(r.URL.Path, "/")
			n, _ := strconv.Atoi(parts[len(parts)-2])
			if currentMode == "mixed-failure" && n == 4 {
				http.Error(w, `{"message":"reviews unavailable"}`, 403)
				return
			}
			head := fmt.Sprintf("%040x", n)
			rv := map[string]any{"id": 1000 + n, "user": map[string]any{"login": "assay-reviewer-app[bot]", "id": 300000004}, "state": "APPROVED", "commit_id": head, "submitted_at": "2026-08-01T00:00:00Z"}
			if n == 1 || n == 5 {
				rv["state"] = "CHANGES_REQUESTED"
			}
			var out []any
			if n != 3 {
				if n == 4 && r.URL.Query().Get("page") == "1" {
					for i := 0; i < 100; i++ {
						out = append(out, rv)
					}
				} else {
					out = []any{rv}
				}
			}
			json.NewEncoder(w).Encode(out)
		case strings.Contains(r.URL.Path, "/commits/"):
			json.NewEncoder(w).Encode(map[string]any{"sha": strings.Split(r.URL.Path, "/commits/")[1], "author": map[string]any{"login": "ada"}, "committer": map[string]any{"login": "ada"}, "commit": map[string]any{"committer": map[string]any{"date": "2026-08-01T00:00:00Z"}}, "files": []any{}})
		case strings.Contains(r.URL.Path, "/comments"):
			if strings.Contains(r.URL.Path, "/issues/5/") {
				json.NewEncoder(w).Encode([]any{map[string]any{"id": 1, "user": map[string]any{"login": "ada", "id": 2001}, "created_at": "2099-01-01T00:00:00Z", "body": "reply"}})
			} else {
				fmt.Fprint(w, "[]")
			}
		case strings.Contains(r.URL.Path, "/compare/"):
			fmt.Fprint(w, `{"status":"behind","behind_by":5,"files":[]}`)
		default:
			t.Errorf("unexpected fixture request %s %s", r.Method, r.URL.RequestURI())
			http.Error(w, "not served", 404)
		}
	}))
	defer srv.Close()
	forgeAPIBase = srv.URL
	for _, phase := range []string{"cold", "warm", "unchanged", "mixed-failure"} {
		mu.Lock()
		mode = phase
		paths = nil
		mu.Unlock()
		var out, errb bytes.Buffer
		start := time.Now()
		code := run([]string{"stalled", "--json"}, &out, &errb)
		elapsed := time.Since(start).Nanoseconds()
		if code != 0 {
			t.Fatalf("%s exit %d: %s", phase, code, errb.String())
		}
		var report map[string]any
		if err := json.Unmarshal(out.Bytes(), &report); err != nil {
			t.Fatal(err)
		}
		stalled := report["stalled"].([]any)
		if len(stalled) != 1 || int(stalled[0].(map[string]any)["number"].(float64)) != 1 {
			t.Fatalf("%s lost clock/head/empty-review semantics: %s", phase, out.String())
		}
		unassessable := report["unassessable"].([]any)
		wantUnknown := 0
		if phase == "mixed-failure" {
			wantUnknown = 1
		}
		if len(unassessable) != wantUnknown {
			t.Fatalf("%s failure hidden: %s", phase, out.String())
		}
		delete(report, "asOf")
		delete(report, "auditFirstTS")
		for _, raw := range stalled {
			delete(raw.(map[string]any), "daysStalled")
		}
		mu.Lock()
		requests := append([]string(nil), paths...)
		mu.Unlock()
		reviewReads := 0
		for _, p := range requests {
			if strings.Contains(p, "/reviews?") {
				reviewReads++
			}
		}
		// Emitting the semantic output and every attempt also makes this fixture
		// usable unchanged against the baseline production source for matched pairs.
		encoded, _ := json.Marshal(map[string]any{"phase": phase, "http": len(requests), "reviewHTTP": reviewReads, "elapsedNs": elapsed, "exit": code, "semantic": report, "requests": requests})
		fmt.Printf("STALLED_COST_FIXTURE %s\n", encoded)
		if reviewReads > 2 {
			t.Errorf("%s reread complete snapshot reviews: %d attempts, want only incomplete PR4 (at most2)", phase, reviewReads)
		}
	}
}
