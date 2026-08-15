package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// --- helpers -------------------------------------------------------------------

// writeFile creates a file with the given content, creating parent dirs.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// writeFileMode creates a file with the given content and mode.
func writeFileMode(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// writeTokenCache creates a token cache file with 0o600 permissions.
func writeTokenCache(t *testing.T, path, content string) {
	t.Helper()
	writeFileMode(t, path, content, 0o600)
}

// makePEM creates a minimal-but-valid RSA private key PEM for testing.
func makePEM(t *testing.T) string {
	t.Helper()
	// Generate a throwaway RSA key.
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	pemData := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	return string(pemData)
}

// makeTokenServer returns an httptest.Server that serves the GitHub
// access_tokens endpoint. It records POST bodies (JWTs) and request paths
// for assertion.
func makeTokenServer(t *testing.T, token, expiresAt string) (*httptest.Server, *[]string, *[]string) {
	t.Helper()
	var recorded []string
	var recordedPaths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "method not allowed", 405)
			return
		}
		body, _ := io.ReadAll(r.Body)
		recorded = append(recorded, string(body))
		recordedPaths = append(recordedPaths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		resp := map[string]string{"token": token, "expires_at": expiresAt}
		json.NewEncoder(w).Encode(resp)
	}))
	return srv, &recorded, &recordedPaths
}

// makeInstallTokenServer returns an httptest.Server that handles both
// GET /app/installations (returns the given installs list) and
// POST /app/installations/*/access_tokens (returns a token).
// recordedPaths captures the path of each access_tokens request so tests
// can assert the install ID reached the correct endpoint.
func makeInstallTokenServer(t *testing.T, installs []installationInfo, token, expiresAt string) (*httptest.Server, *[]string) {
	t.Helper()
	var recordedPaths []string
	mux := http.NewServeMux()
	mux.HandleFunc("GET /app/installations", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(installs)
	})
	mux.HandleFunc("POST /app/installations/", func(w http.ResponseWriter, r *http.Request) {
		recordedPaths = append(recordedPaths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]string{"token": token, "expires_at": expiresAt})
	})
	srv := httptest.NewServer(mux)
	return srv, &recordedPaths
}

// setupTest points HOME to a temp dir, disables the kill switch, sets env
// vars for the deskkit runtime, and returns the home dir path.
//
// It also CLEARS ASSAY_CONFIG_HOME. The knob prepends a directory to the
// App-credential search path, so a developer who exports it in their own shell
// would otherwise have every test here probe their real config home — the run
// would pass or fail on a directory the test never created, which is the same
// class of "resolved somewhere unnamed" bug #794 is about.
func setupTest(t *testing.T) string {
	t.Helper()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("DESK_TOOLS_DISABLED", "")
	t.Setenv("CLAUDE_SESSION_ID", "test-session")
	t.Setenv(deskkit.EnvConfigHome, "")
	return homeDir
}

// runCap captures the exit code, stdout, and stderr of a run invocation.
func runCap(t *testing.T, args []string) (int, string, string) {
	t.Helper()
	old := os.Stdout
	stdoutR, stdoutW, serr := os.Pipe()
	if serr != nil {
		t.Fatalf("stdout pipe: %v", serr)
	}
	os.Stdout = stdoutW

	stderrR, stderrW, terr := os.Pipe()
	if terr != nil {
		t.Fatalf("stderr pipe: %v", terr)
	}
	oldStderr := os.Stderr
	os.Stderr = stderrW

	rc := run(args)

	_ = stdoutW.Close()
	_ = stderrW.Close()
	os.Stdout = old
	os.Stderr = oldStderr

	stdout, _ := io.ReadAll(stdoutR)
	stderr, _ := io.ReadAll(stderrR)
	return rc, string(stdout), string(stderr)
}

// auditEntries reads the audit log from deskkit's test directory. Since we
// set HOME to a temp dir, the audit is at <home>/.config/assay/audit.jsonl.
func auditEntries(t *testing.T) []deskkit.Entry {
	t.Helper()
	entries, err := deskkit.LoadEntries()
	if err != nil {
		return nil // empty
	}
	return entries
}

// --- kill switch ----------------------------------------------------------------

func TestKillSwitchDisabled(t *testing.T) {
	homeDir := setupTest(t)
	t.Setenv("DESK_TOOLS_DISABLED", "1")
	// Set the env so cmdToken gets past the APP_ID check.
	t.Setenv("REVIEWER_APP_ID", "12345")
	// Create the PEM so the file stat succeeds if it were to reach there.
	writeFileMode(t, filepath.Join(homeDir, ".config", "assay", "reviewer-app.pem"), makePEM(t), 0o600)

	rc, _, stderr := runCap(t, []string{"reviewer", "--repo", "example-org/tracker"})
	if rc != deskkit.ExitDisabled {
		t.Fatalf("desktoken under kill switch rc = %d, want 3; stderr: %s", rc, stderr)
	}
	if !strings.Contains(stderr, "disabled") {
		t.Fatalf("stderr should mention disabled; got: %s", stderr)
	}
}

// --- key/0600 violations --------------------------------------------------------

func TestKeyMissingUnverifiable(t *testing.T) {
	_ = setupTest(t)
	t.Setenv("REVIEWER_APP_ID", "12345")
	// Deliberately NOT creating the PEM file.

	rc, _, stderr := runCap(t, []string{"reviewer"})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("missing key rc = %d, want 6; stderr: %s", rc, stderr)
	}
	if !strings.Contains(stderr, "private key not found") {
		t.Fatalf("expected 'private key not found'; got: %s", stderr)
	}
}

