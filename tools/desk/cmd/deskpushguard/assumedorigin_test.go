package main

// assumedorigin_test.go — the class guard for #1201.
//
// The defect class is "a push-time check that names the remote itself instead of using the
// one git handed the hook": a Go string literal in this package's non-test code that spells
// the remote `origin` — the bare name, a `origin/<branch>` short ref, a
// `refs/remotes/origin/...` ref or a `remote.origin.*` config key. Each such literal is a
// place where the guard would judge some remote other than the pushed one. The fix threads
// args[0] through instead, so the allowed count of these literals is zero.
//
// The one deliberate exception is the STRAY LOCAL branch `refs/heads/origin/main`: that is
// the name of a specific local-branch hazard (a bare `origin/main` shadowed by a local
// branch), not a remote being judged, so literals that mention it are allowed.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// assumedOrigin reports whether a string literal's value names the remote `origin`.
func assumedOrigin(v string) bool {
	return v == "origin" ||
		strings.HasPrefix(v, "origin/") ||
		strings.Contains(v, "refs/remotes/origin/") ||
		strings.Contains(v, "remote.origin.")
}

// originLiterals parses one Go source and returns "file:line: literal" for every string
// literal that names the remote `origin`.
func originLiterals(t *testing.T, fset *token.FileSet, name string, src any) []string {
	t.Helper()
	f, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	var hits []string
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		v, err := strconv.Unquote(lit.Value)
		if err != nil {
			return true
		}
		if assumedOrigin(v) {
			hits = append(hits, fset.Position(lit.Pos()).String()+": "+lit.Value)
		}
		return true
	})
	return hits
}

// TestNoAssumedOriginLiteral walks every non-test Go file in this package and fails naming
// any string literal that spells the remote `origin`.
func TestNoAssumedOriginLiteral(t *testing.T) {
	// Positive control: a planted repeat of the defect must be flagged, so a matcher that
	// silently stops matching fails here instead of reporting the package clean.
	planted := `package main
func planted(dir string) { _, _ = resolveRemoteMain(dir, "origin") }`
	if hits := originLiterals(t, token.NewFileSet(), "planted.go", planted); len(hits) != 1 {
		t.Fatalf("positive control: the guard flagged %d literal(s) in a planted "+
			"resolveRemoteMain(dir, \"origin\") call, want 1: %v", len(hits), hits)
	}

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
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
		for _, h := range originLiterals(t, fset, name, src) {
			t.Errorf("%s — names the remote `origin` instead of using the pushed remote "+
				"from the hook's first argument (#1201)", h)
		}
	}
	if scanned == 0 {
		t.Fatal("no non-test Go files scanned — the guard would pass vacuously")
	}
}
