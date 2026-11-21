package deskkit

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// regression: #3343
func TestPinnedEvidence(t *testing.T) {
	head := strings.Repeat("a", 40)
	next := strings.Repeat("b", 40)
	for _, mode := range []string{"success", "moved", "missing-receipt", "wrong-parent", "file-cas", "shrink", "gitlab"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "POST" || r.URL.Path != "/graphql" {
					t.Fatalf("unexpected fallback %s %s", r.Method, r.URL.Path)
				}
				var req struct {
					Query     string
					Variables struct {
						Input struct {
							ExpectedHeadOid string
							Branch          struct{ RepositoryNameWithOwner, BranchName string }
							Message         struct{ Headline, Body string }
							FileChanges     struct {
								Additions []struct{ Path, Contents string }
							}
						}
					}
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Fatal(err)
				}
				in := req.Variables.Input
				if !strings.Contains(req.Query, "createCommitOnBranch") || in.ExpectedHeadOid != head || in.Branch.BranchName != "verification" || in.Branch.RepositoryNameWithOwner != "example-org/tracker" {
					t.Fatal("write lost exact parent binding")
				}
				if in.Message.Headline != "Evidence" || in.Message.Body != "\nOn-behalf-of: human:alex" {
					t.Fatal("commit trailer lost")
				}
				if len(in.FileChanges.Additions) != 1 || in.FileChanges.Additions[0].Path != "docs/streams/verify-outcomes.jsonl" {
					t.Fatal("wrong changed files")
				}
				raw, err := base64.StdEncoding.DecodeString(in.FileChanges.Additions[0].Contents)
				if err != nil || string(raw) != "old\nnew\n" {
					t.Fatal("wrong content")
				}
				if mode == "moved" {
					json.NewEncoder(w).Encode(map[string]any{"errors": []any{map[string]any{"type": "UNPROCESSABLE", "message": "must-not-echo"}}})
					return
				}
				if mode == "missing-receipt" {
					json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{}})
					return
				}
				parent := head
				if mode == "wrong-parent" {
					parent = next
				}
				json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"createCommitOnBranch": map[string]any{"commit": map[string]any{"oid": next, "parents": map[string]any{"nodes": []any{map[string]any{"oid": parent}}}}}}})
			}))
			defer srv.Close()
			in := WriteFileInput{File: "docs/streams/verify-outcomes.jsonl", Branch: "verification", Content: []byte("old\nnew\n"), Message: "Evidence\n\nOn-behalf-of: human:alex", ExpectedSHA: "original-blob", ExpectedHead: head, AppendOnly: true}
			prior := &FileContent{Exists: true, Content: []byte("old\n"), SHA: "original-blob", Kind: FileKindRegular}
			if mode == "file-cas" {
				in.ExpectedSHA = "different-blob"
			}
			if mode == "shrink" {
				prior.Content = []byte("a\nb\nc\n")
			}
			g := &GitHubForge{Token: "fixture", BaseURL: srv.URL, Client: srv.Client()}
			var res *WriteFileResult
			var err error
			if mode == "gitlab" {
				gl := &GitLabForge{Token: "fixture", BaseURL: srv.URL, Client: srv.Client()}
				res, err = gl.WriteFile(ForgeRepo{Owner: "example-org", Name: "tracker"}, in)
			} else {
				res, err = g.writeEvidenceFileFromSnapshot(ForgeRepo{Owner: "example-org", Name: "tracker"}, in, prior)
			}
			if mode == "success" {
				if err != nil || res == nil || string(res.CommitSHA) != next || !res.Changed {
					t.Fatalf("receipt=%+v err=%v", res, err)
				}
			} else if err == nil {
				t.Fatal("unsafe conditional write accepted")
			}
			if mode == "gitlab" || mode == "file-cas" || mode == "shrink" {
				if calls != 0 {
					t.Fatal("refused path issued write")
				}
			} else if calls != 1 {
				t.Fatalf("calls=%d, want one nonreplayed mutation", calls)
			}
		})
	}
}

// A removal of a superseded outcome record is the same pinned createCommitOnBranch commit
// as a pinned write, carrying only a deletion; any removal that is not pinned and
// conditional is refused before a request is made.
func TestPinnedEvidenceDelete(t *testing.T) {
	head := strings.Repeat("a", 40)
	next := strings.Repeat("b", 40)
	const file = "docs/streams/verify-outcomes/example-stream/14-20260907T020000Z-000000000000.json"
	for _, mode := range []string{"success", "no-head", "no-sha", "content", "absent", "gitlab"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "POST" || r.URL.Path != "/graphql" {
					t.Fatalf("unexpected fallback %s %s", r.Method, r.URL.Path)
				}
				var req struct {
					Query     string
					Variables struct {
						Input struct {
							ExpectedHeadOid string
							FileChanges     struct {
								Additions []struct{ Path string }
								Deletions []struct{ Path string }
							}
						}
					}
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Fatal(err)
				}
				in := req.Variables.Input
				if !strings.Contains(req.Query, "createCommitOnBranch") || in.ExpectedHeadOid != head {
					t.Fatal("removal lost exact parent binding")
				}
				if len(in.FileChanges.Additions) != 0 || len(in.FileChanges.Deletions) != 1 || in.FileChanges.Deletions[0].Path != file {
					t.Fatalf("wrong changed files: %+v", in.FileChanges)
				}
				json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"createCommitOnBranch": map[string]any{"commit": map[string]any{"oid": next, "parents": map[string]any{"nodes": []any{map[string]any{"oid": head}}}}}}})
			}))
			defer srv.Close()
			in := WriteFileInput{File: file, Branch: "verification", Delete: true, Message: "Evidence: remove", ExpectedSHA: "original-blob", ExpectedHead: head}
			prior := &FileContent{Exists: true, Content: []byte("{}\n"), SHA: "original-blob", Kind: FileKindRegular}
			switch mode {
			case "no-head":
				in.ExpectedHead = ""
			case "no-sha":
				in.ExpectedSHA = ""
			case "content":
				in.Content = []byte("x")
			case "absent":
				prior = nil
			}
			repo := ForgeRepo{Owner: "example-org", Name: "tracker"}
			var res *WriteFileResult
			var err error
			if mode == "gitlab" {
				gl := &GitLabForge{Token: "fixture", BaseURL: srv.URL, Client: srv.Client()}
				res, err = gl.WriteFile(repo, in)
			} else {
				g := &GitHubForge{Token: "fixture", BaseURL: srv.URL, Client: srv.Client()}
				res, err = g.writeEvidenceFileFromSnapshot(repo, in, prior)
			}
			if mode == "success" {
				if err != nil || res == nil || string(res.CommitSHA) != next || !res.Changed || calls != 1 {
					t.Fatalf("receipt=%+v err=%v calls=%d", res, err, calls)
				}
				return
			}
			if err == nil {
				t.Fatal("unpinned or unconditional removal accepted")
			}
			if calls != 0 {
				t.Fatal("refused removal issued a request")
			}
		})
	}
}
