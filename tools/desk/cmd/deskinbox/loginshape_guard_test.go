package main

// loginshape_guard_test.go — the CLASS guard for #1797.
//
// Defect class: a comment author login reaching format.go in a wire shape other than the
// oracle's. The oracle's desk-note selector and label run over gh's author.login, and every
// forge read deskinbox makes hands it some OTHER shape (REST: `<slug>[bot]`). The one
// sanctioned crossing is ghLogin (detail.go). This test walks every shipping file of the
// package and fails, naming the site, on any `comment` whose Author is set — in a composite
// literal or by assignment — to anything but a ghLogin(...) call. A second comment reader (a
// paginated GraphQL read, a GitLab notes reader) that forgets the normalisation is red here
// before it can drift the walk or the page.
//
// Its positive control is testdata/loginshape_plant.go.txt: two planted raw assignments the
// guard MUST flag, so a matcher that silently stopped matching fails instead of reporting
// clean.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// authorSitesBypassingGHLogin parses one Go source and returns "file:line" for every site
// that sets a comment's Author without going through ghLogin.
func authorSitesBypassingGHLogin(t *testing.T, path string) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	isGHLogin := func(e ast.Expr) bool {
		call, ok := e.(*ast.CallExpr)
		if !ok {
			return false
		}
		id, ok := call.Fun.(*ast.Ident)
		return ok && id.Name == "ghLogin"
	}
	var out []string
	flag := func(n ast.Node) {
		p := fset.Position(n.Pos())
		out = append(out, fmt.Sprintf("%s:%d", filepath.Base(p.Filename), p.Line))
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CompositeLit:
			id, ok := x.Type.(*ast.Ident)
			if !ok || id.Name != "comment" {
				return true
			}
			for i, el := range x.Elts {
				if kv, ok := el.(*ast.KeyValueExpr); ok {
					if k, ok := kv.Key.(*ast.Ident); ok && k.Name == "Author" && !isGHLogin(kv.Value) {
						flag(kv)
					}
					continue
				}
				// Positional literal: Author is comment's first field.
				if i == 0 && !isGHLogin(el) {
					flag(el)
				}
			}
		case *ast.AssignStmt:
			for i, lhs := range x.Lhs {
				sel, ok := lhs.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Author" || i >= len(x.Rhs) {
					continue
				}
				if !isGHLogin(x.Rhs[i]) {
					flag(x)
				}
			}
		}
		return true
	})
	return out
}

func TestCommentAuthorsCrossGHLogin(t *testing.T) {
	// Positive control first: a guard that cannot see a plant proves nothing below.
	planted := authorSitesBypassingGHLogin(t, filepath.Join("testdata", "loginshape_plant.go.txt"))
	if len(planted) != 2 {
		t.Fatalf("positive control: want exactly the 2 planted sites flagged, got %d: %v — the guard's matcher is broken",
			len(planted), planted)
	}

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	var shipping []string
	for _, f := range files {
		if !strings.HasSuffix(f, "_test.go") {
			shipping = append(shipping, f)
		}
	}
	if len(shipping) == 0 {
		t.Fatalf("could-not-check: no shipping .go files found from %q", mustGetwd(t))
	}
	sort.Strings(shipping)
	var bad []string
	for _, f := range shipping {
		bad = append(bad, authorSitesBypassingGHLogin(t, f)...)
	}
	if len(bad) > 0 {
		t.Errorf("comment Author set without ghLogin (the oracle's gh login shape, #1797) at: %s\n"+
			"every comment reader must normalise its login through ghLogin (detail.go) before format.go sees it",
			strings.Join(bad, ", "))
	}
}

func mustGetwd(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		return "?"
	}
	return wd
}
