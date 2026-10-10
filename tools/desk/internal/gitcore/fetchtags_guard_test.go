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

// fetchLit is one fetch-shaped go-git option literal: where it is, which option type it builds,
// and the value expression of each field it sets.
type fetchLit struct {
	pos    string
	typ    string
	fields map[string]ast.Expr
}

// fetchOptionLiterals returns every fetch-shaped go-git option literal in src.
func fetchOptionLiterals(t *testing.T, fset *token.FileSet, name string, src []byte) (lits []fetchLit, isGoGit func(ast.Expr, string) bool) {
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
	isGoGit = func(e ast.Expr, sel string) bool {
		s, ok := e.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		x, ok := s.X.(*ast.Ident)
		return ok && x.Name == alias && (sel == "" || s.Sel.Name == sel)
	}
	if alias == "" {
		return nil, isGoGit
	}
	if f, err = parser.ParseFile(fset, name, src, 0); err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok || !isGoGit(lit.Type, "") || !fetchOptionTypes[lit.Type.(*ast.SelectorExpr).Sel.Name] {
			return true
		}
		fl := fetchLit{
			pos:    fmt.Sprintf("%s:%d", name, fset.Position(lit.Pos()).Line),
			typ:    lit.Type.(*ast.SelectorExpr).Sel.Name,
			fields: map[string]ast.Expr{},
		}
		for _, el := range lit.Elts {
			if kv, ok := el.(*ast.KeyValueExpr); ok {
				if k, ok := kv.Key.(*ast.Ident); ok {
					fl.fields[k.Name] = kv.Value
				}
			}
		}
		lits = append(lits, fl)
		return true
	})
	return lits, isGoGit
}

// fetchLiteralsFollowingTags returns "<file>:<line>" for every fetch-shaped go-git option
// literal in src that does not set Tags to NoTags, and the number of such literals seen.
func fetchLiteralsFollowingTags(t *testing.T, fset *token.FileSet, name string, src []byte) (bad []string, seen int) {
	t.Helper()
	lits, isGoGit := fetchOptionLiterals(t, fset, name, src)
	for _, l := range lits {
		if v, ok := l.fields["Tags"]; !ok || !isGoGit(v, "NoTags") {
			bad = append(bad, l.pos)
		}
	}
	return bad, len(lits)
}

// fetchLiteralsDecidingUpdates returns "<file>:<line>" for every go-git FetchOptions or
// PullOptions literal in src that leaves a ref-update decision to go-git: one that does not set
// `Force: true` (go-git v5.19.2 with tags off drops a refused non-fast-forward update without an
// error) or that sets Prune at all (go-git's prune removes symbolic refs, origin/HEAD among
// them). gitcore.Fetch forces into a staging namespace and applies and prunes itself
// (fetchstage.go); a fetch that does not is the class of fetchupdate_test.go's defects.
func fetchLiteralsDecidingUpdates(t *testing.T, fset *token.FileSet, name string, src []byte) (bad []string, seen int) {
	t.Helper()
	lits, _ := fetchOptionLiterals(t, fset, name, src)
	for _, l := range lits {
		if l.typ != "FetchOptions" && l.typ != "PullOptions" {
			continue
		}
		seen++
		force, ok := l.fields["Force"].(*ast.Ident)
		_, prune := l.fields["Prune"]
		if !ok || force.Name != "true" || prune {
			bad = append(bad, l.pos)
		}
	}
	return bad, seen
}

// walkDeskGo calls check on every non-test Go file under tools/desk (testdata, vendor and
// dot-directories skipped) and returns the joined findings and the number of literals seen.
func walkDeskGo(t *testing.T, check func(*testing.T, *token.FileSet, string, []byte) ([]string, int)) (bad []string, seen int) {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
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
		b, n := check(t, fset, rel, src)
		bad, seen = append(bad, b...), seen+n
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return bad, seen
}

func TestEveryFetchSetsNoTags(t *testing.T) {
	bad, seen := walkDeskGo(t, fetchLiteralsFollowingTags)
	if seen == 0 {
		t.Fatal("found no go-git fetch option literal — the walk is not reading the tree it guards")
	}
	if len(bad) > 0 {
		t.Fatalf("go-git fetch options without `Tags: git.NoTags` (the default follows tags and "+
			"replaces a differing local tag, a write outside the stated refspec): %s", strings.Join(bad, ", "))
	}
}

// TestEveryFetchLeavesUpdatesToGitcore is the class guard for "a fetch's ref update is decided
// by go-git": every FetchOptions/PullOptions literal in non-test tools/desk code is forced and
// sets no Prune, so no fetch can silently keep a superseded ref or prune a symbolic one.
func TestEveryFetchLeavesUpdatesToGitcore(t *testing.T) {
	bad, seen := walkDeskGo(t, fetchLiteralsDecidingUpdates)
	if seen == 0 {
		t.Fatal("found no go-git FetchOptions literal — the walk is not reading the tree it guards")
	}
	if len(bad) > 0 {
		t.Fatalf("go-git fetch options that leave a ref update to go-git (not `Force: true`, or Prune set): %s — "+
			"fetch through gitcore.Fetch, which stages and applies the update itself", strings.Join(bad, ", "))
	}
}

// TestFetchUpdateGuardFlagsPlant is that guard's positive control: the planted file holds three
// literals that leave the update to go-git (no Force, Force false, forced but pruning) and one
// clean one; the matcher must name exactly the three.
func TestFetchUpdateGuardFlagsPlant(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "fetchtags", "planted_update.go.txt"))
	if err != nil {
		t.Fatal(err)
	}
	bad, seen := fetchLiteralsDecidingUpdates(t, token.NewFileSet(), "planted_update.go", src)
	want := "planted_update.go:11 planted_update.go:15 planted_update.go:19"
	if seen != 4 || strings.Join(bad, " ") != want {
		t.Fatalf("guard on the planted instances = %v (seen %d), want [%s] of 4", bad, seen, want)
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
