package local

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// norest_test.go is internal/terminal/norest_test.go's SCROLLBACK-AT-REST
// audit, scoped to this package (the local driver Verify row 7: "the audit covers the
// local sub-package"). It is a SOURCE audit, not a happy-path runtime check,
// because what must hold is that no code path in the local driver can spool
// PTY output to disk — a property of the code, not of one run. The terminal
// package's audit also walks driver subpackages; this local copy checks the
// same invariant when only this package's tests are run.
var scrollbackWriters = map[string]bool{
	"WriteFile":  true,
	"Create":     true,
	"CreateTemp": true,
	"OpenFile":   true,
	"Open":       true,
	"MkdirAll":   true,
	"Mkdir":      true,
	"ReadFile":   true,
}

// TestServiceHoldsNoScrollbackAtRest asserts this package contains no
// filesystem write and no filesystem read via the standard os.* calls that
// could put a PTY byte on disk. The local driver moves bytes between a PTY
// and the registry's in-memory ring only.
func TestServiceHoldsNoScrollbackAtRest(t *testing.T) {
	for _, name := range nonTestGoFiles(t, ".") {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != "os" {
				return true
			}
			if scrollbackWriters[sel.Sel.Name] {
				t.Errorf("%s calls os.%s — the terminal scrollback is memory-only and is NEVER written under the data dir "+
					"(spec §5.1). A filesystem call in this package is the at-rest path the audit forbids.", name, sel.Sel.Name)
			}
			return true
		})
	}
}

// nonTestGoFiles lists the package's NON-test .go files: the audit binds the
// shipped code, and the test files (this one included) legitimately name os.*
// in their audit machinery.
func nonTestGoFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		out = append(out, e.Name())
	}
	return out
}
