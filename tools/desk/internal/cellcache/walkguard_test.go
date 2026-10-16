package cellcache

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// walkingCalls returns "<func>" for every call to cellcache.CheckRecorded in
// one Go source file, resolving the import under
// whatever local name the file gives it.
func walkingCalls(fset *token.FileSet, f *ast.File) []string {
	local := ""
	for _, imp := range f.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || !strings.HasSuffix(path, "/internal/cellcache") {
			continue
		}
		local = "cellcache"
		if imp.Name != nil {
			local = imp.Name.Name
		}
	}
	if local == "" || local == "_" || local == "." {
		return nil
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
			if !ok || sel.Sel.Name != "CheckRecorded" {
				return true
			}
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == local {
				out = append(out, fn.Name.Name)
			}
			return true
		})
	}
	return out
}

// Class guard. Defect class: a boot, launch, dispatch or sweep path that calls
// the walking cellcache.CheckRecorded, so every window start takes the cache
// lock and walks the whole cache. CheckRecorded has no maintenance verb outside
// this package: it exists to be the locked fall-through of AdmitRecorded, so
// any caller elsewhere in tools/desk is a new instance of the class. (The
// sibling guard for cellcache.Check lives with the launch-admission fix and
// allows only the explicit "cellctl cache" maintenance verb.)
//
// regression: #3102
func TestCheckRecordedHasNoCallers(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	var bad []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel == "internal/cellcache" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		f, perr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if perr != nil {
			t.Fatalf("parse %s: %v", rel, perr)
		}
		for _, fn := range walkingCalls(fset, f) {
			bad = append(bad, rel+":"+fn)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(bad)
	if len(bad) > 0 {
		t.Fatalf("calls the walking cellcache.CheckRecorded outside the cellcache package "+
			"(use cellcache.AdmitRecorded on a boot, launch, dispatch or sweep path): %v", bad)
	}
}

// The guard's matcher could silently stop matching (an import rename, a
// changed selector shape) and report clean forever, so it carries a positive
// control: a planted caller, under a plain and an aliased import, must be
// flagged.
//
// regression: #3102
func TestCheckRecordedGuardPlant(t *testing.T) {
	for _, imp := range []string{`cellcache "example.com/m/tools/desk/internal/cellcache"`, `cc "example.com/m/tools/desk/internal/cellcache"`} {
		name := strings.Fields(imp)[0]
		src := "package p\nimport " + imp + "\n" +
			"func sweep(r string) { " + name + ".CheckRecorded(r, false) }\n" +
			"func other(p " + name + ".Policy) { " + name + ".Check(p, false) }\n" +
			"func fine(p " + name + ".Policy) { " + name + ".Admit(p, false) }\n"
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, "plant.go", src, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		got := walkingCalls(fset, f)
		if len(got) != 1 || got[0] != "sweep" {
			t.Fatalf("planted walking callers under import %q not flagged: %v", imp, got)
		}
	}
}
