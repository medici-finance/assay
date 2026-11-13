package main

// kindcollision_test.go — #6264: on GitLab, issues and merge requests are numbered in
// SEPARATE sequences, so a project routinely carries issue #N and merge request !N at once.
// The board's RETIRE read asks about an ISSUE (the placeholder names one), so it must use the
// typed issue read. The bare-number resolver refuses that number as both-kinds, which left
// the work-ready and intake projections could-not-check on every GitLab project where a
// placeholder's issue number had also been reached by the merge-request sequence.
//
// The backend here is the real GitLab forge against a recorded server, so the refusal the
// fix avoids is the resolver's own, not a fake's imitation of it.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// glCollisionServer serves homeRepo as a GitLab project whose every placeholder number is BOTH
// a closed issue and an open merge request. The open-issue listing is empty, so each
// placeholder is absent from it and the board must positively read its issue state.
func glCollisionServer(t *testing.T, closed []int) (*httptest.Server, func() []string) {
	t.Helper()
	isClosed := map[int]bool{}
	for _, n := range closed {
		isClosed[n] = true
	}
	project := "/api/v4/projects/" + strings.ReplaceAll(homeRepo, "/", "%2F")
	var mu sync.Mutex
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		path := r.URL.EscapedPath()
		mu.Lock()
		seen = append(seen, r.Method+" "+path)
		mu.Unlock()
		rw.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet {
			t.Errorf("the board is read-only; got %s %s", r.Method, path)
			rw.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		rest, ok := strings.CutPrefix(path, project+"/")
		if !ok {
			rw.WriteHeader(http.StatusNotFound) // users API and the like: unresolved, never fatal
			return
		}
		if rest == "issues" {
			_, _ = rw.Write([]byte("[]"))
			return
		}
		kind, num, _ := strings.Cut(rest, "/")
		n, err := strconv.Atoi(num)
		if err != nil || !isClosed[n] {
			rw.WriteHeader(http.StatusNotFound)
			return
		}
		switch kind {
		case "issues":
			_ = json.NewEncoder(rw).Encode(map[string]any{"id": 1000 + n, "iid": n,
				"title": fmt.Sprintf("closed-issue-%d", n), "state": "closed"})
		case "merge_requests":
			_ = json.NewEncoder(rw).Encode(map[string]any{"id": 2000 + n, "iid": n,
				"title": fmt.Sprintf("open-change-%d", n), "state": "opened"})
		default:
			rw.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), seen...)
	}
}

// regression: #6264
// Both RETIRE read shapes — one absent placeholder (the single read) and several (the
// positive-state batch, which on GitLab is the serial per-issue read) — under BOTH the
// work-ready and intake projections.
func TestRetireKindCollision(t *testing.T) {
	for _, projection := range []string{"work-ready", "intake"} {
		for _, closed := range [][]int{{9}, {9, 11}} {
			t.Run(fmt.Sprintf("%s/%d", projection, len(closed)), func(t *testing.T) {
				installForge(t, nil)
				srv, seen := glCollisionServer(t, closed)
				gl := deskkit.OutboundChecked(&deskkit.GitLabForge{Token: "fixture-token", BaseURL: srv.URL}, "worker")
				fake := forgeFor
				forgeFor = func(repo string, obs ...deskkit.RequestObserver) (deskkit.Forge, deskkit.ForgeRepo, error) {
					if repo != homeRepo {
						return fake(repo, obs...)
					}
					owner, name, _ := strings.Cut(repo, "/")
					return gl, deskkit.ForgeRepo{Owner: owner, Name: name}, nil
				}
				root := t.TempDir()
				dir := filepath.Join(root, issueLoopDir)
				if err := os.MkdirAll(dir, 0o700); err != nil {
					t.Fatal(err)
				}
				for _, n := range closed {
					p := filepath.Join(dir, fmt.Sprintf("issue-%d.md", n))
					if err := os.WriteFile(p, []byte(placeholderFixture(homeRepo, "todo", "")), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				var out, errb bytes.Buffer
				code := run([]string{"--root", root, "issues", "--projection", projection}, &out, &errb)
				if code != 0 || strings.Contains(errb.String(), "BOTH issue") {
					t.Fatalf("#6264: issue #N and merge request !N both exist; the RETIRE read must ask for the ISSUE "+
						"and classify, not refuse. exit %d\nstdout:\n%s\nstderr:\n%s", code, out.String(), errb.String())
				}
				// intake renders RETIRE rows; work-ready lists only unrepresented work, so there
				// the proof is that each issue was positively read and the sweep classified.
				reqs := strings.Join(seen(), "\n")
				for _, n := range closed {
					if want := fmt.Sprintf("closed-issue-%d", n); projection == "intake" && !strings.Contains(out.String(), want) {
						t.Errorf("RETIRE row for issue %d (title %q) missing:\n%s", n, want, out.String())
					}
					if want := fmt.Sprintf("/issues/%d\n", n); !strings.Contains(reqs+"\n", want) {
						t.Errorf("issue %d was never positively read; requests:\n%s", n, reqs)
					}
				}
				if strings.Contains(reqs, "/merge_requests/") {
					t.Errorf("the issue-state read touched the merge request at the same number:\n%s", reqs)
				}
			})
		}
	}
}