func TestKeyWrongPermsUnverifiable(t *testing.T) {
	homeDir := setupTest(t)
	t.Setenv("REVIEWER_APP_ID", "12345")
	// Create PEM with 0644 (world-readable) — violates the 0600 rule.
	writeFileMode(t, filepath.Join(homeDir, ".config", "assay", "reviewer-app.pem"), makePEM(t), 0o644)

	// Make a fake HTTP server so the exchange would succeed if we somehow got
	// past the perm check (we shouldn't).
	srv, _, _ := makeTokenServer(t, "ghs_test_fake", "2124-01-01T00:00:00Z")
	defer srv.Close()
	oldClient := httpClient
	httpClient = &http.Client{Transport: &http.Transport{}}
	httpClient.Transport = &http.Transport{}
	httpClient = &http.Client{
		Transport: &rewriteTransport{orig: srv.URL},
	}
	defer func() { httpClient = oldClient }()

	rc, _, stderr := runCap(t, []string{"reviewer"})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("wrong perms key rc = %d, want 6; stderr: %s", rc, stderr)
	}
	if !strings.Contains(stderr, "0600") {
		t.Fatalf("expected 0600 error; got: %s", stderr)
	}
}

// --- cache behaviour ------------------------------------------------------------

func TestCacheReuseFresh(t *testing.T) {
	homeDir := setupTest(t)
	t.Setenv("REVIEWER_APP_ID", "12345")

	// Create the PEM key.
	writeFileMode(t, filepath.Join(homeDir, ".config", "assay", "reviewer-app.pem"), makePEM(t), 0o600)

	// Set up install server: GET /app/installations resolves before cache check.
	const installID = "100000004"
	installs := []installationInfo{
		{ID: 100000004, Account: struct {
			Login string `json:"login"`
		}{Login: "example-org"}},
	}

	// Pre-seed a fresh token cache (< 50 min old) at the per-install path.
	tokenPath := filepath.Join(homeDir, ".config", "assay", "reviewer-token-"+installID)
	writeTokenCache(t, tokenPath, "ghs_cached_token")
	// Set mtime to 5 minutes ago (fresh).
	mtime := time.Now().Add(-5 * time.Minute)
	os.Chtimes(tokenPath, mtime, mtime)

	// The access_tokens handler should NOT be reached because we reuse the cache.
	srv, recordedPaths := makeInstallTokenServer(t, installs, "ghs_should_not_be_called", "2124-01-01T00:00:00Z")
	defer srv.Close()
	oldClient := httpClient
	httpClient = &http.Client{
		Transport: &rewriteTransport{orig: srv.URL},
	}
	defer func() { httpClient = oldClient }()

	rc, stdout, stderr := runCap(t, []string{"reviewer"})
	if rc != deskkit.ExitOK {
		t.Fatalf("cache reuse rc = %d, want 0; stderr: %s", rc, stderr)
	}
	// Output should be the path to the cached token.
	if !strings.Contains(stdout, tokenPath) {
		t.Fatalf("stdout should contain token path %s; got: %s", tokenPath, stdout)
	}
	// The cached token value must not leak to stdout.
	if strings.Contains(stdout, "ghs_cached_token") {
		t.Fatalf("token value leaked to stdout: %s", stdout)
	}
	// The access_tokens POST should NOT have been called.
	if len(*recordedPaths) > 0 {
		t.Fatalf("access_tokens POST should not have been called; got %v", *recordedPaths)
	}

	// Audit should mention cache reuse, not a mint.
	entries := auditEntries(t)
	if len(entries) == 0 {
		t.Fatal("expected at least one audit entry")
	}
	last := entries[len(entries)-1]
	if !strings.Contains(last.Detail, "reused cached") {
		t.Fatalf("expected cache reuse in audit; got: %s", last.Detail)
	}
}

func TestCacheExpiredMintsNew(t *testing.T) {
	homeDir := setupTest(t)
	t.Setenv("REVIEWER_APP_ID", "12345")

	// Create the PEM key.
	writeFileMode(t, filepath.Join(homeDir, ".config", "assay", "reviewer-app.pem"), makePEM(t), 0o600)

	// Set up install server: GET /app/installations is called first.
	const installID = "100000004"
	installs := []installationInfo{
		{ID: 100000004, Account: struct {
			Login string `json:"login"`
		}{Login: "example-org"}},
	}

	// Pre-seed a STALE token cache (> 50 min old).
	tokenPath := filepath.Join(homeDir, ".config", "assay", "reviewer-token-"+installID)
	writeTokenCache(t, tokenPath, "ghs_stale_token")
	mtime := time.Now().Add(-55 * time.Minute) // 55 min old > 50 min threshold
	os.Chtimes(tokenPath, mtime, mtime)

	// Set up a fake server that returns a fresh token.
	srv, recordedPaths := makeInstallTokenServer(t, installs, "ghs_fresh_minted_token_789", "2124-01-01T01:00:00Z")
	defer srv.Close()
	oldClient := httpClient
	httpClient = &http.Client{
		Transport: &rewriteTransport{orig: srv.URL},
	}
	defer func() { httpClient = oldClient }()

	rc, stdout, stderr := runCap(t, []string{"reviewer"})
	if rc != deskkit.ExitOK {
		t.Fatalf("expired refresh rc = %d, want 0; stderr: %s", rc, stderr)
	}
	if !strings.Contains(stdout, tokenPath) {
		t.Fatalf("stdout should contain token path %s; got: %s", tokenPath, stdout)
	}
	// The new token value must not leak.
	if strings.Contains(stdout, "ghs_fresh_minted_token_789") {
		t.Fatalf("new token value leaked to stdout: %s", stdout)
	}
	// The access_tokens POST should target the correct install.
	if len(*recordedPaths) == 0 {
		t.Fatal("expected at least one access_tokens request")
	}
	if !strings.Contains((*recordedPaths)[0], installID) {
		t.Fatalf("access_tokens path should contain %s; got: %s", installID, (*recordedPaths)[0])
	}

	// The cached file should now contain the new token.
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		t.Fatalf("read token cache: %v", err)
	}
	if string(data) != "ghs_fresh_minted_token_789" {
		t.Fatalf("cached token = %q, want %q", string(data), "ghs_fresh_minted_token_789")
	}

	// Audit should mention "minted new".
	entries := auditEntries(t)
	if len(entries) == 0 {
		t.Fatal("expected at least one audit entry")
	}
	last := entries[len(entries)-1]
	if !strings.Contains(last.Detail, "minted new") {
		t.Fatalf("expected 'minted new' in audit; got: %s", last.Detail)
	}
}

