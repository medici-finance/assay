package deskkit

// sharedread_test.go — forge-neutral brief 36: the desk-tools side of the shared fact reader.
//
//   - TestAmbientTokenGuardCoversForgeread: the ambient-token-read rule now walks forgeread/ too,
//     with no permit, and a planted GH_TOKEN read inside an adapter is exactly one finding.
//   - TestSharedReadLowerScope: the lower-layer scope check. Bypassing every consumer-side repo
//     check and handing the shared adapter (as deskkit composes it) a repository outside the
//     credential's admitted scope is refused BEFORE any request reaches the provider.

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// forgereadTreeRoot is the shared reader module, relative to this package.
const forgereadTreeRoot = "../../../../forgeread"

// regression: #2395
func TestAmbientTokenGuardCoversForgeread(t *testing.T) {
	found, err := scanAmbientTokenReads(forgereadTreeRoot)
	if err != nil {
		t.Fatalf("the ambient-token guard could not scan forgeread/: %v — could-not-check, NOT clean", err)
	}
	if len(found) != 0 {
		t.Fatalf("forgeread/ reads a forge token from the process environment: %v — the shared reader takes "+
			"already-admitted access and never discovers a credential; there is no permit for this tree", found)
	}
	// The scanner must actually see this tree: a walk that finds no Go file certifies everything.
	goFiles := 0
	_ = filepath.WalkDir(forgereadTreeRoot, func(p string, d fs.DirEntry, _ error) error {
		if d != nil && !d.IsDir() && strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
			goFiles++
		}
		return nil
	})
	if goFiles < 4 {
		t.Fatalf("the scan walked %d forgeread source files, want at least the root and both adapters", goFiles)
	}

	// Negative control: the same scan over a copy with one planted reader must report exactly one.
	tmp := t.TempDir()
	if err := filepath.WalkDir(forgereadTreeRoot, func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		rel, _ := filepath.Rel(forgereadTreeRoot, p)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(tmp, rel), 0o755)
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		return os.WriteFile(filepath.Join(tmp, rel), b, 0o644)
	}); err != nil {
		t.Fatal(err)
	}
	planted := "package github\n\nimport \"os\"\n\nfunc plantedAmbient() string { return os.Getenv(\"GH_TOKEN\") }\n"
	if err := os.WriteFile(filepath.Join(tmp, "adapters", "github", "planted.go"), []byte(planted), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := scanAmbientTokenReads(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !strings.HasSuffix(got[0], "planted.go::plantedAmbient") {
		t.Fatalf("planted GH_TOKEN read in an adapter: findings = %v, want exactly one", got)
	}
}

// TestSharedReadLowerScope drives the shared adapter exactly as deskkit composes it for the CI
// workflow-token transport (a credential admitted for ONE repository), against a fake provider
// that records every request.
// regression: #2395
func TestSharedReadLowerScope(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"number":1,"title":"t","user":{"login":"u","id":1},"labels":[],"created_at":"2026-01-01T00:00:00Z"}]`))
	}))
	defer srv.Close()
	defer SetCITokenAPIBaseForTest(srv.URL)()

	f, _, err := ReadOnlyForgeForCIToken(ForgeRepo{Owner: "example-org", Name: "own"}, "example-org/own", "ghs_fixture")
	if err != nil {
		t.Fatal(err)
	}

	// Control: the repository the credential was admitted for reads.
	got, err := f.ListOpenIssues(ForgeRepo{Owner: "example-org", Name: "own"})
	if err != nil || len(got) != 1 {
		t.Fatalf("in-scope control read: %v %v", got, err)
	}
	if requests.Load() == 0 {
		t.Fatal("the in-scope control never reached the fake provider — the zero below would prove nothing")
	}

	// Bypass: no consumer-side repo check runs here; the call goes straight to the composed forge
	// with a repository outside the admitted scope (including a case-variant lookalike prefix).
	before := requests.Load()
	for _, other := range []ForgeRepo{
		{Owner: "example-org", Name: "other"},
		{Owner: "other-org", Name: "own"},
		{Owner: "example-org", Name: "own-extra"},
	} {
		got, err := f.ListOpenIssues(other)
		if err == nil || len(got) != 0 {
			t.Errorf("%s: out-of-scope read returned %d issues and err=%v, want a refusal", other.Slug(), len(got), err)
		}
	}
	if n := requests.Load() - before; n != 0 {
		t.Fatalf("%d request(s) reached the provider for out-of-scope repositories, want 0 — the lower "+
			"layer must refuse before building a request", n)
	}

	// The same bound holds on the bare composition, below the Forge interface.
	acc := (&GitHubForge{Token: "ghs_fixture", BaseURL: srv.URL, onlyRepo: "example-org/own"}).sharedAccess()
	if _, err := acc.GetPage(nil, "example-org/other", "issues", nil); err == nil { //nolint:staticcheck // nil ctx is unused by the composition
		t.Error("the composed access served a repository outside its scope")
	}
	if n := requests.Load() - before; n != 0 {
		t.Fatalf("%d request(s) reached the provider through the bare composition", n)
	}
}
