package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// --- test key (one per test binary, keygen is slow) ---

var (
	testKeyOnce sync.Once
	testKeyPEM  []byte
)

func verifierPEM(t *testing.T) []byte {
	t.Helper()
	testKeyOnce.Do(func() {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			panic(err)
		}
		der := x509.MarshalPKCS1PrivateKey(key)
		testKeyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der})
	})
	return testKeyPEM
}

// verifierInstallID is the verifier App's installation.
//
// The fake below mints ONLY for this installation and 404s every other one, exactly as
// GitHub does when an App JWT is presented against another App's installation. That is
// what makes a wrong-App mint detectable at all — before #228 the fake
// returned the same token for every installation, so no test could tell the two apart.
const verifierInstallID = "100000003"

// reviewerInstallIDs are the two constants deskevidence used to hardcode (identically to
// cmd/deskpost/github.go, the REVIEWER App's tool) under a doc comment claiming they were
// the verifier's. Kept here only as the negative fixture.
var reviewerInstallIDs = []string{"100000001", "100000002"}

// fakeGH is a scriptable stand-in for the GitHub REST API. Every request is
// recorded so tests can assert that a refusal made NO mutating call.
type fakeGH struct {
	srv *httptest.Server

	mu   sync.Mutex
	hits []string // "METHOD path"

	// GET /repos/{owner}/{repo}/contents/{path} returns these.
	fileContent map[string]string // path -> content
	fileSHA     map[string]string // path -> sha

	// --- App identity ---

	// lookupInstallID is what GET /repos/{o}/{r}/installation returns. Zero value means
	// verifierInstallID. Tests set it to a reviewer ID to simulate a mislabelled App.
	lookupInstallID string
	// lookupStatus, when non-zero and non-200, makes the installation lookup FAIL with
	// that status instead of answering. GitHub 404s the endpoint when the App is not
	// installed on the repo and 5xx's when it is having a bad day; both must fail closed,
	// with no fallback installation ID. Zero value = answer normally.
	lookupStatus int
	// lookupHijack makes the lookup fail at the TRANSPORT layer: the connection is
	// hijacked and closed without a response, so httpClient.Do returns an error and the
	// handler code below resp.StatusCode is never reached.
	//
	// This exists because a status code cannot represent every way the lookup fails. The
	// fixture could previously only script an HTTP response, so the two branches that
	// never see one — the Do error and a body that does not yield an id — were
	// unreachable from a test, and a silent fallback in either survived the suite green
	// (#406). The transport branch is the one a 30s client Timeout
	// routes new traffic to, so it is the least theoretical of the four.
	lookupHijack bool
	// lookupBody, when non-empty, is written verbatim as the 200 response body instead of
	// the normal `{"id": ...}`. It reaches the two post-response failure branches: a body
	// that will not parse, and one that parses to an absent/zero id.
	lookupBody string
	// mintedFor records every installation ID an access_tokens mint was attempted for,
	// in order. This is the record that shows WHICH App's installation was used.
	mintedFor []string
	// commitAuthor is the git author name the PUT response reports on the created
	// commit. Zero value means verifierBotDisplay().
	commitAuthor string

	// For tracking PUTs.
	putCalls      int
	putPath       string
	putContent    string // base64-decoded
	putBranch     string
	putSHA        string // SHA sent to the API
	putNewSHA     string // SHA the fake reported back on the created commit
	putTokenCheck bool   // whether the PUT carried the verifier installation's token

	// onPut, when set, runs inside the PUT handler before it responds. Used by the
	// serialisation tests to observe what a concurrent invocation can do while the
	// write window is open.
	onPut func()

	// repo visibility for the public-repo gate.
	repoVisibility string // default "private" if empty
	// repoVisibilityErr makes GET /repos/{owner}/{repo} return 500, so a test can
	// prove the gate fails CLOSED when visibility cannot be established.
	repoVisibilityErr bool
}

// installToken is the token the fake mints for a given installation. It NAMES the
// installation, so a token minted from the wrong App's installation is a different
// string and the PUT assertion can see it.
func installToken(id string) string { return "fake-installation-token-" + id }

func (f *fakeGH) wantInstallID() string {
	if f.lookupInstallID != "" {
		return f.lookupInstallID
	}
	return verifierInstallID
}

func (f *fakeGH) mintAttempts() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.mintedFor...)
}

func (f *fakeGH) setFile(path, content, sha string) {
	if f.fileContent == nil {
		f.fileContent = make(map[string]string)
		f.fileSHA = make(map[string]string)
	}
	f.fileContent[path] = base64.StdEncoding.EncodeToString([]byte(content))
	f.fileSHA[path] = sha
}

var (
	reAccessToken = regexp.MustCompile(`^/app/installations/([^/]+)/access_tokens$`)
	reInstallLook = regexp.MustCompile(`^/repos/[^/]+/[^/]+/installation$`)
	reContentsGet = regexp.MustCompile(`/repos/[^/]+/[^/]+/contents/`)
	reContentsPut = regexp.MustCompile(`/repos/[^/]+/[^/]+/contents/`)
	reRepoOnly    = regexp.MustCompile(`^/repos/[^/]+/[^/]+$`)
)

