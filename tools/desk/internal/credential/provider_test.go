package credential_test

// provider_test.go — desktools-v2/58, Verify rows 1, 3 and 5: the provider itself, driven
// with the CLI bypassed. The adapters are deskkit's real controls (the live guard, the custody
// link and owner-only checks, the advisory lock) — the same functions the desktoken CLI and
// the in-process lookup hand in — so every refusal here holds below both callers.
//
// This is an EXTERNAL test package: it may import deskkit (which imports credential) without
// a cycle. The shipped package never does; TestCredentialImportConfinement pins that.

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/credential"
	"github.com/medici-finance/assay/tools/desk/internal/custodytest"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// --- fixtures ------------------------------------------------------------------------------

// fakeGitHub stands in for api.github.com: installation 4242 for example-org, and an
// access_tokens endpoint minting fixture-token-<n>. It records every request.
type fakeGitHub struct {
	mu        sync.Mutex
	calls     []string
	minted    int
	expiresAt string // the expires_at the next mint reports
	failMint  bool
}

func (f *fakeGitHub) RoundTrip(r *http.Request) (*http.Response, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, r.Method+" "+r.URL.Host+r.URL.Path)
	reply := func(code int, v any) (*http.Response, error) {
		b, _ := json.Marshal(v)
		return &http.Response{StatusCode: code, Header: http.Header{"Content-Type": {"application/json"}},
			Body: io.NopCloser(bytes.NewReader(b)), Request: r}, nil
	}
	if r.URL.Host != "api.github.com" || !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
		return reply(400, map[string]string{"message": "fixture refuses: wrong host or no App JWT"})
	}
	switch {
	case r.Method == "GET" && r.URL.Path == "/app/installations":
		return reply(200, []map[string]any{{"id": 4242, "account": map[string]string{"login": "example-org"}}})
	case r.Method == "POST" && r.URL.Path == "/app/installations/4242/access_tokens":
		if f.failMint {
			return reply(500, map[string]string{"message": "fixture: exchange down"})
		}
		f.minted++
		return reply(201, map[string]any{
			"token":       fmt.Sprintf("fixture-token-%d", f.minted),
			"expires_at":  f.expiresAt,
			"permissions": map[string]string{"contents": "write"},
		})
	}
	return reply(404, map[string]string{"message": "Not Found"})
}

func (f *fakeGitHub) snapshot() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

// recorder collects the provider's audit records.
type recorder struct {
	mu   sync.Mutex
	recs []credential.Record
}

func (r *recorder) audit(rec credential.Record) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recs = append(r.recs, rec)
}

func (r *recorder) snapshot() []credential.Record {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]credential.Record(nil), r.recs...)
}

