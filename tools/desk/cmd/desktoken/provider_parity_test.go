package main

// provider_parity_test.go — desktools-v2/58, Verify row 4: the desktoken CLI and the
// in-process lookup (deskkit.RoleTokenForOwner) are two adapters over ONE credential
// provider, so they apply the same guard, the same expiry and the same audit rule.

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: desktools-v2/58 — the in-process lookup forked the CLI, so the guard, expiry
// and audit rule held only as long as the binary first on PATH applied them; with the
// provider in process, both adapters must refuse a stop and an expired cache, write no row
// for a reuse, and record a mint exactly once.
func TestProviderGuardAuditParity(t *testing.T) {
	homeDir := setupTest(t)
	cfg := filepath.Join(homeDir, ".config", "assay")
	for _, role := range []string{"reviewer", "worker", "verifier"} {
		prefix := strings.ToUpper(role)
		t.Setenv(prefix+"_APP_ID", "12345")
		for _, s := range []string{"_PEM", "_TOKEN", "_INSTALL_ID", "_APP"} {
			t.Setenv(prefix+s, "")
		}
		writeFileMode(t, filepath.Join(cfg, role+"-app.pem"), makePEM(t), 0o600)
	}

	// One fake forge for both adapters. failMint turns the exchange into a 500 so a test can
	// prove an expired cache is not handed out when no replacement can be minted.
	installs := []installationInfo{{ID: 100000004, Account: struct {
		Login string `json:"login"`
	}{Login: "example-org"}}}
	srv, paths := makeInstallTokenServer(t, installs, "fixture-parity-token", time.Now().Add(time.Hour).UTC().Format(time.RFC3339))
	defer srv.Close()
	var failMint atomic.Bool
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if failMint.Load() && strings.HasSuffix(r.URL.Path, "/access_tokens") {
			return &http.Response{StatusCode: 500, Body: http.NoBody, Header: http.Header{}, Request: r}, nil
		}
		return (&rewriteTransport{orig: srv.URL}).RoundTrip(r)
	})}
	oldClient := httpClient
	httpClient = client
	defer func() { httpClient = oldClient }()
	defer deskkit.SetCredentialHTTP(client)()

	// rows counts the credential rows: the kill-switch's own transition row (written when the
	// STOP below is cleared) is the guard's, not the provider's.
	rows := func() int {
		n := 0
		for _, e := range auditEntries(t) {
			if e.Verb != "guard" {
				n++
			}
		}
		return n
	}
	tokenPath := func(role string) string { return filepath.Join(cfg, role+"-token-100000004") }

	// 1. STOP armed: both adapters refuse, neither mints, nothing is cached.
	stop := filepath.Join(cfg, "STOP")
	writeFile(t, stop, "parity test\n")
	if rc, _, _ := runCap(t, []string{"reviewer"}); rc == deskkit.ExitOK {
		t.Fatal("CLI minted under STOP")
	}
	if tok, _, err := deskkit.RoleTokenForOwner("reviewer", "example-org"); err == nil || tok != "" {
		t.Fatalf("in-process lookup under STOP returned %q %v, want a refusal", tok, err)
	}
	if len(*paths) != 0 {
		t.Fatalf("a stopped lookup reached the token exchange: %v", *paths)
	}
	if _, err := os.Lstat(tokenPath("reviewer")); !os.IsNotExist(err) {
		t.Fatalf("a stopped lookup left a token cache: %v", err)
	}
	if err := os.Remove(stop); err != nil {
		t.Fatal(err)
	}

	// 2. In-process mint: exactly one row, one exchange.
	before := rows()
	tok, path, err := deskkit.RoleTokenForOwner("reviewer", "example-org")
	if err != nil || tok != "fixture-parity-token" || path != tokenPath("reviewer") {
		t.Fatalf("in-process mint = %q %q %v", tok, path, err)
	}
	if got := rows() - before; got != 1 {
		t.Fatalf("an in-process mint wrote %d rows, want exactly 1", got)
	}
	if all := auditEntries(t); len(all) == 0 {
		t.Fatal("no audit rows after a mint")
	} else if last := all[len(all)-1]; last.Tool != "desktoken" || !strings.Contains(last.Detail, "minted new reviewer token") {
		t.Fatalf("the in-process mint's row is not a desktoken mint row: %+v", last)
	}
	if len(*paths) != 1 {
		t.Fatalf("exchanges after one mint = %d, want 1", len(*paths))
	}

	// 3. The CLI reuses the cache the in-process mint wrote: same path, no exchange, no row.
	before = rows()
	rc, stdout, stderr := runCap(t, []string{"reviewer"})
	if rc != deskkit.ExitOK || strings.TrimSpace(stdout) != tokenPath("reviewer") {
		t.Fatalf("CLI reuse rc=%d stdout=%q stderr=%s", rc, stdout, stderr)
	}
	if got := rows() - before; got != 0 || len(*paths) != 1 {
		t.Fatalf("a CLI reuse wrote %d rows and made %d exchanges, want 0 and still 1", got, len(*paths))
	}

	// 4. The CLI mints; the in-process lookup reuses that cache: one row, then none.
	before = rows()
	if rc, _, stderr := runCap(t, []string{"worker"}); rc != deskkit.ExitOK {
		t.Fatalf("CLI worker mint rc=%d stderr=%s", rc, stderr)
	}
	if got := rows() - before; got != 1 || len(*paths) != 2 {
		t.Fatalf("a CLI mint wrote %d rows (%d exchanges), want 1 (2)", got, len(*paths))
	}
	before = rows()
	if tok, path, err := deskkit.RoleTokenForOwner("worker", "example-org"); err != nil || tok != "fixture-parity-token" || path != tokenPath("worker") {
		t.Fatalf("in-process reuse = %q %q %v", tok, path, err)
	}
	if got := rows() - before; got != 0 || len(*paths) != 2 {
		t.Fatalf("an in-process reuse wrote %d rows (%d exchanges), want 0 (still 2)", got, len(*paths))
	}

	// 5. Expiry: a cache inside the 50-minute reuse window whose RECORDED expiry has passed is
	//    never handed out. With the exchange failing, both adapters refuse — one row each.
	if rc, _, stderr := runCap(t, []string{"verifier"}); rc != deskkit.ExitOK {
		t.Fatalf("CLI verifier mint rc=%d stderr=%s", rc, stderr)
	}
	past := time.Now().Add(-time.Minute).UTC().Format(time.RFC3339) + "\n"
	writeFileMode(t, tokenPath("verifier")+".expires", past, 0o600)
	failMint.Store(true)
	before = rows()
	if rc, stdout, _ := runCap(t, []string{"verifier"}); rc == deskkit.ExitOK {
		t.Fatalf("CLI handed out an expired cache: %q", stdout)
	}
	if tok, _, err := deskkit.RoleTokenForOwner("verifier", "example-org"); err == nil || tok != "" {
		t.Fatalf("in-process lookup handed out an expired cache: %q %v", tok, err)
	}
	if got := rows() - before; got != 2 {
		t.Fatalf("two expired-cache refusals wrote %d rows, want one each (2)", got)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
