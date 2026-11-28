package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// goGitRoot is the go-git package that holds the raw repository openers.
const goGitRoot = "github.com/go-git/go-git/v5"

// rawOpeners are the go-git root-package functions that open or create a
// repository without gitcore's extension handling. qualgen opens repositories
// only through gitcore.OpenGoGit (#6606).
var rawOpeners = map[string]bool{
	"PlainOpen": true, "PlainOpenWithOptions": true, "Open": true,
	"PlainInit": true, "PlainInitWithOptions": true, "Init": true,
	"PlainClone": true, "PlainCloneContext": true, "Clone": true, "CloneContext": true,
}

// rawOpenerUses returns "file:line name" for every reference to a raw go-git
// opener in src, under any import name. A dot import of the go-git root
// package is itself reported: it would hide an opener call from the check.
func rawOpenerUses(name string, src []byte) ([]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	local := ""
	var hits []string
	for _, imp := range f.Imports {
		path, _ := strconv.Unquote(imp.Path.Value)
		if path != goGitRoot {
			continue
		}
		local = "git"
		if imp.Name != nil {
			local = imp.Name.Name
		}
		if local == "." {
			hits = append(hits, name+":"+strconv.Itoa(fset.Position(imp.Pos()).Line)+" dot-import")
			local = ""
		}
	}
	if local == "" || local == "_" {
		return hits, nil
	}
	ast.Inspect(f, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if id, ok := sel.X.(*ast.Ident); ok && id.Name == local && rawOpeners[sel.Sel.Name] {
			hits = append(hits, name+":"+strconv.Itoa(fset.Position(sel.Pos()).Line)+" "+sel.Sel.Name)
		}
		return true
	})
	return hits, nil
}

// TestNoRawGoGitOpener: no non-test file anywhere in the qualgen module opens a
// repository with go-git's raw opener; gitcore.OpenGoGit is the one door.
//
// regression: #6606
func TestNoRawGoGitOpener(t *testing.T) {
	var hits []string
	scanned := 0
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != "." && (d.Name() == "testdata" || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		scanned++
		h, err := rawOpenerUses(path, src)
		if err != nil {
			return err
		}
		hits = append(hits, h...)
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if scanned < 20 {
		t.Fatalf("scanned only %d non-test files; the walk is not reading the module", scanned)
	}
	sort.Strings(hits)
	if len(hits) > 0 {
		t.Fatalf("raw go-git opener in non-test code (use gitcore.OpenGoGit): %v", hits)
	}
}

// TestNoRawGoGitOpenerPlant is the guard's positive control: planted openers
// under the default, an aliased and a dot import are flagged; gitcore's opener
// and a non-opener go-git call are not.
func TestNoRawGoGitOpenerPlant(t *testing.T) {
	cases := []struct {
		src  string
		want int
	}{
		{"package p\nimport git \"github.com/go-git/go-git/v5\"\nfunc f() { git.PlainOpenWithOptions(\"x\", nil) }\n", 1},
		{"package p\nimport \"github.com/go-git/go-git/v5\"\nvar o = git.PlainOpen\n", 1},
		{"package p\nimport gg \"github.com/go-git/go-git/v5\"\nfunc f() { gg.Open(nil, nil) }\n", 1},
		{"package p\nimport . \"github.com/go-git/go-git/v5\"\nfunc f() { PlainOpen(\"x\") }\n", 1},
		{"package p\nimport git \"github.com/go-git/go-git/v5\"\nvar o git.LogOptions\n", 0},
		{"package p\nimport \"github.com/medici-finance/assay/gitcore\"\nfunc f() { gitcore.OpenGoGit(\"x\") }\n", 0},
	}
	for i, c := range cases {
		hits, err := rawOpenerUses("plant.go", []byte(c.src))
		if err != nil {
			t.Fatalf("case %d: parse: %v", i, err)
		}
		if len(hits) != c.want {
			t.Errorf("case %d: got %d hits %v, want %d", i, len(hits), hits, c.want)
		}
	}
}
