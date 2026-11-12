package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The production actions collector and typed GitHub adapter read a frozen
// zero-rollup PR. Count HTTP attempts, including 304s, rather than quota points.
func TestActionsZeroCIReadsOnce(t *testing.T) {
	for _, scenario := range []string{"empty", "statuses", "path-filter"} {
		t.Run(scenario, func(t *testing.T) {
			statuses := 0
			if scenario == "statuses" {
				statuses = 1
			}
			currentStatus, currentFile := statuses, "docs/readme.md"
			ff := newEtagForge(t, 1)
			ff.snapshotVisibility = snapshotVisibility("PRIVATE")
			head := ff.prs[0].head
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/graphql" {
					ff.mu.Lock()
					defer ff.mu.Unlock()
					ff.all = append(ff.all, "POST /graphql")
					ff.charged = append(ff.charged, "POST /graphql")
					snapshot := ff.snapshotLocked().(map[string]any)
					repo := snapshot["data"].(map[string]any)["repository"].(map[string]any)
					nodes := repo["pullRequests"].(map[string]any)["nodes"].([]any)
					for _, node := range nodes {
						commits := node.(map[string]any)["commits"].(map[string]any)["nodes"].([]any)
						commits[0].(map[string]any)["commit"].(map[string]any)["statusCheckRollup"] = nil
					}
					json.NewEncoder(w).Encode(snapshot)
					return
				}
				headPath := "/repos/" + fakeRepo + "/commits/" + head
				if r.URL.Path == headPath+"/status" || r.URL.Path == headPath+"/check-runs" || strings.Contains(r.URL.Path, "/contents/.github/workflows") || (scenario == "path-filter" && rePullFiles.MatchString(r.URL.Path)) {
					ff.mu.Lock()
					ff.all = append(ff.all, "GET "+r.URL.Path)
					ff.charged = append(ff.charged, "GET "+r.URL.Path)
					status, file := currentStatus, currentFile
					ff.mu.Unlock()
					switch r.URL.Path {
					case headPath + "/status":
						fmt.Fprintf(w, `{"state":"pending","total_count":%d,"statuses":[]}`, status)
					case headPath + "/check-runs":
						fmt.Fprint(w, `{"total_count":0,"check_runs":[]}`)
					case "/repos/" + fakeRepo + "/pulls/1/files":
						fmt.Fprintf(w, `[{"filename":%q,"status":"modified"}]`, file)
					case "/repos/" + fakeRepo + "/contents/.github/workflows":
						if scenario == "path-filter" {
							fmt.Fprint(w, `[{"name":"tools.yml","type":"file","path":".github/workflows/tools.yml"}]`)
						} else {
							http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
						}
					case "/repos/" + fakeRepo + "/contents/.github/workflows/tools.yml":
						fmt.Fprint(w, strings.Replace(workflowFixture("on:\n  pull_request:\n    paths: ['tools/**']\n"), `{"content":`, `{"sha":"`+strings.Repeat("b", 40)+`","content":`, 1))
					default:
						http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
					}
					return
				}
				ff.serve(w, r)
			}))
			defer srv.Close()
			forgeAPIBase = srv.URL
			for _, phase := range []string{"cold", "unchanged", "changed"} {
				if phase == "changed" {
					ff.mu.Lock()
					if scenario == "path-filter" {
						currentFile = "tools/ci.go"
					} else {
						currentStatus = 1
					}
					ff.mu.Unlock()
				}
				res := ff.sweep(t)
				row := boardRows(t, res)[1]
				want := zeroCINoChecks
				if statuses > 0 || (phase == "changed" && scenario != "path-filter") {
					want = zeroCIUnverified
				}
				if phase == "changed" && scenario == "path-filter" {
					want = zeroCINeverRan
				}
				if row.CIZero != want {
					t.Fatalf("%s: zero=%s want %s: %s", phase, row.CIZero, want, row.CIZeroDetail)
				}
				row.OpenAge, row.ApprovedAge = "", ""
				semantic, err := json.Marshal(row)
				if err != nil {
					t.Fatal(err)
				}
				t.Logf("ZERO_CI_COST scenario=%s phase=%s attempts=%d semantic=%s", scenario, phase, len(res.all), semantic)
				if scenario == "path-filter" {
					for _, path := range []string{"/pulls/1", "/pulls/1/files"} {
						target, n := "GET /repos/"+fakeRepo+path, 0
						for _, req := range res.all {
							if req == target {
								n++
							}
						}
						if n != 1 {
							t.Errorf("%s: file observation %s read %d times, want one fresh read", phase, path, n)
						}
					}
				}
				for _, suffix := range []string{"/check-runs", "/status"} {
					target := "GET /repos/" + fakeRepo + "/commits/" + head + suffix
					n := 0
					for _, req := range res.all {
						if req == target {
							n++
						}
					}
					if n != 1 {
						t.Errorf("%s: %s read %d times, want one fresh read", phase, suffix, n)
					}
				}
			}
		})
	}
}
