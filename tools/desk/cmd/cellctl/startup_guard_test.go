package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

func startupWalkSites(name string, src []byte) ([]string, error) {
	f, err := parser.ParseFile(token.NewFileSet(), name, src, 0)
	if err != nil {
		return nil, err
	}
	cacheNames := map[string]bool{"cellcache": true}
	for _, spec := range f.Imports {
		if strings.HasSuffix(strings.Trim(spec.Path.Value, "\""), "/cellcache") && spec.Name != nil {
			cacheNames[spec.Name.Name] = true
		}
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
			if ok && sel.Sel.Name == "Sweep" && fn.Name.Name != "sweepScratch" {
				bad = append(bad, name+":"+fn.Name.Name)
			}
			if ok {
				pkg, isPackage := sel.X.(*ast.Ident)
				if isPackage && cacheNames[pkg.Name] {
					switch sel.Sel.Name {
					case "Maintain":
						if fn.Name.Name != "cadenceCacheMaintenance" {
							bad = append(bad, name+":"+fn.Name.Name)
						}
					case "Check", "CheckRecorded", "Admit", "AdmitRecorded", "Wipe":
						if fn.Name.Name != "cmdCache" && fn.Name.Name != "resetCell" {
							bad = append(bad, name+":"+fn.Name.Name)
						}
					}
				}
			}
			return true
		})
	}
	return bad, nil
}
func startupWalkGuard(t *testing.T) {
	planted, err := startupWalkSites("second.go", []byte("package main;import cache \"github.com/medici-finance/assay/tools/desk/internal/cellcache\";func secondStartup(){s.Sweep(p,true)};func secondCacheStartup(){cache.Check(p,true)};func secondMaintainStartup(){cache.Maintain(ctx,p)}"))
	must(t, err)
	if len(planted) != 3 || planted[0] != "second.go:secondStartup" || planted[1] != "second.go:secondCacheStartup" || planted[2] != "second.go:secondMaintainStartup" {
		t.Fatal("planted second walking caller escaped", planted)
	}
	entries, err := os.ReadDir(".")
	must(t, err)
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		src, err := os.ReadFile(entry.Name())
		must(t, err)
		bad, err := startupWalkSites(entry.Name(), src)
		must(t, err)
		if len(bad) > 0 {
			t.Fatalf("recursive history walk outside explicit maintenance: %v", bad)
		}
	}
}
