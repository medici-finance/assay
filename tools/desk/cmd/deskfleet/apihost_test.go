package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The GitHub API host literal has one home, deskkit.GitHubAPIBase; a cmd package sources it
// from there and never restates the string (desktools-v2 seam contract, class (d)). deskfleet
// restated it in defaultEnv when it landed (#1864). This test pins this package only: no
// non-test source here carries the literal in code. Comments are not code, so a comment naming
// the host is not flagged.

const apiHost = "api.github.com"

// hostLiterals returns the position of every string literal in src that contains apiHost.
// It walks the parsed AST, so comments are never seen.
func hostLiterals(t *testing.T, fset *token.FileSet, name string, src any) []token.Position {
	t.Helper()
	f, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	var hits []token.Position
	ast.Inspect(f, func(n ast.Node) bool {
		if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING && strings.Contains(lit.Value, apiHost) {
			hits = append(hits, fset.Position(lit.Pos()))
		}
		return true
	})
	return hits
}

func TestNoAPIHostLiteral(t *testing.T) {
	// Positive control: the matcher must flag a planted literal and ignore a comment, so a
	// broken matcher fails here instead of reporting the package clean.
	fset := token.NewFileSet()
	planted := "package p\n\n// " + apiHost + " in a comment is fine\nvar base = \"https://" + apiHost + "\"\n"
	if hits := hostLiterals(t, fset, "planted.go", planted); len(hits) != 1 || hits[0].Line != 4 {
		t.Fatalf("positive control: want exactly one hit on line 4 of the planted source, got %v", hits)
	}

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	scanned := 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		scanned++
		for _, pos := range hostLiterals(t, fset, name, src) {
			t.Errorf("%s: string literal restates %s; source it from deskkit.GitHubAPIBase", pos, apiHost)
		}
	}
	if scanned == 0 {
		t.Fatal("scanned no non-test sources: the glob matched nothing, so the check never looked")
	}
}
