package commstransport

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

// Defect class: a host-local gateway endpoint opened or dialled anywhere but
// this package, or a client dial here that returns a connection before the
// endpoint's owner is checked. Each such site skips the endpoint check.
//
// Parsing ignores build tags, so the Windows file is checked on every host.

// rawEndpointCalls names the raw primitives that bypass this package.
func rawEndpointCall(call *ast.CallExpr) (string, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	// A net.Dialer remains a raw endpoint primitive when stored in a variable.
	if netDialerExpr(endpointDeclaration(sel.X)) {
		network := 0
		if sel.Sel.Name == "DialContext" {
			network = 1
		} else if sel.Sel.Name != "Dial" {
			return "", false
		}
		if len(call.Args) > network && unixNetwork(call.Args[network]) {
			return "net.Dialer." + sel.Sel.Name + `("unix")`, true
		}
		return "", false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return "", false
	}
	switch pkg.Name {
	case "winio":
		if strings.HasPrefix(sel.Sel.Name, "DialPipe") || sel.Sel.Name == "ListenPipe" {
			return "winio." + sel.Sel.Name, true
		}
	case "net":
		switch sel.Sel.Name {
		case "DialUnix", "ListenUnix":
			return "net." + sel.Sel.Name, true
		case "Dial", "DialTimeout", "Listen":
			if len(call.Args) > 0 && unixNetwork(call.Args[0]) {
				return "net." + sel.Sel.Name + `("unix")`, true
			}
		}
	}
	return "", false
}

// endpointDeclaration follows one lexical binding to its declared value or type.
func endpointDeclaration(x ast.Expr) ast.Expr {
	id, ok := ast.Unparen(x).(*ast.Ident)
	if !ok || id.Obj == nil {
		return x
	}
	switch decl := id.Obj.Decl.(type) {
	case *ast.AssignStmt:
		if len(decl.Lhs) != len(decl.Rhs) {
			return x
		}
		for i, lhs := range decl.Lhs {
			if name, ok := lhs.(*ast.Ident); ok && name.Obj == id.Obj {
				return decl.Rhs[i]
			}
		}
	case *ast.ValueSpec:
		for i, name := range decl.Names {
			if name.Obj != id.Obj {
				continue
			}
			if i < len(decl.Values) {
				return decl.Values[i]
			}
			if decl.Type != nil {
				return decl.Type
			}
		}
	}
	return x
}

func unixNetwork(x ast.Expr) bool {
	lit, ok := ast.Unparen(endpointDeclaration(x)).(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return false
	}
	network, err := strconv.Unquote(lit.Value)
	return err == nil && network == "unix"
}

func netDialerExpr(x ast.Expr) bool {
	switch x := ast.Unparen(x).(type) {
	case *ast.UnaryExpr:
		return x.Op == token.AND && netDialerExpr(x.X)
	case *ast.StarExpr:
		return netDialerExpr(x.X)
	case *ast.CompositeLit:
		return netDialerExpr(x.Type)
	case *ast.SelectorExpr:
		pkg, ok := x.X.(*ast.Ident)
		return ok && pkg.Name == "net" && x.Sel.Name == "Dialer"
	}
	return false
}

type endpointSite struct {
	site  string // file:func
	calls map[string]bool
}

func scanEndpointSites(fset *token.FileSet, name string, file *ast.File) []endpointSite {
	var out []endpointSite
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		site := endpointSite{site: filepath.ToSlash(name) + ":" + fn.Name.Name, calls: map[string]bool{}}
		raw := false
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if _, hit := rawEndpointCall(call); hit {
				raw = true
			}
			if id, ok := call.Fun.(*ast.Ident); ok {
				site.calls[id.Name] = true
			}
			return true
		})
		// The shared socket checker is a required node in the ownership graph,
		// even though it does not itself open a connection. Naming an emptied
		// helper at a dial site must not clear this source guard.
		ownerHelper := filepath.ToSlash(name) == "internal/commstransport/local_unix.go" && fn.Name.Name == "checkedUnixSocket"
		if raw || ownerHelper {
			out = append(out, site)
		}
	}
	return out
}

