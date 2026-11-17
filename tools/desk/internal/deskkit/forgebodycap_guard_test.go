package deskkit

// forgebodycap_guard_test.go — the source guard for the forge body cap. A shipped file in the
// trees below that reads a response body with io.ReadAll (bare or wrapped, e.g. in an
// io.LimitReader that silently truncates) instead of ReadCappedForgeBody / ReadForgeBodyUpTo,
// or that DISCARDS the error of a body read, fails the build. The planted fixture proves the
// scanner sees both shapes, so a scanner that sees nothing cannot certify everything.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// forgeBodyGuardRoots are the trees whose forge body reads must go through the cap: the forge
// library, the credential paths, the lock-holding write verbs, and the commands that read
// the forge directly.
var forgeBodyGuardRoots = []string{
	"internal/deskkit", "internal/credential",
	"cmd/deskevidence", "cmd/deskpost", "cmd/deskrelease", "cmd/desktoken",
	"cmd/deskadvisory", "cmd/deskinbox", "cmd/deskgitlab",
}

// forgeBodyReadPermits are the readers that keep their own, narrower bound AND refuse an
// over-bound body (they read limit+1 and check). Key: "<root>/<file>:<func>".
var forgeBodyReadPermits = map[string]string{
	"internal/deskkit/compositionsource.go:httpFetchBounded": "reads limit+1 and refuses a body over its own limit",
}

// bodyReadFuncs are the calls whose error result is a body read's error.
var bodyReadFuncs = map[string]bool{
	"ReadCappedForgeBody": true, "ReadForgeBodyUpTo": true, "readCapped": true,
	"readCappedResponse": true, "readCappedN": true,
}

// regression: #6730
func TestForgeBodyReadsCapped(t *testing.T) {
	var found []string
	for _, r := range forgeBodyGuardRoots {
		hits, err := scanForgeBodyReads(filepath.Join(deskTreeRoot, r), r)
		if err != nil {
			t.Fatalf("the forge-body guard could not scan %s: %v — could-not-check, NOT clean", r, err)
		}
		found = append(found, hits...)
	}
	used := map[string]bool{}
	for _, h := range found {
		key := h[:strings.LastIndex(h, " ")]
		if _, ok := forgeBodyReadPermits[key]; ok {
			used[key] = true
			continue
		}
		t.Errorf("%s — read forge response bodies through deskkit.ReadCappedForgeBody (or ReadForgeBodyUpTo "+
			"for a narrower bound) and never discard the read error: an uncapped or truncating read buffers "+
			"a hostile body whole or hands back a prefix as the answer (#6730)", h)
	}
	for key := range forgeBodyReadPermits {
		if !used[key] {
			t.Errorf("permit %s matches no read any more — drop it", key)
		}
	}
}

// The planted fixture holds one of each shape the guard must catch, and one it must not.
//
// regression: #6730
func TestForgeBodyGuardSeesPlanted(t *testing.T) {
	hits, err := scanForgeBodyReads(filepath.Join("testdata", "forgebodycap"), "planted")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"planted/planted.go:aliasedRead uncapped-body-read",
		"planted/planted.go:discardedCapped discarded-read-error",
		"planted/planted.go:discardedRead discarded-read-error",
		"planted/planted.go:discardedRead uncapped-body-read",
		"planted/planted.go:truncatingRead uncapped-body-read",
	}
	sort.Strings(hits)
	if strings.Join(hits, "\n") != strings.Join(want, "\n") {
		t.Errorf("planted hits:\n%s\nwant:\n%s", strings.Join(hits, "\n"), strings.Join(want, "\n"))
	}
}

// scanForgeBodyReads parses every shipped .go file directly in dir (no _test.go, no testdata)
// and returns one "<label>/<file>:<func> <kind>" per offending read.
func scanForgeBodyReads(dir, label string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	fset := token.NewFileSet()
	files := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, perr := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if perr != nil {
			return nil, perr
		}
		files++
		ioName := ""
		for _, imp := range f.Imports {
			if p, _ := strconv.Unquote(imp.Path.Value); p == "io" {
				ioName = "io"
				if imp.Name != nil {
					ioName = imp.Name.Name
				}
			}
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			where := fmt.Sprintf("%s/%s:%s", label, name, fn.Name.Name)
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.CallExpr:
					if ioName != "" && isPkgCall(x, ioName, "ReadAll") && len(x.Args) == 1 && mentionsBody(x.Args[0]) {
						out = append(out, where+" uncapped-body-read")
					}
				case *ast.AssignStmt:
					if len(x.Lhs) == 2 && len(x.Rhs) == 1 && isBlank(x.Lhs[1]) {
						if c, ok := x.Rhs[0].(*ast.CallExpr); ok && isBodyReadCall(c, ioName) {
							out = append(out, where+" discarded-read-error")
						}
					}
				}
				return true
			})
		}
	}
	if files == 0 {
		return nil, fmt.Errorf("no Go source under %s — a guard that scans nothing certifies nothing", dir)
	}
	return out, nil
}

func isPkgCall(c *ast.CallExpr, pkg, fn string) bool {
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != fn {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == pkg
}

// isBodyReadCall: io.ReadAll(<anything with .Body>) or one of the capped body readers.
func isBodyReadCall(c *ast.CallExpr, ioName string) bool {
	if ioName != "" && isPkgCall(c, ioName, "ReadAll") && len(c.Args) == 1 && mentionsBody(c.Args[0]) {
		return true
	}
	switch f := c.Fun.(type) {
	case *ast.Ident:
		return bodyReadFuncs[f.Name]
	case *ast.SelectorExpr:
		return bodyReadFuncs[f.Sel.Name]
	}
	return false
}

// mentionsBody reports whether e contains a selector ending in .Body (resp.Body, r.Body, …).
func mentionsBody(e ast.Expr) bool {
	seen := false
	ast.Inspect(e, func(n ast.Node) bool {
		if s, ok := n.(*ast.SelectorExpr); ok && s.Sel.Name == "Body" {
			seen = true
		}
		return !seen
	})
	return seen
}

func isBlank(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == "_"
}
