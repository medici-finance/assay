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
			// Every Sweep* entry point walks history: the explicit command and the
			// supervisor's deferred maintenance are the only callers (#5353).
			if ok && strings.HasPrefix(sel.Sel.Name, "Sweep") && fn.Name.Name != "sweepScratch" && fn.Name.Name != "deferredScratchSweep" {
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
					case "Check":
						// upCacheClean is `up`'s one clean after a reclaimable
						// deferral (#6117); its only caller is pinned below.
						if fn.Name.Name != "cmdCache" && fn.Name.Name != "resetCell" && fn.Name.Name != "upCacheClean" {
							bad = append(bad, name+":"+fn.Name.Name)
						}
					case "CheckRecorded", "Admit", "AdmitRecorded", "Wipe":
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

// upCleanPins maps each step of `up`'s one cache clean (#6117) to the only
// function allowed to name it: the clean belongs to up's preparation, and
// that preparation to cmdUp. Any reference counts, not only a call, so a
// method value handed to another launch path is caught too.
var upCleanPins = map[string]string{"upCacheClean": "upPrepareLaunch", "upPrepareLaunch": "cmdUp"}

func upCleanSites(name string, src []byte) ([]string, error) {
	f, err := parser.ParseFile(token.NewFileSet(), name, src, 0)
	if err != nil {
		return nil, err
	}
	var bad []string
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if owner, pinned := upCleanPins[sel.Sel.Name]; pinned && fn.Name.Name != owner {
				bad = append(bad, name+":"+fn.Name.Name+"->"+sel.Sel.Name)
			}
			return true
		})
	}
	return bad, nil
}

func startupWalkGuard(t *testing.T) {
	pinned, err := upCleanSites("second.go", []byte("package main;func deskLaunch(){c.upCacheClean(\"x\")};func scratchLaunch(){f := c.upPrepareLaunch;f()};func upPrepareLaunch(){c.upCacheClean(\"x\")};func cmdUp(){c.upPrepareLaunch()}"))
	must(t, err)
	if len(pinned) != 2 || pinned[0] != "second.go:deskLaunch->upCacheClean" || pinned[1] != "second.go:scratchLaunch->upPrepareLaunch" {
		t.Fatal("planted second up-clean caller escaped", pinned)
	}
	planted, err := startupWalkSites("second.go", []byte("package main;import cache \"github.com/medici-finance/assay/tools/desk/internal/cellcache\";func secondStartup(){s.Sweep(p,true)};func secondCacheStartup(){cache.Check(p,true)};func secondMaintainStartup(){cache.Maintain(ctx,p)};func secondPeriodic(){s.SweepIfDue(p,i,true)}"))
	must(t, err)
	if len(planted) != 4 || planted[0] != "second.go:secondStartup" || planted[1] != "second.go:secondCacheStartup" || planted[2] != "second.go:secondMaintainStartup" || planted[3] != "second.go:secondPeriodic" {
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
		bad, err = upCleanSites(entry.Name(), src)
		must(t, err)
		if len(bad) > 0 {
			t.Fatalf("up's cache clean reached from outside cmdUp: %v", bad)
		}
	}
}
