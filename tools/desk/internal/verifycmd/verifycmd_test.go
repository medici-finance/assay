package verifycmd

import (
	"encoding/json"
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

// vectorsPath is the shared table statusgen's own marker scan is held to
// (statusgen/verifyrun_cmdmarker_test.go reads the same file). One table, two
// implementations: if they disagree about which command a cell names, one of
// these two tests fails.
var vectorsPath = filepath.Join("..", "..", "..", "..", "statusgen", "testdata", "cmd-marker-vectors.json")

type vector struct {
	Name   string  `json:"name"`
	Cell   string  `json:"cell"`
	Marked *string `json:"marked"`
}

func loadVectors(t *testing.T) []vector {
	t.Helper()
	b, err := os.ReadFile(vectorsPath)
	if err != nil {
		t.Fatalf("could-not-check: the shared vector table is unreadable: %v", err)
	}
	var doc struct {
		Vectors []vector `json:"vectors"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Vectors) < 10 {
		t.Fatalf("positive control: only %d vectors loaded", len(doc.Vectors))
	}
	return doc.Vectors
}

// TestMarkedSharedVectors — Marked and Lift satisfy the table statusgen is held
// to: the honoured marker where there is one; the legacy wrapped-cell unwrap
// where there is none.
func TestMarkedSharedVectors(t *testing.T) {
	for _, v := range loadVectors(t) {
		t.Run(v.Name, func(t *testing.T) {
			got, ok := Marked(v.Cell)
			switch {
			case v.Marked == nil && ok:
				t.Errorf("Marked(%q) = %q, want no honoured marker", v.Cell, got)
			case v.Marked != nil && (!ok || got != *v.Marked):
				t.Errorf("Marked(%q) = %q,%v, want %q", v.Cell, got, ok, *v.Marked)
			}
			want := stripInlineCode(v.Cell)
			if v.Marked != nil {
				want = *v.Marked
			}
			if l := Lift(v.Cell); l != want {
				t.Errorf("Lift(%q) = %q, want %q", v.Cell, l, want)
			}
		})
	}
}

// TestLiftUnmarkedIsLegacy — a cell with no marker lifts byte-for-byte what the
// executors lifted before the marker existed (the compatibility hinge).
func TestLiftUnmarkedIsLegacy(t *testing.T) {
	for cell, want := range map[string]string{
		"`go test ./...`":           "go test ./...",
		"  `make test`  ":           "make test",
		"go test ./...":             "go test ./...",
		"run `x` then `y`":          "run `x` then `y`",
		"`a` <!-- `cmd: true` -->`": "a` <!-- `cmd: true` -->",
	} {
		if got := Lift(cell); got != want {
			t.Errorf("Lift(%q) = %q, want the legacy lift %q", cell, got, want)
		}
	}
}

// commandColumnFuncs parses every non-test .go file under root (skipping
// testdata trees) and returns, for each function that reads a Verify table's
// Command column by header name (an index expression keyed "command"), whether
// that same function calls verifycmd.Lift.
func commandColumnFuncs(t *testing.T, root string) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			readsCommand, lifts := false, false
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.IndexExpr:
					if lit, ok := x.Index.(*ast.BasicLit); ok && lit.Kind == token.STRING && lit.Value == `"command"` {
						readsCommand = true
					}
				case *ast.CallExpr:
					if sel, ok := x.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Lift" {
						if id, ok := sel.X.(*ast.Ident); ok && id.Name == "verifycmd" {
							lifts = true
						}
					}
				}
				return true
			})
			if readsCommand {
				out[filepath.ToSlash(rel)+":"+fn.Name.Name] = lifts
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestCommandColumnLiftsViaLift — the CLASS guard for tools/desk (#1808 review,
// SR-1808-2): "an executor lifts the command out of a Verify Command cell by a
// rule other than the marker-aware one". Every function that reads a Verify
// table's Command column must lift it through verifycmd.Lift; a new parser that
// reads the column and hands the raw cell (or its own unwrap) to a shell fails
// here, naming the site. The positive control proves the walker still finds the
// two executors it exists to hold, so a broken matcher cannot report clean.
func TestCommandColumnLiftsViaLift(t *testing.T) {
	found := commandColumnFuncs(t, filepath.Join("..", ".."))
	var sites []string
	for site, lifts := range found {
		sites = append(sites, site)
		if !lifts {
			t.Errorf("%s reads a Verify table's Command column without verifycmd.Lift — lift the command through verifycmd.Lift so the `cmd:` marker is honoured (spec §4.4)", site)
		}
	}
	sort.Strings(sites)
	for _, want := range []string{"cmd/verifyloop/verifyrows.go:parseVerifyRowsIn", "cmd/deskrebaseline/brief.go:parseVerifyRows"} {
		if _, ok := found[want]; !ok {
			t.Errorf("positive control: the walker no longer finds %s (found %v) — the guard is blind", want, sites)
		}
	}
}
