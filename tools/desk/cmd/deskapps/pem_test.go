package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

	// Deliberately not PEM-armor-shaped (no dashed BEGIN/END header line): the property under
	// test is byte-for-byte custody of whatever the conversion's `pem` field carries, which
	// an opaque fake string proves just as well, without a repo-wide secret scanner mistaking
	// test fixture text for a real key.
	const wantPEM = "SOME-DETERMINISTIC-TEST-BYTES-a19f7c04e8"
	fake := fakeConversionServer(t, ownedBy("example", conversionResult{ID: 7, ClientID: "cid", WebhookSecret: "whs", PEM: wantPEM}))
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

// TestPemNeverWrittenOnOrgOwnerMismatch — S-1. The org-owned path (the default) must also
// refuse a conversion whose owner.login is not the `--org` the operator named: a callback
// carrying a valid state and a FOREIGN App's code writes NOTHING to the credential plane (no
// PEM, no apps.env record) and re-arms the row. Before the fix this test fails — the owner
// check was gated on ownerKind=="me", so the org path wrote the foreign App's key outright.
func TestPemNeverWrittenOnOrgOwnerMismatch(t *testing.T) {
	setupTest(t)
	specs, err := TierManifests("team", "example")
	if err != nil {
		t.Fatal(err)
	}
	actSpec := specFor(specs, "example-act")
	sf, nonce := plantPendingRow(t, actSpec, "team")

	// A conversion result for an App owned by some OTHER org than --org (example).
	fake := fakeConversionServer(t, conversionResult{ID: 9, ClientID: "cid", WebhookSecret: "whs", PEM: "PEMBYTES-foreign", Owner: struct {
		Login string `json:"login"`
	}{Login: "attacker-org"}})
	withFakeGitHubAPI(t, fake)

	// ownerKind "org", org "example" — the operator named org "example", GitHub reports the
	// conversion owner as "attacker-org".
	srv := newServer(41873, "team", "example", "example", "org", specs, sf)
	srv.out = &bytes.Buffer{}
	ts := httptest.NewServer(srv.mux())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/callback?code=abc&state=" + nonce)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if _, err := readPEMFor(t, "example-act"); err == nil {
		t.Fatal("a PEM was written despite an org-owner mismatch (foreign App's code on the org path)")
	}
	if content := readAppsEnv(); strings.Contains(content, "EXAMPLE_ACT") {
		t.Fatalf("apps.env carries a record for a foreign App:\n%s", content)
	}
	if got := sf.rowByApp("example-act").State; got != StatePending {
		t.Fatalf("row state = %s, want pending (re-armed) after an org-owner mismatch", got)
	}
}

// TestOwnerCheckFailsClosedOnEmptyOwner — S-7. The owner check is the independent second
// layer behind the state nonce, so an EMPTY owner on either side must be refused like a
// mismatch, never skipped: a conversion response with no owner.login (org path), and a gh
// login that came back empty (personal path). Each case writes no PEM, no apps.env record,
// and re-arms the row.
func TestOwnerCheckFailsClosedOnEmptyOwner(t *testing.T) {
	cases := []struct {
		name      string
		ownerKind string
		org       string
		identity  string
		convOwner string
	}{
		{name: "org path, conversion carries no owner.login", ownerKind: "org", org: "example", convOwner: ""},
		{name: "personal path, gh login empty", ownerKind: "me", identity: "", convOwner: "someone"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupTest(t)
			specs, err := TierManifests("team", "example")
			if err != nil {
				t.Fatal(err)
			}
			actSpec := specFor(specs, "example-act")
			sf, nonce := plantPendingRow(t, actSpec, "team")

			fake := fakeConversionServer(t, ownedBy(tc.convOwner, conversionResult{ID: 5, ClientID: "cid", WebhookSecret: "whs", PEM: "PEMBYTES-unowned"}))
			withFakeGitHubAPI(t, fake)

			srv := newServer(41873, "team", "example", tc.org, tc.ownerKind, specs, sf)
			srv.identity = ghUser{Login: tc.identity}
			srv.out = &bytes.Buffer{}
			ts := httptest.NewServer(srv.mux())
			defer ts.Close()

			resp, err := http.Get(ts.URL + "/callback?code=abc&state=" + nonce)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()

			if _, err := readPEMFor(t, "example-act"); err == nil {
				t.Fatal("a PEM was written although one side of the owner check was empty (fail-open)")
			}
			if content := readAppsEnv(); strings.Contains(content, "EXAMPLE_ACT") {
				t.Fatalf("apps.env carries a record although the owner check could not run:\n%s", content)
			}
			if got := sf.rowByApp("example-act").State; got != StatePending {
				t.Fatalf("row state = %s, want pending (re-armed) on an empty owner", got)
			}
		})
	}
}

// TestPemModeChmodsPreexistingFile — S-4. os.WriteFile applies its perm argument only on
// create, so a PEM the callback rewrites in place must be Chmod'd back to 0600 even when it
// already existed at a looser mode. Pre-create the target 0644, run the callback, assert 0600.
func TestPemModeChmodsPreexistingFile(t *testing.T) {
	setupTest(t)
	specs, err := TierManifests("team", "example")
	if err != nil {
		t.Fatal(err)
	}
	actSpec := specFor(specs, "example-act")
	sf, nonce := plantPendingRow(t, actSpec, "team")

	// Pre-create the PEM at a deliberately looser 0644, as a hand edit or an older writer might.
	pemPath := deskkit.ConfigHomeWritePath("example-act.pem")
	if err := os.MkdirAll(filepath.Dir(pemPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pemPath, []byte("stale-0644-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	fake := fakeConversionServer(t, ownedBy("example", conversionResult{ID: 7, ClientID: "cid", WebhookSecret: "whs", PEM: "FRESH-PEM"}))
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

	fi, err := os.Stat(pemPath)
	if err != nil {
		t.Fatalf("stat pem: %v", err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("pem mode = %o, want 0600 (a pre-existing 0644 file was rewritten in place without being chmod'd)", fi.Mode().Perm())
	}
}
