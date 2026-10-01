package deskkit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// One case inventory is driven through both backend fixtures. Provider-only shapes carry
// an explicit unsupported reason on the other backend instead of disappearing from its suite.
var platformCases = []string{
	"ce_404_approvals", "ce_404_approval_rules", "ce_404_push_rules", "free_tier_403",
	"last_pipeline_empty", "last_pipeline_absent", "x_next_page", "nested_subgroup",
	"internal_visibility", "files_api_400", "draft_change_400", "mr_note_vs_issue_note",
}

func TestContractConformance(t *testing.T) {
	for _, backend := range []string{"github", "gitlab"} {
		t.Run(backend, func(t *testing.T) {
			for _, name := range platformCases {
				t.Run(name, func(t *testing.T) { platformContract(t, backend, name) })
			}
		})
	}
}

func platformContract(t *testing.T, backend, name string) {
	repo := ForgeRepo{Owner: "example", Name: "repo"}
	gh, gl := newGoldenServer(t), newGLServer(t)
	var f Forge = gh.forge()
	requests := func() []reqCapture { return gh.requests }
	if backend == "gitlab" {
		f = gl.forge()
		requests = func() []reqCapture { return gl.requests }
	}
	switch name {
	case "ce_404_approvals", "ce_404_push_rules":
		kind := HardeningReadApprovals
		if name == "ce_404_push_rules" {
			kind = HardeningReadPushRules
		}
		if backend == "gitlab" {
			if kind == HardeningReadApprovals {
				gl.projApprovalStatus = 404
			} else {
				gl.forceStatus["/push_rule"] = 404
			}
		}
		_, err := f.RepoHardeningRead(repo, kind)
		if err == nil {
			t.Fatal("tier gap was reported as a known configuration")
		}
		if backend == "github" {
			t.Log("unsupported: GitHub has neither GitLab approval settings nor push rules")
			if len(requests()) != 0 {
				t.Fatal("unsupported kind contacted forge")
			}
		} else {
			if !strings.Contains(err.Error(), "404") {
				t.Fatalf("tier gap lost its reason: %v", err)
			}
			if len(requests()) != 1 {
				t.Fatalf("tier gap should make one request: %v", requests())
			}
		}
	case "ce_404_approval_rules":
		// There is no approval-rules kind in the frozen seam. Do not add a raw endpoint merely
		// to satisfy a fixture: both backends must refuse before sending a request.
		t.Log("unsupported: the frozen Forge seam exposes approvals configuration, not approval-rules")
		_, err := f.RepoHardeningRead(repo, HardeningReadKind("approval-rules"))
		if err == nil || len(requests()) != 0 {
			t.Fatalf("unlisted read admitted: %v %v", err, requests())
		}
	case "free_tier_403":
		gh.forceStatus["/pulls/7"] = 403
		gl.forceStatus["/merge_requests/7"] = 403
		_, err := f.GetPullRequest(repo, 7)
		if err == nil || !strings.Contains(err.Error(), "403") {
			t.Fatalf("permission gap lost: %v", err)
		}
	case "last_pipeline_empty", "last_pipeline_absent":
		gl.commit = map[string]any{"id": "example-head", "status": "success"}
		if name == "last_pipeline_empty" {
			gl.commit["last_pipeline"] = map[string]any{}
		}
		gl.statuses = []map[string]any{}
		gh.status = map[string]any{"state": "success", "statuses": []any{}}
		gh.checks = map[string]any{"check_runs": []any{}}
		checks, err := f.ChecksAtHead(repo, "example-head")
		if err != nil {
			t.Fatal(err)
		}
		if len(checks.CheckRuns) != 0 || (name == "last_pipeline_absent" && len(checks.Statuses) != 0) {
			t.Fatalf("empty observation fabricated checks: %+v", checks)
		}
		for _, status := range checks.Statuses {
			if status.State == "success" {
				t.Fatal("empty pipeline invented a passing verdict")
			}
		}
		if backend == "github" {
			t.Log("unsupported: last_pipeline is GitLab-only; equivalent missing check rollup exercised")
		}
		if len(requests()) > 4 {
			t.Fatalf("unbounded pipeline fallback: %d", len(requests()))
		}
	case "x_next_page":
		// Both pagination signals must preserve the same two-item result. Short pages carry
		// the continuation header so stopping by page length loses the second item.
		seen := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seen++
			w.Header().Set("Content-Type", "application/json")
			page := r.URL.Query().Get("page")
			if page != "2" {
				if backend == "gitlab" {
					w.Header().Set("X-Next-Page", "2")
				} else {
					w.Header().Set("Link", fmt.Sprintf("<http://%s%s?state=open&per_page=100&page=2>; rel=\"next\"", r.Host, r.URL.Path))
				}
			}
			n := 1
			if page == "2" {
				n = 2
			}
			if backend == "gitlab" {
				json.NewEncoder(w).Encode([]any{glIssue(map[string]any{"id": n, "iid": n, "title": "example"})})
			} else {
				json.NewEncoder(w).Encode([]any{map[string]any{"number": n, "title": "example", "user": map[string]any{"id": 1, "login": "example"}}})
			}
		}))
		defer srv.Close()
		if backend == "github" {
			f = &GitHubForge{Token: "synthetic-minted", BaseURL: srv.URL, Client: srv.Client()}
		} else {
			f = &GitLabForge{Token: "synthetic-minted", BaseURL: srv.URL, Client: srv.Client()}
		}
		items, err := f.ListOpenIssues(repo)
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 2 || seen != 2 {
			t.Fatalf("pagination lost or duplicated data: len=%d requests=%d", len(items), seen)
		}
	case "nested_subgroup":
		if backend == "github" {
			t.Log("unsupported: GitHub repository coordinates do not have nested subgroups")
			return
		}
		gl.project = map[string]any{"visibility": "private"}
		repo.Owner = "group/sub"
		repo.Name = "project"
		got, err := f.RepoVisibility(repo)
		if err != nil || got != "private" {
			t.Fatalf("visibility=%q: %v", got, err)
		}
		if len(requests()) != 1 || !strings.Contains(requests()[0].Path, "group%2Fsub%2Fproject") {
			t.Fatalf("nested group escaped incorrectly: %v", requests())
		}
	case "internal_visibility":
		gl.project = map[string]any{"visibility": "internal"}
		gh.repo = map[string]any{"visibility": "private"}
		got, err := f.RepoVisibility(repo)
		if err != nil {
			t.Fatal(err)
		}
		want := "private"
		if backend == "gitlab" {
			want = "internal"
		}
		if got != want {
			t.Fatalf("visibility=%q want %q", got, want)
		}
		if backend == "github" {
			t.Log("unsupported: GitHub internal visibility is represented by the private flag at this boundary")
		}
	case "files_api_400":
		gl.project = map[string]any{"visibility": "private", "default_branch": "main"}
		gl.repoFile = map[string]map[string]any{"EXAMPLE.md": {"file_name": "EXAMPLE.md", "file_path": "EXAMPLE.md", "content": glB64("old\n"), "encoding": "base64", "last_commit_id": "example-commit", "blob_id": "example-blob"}}
		gl.updateFileStatus = 400
		gh.repo = map[string]any{"private": true, "default_branch": "main"}
		gh.contentsGet = map[string]any{"type": "file", "sha": "example-blob", "content": glB64("old\n"), "encoding": "base64"}
		if backend == "github" {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "PUT" {
					gh.requests = append(gh.requests, reqCapture{Method: r.Method, Path: r.URL.EscapedPath()})
					w.WriteHeader(400)
					fmt.Fprint(w, `{"message":"synthetic update rejected"}`)
					return
				}
				gh.handler(w, r)
			}))
			defer srv.Close()
			f = &GitHubForge{Token: "synthetic-minted", BaseURL: srv.URL, Client: srv.Client()}
		}
		_, err := f.WriteFile(repo, WriteFileInput{File: "EXAMPLE.md", Branch: "example/branch", Content: []byte("old\nnew\n"), Message: "example update"})
		if err == nil {
			t.Fatal("failed update succeeded")
		}
		wrote := false
		for _, r := range requests() {
			if r.Method == "POST" {
				t.Fatalf("failed update widened to create: %v", requests())
			}
			if r.Method == "PUT" {
				wrote = true
			}
		}
		if !wrote {
			t.Fatalf("fixture failed before update: %v %v", err, requests())
		}
	case "draft_change_400":
		gl.createMRStatus = 400
		gl.createMRErrBody = map[string]any{"message": "synthetic create rejected"}
		if backend == "github" {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(400)
				fmt.Fprint(w, `{"message":"synthetic create rejected"}`)
			}))
			defer srv.Close()
			f = &GitHubForge{Token: "synthetic-minted", BaseURL: srv.URL, Client: srv.Client()}
		}
		_, err := f.CreateDraftChange(repo, DraftChangeInput{Title: "example", Body: "example", Head: "example/branch", Base: "main"})
		if err == nil || !strings.Contains(err.Error(), "400") {
			t.Fatalf("creation failure lost detail: %v", err)
		}
		if backend == "gitlab" && !strings.Contains(err.Error(), "synthetic create rejected") {
			t.Fatalf("GitLab creation failure lost its message: %v", err)
		}
	case "mr_note_vs_issue_note":
		// Explicit object kinds must route the write without resolving the other object first.
		for _, kind := range []TargetKind{TargetIssue, TargetChange} {
			before := len(requests())
			if _, err := f.PostCommentTyped(repo, 7, kind, "example note"); err != nil {
				t.Fatal(err)
			}
			rs := requests()[before:]
			if len(rs) != 1 || rs[0].Method != "POST" {
				t.Fatalf("typed write made resolving reads: %v", rs)
			}
			want := "/issues/7/"
			if backend == "gitlab" && kind == TargetChange {
				want = "/merge_requests/7/"
			}
			if !strings.Contains(rs[0].Path, want) {
				t.Fatalf("wrong object thread: %v", rs)
			}
		}
	default:
		t.Fatalf("unimplemented shared case %q", name)
	}
}
