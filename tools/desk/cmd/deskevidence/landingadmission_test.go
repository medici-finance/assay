package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// productionAdmission is the landing's admission binding as shipped, captured
// before any test replaces the seam.
var productionAdmission = verifierEvidenceAdmissionFn

// attestationAPI is an offline GitHub API for the dispatcher-owned attestation
// record. Only forge transport and custody are replaced; deskevidence runs the
// real shared admission reader against it.
type attestationAPI struct {
	mu                 sync.Mutex
	title, body, state string
	labels             []string
	timeline           []map[string]any
	unexpected         []string
}

func (a *attestationAPI) issue() map[string]any {
	labels := []map[string]any{}
	for _, l := range a.labels {
		labels = append(labels, map[string]any{"name": l})
	}
	return map[string]any{"number": 77, "title": a.title, "body": a.body, "state": a.state, "labels": labels,
		"html_url": "https://example.invalid/attestation/77", "user": map[string]any{"login": "assay-desk-app[bot]", "id": 300000001}}
}

func (a *attestationAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	enc := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	p := r.URL.Path
	switch {
	case r.Method == http.MethodPost && strings.HasSuffix(p, "/repos/example-org/tracker/issues"):
		var in struct{ Title, Body string }
		_ = json.NewDecoder(r.Body).Decode(&in)
		a.title, a.body, a.state = in.Title, in.Body, "open"
		w.WriteHeader(http.StatusCreated)
		enc(a.issue())
	case r.Method == http.MethodGet && strings.HasSuffix(p, "/issues/77"):
		enc(a.issue())
	case r.Method == http.MethodPatch && strings.HasSuffix(p, "/issues/77"):
		a.state = "closed"
		enc(a.issue())
	case r.Method == http.MethodPost && strings.HasSuffix(p, "/issues/77/labels"):
		var in struct {
			Labels []string `json:"labels"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		for _, l := range in.Labels {
			a.labels = append(a.labels, l)
			a.timeline = append(a.timeline, map[string]any{"event": "labeled", "label": map[string]any{"name": l}, "actor": map[string]any{"login": "assay-desk-app[bot]"}})
		}
		enc(a.issue()["labels"])
	case r.Method == http.MethodPost && strings.HasSuffix(p, "/repos/example-org/tracker/labels"):
		w.WriteHeader(http.StatusCreated)
		enc(map[string]any{})
	case r.Method == http.MethodGet && strings.HasSuffix(p, "/issues/77/timeline"):
		if r.URL.Query().Get("page") != "1" {
			enc([]any{})
			return
		}
		enc(a.timeline)
	case strings.HasSuffix(p, "/graphql"):
		enc(map[string]any{"data": map[string]any{"repository": map[string]any{"issue": map[string]any{"lastEditedAt": nil, "comments": map[string]any{"nodes": []any{}, "pageInfo": map[string]any{"hasNextPage": false}}}}}})
	default:
		a.unexpected = append(a.unexpected, r.Method+" "+p)
		w.WriteHeader(http.StatusNotFound)
	}
}

// TestVerifierEvidenceLandingFormAdmitted runs the documented desk-side landing
// through the REAL pre-work admission (no admission stub): --root names the
// dispatched verifier home, the --brief-path fragment lives outside it, and the
// refresh before an outcome record rewrites only the brief and stream index in
// the home, never its detached HEAD. The forms the procedure forbids refuse.
func TestVerifierEvidenceLandingFormAdmitted(t *testing.T) {
	if reflect.ValueOf(productionAdmission).Pointer() != reflect.ValueOf(deskkit.CheckVerifierEvidence).Pointer() {
		t.Fatal("the shipped landing does not bind the shared admission reader")
	}
	f, errBuf := setupFake(t)
	verifierEvidenceAdmissionFn = productionAdmission
	api := &attestationAPI{}
	srv := httptest.NewServer(api)
	t.Cleanup(srv.Close)
	deskkit.SetGitHubCustodyMinter(func(string, deskkit.ForgeRepo) (string, string, error) {
		return "example-installation-token", srv.URL, nil
	})
	t.Cleanup(func() { deskkit.SetGitHubCustodyMinter(nil) })

	const repo, brief, index = "example-org/tracker", "docs/streams/x/brief-01-source.md", "docs/streams/x/README.md"
	briefBody := "# Source\n\n## Verify\n\n| 1 | `true` | exit 0 |\n\n## Evidence\n\nPending.\n"
	home := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		c := exec.Command("git", append([]string{"-C", home}, args...)...)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s %v", args, out, err)
		}
	}
	write := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q")
	git("config", "user.name", "fixture")
	git("config", "user.email", "fixture@example.invalid")
	write(filepath.Join(home, brief), briefBody)
	write(filepath.Join(home, index), "# x\n")
	write(filepath.Join(home, "source.txt"), "attested source\n")
	git("add", brief, index, "source.txt")
	git("commit", "-q", "-m", "fixture")
	git("update-ref", "refs/remotes/origin/main", "HEAD")
	git("checkout", "-q", "--detach")
	if err := deskkit.PrepareVerifierAttestation(home, repo, brief, "gpt-6-astra", "strong"); err != nil {
		t.Fatal(err)
	}
	receipt, err := deskkit.RecoverVerifierAttestation(home)
	if err != nil {
		t.Fatalf("dispatcher attestation: %v (unexpected API calls %v)", err, api.unexpected)
	}
	f.setFile(brief, briefBody)

	// The desk writes the verifier's returned rows to its own scratch, outside the home.
	fragment := filepath.Join(t.TempDir(), "evidence.md")
	write(fragment, "| 1 | `true` | 0 | ok | 2026-10-06 | verifier |\n")
	land := func(args ...string) int {
		t.Helper()
		errBuf.Reset()
		return run(append([]string{repo, "main"}, args...))
	}

	t.Run("desk-checkout-root-refuses-and-names-home", func(t *testing.T) {
		if code := land("--root", t.TempDir(), "--brief-path", brief, "--evidence-file", fragment); code == 0 {
			t.Fatal("landing from a checkout that is not the verifier home was admitted")
		}
		if !strings.Contains(errBuf.String(), "--root naming the dispatched verifier home") {
			t.Fatalf("refusal does not direct the lander to the verifier home: %s", errBuf)
		}
	})
	t.Run("fragment-inside-home-refuses", func(t *testing.T) {
		inside := filepath.Join(home, "evidence.md")
		write(inside, "| 1 | `true` | 0 | ok |\n")
		defer os.Remove(inside)
		if code := land("--root", home, "--brief-path", brief, "--evidence-file", "evidence.md"); code == 0 {
			t.Fatal("an untracked fragment inside the home was admitted")
		}
		if !strings.Contains(errBuf.String(), "unattested worktree file") {
			t.Fatalf("refused for another reason: %s", errBuf)
		}
		os.Remove(inside)
		// A tracked, unchanged file passes admission, so only the fragment-location
		// rule refuses an absolute path inside the home.
		if code := land("--root", home, "--brief-path", brief, "--evidence-file", filepath.Join(home, "source.txt")); code == 0 {
			t.Fatal("an absolute fragment inside the home was accepted")
		}
		if !strings.Contains(errBuf.String(), "fragment outside --root") {
			t.Fatalf("refused for another reason: %s", errBuf)
		}
	})
	if len(f.writes) != 0 {
		t.Fatalf("a refused landing wrote: %+v", f.writes)
	}

	t.Run("documented-form-lands", func(t *testing.T) {
		if code := land("--root", home, "--brief-path", brief, "--evidence-file", fragment); code != 0 {
			t.Fatalf("documented landing refused: %s", errBuf)
		}
		if len(f.writes) != 1 || f.writes[0].File != brief || !strings.Contains(f.writes[0].Message, receipt.EvidenceBinding()) {
			t.Fatalf("landing lost target or run binding: %+v", f.writes)
		}
	})

	record := filepath.Join(t.TempDir(), "outcome.json")
	write(record, `{"brief":"x/01","ts":"2026-10-06T00:00:00Z","verdict":"verify-fail","digest":"0123456789abcdef"}`)
	t.Run("refresh-files-only-keeps-admission", func(t *testing.T) {
		// The documented refresh: the landed brief replaces the home's copy; HEAD stays.
		landed := f.files[brief]
		write(filepath.Join(home, brief), landed)
		if code := land("--root", home, "--outcome-record", record); code != 0 {
			t.Fatalf("outcome record after a file-only refresh refused: %s", errBuf)
		}
	})
	t.Run("moving-home-head-refuses", func(t *testing.T) {
		git("commit", "-q", "--allow-empty", "-m", "newer main")
		before := len(f.writes)
		write(record, `{"brief":"x/01","ts":"2026-10-06T00:00:01Z","verdict":"verify-fail","digest":"0123456789abcdef"}`)
		if code := land("--root", home, "--outcome-record", record); code == 0 {
			t.Fatal("landing after moving the home's HEAD was admitted")
		}
		if !strings.Contains(errBuf.String(), "attested detached source commit") {
			t.Fatalf("refused for another reason: %s", errBuf)
		}
		if len(f.writes) != before {
			t.Fatalf("refused landing wrote: %+v", f.writes[before:])
		}
	})
	if len(api.unexpected) != 0 {
		t.Fatalf("unexpected forge calls: %v", api.unexpected)
	}
}
