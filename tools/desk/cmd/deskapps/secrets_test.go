package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The fake conversion's secrets — chosen to be unmistakable and never legitimately
// present anywhere else this package writes, so any test finding one of these
// substrings has found a real leak, not a coincidence.
const (
	fakePEM           = "-----BEGIN RSA PRIVATE KEY-----\nFAKESECRETPEMBODYDONOTLEAK\n-----END RSA PRIVATE KEY-----\n"
	fakeClientSecret  = "fake-client-secret-zzq7"
	fakeWebhookSecret = "fake-webhook-secret-pl4x"
)

// runKeyedCallback stands a server up with one pending row, fakes a successful conversion
// (carrying the secrets above), drives /callback, and returns the server plus the httptest
// server it's mounted on for further requests. It fails the test if the callback did not
// reach "keyed".
func runKeyedCallback(t *testing.T, out io.Writer) (*deskappsServer, *httptest.Server) {
	t.Helper()
	setupTest(t)

	specs, err := TierManifests("team", "example")
	if err != nil {
		t.Fatal(err)
	}
	actSpec := specFor(specs, "example-act")
	sf, nonce := plantPendingRow(t, actSpec, "team")

	fake := fakeConversionServer(t, conversionResult{
		ID: 99, Slug: "example-act", ClientID: "client-id-xyz",
		ClientSecret: fakeClientSecret, WebhookSecret: fakeWebhookSecret, PEM: fakePEM,
	})
	withFakeGitHubAPI(t, fake)

	srv := newServer(41873, "team", "example", "example", "org", specs, sf)
	srv.out = out
	ts := httptest.NewServer(srv.mux())
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/callback?code=abc123&state=" + nonce)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		// http.Get follows the callback's 302 to /run; a non-200 here means either leg failed.
		t.Fatalf("callback (followed to /run) status = %d, want 200 (body: %s)", resp.StatusCode, body)
	}
	if row := sf.rowByApp("example-act"); row == nil || row.State != StateKeyed {
		t.Fatalf("row did not reach keyed: %+v", sf.Apps)
	}
	return srv, ts
}

// TestNoSecretInPage — Verify row 4. Every route's served HTML contains no PEM header, no
// client secret, no webhook secret from a real (fake) conversion.
func TestNoSecretInPage(t *testing.T) {
	_, ts := runKeyedCallback(t, &bytes.Buffer{})

	forbidden := []string{fakePEM, fakeClientSecret, fakeWebhookSecret, "BEGIN RSA PRIVATE KEY", "FAKESECRETPEMBODY"}
	for _, route := range []string{"/", "/tier", "/setup", "/run"} {
		resp, err := http.Get(ts.URL + route)
		if err != nil {
			t.Fatalf("GET %s: %v", route, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		for _, f := range forbidden {
			if strings.Contains(string(body), f) {
				t.Fatalf("route %s served HTML containing secret material %q", route, f)
			}
		}
	}
}

// captureRealStdout redirects the process's REAL os.Stdout for the duration of fn and
// returns everything written to it. This is deliberately in addition to (never instead of)
// checking the server's injectable console writer: a leak introduced via a bare
// fmt.Println/log.Printf call (exactly mutations.json's second mutant) writes to the real
// os.Stdout directly and would never reach an injected io.Writer, so a test that only
// checked the injected writer could not catch it.
func captureRealStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	fn()
	os.Stdout = orig
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// TestNoSecretInLogs — Verify row 5. Neither stdout (the injected console writer AND the
// real process stdout) nor the deskkit audit line ever carries the
// PEM/client-secret/webhook-secret, but both carry app=/state= tokens for every state
// change.
func TestNoSecretInLogs(t *testing.T) {
	var console bytes.Buffer
	realStdout := captureRealStdout(t, func() {
		runKeyedCallback(t, &console)
	})

	forbidden := []string{fakePEM, fakeClientSecret, fakeWebhookSecret, "BEGIN RSA PRIVATE KEY", "FAKESECRETPEMBODY"}
	consoleOut := console.String()
	for _, f := range forbidden {
		if strings.Contains(consoleOut, f) {
			t.Fatalf("stdout (injected writer) carried secret material %q:\n%s", f, consoleOut)
		}
		if strings.Contains(realStdout, f) {
			t.Fatalf("real os.Stdout carried secret material %q:\n%s", f, realStdout)
		}
	}
	if !strings.Contains(consoleOut, "app=") || !strings.Contains(consoleOut, "state=") {
		t.Fatalf("stdout did not carry app=/state= tokens:\n%s", consoleOut)
	}

	audit, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".config", "assay", "audit.jsonl"))
	if err != nil {
		t.Fatalf("reading audit log: %v", err)
	}
	auditStr := string(audit)
	for _, f := range forbidden {
		if strings.Contains(auditStr, f) {
			t.Fatalf("audit log carried secret material %q:\n%s", f, auditStr)
		}
	}
	if !strings.Contains(auditStr, `"app=`) && !strings.Contains(auditStr, "app=") {
		t.Fatalf("audit log did not carry app=/state= tokens:\n%s", auditStr)
	}

	// Cross-check: the audit line's detail field parses and literally carries app=/state=.
	found := false
	for _, line := range strings.Split(strings.TrimSpace(auditStr), "\n") {
		var e map[string]any
		if json.Unmarshal([]byte(line), &e) != nil {
			continue
		}
		detail, _ := e["detail"].(string)
		if strings.Contains(detail, "app=example-act") && strings.Contains(detail, "state=keyed") {
			found = true
		}
	}
	if !found {
		t.Fatalf("no audit line's detail carried app=example-act state=keyed:\n%s", auditStr)
	}
}

// TestNoSecretOnConversionFailure — a conversion that fails on an unexpected status must
// not have its response body surface in the returned error (convert.go's deliberate
// omission).
func TestNoSecretOnConversionFailure(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /app-manifests/{code}/conversions", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, fakeClientSecret)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	withFakeGitHubAPI(t, srv)

	_, err := convertCode("whatever")
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), fakeClientSecret) {
		t.Fatalf("error string leaked the response body: %v", err)
	}
}
