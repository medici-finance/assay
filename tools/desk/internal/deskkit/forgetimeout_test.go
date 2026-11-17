package deskkit

// forgetimeout_test.go — the per-request bound on the desk's forge clients (issue #6718).
//
// The desk write verbs hold the shared audit lock across forge reads and writes, so a forge
// client with no deadline lets one stalled connection hold that lock indefinitely. These tests
// pin three things: the bound exists and is non-zero; the clients the backends build carry it,
// and a stalled server makes a read return within it; and no client in the forge library or a
// lock-holding tool falls back to a deadline-free client (the class guard, with a planted
// positive control).

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/credential"
)

var timeoutTestRepo = ForgeRepo{Owner: "example-org", Name: "example-repo"}

// regression: #6718
func TestForgeRequestTimeoutBounded(t *testing.T) {
	if ForgeRequestTimeout <= 0 {
		t.Fatalf("ForgeRequestTimeout = %v: zero is no deadline, so a stalled forge holds the audit lock", ForgeRequestTimeout)
	}
	// Each lock-holding tool's acquire wait is 60s; the holder's per-request bound must sit
	// inside the wait its waiters tolerate.
	if ForgeRequestTimeout > 60*time.Second {
		t.Fatalf("ForgeRequestTimeout = %v exceeds the 60s audit-lock acquire wait", ForgeRequestTimeout)
	}
	if got := BoundedHTTPClient().Timeout; got != ForgeRequestTimeout {
		t.Fatalf("BoundedHTTPClient().Timeout = %v, want %v", got, ForgeRequestTimeout)
	}
	for name, c := range map[string]*http.Client{"nil": nil, "zero": {}} {
		if got := boundedTimeout(c); got != ForgeRequestTimeout {
			t.Errorf("boundedTimeout(%s client) = %v, want %v", name, got, ForgeRequestTimeout)
		}
	}
	if got := boundedTimeout(&http.Client{Timeout: 5 * time.Second}); got != 5*time.Second {
		t.Errorf("boundedTimeout keeps an injected deadline: got %v, want 5s", got)
	}
}

// restClientTimeout reads the deadline of the http.Client inside go-gh's RESTClient. The field
// is unexported, so it is read by reflection; a go-gh that renames it fails here loudly rather
// than letting the check pass on nothing.
func restClientTimeout(t *testing.T, g *GitHubForge) time.Duration {
	t.Helper()
	rc, err := g.restClient()
	if err != nil {
		t.Fatalf("restClient: %v", err)
	}
	f := reflect.ValueOf(rc).Elem().FieldByName("client")
	if !f.IsValid() || f.IsNil() {
		t.Fatalf("could-not-check: go-gh RESTClient has no non-nil client field — update this probe")
	}
	tf := f.Elem().FieldByName("Timeout")
	if !tf.IsValid() {
		t.Fatalf("could-not-check: http.Client has no Timeout field")
	}
	return time.Duration(tf.Int())
}

// regression: #6718
func TestGitHubRESTClientBounded(t *testing.T) {
	// Production shape: no injected client. The go-gh client must carry ForgeRequestTimeout.
	prod := &GitHubForge{Token: "test-placeholder-token", BaseURL: "https://api.example.invalid"}
	if got := restClientTimeout(t, prod); got != ForgeRequestTimeout {
		t.Fatalf("go-gh client Timeout = %v, want %v — a dropped or zero ClientOptions.Timeout leaves every forge read unbounded", got, ForgeRequestTimeout)
	}
	// A transport-only client (the resolver's seam client, a read-context copy) sets no
	// deadline of its own; it must not erase the bound.
	seam := &GitHubForge{Token: "test-placeholder-token", BaseURL: "https://api.example.invalid",
		Client: &http.Client{Transport: http.DefaultTransport}}
	if got := restClientTimeout(t, seam); got != ForgeRequestTimeout {
		t.Fatalf("go-gh client Timeout with a deadline-free injected client = %v, want %v", got, ForgeRequestTimeout)
	}
}

// stalledServer accepts each request and never answers until the test ends.
func stalledServer(t *testing.T) *httptest.Server {
	t.Helper()
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	t.Cleanup(func() { close(release); srv.Close() })
	return srv
}