func (f *fakeGH) handler(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.hits = append(f.hits, r.Method+" "+r.URL.Path)
	f.mu.Unlock()

	writeJSON := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	path := r.URL.Path

	switch {
	case r.Method == http.MethodGet && reInstallLook.MatchString(path):
		// GET /repos/{owner}/{repo}/installation — the App JWT asks GitHub which of ITS
		// installations covers this repo. Authenticated with "Bearer <jwt>", never a
		// token; a token here would be a bug in the tool.
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			writeJSON(map[string]any{"message": "installation lookup requires the App JWT"})
			return
		}
		if f.lookupHijack {
			// Drop the connection with no response at all. The client sees a transport
			// error, not a status. Go's transport does not retry a request that failed on
			// a fresh (never-reused) connection, so this is one error, not a loop.
			hj, ok := w.(http.Hijacker)
			if !ok {
				panic("test server does not support hijacking; the transport-error branch cannot be reached")
			}
			conn, _, err := hj.Hijack()
			if err != nil {
				panic("hijack failed: " + err.Error())
			}
			_ = conn.Close()
			return
		}
		if f.lookupStatus != 0 && f.lookupStatus != http.StatusOK {
			w.WriteHeader(f.lookupStatus)
			writeJSON(map[string]any{"message": "scripted lookup failure"})
			return
		}
		if f.lookupBody != "" {
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, f.lookupBody)
			return
		}
		w.WriteHeader(http.StatusOK)
		writeJSON(map[string]any{"id": json.Number(f.wantInstallID())})

	case r.Method == http.MethodPost && reAccessToken.MatchString(path):
		id := reAccessToken.FindStringSubmatch(path)[1]
		f.mu.Lock()
		f.mintedFor = append(f.mintedFor, id)
		f.mu.Unlock()
		// GitHub 404s an installation the JWT's App does not own. Modelling that is the
		// whole point: it is how a wrong-App mint becomes visible to a test.
		if id != verifierInstallID {
			w.WriteHeader(http.StatusNotFound)
			writeJSON(map[string]any{"message": "Not Found"})
			return
		}
		w.WriteHeader(http.StatusCreated)
		writeJSON(map[string]any{"token": installToken(id), "expires_at": time.Now().Add(time.Hour).Format(time.RFC3339)})

	case r.Method == http.MethodGet && reContentsGet.MatchString(path):
		// Extract the file path from the URL.
		// /repos/{owner}/{repo}/contents/{path}
		parts := strings.SplitN(path, "/contents/", 2)
		if len(parts) != 2 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		filePath := parts[1]
		f.mu.Lock()
		content, ok := f.fileContent[filePath]
		sha, shaOk := f.fileSHA[filePath]
		f.mu.Unlock()

		if !ok || !shaOk {
			w.WriteHeader(http.StatusNotFound)
			writeJSON(map[string]any{"message": "Not Found"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(map[string]string{"sha": sha, "content": content})

	case r.Method == http.MethodPut && reContentsPut.MatchString(path):
		f.mu.Lock()
		f.putCalls++
		f.putPath = path
		hook := f.onPut
		f.mu.Unlock()
		if hook != nil {
			hook()
		}

		body, _ := io.ReadAll(r.Body)
		var in map[string]any
		if err := json.Unmarshal(body, &in); err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}

		f.mu.Lock()
		f.putBranch = fmt.Sprint(in["branch"])
		f.putSHA = fmt.Sprint(in["sha"])
		content := fmt.Sprint(in["content"])
		// Attribution check: the PUT must carry the token minted from the VERIFIER App's
		// installation, not merely "whatever the mint returned". Before #228 the fake
		// returned one token for every installation, so this flag could not fail.
		auth := r.Header.Get("Authorization")
		if strings.Contains(auth, installToken(verifierInstallID)) {
			f.putTokenCheck = true
		}
		// Decode the content for test assertions.
		decoded, _ := base64.StdEncoding.DecodeString(content)
		f.putContent = string(decoded)
		// Simulate a successful commit: return a new SHA.
		newSHA := "commit-" + fmt.Sprintf("%d", f.putCalls) + "-" + fmt.Sprintf("%x", time.Now().UnixNano())
		f.fileContent[path] = content
		f.fileSHA[path] = newSHA
		f.putNewSHA = newSHA
		author := f.commitAuthor
		f.mu.Unlock()
		if author == "" {
			author = verifierBotDisplay()
		}

		w.WriteHeader(http.StatusCreated)
		// The real Contents API returns the commit it created alongside the content.
		// `commit.author` is the git identity on it — for an App-token commit, the App's
		// bot. Shape and values follow the Contents API's documented response:
		//   gh api search/commits -f q='author-name:assay-verifier-app repo:example-org/tracker'
		//   -> commit.author = {name:"assay-verifier-app[bot]",
		//                       email:"100000011+assay-verifier-app[bot]@users.noreply.github.com"}
		commit := map[string]any{"sha": "c0ffee" + newSHA}
		if author != "none" { // "none" = simulate a response carrying no author at all
			commit["author"] = map[string]any{
				"name":  author,
				"email": "100000011+" + author + "@users.noreply.github.com",
				"date":  time.Now().Format(time.RFC3339),
			}
		}
		writeJSON(map[string]any{
			"content": map[string]any{
				"sha":  newSHA,
				"name": filepath.Base(path),
				"path": path,
			},
			"commit": commit,
		})

	// GET /repos/{owner}/{repo} — visibility check (public-repo gate).
	// Matches ONLY the bare repo path (no /contents/... suffix).
	case r.Method == http.MethodGet && reRepoOnly.MatchString(path):
		f.mu.Lock()
		vis, visErr := f.repoVisibility, f.repoVisibilityErr
		f.mu.Unlock()
		if visErr {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if vis == "" {
			vis = "private"
		}
		writeJSON(map[string]string{"visibility": vis})

	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// setupFake wires isolation: a temp HOME so the audit log + kill switch live in a
// throwaway dir, the verifier PEM, the fake API base URL, and captured stdout/stderr.
func setupFake(t *testing.T) (*fakeGH, *bytes.Buffer) {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	plantFixtureRoster(t, home)
	t.Setenv("DESK_TOOLS_DISABLED", "")
	// Neutralise the harness's real session var ($CLAUDE_CODE_SESSION_ID, present in every
	// Claude Code session) so the legacy fixture value below deterministically drives
	// SessionTag(); otherwise the ambient UUID wins precedence.
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	t.Setenv("CLAUDE_SESSION_ID", "deskevidence-test")
	// Most tests below commit to "main" because that is the verify-desk's real target.
	// The main-branch guard refuses that by default, so the sanction is set here for the
	// common case; the guard's own tests clear it explicitly.
	t.Setenv("VERIFIER_MAIN_OK", "1")

	pemPath := filepath.Join(home, ".config", "assay", "verifier-app.pem")
	if err := os.MkdirAll(filepath.Dir(pemPath), 0o755); err != nil {
		t.Fatalf("mkdir pem dir: %v", err)
	}
	if err := os.WriteFile(pemPath, verifierPEM(t), 0o600); err != nil {
		t.Fatalf("write pem: %v", err)
	}
	t.Setenv("VERIFIER_PEM", pemPath)
	t.Setenv("VERIFIER_APP_ID", "999999")
	// Unset so the installation is DISCOVERED (the production path) rather than handed
	// in. Tests that exercise the override set it themselves.
	t.Setenv("VERIFIER_INSTALL_ID", "")
	// installCache is process-global; a test that scripts a different lookup result must
	// not inherit the previous test's answer.
	installCacheMu.Lock()
	installCache = map[string]string{}
	installCacheMu.Unlock()
	t.Cleanup(func() {
		installCacheMu.Lock()
		installCache = map[string]string{}
		installCacheMu.Unlock()
	})

	f := &fakeGH{}
	f.srv = httptest.NewServer(http.HandlerFunc(f.handler))
	t.Cleanup(f.srv.Close)

	oldBase := apiBaseURL
	apiBaseURL = f.srv.URL
	t.Cleanup(func() { apiBaseURL = oldBase })

	oldClient := httpClient
	httpClient = f.srv.Client()
	httpClient = &http.Client{
		Transport: &rewriteTransport{orig: f.srv.URL},
	}
	t.Cleanup(func() { httpClient = oldClient })

	var errBuf bytes.Buffer
	oldOut, oldErr := stdout, stderr
	stdout = &bytes.Buffer{}
	stderr = &errBuf
	t.Cleanup(func() { stdout, stderr = oldOut, oldErr })

	return f, &errBuf
}

// writeRepoFile creates a local file under a temp directory (simulating
// a checked-out repo file). Returns the absolute path.
func writeRepoFile(t *testing.T, path, content string) string {
	t.Helper()
	base := t.TempDir()
	absPath := filepath.Join(base, path)
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", absPath, err)
	}
	return absPath
}

// auditEntries reads the isolated audit log.
func auditEntries(t *testing.T) []deskkit.Entry {
	t.Helper()
	e, err := deskkit.LoadEntries()
	if err != nil {
		t.Fatalf("load audit: %v", err)
	}
	return e
}

func lastAudit(t *testing.T) deskkit.Entry {
	t.Helper()
	e := auditEntries(t)
	if len(e) == 0 {
		t.Fatal("no audit entries")
	}
	return e[len(e)-1]
}

// rewriteTransport rewrites the request URL to point at the test server.
type rewriteTransport struct {
	orig string
}

func (rt *rewriteTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	u := *r.URL
	u.Scheme = "http"
	u.Host = strings.TrimPrefix(rt.orig, "http://")
	r2 := r.Clone(r.Context())
	r2.URL = &u
	return http.DefaultTransport.RoundTrip(r2)
}

// --- Tests ---

// TestVersionOK verifies version output.
func TestVersionOK(t *testing.T) {
	_, _ = setupFake(t)
	code := run([]string{"--version"})
	if code != deskkit.ExitOK {
		t.Fatalf("--version exit = %d, want 0", code)
	}
}

// TestHelpOK verifies help output.
func TestHelpOK(t *testing.T) {
	_, _ = setupFake(t)
	code := run([]string{"--help"})
	if code != deskkit.ExitOK {
		t.Fatalf("--help exit = %d, want 0", code)
	}
}

// TestNoArgsRefused verifies no-args = usage error.
func TestNoArgsRefused(t *testing.T) {
	_, _ = setupFake(t)
	code := run(nil)
	if code != deskkit.ExitRefused {
		t.Fatalf("no args exit = %d, want %d", code, deskkit.ExitRefused)
	}
}

// TestMissingEvidenceFileRefused verifies --evidence-file is required.
func TestMissingEvidenceFileRefused(t *testing.T) {
	_, _ = setupFake(t)
	code := run([]string{"example-org/tracker", "main"})
	if code != deskkit.ExitRefused {
		t.Fatalf("missing --evidence-file exit = %d, want %d", code, deskkit.ExitRefused)
	}
}

// TestKillSwitchExit3 exercises the kill switch: DESK_TOOLS_DISABLED=1 → exit 3,
// one disabled audit line, and NO network call.
func TestKillSwitchExit3(t *testing.T) {
	f, _ := setupFake(t)
	t.Setenv("DESK_TOOLS_DISABLED", "1")

	code := run([]string{"example-org/tracker", "main", "--evidence-file", "/dev/null"})
	if code != deskkit.ExitDisabled {
		t.Fatalf("disabled exit = %d, want %d", code, deskkit.ExitDisabled)
	}
	if len(f.hits) != 0 {
		t.Fatalf("expected NO network hits while disabled, got %v", f.hits)
	}
	if got := lastAudit(t).Result; got != deskkit.ResultDisabled {
		t.Fatalf("last audit result = %q, want disabled", got)
	}
}

// TestSuccessfulCommitEndToEnd exercises the full happy path:
// local file -> BodyCheck -> fetch remote -> commit via API.
func TestSuccessfulCommitEndToEnd(t *testing.T) {
	f, _ := setupFake(t)

	// Write a local evidence file.
	evidencePath := writeRepoFile(t, "docs/brief.md", `# Brief

## Evidence
| 1 | ... | evidence row |
`)

	// Set up the fake to return the current file content with a known SHA.
	// Use the same absolute path as the evidence file.
	f.setFile(evidencePath,
		`# Brief

## Evidence
`,
		"existing-sha-12345")

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath})
	if code != deskkit.ExitOK {
		t.Fatalf("successful commit exit = %d, want 0", code)
	}

	// Verify an audit line was written.
	entries := auditEntries(t)
	if len(entries) < 1 {
		t.Fatal("expected at least one audit entry")
	}
	last := entries[len(entries)-1]
	if last.Result != deskkit.ResultOK {
		t.Fatalf("last audit result = %q, want ok", last.Result)
	}
	if last.BodyDigest == "" {
		t.Fatal("expected non-empty bodyDigest in audit")
	}

	// Verify the PUT happened via the API as the verifier App.
	if f.putCalls != 1 {
		t.Fatalf("expected 1 PUT call, got %d", f.putCalls)
	}
	if f.putBranch != "main" {
		t.Fatalf("PUT branch = %q, want main", f.putBranch)
	}
	if f.putSHA != "existing-sha-12345" {
		t.Fatalf("PUT sha = %q, want existing-sha-12345", f.putSHA)
	}
	if !f.putTokenCheck {
		t.Fatal("PUT did not carry the verifier App token")
	}
}

