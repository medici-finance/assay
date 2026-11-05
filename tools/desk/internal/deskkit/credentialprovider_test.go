package deskkit

// credentialprovider_test.go — desktools-v2/58, Verify row 2: the in-process lookup runs the
// credential provider as a function call, starts no minter child, and still refuses every
// identity it was not admitted for.

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/custodytest"
)

// fakeGitHub is a RoundTripper standing in for api.github.com: one installation (id 4242,
// account example-org) and an access_tokens endpoint that hands out one fixed token. It
// records every request so a test can assert exactly which forge calls were made.
type fakeGitHub struct {
	mu    sync.Mutex
	calls []string
	bad   []string
}

func (f *fakeGitHub) RoundTrip(r *http.Request) (*http.Response, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, r.Method+" "+r.URL.Path)
	if r.URL.Scheme != "https" || r.URL.Host != "api.github.com" {
		f.bad = append(f.bad, "host "+r.URL.String())
	}
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") || strings.Count(strings.TrimPrefix(auth, "Bearer "), ".") != 2 {
		f.bad = append(f.bad, "authorization is not an App JWT")
	}
	resp := func(code int, v any) (*http.Response, error) {
		b, _ := json.Marshal(v)
		return &http.Response{StatusCode: code, Header: http.Header{"Content-Type": {"application/json"}},
			Body: io.NopCloser(bytes.NewReader(b)), Request: r}, nil
	}
	switch {
	case r.Method == "GET" && r.URL.Path == "/app/installations":
		return resp(200, []map[string]any{{"id": 4242, "account": map[string]string{"login": "example-org"}}})
	case r.Method == "POST" && r.URL.Path == "/app/installations/4242/access_tokens":
		return resp(201, map[string]any{
			"token":       "fixture-provider-token",
			"expires_at":  time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
			"permissions": map[string]string{"contents": "write"},
		})
	}
	f.bad = append(f.bad, "unexpected "+r.Method+" "+r.URL.Path)
	return resp(404, map[string]string{"message": "Not Found"})
}

func (f *fakeGitHub) snapshot() (calls, bad []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...), append([]string(nil), f.bad...)
}