// --- install auto-pick (mint-path tests replace the prior cache-based ones) -------

// --- INSTALL_ID override --------------------------------------------------------

func TestInstallIDOverride(t *testing.T) {
	homeDir := setupTest(t)
	t.Setenv("REVIEWER_APP_ID", "12345")
	t.Setenv("REVIEWER_INSTALL_ID", "99999999")
	writeFileMode(t, filepath.Join(homeDir, ".config", "assay", "reviewer-app.pem"), makePEM(t), 0o600)

	// With override install 99999999, the cache path should be suffix -99999999.
	tokenPath := filepath.Join(homeDir, ".config", "assay", "reviewer-token-99999999")
	writeTokenCache(t, tokenPath, "ghs_override_token")
	mtime := time.Now().Add(-5 * time.Minute)
	os.Chtimes(tokenPath, mtime, mtime)

	srv, _, _ := makeTokenServer(t, "ghs_fake", "2124-01-01T00:00:00Z")
	defer srv.Close()
	httpClient = &http.Client{
		Transport: &rewriteTransport{orig: srv.URL},
	}

	// --repo is ignored when INSTALL_ID is set, but let's pass a example-org repo.
	rc, stdout, _ := runCap(t, []string{"reviewer", "--repo", "example-org/tracker"})
	if rc != deskkit.ExitOK {
		t.Fatalf("install override rc = %d, want 0", rc)
	}
	if !strings.Contains(stdout, "reviewer-token-99999999") {
		t.Fatalf("expected override token path; got: %s", stdout)
	}
}

// --- never-print-token (the load-bearing test) -----------------------------------

func TestTokenNeverPrintedToOutput(t *testing.T) {
	homeDir := setupTest(t)
	t.Setenv("REVIEWER_APP_ID", "12345")
	writeFileMode(t, filepath.Join(homeDir, ".config", "assay", "reviewer-app.pem"), makePEM(t), 0o600)

	// A distinctive token value that MUST never appear in output.
	secretToken := "ghs_secret_test_token_never_leak_xyz_2026"
	expiresAt := "2126-07-12T12:00:00Z"

	installs := []installationInfo{
		{ID: 100000004, Account: struct {
			Login string `json:"login"`
		}{Login: "example-org"}},
	}
	srv, _ := makeInstallTokenServer(t, installs, secretToken, expiresAt)
	defer srv.Close()
	oldClient := httpClient
	httpClient = &http.Client{
		Transport: &rewriteTransport{orig: srv.URL},
	}
	defer func() { httpClient = oldClient }()

	rc, stdout, stderr := runCap(t, []string{"reviewer"})
	if rc != deskkit.ExitOK {
		t.Fatalf("mint rc = %d, want 0; stderr: %s", rc, stderr)
	}

	// THE LOAD-BEARING ASSERTION: the token value must NOT appear in stdout
	// or stderr.
	if strings.Contains(stdout, secretToken) {
		t.Fatalf("FATAL: token value leaked to stdout: %s", stdout)
	}
	if strings.Contains(stderr, secretToken) {
		t.Fatalf("FATAL: token value leaked to stderr: %s", stderr)
	}

	// The token value must also NOT appear in the audit log.
	entries := auditEntries(t)
	for _, e := range entries {
		if strings.Contains(e.Detail, secretToken) {
			t.Fatalf("FATAL: token value leaked to audit log detail: %+v", e)
		}
	}

	// The token value SHOULD be in the cache file (now at per-install path).
	tokenPath := filepath.Join(homeDir, ".config", "assay", "reviewer-token-100000004")
	data, _ := os.ReadFile(tokenPath)
	if string(data) != secretToken {
		t.Fatalf("cached token = %q, want %q", string(data), secretToken)
	}
}

// --- per-role key resolution -----------------------------------------------------

func TestPerRoleKeyResolutionVerifier(t *testing.T) {
	homeDir := setupTest(t)
	t.Setenv("VERIFIER_APP_ID", "67890")
	writeFileMode(t, filepath.Join(homeDir, ".config", "assay", "verifier-app.pem"), makePEM(t), 0o600)
	t.Setenv("VERIFIER_INSTALL_ID", "55555555")

	tokenPath := filepath.Join(homeDir, ".config", "assay", "verifier-token-55555555")
	writeTokenCache(t, tokenPath, "ghs_verifier_token")
	mtime := time.Now().Add(-5 * time.Minute)
	os.Chtimes(tokenPath, mtime, mtime)

	srv, _, _ := makeTokenServer(t, "ghs_fake", "2124-01-01T00:00:00Z")
	defer srv.Close()
	httpClient = &http.Client{
		Transport: &rewriteTransport{orig: srv.URL},
	}

	rc, stdout, _ := runCap(t, []string{"verifier"})
	if rc != deskkit.ExitOK {
		t.Fatalf("verifier role rc = %d, want 0", rc)
	}
	if !strings.Contains(stdout, "verifier-token-55555555") {
		t.Fatalf("expected verifier token path; got: %s", stdout)
	}
}

func TestPerRoleKeyResolutionWorker(t *testing.T) {
	homeDir := setupTest(t)
	t.Setenv("WORKER_APP_ID", "11111")
	writeFileMode(t, filepath.Join(homeDir, ".config", "assay", "worker-app.pem"), makePEM(t), 0o600)

	// No cache pre-seeded — forces the mint path through install resolution.
	// The default owner is "example-org" when no --repo is given.
	installs := []installationInfo{
		{ID: 999888777, Account: struct {
			Login string `json:"login"`
		}{Login: "example-org"}},
	}
	srv, recordedPaths := makeInstallTokenServer(t, installs, "ghs_worker_minted", "2124-01-01T00:00:00Z")
	defer srv.Close()
	oldClient := httpClient
	httpClient = &http.Client{Transport: &rewriteTransport{orig: srv.URL}}
	defer func() { httpClient = oldClient }()

	rc, stdout, _ := runCap(t, []string{"worker"})
	if rc != deskkit.ExitOK {
		t.Fatalf("worker role rc = %d, want 0", rc)
	}
	tokenPath := filepath.Join(homeDir, ".config", "assay", "worker-token-999888777")
	if !strings.Contains(stdout, tokenPath) {
		t.Fatalf("expected worker token path %s; got: %s", tokenPath, stdout)
	}

	// Verify the access_tokens request targeted install 999888777 (the worker
	// App's example-org installation, NOT the reviewer App's 100000004).
	if len(*recordedPaths) == 0 {
		t.Fatal("expected at least one access_tokens request")
	}
	if !strings.Contains((*recordedPaths)[0], "999888777") {
		t.Fatalf("access_tokens path should contain 999888777; got: %s", (*recordedPaths)[0])
	}

	// Token value must not leak.
	if strings.Contains(stdout, "ghs_worker_minted") {
		t.Fatalf("token value leaked to stdout: %s", stdout)
	}
}