// TestIdempotencyNoop verifies that committing the same content already on the branch
// is a noop (exit 0, no PUT).
func TestIdempotencyNoop(t *testing.T) {
	f, _ := setupFake(t)

	content := `# Brief

## Evidence
content already on branch
`
	evidencePath := writeRepoFile(t, "docs/brief.md", content)

	// Set remote content to be IDENTICAL to what we'd commit.
	f.setFile(evidencePath, content, "existing-sha-abc")

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath})
	if code != deskkit.ExitOK {
		t.Fatalf("idempotent noop exit = %d, want 0", code)
	}

	// Verify NO PUT happened.
	if f.putCalls != 0 {
		t.Fatalf("expected 0 PUT calls for noop, got %d", f.putCalls)
	}

	// Audit should show ResultNoop.
	last := lastAudit(t)
	if last.Result != deskkit.ResultNoop {
		t.Fatalf("last audit result = %q, want noop", last.Result)
	}
}

// TestSecretScanRefused verifies that evidence containing a secret is refused (exit 5).
func TestSecretScanRefused(t *testing.T) {
	f, _ := setupFake(t)

	// Evidence content containing a GitHub token prefix - constructed
	// at runtime so the literal does not appear in source (would trip
	// the diff-level secret scan on PR creation).
	content := "# Brief\n\n## Evidence\n| 1 | test | " + "ghp_" + "secret1234567890abcdef\n"
	evidencePath := writeRepoFile(t, "docs/brief.md", content)

	// Still need remote file to exist so we get past fetch.
	f.setFile(evidencePath, "old content", "old-sha")

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath})
	if code != deskkit.ExitRefused {
		t.Fatalf("secret scan exit = %d, want %d (ExitRefused)", code, deskkit.ExitRefused)
	}

	// Verify NO PUT happened.
	if f.putCalls != 0 {
		t.Fatal("expected 0 PUT calls when secret scan refuses")
	}

	// Audit should show ResultRefused.
	last := lastAudit(t)
	if last.Result != deskkit.ResultRefused {
		t.Fatalf("last audit result = %q, want refused", last.Result)
	}
}

// TestSecretScanRefusedNoRemote verifies the secret scan fires BEFORE any
// remote fetch — the evidence itself is scanned first.
func TestSecretScanRefusedNoRemote(t *testing.T) {
	_, _ = setupFake(t)

	content := "ghp_" + "test12345678901234567890"
	evidencePath := writeRepoFile(t, "docs/brief.md", content)

	// No remote file set up — but the secret scan should fire first
	// (only after we check for --brief-path merging which we're not using).
	// The secret scan is on the commit content. But we don't have a remote
	// file set up, so it would fail at fetch before reaching BodyCheck.
	// Actually, BodyCheck runs on the local content early enough.
	// Wait — BodyCheck runs AFTER reading local content but we need the
	// remote SHA for idempotency. Let me check the flow...

	// The flow in cmdEvidence is:
	// 1. Read local file
	// 2. BodyCheck (after determining commit content)
	// 3. Fetch remote
	// 4. Idempotency
	// 5. AllowWrite
	// 6. Commit

	// If there's no --brief-path, commitContent = localContent.
	// BodyCheck runs BEFORE fetchRemoteFile. So secret scan fires first.
	// But we need the remote file to exist because the flow fetches it.
	f := &fakeGH{srv: nil} // won't be used but need the var

	// Set up a minimal fake GH server.
	fakeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "access_tokens") {
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"token": "fake-token", "expires_at": time.Now().Add(time.Hour).Format(time.RFC3339)})
			return
		}
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "contents") {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"sha": "abc", "content": base64.StdEncoding.EncodeToString([]byte("old"))})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer fakeServer.Close()

	oldBase := apiBaseURL
	apiBaseURL = fakeServer.URL
	defer func() { apiBaseURL = oldBase }()

	oldClient := httpClient
	httpClient = &http.Client{Transport: &rewriteTransport{orig: fakeServer.URL}}
	defer func() { httpClient = oldClient }()

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath})
	if code != deskkit.ExitRefused {
		t.Fatalf("secret scan exit = %d, want %d (ExitRefused)", code, deskkit.ExitRefused)
	}

	_ = f
}

// TestOversizeRefused verifies that an evidence file exceeding maxBytes is refused.
func TestOversizeRefused(t *testing.T) {
	_, _ = setupFake(t)

	content := strings.Repeat("x", maxBytes+1)
	evidencePath := writeRepoFile(t, "docs/brief.md", content)

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath})
	if code != deskkit.ExitRefused {
		t.Fatalf("oversize exit = %d, want %d (ExitRefused)", code, deskkit.ExitRefused)
	}
}

