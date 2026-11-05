package cellcache

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

func cacheInventorySites(name string, src []byte) ([]string, error) {
	f, err := parser.ParseFile(token.NewFileSet(), name, src, 0)
	if err != nil {
		return nil, err
	}
	allowed := map[string]map[string]bool{
		"check":          {"Check": true, "CheckRecorded": true},
		"checkMode":      {"check": true, "Wipe": true},
		"measure":        {"checkMode": true},
		"BoundedWalkDir": {"measure": true, "verifiedTreeSize": true, "removeTree": true},
		"WalkDir":        {"measure": true, "verifiedTreeSize": true, "removeTree": true},
	}
	var bad []string
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if callers, guarded := allowed[sel.Sel.Name]; guarded && !callers[fn.Name.Name] {
				bad = append(bad, name+":"+fn.Name.Name+":"+sel.Sel.Name)
			}
			return true
		})
	}
	return bad, nil
}
func cacheInventoryGuard(t *testing.T) {
	planted, err := cacheInventorySites("second.go", []byte("package cellcache;func otherAdmission(m *manager){m.check(false);fs.WalkDir(nil,\".\",nil)}"))
	if err != nil {
		t.Fatal(err)
	}
	if len(planted) != 2 {
		t.Fatal("inventory guard missed planted second sites", planted)
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		src, err := os.ReadFile(entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		bad, err := cacheInventorySites(entry.Name(), src)
		if err != nil {
			t.Fatal(err)
		}
		if len(bad) > 0 {
			t.Fatal("inventory outside maintenance owner", bad)
		}
	}
}

// regression: #3353
func TestCellPrepBounded(t *testing.T) { t.Run("walking_callers", cacheInventoryGuard) }
