package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Token-SHAPED fixtures (glpat- prefix, 30+ chars, bare [A-Za-z0-9_-]) so leak assertions
// exercise both the prefix pattern and the bare-token pattern the Verify table greps for.
// These are deliberately LOW-ENTROPY, obvious-placeholder values (the `example` marker plus a
// repeated digit): they carry the shape the redaction tests need while the pattern leaksweep's
// own placeholder/entropy filter correctly classifies them as non-secret, so no allowlist entry
// (and no weakening of a dedicated detection rule) is required to keep them out of the sweep.
const (
	glOldWorker = "glpat-example00000000000000000000-old"
	glNewWorker = "glpat-example11111111111111111111-new"
)

// makeRotateServer serves POST .../personal_access_tokens/self/rotate. It authenticates the
// caller by the PRIVATE-TOKEN header against *valid: a request bearing the current valid
// token rotates (returns newToken and sets *valid = newToken, atomically invalidating the
// old one); any other token is rejected 401 — modelling GitLab's own invalidation so a test
// can assert that a captured old token is dead after a mint. calls counts requests reaching
// the rotate endpoint.
func makeRotateServer(t *testing.T, valid *string, newToken, expiresAt string) (*httptest.Server, *int) {
	t.Helper()
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || !strings.HasSuffix(r.URL.Path, "/personal_access_tokens/self/rotate") {
			http.Error(w, "not found", 404)
			return
		}
		calls++
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("PRIVATE-TOKEN") != *valid {
			w.WriteHeader(401)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "401 Unauthorized"})
			return
		}
		*valid = newToken // the old token is now invalid — exactly one credential is live.
		w.WriteHeader(200)
		_ = json.NewEncoder(w).Encode(map[string]any{"token": newToken, "expires_at": expiresAt, "active": true})
	}))
	return srv, &calls
}

// pointHTTPClientAt routes the package httpClient at srv (keeping request paths) and restores
// it on cleanup. It also sets GITLAB_API_BASE to an explicit self-hosted-style base: rotation
// requires the base to be configured (no default target), and rewriteTransport rewrites the
// host to srv, so the value's host is irrelevant — only that it is SET matters to the code path.
func pointHTTPClientAt(t *testing.T, srv *httptest.Server) {
	t.Helper()
	t.Setenv("GITLAB_API_BASE", "https://gitlab.example.com/api/v4")
	old := httpClient
	httpClient = &http.Client{Transport: &rewriteTransport{orig: srv.URL}}
	t.Cleanup(func() { httpClient = old })
}

func gitlabTokenPath(homeDir, role string) string {
	return filepath.Join(homeDir, ".config", "assay", "gitlab-"+role+".token")
}

// --- rotation success -----------------------------------------------------------

func TestGitLabRotateSuccess(t *testing.T) {
	homeDir := setupTest(t)
	tokPath := gitlabTokenPath(homeDir, "worker")
	writeTokenCache(t, tokPath, glOldWorker)

	valid := glOldWorker
	srv, calls := makeRotateServer(t, &valid, glNewWorker, "2124-01-08T00:00:00Z")
	defer srv.Close()
	pointHTTPClientAt(t, srv)

	rc, stdout, stderr := runCap(t, []string{"--forge", "gitlab", "worker"})
	if rc != deskkit.ExitOK {
		t.Fatalf("rotate rc = %d, want 0; stderr: %s", rc, stderr)
	}
	if *calls != 1 {
		t.Fatalf("rotate endpoint hit %d times, want 1", *calls)
	}
	if !strings.Contains(stdout, tokPath) {
		t.Fatalf("stdout should print the token PATH %q; got: %s", tokPath, stdout)
	}
	// Neither the old nor the new token value may reach any output stream.
	assertNoTokenLeak(t, stdout+stderr)
	// The file must now hold the rotated token, 0600.
	got, err := os.ReadFile(tokPath)
	if err != nil {
		t.Fatalf("read token file: %v", err)
	}
	if string(got) != glNewWorker {
		t.Fatalf("token file = %q, want rotated value", string(got))
	}
	fi, _ := os.Stat(tokPath)
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("token file mode = %o, want 0600", fi.Mode().Perm())
	}
	// Audit records a rotation, not a mint.
	entries := auditEntries(t)
	if len(entries) == 0 {
		t.Fatal("expected an audit entry")
	}
	last := entries[len(entries)-1]
	if !strings.Contains(last.Detail, "rotated gitlab worker") {
		t.Fatalf("audit detail = %q, want rotation", last.Detail)
	}
}