func TestPerRoleKeyResolutionDesk(t *testing.T) {
	homeDir := setupTest(t)
	t.Setenv("DESK_APP_ID", "22222")
	writeFileMode(t, filepath.Join(homeDir, ".config", "assay", "desk-app.pem"), makePEM(t), 0o600)

	// No cache pre-seeded — forces the mint path with install resolution.
	installs := []installationInfo{
		{ID: 777666555, Account: struct {
			Login string `json:"login"`
		}{Login: "example-org"}},
	}
	srv, recordedPaths := makeInstallTokenServer(t, installs, "ghs_desk_minted", "2124-01-01T00:00:00Z")
	defer srv.Close()
	oldClient := httpClient
	httpClient = &http.Client{Transport: &rewriteTransport{orig: srv.URL}}
	defer func() { httpClient = oldClient }()

	rc, stdout, _ := runCap(t, []string{"desk"})
	if rc != deskkit.ExitOK {
		t.Fatalf("desk role rc = %d, want 0", rc)
	}
	tokenPath := filepath.Join(homeDir, ".config", "assay", "desk-token-777666555")
	if !strings.Contains(stdout, tokenPath) {
		t.Fatalf("expected desk token path %s; got: %s", tokenPath, stdout)
	}
	if len(*recordedPaths) == 0 {
		t.Fatal("expected at least one access_tokens request")
	}
	if !strings.Contains((*recordedPaths)[0], "777666555") {
		t.Fatalf("access_tokens path should contain 777666555; got: %s", (*recordedPaths)[0])
	}
	if strings.Contains(stdout, "ghs_desk_minted") {
		t.Fatalf("token value leaked to stdout: %s", stdout)
	}
}

func TestPerRoleKeyResolutionIssueLoop(t *testing.T) {
	homeDir := setupTest(t)
	t.Setenv("ISSUE_LOOP_APP_ID", "33333")
	writeFileMode(t, filepath.Join(homeDir, ".config", "assay", "issue-loop-app.pem"), makePEM(t), 0o600)

	// No cache pre-seeded — forces the mint path with install resolution.
	installs := []installationInfo{
		{ID: 444333222, Account: struct {
			Login string `json:"login"`
		}{Login: "example-org"}},
	}
	srv, recordedPaths := makeInstallTokenServer(t, installs, "ghs_issue_minted", "2124-01-01T00:00:00Z")
	defer srv.Close()
	oldClient := httpClient
	httpClient = &http.Client{Transport: &rewriteTransport{orig: srv.URL}}
	defer func() { httpClient = oldClient }()

	rc, stdout, _ := runCap(t, []string{"issue-loop"})
	if rc != deskkit.ExitOK {
		t.Fatalf("issue-loop role rc = %d, want 0", rc)
	}
	tokenPath := filepath.Join(homeDir, ".config", "assay", "issue-loop-token-444333222")
	if !strings.Contains(stdout, tokenPath) {
		t.Fatalf("expected issue-loop token path %s; got: %s", tokenPath, stdout)
	}
	if len(*recordedPaths) == 0 {
		t.Fatal("expected at least one access_tokens request")
	}
	if !strings.Contains((*recordedPaths)[0], "444333222") {
		t.Fatalf("access_tokens path should contain 444333222; got: %s", (*recordedPaths)[0])
	}
	if strings.Contains(stdout, "ghs_issue_minted") {
		t.Fatalf("token value leaked to stdout: %s", stdout)
	}
}

// --- unknown role ----------------------------------------------------------------

func TestUnknownRoleRefused(t *testing.T) {
	setupTest(t)
	rc, _, stderr := runCap(t, []string{"unknown-role"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("unknown role rc = %d, want 5; stderr: %s", rc, stderr)
	}
}

// --- missing APP_ID --------------------------------------------------------------

func TestMissingAppIDUnverifiable(t *testing.T) {
	setupTest(t)
	// Neither the env var nor an apps.env file provides the App ID (HOME is an empty
	// temp dir), so the loader must fail unverifiable and name both fixes.
	t.Setenv("REVIEWER_APP_ID", "")
	rc, _, stderr := runCap(t, []string{"reviewer"})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("missing APP_ID rc = %d, want 6; stderr: %s", rc, stderr)
	}
	if !strings.Contains(stderr, "REVIEWER_APP_ID") {
		t.Fatalf("expected the error to name REVIEWER_APP_ID; got: %s", stderr)
	}
}

// --- version/help ----------------------------------------------------------------

func TestVersionOK(t *testing.T) {
	if rc := run([]string{"--version"}); rc != deskkit.ExitOK {
		t.Fatalf("--version rc = %d, want 0", rc)
	}
}

func TestHelpOK(t *testing.T) {
	if rc := run([]string{"--help"}); rc != deskkit.ExitOK {
		t.Fatalf("--help rc = %d, want 0", rc)
	}
}

func TestNoArgsRefused(t *testing.T) {
	if rc := run(nil); rc != deskkit.ExitRefused {
		t.Fatalf("no args rc = %d, want 5", rc)
	}
}

// --- --ttl flag ------------------------------------------------------------------

