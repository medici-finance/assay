package deskkit

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Count real transport requests, not calls to the Forge mock. The native
// edit-history and label-provenance readers must still reach GitLab.
func TestGitLabEvidenceLandingRequests(t *testing.T) {
	for _, scenario := range []string{"update", "conflict", "protected-default"} {
		t.Run(scenario, func(t *testing.T) {
			obRoster(t)
			repo := obRepo(obPrivate)
			const file = "docs/streams/example/brief-01-example.md"
			var calls []string
			var written bool
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls = append(calls, r.Method+" "+r.URL.Path)
				if r.Header.Get("PRIVATE-TOKEN") != "fixture-token" {
					t.Error("missing scoped GitLab credential")
				}
				enc := func(v any) { _ = json.NewEncoder(w).Encode(v) }
				switch {
				case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/users/1"):
					enc(map[string]any{"id": 1, "username": "dispatcher", "bot": false})
				case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/issues/77"):
					enc(map[string]any{"id": 77, "iid": 77, "title": "attestation", "description": "fixture", "state": "closed", "labels": []string{"model:fixture"}, "author": map[string]any{"username": "dispatcher", "id": 1}})
				case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/api/graphql"):
					enc(map[string]any{"data": map[string]any{"project": map[string]any{"issue": map[string]any{
						"comments": map[string]any{"nodes": []any{}, "pageInfo": map[string]any{"hasNextPage": false}},
						"activity": map[string]any{"nodes": []any{}, "pageInfo": map[string]any{"hasPreviousPage": false}},
					}}}})
				case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/resource_label_events"):
					enc([]any{map[string]any{"action": "add", "label": map[string]any{"name": "model:fixture"}, "user": map[string]any{"username": "dispatcher"}}})
				case strings.Contains(r.URL.Path, "/repository/files/"):
					if r.Method == "GET" {
						if r.URL.Query().Get("ref") == "evidence/fixture" {
							w.WriteHeader(404)
							return
						}
						enc(map[string]any{"last_commit_id": "observed-file-commit", "blob_id": "blob", "content": base64.StdEncoding.EncodeToString([]byte("old evidence\n"))})
						return
					}
					var in struct {
						LastCommitID string `json:"last_commit_id"`
						StartBranch  string `json:"start_branch"`
						Branch       string
					}
					_ = json.NewDecoder(r.Body).Decode(&in)
					if in.LastCommitID != "observed-file-commit" {
						t.Errorf("lost snapshot precondition: %+v", in)
					}
					if scenario == "conflict" {
						w.WriteHeader(400)
						enc(map[string]any{"message": "last commit changed"})
						return
					}
					if scenario == "protected-default" && (in.StartBranch != "main" || in.Branch != "evidence/fixture") {
						t.Errorf("lost inline branch creation: %+v", in)
					}
					written = true
					enc(map[string]any{"file_path": file})
				case r.Method == "GET" && strings.HasSuffix(r.URL.Path, repo.Slug()):
					enc(map[string]any{"visibility": "private", "default_branch": "main"})
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
					w.WriteHeader(404)
				}
			}))
			defer srv.Close()
			branch := "work"
			if scenario == "protected-default" {
				branch = "main"
			}
			f := EvidenceLandingForge(OutboundChecked(&GitLabForge{Token: "fixture-token", BaseURL: srv.URL}, "verifier"), repo, file, branch)
			for i := 0; i < 2; i++ {
				if _, err := f.GetIssueTyped(repo, 77, TargetIssue); err != nil {
					t.Fatal(err)
				}
			}
			if trust, err := f.IssueTrustEvents(repo, 77); err != nil || !trust.Complete || !trust.BodyHistoryKnown {
				t.Fatalf("native trust history lost: %+v %v", trust, err)
			}
			if events, err := f.ListIssueLabelEvents(repo, 77); err != nil || len(events) != 1 || events[0].AppliedBy != "dispatcher" {
				t.Fatalf("native label history lost: %+v %v", events, err)
			}
			for i := 0; i < 2; i++ {
				if _, err := f.ReadFile(repo, ReadFileInput{File: file, Ref: branch}); err != nil {
					t.Fatal(err)
				}
			}
			if v, err := f.RepoVisibility(repo); err != nil || v != "private" {
				t.Fatalf("visibility: %s %v", v, err)
			}
			input := WriteFileInput{File: file, Branch: branch, Content: []byte("old evidence\nnew evidence\n"), Message: "Evidence: fixture", AppendOnly: true}
			res, err := f.WriteFile(repo, input)
			if scenario == "protected-default" {
				if err != nil || !res.DefaultBranchNotWritable || written || len(calls) != 6 {
					t.Fatalf("default branch must produce no write or extra probe: %+v %v calls=%v", res, err, calls)
				}
				input.Branch, input.StartBranch = "evidence/fixture", "main"
				res, err = f.WriteFile(repo, input)
			}
			if scenario == "conflict" {
				if err == nil || written {
					t.Fatal("stale GitLab revision was accepted")
				}
			} else if err != nil || !written || !res.Changed {
				t.Fatalf("write failed: %+v %v", res, err)
			}
			want := 7 // includes GitLab's native author identity lookup
			if scenario == "protected-default" {
				want++
			} // side-branch existence read for the outbound checker
			if len(calls) != want {
				t.Fatalf("want %d requests with native provenance, got %d: %v", want, len(calls), calls)
			}
		})
	}
}

func TestEvidenceLandingDoesNotUnwrapReadOnly(t *testing.T) {
	f := ReadOnly(OutboundChecked(&GitHubForge{}, "verifier"))
	if EvidenceLandingForge(f, ForgeRepo{}, "file", "main") != f {
		t.Fatal("snapshot helper unwrapped credential fence")
	}
}