// regression: desktools-v2/58 — RoleTokenForOwner forked the desktoken binary found first on
// PATH for every mint; a host with no desktoken on PATH could not mint at all, and a planted
// exec fallback would reintroduce both the fork and the PATH dependency.
func TestDirectProviderHasNoMinterChild(t *testing.T) {
	stateDir := perfFixture(t)
	home := custodytest.PrivateTempDir(t)
	cfg := filepath.Join(home, "cfg")
	if err := os.MkdirAll(cfg, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv(EnvConfigHome, cfg)
	t.Setenv("PATH", t.TempDir()) // no desktoken anywhere on PATH
	for _, p := range []string{"WORKER", "REVIEWER"} {
		for _, s := range []string{"_APP_ID", "_PEM", "_TOKEN", "_INSTALL_ID", "_APP"} {
			t.Setenv(p+s, "")
		}
	}
	t.Setenv("WORKER_APP_ID", "12345")
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err := os.WriteFile(filepath.Join(cfg, "worker-app.pem"), keyPEM, 0o600); err != nil {
		t.Fatal(err)
	}

	fake := &fakeGitHub{}
	prev := credentialHTTP
	credentialHTTP = &http.Client{Transport: fake}
	t.Cleanup(func() { credentialHTTP = prev })
	resetRoleTokenMemo()
	t.Cleanup(resetRoleTokenMemo)
	perfResetCounters()
	auditBefore, _ := LoadEntries()

	// The admitted role, for the account its App is installed on: one installations call,
	// one exchange, the fake's token read back from the provider's cache.
	tok, path, err := RoleTokenForOwner("worker", "example-org")
	if err != nil {
		t.Fatalf("admitted worker lookup failed: %v", err)
	}
	if tok != "fixture-provider-token" {
		t.Fatalf("token = %q, want the fake forge's token", tok)
	}
	if want := filepath.Join(cfg, "worker-token-4242"); path != want {
		t.Fatalf("cache path = %q, want %q", path, want)
	}
	if fi, serr := os.Lstat(path); serr != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("cache %s not a 0600 file: %v", path, serr)
	}
	calls, bad := fake.snapshot()
	if len(bad) > 0 {
		t.Fatalf("the provider sent requests the fake forge refuses: %v", bad)
	}
	if got := strings.Join(calls, ","); got != "GET /app/installations,POST /app/installations/4242/access_tokens" {
		t.Fatalf("forge calls = %s", got)
	}

	// A fresh process (memo empty) is served from the provider's on-disk cache: the same
	// token and path, no forge call, no audit row.
	resetRoleTokenMemo()
	if tok2, path2, rerr := RoleTokenForOwner("worker", "example-org"); rerr != nil || tok2 != tok || path2 != path {
		t.Fatalf("cache reuse returned %q %q %v, want the cached token at %s", tok2, path2, rerr, path)
	}
	if calls, _ = fake.snapshot(); len(calls) != 2 {
		t.Fatalf("a cache reuse contacted the forge: %v", calls)
	}

	// Another identity is refused, with no fallback to the worker's credential: an account
	// the App is not installed on, a role with no App credential provisioned, and a name
	// that is not a desk role at all.
	for _, c := range []struct{ role, owner string }{
		{"worker", "other-org"},
		{"reviewer", "example-org"},
		{"intruder", "example-org"},
	} {
		got, _, rerr := RoleTokenForOwner(c.role, c.owner)
		if rerr == nil || got != "" {
			t.Fatalf("%s/%s: got token %q err %v, want a refusal", c.role, c.owner, got, rerr)
		}
	}
	calls, bad = fake.snapshot()
	if len(bad) > 0 {
		t.Fatalf("a refused identity reached the fake forge with a bad request: %v", bad)
	}
	for _, c := range calls[2:] {
		if strings.HasSuffix(c, "/access_tokens") {
			t.Fatalf("a refused identity reached the token exchange: %v", calls)
		}
	}

	children, _, _ := PerfCounters()
	if children != 0 {
		t.Fatalf("the in-process lookup started %d child process(es), want 0", children)
	}
	if got := RoleTokenMints(); got != 4 {
		t.Fatalf("RoleTokenMints() = %d, want 4 (one per lookup since the reset; failures are never memoised)", got)
	}

	// The ledger carries the mint and each refusal past the guard, as the CLI would.
	after, _ := LoadEntries()
	rows := after[len(auditBefore):]
	var mints, refusals int
	for _, e := range rows {
		if e.Tool != "desktoken" {
			continue
		}
		if e.Result == ResultOK && strings.Contains(e.Detail, "minted new worker token") {
			mints++
		} else if e.Result != ResultOK {
			refusals++
		}
	}
	if mints != 1 {
		t.Fatalf("audit rows %s: want exactly one desktoken mint row", fmt.Sprint(rows))
	}
	if refusals != 3 {
		t.Fatalf("audit rows %s: want exactly one refusal row per refused identity (3)", fmt.Sprint(rows))
	}

	// With STOP armed the lookup refuses BEFORE it builds a request, as the CLI's top-of-run
	// guard does: an unknown role writes the guard's row only, never a desktoken refusal row,
	// and an admitted role never reaches the forge.
	if err := os.WriteFile(filepath.Join(stateDir, "STOP"), []byte("provider test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	resetRoleTokenMemo()
	stoppedBefore, _ := LoadEntries()
	callsBefore, _ := fake.snapshot()
	for _, role := range []string{"intruder", "worker"} {
		got, _, rerr := RoleTokenForOwner(role, "example-org")
		if rerr == nil || got != "" || !strings.Contains(rerr.Error(), "stop flag active") {
			t.Fatalf("%s under STOP: got %q err %v, want the stop-flag refusal", role, got, rerr)
		}
	}
	stopped, _ := LoadEntries()
	for _, e := range stopped[len(stoppedBefore):] {
		if e.Tool == "desktoken" {
			t.Fatalf("a stopped lookup wrote a desktoken row %+v — the request was built past the guard", e)
		}
	}
	if callsAfter, _ := fake.snapshot(); len(callsAfter) != len(callsBefore) {
		t.Fatalf("a stopped lookup contacted the forge: %v", callsAfter[len(callsBefore):])
	}
}