// TestCommitAttributionToVerifierApp asserts the two things that make an Evidence commit
// unforgeable, neither of which the pre-#228 version of this test
// could observe:
//
//  1. the token was minted from the VERIFIER App's installation — the fake now 404s every
//     other installation, so a wrong-App mint cannot silently succeed; and
//  2. the commit that LANDED is authored by assay-verifier-app[bot] — read out of the
//     response GitHub returned, not inferred from the token we sent.
//
// The old version asserted `f.putTokenCheck`, which the fake set whenever the PUT carried
// "fake-verifier-installation-token" — a string the fake returned for EVERY installation.
// It therefore proved only that the tool forwards whatever the mint handed back, and
// passed unchanged while `installForOwner` returned the reviewer App's installation IDs.
func TestCommitAttributionToVerifierApp(t *testing.T) {
	f, _ := setupFake(t)

	content := `# Brief

## Evidence
| 1 | test | verified |
`
	evidencePath := writeRepoFile(t, "docs/brief.md", content)
	f.setFile(evidencePath, "old content", "old-sha")

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath})
	if code != deskkit.ExitOK {
		t.Fatalf("commit exit = %d, want 0", code)
	}

	// (1) Every mint targeted the verifier App's installation — no reviewer ID anywhere.
	attempts := f.mintAttempts()
	if len(attempts) == 0 {
		t.Fatal("no installation token was minted at all")
	}
	for _, got := range attempts {
		if got != verifierInstallID {
			t.Fatalf("minted against installation %s, want the verifier App's %s", got, verifierInstallID)
		}
		for _, bad := range reviewerInstallIDs {
			if got == bad {
				t.Fatalf("minted against the REVIEWER App's installation %s", bad)
			}
		}
	}

	// (2) The PUT carried that installation's token, and the landed commit is the bot's.
	if !f.putTokenCheck {
		t.Fatalf("the PUT did not carry the verifier installation's token (%s)", installToken(verifierInstallID))
	}
	last := lastAudit(t)
	if !strings.Contains(last.Detail, "author="+verifierBotDisplay()) {
		t.Fatalf("audit detail = %q, want it to record author=%s", last.Detail, verifierBotDisplay())
	}
}

// TestWrongAppInstallationIsNotSilentlyAccepted is the negative half of the test above.
//
// It scripts exactly the pre-#228 production state: the installation lookup answers with
// the REVIEWER App's medici-finance installation (100000001) — the very constant
// `installForOwner` used to return under a doc comment claiming it was the verifier's.
// GitHub rejects an App JWT against another App's installation, and the fake models that,
// so the tool must fail rather than commit.
//
// The load-bearing assertion is `mintedFor`, not the exit code. A build that ignores the
// lookup entirely also exits 6 here (its own hardcoded 100000002 is refused by the same
// fake), so an exit-code-only test would pass on pre-fix main for an unrelated reason and
// prove nothing. Asserting WHICH installation was tried is what shows the ID came from
// discovery.
func TestWrongAppInstallationIsNotSilentlyAccepted(t *testing.T) {
	f, _ := setupFake(t)
	f.lookupInstallID = "100000001" // assay-reviewer-app

	evidencePath := writeRepoFile(t, "docs/brief.md", "# Brief\n\n## Evidence\n| 1 | x | y |\n")
	f.setFile(evidencePath, "old content", "old-sha")

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath})
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("exit = %d, want %d (unverifiable)", code, deskkit.ExitUnverifiable)
	}
	if f.putCalls != 0 {
		t.Fatalf("putCalls = %d, want 0 — nothing may be committed under a foreign App", f.putCalls)
	}
	got := f.mintAttempts()
	if len(got) != 1 || got[0] != f.lookupInstallID {
		t.Fatalf("minted against %v, want exactly [%s] — the installation must come from the "+
			"lookup, not from a constant in this package", got, f.lookupInstallID)
	}
}

// TestCommitAttributedToAnotherAppIsRefused covers the case the token check cannot: the
// mint succeeded, the PUT succeeded, and the commit landed under someone else's identity.
// That is the failure mode Evidence rows must never report as success.
func TestCommitAttributedToAnotherAppIsRefused(t *testing.T) {
	f, _ := setupFake(t)
	// Pin the installation so the mint succeeds regardless of how it is resolved. This
	// test is about the ATTRIBUTION post-condition alone: the token was fine, the PUT
	// returned 201, and the commit still landed under another identity. Without the pin,
	// a build with the wrong hardcoded installation would fail earlier and the attribution
	// property would never be exercised.
	t.Setenv("VERIFIER_INSTALL_ID", verifierInstallID)
	f.commitAuthor = "assay-reviewer-app[bot]"

	evidencePath := writeRepoFile(t, "docs/brief.md", "# Brief\n\n## Evidence\n| 1 | x | y |\n")
	f.setFile(evidencePath, "old content", "old-sha")

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath})
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("exit = %d, want %d — a commit authored by another App is not verified Evidence", code, deskkit.ExitUnverifiable)
	}
	last := lastAudit(t)
	if !strings.Contains(last.Detail, "assay-reviewer-app[bot]") {
		t.Fatalf("audit detail = %q, want it to name the identity that actually landed", last.Detail)
	}
	// This is the ONE path where the commit has already landed on the remote and the tool
	// cannot undo it, and the flag is one-shot (a re-run noops on idempotency). The audit
	// line is therefore the whole durable record, so it must also say WHAT landed WHERE —
	// not just that the identity was wrong (#406). Logging the bare
	// err.Error() drops all four.
	f.mu.Lock()
	landedSHA := f.putNewSHA
	f.mu.Unlock()
	for _, want := range []string{
		evidencePath,                 // path
		"example-org/tracker",        // repo
		"on main",                    // branch
		"sha " + shortSHA(landedSHA), // what landed
	} {
		if !strings.Contains(last.Detail, want) {
			t.Errorf("audit detail = %q, want it to record %q — the misattributed commit is already "+
				"on the remote and this line is the only record of it", last.Detail, want)
		}
	}
}

// TestLookupFailureFailsClosed is the behavioural half of #228's "no silent fallback"
// claim, and the one property the source-level TestInstallForOwnerIsGone cannot cover:
// that guard greps for the two REVIEWER App literals, so a fallback to any OTHER
// installation ID is invisible to it. Mutating either failure branch of resolveInstallID
// to `return <some-other-id>, nil` leaves that guard — and the rest of the suite — green.
//
// mintAttempts() is the load-bearing assertion, not the exit code. A build that fell back
// to a wrong ID would ALSO exit 6 here, because the fake 404s every installation but the
// verifier's; only "which installation did it try" distinguishes fail-closed from
// fell-back-and-was-rejected. Zero mints is the property.
//
// The table covers EVERY way resolveInstallID can fail, not the two the original pair of
// subtests could express. It was two because the fixture could only script an HTTP status:
// the branches that never see a response (the request-build error, the transport error)
// and the ones that see a useless one (unparseable body, absent/zero id) had no way to be
// reached, so a silent fallback in any of the four survived the whole package green
// (#406). Confirmed before the fix — each of the four, mutated to
// `return "148002199", nil`, left `go test ./cmd/deskevidence/ -count=1` reporting ok.
// The fixture now carries lookupHijack and lookupBody, and the fourth arrives through an
// unparseable apiBaseURL, so all six modes are behavioural.
//
// TestResolveInstallIDHasNoSilentFallback (failclosed_test.go) closes the same class at
// the source level, for a branch that does not exist yet.
func TestLookupFailureFailsClosed(t *testing.T) {
	cases := []struct {
		name string
		// arrange scripts the failure. It runs after setupFake, so it may override
		// apiBaseURL (restored by setupFake's own cleanup).
		arrange func(t *testing.T, f *fakeGH)
		// branch names the arm of resolveInstallID this drives, for the failure message.
		branch string
	}{
		{
			name:    "HTTP404",
			branch:  "resp.StatusCode == http.StatusNotFound",
			arrange: func(_ *testing.T, f *fakeGH) { f.lookupStatus = http.StatusNotFound },
		},
		{
			name:    "HTTP500",
			branch:  "resp.StatusCode < 200 || >= 300",
			arrange: func(_ *testing.T, f *fakeGH) { f.lookupStatus = http.StatusInternalServerError },
		},
		{
			name:    "TransportError",
			branch:  "httpClient.Do returns an error",
			arrange: func(_ *testing.T, f *fakeGH) { f.lookupHijack = true },
		},
		{
			name:    "UnparseableBody",
			branch:  "json.Unmarshal fails",
			arrange: func(_ *testing.T, f *fakeGH) { f.lookupBody = "<html>502 Bad Gateway</html>" },
		},
		{
			name:    "ZeroID",
			branch:  `id == "0"`,
			arrange: func(_ *testing.T, f *fakeGH) { f.lookupBody = `{"id": 0}` },
		},
		{
			name:    "MissingIDField",
			branch:  `id == ""`,
			arrange: func(_ *testing.T, f *fakeGH) { f.lookupBody = `{"account": {"login": "medici-finance"}}` },
		},
		{
			name:   "UnbuildableRequest",
			branch: "http.NewRequest returns an error",
			arrange: func(_ *testing.T, _ *fakeGH) {
				// A DEL byte is a control character; url.Parse rejects it, so the request
				// is never built and no connection is ever opened.
				apiBaseURL = "http://api.\x7fgithub.invalid"
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, _ := setupFake(t)
			tc.arrange(t, f)

			evidencePath := writeRepoFile(t, "docs/brief.md", "# Brief\n\n## Evidence\n| 1 | x | y |\n")
			f.setFile(evidencePath, "old content", "old-sha")

			code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath})
			if code != deskkit.ExitUnverifiable {
				t.Errorf("exit = %d, want %d (unverifiable) — a lookup that failed at %q is not a licence to guess",
					code, deskkit.ExitUnverifiable, tc.branch)
			}
			if got := f.mintAttempts(); len(got) != 0 {
				t.Errorf("minted against %v, want none — the installation lookup failed at %q, so there is no "+
					"installation to mint against; any ID here is a silent fallback", got, tc.branch)
			}
			f.mu.Lock()
			puts := f.putCalls
			f.mu.Unlock()
			if puts != 0 {
				t.Errorf("putCalls = %d, want 0 — nothing may be committed on an unresolved installation", puts)
			}
		})
	}
}