func TestTTLNoCacheUnverifiable(t *testing.T) {
	setupTest(t)
	// Ensure REVIEWER_APP_ID is set so we get past the basic env check.
	t.Setenv("REVIEWER_APP_ID", "12345")
	// Bypass install resolution with INSTALL_ID so we can test --ttl directly.
	t.Setenv("REVIEWER_INSTALL_ID", "100000004")
	rc, _, stderr := runCap(t, []string{"reviewer", "--ttl"})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("--ttl with no cache rc = %d, want 6; stderr: %s", rc, stderr)
	}
	if !strings.Contains(stderr, "no cached token") {
		t.Fatalf("expected 'no cached token'; got: %s", stderr)
	}
}

func TestTTLWithCache(t *testing.T) {
	homeDir := setupTest(t)
	t.Setenv("REVIEWER_APP_ID", "12345")
	writeFileMode(t, filepath.Join(homeDir, ".config", "assay", "reviewer-app.pem"), makePEM(t), 0o600)

	// Set up install resolution for the default example-org owner.
	installs := []installationInfo{
		{ID: 100000004, Account: struct {
			Login string `json:"login"`
		}{Login: "example-org"}},
	}
	srv, _ := makeInstallTokenServer(t, installs, "ghs_unused", "2124-01-01T00:00:00Z")
	defer srv.Close()
	oldClient := httpClient
	httpClient = &http.Client{Transport: &rewriteTransport{orig: srv.URL}}
	defer func() { httpClient = oldClient }()

	// Pre-seed cache at the per-install path.
	tokenPath := filepath.Join(homeDir, ".config", "assay", "reviewer-token-100000004")
	writeTokenCache(t, tokenPath, "ghs_cached")
	mtime := time.Now().Add(-5 * time.Minute)
	os.Chtimes(tokenPath, mtime, mtime)

	rc, stdout, _ := runCap(t, []string{"reviewer", "--ttl"})
	if rc != deskkit.ExitOK {
		t.Fatalf("--ttl with cache rc = %d, want 0", rc)
	}
	if !strings.Contains(stdout, "TTL=") {
		t.Fatalf("expected TTL= in stdout; got: %s", stdout)
	}
	// Token value must not leak in --ttl mode either.
	if strings.Contains(stdout, "ghs_cached") {
		t.Fatalf("token value leaked in --ttl output: %s", stdout)
	}
}

// --- mint-path: exercise the HTTP exchange (Blocker 4) ---------------------------

func TestMintPathExampleOrg(t *testing.T) {
	homeDir := setupTest(t)
	t.Setenv("REVIEWER_APP_ID", "12345")
	writeFileMode(t, filepath.Join(homeDir, ".config", "assay", "reviewer-app.pem"), makePEM(t), 0o600)

	// No cache pre-seeded — forces the mint path.
	installs := []installationInfo{
		{ID: 100000004, Account: struct {
			Login string `json:"login"`
		}{Login: "example-org"}},
	}
	srv, recordedPaths := makeInstallTokenServer(t, installs, "ghs_minted_reviewer", "2124-01-01T00:00:00Z")
	defer srv.Close()
	oldClient := httpClient
	httpClient = &http.Client{Transport: &rewriteTransport{orig: srv.URL}}
	defer func() { httpClient = oldClient }()

	rc, stdout, _ := runCap(t, []string{"reviewer", "--repo", "example-org/tracker"})
	if rc != deskkit.ExitOK {
		t.Fatalf("mint example-org rc = %d, want 0", rc)
	}
	tokenPath := filepath.Join(homeDir, ".config", "assay", "reviewer-token")
	if !strings.Contains(stdout, tokenPath) {
		t.Fatalf("stdout should contain token path %s; got: %s", tokenPath, stdout)
	}

	// Verify the access_tokens request targeted install 100000004.
	if len(*recordedPaths) == 0 {
		t.Fatal("expected at least one access_tokens request")
	}
	if !strings.Contains((*recordedPaths)[0], "100000004") {
		t.Fatalf("access_tokens path should contain 100000004; got: %s", (*recordedPaths)[0])
	}

	// Token value must not leak.
	if strings.Contains(stdout, "ghs_minted_reviewer") {
		t.Fatalf("token value leaked to stdout: %s", stdout)
	}
}

func TestMintPathMediciFinance(t *testing.T) {
	homeDir := setupTest(t)
	t.Setenv("REVIEWER_APP_ID", "12345")
	writeFileMode(t, filepath.Join(homeDir, ".config", "assay", "reviewer-app.pem"), makePEM(t), 0o600)

	// No cache pre-seeded — forces the mint path.
	installs := []installationInfo{
		{ID: 100000005, Account: struct {
			Login string `json:"login"`
		}{Login: "medici-finance"}},
	}
	srv, recordedPaths := makeInstallTokenServer(t, installs, "ghs_minted_medici", "2124-01-01T00:00:00Z")
	defer srv.Close()
	oldClient := httpClient
	httpClient = &http.Client{Transport: &rewriteTransport{orig: srv.URL}}
	defer func() { httpClient = oldClient }()

	rc, stdout, _ := runCap(t, []string{"reviewer", "--repo", "medici-finance/assay"})
	if rc != deskkit.ExitOK {
		t.Fatalf("mint medici-finance rc = %d, want 0", rc)
	}
	// medici-finance gets install-specific token path.
	tokenPath := filepath.Join(homeDir, ".config", "assay", "reviewer-token-100000005")
	if !strings.Contains(stdout, tokenPath) {
		t.Fatalf("stdout should contain token path %s; got: %s", tokenPath, stdout)
	}

	// Verify the access_tokens request targeted install 100000005.
	if len(*recordedPaths) == 0 {
		t.Fatal("expected at least one access_tokens request")
	}
	if !strings.Contains((*recordedPaths)[0], "100000005") {
		t.Fatalf("access_tokens path should contain 100000005; got: %s", (*recordedPaths)[0])
	}

	// Token value must not leak.
	if strings.Contains(stdout, "ghs_minted_medici") {
		t.Fatalf("token value leaked to stdout: %s", stdout)
	}
}

