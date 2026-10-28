package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/reviewtest"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// regression: class issue-projection
func TestProjectionCostProbe(t *testing.T) {
	q := reviewtest.Read(t)
	installForge(t, nil)
	var rows []map[string]any
	for i := 1; i <= q.Size; i++ {
		labels := []map[string]string{}
		switch i % 8 {
		case 2:
			labels = append(labels, map[string]string{"name": "question"})
		case 3:
			labels = append(labels, map[string]string{"name": "to:worker"})
		case 4:
			labels = append(labels, map[string]string{"name": "review-request"})
		case 6:
			labels = append(labels, map[string]string{"name": "help wanted"})
		case 7:
			labels = append(labels, map[string]string{"name": "to:reviewer"})
		}
		rows = append(rows, map[string]any{"number": i, "title": fmt.Sprintf("candidate-%d", i), "state": "open", "user": map[string]any{"login": "shared-agent", "id": 2002}, "labels": labels, "created_at": "2026-01-01T00:00:00Z"})
		if i%8 == 5 {
			dir := filepath.Join(q.Root, issueLoopDir)
			if e := os.MkdirAll(dir, 0700); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(filepath.Join(dir, fmt.Sprintf("issue-%d.md", i)), []byte(placeholderFixture(homeRepo, "todo", "")), 0600); e != nil {
				t.Fatal(e)
			}
		}
	}
	transport := &reviewtest.Transport{Answer: func(req *http.Request) (int, string) {
		if req.Method == "POST" && req.URL.Path == "/graphql" {
			body, err := projectionCostComments(req, q.Size)
			if err != nil {
				t.Fatal(err)
			}
			return 200, body
		}
		if req.Method == "GET" && strings.HasSuffix(req.URL.Path, "/timeline") {
			return 200, `[]`
		}
		if req.Method == "GET" && strings.HasSuffix(req.URL.Path, "/issues") {
			if !strings.Contains(req.URL.Path, "/tracker/") || req.URL.Query().Get("page") != "1" {
				return 200, `[]`
			}
			b, e := json.Marshal(rows)
			if e != nil {
				t.Fatal(e)
			}
			return 200, string(b)
		}
		t.Errorf("unexpected provider request %s %s", req.Method, req.URL.Path)
		return 500, `{}`
	}}
	forgeFor = func(repo string, _ ...deskkit.RequestObserver) (deskkit.Forge, deskkit.ForgeRepo, error) {
		owner, name, _ := strings.Cut(repo, "/")
		return &deskkit.GitHubForge{Token: "local-fixture", BaseURL: "https://fixture.invalid", Client: &http.Client{Transport: transport}}, deskkit.ForgeRepo{Owner: owner, Name: name}, nil
	}
	args := []string{"--root", q.Root, "issues"}
	switch q.Case {
	case "worker":
		args = append(args, "--projection", "work-ready")
	case "intake":
		args = append(args, "--projection", "intake")
	case "inbox":
		args = append(args, "--projection", "inbox", "--to", "worker")
	case "legacy-inbox":
		args = append(args, "--to", "worker")
	case "legacy":
	default:
		t.Fatal("unknown cost fixture")
	}
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	reviewtest.Write(t, reviewtest.Response{Exit: code, Output: out.String(), Facts: map[string]string{"stderr": errb.String()}, Attempts: transport.Snapshot(), HTTPComplete: true})
}

// This fake provider must answer the operation the production reader actually
// sent. Returning the old serial field for a batch is invalid provider data, not
// a reason to relax the production alias/completeness guards.
func projectionCostComments(req *http.Request, size int) (string, error) {
	var in struct {
		Variables map[string]any `json:"variables"`
	}
	if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
		return "", err
	}
	valid := func(value any) bool {
		n, ok := value.(float64)
		return ok && n >= 1 && n <= float64(size) && n == float64(int(n))
	}
	if n, serial := in.Variables["number"]; serial {
		if !valid(n) {
			return "", fmt.Errorf("invalid serial fixture issue identity")
		}
		return `{"data":{"repository":{"issue":{"comments":{"pageInfo":{"hasNextPage":false},"nodes":[]}}}}}`, nil
	}
	repository := map[string]any{}
	for slot := 0; slot < deskkit.IssueCommentBatchSize; slot++ {
		if in.Variables[fmt.Sprintf("use%d", slot)] != true {
			continue
		}
		n := in.Variables[fmt.Sprintf("n%d", slot)]
		if !valid(n) {
			return "", fmt.Errorf("invalid batched fixture issue identity")
		}
		repository[fmt.Sprintf("a%d", slot)] = map[string]any{"number": n,
			"comments": map[string]any{"pageInfo": map[string]bool{"hasNextPage": false}, "nodes": []any{}}}
	}
	if len(repository) == 0 {
		return "", fmt.Errorf("fixture query requested no comment history")
	}
	raw, err := json.Marshal(map[string]any{"data": map[string]any{"repository": repository}})
	return string(raw), err
}

// regression: #4622
func TestCostHistoryFixture(t *testing.T) {
	request := func(variables string) *http.Request {
		r, err := http.NewRequest("POST", "https://fixture.invalid/graphql", strings.NewReader(`{"variables":`+variables+`}`))
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	serial, err := projectionCostComments(request(`{"number":2}`), 8)
	if err != nil || !strings.Contains(serial, `"issue":{"comments"`) {
		t.Fatal("serial fixture response changed", serial, err)
	}
	raw, err := projectionCostComments(request(`{"use0":true,"n0":2,"use1":false,"n1":99,"use2":true,"n2":3}`), 8)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Data struct {
			Repository map[string]struct {
				Number   int `json:"number"`
				Comments struct {
					PageInfo *struct{ HasNextPage *bool } `json:"pageInfo"`
					Nodes    []any                        `json:"nodes"`
				} `json:"comments"`
			} `json:"repository"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Data.Repository) != 2 || got.Data.Repository["a0"].Number != 2 || got.Data.Repository["a2"].Number != 3 {
		t.Fatal("fixture lost active alias identities", raw)
	}
	for _, row := range got.Data.Repository {
		if row.Comments.PageInfo == nil || row.Comments.PageInfo.HasNextPage == nil || *row.Comments.PageInfo.HasNextPage || row.Comments.Nodes == nil || len(row.Comments.Nodes) != 0 {
			t.Fatal("fixture invented incomplete/nonempty comment history", raw)
		}
	}
	for _, bad := range []string{`{"number":0}`, `{"use0":true,"n0":9}`, `{"use0":false,"n0":2}`} {
		if _, err := projectionCostComments(request(bad), 8); err == nil {
			t.Fatal("fixture accepted an invalid or empty selection", bad)
		}
	}
}
