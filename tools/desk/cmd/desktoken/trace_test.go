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

// trace_test.go — the desktoken output contract, made self-explaining.
//
// THE FIELD FAILURE. desktoken prints a token FILE PATH on stdout and never the token, on
// purpose: a credential that reaches stdout reaches every log, transcript and prompt that
// captured it. But nothing on the output SAID so. A caller that used stdout as the
// credential — the obvious reading of a tool named desktoken — got
//
//	401 Bad credentials
//
// from its next forge call, with nothing anywhere naming the cause. The 401 is three
// processes downstream of the mistake and names neither desktoken nor the path.
//
// The fix is deliberately the SMALLEST one that closes the loop: a NOTICE on stderr, next to
// the path, saying what stdout is and how to read the value. It changes no behaviour, leaves
// stdout byte-identical (a caller that pipes stdout is untouched), and costs one line.
//
// WHAT WAS CONSIDERED AND REJECTED: a `--print-token` flag. This tool's posture is stated at
// both print sites in its own source — "Output only the token file path — never the token
// value" — and the file it writes is 0600 with a permissions sidecar. A flag that prints the
// credential on stdout would exist precisely to defeat that, and the caller it is meant to
// serve is already served by `cat "$(desktoken …)"`, which keeps the value out of the
// process's own stdout. Adding it is a security-posture change and would need a human ruling,
// not a worker's judgement; the notice needs neither.

// TestTokenPathNoticeIsPrintedOnStderrNotStdout is the fail-first pin: before this, stdout
// carried a path and NOTHING said it was a path.
func TestTokenPathNoticeIsPrintedOnStderrNotStdout(t *testing.T) {
	deskkit.ResetTrace()
	homeDir := setupTest(t)
	t.Setenv("REVIEWER_APP_ID", "12345")
	writeFileMode(t, filepath.Join(homeDir, ".config", "assay", "reviewer-app.pem"), makePEM(t), 0o600)

	const installID = "100000004"
	installs := []installationInfo{
		{ID: 100000004, Account: struct {
			Login string `json:"login"`
		}{Login: "example-org"}},
	}
	tokenPath := filepath.Join(homeDir, ".config", "assay", "reviewer-token-"+installID)
	writeTokenCache(t, tokenPath, "ghs_cached_token")
	mtime := time.Now().Add(-5 * time.Minute)
	if err := os.Chtimes(tokenPath, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	srv, _ := makeInstallTokenServer(t, installs, "ghs_should_not_be_called", "2124-01-01T00:00:00Z")
	defer srv.Close()
	oldClient := httpClient
	httpClient = &http.Client{Transport: &rewriteTransport{orig: srv.URL}}
	defer func() { httpClient = oldClient }()

	rc, stdout, stderr := runCap(t, []string{"reviewer"})
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0; stderr: %s", rc, stderr)
	}

	// STDOUT is unchanged: the path, and nothing else on it. A caller that pipes stdout into
	// a variable must see exactly what it saw before.
	if strings.TrimSpace(stdout) != tokenPath {
		t.Errorf("stdout is no longer exactly the token path:\n got %q\nwant %q",
			strings.TrimSpace(stdout), tokenPath)
	}
	if strings.Contains(stdout, "NOTICE") {
		t.Errorf("the notice was written to STDOUT, which corrupts every caller that reads it:\n%s", stdout)
	}

	// STDERR carries the notice, and the notice must be actionable: it says what stdout is,
	// names the symptom a caller will otherwise hit, and gives the exact incantation.
	for _, want := range []string{
		"NOTICE",
		"FILE PATH",
		"401",
		`cat "$(desktoken`,
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("the stderr notice is missing %q; got:\n%s", want, stderr)
		}
	}

	// And the notice must never carry the credential it is describing.
	if strings.Contains(stdout+stderr, "ghs_cached_token") {
		t.Fatalf("the token VALUE reached the output:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
}

// TestDesktokenTraceFlagIsAcceptedAndStripped asserts the global switch reaches this verb
// without its positional grammar (a bare role name) rejecting it.
func TestDesktokenTraceFlagIsAcceptedAndStripped(t *testing.T) {
	deskkit.ResetTrace()
	defer deskkit.ResetTrace()
	homeDir := setupTest(t)
	t.Setenv("REVIEWER_APP_ID", "12345")
	writeFileMode(t, filepath.Join(homeDir, ".config", "assay", "reviewer-app.pem"), makePEM(t), 0o600)

	const installID = "100000004"
	installs := []installationInfo{
		{ID: 100000004, Account: struct {
			Login string `json:"login"`
		}{Login: "example-org"}},
	}
	tokenPath := filepath.Join(homeDir, ".config", "assay", "reviewer-token-"+installID)
	writeTokenCache(t, tokenPath, "ghs_cached_token")
	mtime := time.Now().Add(-5 * time.Minute)
	if err := os.Chtimes(tokenPath, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	srv, _ := makeInstallTokenServer(t, installs, "ghs_x", "2124-01-01T00:00:00Z")
	defer srv.Close()
	oldClient := httpClient
	httpClient = &http.Client{Transport: &rewriteTransport{orig: srv.URL}}
	defer func() { httpClient = oldClient }()

	rc, stdout, stderr := runCap(t, []string{"reviewer", "--trace"})
	if rc != deskkit.ExitOK {
		t.Fatalf("--trace was not stripped before flag parsing: rc = %d; stderr: %s", rc, stderr)
	}
	if strings.TrimSpace(stdout) != tokenPath {
		t.Errorf("--trace changed stdout:\n got %q\nwant %q", strings.TrimSpace(stdout), tokenPath)
	}
	if !deskkit.TraceEnabled() {
		t.Errorf("--trace did not set the switch")
	}
}