// TestMissingAuthorIsCouldNotCheckNotSuccess pins the third state. A response carrying no
// author is NOT proof of a wrong identity, so it must not refuse; it is also not proof of
// the right one, so it must not report plain success either.
func TestMissingAuthorIsCouldNotCheckNotSuccess(t *testing.T) {
	f, errBuf := setupFake(t)
	t.Setenv("VERIFIER_INSTALL_ID", verifierInstallID) // isolate the attribution property
	f.commitAuthor = "none"                            // fake omits `commit.author` entirely

	evidencePath := writeRepoFile(t, "docs/brief.md", "# Brief\n\n## Evidence\n| 1 | x | y |\n")
	f.setFile(evidencePath, "old content", "old-sha")

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath})
	if code != deskkit.ExitOK {
		t.Fatalf("exit = %d, want 0 — an unreadable author is not evidence of a wrong one", code)
	}
	if !strings.Contains(errBuf.String(), "could NOT verify") {
		t.Fatalf("stderr = %q, want a could-not-check warning", errBuf.String())
	}
	if d := lastAudit(t).Detail; !strings.Contains(d, "could-not-check") {
		t.Fatalf("audit detail = %q, want it to record could-not-check", d)
	}
}

// TestVerifierInstallIDOverridesDiscovery keeps the documented escape hatch honest: when
// the env var is set, no lookup is made and that installation is used.
//
// Stated plainly: this one PASSES on pre-fix main too (that build never calls the lookup
// endpoint because it has none). It is a regression guard on the override, not a
// demonstration of the defect — the demonstrations are the two tests above.
func TestVerifierInstallIDOverridesDiscovery(t *testing.T) {
	f, _ := setupFake(t)
	t.Setenv("VERIFIER_INSTALL_ID", verifierInstallID)
	f.lookupInstallID = "100000002" // would be wrong; must never be consulted

	evidencePath := writeRepoFile(t, "docs/brief.md", "# Brief\n\n## Evidence\n| 1 | x | y |\n")
	f.setFile(evidencePath, "old content", "old-sha")

	if code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath}); code != deskkit.ExitOK {
		t.Fatalf("exit = %d, want 0", code)
	}
	f.mu.Lock()
	hits := append([]string(nil), f.hits...)
	f.mu.Unlock()
	for _, h := range hits {
		if strings.HasSuffix(h, "/installation") {
			t.Fatalf("discovery was called despite VERIFIER_INSTALL_ID being set: %s", h)
		}
	}
	for _, got := range f.mintAttempts() {
		if got != verifierInstallID {
			t.Fatalf("minted against %s, want the override %s", got, verifierInstallID)
		}
	}
}

// TestInstallForOwnerIsGone is a source-level guard. The defect in #228 was a hardcoded
// constant; a future edit that "fixes" it by seating a corrected constant re-opens the
// class. The installation must come from GitHub or from the explicit override, never from
// a table in this package.
func TestInstallForOwnerIsGone(t *testing.T) {
	src, err := os.ReadFile("github.go")
	if err != nil {
		t.Fatalf("read github.go: %v", err)
	}
	for _, bad := range reviewerInstallIDs {
		// The doc comment on resolveInstallID quotes both IDs as the negative example;
		// only a code occurrence is a defect, so require the line to not be a comment.
		for _, line := range strings.Split(string(src), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") {
				continue
			}
			if strings.Contains(trimmed, bad) {
				t.Fatalf("installation ID %s is hardcoded in non-comment source: %q", bad, trimmed)
			}
		}
	}
}

// TestMergeEvidenceIntoBrief verifies --brief-path mode: evidence from one file
// is merged into the Evidence section of another brief file.
func TestMergeEvidenceIntoBrief(t *testing.T) {
	f, _ := setupFake(t)

	// Write evidence file with the rows to append.
	evidenceContent := "| 1 | `go test ./...` | exit 0 |\n| 2 | `go vet ./...` | exit 0 |"
	evidencePath := writeRepoFile(t, "evidence.md", evidenceContent)

	// Set up remote brief file with existing Evidence section.
	briefContent := `# Brief

## Context
Some content.

## Evidence
existing row
`
	f.setFile("docs/brief.md", briefContent, "brief-sha-1")

	code := run([]string{"example-org/tracker", "main",
		"--evidence-file", evidencePath,
		"--brief-path", "docs/brief.md",
	})
	if code != deskkit.ExitOK {
		t.Fatalf("merge commit exit = %d, want 0", code)
	}

	if f.putCalls != 1 {
		t.Fatalf("expected 1 PUT call, got %d", f.putCalls)
	}

	// The committed content should include both the existing evidence and the new rows.
	if !strings.Contains(f.putContent, "existing row") {
		t.Fatal("committed content should contain existing evidence row")
	}
	if !strings.Contains(f.putContent, "| 1 | `go test ./...` | exit 0 |") {
		t.Fatal("committed content should contain new evidence rows")
	}
	if !strings.Contains(f.putContent, "## Evidence") {
		t.Fatal("committed content should contain Evidence section header")
	}
}

// TestBadRepoRefused verifies that a repo slug not matching owner/name is refused.
func TestBadRepoRefused(t *testing.T) {
	_, _ = setupFake(t)

	code := run([]string{"not-a-repo", "main", "--evidence-file", "file.md"})
	if code != deskkit.ExitRefused {
		t.Fatalf("bad repo exit = %d, want %d", code, deskkit.ExitRefused)
	}
}

// TestMissingBranchRefused verifies that an empty branch is refused.
func TestMissingBranchRefused(t *testing.T) {
	_, _ = setupFake(t)

	code := run([]string{"example-org/tracker", "", "--evidence-file", "file.md"})
	if code != deskkit.ExitRefused {
		t.Fatalf("empty branch exit = %d, want %d", code, deskkit.ExitRefused)
	}
}

