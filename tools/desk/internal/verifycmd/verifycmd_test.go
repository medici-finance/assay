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

// commentVectorsPath is the shell-comment table statusgen's shellCommentOnly is
// held to as well (#2684).
var commentVectorsPath = filepath.Join("..", "..", "..", "..", "statusgen", "testdata", "shell-comment-vectors.json")

// TestCommentOnlySharedVectors — CommentOnly agrees with the table statusgen is
// held to: a lifted command that is only a shell comment is never runnable.
// regression: #2684
func TestCommentOnlySharedVectors(t *testing.T) {
	b, err := os.ReadFile(commentVectorsPath)
	if err != nil {
		t.Fatalf("could-not-check: the shared comment vector table is unreadable: %v", err)
	}
	var doc struct {
		Vectors []struct {
			Name        string `json:"name"`
			Cmd         string `json:"cmd"`
			CommentOnly bool   `json:"commentOnly"`
		} `json:"vectors"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Vectors) < 8 {
		t.Fatalf("positive control: only %d vectors loaded", len(doc.Vectors))
	}
	for _, v := range doc.Vectors {
		if got := CommentOnly(v.Cmd); got != v.CommentOnly {
			t.Errorf("%s: CommentOnly(%q) = %v, want %v", v.Name, v.Cmd, got, v.CommentOnly)
		}
	}
	// The lifted form of the reported cell: a cell wrapped whole in one span.
	if !CommentOnly(Lift("`## Coverage`")) {
		t.Errorf("Lift of a lone heading span must be comment-only, got %q", Lift("`## Coverage`"))
	}
	if CommentOnly(Lift("Inspect `## Coverage`, then `cmd: go test ./x -count=1`")) {
		t.Error("a marked command beside a heading mention must not be comment-only")
	}
}

// commentCell is one whole Command cell from the shared table's "cells" section
// (#2684 review S1, F2), which statusgen's lint and witness are held to as well.
type commentCell struct {
	Name      string `json:"name"`
	Cell      string `json:"cell"`
	FirstSpan string `json:"firstSpan"`
	Marked    bool   `json:"marked"`
	Statusgen string `json:"statusgen"`
	Desk      string `json:"desk"`
}

// loadCommentCells reads the shared cell vectors (verifyloop's and
// deskrebaseline's tests read the same section).
func loadCommentCells(t *testing.T) []commentCell {
	t.Helper()
	b, err := os.ReadFile(commentVectorsPath)
	if err != nil {
		t.Fatalf("could-not-check: the shared comment vector table is unreadable: %v", err)
	}
	var doc struct {
		Cells []commentCell `json:"cells"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Cells) < 12 {
		t.Fatalf("positive control: only %d cell vectors loaded", len(doc.Cells))
	}
	return doc.Cells
}

// TestCommentCellSharedVectors — CommentLed's cell-to-verdict mapping matches
// the shared table, the firstSpan port matches statusgen's lift on every cell,
// and the executors are never looser than statusgen: a cell statusgen refuses is
// refused here, and a cell refused only here is one whose own executed lift is
// a comment (review S1: the predicate was shared, the string it judged was not).
// regression: #2684
func TestCommentCellSharedVectors(t *testing.T) {
	for _, v := range loadCommentCells(t) {
		if got := firstSpan(v.Cell); got != v.FirstSpan {
			t.Errorf("%s: firstSpan(%q) = %q, want statusgen's lift %q", v.Name, v.Cell, got, v.FirstSpan)
		}
		if _, ok := Marked(v.Cell); ok != v.Marked {
			t.Errorf("%s: Marked ok = %v, want %v", v.Name, ok, v.Marked)
		}
		why := CommentLed(v.Cell)
		got := "runs"
		if why != "" {
			got = "comment-refused"
		}
		if got != v.Desk {
			t.Errorf("%s: CommentLed(%q) = %q, want %s", v.Name, v.Cell, why, v.Desk)
		}
		if v.Statusgen == "comment-refused" && got != "comment-refused" {
			t.Errorf("%s: statusgen refuses the cell but the executors would run it", v.Name)
		}
		if v.Statusgen != v.Desk && !CommentOnly(Lift(v.Cell)) {
			t.Errorf("%s: the executors differ from statusgen without their own lift (%q) being a comment", v.Name, Lift(v.Cell))
		}
		if wantMarked := v.Marked && why != ""; wantMarked != (why == MarkedCommentWhy) {
			t.Errorf("%s: a marked comment needs the replace-it reason and only it, got %q", v.Name, why)
		}
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
// that same function calls verifycmd.Lift and verifycmd.CommentLed.
func commandColumnFuncs(t *testing.T, root string) map[string]columnUse {
	t.Helper()
	out := map[string]columnUse{}
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
			readsCommand, use := false, columnUse{}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.IndexExpr:
					if lit, ok := x.Index.(*ast.BasicLit); ok && lit.Kind == token.STRING && lit.Value == `"command"` {
						readsCommand = true
					}
				case *ast.CallExpr:
					if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
						if id, ok := sel.X.(*ast.Ident); ok && id.Name == "verifycmd" {
							switch sel.Sel.Name {
							case "Lift":
								use.lifts = true
							case "CommentLed":
								use.judges = true
							}
						}
					}
				}
				return true
			})
			if readsCommand {
				out[filepath.ToSlash(rel)+":"+fn.Name.Name] = use
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// columnUse is what a function reading the Command column does with it.
type columnUse struct{ lifts, judges bool }

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
	for site, use := range found {
		sites = append(sites, site)
		if !use.lifts {
			t.Errorf("%s reads a Verify table's Command column without verifycmd.Lift — lift the command through verifycmd.Lift so the `cmd:` marker is honoured (spec §4.4)", site)
		}
		// regression: #2684 — the class "an executor judges a comment-led cell on
		// a different string than statusgen" (review S1): the raw cell must be
		// judged by verifycmd.CommentLed, never only the lift by CommentOnly.
		if !use.judges {
			t.Errorf("%s reads a Verify table's Command column without verifycmd.CommentLed — judge the RAW cell with verifycmd.CommentLed so a comment-led cell is never executed (#2684)", site)
		}
	}
	sort.Strings(sites)
	for _, want := range []string{"cmd/verifyloop/verifyrows.go:parseVerifyRowsIn", "cmd/deskrebaseline/brief.go:parseVerifyRows"} {
		if _, ok := found[want]; !ok {
			t.Errorf("positive control: the walker no longer finds %s (found %v) — the guard is blind", want, sites)
		}
	}
}