// allowedEndpointSites is the complete endpoint/ownership graph; each entry
// names the check required before that function can hand back a connection.
var allowedEndpointSites = map[string]string{
	"internal/commstransport/local_unix.go:Listen":            "ownerOnlyParent",
	"internal/commstransport/local_unix.go:RemoveStale":       "ownedByCurrentUser",
	"internal/commstransport/local_unix.go:Dial":              "checkedUnixSocket",
	"internal/commstransport/local_unix.go:DialUnixContext":   "checkedUnixSocket",
	"internal/commstransport/local_unix.go:checkedUnixSocket": "ownedByCurrentUser",
	"internal/commstransport/local_windows.go:Listen":         "pipeSecurity",
	"internal/commstransport/local_windows.go:dialOwnedBy":    "verifyPipeOwner",
}

func checkEndpointSites(sites []endpointSite) []string {
	var bad []string
	for _, s := range sites {
		need, ok := allowedEndpointSites[s.site]
		if !ok {
			bad = append(bad, s.site+": raw endpoint call outside commstransport")
			continue
		}
		if !s.calls[need] {
			bad = append(bad, s.site+": missing "+need)
		}
	}
	sort.Strings(bad)
	return bad
}

func TestEndpointSitesAreConfined(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("could-not-check: module root not found at %s: %v", root, err)
	}
	fset := token.NewFileSet()
	var sites []endpointSite
	files := 0
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".") && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		files++
		rel, _ := filepath.Rel(root, path)
		sites = append(sites, scanEndpointSites(fset, rel, file)...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if files == 0 || len(sites) == 0 {
		t.Fatalf("could-not-check: scanned %d files, found %d endpoint sites", files, len(sites))
	}
	if bad := checkEndpointSites(sites); len(bad) > 0 {
		t.Fatalf("endpoint sites outside the checked transport:\n  %s", strings.Join(bad, "\n  "))
	}
	for want := range allowedEndpointSites {
		found := false
		for _, s := range sites {
			found = found || s.site == want
		}
		if !found {
			t.Fatalf("allow-listed site %s no longer found; the scan may have stopped matching", want)
		}
	}
}

// Positive control: planted repeats of the defect must be flagged.
func TestEndpointSiteScanFlagsPlant(t *testing.T) {
	const src = `package plant
func a() { net.DialTimeout("unix", p, d) }
func b() { winio.DialPipe(p, &d) }
func c() { (&net.Dialer{}).DialContext(ctx, "unix", p) }
func d() { (&net.Dialer{}).Dial("unix", p) }
func e() { d := net.Dialer{}; d.DialContext(ctx, "unix", p) }
func f() { nw := "unix"; (&net.Dialer{}).DialContext(ctx, nw, p) }
func g() { d, nw := &net.Dialer{}, "unix"; d.DialContext(ctx, nw, p) }
func h() { var d net.Dialer; const nw = "unix"; d.Dial(nw, p) }
func i() { nw := "unix"; net.DialTimeout(nw, p, d) }
func otherDialer() { d := other.Dialer{}; d.DialContext(ctx, "unix", p) }
func shadowedNetwork() {
	nw := "unix"
	{
		nw := "tcp"
		d := net.Dialer{}
		d.DialContext(ctx, nw, p)
	}
	_ = nw
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "plant.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	bad := checkEndpointSites(scanEndpointSites(fset, "cmd/plant/plant.go", file))
	want := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"}
	if len(bad) != len(want) {
		t.Fatalf("unexpected planted endpoint sites: %v", bad)
	}
	for i, name := range want {
		if expected := "cmd/plant/plant.go:" + name + ": raw endpoint call outside commstransport"; bad[i] != expected {
			t.Errorf("site %d: got %q, want %q", i, bad[i], expected)
		}
	}
	unchecked := endpointSite{site: "internal/commstransport/local_windows.go:dialOwnedBy", calls: map[string]bool{}}
	if bad := checkEndpointSites([]endpointSite{unchecked}); len(bad) != 1 {
		t.Fatalf("dial without owner check not flagged: %v", bad)
	}
	unchecked = endpointSite{site: "internal/commstransport/local_unix.go:DialUnixContext", calls: map[string]bool{}}
	if bad := checkEndpointSites([]endpointSite{unchecked}); len(bad) != 1 {
		t.Fatalf("context dial without shared owner check not flagged: %v", bad)
	}
	unchecked = endpointSite{site: "internal/commstransport/local_unix.go:checkedUnixSocket", calls: map[string]bool{}}
	if bad := checkEndpointSites([]endpointSite{unchecked}); len(bad) != 1 {
		t.Fatalf("shared checker without socket owner check not flagged: %v", bad)
	}
}
