package humanid

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The policy is reachable from MCP, so its entire production package is kept
// free of I/O, credential custody and writes by an exact pure-import allowlist.
// regression: #3669
func TestPolicyHasNoEffectfulImports(t *testing.T) {
	allowed := map[string]bool{"errors": true, "regexp": true, "strings": true, "sync": true}
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		for _, i := range f.Imports {
			name, _ := strconv.Unquote(i.Path.Value)
			if !allowed[name] {
				t.Errorf("effectful or unreviewed policy import: %s", name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Count selector references, not only calls: aliasing the initializer into a
// request handler is forbidden too. Dot imports cannot hide an initializer.
// regression: #3669
func TestOnlyCommandStartupInitializesPolicy(t *testing.T) {
	const module = "github.com/medici-finance/assay/deskd/internal/humanid"
	count := 0
	err := filepath.WalkDir("../..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		alias := ""
		for _, i := range f.Imports {
			name, _ := strconv.Unquote(i.Path.Value)
			if name == module {
				alias = "humanid"
				if i.Name != nil {
					alias = i.Name.Name
				}
				if alias == "." {
					t.Errorf("dot-imported policy: %s", path)
				}
			}
		}
		if alias == "" {
			return nil
		}
		for _, decl := range f.Decls {
			// The checked initializer must be a direct startup statement, not
			// a closure that could later run from a request or lazy callback.
			direct := map[*ast.SelectorExpr]bool{}
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil {
				for _, stmt := range fn.Body.List {
					check, ok := stmt.(*ast.IfStmt)
					if !ok || check.Init == nil {
						continue
					}
					ast.Inspect(check.Init, func(n ast.Node) bool {
						if _, nested := n.(*ast.FuncLit); nested {
							return false
						}
						if sel, ok := n.(*ast.SelectorExpr); ok {
							direct[sel] = true
						}
						return true
					})
				}
			}
			ast.Inspect(decl, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Configure" {
					return true
				}
				id, ok := sel.X.(*ast.Ident)
				if !ok || id.Name != alias {
					return true
				}
				count++
				fn, ok := decl.(*ast.FuncDecl)
				if filepath.ToSlash(path) != "../../cmd/deskd/main.go" || !ok || fn.Name.Name != "main" || !direct[sel] {
					t.Errorf("policy initializer outside command startup: %s", path)
				}
				return true
			})
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("initializer references = %d, want 1", count)
	}
}
