package deskkit

// commentlookup_guard_test.go — the cmd-side CLASS guard for "an authorization lookup reads a
// comment thread's first page as the whole thread" (desktools-v2/03, review finding
// F1-signoff-first-100).
//
// cmd/deskmerge's R-5 gate matched the sign-off comment by id on a pull-request thread read
// that stops at the first 100 comments, and read "not found" as "deleted": a valid sign-off
// past comment 100 was refused. Two layers close the class:
//
//   - TestTypedCommentsWalkEveryKind (forge_github_typedwalk_test.go): ListCommentsTyped walks
//     EVERY kind it accepts to the end of the thread or refuses — it never returns a first page.
//   - this test: no shipped FUNCTION under tools/desk both looks a comment up by id (a
//     `.DatabaseID` compared with == or !=) and reads the thread through ListComments — the
//     untyped read, which on GitHub is still the single first-100 request the golden corpus
//     pins. An id lookup must read through ListCommentsTyped. TARGET and ceiling: 0 permits.
//
// The positive control (testdata/commentlookup/planted.go.txt) holds one planted instance; a
// matcher that silently stopped matching fails there instead of reporting the tree clean.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestNoIDLookupOnFirstPage(t *testing.T) {
	found, err := scanIDLookupOnFirstPage(deskTreeRoot, ".go")
	if err != nil {
		t.Fatalf("the comment-lookup guard could not scan the desk tree: %v — could-not-check, NOT clean", err)
	}
	for _, key := range found {
		t.Errorf("%s looks a comment up by id on a ListComments read. On GitHub that read is a pull "+
			"request's FIRST 100 comments, so \"not found\" there is not absence — a sign-off past comment "+
			"100 reads as deleted (F1-signoff-first-100). Read the thread through ListCommentsTyped, which "+
			"walks every kind to the end or refuses.", key)
	}
}

func TestIDLookupGuardSeesPlant(t *testing.T) {
	found, err := scanIDLookupOnFirstPage(filepath.Join("testdata", "commentlookup"), ".go.txt")
	if err != nil {
		t.Fatalf("scan positive control: %v", err)
	}
	want := []string{"planted.go.txt::findSignOff"}
	if strings.Join(found, ",") != strings.Join(want, ",") {
		t.Fatalf("the guard flagged %v on the planted fixture, want %v — its matcher no longer sees the "+
			"defect, so a clean scan of the desk tree certifies nothing", found, want)
	}
}

// scanIDLookupOnFirstPage returns the sorted `<rel path>::<func>` keys of every function in the
// files under root (suffix ext, test files excluded) that both compares a `.DatabaseID` with ==
// or != and calls a method named ListComments.
func scanIDLookupOnFirstPage(root, ext string) ([]string, error) {
	fset := token.NewFileSet()
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if d.IsDir() {
			if path != root {
				switch d.Name() {
				case "testdata", "vendor", ".git":
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(path, ext) || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		f, perr := parser.ParseFile(fset, path, src, 0)
		if perr != nil {
			return perr
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		ast.Inspect(f, func(n ast.Node) bool {
			var body ast.Node
			name := ""
			switch v := n.(type) {
			case *ast.FuncDecl:
				if v.Body == nil {
					return true
				}
				body, name = v.Body, v.Name.Name
			default:
				return true
			}
			if idLookupOverListComments(body) {
				out = append(out, rel+"::"+name)
			}
			return false
		})
		return nil
	})
	sort.Strings(out)
	return out, err
}

func idLookupOverListComments(n ast.Node) bool {
	compares, untyped := false, false
	isDBID := func(e ast.Expr) bool {
		sel, ok := e.(*ast.SelectorExpr)
		return ok && sel.Sel.Name == "DatabaseID"
	}
	ast.Inspect(n, func(x ast.Node) bool {
		switch v := x.(type) {
		case *ast.BinaryExpr:
			if (v.Op == token.EQL || v.Op == token.NEQ) && (isDBID(v.X) || isDBID(v.Y)) {
				compares = true
			}
		case *ast.CallExpr:
			if sel, ok := v.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "ListComments" {
				untyped = true
			}
		}
		return true
	})
	return compares && untyped
}
