package deskkit

// gitlabcredsites_test.go — the class guard for review finding SEC-1
// (credential-forwarded-on-redirect).
//
// THE CLASS. A GitLab credential travels as a CUSTOM header (PRIVATE-TOKEN, JOB-TOKEN) or in
// a request body (the pipeline trigger token). net/http strips Authorization and cookies when
// it follows a redirect to another host and forwards every other header, and a 307/308 re-sends
// the body. So any site that sends a GitLab credential through a client that follows redirects
// hands the credential to whatever host a Location header names. SEC-1 was one such site (the
// run-log trace read); the guard has to catch the next one, which the forge's own tests cannot.
//
// THE GUARD. An AST walk of every shipped (non-test, non-testdata) Go file under tools/desk
// enumerates each site that can send a GitLab credential: a call of a client-go constructor
// (gitlab.New*Client) and every string literal naming a GitLab credential header. The set must
// equal gitlabCredSites below, and each entry names the wire test that drives that site through
// a cross-host redirect and asserts the other host received nothing. The guard checks that the
// named test exists. A new site, or a listed one that disappears, fails until it is reviewed.
//
// POSITIVE CONTROL. testdata/gitlabcredclass/planted.go holds an unlisted client constructor and
// an unlisted header literal; the guard must flag both, or a clean verdict on the real tree
// means nothing.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// gitlabCredSites is the reviewed register: site key → the test that pins it redirect-safe.
// Key: "<path from tools/desk>|<enclosing func>|<kind>". Kinds: "client" (a gitlab.New*Client
// call) and "header" (a credential-header string literal).
var gitlabCredSites = map[string]string{
	// The forge backend: both clients go through gitlabNoRedirectClient.
	"internal/deskkit/forge_gitlab.go|client|client":        "TestGitLabRunLogRedirectKeepsToken",
	"internal/deskkit/forge_gitlab.go|triggerClient|client": "TestGitLabClientRedirectKeepsToken",
	// Account liveness: no-redirect default, and an injected client is copied with redirects refused.
	"internal/deskkit/trustliveness_gitlab.go|GetAccount|header": "TestGitLabAccountFetcherInjectedClientRefusesRedirect",
	// desktoken rotation and self-check: gitlabHTTPClient.
	"cmd/desktoken/gitlab.go|rotateGitLabToken|header": "TestGitLabRotateRefusesRedirect",
	"cmd/desktoken/gitlab.go|gitlabSelfCheck|header":   "TestGitLabSelfCheckRefusesRedirect",
	// deskfleet: noRedirects wraps every client it builds.
	"cmd/deskfleet/client.go|newGitLabClient|header": "TestFleetNeverFollowsRedirect",
}

// gitlabCredHeaders are the GitLab credential header names, compared case-insensitively
// (net/http canonicalises PRIVATE-TOKEN to Private-Token, so either spelling sends it).
var gitlabCredHeaders = []string{"private-token", "job-token"}

type gitlabCredSite struct{ key, pos string }

// scanGitLabCredSites walks root's shipped Go files and returns every site, plus the set of
// Test function names declared in root's _test.go files (the register's pins must exist).
func scanGitLabCredSites(root string) (sites []gitlabCredSite, tests map[string]bool, files int, err error) {
	tests = map[string]bool{}
	fset := token.NewFileSet()
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
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return perr
		}
		if strings.HasSuffix(path, "_test.go") {
			for _, decl := range f.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && strings.HasPrefix(fn.Name.Name, "Test") {
					tests[fn.Name.Name] = true
				}
			}
			return nil
		}
		files++
		rel, _ := filepath.Rel(root, path)
		sites = append(sites, gitlabCredSitesIn(fset, f, filepath.ToSlash(rel))...)
		return nil
	})
	return sites, tests, files, err
}

