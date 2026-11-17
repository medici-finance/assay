package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// roForge is a fake forge for the --read-only-out mint: it records every access_tokens body and
// every revocation, and grants whatever permission map the test sets.
type roForge struct {
	mu      sync.Mutex
	bodies  []string
	revokes int
	grant   map[string]string
}

func (f *roForge) server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /app/installations/", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.bodies = append(f.bodies, string(b))
		grant := f.grant
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token": "ghs_example_readonly", "expires_at": "2124-01-01T00:00:00Z", "permissions": grant,
		})
	})
	mux.HandleFunc("DELETE /installation/token", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.revokes++
		f.mu.Unlock()
		w.WriteHeader(204)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	old := httpClient
	httpClient = &http.Client{Transport: &rewriteTransport{orig: srv.URL}}
	t.Cleanup(func() { httpClient = old })
	return srv
}

// roSetup is a verify-desk window holding the verifier key, with a pinned install id.
func roSetup(t *testing.T) (home, outDir string) {
	t.Helper()
	home = setupTest(t)
	t.Setenv("DESK_LOOP", "verify-desk")
	t.Setenv("VERIFIER_APP_ID", "67890")
	t.Setenv("VERIFIER_INSTALL_ID", "55555555")
	writeFileMode(t, filepath.Join(home, ".config", "assay", "verifier-app.pem"), makePEM(t), 0o600)
	outDir = filepath.Join(t.TempDir(), "run")
	if err := os.Mkdir(outDir, 0o700); err != nil {
		t.Fatal(err)
	}
	return home, outDir
}

// regression: #6746
func TestReadOnlyOutMintsNarrow(t *testing.T) {
	home, dir := roSetup(t)
	f := &roForge{grant: map[string]string{"issues": "read", "metadata": "read", "pull_requests": "read"}}
	f.server(t)
	out := filepath.Join(dir, "read-credential")

	rc, stdout, stderr := runCap(t, []string{"verifier", "--repo", "example-org/tracker", "--read-only-out", out})
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0; stderr: %s", rc, stderr)
	}
	if strings.TrimSpace(stdout) != out {
		t.Errorf("stdout = %q, want exactly the out path %q", stdout, out)
	}
	if strings.Contains(stdout+stderr, "ghs_example_readonly") {
		t.Fatal("the credential value reached stdout/stderr")
	}
	fi, err := os.Stat(out)
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("out file: %v mode %v, want 0600", err, fi)
	}
	if b, _ := os.ReadFile(out); string(b) != "ghs_example_readonly" {
		t.Errorf("out holds %q", b)
	}
	if b, _ := os.ReadFile(out + ".perms"); !strings.Contains(string(b), `"pull_requests":"read"`) {
		t.Errorf(".perms = %q", b)
	}
	if len(f.bodies) != 1 {
		t.Fatalf("access_tokens calls = %d, want 1", len(f.bodies))
	}
	var body struct {
		Repositories []string          `json:"repositories"`
		Permissions  map[string]string `json:"permissions"`
	}
	if err := json.Unmarshal([]byte(f.bodies[0]), &body); err != nil {
		t.Fatalf("access_tokens body %q: %v", f.bodies[0], err)
	}
	if len(body.Repositories) != 1 || body.Repositories[0] != "tracker" {
		t.Errorf("repositories = %v, want [tracker]", body.Repositories)
	}
	for k, v := range body.Permissions {
		if v != "read" {
			t.Errorf("asked for %s=%s, want read only", k, v)
		}
	}
	if len(body.Permissions) == 0 {
		t.Error("no permissions narrowing in the request")
	}
	// Never cached: the role cache is untouched.
	if _, err := os.Stat(filepath.Join(home, ".config", "assay", "verifier-token-55555555")); !os.IsNotExist(err) {
		t.Errorf("a read-only mint wrote the role cache: %v", err)
	}
}

// regression: #6746
func TestReadOnlyOutWideGrantRefused(t *testing.T) {
	_, dir := roSetup(t)
	f := &roForge{grant: map[string]string{"metadata": "read", "pull_requests": "write"}}
	f.server(t)
	out := filepath.Join(dir, "read-credential")

	rc, stdout, stderr := runCap(t, []string{"verifier", "--repo", "example-org/tracker", "--read-only-out", out})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("rc = %d, want %d (could-not-check); stderr: %s", rc, deskkit.ExitUnverifiable, stderr)
	}
	if _, err := os.Lstat(out); !os.IsNotExist(err) {
		t.Errorf("a wider-than-read grant was written to %s", out)
	}
	if f.revokes != 1 {
		t.Errorf("revocations = %d, want 1 — a refused wider grant is ended at once", f.revokes)
	}
	if strings.Contains(stdout+stderr, "ghs_example_readonly") {
		t.Fatal("the credential value reached stdout/stderr")
	}
}

func TestReadOnlyOutFlagRefusals(t *testing.T) {
	_, dir := roSetup(t)
	f := &roForge{grant: map[string]string{"metadata": "read"}}
	f.server(t)
	out := filepath.Join(dir, "read-credential")
	existing := filepath.Join(dir, "taken")
	writeFileMode(t, existing, "x", 0o600)
	open := filepath.Join(t.TempDir(), "open")
	if err := os.Mkdir(open, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]struct {
		args []string
		rc   int
	}{
		"no repo":           {[]string{"verifier", "--read-only-out", out}, deskkit.ExitRefused},
		"owner only":        {[]string{"verifier", "--repo", "example-org", "--read-only-out", out}, deskkit.ExitRefused},
		"with ttl":          {[]string{"verifier", "--repo", "example-org/tracker", "--ttl", "--read-only-out", out}, deskkit.ExitRefused},
		"with fresh":        {[]string{"verifier", "--repo", "example-org/tracker", "--fresh", "--read-only-out", out}, deskkit.ExitRefused},
		"gitlab":            {[]string{"verifier", "--forge", "gitlab", "--repo", "example-org/tracker", "--read-only-out", out}, deskkit.ExitRefused},
		"relative path":     {[]string{"verifier", "--repo", "example-org/tracker", "--read-only-out", "rel/out"}, deskkit.ExitRefused},
		"existing path":     {[]string{"verifier", "--repo", "example-org/tracker", "--read-only-out", existing}, deskkit.ExitRefused},
		"open directory":    {[]string{"verifier", "--repo", "example-org/tracker", "--read-only-out", filepath.Join(open, "c")}, deskkit.ExitUnverifiable},
		"another loop role": {[]string{"worker", "--repo", "example-org/tracker", "--read-only-out", out}, deskkit.ExitRefused},
	} {
		t.Run(name, func(t *testing.T) {
			before := len(f.bodies)
			if rc, _, stderr := runCap(t, c.args); rc != c.rc {
				t.Fatalf("rc = %d, want %d; stderr: %s", rc, c.rc, stderr)
			}
			if len(f.bodies) != before {
				t.Error("a refused --read-only-out still contacted the forge")
			}
			if _, err := os.Lstat(out); !os.IsNotExist(err) {
				t.Error("a refused --read-only-out wrote the out path")
			}
		})
	}
}
