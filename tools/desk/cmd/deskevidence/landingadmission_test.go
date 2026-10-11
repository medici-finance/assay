package main

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
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
//
// It also serves the Contents API for the fixture repository, so a landing can
// run its own remote read and its write through the same resolved backend that
// admission reads through. Every request is logged with the credential it
// carried, which is what lets a test say WHICH token reached the forge and in
// what order admission and the landing read it.
type attestationAPI struct {
	mu                 sync.Mutex
	title, body, state string
	labels             []string
	timeline           []map[string]any
	unexpected         []string

	files map[string]string // branch content, by repo path
	puts  []contentsPut     // every Contents-API write, in order
	calls []apiCall         // every request, in order
}

// contentsPut is one Contents-API write the offline API accepted.
type contentsPut struct{ File, Message, Content string }

// apiCall is one request the offline API received and the credential it carried.
type apiCall struct{ Method, Path, Authorization string }

// contentsPrefix is the Contents-API path of the fixture repository.
const contentsPrefix = "/repos/example-org/tracker/contents/"

// blobID is a stand-in blob id: stable for equal content, different otherwise.
func blobID(content string) string {
	sum := sha1.Sum([]byte(content))
	return hex.EncodeToString(sum[:])
}

// serveContents answers a Contents-API read or write for file.
func (a *attestationAPI) serveContents(w http.ResponseWriter, r *http.Request, file string) {
	enc := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	switch r.Method {
	case http.MethodGet:
		content, ok := a.files[file]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			enc(map[string]any{"message": "Not Found"})
			return
		}
		enc(map[string]any{"sha": blobID(content), "encoding": "base64",
			"content": base64.StdEncoding.EncodeToString([]byte(content))})
	case http.MethodPut:
		var in struct{ Message, Content, Branch, SHA string }
		_ = json.NewDecoder(r.Body).Decode(&in)
		decoded, err := base64.StdEncoding.DecodeString(in.Content)
		if err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		if a.files == nil {
			a.files = map[string]string{}
		}
		a.files[file] = string(decoded)
		a.puts = append(a.puts, contentsPut{File: file, Message: in.Message, Content: string(decoded)})
		enc(map[string]any{
			"content": map[string]any{"sha": blobID(string(decoded))},
			"commit": map[string]any{"sha": blobID(in.Message + string(decoded)),
				"author": map[string]any{"name": "assay-verifier-app[bot]"}},
		})
	default:
		a.unexpected = append(a.unexpected, r.Method+" "+r.URL.Path)
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
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
	a.calls = append(a.calls, apiCall{Method: r.Method, Path: p, Authorization: r.Header.Get("Authorization")})
	switch {
	case strings.Contains(p, contentsPrefix):
		a.serveContents(w, r, p[strings.Index(p, contentsPrefix)+len(contentsPrefix):])
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
// through the REAL pre-work admission (no admission stub) and the tool's real
// custody step, each landing minting its own token (newLandingFixture): --root
// names the dispatched verifier home, the --brief-path fragment lives outside
// it, and the refresh before an outcome record rewrites only the brief and
// stream index in the home, never its detached HEAD. The forms the procedure
// forbids refuse.
func TestVerifierEvidenceLandingFormAdmitted(t *testing.T) {
	if reflect.ValueOf(productionAdmission).Pointer() != reflect.ValueOf(deskkit.CheckVerifierEvidenceWithForge).Pointer() {
		t.Fatal("the shipped landing does not bind the shared admission reader")
	}
	lf := newLandingFixture(t)
	api, home, errBuf := lf.api, lf.home, lf.errBuf
	const brief = fixtureBrief

	// The desk writes the verifier's returned rows to its own scratch, outside the home.
	fragment := filepath.Join(t.TempDir(), "evidence.md")
	writeFixtureFile(t, fragment, "| 1 | `true` | 0 | ok | 2026-10-06 | verifier |\n")
	land := func(args ...string) int {
		t.Helper()
		code, _ := lf.land(t, args...)
		return code
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
		writeFixtureFile(t, inside, "| 1 | `true` | 0 | ok |\n")
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
	if len(api.puts) != 0 {
		t.Fatalf("a refused landing wrote: %+v", api.puts)
	}

	t.Run("documented-form-lands", func(t *testing.T) {
		if code := land("--root", home, "--brief-path", brief, "--evidence-file", fragment); code != 0 {
			t.Fatalf("documented landing refused: %s", errBuf)
		}
		if len(api.puts) != 1 || api.puts[0].File != brief || !strings.Contains(api.puts[0].Message, lf.receipt.EvidenceBinding()) {
			t.Fatalf("landing lost target or run binding: %+v", api.puts)
		}
	})

	record := filepath.Join(t.TempDir(), "outcome.json")
	writeFixtureFile(t, record, `{"brief":"x/01","ts":"2026-10-06T00:00:00Z","verdict":"verify-fail","digest":"0123456789abcdef"}`)
	t.Run("refresh-files-only-keeps-admission", func(t *testing.T) {
		// The documented refresh: the landed brief replaces the home's copy; HEAD stays.
		writeFixtureFile(t, filepath.Join(home, brief), api.files[brief])
		if code := land("--root", home, "--outcome-record", record); code != 0 {
			t.Fatalf("outcome record after a file-only refresh refused: %s", errBuf)
		}
	})
	t.Run("moving-home-head-refuses", func(t *testing.T) {
		lf.git(t, "commit", "-q", "--allow-empty", "-m", "newer main")
		before := len(api.puts)
		writeFixtureFile(t, record, `{"brief":"x/01","ts":"2026-10-06T00:00:01Z","verdict":"verify-fail","digest":"0123456789abcdef"}`)
		if code := land("--root", home, "--outcome-record", record); code == 0 {
			t.Fatal("landing after moving the home's HEAD was admitted")
		}
		if !strings.Contains(errBuf.String(), "attested detached source commit") {
			t.Fatalf("refused for another reason: %s", errBuf)
		}
		if len(api.puts) != before {
			t.Fatalf("refused landing wrote: %+v", api.puts[before:])
		}
	})
	if len(api.unexpected) != 0 {
		t.Fatalf("unexpected forge calls: %v", api.unexpected)
	}
}

// pinFixtureForge names the fixture repository's forge in the fixture roster,
// so admission never falls back to the forge of the enclosing checkout's origin.
func pinFixtureForge(t *testing.T, home, repo string) {
	t.Helper()
	path := filepath.Join(home, ".config", "assay", "roster.env")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	contents = append(contents, []byte("\nASSAY_REPO_FORGES="+repo+"=github\n")...)
	if err := os.WriteFile(path, contents, 0600); err != nil {
		t.Fatal(err)
	}
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
}