// TestRotateInvalidatesOld backs Verify item 3: after a mint, the OLD token is rejected. The
// fixture invalidates the old token on rotation; replaying it (as a captured token would be)
// is refused.
func TestRotateInvalidatesOld(t *testing.T) {
	homeDir := setupTest(t)
	tokPath := gitlabTokenPath(homeDir, "worker")
	writeTokenCache(t, tokPath, glOldWorker)

	valid := glOldWorker
	srv, _ := makeRotateServer(t, &valid, glNewWorker, "2124-01-08T00:00:00Z")
	defer srv.Close()
	pointHTTPClientAt(t, srv)

	// First rotation succeeds and invalidates glOldWorker server-side.
	if rc, _, stderr := runCap(t, []string{"--forge", "gitlab", "worker"}); rc != deskkit.ExitOK {
		t.Fatalf("first rotate rc = %d, want 0; stderr: %s", rc, stderr)
	}

	// Replay the captured OLD token: write it back and rotate again. The server must now
	// reject it — proving the old credential died at the first mint.
	writeTokenCache(t, tokPath, glOldWorker)
	rc, stdout, stderr := runCap(t, []string{"--forge", "gitlab", "worker"})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("replay of invalidated token rc = %d, want 6; stderr: %s", rc, stderr)
	}
	if !strings.Contains(stderr, "401") {
		t.Fatalf("expected a 401 rejection of the old token; stderr: %s", stderr)
	}
	assertNoTokenLeak(t, stdout+stderr)
}

// TestGitLabWriteFailureLockout backs the write-failure-lockout behaviour: rotation succeeded
// but persistence failed, so the message names the recovery path and prints NO token.
func TestGitLabWriteFailureLockout(t *testing.T) {
	homeDir := setupTest(t)
	tokPath := gitlabTokenPath(homeDir, "worker")
	writeTokenCache(t, tokPath, glOldWorker)

	// Make the custody directory non-writable so the atomic temp-file write cannot land,
	// after the file itself is already readable (mode check + read succeed; the write fails).
	dir := filepath.Dir(tokPath)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	valid := glOldWorker
	srv, calls := makeRotateServer(t, &valid, glNewWorker, "2124-01-08T00:00:00Z")
	defer srv.Close()
	pointHTTPClientAt(t, srv)

	rc, stdout, stderr := runCap(t, []string{"--forge", "gitlab", "worker"})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("write-failure rc = %d, want 6; stderr: %s", rc, stderr)
	}
	if *calls != 1 {
		t.Fatalf("rotation should have been attempted exactly once; calls = %d", *calls)
	}
	if !strings.Contains(stderr, "LOCKOUT") {
		t.Fatalf("expected LOCKOUT in message; got: %s", stderr)
	}
	if !strings.Contains(stderr, "NOT printed") {
		t.Fatalf("lockout message must state the token is not printed; got: %s", stderr)
	}
	// The critical property: the new token never reaches output on the lockout path.
	assertNoTokenLeak(t, stdout+stderr)
	// The original file is untouched (atomic write never renamed over it).
	got, err := os.ReadFile(tokPath)
	if err != nil {
		t.Fatalf("read token file: %v", err)
	}
	if string(got) != glOldWorker {
		t.Fatalf("token file was mutated on a failed write: %q", string(got))
	}
}

// --- refusals -------------------------------------------------------------------

