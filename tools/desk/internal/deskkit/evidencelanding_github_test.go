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

// regression: #5543 — Blob.text is not the blob's bytes: the go-gh transport
// rewrites a literal backslash-u-0003 in any JSON answer into caret notation.
// When text does not prove itself against the blob id, the snapshot reads the
// bytes by that id, and still refuses bytes that cannot be proven.
func TestSnapshotBlobTextRewritten(t *testing.T) {
	plain := "| cmd | `curl -d '{\"input\":\"touch x\"}'` |\n"
	real := "| cmd | `curl -d '{\"input\":\"\\u0003touch x\"}'` |\n"
	sameLen := strings.Replace(real, "\\u0003", "^C^C^C", 1) // a rendering the length check alone cannot tell apart
	oid := blobID(real, false)
	type blobResp struct {
		status int
		body   map[string]any
	}
	okBlob := func() *blobResp {
		return &blobResp{200, map[string]any{"sha": oid, "size": len(real), "encoding": "base64", "content": ghB64(real)}}
	}
	cases := []struct {
		name  string
		body  string // the blob's bytes; "" means real
		text  any
		blob  *blobResp // nil: the blob endpoint must not be called
		want  bool
		calls int32
	}{
		{name: "text proves itself", body: plain, text: plain, want: true, calls: 1},
		{name: "transport rewrites escape", text: real, blob: okBlob(), want: true, calls: 2},
		{name: "rewritten same length", text: sameLen, blob: okBlob(), want: true, calls: 2},
		{name: "text omitted", text: nil, blob: okBlob(), want: true, calls: 2},
		{name: "blob bytes mismatch", text: real, calls: 2, blob: &blobResp{200, map[string]any{"sha": oid, "size": len(sameLen), "encoding": "base64", "content": ghB64(sameLen)}}},
		{name: "blob wrong size", text: real, calls: 2, blob: &blobResp{200, map[string]any{"sha": oid, "size": len(real) + 1, "encoding": "base64", "content": ghB64(real)}}},
		{name: "blob wrong sha", text: real, calls: 2, blob: &blobResp{200, map[string]any{"sha": strings.Repeat("b", 40), "size": len(real), "encoding": "base64", "content": ghB64(real)}}},
		{name: "blob not base64", text: real, calls: 2, blob: &blobResp{200, map[string]any{"sha": oid, "size": len(real), "encoding": "utf-8", "content": real}}},
		{name: "blob unreadable", text: real, calls: 2, blob: &blobResp{404, map[string]any{"message": "untrusted-message-marker"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body, id := real, oid
			if tc.body != "" {
				body, id = tc.body, blobID(tc.body, false)
			}
			repo := snapshotFixture()
			repo["ref"].(map[string]any)["target"].(map[string]any)["file"] = map[string]any{
				"mode": 0100644, "object": map[string]any{"__typename": "Blob", "oid": id, "text": tc.text, "byteSize": len(body), "isBinary": false},
			}
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				switch {
				case r.Method == http.MethodPost && r.URL.Path == "/graphql":
					_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"repository": repo}})
				case r.Method == http.MethodGet && r.URL.Path == "/repos/example-org/project/git/blobs/"+oid && tc.blob != nil:
					w.WriteHeader(tc.blob.status)
					_ = json.NewEncoder(w).Encode(tc.blob.body)
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
					w.WriteHeader(http.StatusTeapot)
				}
			}))
			defer srv.Close()
			g := &GitHubForge{Token: "stub", BaseURL: srv.URL, Client: srv.Client()}
			s, err := g.evidenceSnapshot(ForgeRepo{Owner: "example-org", Name: "project"}, 77, "docs/streams/example/brief.md", "evidence")
			if tc.want {
				if err != nil || s == nil || s.file == nil {
					t.Fatalf("provable blob refused: %v", err)
				}
				if string(s.file.Content) != body || s.file.SHA != id || s.file.Kind != FileKindRegular {
					t.Fatalf("destination is not the blob's own bytes: %q", s.file.Content)
				}
			} else if err == nil || !IsUnverifiable(err) && !IsForgeNotFound(err) {
				t.Fatalf("unprovable blob accepted: %+v, %v", s, err)
			}
			if err != nil && strings.Contains(err.Error(), "untrusted-message-marker") {
				t.Errorf("forge response message disclosed: %v", err)
			}
			if calls.Load() != tc.calls {
				t.Fatalf("requests = %d, want %d", calls.Load(), tc.calls)
			}
		})
	}
}