// returnsWithin runs call and fails the test if it has not returned inside limit, or if it
// returned without an error (a stalled read is never an answer).
func returnsWithin(t *testing.T, what string, limit time.Duration, call func() error) {
	t.Helper()
	done := make(chan error, 1)
	start := time.Now()
	go func() { done <- call() }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatalf("%s against a stalled forge returned no error", what)
		}
		t.Logf("%s returned after %v: %v", what, time.Since(start).Round(time.Millisecond), err)
	case <-time.After(limit):
		t.Fatalf("%s against a stalled forge did not return within %v — the request is unbounded", what, limit)
	}
}

// regression: #6718
func TestStalledGitHubReadBounded(t *testing.T) {
	srv := stalledServer(t)
	// The injected client keeps its own short deadline (boundedTimeout honours a non-zero
	// one), so the test proves the wire bound in milliseconds instead of 30 seconds.
	g := &GitHubForge{Token: "test-placeholder-token", BaseURL: srv.URL,
		Client: &http.Client{Transport: srv.Client().Transport, Timeout: 300 * time.Millisecond}}
	returnsWithin(t, "GetIssue", 5*time.Second, func() error {
		_, err := g.GetIssue(timeoutTestRepo, 1)
		return err
	})
	returnsWithin(t, "ChangeDiff", 5*time.Second, func() error {
		_, err := g.ChangeDiff(timeoutTestRepo, 1)
		return err
	})
}

// regression: #6718
func TestGitLabClientBounded(t *testing.T) {
	for name, base := range map[string]*http.Client{"nil": nil, "zero": {Transport: http.DefaultTransport}} {
		if got := gitlabNoRedirectClient(base).Timeout; got != ForgeRequestTimeout {
			t.Errorf("gitlabNoRedirectClient(%s).Timeout = %v, want %v", name, got, ForgeRequestTimeout)
		}
	}
	if got := gitlabNoRedirectClient(&http.Client{Timeout: time.Second}).Timeout; got != time.Second {
		t.Errorf("gitlabNoRedirectClient keeps an injected deadline: got %v, want 1s", got)
	}
}

// --- The class guard ---------------------------------------------------------------------

// unboundedClientRoots are the trees the class guard scans: the forge library, the credential
// provider, and every tool that holds the shared audit lock across forge calls (or that one of
// them runs while holding it — deskrelease runs desktoken to mint). Paths are from tools/desk.
var unboundedClientRoots = []string{
	"internal/deskkit",
	"internal/credential",
	"cmd/deskevidence",
	"cmd/deskpost",
	"cmd/deskrelease",
	"cmd/desktoken",
}

// unboundedClientSites returns every site in root's shipped Go files that sends, or builds a
// client that sends, with no request deadline: a reference to http.DefaultClient, a call of
// net/http's package-level Get/Head/Post/PostForm (which use it), and an http.Client or go-gh
// ClientOptions literal with no Timeout key (copies go through boundedCopy instead). It also
// reports an io.ReadAll whose error is not checked (readAllDiscardSites lists the shapes):
// once a deadline can cut a body short, a discarded read error turns a truncated page into a
// complete-looking answer (use ReadForgeBody).
func unboundedClientSites(root string) (sites []string, files int, err error) {
	fset := token.NewFileSet()
	byDir := map[string][]*ast.File{}
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if d.IsDir() {
			if n := d.Name(); path != root && (n == "testdata" || n == "vendor" || strings.HasPrefix(n, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return perr
		}
		files++
		byDir[filepath.Dir(path)] = append(byDir[filepath.Dir(path)], f)
		return nil
	})
	if err != nil {
		return nil, files, err
	}
	for _, pkg := range byDir {
		wrappers := readAllWrappers(pkg)
		for _, f := range pkg {
			sites = append(sites, clientSitesIn(fset, f)...)
			sites = append(sites, readAllDiscardSites(fset, f, wrappers)...)
		}
	}
	sort.Strings(sites)
	return sites, files, nil
}