func TestMintPathInstallIDOverride(t *testing.T) {
	homeDir := setupTest(t)
	t.Setenv("REVIEWER_APP_ID", "12345")
	t.Setenv("REVIEWER_INSTALL_ID", "99999999")
	writeFileMode(t, filepath.Join(homeDir, ".config", "assay", "reviewer-app.pem"), makePEM(t), 0o600)

	// No cache pre-seeded — forces the mint path.
	// INSTALL_ID override: no GET /app/installations needed; straight to token exchange.
	srv, _, recordedPaths := makeTokenServer(t, "ghs_minted_override", "2124-01-01T00:00:00Z")
	defer srv.Close()
	oldClient := httpClient
	httpClient = &http.Client{Transport: &rewriteTransport{orig: srv.URL}}
	defer func() { httpClient = oldClient }()

	rc, stdout, _ := runCap(t, []string{"reviewer"})
	if rc != deskkit.ExitOK {
		t.Fatalf("mint override rc = %d, want 0", rc)
	}
	tokenPath := filepath.Join(homeDir, ".config", "assay", "reviewer-token-99999999")
	if !strings.Contains(stdout, tokenPath) {
		t.Fatalf("stdout should contain token path %s; got: %s", tokenPath, stdout)
	}

	// Verify the access_tokens request targeted install 99999999.
	if len(*recordedPaths) == 0 {
		t.Fatal("expected at least one access_tokens request")
	}
	if !strings.Contains((*recordedPaths)[0], "99999999") {
		t.Fatalf("access_tokens path should contain 99999999; got: %s", (*recordedPaths)[0])
	}

	// Token value must not leak.
	if strings.Contains(stdout, "ghs_minted_override") {
		t.Fatalf("token value leaked to stdout: %s", stdout)
	}
}

func TestMintPathUnknownOwnerFailsClosed(t *testing.T) {
	homeDir := setupTest(t)
	t.Setenv("REVIEWER_APP_ID", "12345")
	writeFileMode(t, filepath.Join(homeDir, ".config", "assay", "reviewer-app.pem"), makePEM(t), 0o600)

	// No cache pre-seeded; no matching installation in the list.
	installs := []installationInfo{
		{ID: 100000004, Account: struct {
			Login string `json:"login"`
		}{Login: "example-org"}},
	}
	srv, _ := makeInstallTokenServer(t, installs, "ghs_unused", "2124-01-01T00:00:00Z")
	defer srv.Close()
	oldClient := httpClient
	httpClient = &http.Client{Transport: &rewriteTransport{orig: srv.URL}}
	defer func() { httpClient = oldClient }()

	rc, _, stderr := runCap(t, []string{"reviewer", "--repo", "other-org/some-repo"})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("unknown owner rc = %d, want 6; stderr: %s", rc, stderr)
	}
	if !strings.Contains(stderr, "no installation found") {
		t.Fatalf("expected 'no installation found'; got: %s", stderr)
	}
}

// TestMintPathWorkerDefaultOwner exercises the no-`--repo` worker path.
// When --repo is absent, owner defaults to "example-org" and the install ID
// is resolved at runtime. This is the test that catches the regression
// where a hardcoded default install ID (the reviewer App's 100000004)
// would silently mint against the wrong App's installation.
func TestMintPathWorkerDefaultOwner(t *testing.T) {
	homeDir := setupTest(t)
	t.Setenv("WORKER_APP_ID", "11111")
	writeFileMode(t, filepath.Join(homeDir, ".config", "assay", "worker-app.pem"), makePEM(t), 0o600)

	// No cache pre-seeded — forces the full mint path.
	// The worker App's example-org install is NOT the reviewer App's 100000004.
	installs := []installationInfo{
		{ID: 666000111, Account: struct {
			Login string `json:"login"`
		}{Login: "example-org"}},
	}
	srv, recordedPaths := makeInstallTokenServer(t, installs, "ghs_worker_default", "2124-01-01T00:00:00Z")
	defer srv.Close()
	oldClient := httpClient
	httpClient = &http.Client{Transport: &rewriteTransport{orig: srv.URL}}
	defer func() { httpClient = oldClient }()

	rc, stdout, _ := runCap(t, []string{"worker"})
	if rc != deskkit.ExitOK {
		t.Fatalf("worker default owner rc = %d, want 0", rc)
	}
	tokenPath := filepath.Join(homeDir, ".config", "assay", "worker-token-666000111")
	if !strings.Contains(stdout, tokenPath) {
		t.Fatalf("stdout should contain token path %s; got: %s", tokenPath, stdout)
	}

	// THE LOAD-BEARING ASSERTION: the access_tokens POST targeted the worker
	// App's example-org install (666000111), NOT the reviewer App's (100000004).
	if len(*recordedPaths) == 0 {
		t.Fatal("expected at least one access_tokens request")
	}
	if !strings.Contains((*recordedPaths)[0], "666000111") {
		t.Fatalf("access_tokens path should contain 666000111; got: %s", (*recordedPaths)[0])
	}
	if strings.Contains((*recordedPaths)[0], "100000004") {
		t.Fatalf("access_tokens path should NOT contain the reviewer App's install 100000004; got: %s", (*recordedPaths)[0])
	}

	// Token value must not leak.
	if strings.Contains(stdout, "ghs_worker_default") {
		t.Fatalf("token value leaked to stdout: %s", stdout)
	}
}

// --- App-credential search path (#794) -------------------------------------------