func TestGitLabModeRefusal(t *testing.T) {
	homeDir := setupTest(t)
	tokPath := gitlabTokenPath(homeDir, "worker")
	writeFileMode(t, tokPath, glOldWorker, 0o644) // world-readable — violates 0600.

	valid := glOldWorker
	srv, calls := makeRotateServer(t, &valid, glNewWorker, "2124-01-08T00:00:00Z")
	defer srv.Close()
	pointHTTPClientAt(t, srv)

	rc, stdout, stderr := runCap(t, []string{"--forge", "gitlab", "worker"})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("mode refusal rc = %d, want 6; stderr: %s", rc, stderr)
	}
	if !strings.Contains(stderr, "0600") {
		t.Fatalf("expected 0600 remedy; got: %s", stderr)
	}
	if *calls != 0 {
		t.Fatalf("must refuse BEFORE rotating; rotate hit %d times", *calls)
	}
	assertNoTokenLeak(t, stdout+stderr)
}

func TestGitLabMissingFileRefusal(t *testing.T) {
	setupTest(t)
	// No gitlab-worker.token provisioned.
	rc, _, stderr := runCap(t, []string{"--forge", "gitlab", "worker"})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("missing-file rc = %d, want 6; stderr: %s", rc, stderr)
	}
	if !strings.Contains(stderr, "not found") {
		t.Fatalf("expected a not-found remedy naming search dirs; got: %s", stderr)
	}
}

// TestGitLabRotateRefusesWithoutAPIBase backs Blocker 2: with a valid custody file present but
// GITLAB_API_BASE unset, the command must refuse BEFORE any network contact rather than probe a
// default target — the role's live PAT is never transmitted to a guessed SaaS host. No client is
// pointed at a server, so any attempted request would fail the test's isolation regardless; the
// assertion is that the refusal names the missing env var and no token leaks.
func TestGitLabRotateRefusesWithoutAPIBase(t *testing.T) {
	homeDir := setupTest(t)
	tokPath := gitlabTokenPath(homeDir, "worker")
	writeTokenCache(t, tokPath, glOldWorker)

	// Ensure the base is unset for this process even if an ambient value leaked in.
	t.Setenv("GITLAB_API_BASE", "")

	rc, stdout, stderr := runCap(t, []string{"--forge", "gitlab", "worker"})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("unset-base rc = %d, want 6; stderr: %s", rc, stderr)
	}
	if !strings.Contains(stderr, "GITLAB_API_BASE") {
		t.Fatalf("refusal must name GITLAB_API_BASE; got: %s", stderr)
	}
	// The custody file is untouched — nothing rotated.
	got, err := os.ReadFile(tokPath)
	if err != nil {
		t.Fatalf("read token file: %v", err)
	}
	if string(got) != glOldWorker {
		t.Fatalf("token file mutated on a pre-contact refusal: %q", string(got))
	}
	assertNoTokenLeak(t, stdout+stderr)
}

func TestUnknownForgeRefused(t *testing.T) {
	setupTest(t)
	rc, _, stderr := runCap(t, []string{"--forge", "bogus", "worker"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("unknown forge rc = %d, want 5; stderr: %s", rc, stderr)
	}
	if !strings.Contains(stderr, "unknown --forge") {
		t.Fatalf("expected unknown-forge refusal; got: %s", stderr)
	}
}

// assertNoTokenLeak fails if any token-shaped value appears in out — mirrors Verify item 2's
// grep for the glpat- prefix and for a bare 30+ char token-shaped run on its own line.
func assertNoTokenLeak(t *testing.T, out string) {
	t.Helper()
	if strings.Contains(out, "glpat-") {
		t.Fatalf("token-shaped value (glpat- prefix) leaked to output:\n%s", out)
	}
	for _, line := range strings.Split(out, "\n") {
		s := strings.TrimSpace(line)
		if len(s) < 30 {
			continue
		}
		if isBareToken(s) {
			t.Fatalf("bare token-shaped line (30+ chars) leaked to output: %q", s)
		}
	}
}

// isBareToken reports whether s is entirely [A-Za-z0-9_-] — the shape Verify item 2 rejects.
func isBareToken(s string) bool {
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
		default:
			return false
		}
	}
	return true
}