// clientSitesIn reports the deadline-free client shapes in one file.
func clientSitesIn(fset *token.FileSet, f *ast.File) (sites []string) {
	// Resolve the name this file gives net/http, so an aliased import
	// (nethttp "net/http") is checked exactly like the default name.
	httpName := importName(f, "net/http", "http")
	ast.Inspect(f, func(n ast.Node) bool {
		what := ""
		switch x := n.(type) {
		case *ast.SelectorExpr:
			if id, ok := x.X.(*ast.Ident); ok && httpName != "" && id.Name == httpName {
				switch x.Sel.Name {
				case "DefaultClient":
					what = "http.DefaultClient"
				case "Get", "Head", "Post", "PostForm":
					what = "http." + x.Sel.Name + " (uses http.DefaultClient)"
				}
			}
		case *ast.CompositeLit:
			sel, isSel := x.Type.(*ast.SelectorExpr)
			httpClient := false
			if isSel && sel.Sel.Name == "Client" {
				if id, ok := sel.X.(*ast.Ident); ok && httpName != "" && id.Name == httpName {
					httpClient = true
				}
			}
			if isSel && (httpClient || sel.Sel.Name == "ClientOptions") {
				has := false
				for _, e := range x.Elts {
					if kv, ok := e.(*ast.KeyValueExpr); ok {
						if k, ok := kv.Key.(*ast.Ident); ok && k.Name == "Timeout" {
							has = true
						}
					}
				}
				if !has {
					what = sel.Sel.Name + " literal with no Timeout"
					if httpClient {
						what = "http.Client literal with no Timeout"
					}
				}
			}
		}
		if what != "" {
			sites = append(sites, fset.Position(n.Pos()).String()+": "+what)
		}
		return true
	})
	return sites
}

// importName is the local name file f gives the import path: its alias when it has one, def
// when it is imported plainly, "" when the file does not import it (or blank-imports it).
func importName(f *ast.File, path, def string) string {
	for _, im := range f.Imports {
		if strings.Trim(im.Path.Value, "`\"") != path {
			continue
		}
		if im.Name == nil {
			return def
		}
		if im.Name.Name == "_" {
			return ""
		}
		return im.Name.Name
	}
	return ""
}

// regression: #6718
func TestNoUnboundedForgeClient(t *testing.T) {
	var all []string
	total := 0
	for _, r := range unboundedClientRoots {
		sites, files, err := unboundedClientSites(filepath.Join(deskTreeRoot, r))
		if err != nil {
			t.Fatalf("could not scan %s: %v — could-not-check, NOT clean", r, err)
		}
		if files == 0 {
			t.Fatalf("scanned no Go file under %s — a scanner that sees nothing certifies everything", r)
		}
		total += files
		all = append(all, sites...)
	}
	if len(all) > 0 {
		t.Fatalf("deadline-free forge client(s) in a tree that runs under the shared audit lock — "+
			"use deskkit.BoundedHTTPClient (or set a Timeout) instead:\n  %s", strings.Join(all, "\n  "))
	}
	t.Logf("CLEAN: 0 unbounded client sites across %d file(s) in %d tree(s)", total, len(unboundedClientRoots))
}

// TestUnboundedGuardSeesPlant is the positive control: the planted fixture holds one instance
// of each shape (planted.go) and the aliased-import variants (planted_alias.go), and the guard
// must report every one.
func TestUnboundedGuardSeesPlant(t *testing.T) {
	dir := filepath.Join("testdata", "unboundedclient")
	if _, err := os.Stat(filepath.Join(dir, "planted.go")); err != nil {
		t.Fatalf("positive-control fixture missing: %v", err)
	}
	sites, _, err := unboundedClientSites(dir)
	if err != nil {
		t.Fatalf("scan the planted fixture: %v", err)
	}
	for _, want := range []string{
		"planted.go:*: http.DefaultClient", "planted.go:*: http.Get (uses http.DefaultClient)",
		"planted.go:*: ClientOptions literal with no Timeout", "planted.go:*: http.Client literal with no Timeout",
		"planted.go:*: io.ReadAll error discarded",
		// The same shapes under aliased imports (nethttp "net/http", stdio "io").
		"planted_alias.go:*: http.DefaultClient", "planted_alias.go:*: http.Client literal with no Timeout",
		"planted_alias.go:*: io.ReadAll error discarded",
	} {
		file, shape, _ := strings.Cut(want, ":*: ")
		found := false
		for _, s := range sites {
			if strings.Contains(s, string(filepath.Separator)+file+":") && strings.HasSuffix(s, ": "+shape) {
				found = true
			}
		}
		if !found {
			t.Errorf("guard missed the planted %q; it reported: %v", want, sites)
		}
	}
}

// readAllSitesIn scans dir with the class guard and returns only its io.ReadAll findings.
func readAllSitesIn(t *testing.T, dir string) []string {
	t.Helper()
	sites, _, err := unboundedClientSites(dir)
	if err != nil {
		t.Fatalf("scan %s: %v", dir, err)
	}
	var out []string
	for _, s := range sites {
		if strings.Contains(s, "io.ReadAll") {
			out = append(out, s)
		}
	}
	return out
}

