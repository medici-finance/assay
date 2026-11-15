package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #5022
func TestRepresentedBatchSingleton(t *testing.T) {
	for _, status := range []string{"renamed", "copied"} {
		t.Run(status, func(t *testing.T) {
			var calls atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if strings.HasSuffix(r.URL.Path, "/files") {
					json.NewEncoder(w).Encode([]any{map[string]any{"filename": "docs/streams/example-port/brief-11-test.md", "status": "added"}, map[string]any{"filename": "docs/streams/example-port/README.md", "status": status, "previous_filename": "tools/old.go"}})
					return
				}
				if r.URL.Path == "/graphql" {
					t.Error("singleton should keep the shared native reader")
					http.Error(w, "unexpected query", 400)
					return
				}
				json.NewEncoder(w).Encode(map[string]any{"number": 11, "state": "open", "merged": false, "changed_files": 2, "user": map[string]any{"login": "fixture[bot]", "id": 300000005}, "head": map[string]any{"sha": strings.Repeat("a", 40), "repo": map[string]any{"full_name": "example-org/example-repo"}}, "base": map[string]any{"ref": "main", "repo": map[string]any{"full_name": "example-org/example-repo"}}})
			}))
			defer server.Close()
			g := &deskkit.GitHubForge{Token: "fixture", BaseURL: server.URL, Client: server.Client()}
			repo := deskkit.ForgeRepo{Owner: "example-org", Name: "example-repo"}
			got := representedFilesBatch(g, repo, []int{11}, deskkit.PendingChangeBatchReader(g))[11]
			if got.Err != nil || deskkit.BriefAuthoringOnly("example-port/11", got.Files) || got.Files[1].PreviousFilename != "tools/old.go" {
				t.Fatalf("singleton lost complete ancestry: %+v", got)
			}
			if calls.Load() != 2 {
				t.Fatalf("shared singleton already read complete ancestry: got %d HTTP calls, want 2", calls.Load())
			}
		})
	}
}

// regression: #5022
func TestRepresentedBatchOverflow(t *testing.T) {
	for _, status := range []string{"renamed", "copied"} {
		t.Run(status, func(t *testing.T) {
			var calls atomic.Int64
			files := []deskkit.ChangedFile{{Filename: "docs/streams/example-port/brief-11-test.md", Status: "added"}, {Filename: "docs/streams/example-port/README.md", Status: status, PreviousFilename: "tools/old.go"}}
			for i := 0; i < 99; i++ {
				files = append(files, deskkit.ChangedFile{Filename: fmt.Sprintf("changelog/fixture-%d.md", i), Status: "modified"})
			}
			metadata := func(n int) map[string]any {
				count := 1
				if n == 11 {
					count = len(files)
				}
				return map[string]any{"number": n, "state": "OPEN", "merged": false, "headRefOid": strings.Repeat("a", 40), "baseRefName": "main", "changedFiles": count, "author": map[string]any{"login": "fixture", "__typename": "Bot", "databaseId": 300000005}, "headRepository": map[string]any{"nameWithOwner": "example-org/example-repo"}, "baseRepository": map[string]any{"nameWithOwner": "example-org/example-repo"}}
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.URL.Path == "/graphql" {
					var in struct {
						Query     string
						Variables map[string]any
					}
					if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
						t.Error(err)
						return
					}
					values := map[string]any{}
					for i := 0; i < 32; i++ {
						if in.Variables[fmt.Sprintf("use%d", i)] != true {
							continue
						}
						n := int(in.Variables[fmt.Sprintf("n%d", i)].(float64))
						node := metadata(n)
						if strings.Contains(in.Query, "files(first:100)") {
							nodes := []any{map[string]any{"path": "tools/code.go", "changeType": "MODIFIED"}}
							count := 1
							if n == 11 {
								count = len(files)
								nodes = nil
								for _, f := range files[:100] {
									nodes = append(nodes, map[string]any{"path": f.Filename, "changeType": strings.ToUpper(f.Status)})
								}
							}
							node["files"] = map[string]any{"totalCount": count, "pageInfo": map[string]any{"hasNextPage": n == 11}, "nodes": nodes}
						}
						values[fmt.Sprintf("a%d", i)] = node
					}
					json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"repository": values, "_assayPerf": map[string]any{"cost": 1}}})
					return
				}
				var n int
				fmt.Sscanf(r.URL.Path, "/repos/example-org/example-repo/pulls/%d", &n)
				if strings.HasSuffix(r.URL.Path, "/files") {
					page, _ := strconv.Atoi(r.URL.Query().Get("page"))
					out := []any{}
					for i := (page - 1) * 100; i < min(page*100, len(files)); i++ {
						f := files[i]
						out = append(out, map[string]any{"filename": f.Filename, "status": f.Status, "previous_filename": f.PreviousFilename})
					}
					json.NewEncoder(w).Encode(out)
					return
				}
				json.NewEncoder(w).Encode(map[string]any{"number": n, "state": "open", "merged": false, "changed_files": len(files), "user": map[string]any{"login": "fixture[bot]", "id": 300000005}, "head": map[string]any{"sha": strings.Repeat("a", 40), "repo": map[string]any{"full_name": "example-org/example-repo"}}, "base": map[string]any{"ref": "main", "repo": map[string]any{"full_name": "example-org/example-repo"}}})
			}))
			defer server.Close()
			g := &deskkit.GitHubForge{Token: "fixture", BaseURL: server.URL, Client: server.Client()}
			repo := deskkit.ForgeRepo{Owner: "example-org", Name: "example-repo"}
			got := representedFilesBatch(g, repo, []int{11, 12}, deskkit.PendingChangeBatchReader(g))[11]
			if got.Err != nil || len(got.Files) != 101 || got.Files[1].PreviousFilename != "tools/old.go" || deskkit.BriefAuthoringOnly("example-port/11", got.Files) {
				t.Fatalf("overflow lost native ancestry: %+v", got)
			}
			if calls.Load() != 5 {
				t.Fatalf("overflow already read native ancestry: got %d HTTP calls, want 5", calls.Load())
			}
		})
	}
}
