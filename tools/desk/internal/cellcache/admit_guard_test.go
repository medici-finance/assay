package cellcache

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// checkCallers returns "file:func" for every call to cellcache.Check in the Go
// source src that is not in a function named in allowed.
func checkCallers(t *testing.T, name, src string, allowed map[string]bool) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), name, src, 0)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	var out []string
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Check" {
				return true
			}
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "cellcache" && !allowed[fn.Name.Name] {
				out = append(out, name+":"+fn.Name.Name)
			}
			return true
		})
	}
	return out
}

// Check takes the cache custody lock and walks build/ and mod/ in full. A
// launch or dispatch path that calls it serialises every desk behind that walk
// even when nothing needs cleaning; those paths use Admit, which runs Check
// only when the lock-free pre-check cannot clear the cache. The one production
// caller of Check outside this package is the explicit maintenance verb
// (`cellctl cache <cell> status|clean`), a function named cmdCache.
//
// regression: #3045
func TestCacheLaunchPathsUseAdmit(t *testing.T) {
	// Positive control: the matcher must flag a planted second instance, or a
	// broken guard would report clean.
	planted := "package main\nimport \"x/cellcache\"\nfunc launchAdmission() { cellcache.Check(p, false) }\nfunc cmdCache() { cellcache.Check(p, true) }\n"
	got := checkCallers(t, "planted.go", planted, map[string]bool{"cmdCache": true})
	if len(got) != 1 || got[0] != "planted.go:launchAdmission" {
		t.Fatalf("guard matcher did not flag the planted caller: %v", got)
	}
	allowed := map[string]bool{"cmdCache": true}
	module := filepath.Join("..", "..")
	var found []string
	err := filepath.WalkDir(module, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path == filepath.Join(module, "internal", "cellcache") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, err := readSource(path)
		if err != nil {
			return err
		}
		found = append(found, checkCallers(t, path, b, allowed)...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) > 0 {
		t.Fatalf("launch/dispatch-path callers must use cellcache.Admit, not the locked cellcache.Check: %v", found)
	}
}

func readSource(path string) (string, error) {
	b, err := os.ReadFile(path)
	return string(b), err
}
