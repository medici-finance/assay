//go:build darwin || linux

package cellprocess

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/cellcache"
)

func TestCacheProcessCustody(t *testing.T) {
	cell, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p, err := cellcache.Resolve(cell, func(k string) string {
		if k == "CELL_GO_CACHE" {
			return "on"
		}
		if k == "CELL_GO_CACHE_MIN_FREE" {
			return "0"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, uncertain := range []bool{false, true} {
		code, got, err := withCache(p.Env(), func() (int, bool, error) {
			r, err := cellcache.Check(*p, true)
			if err != nil || len(r.SkippedActive) != 1 {
				t.Fatalf("child ran without custody: %+v %v", r, err)
			}
			return 0, uncertain, nil
		})
		if code != 0 || got != uncertain || err != nil {
			t.Fatal(code, got, err)
		}
		r, err := cellcache.Check(*p, true)
		if err != nil {
			t.Fatal(err)
		}
		want := 0
		if uncertain {
			want = 1
		}
		if len(r.SkippedActive) != want {
			t.Fatal("lost uncertain cache custody", r)
		}
		if uncertain {
			if err = cellcache.Recover(*p, true); err != nil {
				t.Fatal(err)
			}
		}
	}
	p.Floor = ^uint64(0)
	called := false
	_, _, err = withCache(p.Env(), func() (int, bool, error) { called = true; return 0, false, nil })
	var held *cellcache.Deferred
	if called || !errors.As(err, &held) {
		t.Fatal("storage pressure launched a child", err)
	}
}

// Defect class: raw process launch outside cache custody. Enumerate all calls
// to the only raw runner, and require each exported launch wrapper to enroll.
// A planted second caller exercises the matcher as a positive control.
func rawCacheBypasses(name string, source []byte) ([]string, error) {
	f, err := parser.ParseFile(token.NewFileSet(), name, source, 0)
	if err != nil {
		return nil, err
	}
	var bad []string
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		hasCustody := false
		raw := false
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			id, ok := call.Fun.(*ast.Ident)
			if ok {
				if id.Name == "run" {
					raw = true
				}
				if id.Name == "withCache" {
					hasCustody = true
				}
			}
			return true
		})
		if raw && (fn.Name.Name != "Run" && fn.Name.Name != "RunInteractive" || !hasCustody) {
			bad = append(bad, fn.Name.Name)
		}
	}
	return bad, nil
}
func TestCacheClassGuard(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		bad, err := rawCacheBypasses(name, b)
		if err != nil || len(bad) > 0 {
			t.Fatalf("uncustodied raw launch %s: %v %v", name, bad, err)
		}
	}
	planted := []byte("package cellprocess\nfunc secondSite(){ run(nil,nil,nil,\"\",nil,nil,nil) }\n")
	bad, err := rawCacheBypasses("planted.go", planted)
	if err != nil || len(bad) != 1 || bad[0] != "secondSite" {
		t.Fatalf("class guard missed planted second site: %v %v", bad, err)
	}
}