// TestUnpinnedWarning verifies the unpinned warning appears for any non-version invocation.
func TestUnpinnedWarning(t *testing.T) {
	_, errBuf := setupFake(t)
	// Use a bad-args call that hits WarnIfUnpinned before the error.
	run([]string{"bad-owner", "main", "--evidence-file", "file.md"})
	if !strings.Contains(errBuf.String(), "UNPINNED") {
		t.Fatalf("expected UNPINNED warning on stderr, got %q", errBuf.String())
	}
}

// TestAuditFields verifies the audit line carries all expected fields.
func TestAuditFields(t *testing.T) {
	f, _ := setupFake(t)

	content := `# Brief

## Evidence
row
`
	evidencePath := writeRepoFile(t, "docs/brief.md", content)
	f.setFile(evidencePath, "old", "old-sha")

	run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath})

	last := lastAudit(t)
	if last.Tool != "deskevidence" {
		t.Fatalf("audit Tool = %q, want deskevidence", last.Tool)
	}
	if last.Verb != "commit" {
		t.Fatalf("audit Verb = %q, want commit", last.Verb)
	}
	if last.Repo != "example-org/tracker" {
		t.Fatalf("audit Repo = %q, want example-org/tracker", last.Repo)
	}
	if last.ArgsDigest == "" {
		t.Fatal("audit ArgsDigest should not be empty")
	}
	if last.BodyDigest == "" {
		t.Fatal("audit BodyDigest should not be empty for a commit")
	}
	if last.SourceSHA == "" {
		t.Fatal("audit SourceSHA should not be empty")
	}
	if last.SessionTag != "deskevidence-test" {
		t.Fatalf("audit SessionTag = %q, want deskevidence-test", last.SessionTag)
	}
}

// --- Guard 1: repo-set gate (#1282) ---
//
// deskevidence was the only outward-writing desk command with no repo allowlist
// check, so a typo'd or hostile owner/repo reached the App-token commit path
// unchecked. The gate must refuse (exit 5), audit the attempt, and make NO
// network call.

func TestRepoNotInSetRefused(t *testing.T) {
	f, _ := setupFake(t)

	evidencePath := writeRepoFile(t, "docs/brief.md", "# Brief\n\n## Evidence\nrow\n")

	for _, repo := range []string{
		"attacker/evil-repo",
		"example-org/trackr", // one-character typo of an allowed repo
		"medici-finance/not-a-desk-repo",
	} {
		code := run([]string{repo, "feature-branch", "--evidence-file", evidencePath})
		if code != deskkit.ExitRefused {
			t.Fatalf("repo %q exit = %d, want %d (refused)", repo, code, deskkit.ExitRefused)
		}
		if last := lastAudit(t); last.Result != deskkit.ResultRefused {
			t.Fatalf("repo %q audit result = %q, want refused", repo, last.Result)
		}
	}

	if len(f.hits) != 0 {
		t.Fatalf("out-of-set repo must make NO network call, got %v", f.hits)
	}
	if f.putCalls != 0 {
		t.Fatalf("out-of-set repo must make no PUT, got %d", f.putCalls)
	}
}

// TestRepoInSetNotRefusedByRepoGate proves the gate is a gate and not a blanket
// refusal: an allowed repo passes it and proceeds to the commit.
func TestRepoInSetNotRefusedByRepoGate(t *testing.T) {
	f, _ := setupFake(t)

	evidencePath := writeRepoFile(t, "docs/brief.md", "# Brief\n\n## Evidence\nrow\n")
	f.setFile(evidencePath, "# Brief\n\n## Evidence\n", "old-sha")

	code := run([]string{"medici-finance/assay", "feature-branch", "--evidence-file", evidencePath})
	if code != deskkit.ExitOK {
		t.Fatalf("allowed repo exit = %d, want 0", code)
	}
	if f.putCalls != 1 {
		t.Fatalf("allowed repo PUT calls = %d, want 1", f.putCalls)
	}
}

// --- Guard 2: main-branch write guard (#1282) ---
//
// A Contents-API commit writes to the REMOTE branch immediately, so branch=main
// is a live write to main. Refuse unless VERIFIER_MAIN_OK is set.

func TestMainBranchRefusedWithoutSanction(t *testing.T) {
	f, _ := setupFake(t)
	t.Setenv("VERIFIER_MAIN_OK", "") // clear the blanket sanction set by setupFake

	evidencePath := writeRepoFile(t, "docs/brief.md", "# Brief\n\n## Evidence\nrow\n")
	f.setFile(evidencePath, "# Brief\n\n## Evidence\n", "old-sha")

	for _, branch := range []string{"main", "master"} {
		code := run([]string{"example-org/tracker", branch, "--evidence-file", evidencePath})
		if code != deskkit.ExitRefused {
			t.Fatalf("branch %q exit = %d, want %d (refused)", branch, code, deskkit.ExitRefused)
		}
		if last := lastAudit(t); last.Result != deskkit.ResultRefused {
			t.Fatalf("branch %q audit result = %q, want refused", branch, last.Result)
		}
	}

	if len(f.hits) != 0 {
		t.Fatalf("main-branch refusal must make NO network call, got %v", f.hits)
	}
	if f.putCalls != 0 {
		t.Fatalf("main-branch refusal must make no PUT, got %d", f.putCalls)
	}
}

func TestMainBranchAllowedWithSanction(t *testing.T) {
	f, _ := setupFake(t)
	t.Setenv("VERIFIER_MAIN_OK", "1")

	evidencePath := writeRepoFile(t, "docs/brief.md", "# Brief\n\n## Evidence\nrow\n")
	f.setFile(evidencePath, "# Brief\n\n## Evidence\n", "old-sha")

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath})
	if code != deskkit.ExitOK {
		t.Fatalf("sanctioned main commit exit = %d, want 0", code)
	}
	if f.putCalls != 1 {
		t.Fatalf("sanctioned main commit PUT calls = %d, want 1", f.putCalls)
	}
	if f.putBranch != "main" {
		t.Fatalf("PUT branch = %q, want main", f.putBranch)
	}
}

// TestNonMainBranchNeedsNoSanction proves the branch guard keys on main/master and
// does not gate ordinary branches — otherwise "refuses everything" would pass the
// test above for the wrong reason.
func TestNonMainBranchNeedsNoSanction(t *testing.T) {
	f, _ := setupFake(t)
	t.Setenv("VERIFIER_MAIN_OK", "")

	evidencePath := writeRepoFile(t, "docs/brief.md", "# Brief\n\n## Evidence\nrow\n")
	f.setFile(evidencePath, "# Brief\n\n## Evidence\n", "old-sha")

	code := run([]string{"example-org/tracker", "fix/some-branch", "--evidence-file", evidencePath})
	if code != deskkit.ExitOK {
		t.Fatalf("unsanctioned non-main commit exit = %d, want 0", code)
	}
	if f.putCalls != 1 {
		t.Fatalf("non-main commit PUT calls = %d, want 1", f.putCalls)
	}
}

// TestMainSanctionMustBeExactlyOne pins the arming value. VERIFIER_MAIN_OK is a
// safety-arming variable, and this repo's convention for those is an exact "1"
// (internal/deskkit/killswitch.go); the tool's own help text says "set
// VERIFIER_MAIN_OK=1". A truthy-looking-but-not-1 value must NOT sanction a main
// write, or "VERIFIER_MAIN_OK=0" reads as off and behaves as on.
func TestMainSanctionMustBeExactlyOne(t *testing.T) {
	for _, val := range []string{"0", " ", "true", "yes"} {
		t.Run(val, func(t *testing.T) {
			f, _ := setupFake(t)
			t.Setenv("VERIFIER_MAIN_OK", val)

			evidencePath := writeRepoFile(t, "docs/brief.md", "# Brief\n\n## Evidence\nrow\n")
			f.setFile(evidencePath, "# Brief\n\n## Evidence\n", "old-sha")

			code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath})
			if code != deskkit.ExitRefused {
				t.Fatalf("VERIFIER_MAIN_OK=%q exit = %d, want %d (refused)", val, code, deskkit.ExitRefused)
			}
			if len(f.hits) != 0 {
				t.Fatalf("VERIFIER_MAIN_OK=%q must make NO network call, got %v", val, f.hits)
			}
			if f.putCalls != 0 {
				t.Fatalf("VERIFIER_MAIN_OK=%q must make no PUT, got %d", val, f.putCalls)
			}
		})
	}
}

