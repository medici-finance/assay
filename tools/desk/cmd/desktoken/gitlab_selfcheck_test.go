package main

// gitlab_selfcheck_test.go — the post-rotation SELF-CHECK (#1142).
//
// The rotation endpoint's 200 says the forge issued the successor; in the field the very next
// authenticated read with that successor answered 401 while a direct probe of the on-disk token
// answered 200 seconds later — server-side propagation lag after self-rotation, which the mint
// path used to assume away by printing the custody path the moment the write-verify landed.
//
// These tests pin the contract: after a rotation the command performs exactly ONE live
// read-only GET with the NEW token and returns the path only when that read is 200. On any
// other outcome it exits non-zero, names the endpoint, names which token (new vs previous) the
// forge accepted, makes no retry and never prints the path as good.

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// selfCheckFixture models a GitLab instance whose read plane may LAG its rotation endpoint.
// rotate always issues newToken to a caller presenting the live token; with lag=false the
// read plane accepts newToken from then on (the fixture in gitlab_test.go), with lag=true the
// read plane keeps accepting ONLY the pre-rotation token — the observed field shape. Every
// self-check read is counted by which token it presented, so a test can assert "one read with
// the new token, one with the previous, nothing else".
type selfCheckFixture struct {
	mu          sync.Mutex
	readAccepts string // the token the read plane answers 200 to
	rotateCalls int
	checksNew   int // GET /user presenting the rotated token
	checksOld   int // GET /user presenting the pre-rotation token
	checksOther int // GET /user presenting anything else
}

func newSelfCheckServer(t *testing.T, oldToken, newToken string, lag bool) (*httptest.Server, *selfCheckFixture) {
	t.Helper()
	f := &selfCheckFixture{readAccepts: oldToken}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "GET" && strings.HasSuffix(r.URL.Path, gitlabSelfCheckPath):
			switch r.Header.Get("PRIVATE-TOKEN") {
			case newToken:
				f.checksNew++
			case oldToken:
				f.checksOld++
			default:
				f.checksOther++
			}
			if r.Header.Get("PRIVATE-TOKEN") != f.readAccepts {
				w.WriteHeader(401)
				_, _ = w.Write([]byte(`{"error":"invalid_token","error_description":"Token was revoked."}`))
				return
			}
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"id":42,"username":"example-bot","bot":true}`))
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/personal_access_tokens/self/rotate"):
			f.rotateCalls++
			if r.Header.Get("PRIVATE-TOKEN") != oldToken {
				w.WriteHeader(401)
				_, _ = w.Write([]byte(`{"message":"401 Unauthorized"}`))
				return
			}
			if !lag {
				f.readAccepts = newToken
			}
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"token":"` + newToken + `","expires_at":"2124-01-08T00:00:00Z","active":true}`))
		default:
			http.Error(w, "not found", 404)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, f
}

// TestGitLabRotateSelfCheckOK: rotate, then ONE live read with the new token answers 200, and
// only then is the path printed. The read is made exactly once, with the new token only, and the
// audit line records it.
//
// Fail-first: before the self-check existed no GET reached the fixture, so checksNew == 0.
func TestGitLabRotateSelfCheckOK(t *testing.T) {
	homeDir := setupTest(t)
	tokPath := gitlabTokenPath(homeDir, "worker")
	writeTokenCache(t, tokPath, glOldWorker)

	srv, fx := newSelfCheckServer(t, glOldWorker, glNewWorker, false)
	pointHTTPClientAt(t, srv)

	rc, stdout, stderr := runCap(t, []string{"--forge", "gitlab", "worker"})
	if rc != deskkit.ExitOK {
		t.Fatalf("rotate rc = %d, want 0; stderr: %s", rc, stderr)
	}
	if !strings.Contains(stdout, tokPath) {
		t.Fatalf("stdout should print the token PATH %q; got: %s", tokPath, stdout)
	}
	if fx.rotateCalls != 1 {
		t.Fatalf("rotate endpoint hit %d times, want 1", fx.rotateCalls)
	}
	if fx.checksNew != 1 || fx.checksOld != 0 || fx.checksOther != 0 {
		t.Fatalf("self-check reads: new=%d old=%d other=%d, want exactly one read with the NEW token",
			fx.checksNew, fx.checksOld, fx.checksOther)
	}
	assertNoTokenLeak(t, stdout+stderr)
	entries := auditEntries(t)
	if len(entries) == 0 {
		t.Fatal("expected an audit entry")
	}
	if last := entries[len(entries)-1]; !strings.Contains(last.Detail, "self-check GET "+gitlabSelfCheckPath+" 200") {
		t.Fatalf("audit detail = %q, want the self-check recorded", last.Detail)
	}
}

