package adapter_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The defect class the authorization contract closes: a read-token consumer that is
// built from a token RESOLVED ONCE (a map from [adapter.TokenSource.Resolve] or
// [adapter.ResolveTokens], or a single token string) and kept, rather than
// from the [adapter.TokenSource] itself re-resolved per cycle or per read.
// Under --tokens-dir such a consumer keeps using the start-time token after
// the sidecar rotates it, and fails closed about an hour after start.
//
// The guard below is an allow-list over every production call site, module
// wide, of the primitives that produce or wrap a held token:
//
//   - a `.Resolve(...)` method call (TokenSource.Resolve; the name also
//     matches the terminal registry's Resolve, allow-listed by site below);
//   - ResolveTokens, the env-source resolver;
//   - NewGitHubRouter / NewGitHub, the client constructors that take a token.
//
// Each allowed site re-resolves per cycle or per read, or is the source
// itself. A new site anywhere in the module fails this test until a reviewer
// adds it here with the reason it does not hold a start-time token.
var tokenSnapshotAllow = map[string]string{
	"internal/adapter/tokens.go EnvTokenSource.Resolve ResolveTokens":         "the env source itself, called fresh on every Resolve",
	"internal/adapter/tokens.go MissingTokens Resolve":                        "startup refusal: returns only the missing list, the map is discarded",
	"internal/adapter/tokens.go CurrentToken Resolve":                         "the per-read resolution PermissionRouter and fleet.Cache go through",
	"internal/adapter/tokens.go ReloadingRouter.RefreshCycle Resolve":         "re-resolved at the top of every poll cycle",
	"internal/adapter/tokens.go ReloadingRouter.RefreshCycle NewGitHubRouter": "clients rebuilt every poll cycle from that cycle's tokens",
	"internal/adapter/routing.go NewGitHubRouter NewGitHub":                   "the router constructor; its only production caller is RefreshCycle",
	"internal/fleet/cache.go Cache.clientFor NewGitHub":                       "built from CurrentToken's result on every read, rebuilt when the token changes",
	"internal/serve/v1.go Server.handleV1Terminal Resolve":                    "terminal registry lookup by desk id — a name collision, not a token source",
}

var tokenSnapshotPrimitives = map[string]bool{
	"Resolve": true, "ResolveTokens": true, "NewGitHubRouter": true, "NewGitHub": true,
}

// scanTokenSites returns "<relpath> <Func> <primitive>" for every call to a
// token-snapshot primitive in the non-test .go files under root, skipping
// testdata, vendor and hidden directories.
func scanTokenSites(t *testing.T, root string) []string {
	t.Helper()
	var sites []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			n := d.Name()
			if path != root && (n == "testdata" || n == "vendor" || n == "node_modules" || strings.HasPrefix(n, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		// Every declaration is scanned: function bodies (named by their
		// function, with receiver) and package-level var/const initialisers
		// (named "(package)"), so a client built once at package init is
		// caught as well as one built inside a function.
		for _, decl := range file.Decls {
			name := "(package)"
			var body ast.Node = decl
			if fn, ok := decl.(*ast.FuncDecl); ok {
				if fn.Body == nil {
					continue
				}
				name = fn.Name.Name
				if fn.Recv != nil && len(fn.Recv.List) == 1 {
					name = recvName(fn.Recv.List[0].Type) + "." + name
				}
				body = fn.Body
			}
			ast.Inspect(body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				var prim string
				switch f := call.Fun.(type) {
				case *ast.Ident:
					prim = f.Name
				case *ast.SelectorExpr:
					prim = f.Sel.Name
				}
				if tokenSnapshotPrimitives[prim] {
					sites = append(sites, filepath.ToSlash(rel)+" "+name+" "+prim)
				}
				return true
			})
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	sort.Strings(sites)
	return sites
}

func recvName(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.StarExpr:
		return recvName(x.X)
	case *ast.Ident:
		return x.Name
	case *ast.IndexExpr:
		return recvName(x.X)
	}
	return "?"
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no go.mod above the test directory")
		}
		dir = parent
	}
}

// TestTokenSnapshotClass fails on any production call to a token-snapshot
// primitive outside the allow-list, and on any allow-list entry the scan no
// longer finds (so a matcher that silently stops matching is red, not clean).
func TestTokenSnapshotClass(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range scanTokenSites(t, moduleRoot(t)) {
		seen[s] = true
		if _, ok := tokenSnapshotAllow[s]; !ok {
			t.Errorf("token-snapshot primitive at an unlisted site: %s — build the consumer from the adapter.TokenSource and resolve per read (adapter.CurrentToken) or per cycle (adapter.ReloadingRouter), never from a token resolved once (the authorization contract); if this site does re-resolve, add it to tokenSnapshotAllow with the reason", s)
		}
	}
	for s := range tokenSnapshotAllow {
		if !seen[s] {
			t.Errorf("allow-listed site not found by the scan: %s — the site moved or the matcher stopped matching; update the allow-list", s)
		}
	}
}

// TestTokenSnapshotPlanted is the guard's positive control: a committed
// fixture repeating the #217 shape (a start-time Resolve feeding a client
// constructor, and a client built once at package init) must be flagged.
func TestTokenSnapshotPlanted(t *testing.T) {
	sites := scanTokenSites(t, filepath.Join("testdata", "tokenclass"))
	want := []string{
		"planted.go (package) NewGitHub",
		"planted.go buildConsumers NewGitHubRouter",
		"planted.go buildConsumers Resolve",
	}
	if strings.Join(sites, "\n") != strings.Join(want, "\n") {
		t.Fatalf("scan of the planted fixture = %q, want %q", sites, want)
	}
	for _, s := range sites {
		if _, ok := tokenSnapshotAllow[s]; ok {
			t.Fatalf("planted site %s is allow-listed — the positive control proves nothing", s)
		}
	}
}