// gitlabCredSitesIn returns one file's sites. A site outside any function (a package-level
// var or const) is keyed to "<package>" — it still has to be listed.
func gitlabCredSitesIn(fset *token.FileSet, f *ast.File, rel string) []gitlabCredSite {
	var sites []gitlabCredSite
	visit := func(fn string, root ast.Node) {
		ast.Inspect(root, func(n ast.Node) bool {
			kind := ""
			switch x := n.(type) {
			case *ast.CallExpr:
				if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
					if id, ok := sel.X.(*ast.Ident); ok && id.Name == "gitlab" &&
						strings.HasPrefix(sel.Sel.Name, "New") && strings.HasSuffix(sel.Sel.Name, "Client") {
						kind = "client"
					}
				}
			case *ast.BasicLit:
				if x.Kind == token.STRING {
					if v, err := strconv.Unquote(x.Value); err == nil {
						for _, h := range gitlabCredHeaders {
							if strings.EqualFold(strings.TrimSpace(v), h) {
								kind = "header"
							}
						}
					}
				}
			}
			if kind != "" {
				sites = append(sites, gitlabCredSite{key: rel + "|" + fn + "|" + kind, pos: fset.Position(n.Pos()).String()})
			}
			return true
		})
	}
	for _, decl := range f.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			if fn.Body != nil {
				visit(fn.Name.Name, fn.Body)
			}
			continue
		}
		visit("<package>", decl)
	}
	return sites
}

// reconcileGitLabCredSites returns the problems: unlisted sites, listed sites no longer
// present, and pins naming a test that does not exist.
func reconcileGitLabCredSites(sites []gitlabCredSite, tests map[string]bool, register map[string]string) []string {
	var problems []string
	seen := map[string]bool{}
	for _, s := range sites {
		if seen[s.key] {
			continue
		}
		seen[s.key] = true
		if _, ok := register[s.key]; !ok {
			problems = append(problems, "UNLISTED GitLab credential site — a custom-header or body credential is forwarded "+
				"on a followed redirect; route the call through a no-redirect client, add a cross-host redirect wire test, "+
				"and list it in gitlabCredSites: "+s.pos+" "+s.key)
		}
	}
	for k, pin := range register {
		if !seen[k] {
			problems = append(problems, "listed GitLab credential site no longer present (remove it or restore the call): "+k)
		}
		if tests != nil && !tests[pin] {
			problems = append(problems, "pinning test "+pin+" for "+k+" does not exist")
		}
	}
	sort.Strings(problems)
	return problems
}

// TestGitLabCredSitesRegistered is the class guard over the real tree.
func TestGitLabCredSitesRegistered(t *testing.T) {
	sites, tests, files, err := scanGitLabCredSites(deskTreeRoot)
	if err != nil {
		t.Fatalf("could not scan the desk tree: %v — could-not-check, NOT clean", err)
	}
	if files == 0 || len(sites) == 0 || len(tests) == 0 {
		t.Fatalf("scanned %d file(s), %d site(s), %d test(s) — a scanner that sees nothing certifies everything", files, len(sites), len(tests))
	}
	if problems := reconcileGitLabCredSites(sites, tests, gitlabCredSites); len(problems) > 0 {
		t.Fatalf("GitLab credential sites do not match the reviewed register:\n  %s", strings.Join(problems, "\n  "))
	}
}

// TestGitLabCredGuardSeesPlant is the positive control.
func TestGitLabCredGuardSeesPlant(t *testing.T) {
	dir := filepath.Join("testdata", "gitlabcredclass")
	if _, err := os.Stat(filepath.Join(dir, "planted.go")); err != nil {
		t.Fatalf("positive-control fixture missing: %v", err)
	}
	sites, _, _, err := scanGitLabCredSites(dir)
	if err != nil {
		t.Fatalf("scan the planted fixture: %v", err)
	}
	problems := strings.Join(reconcileGitLabCredSites(sites, nil, gitlabCredSites), "\n")
	for _, want := range []string{"planted.go|plantedClient|client", "planted.go|plantedHeader|header"} {
		if !strings.Contains(problems, "UNLISTED GitLab credential site") || !strings.Contains(problems, want) {
			t.Errorf("the guard did not flag the planted instance %q; it reported:\n%s", want, problems)
		}
	}
}
