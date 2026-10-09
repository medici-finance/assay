package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The operator config context is an expectation the house check trusts, so it
// may be produced at ONE site: codexCommandEnvironment, storing only the value
// captureOperatorConfig returned (which refuses cell-owned candidates). Readers
// are the resolver and the capture itself. Any other reference is a new way to
// mint the expectation and is reported by name.
var operatorContextAllowed = map[string]bool{
	"homeresolve.go:const":                         true,
	"homeresolve.go:operatorConfigHomeFor":         true,
	"homeresolve.go:captureOperatorConfig":         true,
	"codex_environment.go:codexCommandEnvironment": true,
}

func isOperatorKey(n ast.Node) bool {
	switch v := n.(type) {
	case *ast.Ident:
		return v.Name == "operatorConfigKey"
	case *ast.BasicLit:
		return v.Kind == token.STRING && strings.Contains(v.Value, "CELLCTL_OPERATOR_CONFIG_HOME")
	}
	return false
}

// operatorContextViolations parses each named source and returns every site
// outside the allow-list, plus any write in the producer whose value is not
// the captured result.
func operatorContextViolations(t *testing.T, sources map[string]any) []string {
	t.Helper()
	var bad []string
	for name, src := range sources {
		f, err := parser.ParseFile(token.NewFileSet(), name, src, 0)
		must(t, err)
		for _, d := range f.Decls {
			site := name + ":const"
			var body ast.Node = d
			if fn, ok := d.(*ast.FuncDecl); ok {
				site = name + ":" + fn.Name.Name
				if fn.Body == nil {
					continue
				}
				body = fn.Body
			}
			captured := map[string]bool{}
			found := false
			ast.Inspect(body, func(n ast.Node) bool {
				if isOperatorKey(n) {
					found = true
				}
				if as, ok := n.(*ast.AssignStmt); ok {
					for i, lhs := range as.Lhs {
						if call, ok := as.Rhs[0].(*ast.CallExpr); ok && len(as.Rhs) == 1 && i == 0 {
							if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "captureOperatorConfig" {
								if l, ok := lhs.(*ast.Ident); ok {
									captured[l.Name] = true
								}
							}
						}
						if ix, ok := lhs.(*ast.IndexExpr); ok && isOperatorKey(ix.Index) {
							v, ok := as.Rhs[i%len(as.Rhs)].(*ast.Ident)
							if !ok || !captured[v.Name] {
								bad = append(bad, site+": stores a value not returned by captureOperatorConfig")
							}
						}
					}
				}
				if kv, ok := n.(*ast.KeyValueExpr); ok && isOperatorKey(kv.Key) {
					bad = append(bad, site+": composite literal sets the operator context")
				}
				return true
			})
			if found && !operatorContextAllowed[site] {
				bad = append(bad, site+": references the operator context")
			}
		}
	}
	sort.Strings(bad)
	return bad
}

func TestOperatorContextChoke(t *testing.T) {
	files, err := filepath.Glob("*.go")
	must(t, err)
	sources := map[string]any{}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(name)
		must(t, err)
		sources[name] = raw
	}
	if bad := operatorContextViolations(t, sources); len(bad) != 0 {
		t.Fatalf("operator config context minted outside the capture site:\n%s", strings.Join(bad, "\n"))
	}
	// Positive control: a planted second producer must be reported.
	planted := operatorContextViolations(t, map[string]any{"planted.go": `package main
func plant(v map[string]string) { v[operatorConfigKey] = "/elsewhere" }`})
	if len(planted) == 0 {
		t.Fatal("guard missed a planted operator-context producer")
	}
}
