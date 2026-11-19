package deskkit

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testBlobID(content []byte) string {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(content))
	h.Write(content)
	return hex.EncodeToString(h.Sum(nil))
}

// regression: #2026
// TestPairedEvidenceWrite — a write with companions (a witness row and its
// read-set file) is one commit on both backends: GitHub's createCommitOnBranch
// carries every file, parented on the head the destination was re-read at, and
// GitLab's commit carries a create action per companion. A destination that is
// no longer the judged version, or a companion path holding other bytes, is
// refused before any mutation.
func TestPairedEvidenceWrite(t *testing.T) {
	const brief, reads = "docs/streams/x/brief-01.md", "docs/streams/x/reads/abc"
	head := strings.Repeat("a", 40)
	next := strings.Repeat("b", 40)
	old, row, set := []byte("old\n"), []byte("old\nrow\n"), []byte("assay-read-set v1\n")
	repo := ForgeRepo{Owner: "example-org", Name: "tracker"}

	for _, mode := range []string{"pair", "companion-same", "companion-other", "moved", "bad-path"} {
		t.Run("github/"+mode, func(t *testing.T) {
			var mutations int
			var added []string
			files := map[string][]byte{brief: old}
			if mode == "companion-same" {
				files[reads] = set
			}
			if mode == "companion-other" {
				files[reads] = []byte("other\n")
			}
			// atHead is the tree at the resolved head; the branch-name read (the
			// snapshot the write is judged against) still serves the judged bytes.
			atHead := map[string][]byte{}
			for k, v := range files {
				atHead[k] = v
			}
			if mode == "moved" {
				atHead[brief] = []byte("old\nother\n")
			}
			var headReads int
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				switch {
				case r.Method == "GET" && r.URL.Path == "/repos/example-org/tracker/git/ref/heads/main":
					json.NewEncoder(w).Encode(map[string]any{"object": map[string]any{"sha": head, "type": "commit"}})
				case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/repos/example-org/tracker/contents/"):
					tree := files
					switch r.URL.Query().Get("ref") {
					case "main":
					case head:
						tree = atHead
						headReads++
					default:
						t.Fatalf("read %s at %q", r.URL.Path, r.URL.Query().Get("ref"))
					}
					p := strings.TrimPrefix(r.URL.Path, "/repos/example-org/tracker/contents/")
					c, ok := tree[p]
					if !ok {
						w.WriteHeader(http.StatusNotFound)
						return
					}
					json.NewEncoder(w).Encode(map[string]any{"type": "file", "path": p, "encoding": "base64",
						"content": base64.StdEncoding.EncodeToString(c), "sha": testBlobID(c)})
				case r.Method == "POST" && r.URL.Path == "/graphql" && strings.Contains(string(body), "FileEntryMode"):
					var q struct{ Variables struct{ File, Rev string } }
					_ = json.Unmarshal(body, &q)
					tree := files
					if q.Variables.Rev == head {
						tree = atHead
					}
					json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"repository": map[string]any{"object": map[string]any{
						"__typename": "Commit", "file": map[string]any{"mode": gitModeRegular, "oid": testBlobID(tree[q.Variables.File])}}}}})
				case r.Method == "POST" && r.URL.Path == "/graphql" && strings.Contains(string(body), "createCommitOnBranch"):
					mutations++
					var req struct {
						Variables struct {
							Input struct {
								ExpectedHeadOid string
								FileChanges     struct {
									Additions []struct{ Path, Contents string }
								}
							}
						}
					}
					_ = json.Unmarshal(body, &req)
					if req.Variables.Input.ExpectedHeadOid != head {
						t.Fatalf("paired write parented on %q, want the head it read", req.Variables.Input.ExpectedHeadOid)
					}
					for _, a := range req.Variables.Input.FileChanges.Additions {
						raw, _ := base64.StdEncoding.DecodeString(a.Contents)
						added = append(added, a.Path+"="+string(raw))
					}
					json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"createCommitOnBranch": map[string]any{"commit": map[string]any{
						"oid": next, "parents": map[string]any{"nodes": []any{map[string]any{"oid": head}}}}}}})
				default:
					t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
				}
			}))
			defer srv.Close()
			g := &GitHubForge{Token: "fixture", BaseURL: srv.URL, Client: srv.Client()}
			comp := CompanionFile{File: reads, Content: set}
			if mode == "bad-path" {
				comp.File = "docs/streams/x/../../etc/x"
			}
			res, err := g.WriteFile(repo, WriteFileInput{File: brief, Branch: "main", Content: row, Message: "Evidence",
				AppendOnly: true, Companions: []CompanionFile{comp}})
			switch mode {
			case "pair", "companion-same":
				if err != nil || res == nil || string(res.CommitSHA) != next {
					t.Fatalf("res=%+v err=%v", res, err)
				}
				want := []string{brief + "=" + string(row)}
				if mode == "pair" {
					want = append(want, reads+"="+string(set))
				}
				if headReads == 0 || mutations != 1 || strings.Join(added, "|") != strings.Join(want, "|") {
					t.Fatalf("one commit should carry %v; %d mutation(s) carried %v", want, mutations, added)
				}
			default:
				why := map[string]string{
					"companion-other": "already holds other content",
					"moved":           "not the version this write was judged against",
					"bad-path":        "not a distinct, clean repo-relative path",
				}[mode]
				if err == nil || mutations != 0 || !strings.Contains(err.Error(), why) {
					t.Fatalf("unsafe paired write: err=%v mutations=%d, want refusal %q", err, mutations, why)
				}
			}
		})
	}

	for _, mode := range []string{"pair", "companion-other"} {
		t.Run("gitlab/"+mode, func(t *testing.T) {
			var actions []string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				p := r.URL.EscapedPath()
				switch {
				case r.Method == "GET" && strings.HasSuffix(p, "/projects/example-org%2Ftracker"):
					json.NewEncoder(w).Encode(map[string]any{"visibility": "private", "default_branch": "main"})
				case r.Method == "GET" && strings.Contains(p, "/repository/files/"):
					if mode == "companion-other" && strings.Contains(p, "reads") {
						json.NewEncoder(w).Encode(map[string]any{"file_path": reads, "content": base64.StdEncoding.EncodeToString([]byte("other\n")),
							"encoding": "base64", "blob_id": "not-the-blob", "last_commit_id": "c1"})
						return
					}
					w.WriteHeader(http.StatusNotFound)
					json.NewEncoder(w).Encode(map[string]any{"message": "404 File Not Found"})
				case r.Method == "POST" && strings.HasSuffix(p, "/repository/commits"):
					var in struct {
						Actions []struct {
							Action   string `json:"action"`
							FilePath string `json:"file_path"`
						} `json:"actions"`
					}
					_ = json.Unmarshal(body, &in)
					for _, a := range in.Actions {
						actions = append(actions, a.Action+" "+a.FilePath)
					}
					w.WriteHeader(http.StatusCreated)
					json.NewEncoder(w).Encode(map[string]any{"id": "landed-commit"})
				default:
					t.Fatalf("unexpected %s %s", r.Method, p)
				}
			}))
			defer srv.Close()
			gl := &GitLabForge{Token: "fixture", BaseURL: srv.URL, Client: srv.Client()}
			_, err := gl.WriteFile(repo, WriteFileInput{File: brief, Branch: "evidence/x", StartBranch: "main", Content: row,
				Message: "Evidence", Companions: []CompanionFile{{File: reads, Content: set}}})
			if mode == "pair" {
				if err != nil || strings.Join(actions, "|") != "create "+brief+"|create "+reads {
					t.Fatalf("one commit should create both files: err=%v actions=%v", err, actions)
				}
				return
			}
			if err == nil || len(actions) != 0 || !strings.Contains(err.Error(), "other content") {
				t.Fatalf("a companion path holding other bytes landed: err=%v actions=%v", err, actions)
			}
		})
	}
}