// TestColdShellMintFromProvisionedConfigHome is the #794 regression, end to end and in
// the exact shape that was reported: the App key and apps.env are provisioned outside
// the shipped default, reached by putting that directory at the head of the search path
// (ASSAY_CONFIG_HOME) — the mechanism a deployment whose provisioning writes App material
// somewhere other than the default must use. ~/.config/assay holds only roster.env,
// NOTHING is exported, and no cached token exists (a cold shell more than 50 minutes
// into a session).
//
// Before the fix there was no search path at all: the tools always read
// ~/.config/assay/worker-app.pem and returned exit 6 "private key not found" whenever
// provisioning had written the key elsewhere. In a live session the failure did not even
// surface immediately: the desk kept running on a still-warm cache and it appeared an
// hour later as a push "Authentication failed" against an empty token file, which is why
// this asserts the COLD path (no cache seeded at all) rather than the convenient one.
func TestColdShellMintFromProvisionedConfigHome(t *testing.T) {
	homeDir := setupTest(t)
	// Nothing exported: the App ID must come off disk too, or the test proves only
	// half the resolution.
	t.Setenv("WORKER_APP_ID", "")
	t.Setenv("WORKER_INSTALL_ID", "")
	t.Setenv("WORKER_PEM", "")
	t.Setenv("WORKER_TOKEN", "")

	const installID = "666000111"
	provisionedDir := filepath.Join(homeDir, "vault", "provisioned")
	t.Setenv(deskkit.EnvConfigHome, provisionedDir)
	writeFileMode(t, filepath.Join(provisionedDir, "worker-app.pem"), makePEM(t), 0o600)
	writeFileMode(t, filepath.Join(provisionedDir, "apps.env"),
		"# desk App family\nexport WORKER_APP_ID=555555\n", 0o600)
	// ~/.config/assay exists with roster.env ONLY — the reported state, and a reminder
	// that ASSAY_CONFIG_HOME deliberately does not move roster.env. The shipped default
	// stays on the search path; it just has no App material to win with.
	writeFileMode(t, filepath.Join(homeDir, ".config", "assay", "roster.env"),
		"ASSAY_BLESS_LOGIN=someone:1\n", 0o600)

	installs := []installationInfo{
		{ID: 666000111, Account: struct {
			Login string `json:"login"`
		}{Login: "example-org"}},
	}
	srv, recordedPaths := makeInstallTokenServer(t, installs, "fake-token-cold-shell", "2124-01-01T00:00:00Z")
	defer srv.Close()
	oldClient := httpClient
	httpClient = &http.Client{Transport: &rewriteTransport{orig: srv.URL}}
	defer func() { httpClient = oldClient }()

	rc, stdout, stderr := runCap(t, []string{"worker", "--repo", "example-org/tracker"})
	if rc != deskkit.ExitOK {
		t.Fatalf("cold-shell mint rc = %d, want 0; stderr: %s", rc, stderr)
	}
	if len(*recordedPaths) == 0 {
		t.Fatal("no access_tokens request — the mint never happened")
	}

	// The cache lands in the SAME directory the key was read from. A token written to
	// ~/.config/assay while the key lives at the head of the search path is the split
	// this fixes: it would "work" for 50 minutes and then fail exactly as #794 describes.
	wantToken := filepath.Join(provisionedDir, "worker-token-"+installID)
	if !strings.Contains(stdout, wantToken) {
		t.Fatalf("stdout should name the token cache %s; got: %s", wantToken, stdout)
	}
	if _, err := os.Stat(wantToken); err != nil {
		t.Fatalf("token cache not written to the config home: %v", err)
	}
	strayToken := filepath.Join(homeDir, ".config", "assay", "worker-token-"+installID)
	if _, err := os.Stat(strayToken); err == nil {
		t.Fatalf("token cached at %s — key dir and cache dir must be the SAME directory", strayToken)
	}
	if strings.Contains(stdout, "fake-token-cold-shell") {
		t.Fatalf("token value leaked to stdout: %s", stdout)
	}
}

// TestConfigHomeHeadWinsOverDefault — the knob PREPENDS, and the head of the search path
// is what a provisioned directory wins with even when the shipped default is also fully
// provisioned. This is the escape hatch that keeps the fix from being another hardcoded
// guess; without the ordering assertion, a deployment could read its key from one
// directory and still cache the token in the other.
func TestConfigHomeHeadWinsOverDefault(t *testing.T) {
	homeDir := setupTest(t)
	t.Setenv("WORKER_APP_ID", "")
	t.Setenv("WORKER_INSTALL_ID", "")
	t.Setenv("WORKER_PEM", "")
	t.Setenv("WORKER_TOKEN", "")

	pinned := filepath.Join(homeDir, "vault", "app-keys")
	t.Setenv(deskkit.EnvConfigHome, pinned)
	writeFileMode(t, filepath.Join(pinned, "worker-app.pem"), makePEM(t), 0o600)
	writeFileMode(t, filepath.Join(pinned, "apps.env"), "export WORKER_APP_ID=555555\n", 0o600)
	// A fully provisioned shipped default that must LOSE to the head of the search path.
	writeFileMode(t, filepath.Join(homeDir, ".config", "assay", "worker-app.pem"), makePEM(t), 0o600)
	writeFileMode(t, filepath.Join(homeDir, ".config", "assay", "apps.env"),
		"export WORKER_APP_ID=999999\n", 0o600)

	const installID = "666000222"
	installs := []installationInfo{
		{ID: 666000222, Account: struct {
			Login string `json:"login"`
		}{Login: "example-org"}},
	}
	srv, _ := makeInstallTokenServer(t, installs, "fake-token-pinned", "2124-01-01T00:00:00Z")
	defer srv.Close()
	oldClient := httpClient
	httpClient = &http.Client{Transport: &rewriteTransport{orig: srv.URL}}
	defer func() { httpClient = oldClient }()

	rc, stdout, stderr := runCap(t, []string{"worker", "--repo", "example-org/tracker"})
	if rc != deskkit.ExitOK {
		t.Fatalf("head-of-search-path mint rc = %d, want 0; stderr: %s", rc, stderr)
	}
	wantToken := filepath.Join(pinned, "worker-token-"+installID)
	if !strings.Contains(stdout, wantToken) {
		t.Fatalf("stdout should name %s; got: %s", wantToken, stdout)
	}
	strayToken := filepath.Join(homeDir, ".config", "assay", "worker-token-"+installID)
	if _, err := os.Stat(strayToken); err == nil {
		t.Fatalf("token cached at %s — the default must not receive the write when the knob is set", strayToken)
	}
}

