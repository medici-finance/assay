package answer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// TestExactlyOneWritePath is the structural guarantee behind the answer flow's "the
// answer path is the ONE write path". It parses every non-test file in this
// package and asserts that OUTBOUND HTTP REQUESTS are constructed in exactly one
// file — github.go, where HumanWriter lives. A second write path anywhere in
// the package would build an http.NewRequest[WithContext] in another file and
// fail this test, so the property is checked on every build rather than argued
// in a review.
//
// It is the sibling of internal/mcp's authority test: that one proves the MCP
// surface has NO operator write path; this one proves the answer surface has
// EXACTLY ONE.
func TestExactlyOneWritePath(t *testing.T) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("could not read package dir: %v", err)
	}

	requestBuilders := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, perr := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if perr != nil {
			t.Fatalf("parse %s: %v", name, perr)
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
			if !ok || pkg.Name != "http" {
				return true
			}
			if sel.Sel.Name == "NewRequest" || sel.Sel.Name == "NewRequestWithContext" {
				requestBuilders[name] = true
			}
			return true
		})
	}

	if len(requestBuilders) == 0 {
		t.Fatal("no file builds an outbound request — the check is not load-bearing (did github.go stop using net/http?)")
	}
	for file := range requestBuilders {
		if file != "github.go" {
			t.Fatalf("outbound request built in %s — the answer path must construct platform writes only in github.go (HumanWriter), so there is exactly ONE write path", file)
		}
	}
}

// TestServiceHoldsOneWriter proves the Service funnels through a single writer
// field: there is no second writer to route a bypass through.
func TestServiceHoldsOneWriter(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "answer.go", nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse answer.go: %v", err)
	}
	writerFields := 0
	ast.Inspect(file, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok || ts.Name.Name != "Service" {
			return true
		}
		st, ok := ts.Type.(*ast.StructType)
		if !ok {
			return true
		}
		for _, f := range st.Fields.List {
			if id, ok := f.Type.(*ast.Ident); ok && id.Name == "platformWriter" {
				writerFields += len(f.Names)
			}
		}
		return true
	})
	if writerFields != 1 {
		t.Fatalf("Service holds %d platformWriter fields, want exactly 1 (the single write path)", writerFields)
	}
}
