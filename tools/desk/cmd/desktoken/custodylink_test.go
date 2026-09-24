package main

// custodylink_test.go — desktoken's own custody checks must not follow a symlink planted at a
// custody path (follow-up to #1573).
//
//   - The GitHub token cache (<role>-token-<install>) is written by desktoken itself and has no
//     symlink layout. A link there used to be stat'd THROUGH: a fresh-looking target was handed
//     out as the role's cached token, and a stale or dangling one made the mint write the new
//     installation token through the link to wherever it pointed.
//   - The GitLab custody file keeps its documented same-directory link
//     (gitlab_custody_layout_test.go); a link that leaves the custody directory is refused.

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// seedReviewerMint sets up a reviewer App (id + PEM) and an install server for example-org,
// returning the token-cache path desktoken would reuse or write, and the recorded
// access_tokens POSTs.
func seedReviewerMint(t *testing.T, homeDir string) (tokenPath string, posts *[]string) {
	t.Helper()
	t.Setenv("REVIEWER_APP_ID", "12345")
	writeFileMode(t, filepath.Join(homeDir, ".config", "assay", "reviewer-app.pem"), makePEM(t), 0o600)

	const installID = "100000004"
	installs := []installationInfo{
		{ID: 100000004, Account: struct {
			Login string `json:"login"`
		}{Login: "example-org"}},
	}
	srv, recorded := makeInstallTokenServer(t, installs, "ghs_minted_through_link", "2124-01-01T01:00:00Z")
	t.Cleanup(srv.Close)
	old := httpClient
	httpClient = &http.Client{Transport: &rewriteTransport{orig: srv.URL}}
	t.Cleanup(func() { httpClient = old })
	return filepath.Join(homeDir, ".config", "assay", "reviewer-token-"+installID), recorded
}

func plantLink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable on this platform: %v", err)
	}
}

// TestCacheRefusesSymlinkedTokenCache — a link at the token-cache path, pointing at a fresh
// 0600 file, is refused rather than handed out as the role's cached installation token.
func TestCacheRefusesSymlinkedTokenCache(t *testing.T) {
	homeDir := setupTest(t)
	tokenPath, posts := seedReviewerMint(t, homeDir)

	real := filepath.Join(filepath.Dir(tokenPath), "some-other-0600-file")
	writeTokenCache(t, real, "not_an_installation_token")
	fresh := time.Now().Add(-5 * time.Minute)
	_ = os.Chtimes(real, fresh, fresh)
	plantLink(t, real, tokenPath)

	rc, stdout, stderr := runCap(t, []string{"reviewer"})
	if rc == deskkit.ExitOK {
		t.Fatalf("desktoken reused a SYMLINKED token cache (stdout %q) — it must refuse", stdout)
	}
	if !strings.Contains(stderr, "symlink") {
		t.Errorf("refusal must name the symlink; got: %s", stderr)
	}
	if len(*posts) != 0 {
		t.Errorf("a refused custody path must not mint; access_tokens hit %v", *posts)
	}
}

// TestMintRefusesToWriteThroughDanglingCacheLink — a DANGLING link at the token-cache path is
// refused before any mint, so the new token is never written through it to the link target.
func TestMintRefusesToWriteThroughDanglingCacheLink(t *testing.T) {
	homeDir := setupTest(t)
	tokenPath, posts := seedReviewerMint(t, homeDir)

	target := filepath.Join(t.TempDir(), "written-through-link")
	plantLink(t, target, tokenPath)

	rc, _, stderr := runCap(t, []string{"reviewer"})
	if _, err := os.Stat(target); err == nil {
		t.Fatalf("desktoken wrote the minted token THROUGH a dangling cache link to %s", target)
	}
	if rc == deskkit.ExitOK {
		t.Fatalf("desktoken accepted a dangling symlink at the token-cache path — it must refuse")
	}
	if !strings.Contains(stderr, "symlink") {
		t.Errorf("refusal must name the symlink; got: %s", stderr)
	}
	if len(*posts) != 0 {
		t.Errorf("a refused custody path must not mint; access_tokens hit %v", *posts)
	}
}

// TestGitLabNoRotateRefusesOutOfDirCustodyLink — a gitlab-<role>.token link resolving OUT of
// the custody directory is refused by the read-only lookup (and so by the rotation, which
// runs the same checks first).
func TestGitLabNoRotateRefusesOutOfDirCustodyLink(t *testing.T) {
	homeDir := setupTest(t)
	custody := gitlabTokenPath(homeDir, "worker")
	if err := os.MkdirAll(filepath.Dir(custody), 0o700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "planted-0600")
	writeTokenCache(t, outside, glOldWorker)
	plantLink(t, outside, custody)
	t.Setenv("GITLAB_API_BASE", "")

	rc, stdout, stderr := runCap(t, []string{"--forge", "gitlab", "--no-rotate", "worker"})
	if rc == deskkit.ExitOK {
		t.Fatalf("--no-rotate accepted a custody link out of its directory (stdout %q) — it must refuse", stdout)
	}
	if !strings.Contains(stderr, "symlink") {
		t.Errorf("refusal must name the symlink; got: %s", stderr)
	}
	assertNoTokenLeak(t, stdout+stderr)
}