// TestFullRefBranchStillGuarded proves the main guard cannot be walked past by
// spelling the branch as a full ref. The guard compares the ref name with any
// "refs/heads/" prefix stripped.
func TestFullRefBranchStillGuarded(t *testing.T) {
	for _, branch := range []string{"refs/heads/main", "refs/heads/master"} {
		t.Run(branch, func(t *testing.T) {
			f, _ := setupFake(t)
			t.Setenv("VERIFIER_MAIN_OK", "")

			evidencePath := writeRepoFile(t, "docs/brief.md", "# Brief\n\n## Evidence\nrow\n")
			f.setFile(evidencePath, "# Brief\n\n## Evidence\n", "old-sha")

			code := run([]string{"example-org/tracker", branch, "--evidence-file", evidencePath})
			if code != deskkit.ExitRefused {
				t.Fatalf("branch %q exit = %d, want %d (refused)", branch, code, deskkit.ExitRefused)
			}
			if len(f.hits) != 0 {
				t.Fatalf("branch %q refusal must make NO network call, got %v", branch, f.hits)
			}
		})
	}
}

// TestFullRefNonMainNotGuarded is the positive control for the normalisation above:
// stripping the prefix must not turn every full ref into a main write.
func TestFullRefNonMainNotGuarded(t *testing.T) {
	f, _ := setupFake(t)
	t.Setenv("VERIFIER_MAIN_OK", "")

	evidencePath := writeRepoFile(t, "docs/brief.md", "# Brief\n\n## Evidence\nrow\n")
	f.setFile(evidencePath, "# Brief\n\n## Evidence\n", "old-sha")

	code := run([]string{"example-org/tracker", "refs/heads/fix/some-branch", "--evidence-file", evidencePath})
	if code != deskkit.ExitOK {
		t.Fatalf("unsanctioned full-ref non-main commit exit = %d, want 0", code)
	}
	if f.putCalls != 1 {
		t.Fatalf("full-ref non-main commit PUT calls = %d, want 1", f.putCalls)
	}
}

// --- Generated-file guard: STATUS.md ---
//
// STATUS.md is generated and main's CI is its single writer. This tool is what turns
// "main is ungated" into "main is a sanctioned channel", and VERIFIER_MAIN_OK is set
// routinely in the verify-desk window, so the branch guard alone would let one coarse
// env var open main to a generated file. The refusal applies on every branch and to
// BOTH the --evidence-file and --brief-path targets, before any network call.

func TestStatusMDRefusedAsEvidenceFile(t *testing.T) {
	for _, branch := range []string{"main", "fix/some-branch"} {
		t.Run(branch, func(t *testing.T) {
			f, _ := setupFake(t) // VERIFIER_MAIN_OK=1 — the branch guard must not be what refuses

			evidencePath := writeRepoFile(t, "STATUS.md", "# STATUS\n\nregenerated\n")
			f.setFile(evidencePath, "# STATUS\n", "old-sha")

			code := run([]string{"example-org/tracker", branch, "--evidence-file", evidencePath})
			if code != deskkit.ExitRefused {
				t.Fatalf("STATUS.md on %q exit = %d, want %d (refused)", branch, code, deskkit.ExitRefused)
			}
			if last := lastAudit(t); last.Result != deskkit.ResultRefused {
				t.Fatalf("STATUS.md on %q audit result = %q, want refused", branch, last.Result)
			}
			if len(f.hits) != 0 {
				t.Fatalf("STATUS.md refusal must make NO network call, got %v", f.hits)
			}
			if f.putCalls != 0 {
				t.Fatalf("STATUS.md refusal must make no PUT, got %d", f.putCalls)
			}
		})
	}
}

// TestStatusMDRefusedAsBriefPath covers the merge path, where the target is --brief-path
// and the brief is FETCHED before the target path is resolved — so a check placed on the
// resolved target only would already have hit the network.
func TestStatusMDRefusedAsBriefPath(t *testing.T) {
	f, _ := setupFake(t)

	evidencePath := writeRepoFile(t, "evidence-row.md", "| row |\n")
	f.setFile("STATUS.md", "# STATUS\n\n## Evidence\n", "old-sha")

	code := run([]string{"example-org/tracker", "main",
		"--evidence-file", evidencePath, "--brief-path", "STATUS.md"})
	if code != deskkit.ExitRefused {
		t.Fatalf("--brief-path STATUS.md exit = %d, want %d (refused)", code, deskkit.ExitRefused)
	}
	if len(f.hits) != 0 {
		t.Fatalf("--brief-path STATUS.md must make NO network call, got %v", f.hits)
	}
	if f.putCalls != 0 {
		t.Fatalf("--brief-path STATUS.md must make no PUT, got %d", f.putCalls)
	}
}

// TestNonStatusFileStillCommits is the positive control: the guard keys on the
// basename STATUS.md and does not refuse ordinary evidence files whose names merely
// resemble it — otherwise "refuses everything" would pass the two tests above.
func TestNonStatusFileStillCommits(t *testing.T) {
	for _, name := range []string{"docs/STATUS-notes.md", "docs/status.md"} {
		t.Run(name, func(t *testing.T) {
			f, _ := setupFake(t)

			evidencePath := writeRepoFile(t, name, "# Brief\n\n## Evidence\nrow\n")
			f.setFile(evidencePath, "# Brief\n\n## Evidence\n", "old-sha")

			code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath})
			if code != deskkit.ExitOK {
				t.Fatalf("%s exit = %d, want 0", name, code)
			}
			if f.putCalls != 1 {
				t.Fatalf("%s PUT calls = %d, want 1", name, f.putCalls)
			}
		})
	}
}

// --- the public-repo gate, proven by its EFFECT on the write ---
//
// These three exist because the gate shipped untested: an early review
// removed the PublicRepoGate call from deskevidence.go entirely, re-ran this package,
// and nothing failed. A gate no test can miss the absence of is a gate a later
// refactor deletes silently. Each of these fails if the call is removed.

// TestPublicRepoGateRefusesCommitToPublicRepo — deskevidence commits a file via
// PUT /repos/{owner}/{repo}/contents/{path}, an outward write. A file commit has no
// issue/PR number, so there is no reactions surface on which a ada +1 could live:
// on a PUBLIC repo the gate can never be satisfied and must refuse (exit 6) BEFORE
// any write. Asserting putCalls == 0 as well as the exit code is the point — the exit
// code alone would still pass if the refusal happened after the PUT.
func TestPublicRepoGateRefusesCommitToPublicRepo(t *testing.T) {
	f, _ := setupFake(t)
	f.repoVisibility = "public"

	evidencePath := writeRepoFile(t, "docs/brief.md", "# Brief\n\n## Evidence\nrow\n")
	f.setFile(evidencePath, "# Brief\n\n## Evidence\n", "old-sha")

	code := run([]string{"example-org/tracker", "fix/some-branch", "--evidence-file", evidencePath})
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("commit to a PUBLIC repo: exit = %d, want %d (unverifiable) — "+
			"the public-repo gate is not wired into deskevidence", code, deskkit.ExitUnverifiable)
	}
	if f.putCalls != 0 {
		t.Fatalf("PUT calls = %d, want 0 — deskevidence wrote to a public repo behind the gate", f.putCalls)
	}
}

