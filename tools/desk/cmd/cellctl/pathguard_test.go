package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// oldPathChecks inventories the whole package, including any future caller.
func oldPathChecks(name string, src any) ([]string, error) {
	f, err := parser.ParseFile(token.NewFileSet(), name, src, 0)
	if err != nil {
		return nil, err
	}
	var sites []string
	ast.Inspect(f, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok || len(c.Args) != 2 {
			return true
		}
		sel, ok := c.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "HasPrefix" {
			return true
		}
		lit, ok := c.Args[1].(*ast.BasicLit)
		if ok && lit.Value == `"/"` {
			sites = append(sites, name)
		}
		return true
	})
	return sites, nil
}
func TestCellPathClassGuard(t *testing.T) {
	files, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	scanned := 0
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".go") || strings.HasSuffix(f.Name(), "_test.go") {
			continue
		}
		scanned++
		sites, err := oldPathChecks(f.Name(), nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(sites) > 0 {
			t.Errorf("POSIX-only path guard remains: %v", sites)
		}
	}
	if scanned == 0 {
		t.Fatal("no source scanned")
	}
	sites, err := oldPathChecks("example.go", `package example; func bad(p string) bool { return strings.HasPrefix(p,"/") }`)
	if err != nil || len(sites) != 1 {
		t.Fatalf("positive control: %v %v", sites, err)
	}
}
