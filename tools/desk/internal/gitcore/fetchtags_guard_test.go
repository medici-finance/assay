package gitcore

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// fetchtags_guard_test.go — the CLASS guard for "a fetch writes refs outside the refspec it was
// handed".
//
// The defect: go-git's fetch options default to tag-following, and that mode writes every
// advertised tag whose object is present locally — replacing a local tag of the same name that
// differs — although the caller's refspec names only branches. Every in-process fetch in
// tools/desk states its writes as its refspecs, so every go-git fetch-shaped option literal
// (FetchOptions, CloneOptions, PullOptions) in non-test code must set `Tags: git.NoTags`. This
// guard walks every non-test Go file under tools/desk and fails naming any literal that does
// not, so the next fetch added anywhere in the tree is red here. A planted instance in
// testdata/fetchtags is the positive control: if the matcher stops matching, the control fails
// rather than the guard reporting clean.

const goGitImport = "github.com/go-git/go-git/v5"

// fetchOptionTypes are the go-git option structs whose Tags field defaults to following tags.
var fetchOptionTypes = map[string]bool{"FetchOptions": true, "CloneOptions": true, "PullOptions": true}

// fetchLiteralsFollowingTags returns "<file>:<line>" for every fetch-shaped go-git option
// literal in src that does not set Tags to NoTags, and the number of such literals seen.
func fetchLiteralsFollowingTags(t *testing.T, fset *token.FileSet, name string, src []byte) (bad []string, seen int) {
	t.Helper()
	f, err := parser.ParseFile(fset, name, src, parser.ImportsOnly|parser.ParseComments)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	alias := ""
	for _, imp := range f.Imports {
		if p, _ := strconv.Unquote(imp.Path.Value); p == goGitImport {
			alias = "git"
			if imp.Name != nil {
				alias = imp.Name.Name
			}
		}
	}
	if alias == "" {
		return nil, 0
	}
	if f, err = parser.ParseFile(fset, name, src, 0); err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	isGoGit := func(e ast.Expr, sel string) bool {
		s, ok := e.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		x, ok := s.X.(*ast.Ident)
		return ok && x.Name == alias && (sel == "" || s.Sel.Name == sel)
	}
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok || !isGoGit(lit.Type, "") || !fetchOptionTypes[lit.Type.(*ast.SelectorExpr).Sel.Name] {
			return true
		}
		seen++
		noTags := false
		for _, el := range lit.Elts {
			kv, ok := el.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			if k, ok := kv.Key.(*ast.Ident); ok && k.Name == "Tags" && isGoGit(kv.Value, "NoTags") {
				noTags = true
			}
		}
		if !noTags {
			bad = append(bad, fmt.Sprintf("%s:%d", name, fset.Position(lit.Pos()).Line))
		}
		return true
	})
	return bad, seen
}

func TestEveryFetchSetsNoTags(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var bad []string
	seen := 0
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if d.IsDir() {
			if d.Name() == "testdata" || d.Name() == "vendor" || strings.HasPrefix(d.Name(), ".") && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		rel, _ := filepath.Rel(root, path)
		b, n := fetchLiteralsFollowingTags(t, fset, rel, src)
		bad, seen = append(bad, b...), seen+n
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	if seen == 0 {
		t.Fatal("found no go-git fetch option literal — the walk is not reading the tree it guards")
	}
	if len(bad) > 0 {
		t.Fatalf("go-git fetch options without `Tags: git.NoTags` (the default follows tags and "+
			"replaces a differing local tag, a write outside the stated refspec): %s", strings.Join(bad, ", "))
	}
}

// TestFetchTagsGuardFlagsPlant is the positive control: the planted file builds fetch options
// that follow tags (one with no Tags field, one with an explicit non-NoTags mode, both under a
// renamed import), and one that is clean; the matcher must name exactly the two.
func TestFetchTagsGuardFlagsPlant(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "fetchtags", "planted.go.txt"))
	if err != nil {
		t.Fatal(err)
	}
	bad, seen := fetchLiteralsFollowingTags(t, token.NewFileSet(), "planted.go", src)
	if seen != 3 || len(bad) != 2 || bad[0] != "planted.go:13" || bad[1] != "planted.go:17" {
		t.Fatalf("guard on the planted instance = %v (seen %d), want [planted.go:13 planted.go:17] of 3", bad, seen)
	}
}
