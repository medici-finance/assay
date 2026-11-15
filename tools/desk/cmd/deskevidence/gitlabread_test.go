package main

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// gitlabread_test.go — #6163: on a GitLab project every file read goes through the real
// GitLab adapter, whose Repository Files API response names no entry type. The landing must
// judge a served blob a plain file from the response and the tree entry, and still refuse an
// entry the adapter cannot type, a symbolic link and a submodule.

// glReadForge is the fake forge with its file reads served by the real GitLab adapter.
type glReadForge struct {
	*fakeForge
	gl *deskkit.GitLabForge
}

func (g glReadForge) ReadFile(r deskkit.ForgeRepo, in deskkit.ReadFileInput) (*deskkit.FileContent, error) {
	return g.gl.ReadFile(r, in)
}

// glFiles serves f.files the way GitLab's Repository Files and Repository Tree APIs do: no
// type and no mode on a file read, the octal mode on a tree entry. noBlobID drops blob_id
// from every file read (an entry the adapter cannot type); modes overrides a path's mode.
type glFiles struct {
	f        *fakeForge
	noBlobID bool
	modes    map[string]string
}

func glBlobID(content string) string {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(content))
	h.Write([]byte(content))
	return hex.EncodeToString(h.Sum(nil))
}

func (s *glFiles) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.f.mu.Lock()
	files := map[string]string{}
	for k, v := range s.f.files {
		files[k] = v
	}
	s.f.mu.Unlock()
	esc := r.URL.EscapedPath()
	w.Header().Set("Content-Type", "application/json")
	if i := strings.Index(esc, "/repository/files/"); r.Method == http.MethodGet && i >= 0 {
		p, _ := url.PathUnescape(esc[i+len("/repository/files/"):])
		c, ok := files[p]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"message": "404 File Not Found"})
			return
		}
		body := map[string]any{
			"file_name": path.Base(p), "file_path": p, "size": len(c), "encoding": "base64",
			"content": base64.StdEncoding.EncodeToString([]byte(c)), "ref": r.URL.Query().Get("ref"),
			"commit_id": "commit-0", "last_commit_id": "sha-" + p,
			"execute_filemode": s.modes[p] == "100755",
		}
		if !s.noBlobID {
			body["blob_id"] = glBlobID(c)
		}
		_ = json.NewEncoder(w).Encode(body)
		return
	}
	if r.Method == http.MethodGet && strings.HasSuffix(esc, "/repository/tree") {
		dir := r.URL.Query().Get("path")
		var out []map[string]any
		for p, c := range files {
			if d := path.Dir(p); d == dir || (d == "." && dir == "") {
				mode := "100644"
				if m, ok := s.modes[p]; ok {
					mode = m
				}
				out = append(out, map[string]any{"id": glBlobID(c), "name": path.Base(p), "type": "blob", "path": p, "mode": mode})
			}
		}
		if out == nil {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"message": "404 Tree Not Found"})
			return
		}
		_ = json.NewEncoder(w).Encode(out)
		return
	}
	w.WriteHeader(http.StatusNotFound)
}

// gitlabGate is unprovenGate (the real judge and the real branch-board read) on a GitLab
// project whose file reads go through the GitLab adapter.
func gitlabGate(t *testing.T, s *glFiles) (realGate, *fakeForge) {
	t.Helper()
	g, f := unprovenGate(t)
	s.f = f
	forgeKindFn = func(string, string) (deskkit.ForgeKind, error) { return deskkit.ForgeGitLab, nil }
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	gl := &deskkit.GitLabForge{Token: "fixture-token", BaseURL: srv.URL}
	forgeForFn = func(owner, name string) (deskkit.Forge, deskkit.ForgeRepo, error) {
		return deskkit.OutboundChecked(glReadForge{fakeForge: f, gl: gl}, "verifier"), deskkit.ForgeRepo{Owner: owner, Name: name}, nil
	}
	f.setFile(unprovenBrief, briefOf("human", ""))
	f.setFile(g.readme, g.board("todo"))
	return g, f
}

// regression: #6163
func TestGitLabBriefLands(t *testing.T) {
	fragment := filepath.Join(t.TempDir(), "row.md")
	if err := os.WriteFile(fragment, []byte("| 1 | pass | the row ran |\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	args := func(g realGate, extra ...string) []string {
		return append([]string{"example-org/tracker", "main", "--brief-path", unprovenBrief, "--evidence-file", fragment, "--root", g.root}, extra...)
	}
	t.Run("dry-run", func(t *testing.T) {
		g, f := gitlabGate(t, &glFiles{})
		if code := run(args(g, "--dry-run")); code != deskkit.ExitOK {
			t.Fatalf("GitLab --dry-run exit = %d, want %d: a served blob with a regular tree entry is a plain file", code, deskkit.ExitOK)
		}
		if f.putCalls != 0 {
			t.Fatalf("--dry-run wrote %d time(s)", f.putCalls)
		}
	})
	t.Run("landing", func(t *testing.T) {
		g, f := gitlabGate(t, &glFiles{})
		if code := run(args(g)); code != deskkit.ExitOK {
			t.Fatalf("GitLab landing exit = %d, want %d", code, deskkit.ExitOK)
		}
		if f.putCalls != 1 || !strings.HasPrefix(f.putContent, briefOf("human", "")) || !strings.Contains(f.putContent, "| 1 | pass | the row ran |") {
			t.Fatalf("landing wrote %d time(s), want the brief with the row appended:\n%s", f.putCalls, f.putContent)
		}
	})
	t.Run("executable brief lands", func(t *testing.T) {
		g, f := gitlabGate(t, &glFiles{modes: map[string]string{unprovenBrief: "100755"}})
		if code := run(args(g, "--dry-run")); code != deskkit.ExitOK {
			t.Fatalf("executable brief exit = %d, want %d: the neutral type has no executable slot", code, deskkit.ExitOK)
		}
		if f.putCalls != 0 {
			t.Fatalf("--dry-run wrote %d time(s)", f.putCalls)
		}
	})
	for _, c := range []struct {
		name string
		srv  *glFiles
		want string
	}{
		{"untyped read refused", &glFiles{noBlobID: true}, "did not type"},
		{"symlink refused", &glFiles{modes: map[string]string{unprovenBrief: "120000"}}, "symbolic link"},
		{"gitlink refused", &glFiles{modes: map[string]string{unprovenBrief: "160000"}}, "a submodule"},
	} {
		t.Run(c.name, func(t *testing.T) {
			g, f := gitlabGate(t, c.srv)
			code := run(args(g, "--dry-run"))
			if code != deskkit.ExitUnverifiable {
				t.Fatalf("exit = %d, want %d", code, deskkit.ExitUnverifiable)
			}
			if msg := stderr.(interface{ String() string }).String(); !strings.Contains(msg, c.want) {
				t.Fatalf("refusal does not name %q:\n%s", c.want, msg)
			}
			if f.putCalls != 0 {
				t.Fatalf("refused landing wrote %d time(s)", f.putCalls)
			}
		})
	}
}