// TestGitLabRotateSelfCheck401NotYetValid is the field shape: rotate answers 200 with the new
// token, but the read plane still rejects it (401) and still accepts the previous one. The
// command must NOT return the path as good: it exits 6, names the endpoint, the status the new
// token got, and that the previous token was still accepted — with exactly one read per token
// and no retry. The persisted custody holds the rotated value (the rotation is not undone).
//
// Fail-first: before the self-check existed this returned rc 0 and printed the path.
func TestGitLabRotateSelfCheck401NotYetValid(t *testing.T) {
	homeDir := setupTest(t)
	tokPath := gitlabTokenPath(homeDir, "worker")
	writeTokenCache(t, tokPath, glOldWorker)

	srv, fx := newSelfCheckServer(t, glOldWorker, glNewWorker, true)
	pointHTTPClientAt(t, srv)

	rc, stdout, stderr := runCap(t, []string{"--forge", "gitlab", "worker"})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("lagging self-check rc = %d, want 6; stdout: %s stderr: %s", rc, stdout, stderr)
	}
	if strings.Contains(stdout, tokPath) {
		t.Fatalf("path must NOT be printed as good on a failed self-check; stdout: %s", stdout)
	}
	for _, want := range []string{
		"self-check FAILED",
		"GET " + gitlabSelfCheckPath,
		"NEW token answered HTTP 401",
		"PREVIOUS token was still ACCEPTED (HTTP 200)",
		"re-run the mint once",
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr should contain %q; got: %s", want, stderr)
		}
	}
	if fx.rotateCalls != 1 {
		t.Fatalf("rotate endpoint hit %d times, want 1 (no retry)", fx.rotateCalls)
	}
	if fx.checksNew != 1 || fx.checksOld != 1 || fx.checksOther != 0 {
		t.Fatalf("self-check reads: new=%d old=%d other=%d, want exactly one with the NEW and one with the PREVIOUS token (no retry loop)",
			fx.checksNew, fx.checksOld, fx.checksOther)
	}
	assertNoTokenLeak(t, stdout+stderr)
	got, err := os.ReadFile(tokPath)
	if err != nil {
		t.Fatalf("read token file: %v", err)
	}
	if string(got) != glNewWorker {
		t.Fatalf("custody should hold the rotated value (the rotation is not undone); got %q", string(got))
	}
}

// TestGitLabRotateSelfCheckBothRejected: the read plane accepts NEITHER token after the
// rotation — a lockout, not lag. The verdict must say the previous token was rejected too and
// name the group-owner recovery path, and still make exactly one read per token.
func TestGitLabRotateSelfCheckBothRejected(t *testing.T) {
	homeDir := setupTest(t)
	tokPath := gitlabTokenPath(homeDir, "worker")
	writeTokenCache(t, tokPath, glOldWorker)

	srv, fx := newSelfCheckServer(t, glOldWorker, glNewWorker, true)
	fx.mu.Lock()
	fx.readAccepts = "" // nothing is live on the read plane
	fx.mu.Unlock()
	pointHTTPClientAt(t, srv)

	rc, stdout, stderr := runCap(t, []string{"--forge", "gitlab", "worker"})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("both-rejected rc = %d, want 6; stderr: %s", rc, stderr)
	}
	for _, want := range []string{"PREVIOUS token was rejected (HTTP 401)", "group owner"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr should contain %q; got: %s", want, stderr)
		}
	}
	if fx.checksNew != 1 || fx.checksOld != 1 {
		t.Fatalf("self-check reads: new=%d old=%d, want one each", fx.checksNew, fx.checksOld)
	}
	assertNoTokenLeak(t, stdout+stderr)
}

// TestGitLabRotateSelfCheckTransportFailure: the self-check could not be MADE (the forge went
// away between the rotate and the read). That is could-not-check, not a rejection: exit 6, the
// path not printed, and no probe of the previous token is attempted against a dead host.
func TestGitLabRotateSelfCheckTransportFailure(t *testing.T) {
	homeDir := setupTest(t)
	tokPath := gitlabTokenPath(homeDir, "worker")
	writeTokenCache(t, tokPath, glOldWorker)

	srv, fx := newSelfCheckServer(t, glOldWorker, glNewWorker, false)
	t.Setenv("GITLAB_API_BASE", "https://gitlab.example.com/api/v4")
	old := httpClient
	httpClient = &http.Client{Transport: &failSelfCheckTransport{inner: &rewriteTransport{orig: srv.URL}}}
	t.Cleanup(func() { httpClient = old })

	rc, stdout, stderr := runCap(t, []string{"--forge", "gitlab", "worker"})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("transport-failure rc = %d, want 6; stderr: %s", rc, stderr)
	}
	if strings.Contains(stdout, tokPath) {
		t.Fatalf("path must NOT be printed as good when the self-check could not be made; stdout: %s", stdout)
	}
	if !strings.Contains(stderr, "self-check for role worker could not be completed") {
		t.Fatalf("expected a could-not-complete verdict; got: %s", stderr)
	}
	if fx.checksNew != 0 || fx.checksOld != 0 {
		t.Fatalf("no read should have reached the fixture (the transport failed): new=%d old=%d", fx.checksNew, fx.checksOld)
	}
	assertNoTokenLeak(t, stdout+stderr)
}

// failSelfCheckTransport forwards everything to inner except the self-check read, which fails
// at the transport — the forge went away between the rotate and the read.
type failSelfCheckTransport struct {
	inner http.RoundTripper
}

func (f *failSelfCheckTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Method == "GET" && strings.HasSuffix(r.URL.Path, gitlabSelfCheckPath) {
		return nil, errors.New("dial tcp: connection refused (simulated)")
	}
	return f.inner.RoundTrip(r)
}
