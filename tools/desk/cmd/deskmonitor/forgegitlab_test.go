package main

// forgegitlab_test.go — the pr poller's seed read on a repo the roster maps to GitLab.
//
// pr-monitor.sh hands a legacy cycle to `deskmonitor pr` because this verb resolves each repo's
// forge through deskkit.ForgeFor. This pins the property that handoff relies on: a repo mapped to
// gitlab is read from the GitLab backend, under the role's GitLab custody credential, and seeds a
// baseline — the GitHub mint is never consulted for it.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestPRSeedReadsGitLabRoster pins an existing property (green at base too): it is the contract
// the #5889 handoff stands on, not the fail-first test (that is pr-monitor.test.sh's).
func TestPRSeedReadsGitLabRoster(t *testing.T) {
	const repo = "example-org/gl-pilot"
	const pat = "example-placeholder-reviewer-0000" // low-entropy placeholder, no vendor prefix

	home := t.TempDir()
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	roster := "ASSAY_BLESS_LOGIN=ada:2001\n" +
		"ASSAY_TRUSTED_LOGINS=ada:2001\n" +
		"ASSAY_ALLOWED_REPOS=" + repo + ":ci:private\n" +
		"ASSAY_REPO_FORGES=" + repo + "=gitlab\n"
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(roster), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gitlab-reviewer.token"), []byte(pat+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv(deskkit.EnvConfigHome, dir)
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)

	var mu sync.Mutex
	var seen []string // "path token"
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		path := r.URL.EscapedPath()
		mu.Lock()
		seen = append(seen, path+" "+r.Header.Get("PRIVATE-TOKEN"))
		mu.Unlock()
		rw.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(path, "/merge_requests"):
			_ = json.NewEncoder(rw).Encode([]map[string]any{{
				"id": 501, "iid": 7, "title": "a change", "state": "opened", "draft": false,
				"sha": "abc123", "source_branch": "feat", "target_branch": "main",
			}})
		case strings.HasSuffix(path, "/pipelines"):
			_, _ = rw.Write([]byte("[]"))
		default:
			rw.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("GITLAB_API_BASE", srv.URL)

	oldRole, oldMint := sessionRoleFn, mintTokenFn
	sessionRoleFn = func(string) (string, string, error) { return "reviewer", "pr-review-desk", nil }
	mintTokenFn = func(string, string) (string, string, error) {
		t.Errorf("the GitHub App mint was consulted for a repo the roster maps to gitlab")
		return "", "", os.ErrPermission
	}
	t.Cleanup(func() { sessionRoleFn, mintTokenFn = oldRole, oldMint })

	state := filepath.Join(t.TempDir(), "state")
	t.Setenv("PR_MONITOR_STATE_DIR", state)
	t.Setenv("ASSAY_MONITOR_PACE_SECONDS", "0")

	var out, errb bytes.Buffer
	if code := run([]string{"pr", repo}, &out, &errb); code != 0 {
		t.Fatalf("pr seed on a gitlab-mapped repo: exit %d\nstdout: %s\nstderr: %s", code, out.String(), errb.String())
	}
	if !strings.Contains(out.String(), "MONITOR-ARMED") || strings.Contains(out.String(), "MONITOR-DEGRADED") {
		t.Fatalf("want a clean arm, got:\n%s", out.String())
	}
	mu.Lock()
	reads := append([]string(nil), seen...)
	mu.Unlock()
	mrRead := false
	for _, s := range reads {
		if strings.Contains(s, "/merge_requests ") {
			mrRead = true
			if !strings.HasSuffix(s, " "+pat) {
				t.Fatalf("the merge-request read did not carry the role's GitLab credential: %q", s)
			}
		}
	}
	if !mrRead {
		t.Fatalf("the seed never read the GitLab merge-request list; requests: %q", reads)
	}
	b, err := os.ReadFile(filepath.Join(state, strings.ReplaceAll(repo, "/", "__")+".state"))
	if err != nil {
		t.Fatalf("no baseline seeded for the gitlab-mapped repo: %v", err)
	}
	if !strings.Contains(string(b), "abc123") {
		t.Fatalf("baseline does not carry the change's head: %q", b)
	}
}