// funcLines maps each top-level function in path to its [start, end] line span, so a test
// can say which planted function a reported site falls in.
func funcLines(t *testing.T, path string) map[string][2]int {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	out := map[string][2]int{}
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok {
			out[fd.Name.Name] = [2]int{fset.Position(fd.Pos()).Line, fset.Position(fd.End()).Line}
		}
	}
	return out
}

// regression: #6718
func TestReadAllGuardWideShapes(t *testing.T) {
	dir := filepath.Join("testdata", "unboundedclient")
	path := filepath.Join(dir, "planted_readall.go")
	spans := funcLines(t, path)
	sites := readAllSitesIn(t, dir)
	inFunc := func(fn, shape string) bool {
		span, ok := spans[fn]
		if !ok {
			t.Fatalf("fixture has no function %s", fn)
		}
		for _, s := range sites {
			if !strings.HasPrefix(s, path+":") || !strings.HasSuffix(s, ": "+shape) {
				continue
			}
			if line := siteLine(s, path); line >= span[0] && line <= span[1] {
				return true
			}
		}
		return false
	}
	for _, c := range []struct{ fn, shape string }{
		{"plantedBlankAssign", "io.ReadAll error discarded"},
		{"plantedVarDecl", "io.ReadAll error discarded"},
		{"plantedIgnored", "io.ReadAll result ignored"},
		{"plantedUnchecked", "io.ReadAll error never checked"},
		{"plantedBlanked", "io.ReadAll error never checked"},
		{"plantedOverwritten", "io.ReadAll error never checked"},
		{"plantedIoutil", "io.ReadAll error discarded"},
		{"plantedWrapper", "io.ReadAll error discarded (via readBody)"},
	} {
		if !inFunc(c.fn, c.shape) {
			t.Errorf("guard missed %s (%q); it reported: %v", c.fn, c.shape, sites)
		}
	}
	// The package-level `var x, _ = io.ReadAll(...)` sits outside any function.
	found := false
	for _, s := range sites {
		if strings.HasPrefix(s, path+":") && strings.HasSuffix(s, ": io.ReadAll error discarded") {
			if line := siteLine(s, path); line > 0 && !inAnySpan(line, spans) {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("guard missed the package-level var declaration; it reported: %v", sites)
	}
}

// siteLine is the line number in a "<path>:<line>:<col>: <what>" site, 0 when absent.
func siteLine(site, path string) int {
	rest, _ := strings.CutPrefix(site, path+":")
	line := 0
	for _, c := range rest {
		if c < '0' || c > '9' {
			break
		}
		line = line*10 + int(c-'0')
	}
	return line
}

func inAnySpan(line int, spans map[string][2]int) bool {
	for _, s := range spans {
		if line >= s[0] && line <= s[1] {
			return true
		}
	}
	return false
}

// regression: #6718
func TestReadAllGuardNoFalseHit(t *testing.T) {
	if sites := readAllSitesIn(t, filepath.Join("testdata", "readallclean")); len(sites) > 0 {
		t.Fatalf("guard flagged a checked io.ReadAll error:\n  %s", strings.Join(sites, "\n  "))
	}
}

// regression: #6718
func TestCredentialBoundMatches(t *testing.T) {
	// The credential package cannot import this one, so it carries its own copy of the bound.
	if credential.RequestTimeout != ForgeRequestTimeout {
		t.Fatalf("credential.RequestTimeout = %v, want deskkit.ForgeRequestTimeout %v", credential.RequestTimeout, ForgeRequestTimeout)
	}
	for name, base := range map[string]*http.Client{"nil": nil, "zero": {}} {
		if got := boundedCopy(base).Timeout; got != ForgeRequestTimeout {
			t.Errorf("boundedCopy(%s).Timeout = %v, want %v", name, got, ForgeRequestTimeout)
		}
		if got := countingClient(base).Timeout; got != ForgeRequestTimeout {
			t.Errorf("countingClient(%s).Timeout = %v, want %v", name, got, ForgeRequestTimeout)
		}
		if got := mirrorResponseClient(base).Timeout; got != ForgeRequestTimeout {
			t.Errorf("mirrorResponseClient(%s).Timeout = %v, want %v", name, got, ForgeRequestTimeout)
		}
	}
}
