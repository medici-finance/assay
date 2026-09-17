package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// readPEMFor reads the PEM this package would have written for app, using the same
// resolution writePEM uses.
func readPEMFor(t *testing.T, app string) ([]byte, error) {
	t.Helper()
	return os.ReadFile(deskkit.ConfigHomeWritePath(app + ".pem"))
}

// TestPemMode — Verify row 8. The written key is mode 0600 and byte-equal to the fake
// conversion's `pem` field.
func TestPemMode(t *testing.T) {
	setupTest(t)
	specs, err := TierManifests("team", "example")
	if err != nil {
		t.Fatal(err)
	}
	actSpec := specFor(specs, "example-act")
	sf, nonce := plantPendingRow(t, actSpec, "team")

	const wantPEM = "-----BEGIN RSA PRIVATE KEY-----\nSOME-DETERMINISTIC-TEST-BYTES\n-----END RSA PRIVATE KEY-----\n"
	fake := fakeConversionServer(t, conversionResult{ID: 7, ClientID: "cid", WebhookSecret: "whs", PEM: wantPEM})
	withFakeGitHubAPI(t, fake)

	srv := newServer(41873, "team", "example", "example", "org", specs, sf)
	srv.out = &bytes.Buffer{}
	ts := httptest.NewServer(srv.mux())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/callback?code=abc&state=" + nonce)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	pemPath := deskkit.ConfigHomeWritePath("example-act.pem")
	fi, err := os.Stat(pemPath)
	if err != nil {
		t.Fatalf("stat pem: %v", err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("pem mode = %o, want 0600", fi.Mode().Perm())
	}
	got, err := os.ReadFile(pemPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != wantPEM {
		t.Fatalf("pem content mismatch:\ngot:  %q\nwant: %q", got, wantPEM)
	}
}

// TestPemNeverWrittenOnMismatch — a personal-owned callback whose conversion owner differs
// from the CLI's gh login writes NOTHING (design.md §8).
func TestPemNeverWrittenOnMismatch(t *testing.T) {
	setupTest(t)
	specs, err := TierManifests("team", "example")
	if err != nil {
		t.Fatal(err)
	}
	actSpec := specFor(specs, "example-act")
	sf, nonce := plantPendingRow(t, actSpec, "team")

	fake := fakeConversionServer(t, conversionResult{ID: 1, PEM: "PEMBYTES", Owner: struct {
		Login string `json:"login"`
	}{Login: "someone-else"}})
	withFakeGitHubAPI(t, fake)

	srv := newServer(41873, "team", "example", "example", "me", specs, sf)
	srv.identity = ghUser{Login: "the-real-operator"}
	srv.out = &bytes.Buffer{}
	ts := httptest.NewServer(srv.mux())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/callback?code=abc&state=" + nonce)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if _, err := readPEMFor(t, "example-act"); err == nil {
		t.Fatal("a PEM was written despite an identity mismatch")
	}
	if got := sf.rowByApp("example-act").State; got != StatePending {
		t.Fatalf("row state = %s, want pending (re-armed) after a mismatch", got)
	}
}