// deskEnv points the desk runtime at a private temp home with the kill switch disarmed and
// returns the App-credential directory.
func deskEnv(t *testing.T) string {
	t.Helper()
	home := custodytest.PrivateTempDir(t)
	cfg := filepath.Join(home, "cfg")
	if err := os.MkdirAll(cfg, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv(deskkit.EnvConfigHome, cfg)
	t.Setenv("DESK_TOOLS_DISABLED", "")
	t.Setenv("CLAUDE_SESSION_ID", "test-session")
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
	return cfg
}

func writePEM(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	b := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err := os.WriteFile(path, b, mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func githubReq(cfg string) credential.GitHubRequest {
	return credential.GitHubRequest{
		Host:    credential.GitHubHost,
		Role:    "worker",
		AppName: "worker-app",
		AppID:   "12345",
		Owner:   "example-org",
		PEMPath: filepath.Join(cfg, "worker-app.pem"),
		Paths:   credential.Paths{Dirs: []string{cfg}, Knob: deskkit.EnvConfigHome, Doc: deskkit.AppProvisioningDoc},
		Scope:   credential.ScopeInstallation,
	}
}

func classOf(err error) credential.Class {
	var ce *credential.Error
	if errors.As(err, &ce) {
		return ce.Class
	}
	return 0
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(b))
}

// --- Verify row 1 ----------------------------------------------------------------------------

// regression: desktools-v2/58 — the mint moved out of cmd/desktoken into the provider; the
// provider must keep the CLI's on-disk cache byte-for-byte (one cache both callers share),
// reuse it with no network call and no record, report the ACTUAL expiry (the forge's own, or
// the conservative bound for a cache the old CLI wrote), and keep --fresh exact.
func TestProviderCLICacheParity(t *testing.T) {
	cfg := deskEnv(t)
	writePEM(t, filepath.Join(cfg, "worker-app.pem"), 0o600)
	forgeExpiry := time.Now().Add(57 * time.Minute).UTC().Truncate(time.Second)
	fake := &fakeGitHub{expiresAt: forgeExpiry.Format(time.RFC3339)}
	rec := &recorder{}
	ad := deskkit.CredentialAdapters(rec.audit, &http.Client{Transport: fake})
	cache := filepath.Join(cfg, "worker-token-4242")

	// 1. Cold mint: the CLI's layout — <role>-token-<install> 0600 at the head of the search
	//    path, with the .owner, .perms and (new) .expires sidecars and the install-id cache.
	cred, err := credential.GitHub(githubReq(cfg), ad)
	if err != nil {
		t.Fatalf("cold mint: %v", err)
	}
	if cred.Outcome != credential.OutcomeMinted || cred.Path != cache || cred.InstallID != "4242" {
		t.Fatalf("cold mint = %+v", cred)
	}
	if cred.Secret.Reveal() != "fixture-token-1" || readFile(t, cache) != "fixture-token-1" {
		t.Fatal("the minted token is not the value cached at the CLI's path")
	}
	if fi, _ := os.Lstat(cache); fi == nil || fi.Mode().Perm() != 0o600 || !fi.Mode().IsRegular() {
		t.Fatalf("cache %s is not a 0600 regular file", cache)
	}
	if !cred.ExpiresAt.Equal(forgeExpiry) {
		t.Fatalf("minted ExpiresAt = %s, want the forge's own %s", cred.ExpiresAt, forgeExpiry)
	}
	if got := readFile(t, credential.ExpirySidecarPath(cache)); got != forgeExpiry.Format(time.RFC3339) {
		t.Fatalf(".expires sidecar = %q", got)
	}
	if !credential.SidecarMatches(cache, "worker-app", "example-org") {
		t.Fatal("the mint did not record the .owner sidecar the CLI's probe reads")
	}
	if got := readFile(t, credential.PermsPath(cache)); got != `{"contents":"write"}` {
		t.Fatalf(".perms sidecar = %q", got)
	}
	if id, ok := credential.ReadInstallIDCache(githubReq(cfg).Paths, time.Now(), "worker-app", "example-org"); !ok || id != "4242" {
		t.Fatalf("install-id cache = %q %v, want 4242", id, ok)
	}
	if recs := rec.snapshot(); len(recs) != 1 || recs[0].Verb != "mint" || recs[0].Err != nil ||
		!strings.Contains(recs[0].Detail, "minted new worker token [app=worker-app install 4242]") {
		t.Fatalf("a mint must write exactly one mint record, got %+v", recs)
	}

	// 2. Reuse: no forge call, no record, the RECORDED expiry handed back.
	calls := len(fake.snapshot())
	cred, err = credential.GitHub(githubReq(cfg), ad)
	if err != nil || cred.Outcome != credential.OutcomeReused || cred.Secret.Reveal() != "fixture-token-1" {
		t.Fatalf("reuse = %+v %v", cred, err)
	}
	if !cred.ExpiresAt.Equal(forgeExpiry) {
		t.Fatalf("reused ExpiresAt = %s, want the recorded %s", cred.ExpiresAt, forgeExpiry)
	}
	if len(fake.snapshot()) != calls || len(rec.snapshot()) != 1 {
		t.Fatalf("a reuse contacted the forge or wrote a record: calls %v records %+v", fake.snapshot(), rec.snapshot())
	}

	// 3. A cache the PRE-PROVIDER CLI wrote — token + .owner, no .expires — is reused as-is,
	//    reported at the conservative bound mtime + ReuseWindow, never a guessed lifetime.
	if err := os.Remove(credential.ExpirySidecarPath(cache)); err != nil {
		t.Fatal(err)
	}
	mtime := time.Now().Add(-20 * time.Minute).Truncate(time.Second)
	if err := os.Chtimes(cache, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	cred, err = credential.GitHub(githubReq(cfg), ad)
	if err != nil || cred.Outcome != credential.OutcomeReused {
		t.Fatalf("legacy-cache reuse = %+v %v", cred, err)
	}
	if want := mtime.Add(credential.ReuseWindow); !cred.ExpiresAt.Equal(want) {
		t.Fatalf("legacy ExpiresAt = %s, want the conservative bound %s", cred.ExpiresAt, want)
	}
	if exp, recorded, cerr := credential.CacheExpiry(cache); cerr != nil || recorded || !exp.Equal(mtime.Add(credential.ReuseWindow)) {
		t.Fatalf("CacheExpiry(legacy) = %s %v %v, want the bound, not recorded", exp, recorded, cerr)
	}

	// 4. A RECORDED expiry already past forces a mint, inside the reuse window.
	past := time.Now().Add(-time.Minute).UTC().Format(time.RFC3339) + "\n"
	if err := os.WriteFile(credential.ExpirySidecarPath(cache), []byte(past), 0o600); err != nil {
		t.Fatal(err)
	}
	cred, err = credential.GitHub(githubReq(cfg), ad)
	if err != nil || cred.Outcome != credential.OutcomeMinted || cred.Secret.Reveal() != "fixture-token-2" {
		t.Fatalf("a past recorded expiry was reused: %+v %v", cred, err)
	}

	// 5. An aged cache (past the 50-minute window) re-mints even with a future expiry.
	old := time.Now().Add(-credential.ReuseWindow - time.Minute)
	if err := os.Chtimes(cache, old, old); err != nil {
		t.Fatal(err)
	}
	cred, err = credential.GitHub(githubReq(cfg), ad)
	if err != nil || cred.Outcome != credential.OutcomeMinted || cred.Secret.Reveal() != "fixture-token-3" {
		t.Fatalf("an aged cache was reused: %+v %v", cred, err)
	}

	// 6. An EMPTY cache is not a credential: replaced, never handed out.
	if err := os.WriteFile(cache, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	cred, err = credential.GitHub(githubReq(cfg), ad)
	if err != nil || cred.Outcome != credential.OutcomeMinted || cred.Secret.Reveal() != "fixture-token-4" {
		t.Fatalf("an empty cache was reused: %+v %v", cred, err)
	}

	// 7. --fresh is exact: a cache well inside the window is dropped with every sidecar and
	//    replaced, and the install-id caches are bypassed (one installations call).
	before := len(fake.snapshot())
	fresh := githubReq(cfg)
	fresh.Fresh = true
	cred, err = credential.GitHub(fresh, ad)
	if err != nil || cred.Outcome != credential.OutcomeMinted || cred.Secret.Reveal() != "fixture-token-5" {
		t.Fatalf("--fresh = %+v %v", cred, err)
	}
	if got := strings.Join(fake.snapshot()[before:], ","); got !=
		"GET api.github.com/app/installations,POST api.github.com/app/installations/4242/access_tokens" {
		t.Fatalf("--fresh forge calls = %s, want a resolution and one exchange", got)
	}

	// 8. Path-only (--ttl): the path, nothing read, minted or recorded.
	before, recsBefore := len(fake.snapshot()), len(rec.snapshot())
	po := githubReq(cfg)
	po.PathOnly = true
	cred, err = credential.GitHub(po, ad)
	if err != nil || cred.Outcome != credential.OutcomePathOnly || cred.Path != cache || !cred.Secret.Empty() {
		t.Fatalf("path-only = %+v %v", cred, err)
	}
	if len(fake.snapshot()) != before || len(rec.snapshot()) != recsBefore {
		t.Fatal("a path-only lookup contacted the forge or wrote a record")
	}

	// Every act above wrote one record; no reuse or path-only call wrote any.
	mints := 0
	for _, r := range rec.snapshot() {
		if r.Verb == "mint" && r.Err == nil {
			mints++
		}
	}
	if mints != 5 || len(rec.snapshot()) != 5 {
		t.Fatalf("records = %+v, want exactly the five mints", rec.snapshot())
	}
	// The value never escapes through formatting.
	if s := fmt.Sprintf("%v %+v %#v %s", cred.Secret, cred.Secret, cred.Secret, cred.Secret); strings.Contains(s, "fixture-token") {
		t.Fatalf("a formatted Secret leaked its value: %s", s)
	}
	if _, merr := json.Marshal(cred); merr == nil {
		t.Fatal("a Credential serialised — the Secret's encoder must refuse")
	}
}

// --- Verify row 3 ----------------------------------------------------------------------------

// regression: desktools-v2/58 — with the CLI bypassed the provider is the only thing between
// a caller and an App token; each admission and custody check must refuse on its own, before
// any key is read or the forge is contacted, with no fallback to another identity.
func TestProviderLowerCustodyBoundary(t *testing.T) {
	type tc struct {
		name  string
		setup func(t *testing.T, cfg string, req *credential.GitHubRequest, ad *credential.Adapters)
		class credential.Class
		want  string // a fragment of the refusal
		// noRecord: the refusal is the guard's own, so the provider writes none.
		noRecord bool
	}
	cases := []tc{
		{name: "wrong host", class: credential.ClassRefused, want: `host "gitlab.com" is not github.com`,
			setup: func(_ *testing.T, _ string, r *credential.GitHubRequest, _ *credential.Adapters) {
				r.Host = "gitlab.com"
			}},
		{name: "empty host", class: credential.ClassRefused, want: `host "" is not github.com`,
			setup: func(_ *testing.T, _ string, r *credential.GitHubRequest, _ *credential.Adapters) { r.Host = "" }},
		{name: "unknown role", class: credential.ClassRefused, want: "unknown role intruder",
			setup: func(_ *testing.T, _ string, r *credential.GitHubRequest, _ *credential.Adapters) { r.Role = "intruder" }},
		{name: "path-shaped role", class: credential.ClassRefused, want: "unknown role ../worker",
			setup: func(_ *testing.T, _ string, r *credential.GitHubRequest, _ *credential.Adapters) {
				r.Role = "../worker"
			}},
		{name: "path-shaped App name", class: credential.ClassRefused, want: "is not a valid App binding",
			setup: func(_ *testing.T, _ string, r *credential.GitHubRequest, _ *credential.Adapters) {
				r.AppName = "../worker-app"
			}},
		{name: "no App ID", class: credential.ClassRefused, want: "no App ID",
			setup: func(_ *testing.T, _ string, r *credential.GitHubRequest, _ *credential.Adapters) { r.AppID = " " }},
		{name: "non-digit installation", class: credential.ClassRefused, want: "is not an installation id",
			setup: func(_ *testing.T, _ string, r *credential.GitHubRequest, _ *credential.Adapters) {
				r.InstallID = "4242/../1"
			}},
		{name: "no account and no installation", class: credential.ClassRefused, want: "no account named",
			setup: func(_ *testing.T, _ string, r *credential.GitHubRequest, _ *credential.Adapters) { r.Owner = "" }},
		{name: "narrowed scope", class: credential.ClassRefused, want: `scope "repository" is not admitted`,
			setup: func(_ *testing.T, _ string, r *credential.GitHubRequest, _ *credential.Adapters) {
				r.Scope = "repository"
			}},
		{name: "missing private key", class: credential.ClassUnverifiable, want: "private key not found",
			setup: func(t *testing.T, cfg string, _ *credential.GitHubRequest, _ *credential.Adapters) {
				if err := os.Remove(filepath.Join(cfg, "worker-app.pem")); err != nil {
					t.Fatal(err)
				}
			}},
		{name: "loose private key", class: credential.ClassUnverifiable, want: "worker-app.pem",
			setup: func(t *testing.T, cfg string, _ *credential.GitHubRequest, _ *credential.Adapters) {
				if err := os.Chmod(filepath.Join(cfg, "worker-app.pem"), 0o644); err != nil {
					t.Fatal(err)
				}
			}},
		{name: "loose token cache", class: credential.ClassUnverifiable, want: "must be 0600",
			setup: func(t *testing.T, cfg string, r *credential.GitHubRequest, _ *credential.Adapters) {
				r.InstallID = "4242"
				p := filepath.Join(cfg, "worker-token-4242")
				if err := os.WriteFile(p, []byte("planted"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(p, 0o644); err != nil {
					t.Fatal(err)
				}
			}},
		{name: "symlinked token cache", class: credential.ClassUnverifiable, want: "worker-token-4242",
			setup: func(t *testing.T, cfg string, r *credential.GitHubRequest, _ *credential.Adapters) {
				r.InstallID = "4242"
				other := filepath.Join(cfg, "reviewer-token-4242")
				if err := os.WriteFile(other, []byte("another role's token"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(other, filepath.Join(cfg, "worker-token-4242")); err != nil {
					t.Skipf("symlinks unavailable here: %v — could-not-check", err)
				}
			}},
		{name: "refusing guard", class: 0, want: "fixture guard", noRecord: true,
			setup: func(_ *testing.T, _ string, _ *credential.GitHubRequest, a *credential.Adapters) {
				a.Guard = func() error { return errors.New("fixture guard: STOP armed") }
			}},
		{name: "live STOP flag", class: 0, want: "STOP", noRecord: true,
			setup: func(t *testing.T, cfg string, _ *credential.GitHubRequest, _ *credential.Adapters) {
				// The kill-switch flags live in the desk state dir, <home>/.config/assay.
				state := filepath.Join(filepath.Dir(cfg), ".config", "assay")
				if err := os.MkdirAll(state, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(state, "STOP"), []byte("boundary test\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}},
		{name: "no Guard adapter", class: credential.ClassRefused, want: "the Guard adapter is not wired",
			setup: func(_ *testing.T, _ string, _ *credential.GitHubRequest, a *credential.Adapters) { a.Guard = nil }},
		{name: "no Custody adapter", class: credential.ClassRefused, want: "the Custody adapter is not wired",
			setup: func(_ *testing.T, _ string, _ *credential.GitHubRequest, a *credential.Adapters) {
				a.Custody.OwnerOnly = nil
			}},
		{name: "no HTTP adapter", class: credential.ClassRefused, want: "the HTTP adapter is not wired",
			setup: func(_ *testing.T, _ string, _ *credential.GitHubRequest, a *credential.Adapters) { a.HTTP = nil }},
		{name: "no clock", class: credential.ClassRefused, want: "the Now adapter is not wired",
			setup: func(_ *testing.T, _ string, _ *credential.GitHubRequest, a *credential.Adapters) { a.Now = nil }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := deskEnv(t)
			writePEM(t, filepath.Join(cfg, "worker-app.pem"), 0o600)
			fake := &fakeGitHub{expiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}
			rec := &recorder{}
			ad := deskkit.CredentialAdapters(rec.audit, &http.Client{Transport: fake})
			req := githubReq(cfg)
			c.setup(t, cfg, &req, &ad)

			cred, err := credential.GitHub(req, ad)
			if err == nil {
				t.Fatalf("admitted: %+v", cred)
			}
			if !cred.Secret.Empty() || cred.Path != "" {
				t.Fatalf("a refusal returned a credential: %+v", cred)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("refusal %q does not name %q", err, c.want)
			}
			if got := classOf(err); got != c.class {
				t.Fatalf("refusal class = %d, want %d (%v)", got, c.class, err)
			}
			for _, call := range fake.snapshot() {
				if strings.HasSuffix(call, "/access_tokens") {
					t.Fatalf("a refused request reached the token exchange: %v", fake.snapshot())
				}
			}
			if strings.Contains(c.name, "host") || strings.Contains(c.name, "guard") || strings.Contains(c.name, "STOP") ||
				strings.Contains(c.name, "adapter") || strings.Contains(c.name, "clock") {
				if calls := fake.snapshot(); len(calls) != 0 {
					t.Fatalf("a refusal before admission contacted the forge: %v", calls)
				}
			}
			recs := rec.snapshot()
			switch {
			case c.noRecord && len(recs) != 0:
				t.Fatalf("a guard refusal is the guard's own row; the provider wrote %+v", recs)
			case !c.noRecord && (len(recs) != 1 || recs[0].Err == nil || recs[0].Verb != "mint"):
				t.Fatalf("a refusal past the guard must write exactly one failing mint record, got %+v", recs)
			}
			if fi, lerr := os.Lstat(filepath.Join(cfg, "worker-token-4242")); lerr == nil && fi.Mode().IsRegular() {
				if b, _ := os.ReadFile(filepath.Join(cfg, "worker-token-4242")); strings.HasPrefix(string(b), "fixture-token") {
					t.Fatal("a refused request left a minted token in custody")
				}
			}
		})
	}

	// The allowed case: the same environment, untouched, mints.
	t.Run("allowed", func(t *testing.T) {
		cfg := deskEnv(t)
		writePEM(t, filepath.Join(cfg, "worker-app.pem"), 0o600)
		fake := &fakeGitHub{expiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}
		rec := &recorder{}
		cred, err := credential.GitHub(githubReq(cfg), deskkit.CredentialAdapters(rec.audit, &http.Client{Transport: fake}))
		if err != nil || cred.Outcome != credential.OutcomeMinted || cred.Secret.Reveal() != "fixture-token-1" {
			t.Fatalf("the admitted request was refused: %+v %v", cred, err)
		}
		if len(rec.snapshot()) != 1 {
			t.Fatalf("records = %+v", rec.snapshot())
		}
	})
}

// --- Verify row 5 ----------------------------------------------------------------------------

// fakeGitLab is a GitLab REST v4 stand-in: one live PAT at a time. A rotation presented with
// anything but the live token is refused 401 — exactly what an overlapping, unserialised
// rotation runs into — and /user answers 200 only for the live token.
type fakeGitLab struct {
	mu       sync.Mutex
	live     string
	rotates  int
	checks   int
	inFlight int
	overlap  bool
}

func (g *fakeGitLab) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v4/personal_access_tokens/self/rotate", func(w http.ResponseWriter, r *http.Request) {
		g.mu.Lock()
		g.inFlight++
		if g.inFlight > 1 {
			g.overlap = true
		}
		presented, live := r.Header.Get("PRIVATE-TOKEN"), g.live
		g.mu.Unlock()
		time.Sleep(40 * time.Millisecond) // widen the window an unserialised peer would race into
		g.mu.Lock()
		defer g.mu.Unlock()
		g.inFlight--
		if presented != live || presented != g.live {
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"message":"401 Unauthorized"}`))
			return
		}
		g.rotates++
		g.live = fmt.Sprintf("example-placeholder-rot-%04d", g.rotates)
		_ = json.NewEncoder(w).Encode(map[string]any{"token": g.live, "expires_at": "2026-12-31", "active": true})
	})
	mux.HandleFunc("GET /api/v4/user", func(w http.ResponseWriter, r *http.Request) {
		g.mu.Lock()
		defer g.mu.Unlock()
		g.checks++
		if r.Header.Get("PRIVATE-TOKEN") != g.live {
			w.WriteHeader(401)
			return
		}
		_, _ = w.Write([]byte(`{"id":1}`))
	})
	return mux
}

func (g *fakeGitLab) counts() (rotates, checks int, overlap bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.rotates, g.checks, g.overlap
}

// regression: desktools-v2/58 — GitLab custody moved into the provider; a read-only lookup
// must never rotate (it would spend the role's only live PAT), concurrent rotations must stay
// serialised by the per-role lock and each self-checked, and a custody link to another role's
// file must be refused for both reads and rotations.
func TestProviderGitLabReadRotationParity(t *testing.T) {
	cfg := deskEnv(t)
	gl := &fakeGitLab{live: "example-placeholder-cur-0000"}
	srv := httptest.NewServer(gl.handler())
	defer srv.Close()
	base := srv.URL + "/api/v4"
	custody := filepath.Join(cfg, credential.GitLabTokenFileName("worker"))
	if err := os.WriteFile(custody, []byte(gl.live+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rec := &recorder{}
	ad := deskkit.CredentialAdapters(rec.audit, srv.Client())
	paths := credential.Paths{Dirs: []string{cfg}, Knob: deskkit.EnvConfigHome, Doc: deskkit.AppProvisioningDoc}
	req := func(role string, rotate bool) credential.GitLabRequest {
		return credential.GitLabRequest{Role: role, Paths: paths, APIBase: base, Rotate: rotate, LockWait: 20 * time.Second}
	}

	// 1. Read-only lookup: the custody value, zero network contact, a custody-read record —
	//    even with the API base set, and even when the Lock adapter is absent.
	noLock := ad
	noLock.Lock = credential.Locker{}
	for i := 0; i < 3; i++ {
		cred, err := credential.GitLab(req("worker", false), noLock)
		if err != nil || cred.Outcome != credential.OutcomeCustodyRead || cred.Path != custody ||
			cred.Secret.Reveal() != "example-placeholder-cur-0000" {
			t.Fatalf("read-only lookup = %+v %v", cred, err)
		}
	}
	if r, c, _ := gl.counts(); r != 0 || c != 0 {
		t.Fatalf("a read-only lookup contacted the forge: %d rotations, %d self-checks", r, c)
	}
	if recs := rec.snapshot(); len(recs) != 3 || recs[0].Verb != "custody-read" || recs[0].Err != nil {
		t.Fatalf("read-only records = %+v, want one custody-read each", recs)
	}

	// 2. A rotation without the Lock adapter is refused before any contact: unserialised
	//    rotation is never run.
	if _, err := credential.GitLab(req("worker", true), noLock); err == nil || !strings.Contains(err.Error(), "the Lock adapter is not wired") {
		t.Fatalf("an unserialisable rotation was not refused: %v", err)
	}
	// 3. A rotation with no API base is refused before the PAT is transmitted anywhere.
	nb := req("worker", true)
	nb.APIBase = ""
	if _, err := credential.GitLab(nb, ad); err == nil || !strings.Contains(err.Error(), "GITLAB_API_BASE is not set") {
		t.Fatalf("a base-less rotation was not refused: %v", err)
	}
	if r, c, _ := gl.counts(); r != 0 || c != 0 {
		t.Fatal("a refused rotation contacted the forge")
	}

	// 4. Concurrent rotations: every one serialised (no overlap at the forge), each presenting
	//    the value its predecessor persisted, each self-checked; custody ends on the last.
	const n = 4
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cred, err := credential.GitLab(req("worker", true), ad)
			if err == nil && cred.Outcome != credential.OutcomeRotated {
				err = fmt.Errorf("outcome %s", cred.Outcome)
			}
			errs[i] = err
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent rotation %d failed: %v", i, err)
		}
	}
	rot, chk, overlap := gl.counts()
	if overlap {
		t.Fatal("two rotations were in flight at the forge at once — the per-role lock did not serialise them")
	}
	if rot != n || chk != n {
		t.Fatalf("rotations %d, self-checks %d; want %d of each", rot, chk, n)
	}
	if got := readFile(t, custody); got != fmt.Sprintf("example-placeholder-rot-%04d", n) {
		t.Fatalf("custody holds %q after %d rotations, want the last successor", got, n)
	}
	rotRecs := 0
	for _, r := range rec.snapshot() {
		if r.Verb == "rotate" && r.Err == nil {
			rotRecs++
		}
	}
	if rotRecs != n {
		t.Fatalf("records %+v: want one rotate record per rotation (%d)", rec.snapshot(), n)
	}

	// 5. A peer holding the lock: a rotation that cannot take it is refused, not run.
	lockPath := credential.GitLabRotateLockPath(custody)
	lf, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := deskkit.TryLockExclusive(lf); err != nil {
		t.Fatalf("hold the lock: %v", err)
	}
	busy := req("worker", true)
	busy.LockWait = 0
	if _, err := credential.GitLab(busy, ad); err == nil || !strings.Contains(err.Error(), "REFUSING to start a second") {
		t.Fatalf("a rotation ran while a peer held the lock: %v", err)
	}
	_ = deskkit.UnlockFile(lf)
	_ = lf.Close()
	if r, _, _ := gl.counts(); r != n {
		t.Fatal("a lock-refused rotation reached the forge")
	}

	// 6. Wrong-role custody: worker's custody path linked to reviewer's file is refused for a
	//    read and for a rotation, before any contact.
	reviewerFile := filepath.Join(cfg, credential.GitLabTokenFileName("reviewer"))
	if err := os.WriteFile(reviewerFile, []byte("example-placeholder-rev-0000\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(custody); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Base(reviewerFile), custody); err != nil {
		t.Skipf("symlinks unavailable here: %v — could-not-check", err)
	}
	for _, rotate := range []bool{false, true} {
		cred, err := credential.GitLab(req("worker", rotate), ad)
		if err == nil || !cred.Secret.Empty() || !strings.Contains(err.Error(), "another role's custody file") {
			t.Fatalf("rotate=%v: a link to another role's custody was not refused: %+v %v", rotate, cred, err)
		}
	}
	if r, _, _ := gl.counts(); r != n {
		t.Fatal("a wrong-role custody request reached the forge")
	}
	if got := readFile(t, reviewerFile); got != "example-placeholder-rev-0000" {
		t.Fatalf("the reviewer's custody was changed through the worker's link: %q", got)
	}

	// 7. The release-runner credential (a trigger token) is never rotated.
	if _, err := credential.GitLab(req("release-runner", true), ad); err == nil || !strings.Contains(err.Error(), "pipeline trigger token") {
		t.Fatalf("a release-runner rotation was not refused: %v", err)
	}
}
