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
// to: the honoured marker where there is one; statusgen's first-span lift where
// there is none (#3147).
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
			want := firstSpan(v.Cell)
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
// (#2684 review S1, F2; #3147), which statusgen's lint and witness are held to as
// well.
type commentCell struct {
	Name      string `json:"name"`
	Cell      string `json:"cell"`
	FirstSpan string `json:"firstSpan"`
	Marked    bool   `json:"marked"`
	Lift      string `json:"lift"`
	Statusgen string `json:"statusgen"`
	Desk      string `json:"desk"`
	Passes    bool   `json:"passes"`
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
	if len(doc.Cells) < 30 {
		t.Fatalf("positive control: only %d cell vectors loaded", len(doc.Cells))
	}
	return doc.Cells
}

// cellVerdict maps Refused's reason to the shared table's verdict words.
func cellVerdict(why string) string {
	switch why {
	case "":
		return "runs"
	case MarkedCommentWhy, UnmarkedCommentWhy:
		return "comment-refused"
	}
	return "refused"
}

// TestCommentCellSharedVectors — Refused's cell-to-verdict mapping and Lift
// match the shared table on every cell, and the table holds the executors to
// statusgen verdict for verdict: lift parity, so a cell statusgen runs is the
// same command here and a cell statusgen refuses is never run here (#3147).
// regression: #2684, #3147
func TestCommentCellSharedVectors(t *testing.T) {
	for _, v := range loadCommentCells(t) {
		if got := firstSpan(v.Cell); got != v.FirstSpan {
			t.Errorf("%s: firstSpan(%q) = %q, want statusgen's lift %q", v.Name, v.Cell, got, v.FirstSpan)
		}
		if _, ok := Marked(v.Cell); ok != v.Marked {
			t.Errorf("%s: Marked ok = %v, want %v", v.Name, ok, v.Marked)
		}
		if got := Lift(v.Cell); got != v.Lift {
			t.Errorf("%s: Lift(%q) = %q, want statusgen's lift %q", v.Name, v.Cell, got, v.Lift)
		}
		why := Refused(v.Cell)
		if got := cellVerdict(why); got != v.Desk {
			t.Errorf("%s: Refused(%q) = %q, want %s", v.Name, v.Cell, why, v.Desk)
		}
		if v.Statusgen != v.Desk {
			t.Errorf("%s: the table lets the executors (%s) differ from statusgen (%s)", v.Name, v.Desk, v.Statusgen)
		}
		if wantMarked := v.Marked && v.Desk == "comment-refused"; wantMarked != (why == MarkedCommentWhy) {
			t.Errorf("%s: a marked comment needs the replace-it reason and only it, got %q", v.Name, why)
		}
	}
}

// TestCommandRefusedBareLifts — a row built without a cell is judged on its
// lifted command: blank, comment and no-op are refused, a real command is not.
// regression: #3147
func TestCommandRefusedBareLifts(t *testing.T) {
	for cmd, want := range map[string]string{
		"":                "refused",
		"   ":             "refused",
		"## Coverage":     "comment-refused",
		":":               "refused",
		": confirm it":    "refused",
		"REM look":        "refused",
		":: look":         "refused",
		"true":            "runs",
		": ; test -d .":   "runs",
		"test -d . # why": "runs",
	} {
		if got := cellVerdict(CommandRefused(cmd)); got != want {
			t.Errorf("CommandRefused(%q) = %s, want %s", cmd, got, want)
		}
	}
}

// TestLiftUnmarkedIsFirstSpan — a cell with no marker lifts its first code span,
// byte for byte what statusgen's verifyCommand lifts, never the whole cell
// (#3147: the whole-cell lift ran "`false`: confirm" as a passing `:`).
func TestLiftUnmarkedIsFirstSpan(t *testing.T) {
	for cell, want := range map[string]string{
		"`go test ./...`":           "go test ./...",
		"  `make test`  ":           "make test",
		"go test ./...":             "go test ./...",
		"run `x` then `y`":          "x",
		"`false`: confirm":          "false",
		"`a` <!-- `cmd: true` -->`": "a",
	} {
		if got := Lift(cell); got != want {
			t.Errorf("Lift(%q) = %q, want the first span %q", cell, got, want)
		}
	}
}

// commandColumnFuncs parses every non-test .go file under root (skipping
// testdata trees) and returns, for each function that reads a Verify table's
// Command column by header name (an index expression keyed "command"), the
// verifycmd functions that same function calls.
func commandColumnFuncs(t *testing.T, root string) map[string]map[string]bool {
	t.Helper()
	out := map[string]map[string]bool{}
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
			readsCommand, calls := false, map[string]bool{}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.IndexExpr:
					if lit, ok := x.Index.(*ast.BasicLit); ok && lit.Kind == token.STRING && lit.Value == `"command"` {
						readsCommand = true
					}
				case *ast.CallExpr:
					if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
						if id, ok := sel.X.(*ast.Ident); ok && id.Name == "verifycmd" {
							calls[sel.Sel.Name] = true
						}
					}
				}
				return true
			})
			if readsCommand {
				out[filepath.ToSlash(rel)+":"+fn.Name.Name] = calls
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
// here, naming the site. The same function must split the row with
// verifycmd.SplitRow and un-escape the command with verifycmd.UnescapePipes, so
// a `\|` in the cell is neither a column break nor passed to the shell (#1953).
// The positive control proves the walker still finds the two executors it
// exists to hold, so a broken matcher cannot report clean.
//
// regression: #1953
func TestCommandColumnLiftsViaLift(t *testing.T) {
	found := commandColumnFuncs(t, filepath.Join("..", ".."))
	var sites []string
	for site, calls := range found {
		sites = append(sites, site)
		if !calls["Lift"] {
			t.Errorf("%s reads a Verify table's Command column without verifycmd.Lift — lift the command through verifycmd.Lift so the `cmd:` marker is honoured (spec §4.4)", site)
		}
		// regression: #2684, #3147 — the class "an executor judges or runs a cell
		// differently from statusgen" (review S1): the raw cell must be judged
		// by verifycmd.Refused, never only the lift by CommentOnly.
		if !calls["Refused"] {
			t.Errorf("%s reads a Verify table's Command column without verifycmd.Refused — judge the RAW cell with verifycmd.Refused so a cell statusgen refuses is never executed (#2684, #3147)", site)
		}
		if !calls["SplitRow"] || !calls["UnescapePipes"] {
			t.Errorf("%s reads a Verify table's Command column without verifycmd.SplitRow + verifycmd.UnescapePipes — an escaped `\\|` would cut or reach the command (#1953)", site)
		}
	}
	sort.Strings(sites)
	// The integer execution and exact scheduling entrypoints now share this
	// command-column reader. Keep the positive control on the actual leaf; the
	// repo-wide Lift and raw Refused assertions above remain unchanged.
	for _, want := range []string{"cmd/verifyloop/verifyrows.go:parseVerifyRowsMode", "cmd/deskrebaseline/brief.go:parseVerifyRows"} {
		if _, ok := found[want]; !ok {
			t.Errorf("positive control: the walker no longer finds %s (found %v) — the guard is blind", want, sites)
		}
	}
}
