package verifycmd

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// statusgenRowsPath is the statusgen source proseLedShape and its tables are
// ported from.
var statusgenRowsPath = filepath.Join("..", "..", "..", "..", "statusgen", "verifyrows.go")

// portedFromStatusgen names the declarations ported verbatim from statusgen.
var portedFromStatusgen = []string{"nonExecExtensions", "scriptExtensions", "codeIdentRe", "bareWordRe", "proseLedShape"}

// TestProseLedRuleMatchesStatusgen — proseLedShape and the tables it reads are
// a port of statusgen's. The shared vector table pins the verdicts it covers;
// this pins the source, so an extension, a pattern or a case changed on one
// side alone fails here, not on the first cell that happens to hit it (#3147).
func TestProseLedRuleMatchesStatusgen(t *testing.T) {
	ours := portedDecls(t, "verifycmd.go", nil)
	theirs := portedDecls(t, statusgenRowsPath, map[string]string{"verifyCommandMarker": "Marker"})
	for _, name := range portedFromStatusgen {
		o, s := ours[name], theirs[name]
		if o == "" || s == "" {
			t.Fatalf("could-not-check: %s not found (verifycmd %t, statusgen %t)", name, o != "", s != "")
		}
		if o != s {
			t.Errorf("%s drifted from statusgen/verifyrows.go:\nverifycmd: %s\nstatusgen: %s", name, o, s)
		}
	}
}

// portedDecls returns, for each name in portedFromStatusgen declared in path,
// its value (a package var) or its body (a function), printed without comments
// and with whitespace collapsed; rename maps an identifier spelled differently
// on the two sides.
func portedDecls(t *testing.T, path string, rename map[string]string) map[string]string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("could-not-check: %s does not parse: %v", path, err)
	}
	want := map[string]bool{}
	for _, n := range portedFromStatusgen {
		want[n] = true
	}
	render := func(n ast.Node) string {
		ast.Inspect(n, func(x ast.Node) bool {
			if id, ok := x.(*ast.Ident); ok && rename[id.Name] != "" {
				id.Name = rename[id.Name]
			}
			return true
		})
		var b strings.Builder
		if err := printer.Fprint(&b, fset, n); err != nil {
			t.Fatalf("could-not-check: print %s: %v", path, err)
		}
		return strings.Join(strings.Fields(b.String()), " ")
	}
	out := map[string]string{}
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil && want[d.Name.Name] {
				out[d.Name.Name] = render(d.Body)
			}
		case *ast.GenDecl:
			for _, s := range d.Specs {
				vs, ok := s.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, n := range vs.Names {
					if want[n.Name] && i < len(vs.Values) {
						out[n.Name] = render(vs.Values[i])
					}
				}
			}
		}
	}
	return out
}
