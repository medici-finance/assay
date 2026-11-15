package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// custodynotice_test.go — #6068: on GitLab the printed path is the role's custody file and the
// only copy of its credential, and desktoken must say so; on GitHub the printed path is a
// re-mintable cache and the output stays exactly what it was.

// gitlabCustodyPhrases are the parts of the GitLab notice a caller must be able to act on.
var gitlabCustodyPhrases = []string{
	"CUSTODY FILE",
	"ONLY copy",
	"Never delete",
	"desktoken --no-rotate <role> --repo <slug>",
	"docs/adopting-assay-gitlab.md §5",
}

// regression: #6068
func TestGitLabCustodyNotice(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []string
		rotates bool
	}{
		{"rotate", []string{"--forge", "gitlab", "worker"}, true},
		{"no-rotate", []string{"--forge", "gitlab", "--no-rotate", "worker"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			homeDir := setupTest(t)
			tokPath := gitlabTokenPath(homeDir, "worker")
			writeTokenCache(t, tokPath, glOldWorker)

			valid := glOldWorker
			srv, calls := makeRotateServer(t, &valid, glNewWorker, "2124-01-08T00:00:00Z")
			defer srv.Close()
			pointHTTPClientAt(t, srv)

			rc, stdout, stderr := runCap(t, tc.args)
			if rc != deskkit.ExitOK {
				t.Fatalf("rc = %d, want 0; stderr: %s", rc, stderr)
			}
			if got := strings.TrimSpace(stdout); got != tokPath {
				t.Fatalf("stdout must stay exactly the custody path:\n got %q\nwant %q", got, tokPath)
			}
			for _, want := range gitlabCustodyPhrases {
				if !strings.Contains(stderr, want) {
					t.Errorf("GitLab stderr lacks %q; got:\n%s", want, stderr)
				}
			}
			if strings.Contains(stderr, tokenPathNotice) {
				t.Errorf("GitLab stderr carries the GitHub cache notice:\n%s", stderr)
			}
			if gotRot := strings.Contains(stderr, "rotated"); gotRot != tc.rotates {
				t.Errorf("stderr names a rotation = %v, want %v (endpoint calls %d):\n%s",
					gotRot, tc.rotates, *calls, stderr)
			}
			assertNoTokenLeak(t, stdout+stderr)
		})
	}
}

// regression: #6068
func TestGitHubOutputUnchanged(t *testing.T) {
	deskkit.ResetTrace()
	homeDir := setupTest(t)
	t.Setenv("REVIEWER_APP_ID", "12345")
	writeFileMode(t, filepath.Join(homeDir, ".config", "assay", "reviewer-app.pem"), makePEM(t), 0o600)

	installs := []installationInfo{
		{ID: 100000004, Account: struct {
			Login string `json:"login"`
		}{Login: "example-org"}},
	}
	tokenPath := filepath.Join(homeDir, ".config", "assay", "reviewer-token-100000004")
	writeTokenCache(t, tokenPath, cachedTokenValue)
	mtime := time.Now().Add(-5 * time.Minute)
	if err := os.Chtimes(tokenPath, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	srv, _ := makeInstallTokenServer(t, installs, "minted-value-the-cache-path-must-not-reach", "2124-01-01T00:00:00Z")
	defer srv.Close()
	oldClient := httpClient
	httpClient = &http.Client{Transport: &rewriteTransport{orig: srv.URL}}
	defer func() { httpClient = oldClient }()

	rc, stdout, stderr := runCap(t, []string{"reviewer"})
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0; stderr: %s", rc, stderr)
	}
	if got := strings.TrimSpace(stdout); got != tokenPath {
		t.Fatalf("GitHub stdout changed:\n got %q\nwant %q", got, tokenPath)
	}
	if !strings.Contains(stderr, tokenPathNotice) {
		t.Errorf("GitHub stderr lost the cache-path notice:\n%s", stderr)
	}
	for _, unwanted := range gitlabCustodyPhrases {
		if strings.Contains(stderr, unwanted) {
			t.Errorf("GitHub stderr carries the GitLab custody phrase %q:\n%s", unwanted, stderr)
		}
	}
}
