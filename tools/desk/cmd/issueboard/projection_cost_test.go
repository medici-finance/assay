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
			return 200, `{"data":{"repository":{"issue":{"comments":{"pageInfo":{"hasNextPage":false},"nodes":[]}}}}}`
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
	forgeFor = func(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
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
