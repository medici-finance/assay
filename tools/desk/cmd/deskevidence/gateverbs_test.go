package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestPublicRepoGateVerbsMatchCallers keeps the public-repo gate's refusal honest about
// its reach. The refusal names deskkit.PublicRepoGateVerbs as the commands the
// authorization covers; this test walks every non-test source file directly under cmd/
// and collects the commands whose code references deskkit.PublicRepoGate. The two sets
// must be equal, so a command that gains or drops the gate without the list following it
// fails here instead of leaving the refusal wrong.
//
// regression: #5436
func TestPublicRepoGateVerbsMatchCallers(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "*", "*.go"))
	if err != nil {
		t.Fatalf("glob the command tree: %v", err)
	}
	fset := token.NewFileSet()
	scanned := 0
	callers := map[string]bool{}
	for _, f := range files {
		if isTestFile(f) {
			continue
		}
		scanned++
		af, perr := parser.ParseFile(fset, f, nil, 0)
		if perr != nil {
			t.Fatalf("parse %s: %v", f, perr)
		}
		cmd := filepath.Base(filepath.Dir(f))
		ast.Inspect(af, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "PublicRepoGate" {
				return true
			}
			if x, ok := sel.X.(*ast.Ident); ok && x.Name == "deskkit" {
				callers[cmd] = true
			}
			return true
		})
	}
	if scanned == 0 {
		t.Fatal("scanned 0 non-test source files under cmd/ — the enumeration proved nothing")
	}

	var got []string
	for c := range callers {
		got = append(got, c)
	}
	slices.Sort(got)
	want := slices.Clone(deskkit.PublicRepoGateVerbs)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("commands referencing deskkit.PublicRepoGate = %v, but deskkit.PublicRepoGateVerbs "+
			"(the commands the gate's refusal names) = %v — update the list so the refusal "+
			"names exactly the commands that run the gate", got, want)
	}
}
