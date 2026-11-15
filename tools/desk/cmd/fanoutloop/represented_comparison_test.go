package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestRepresentedMatchedComparison is an opt-in offline experiment, not a production
// latency assertion. Both paths execute the same complete planner/authoring workload;
// the loopback provider adds a declared one-millisecond per-request service delay.
func TestRepresentedMatchedComparison(t *testing.T) {
	destination := os.Getenv("FANOUTLOOP_COMPARE_FILE")
	if destination == "" {
		t.Skip("set FANOUTLOOP_COMPARE_FILE for the bounded loopback experiment")
	}
	setupDeskHome(t)
	var calls atomic.Int64
	filesFor := func(n int) []deskkit.ChangedFile {
		id := n % 100
		out := []deskkit.ChangedFile{{Filename: fmt.Sprintf("docs/streams/example-port/brief-%02d-test.md", id), Status: "added"}}
		if n != 101 {
			out = append(out, deskkit.ChangedFile{Filename: "tools/code.go", Status: "modified"})
		}
		if n == 102 {
			out[1] = deskkit.ChangedFile{Filename: "docs/streams/example-port/README.md", Status: "renamed", PreviousFilename: "tools/old.go"}
		}
		return out
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		time.Sleep(time.Millisecond)
		if r.URL.Path == "/graphql" {
			var in struct {
				Query     string
				Variables map[string]any
			}
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
				t.Error(err)
				http.Error(w, "bad fixture request", 400)
				return
			}
			values := map[string]any{}
			for i := 0; i < 32; i++ {
				if in.Variables[fmt.Sprintf("use%d", i)] != true {
					continue
				}
				n := int(in.Variables[fmt.Sprintf("n%d", i)].(float64))
				files := filesFor(n)
				node := map[string]any{"number": n, "state": "OPEN", "merged": false, "headRefOid": strings.Repeat("a", 40), "baseRefName": "main", "changedFiles": len(files), "author": map[string]any{"login": "fixture", "__typename": "Bot", "databaseId": 300000005}, "headRepository": map[string]any{"nameWithOwner": "example-org/example-repo"}, "baseRepository": map[string]any{"nameWithOwner": "example-org/example-repo"}}
				if strings.Contains(in.Query, "files(first:100)") {
					nodes := []any{}
					for _, f := range files {
						kind := strings.ToUpper(f.Status)
						nodes = append(nodes, map[string]any{"path": f.Filename, "changeType": kind})
					}
					node["files"] = map[string]any{"totalCount": len(files), "pageInfo": map[string]any{"hasNextPage": false}, "nodes": nodes}
				}
				values[fmt.Sprintf("a%d", i)] = node
			}
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"repository": values, "_assayPerf": map[string]any{"cost": 1}}})
			return
		}
		var n int
		fmt.Sscanf(r.URL.Path, "/repos/example-org/example-repo/pulls/%d", &n)
		files := filesFor(n)
		if strings.HasSuffix(r.URL.Path, "/files") {
			out := []any{}
			for _, f := range files {
				out = append(out, map[string]any{"filename": f.Filename, "status": f.Status, "previous_filename": f.PreviousFilename})
			}
			json.NewEncoder(w).Encode(out)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"number": n, "state": "open", "merged": false, "changed_files": len(files), "user": map[string]any{"login": "fixture[bot]", "id": 300000005}, "head": map[string]any{"sha": strings.Repeat("a", 40), "repo": map[string]any{"full_name": "example-org/example-repo"}}, "base": map[string]any{"ref": "main", "repo": map[string]any{"full_name": "example-org/example-repo"}}})
	}))
	defer server.Close()
	var prs []deskkit.PRRef
	var rows []BoardRow
	for i := 1; i <= 33; i++ {
		prs = append(prs, deskkit.PRRef{Number: 100 + i, State: "OPEN", Body: fmt.Sprintf("Brief: example-port/%02d", i)})
		rows = append(rows, briefRow("example-port", fmt.Sprintf("%02d", i), "M", "", "model", false))
	}
	prs = append(prs, deskkit.PRRef{Number: 1, State: "OPEN", Body: "Brief: example-port/01"})
	repo := deskkit.ForgeRepo{Owner: "example-org", Name: "example-repo"}
	g := &deskkit.GitHubForge{Token: "fixture", BaseURL: server.URL, Client: server.Client()}
	withPlanTransports(t, prs, func(_ string, n int) ([]deskkit.ChangedFile, error) { return deskkit.CompleteChangedFiles(g, repo, n) })
	withBatchFiles(t, nil)
	type sample struct {
		Pair      int    `json:"pair"`
		Mode      string `json:"mode"`
		ElapsedNS int64  `json:"elapsed_ns"`
		HTTP      int64  `json:"http"`
		OutputSHA string `json:"output_sha256"`
	}
	var samples []sample
	var expected []byte
	for pair := 0; pair < 10; pair++ {
		modes := []string{"baseline", "candidate"}
		if pair%2 == 1 {
			modes = []string{"candidate", "baseline"}
		}
		for _, mode := range modes {
			representedPRBatchFiles = nil
			if mode == "candidate" {
				representedPRBatchFiles = func(_ string, ns []int) map[int]deskkit.PendingChangeObservation {
					return representedFilesBatch(g, repo, ns, deskkit.PendingChangeBatchReader(g))
				}
			}
			before := calls.Load()
			started := time.Now()
			var out bytes.Buffer
			if err := renderPlan(planWithRows(rows), &out); err != nil {
				t.Fatal(err)
			}
			elapsed := time.Since(started)
			if expected == nil {
				expected = append([]byte(nil), out.Bytes()...)
			} else if !bytes.Equal(expected, out.Bytes()) {
				t.Fatal("planner output changed")
			}
			samples = append(samples, sample{Pair: pair, Mode: mode, ElapsedNS: elapsed.Nanoseconds(), HTTP: calls.Load() - before, OutputSHA: fmt.Sprintf("%x", sha256.Sum256(out.Bytes()))})
		}
	}
	data, err := json.MarshalIndent(samples, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(destination, append(data, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}