// TestPublicRepoGateFailsClosedOnVisibilityError — visibility unreadable (500, rate
// limit, token hiccup) is NOT "assume private". Refuse, and make no write.
func TestPublicRepoGateFailsClosedOnVisibilityError(t *testing.T) {
	f, _ := setupFake(t)
	f.repoVisibilityErr = true

	evidencePath := writeRepoFile(t, "docs/brief.md", "# Brief\n\n## Evidence\nrow\n")
	f.setFile(evidencePath, "# Brief\n\n## Evidence\n", "old-sha")

	code := run([]string{"example-org/tracker", "fix/some-branch", "--evidence-file", evidencePath})
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("unreadable visibility: exit = %d, want %d (unverifiable) — never guess private",
			code, deskkit.ExitUnverifiable)
	}
	if f.putCalls != 0 {
		t.Fatalf("PUT calls = %d, want 0 — wrote without establishing visibility", f.putCalls)
	}
}

// TestPublicRepoGatePassesPrivateRepo — the other half: the gate must not break the
// normal path. Without this, "refuse everything" would pass the two tests above.
func TestPublicRepoGatePassesPrivateRepo(t *testing.T) {
	f, _ := setupFake(t)
	f.repoVisibility = "private"

	evidencePath := writeRepoFile(t, "docs/brief.md", "# Brief\n\n## Evidence\nrow\n")
	f.setFile(evidencePath, "# Brief\n\n## Evidence\n", "old-sha")

	code := run([]string{"example-org/tracker", "fix/some-branch", "--evidence-file", evidencePath})
	if code != deskkit.ExitOK {
		t.Fatalf("commit to a PRIVATE repo: exit = %d, want 0 — the gate must pass private through", code)
	}
	if f.putCalls != 1 {
		t.Fatalf("PUT calls = %d, want 1", f.putCalls)
	}
}

// --- #1709: --root binding, append-only shrink guard, and net-delta reporting ---

// jsonlRows builds n distinct newline-terminated JSONL rows, so row counts and
// multiset deltas in the tests below are unambiguous.
func jsonlRows(t *testing.T, n int) string {
	t.Helper()
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "{\"row\":%d}\n", i)
	}
	return b.String()
}

// stdoutString returns what the tool printed to the captured stdout buffer.
func stdoutString(t *testing.T) string {
	t.Helper()
	buf, ok := stdout.(*bytes.Buffer)
	if !ok {
		t.Fatal("stdout is not a *bytes.Buffer — setupFake not in effect")
	}
	return buf.String()
}

// TestRootResolvesEvidenceFileAgainstCheckout proves --root binds a repo-relative
// --evidence-file to the named checkout rather than the process cwd (#1709). The file is
// written UNDER a --root dir; the tool must read it from root and commit it to the
// repo-relative target path.
func TestRootResolvesEvidenceFileAgainstCheckout(t *testing.T) {
	f, _ := setupFake(t)

	root := t.TempDir()
	rel := "docs/streams/outcomes.jsonl"
	abs := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	local := jsonlRows(t, 2) // an append over the 1-row remote
	if err := os.WriteFile(abs, []byte(local), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Remote is keyed by the REPO-RELATIVE target path, not the local absolute path.
	f.setFile(rel, jsonlRows(t, 1), "sha-root-1")

	code := run([]string{"example-org/tracker", "main", "--evidence-file", rel, "--root", root})
	if code != deskkit.ExitOK {
		t.Fatalf("--root commit exit = %d, want 0", code)
	}
	if f.putCalls != 1 {
		t.Fatalf("expected 1 PUT, got %d", f.putCalls)
	}
	if f.putPath != "/repos/example-org/tracker/contents/"+rel {
		t.Fatalf("PUT path = %q, want repo-relative target %q", f.putPath, rel)
	}
	if f.putContent != local {
		t.Fatalf("committed content did not come from the --root checkout: got %q", f.putContent)
	}
}

// TestRootWithAbsoluteEvidenceFileRefused: --root plus an absolute --evidence-file is
// contradictory and refused before any network call.
func TestRootWithAbsoluteEvidenceFileRefused(t *testing.T) {
	f, _ := setupFake(t)
	code := run([]string{"example-org/tracker", "main",
		"--evidence-file", "/abs/docs/streams/x.jsonl", "--root", t.TempDir()})
	if code != deskkit.ExitRefused {
		t.Fatalf("absolute --evidence-file with --root exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if len(f.hits) != 0 {
		t.Fatalf("expected NO network hits on a pre-network refusal, got %v", f.hits)
	}
}

// TestAppendOnlyShrinkRefused is the #1709 regression: a .jsonl target auto-enables the
// append-only guard, so a commit that would drop rows (25→17) is refused with NO PUT.
func TestAppendOnlyShrinkRefused(t *testing.T) {
	f, _ := setupFake(t)

	evidencePath := writeRepoFile(t, "docs/streams/outcomes.jsonl", jsonlRows(t, 17))
	f.setFile(evidencePath, jsonlRows(t, 25), "sha-shrink-1")

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath})
	if code != deskkit.ExitRefused {
		t.Fatalf("append-only shrink exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if f.putCalls != 0 {
		t.Fatalf("putCalls = %d, want 0 — a shrinking append-only commit must not land", f.putCalls)
	}
}

// TestAppendOnlyShrinkOverride: --allow-shrink is the intentional-edit escape hatch, so the
// same shrink commits when the operator sanctions it.
func TestAppendOnlyShrinkOverride(t *testing.T) {
	f, _ := setupFake(t)

	evidencePath := writeRepoFile(t, "docs/streams/outcomes.jsonl", jsonlRows(t, 17))
	f.setFile(evidencePath, jsonlRows(t, 25), "sha-shrink-2")

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--allow-shrink"})
	if code != deskkit.ExitOK {
		t.Fatalf("--allow-shrink exit = %d, want 0", code)
	}
	if f.putCalls != 1 {
		t.Fatalf("putCalls = %d, want 1 — --allow-shrink should permit the reduction", f.putCalls)
	}
}

// TestAppendOnlyGrowthAllowed: a normal append (the common case) is never blocked, and the
// success line names the net delta.
func TestAppendOnlyGrowthAllowed(t *testing.T) {
	f, _ := setupFake(t)

	evidencePath := writeRepoFile(t, "docs/streams/outcomes.jsonl", jsonlRows(t, 25))
	f.setFile(evidencePath, jsonlRows(t, 17), "sha-grow-1")

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath})
	if code != deskkit.ExitOK {
		t.Fatalf("append growth exit = %d, want 0", code)
	}
	if f.putCalls != 1 {
		t.Fatalf("putCalls = %d, want 1", f.putCalls)
	}
	if out := stdoutString(t); !strings.Contains(out, "+8/-0 rows") {
		t.Fatalf("success line missing net-delta; got %q", out)
	}
}

// TestNonJSONLShrinkNotBlockedWithoutFlag: the auto-guard is scoped to .jsonl sidecars; a
// non-sidecar file is not blocked unless --append-only is explicitly requested.
func TestNonJSONLShrinkNotBlockedWithoutFlag(t *testing.T) {
	f, _ := setupFake(t)

	evidencePath := writeRepoFile(t, "docs/notes.md", "one\n")
	f.setFile(evidencePath, "one\ntwo\nthree\n", "sha-md-1")

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath})
	if code != deskkit.ExitOK {
		t.Fatalf("non-jsonl shrink (no flag) exit = %d, want 0", code)
	}
	if f.putCalls != 1 {
		t.Fatalf("putCalls = %d, want 1", f.putCalls)
	}
}

// TestAppendOnlyFlagBlocksNonJSONLShrink: --append-only opts a non-.jsonl file into the guard.
func TestAppendOnlyFlagBlocksNonJSONLShrink(t *testing.T) {
	f, _ := setupFake(t)

	evidencePath := writeRepoFile(t, "docs/notes.md", "one\n")
	f.setFile(evidencePath, "one\ntwo\nthree\n", "sha-md-2")

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--append-only"})
	if code != deskkit.ExitRefused {
		t.Fatalf("--append-only shrink exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if f.putCalls != 0 {
		t.Fatalf("putCalls = %d, want 0", f.putCalls)
	}
}
