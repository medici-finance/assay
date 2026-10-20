package deskkit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func snapshotFixture() map[string]any {
	page := func() map[string]any {
		return map[string]any{"nodes": []any{}, "pageInfo": map[string]any{"hasNextPage": false}}
	}
	return map[string]any{
		"visibility": "PUBLIC",
		"ref": map[string]any{"target": map[string]any{
			"__typename": "Commit", "oid": strings.Repeat("a", 40), "file": nil,
		}},
		"issue": map[string]any{
			"number": 77, "title": "admission", "body": "fixture", "state": "OPEN",
			"url": "https://example.invalid/issues/77", "lastEditedAt": nil,
			"author":   map[string]any{"login": "example-app", "__typename": "Bot", "databaseId": 42},
			"comments": page(), "labels": page(), "timelineItems": page(),
		},
	}
}

func missingSnapshotError() map[string]any {
	return map[string]any{
		"type": "NOT_FOUND", "path": []any{"repository", "ref", "target", "file"},
		"message": "untrusted-message-marker",
	}
}

// regression: #3231
func TestSnapshotMissingFile(t *testing.T) {
	cases := []struct {
		name, file, hint string
		want             bool
		mutate           func(map[string]any, map[string]any) []any
	}{
		{name: "new outcome", want: true},
		{name: "second destination", file: "docs/streams/example/new-evidence.md", want: true},
		{name: "absence without errors", want: true, mutate: func(_, _ map[string]any) []any { return nil }},
		{name: "existing file", want: true, mutate: func(repo, _ map[string]any) []any {
			body := "existing\n"
			repo["ref"].(map[string]any)["target"].(map[string]any)["file"] = map[string]any{
				"mode": 0100644, "object": map[string]any{"__typename": "Blob", "oid": blobID(body, false), "text": body, "byteSize": len(body), "isBinary": false},
			}
			return nil
		}},
		{name: "unknown error", mutate: func(_, e map[string]any) []any { e["type"] = "FORBIDDEN"; return []any{e} }},
		{name: "missing type", mutate: func(_, e map[string]any) []any { delete(e, "type"); return []any{e} }},
		{name: "missing path", mutate: func(_, e map[string]any) []any { delete(e, "path"); return []any{e} }},
		{name: "branch not found", hint: "NOT_FOUND", mutate: func(_, e map[string]any) []any {
			e["path"] = []any{"repository", "ref"}
			return []any{e}
		}},
		{name: "attestation not found", hint: "NOT_FOUND", mutate: func(_, e map[string]any) []any {
			e["path"] = []any{"repository", "issue"}
			return []any{e}
		}},
		{name: "non-string path", mutate: func(_, e map[string]any) []any {
			e["path"] = []any{"repository", "ref", "target", 0}
			return []any{e}
		}},
		{name: "mixed errors", mutate: func(_, e map[string]any) []any {
			return []any{e, map[string]any{"type": "FORBIDDEN", "message": "untrusted-message-marker"}}
		}},
		{name: "duplicate errors", mutate: func(_, e map[string]any) []any { return []any{e, e} }},
		{name: "missing repository", mutate: func(repo, e map[string]any) []any {
			for k := range repo {
				delete(repo, k)
			}
			return []any{e}
		}},
		{name: "missing branch", mutate: func(repo, e map[string]any) []any { repo["ref"] = nil; return []any{e} }},
		{name: "non-commit target", mutate: func(repo, e map[string]any) []any {
			repo["ref"].(map[string]any)["target"].(map[string]any)["__typename"] = "Blob"
			return []any{e}
		}},
		{name: "missing commit id", mutate: func(repo, e map[string]any) []any {
			delete(repo["ref"].(map[string]any)["target"].(map[string]any), "oid")
			return []any{e}
		}},
		{name: "missing file lookup", mutate: func(repo, e map[string]any) []any {
			delete(repo["ref"].(map[string]any)["target"].(map[string]any), "file")
			return []any{e}
		}},
		{name: "contradictory file", mutate: func(repo, e map[string]any) []any {
			body := "existing\n"
			repo["ref"].(map[string]any)["target"].(map[string]any)["file"] = map[string]any{
				"mode": 0100644, "object": map[string]any{"__typename": "Blob", "oid": blobID(body, false), "text": body, "byteSize": len(body), "isBinary": false},
			}
			return []any{e}
		}},
		{name: "missing attestation", mutate: func(repo, e map[string]any) []any { repo["issue"] = nil; return []any{e} }},
		{name: "unknown visibility", mutate: func(repo, e map[string]any) []any { delete(repo, "visibility"); return []any{e} }},
		{name: "missing edit history", mutate: func(repo, e map[string]any) []any {
			delete(repo["issue"].(map[string]any), "lastEditedAt")
			return []any{e}
		}},
		{name: "partial comments", mutate: func(repo, e map[string]any) []any {
			repo["issue"].(map[string]any)["comments"].(map[string]any)["pageInfo"] = map[string]any{"hasNextPage": nil}
			return []any{e}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, e := snapshotFixture(), missingSnapshotError()
			errs := []any{e}
			if tc.mutate != nil {
				errs = tc.mutate(repo, e)
			}
			file := tc.file
			if file == "" {
				file = "docs/streams/verify-outcomes/example/01-new.json"
			}
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				var in struct {
					Query     string
					Variables map[string]any
				}
				if r.Method != http.MethodPost || r.URL.Path != "/graphql" || r.Header.Get("Authorization") != "token stub" {
					t.Errorf("unexpected snapshot request: %s %s", r.Method, r.URL.Path)
				}
				if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
					t.Error(err)
				}
				if in.Query != githubEvidenceSnapshotQuery || in.Variables["file"] != file || in.Variables["branch"] != "refs/heads/evidence" {
					t.Errorf("snapshot request lost its destination binding: %+v", in.Variables)
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"repository": repo}, "errors": errs})
			}))
			defer srv.Close()
			g := &GitHubForge{Token: "stub", BaseURL: srv.URL, Client: srv.Client()}
			s, err := g.evidenceSnapshot(ForgeRepo{Owner: "example-org", Name: "project"}, 77, file, "evidence")
			if tc.want {
				if err != nil || s == nil || s.issue.Number != 77 {
					t.Fatalf("complete snapshot rejected: %v", err)
				}
				if (s.file != nil) != (tc.name == "existing file") {
					t.Fatalf("wrong destination presence: %+v", s.file)
				}
			} else {
				if err == nil || !IsUnverifiable(err) {
					t.Fatalf("unsafe snapshot accepted: %+v, %v", s, err)
				}
				if tc.hint != "" && !strings.Contains(err.Error(), tc.hint) {
					t.Errorf("safe error classification %q absent: %v", tc.hint, err)
				}
				if strings.Contains(err.Error(), "untrusted-message-marker") {
					t.Errorf("forge response message disclosed: %v", err)
				}
			}
			if calls.Load() != 1 {
				t.Fatalf("snapshot retried: %d calls", calls.Load())
			}
		})
	}
}
