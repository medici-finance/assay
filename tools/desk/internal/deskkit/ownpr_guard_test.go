package deskkit

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

// ownPRCompareAllow is the committed allow-list for the #1901 defect class: the ONLY
// production file that may compare a pull request's head BRANCH NAME (a `.HeadRef` /
// `.HeadRefName` field) against anything other than the empty string. Everything else that
// needs "is this worktree the PR's own checkout" calls CheckOwnPR, which also admits a
// worktree sitting exactly on the PR's head commit under another branch name.
var ownPRCompareAllow = map[string]bool{
	"internal/deskkit/ownpr.go": true,
}

// headRefCompares returns "file:line" for every `==`/`!=` whose one side is a `.HeadRef` or
// `.HeadRefName` selector and whose other side is not the empty-string literal.
func headRefCompares(t *testing.T, name string, src []byte) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	isHeadRef := func(e ast.Expr) bool {
		sel, ok := e.(*ast.SelectorExpr)
		return ok && (sel.Sel.Name == "HeadRef" || sel.Sel.Name == "HeadRefName")
	}
	isEmpty := func(e ast.Expr) bool {
		lit, ok := e.(*ast.BasicLit)
		return ok && lit.Kind == token.STRING && (lit.Value == `""` || lit.Value == "``")
	}
	var hits []string
	ast.Inspect(f, func(n ast.Node) bool {
		be, ok := n.(*ast.BinaryExpr)
		if !ok || (be.Op != token.EQL && be.Op != token.NEQ) {
			return true
		}
		for _, pair := range [][2]ast.Expr{{be.X, be.Y}, {be.Y, be.X}} {
			if isHeadRef(pair[0]) && !isEmpty(pair[1]) {
				hits = append(hits, fset.Position(be.Pos()).String())
				break
			}
		}
		return true
	})
	return hits
}

// TestOwnPRClassGuard is the CLASS guard for #1901: no production file under tools/desk
// may re-implement the own-PR check as a bare branch-name compare. The reported instance
// (deskreply's `view.HeadRefName != facts.branch`) is gone; this fails on the NEXT caller
// that writes one, naming it, so it goes through CheckOwnPR instead.
func TestOwnPRClassGuard(t *testing.T) {
	// Positive control: the planted fixture MUST be flagged, or the matcher is broken.
	planted, err := os.ReadFile(filepath.Join("testdata", "ownprguard", "planted.go.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if hits := headRefCompares(t, "planted.go", planted); len(hits) != 1 {
		t.Fatalf("positive control: the planted branch-name compare was flagged %d times, want 1: %v", len(hits), hits)
	}

	root := filepath.Join("..", "..") // tools/desk
	var offenders []string
	werr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" || strings.HasPrefix(d.Name(), ".") && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if ownPRCompareAllow[rel] {
			return nil
		}
		src, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		offenders = append(offenders, headRefCompares(t, rel, src)...)
		return nil
	})
	if werr != nil {
		t.Fatalf("walk %s: %v", root, werr)
	}
	if len(offenders) > 0 {
		t.Fatalf("a PR head-branch NAME compare outside deskkit.CheckOwnPR (#1901 defect class) — "+
			"call CheckOwnPR, which also admits a worktree at the PR's head commit:\n  %s",
			strings.Join(offenders, "\n  "))
	}
}
