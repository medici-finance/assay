package container

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// forbiddenTokens are docker-exec CLI flags that would let this driver widen
// an exec beyond the container's own user and environment (the container driver SPOF
// layer 2 — the exec runs with the container's OWN user and env, never an
// override). This is the internal/custody/norest_test.go pattern applied
// here: a SOURCE audit, because what must hold is that NO code path can ever
// construct one of these flags — a property of the code, not of one run.
var forbiddenTokens = map[string]bool{
	"-u":           true,
	"--user":       true,
	"-e":           true,
	"--env":        true,
	"--privileged": true,
}

// TestDriverNeverOverridesUserOrEnv asserts no non-test .go file in this
// package contains a string literal naming a docker-exec user/env/privileged
// override flag. A runtime happy-path test could pass today and silently
// regress tomorrow if a later change threads a -u/-e/--privileged flag
// through a code path that test's fixtures do not exercise; a source-level
// ban is checkable at build time regardless of which paths a test happens to
// run.
func TestDriverNeverOverridesUserOrEnv(t *testing.T) {
	for _, name := range nonTestGoFiles(t, ".") {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		if hits := forbiddenTokenHits(file); len(hits) > 0 {
			t.Errorf("%s contains %v — the exec must run with the container's OWN user and environment, never an override (the container driver SPOF layer 2)", name, hits)
		}
	}
}

// TestDriverNeverOverridesUserOrEnvCatchesAViolation is the positive control
// (the altitude panes norest_test.go / TestNoTargetShapedRequestCatchesAnAddressField
// pattern): it proves the walk actually flags a -u override rather than
// vacuously passing because the walk itself is broken.
func TestDriverNeverOverridesUserOrEnvCatchesAViolation(t *testing.T) {
	const src = `package container
func bad() []string {
	return []string{"exec", "-u", "root", "some-container", "bash"}
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "synthetic.go", src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse synthetic fixture: %v", err)
	}
	if hits := forbiddenTokenHits(file); len(hits) == 0 {
		t.Fatal("the walk did not flag a synthetic -u override — the audit is not discriminating")
	}
}

// forbiddenTokenHits returns the forbidden flag literals present anywhere in
// file's string literals.
func forbiddenTokenHits(file *ast.File) []string {
	var hits []string
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		v, err := stringLitValue(lit.Value)
		if err != nil {
			return true
		}
		if forbiddenTokens[v] {
			hits = append(hits, v)
		}
		return true
	})
	return hits
}

// stringLitValue unquotes a Go string literal's raw token text.
func stringLitValue(raw string) (string, error) {
	if len(raw) >= 2 {
		return strings.Trim(raw, "\"`"), nil
	}
	return raw, nil
}

// nonTestGoFiles lists the package's NON-test .go files: the audit binds the
// shipped code, and this file legitimately names the forbidden flags in its
// own audit machinery and would otherwise flag itself.
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