// TestKeyMissingNamesTheKnobs — when the key really is nowhere, the message has to move
// the reader toward the SEARCH LOCATIONS, not just repeat one path. The bare
// "private key not found at <path>" that #794 produced sent a session hunting for a
// missing file that was never missing. This is the positive control for the refusal: an
// unprovisioned config home, proven to fail closed at exit 6 with every searched
// directory and both knobs named.
func TestKeyMissingNamesTheKnobs(t *testing.T) {
	homeDir := setupTest(t)
	t.Setenv("REVIEWER_APP_ID", "12345")
	t.Setenv("REVIEWER_PEM", "")
	// No PEM anywhere; the head of the search path exists but holds nothing.
	empty := filepath.Join(homeDir, "vault", "empty")
	if err := os.MkdirAll(empty, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Setenv(deskkit.EnvConfigHome, empty)

	rc, _, stderr := runCap(t, []string{"reviewer"})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("missing key rc = %d, want 6; stderr: %s", rc, stderr)
	}
	for _, want := range []string{
		"private key not found", deskkit.EnvConfigHome, "REVIEWER_PEM",
		empty, filepath.Join(homeDir, ".config", "assay"),
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr should mention %q; got: %s", want, stderr)
		}
	}
}

// --- --fresh flag (permission-change re-mint) -----------------------------------

// TestFreshDeletesCacheAndPermsThenMints is the #571 remediation: after a GitHub-App
// permission change the cached token still carries the OLD grant for the rest of the ~50-min
// reuse window, and its .perms sidecar (which `deskroster preflight` reads for the app-scopes
// check) is only rewritten on a FRESH mint. --fresh must therefore delete BOTH the cached
// token and its sidecar before minting, so a fresh token — and a fresh grant record — is
// produced even when the cache is well within the reuse window.
func TestFreshDeletesCacheAndPermsThenMints(t *testing.T) {
	homeDir := setupTest(t)
	t.Setenv("REVIEWER_APP_ID", "12345")
	t.Setenv("REVIEWER_INSTALL_ID", "100000004")
	writeFileMode(t, filepath.Join(homeDir, ".config", "assay", "reviewer-app.pem"), makePEM(t), 0o600)

	// A FRESH cache (5 min old) that a plain mint would reuse, plus a stale .perms sidecar
	// carrying the OLD (pre-change) grant.
	tokenPath := filepath.Join(homeDir, ".config", "assay", "reviewer-token-100000004")
	writeTokenCache(t, tokenPath, "ghs_stale_cached_token")
	permsPath := tokenPath + ".perms"
	writeFileMode(t, permsPath, `{"contents":"read"}`, 0o600)
	mtime := time.Now().Add(-5 * time.Minute)
	_ = os.Chtimes(tokenPath, mtime, mtime)
	_ = os.Chtimes(permsPath, mtime, mtime)

	// The mint server returns a fresh token with the NEW grant (contents:write).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":       "ghs_fresh_after_perm_change",
			"expires_at":  "2124-01-01T00:00:00Z",
			"permissions": map[string]string{"contents": "write", "pull_requests": "write", "issues": "write"},
		})
	}))
	defer srv.Close()
	oldClient := httpClient
	httpClient = &http.Client{Transport: &rewriteTransport{orig: srv.URL}}
	defer func() { httpClient = oldClient }()

	rc, stdout, stderr := runCap(t, []string{"reviewer", "--fresh"})
	if rc != deskkit.ExitOK {
		t.Fatalf("--fresh rc = %d, want 0; stderr: %s", rc, stderr)
	}
	// The cache now holds the freshly minted token, not the stale one.
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		t.Fatalf("read token cache: %v", err)
	}
	if string(data) != "ghs_fresh_after_perm_change" {
		t.Fatalf("cache = %q, want the freshly minted token — --fresh did not force a mint", string(data))
	}
	// The .perms sidecar was rewritten with the NEW grant (contents now write).
	perms, err := os.ReadFile(permsPath)
	if err != nil {
		t.Fatalf("read perms sidecar: %v", err)
	}
	if !strings.Contains(string(perms), `"contents":"write"`) {
		t.Fatalf("perms sidecar = %q, want the new grant (contents:write) — sidecar not refreshed", string(perms))
	}
	// The token value never leaks.
	if strings.Contains(stdout, "ghs_fresh_after_perm_change") {
		t.Fatalf("token value leaked to stdout: %s", stdout)
	}
}

// TestFreshWithNoCacheStillMints — --fresh is idempotent: a missing cache/sidecar is not an
// error, the mint proceeds normally.
func TestFreshWithNoCacheStillMints(t *testing.T) {
	homeDir := setupTest(t)
	t.Setenv("REVIEWER_APP_ID", "12345")
	t.Setenv("REVIEWER_INSTALL_ID", "100000004")
	writeFileMode(t, filepath.Join(homeDir, ".config", "assay", "reviewer-app.pem"), makePEM(t), 0o600)

	srv, _, _ := makeTokenServer(t, "ghs_minted_fresh_nocache", "2124-01-01T00:00:00Z")
	defer srv.Close()
	oldClient := httpClient
	httpClient = &http.Client{Transport: &rewriteTransport{orig: srv.URL}}
	defer func() { httpClient = oldClient }()

	rc, stdout, stderr := runCap(t, []string{"reviewer", "--fresh"})
	if rc != deskkit.ExitOK {
		t.Fatalf("--fresh with no cache rc = %d, want 0; stderr: %s", rc, stderr)
	}
	tokenPath := filepath.Join(homeDir, ".config", "assay", "reviewer-token-100000004")
	if !strings.Contains(stdout, tokenPath) {
		t.Fatalf("stdout should name the token path %s; got: %s", tokenPath, stdout)
	}
}

// --- transport rewrite helper ---------------------------------------------------

// rewriteTransport rewrites the request URL to point at the test server.
type rewriteTransport struct {
	orig string
}

func (rt *rewriteTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	// Keep the path but point at the test server.
	u := *r.URL
	u.Scheme = "http"
	u.Host = strings.TrimPrefix(rt.orig, "http://")
	r2 := r.Clone(r.Context())
	r2.URL = &u
	return http.DefaultTransport.RoundTrip(r2)
}
