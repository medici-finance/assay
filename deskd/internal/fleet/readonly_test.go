package fleet

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPackageMakesNoDirectMutatingCall is the mechanical evidence for the
// brief's Verify row 3 (no server routes) and its own "prove it with a
// readonly_test.go" design note: this package reaches GitHub ONLY through
// [adapter.Client] (whose transport is structurally read-only —
// internal/adapter/readonly.go), never through its own os/exec or a raw
// net/http mutating verb. It is an AST audit, not a grep, mirroring the
// house pattern in internal/custody/norest_test.go and internal/purity: a
// grep cannot tell a method NAME from a struct field or a doc-comment
// sentence, so it either misses a violation buried in an expression or
// fires on prose.
//
// This is a tripwire against a FUTURE change reaching for its own HTTP
// client or shelling out, bypassing the one read-only transport every
// other GitHub read in this codebase goes through
// ([adapter.NewReadOnlyTransport]).
func TestPackageMakesNoDirectMutatingCall(t *testing.T) {
	forbiddenHTTPMethods := map[string]bool{
		"MethodPost": true, "MethodPut": true, "MethodPatch": true, "MethodDelete": true,
	}
	forbiddenOSExecCalls := map[string]bool{
		"Command": true, "CommandContext": true,
	}

	for _, name := range goFiles(t, ".") {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}

		// No os/exec import at all: this package fetches over the network
		// (via internal/adapter), never by shelling out to git/gh.
		for _, imp := range file.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if path == "os/exec" {
				t.Errorf("%s imports os/exec — the fleet map is a live GitHub read through internal/adapter's read-only client, never a shell-out", name)
			}
		}

		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			switch pkg.Name {
			case "http":
				if forbiddenHTTPMethods[sel.Sel.Name] {
					t.Errorf("%s references http.%s — this package must never spell a mutating HTTP verb; every read goes through adapter.Client's read-only transport", name, sel.Sel.Name)
				}
			case "exec":
				if forbiddenOSExecCalls[sel.Sel.Name] {
					t.Errorf("%s calls exec.%s — no shelling out from the fleet map", name, sel.Sel.Name)
				}
			}
			return true
		})
	}
}

func goFiles(t *testing.T, dir string) []string {
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
		out = append(out, filepath.Join(dir, e.Name()))
	}
	return out
}
