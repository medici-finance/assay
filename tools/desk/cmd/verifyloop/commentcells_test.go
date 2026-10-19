package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// commentCellsPath is the shared table statusgen and verifycmd are held to; its
// "cells" section maps whole Command cells to a verdict (#2684 review S1, F2).
var commentCellsPath = filepath.Join("..", "..", "..", "..", "statusgen", "testdata", "shell-comment-vectors.json")

type commentCell struct {
	Name   string `json:"name"`
	Cell   string `json:"cell"`
	Marked bool   `json:"marked"`
	Desk   string `json:"desk"`
}

// commentCellBrief loads the shared cells and renders them as one brief's
// Verify table of check rows, row i+1 holding cell i.
func commentCellBrief(t *testing.T) ([]commentCell, string) {
	t.Helper()
	b, err := os.ReadFile(commentCellsPath)
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
	var sb strings.Builder
	sb.WriteString("## Verify\n\n| # | Class | Command | Expect |\n|---|-------|---------|--------|\n")
	for i, c := range doc.Cells {
		fmt.Fprintf(&sb, "| %d | check | %s | exit 0 |\n", i+1, c.Cell)
	}
	sb.WriteString("\n## Evidence\n")
	return doc.Cells, sb.String()
}

// writeCellBrief writes brief under root and returns its repo-relative path.
func writeCellBrief(t *testing.T, root, brief string) string {
	t.Helper()
	rel := filepath.Join("docs", "streams", "demo", "brief-01-demo.md")
	if err := os.MkdirAll(filepath.Join(root, filepath.Dir(rel)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, rel), []byte(brief), 0o644); err != nil {
		t.Fatal(err)
	}
	return rel
}

// TestRunBriefRowsCommentCells — every shared cell through the real parse and
// runBriefRows: a comment-refused cell never reaches the executor and records a
// non-zero could-not-run with the reason that fits it (a marked comment is told
// to replace the comment, never to add the marker it has); a runs cell does.
// regression: #2684
func TestRunBriefRowsCommentCells(t *testing.T) {
	cells, brief := commentCellBrief(t)
	root := t.TempDir()
	rel := writeCellBrief(t, root, brief)
	ran := map[string]bool{}
	exec := func(_ string, command string) (int, string) {
		ran[command] = true
		return 0, ""
	}
	res := runBriefRows(root, loopengine.Item{BriefPath: rel}, exec)
	if len(res) != len(cells) {
		t.Fatalf("want %d row results, got %d", len(cells), len(res))
	}
	for i, c := range cells {
		r := res[i]
		refused := r.Exit == commentOnlyExit && strings.HasPrefix(r.Output, "could-not-run: ") && !ran[r.Command]
		if want := c.Desk == "comment-refused"; refused != want || (!want && (r.Exit != 0 || !ran[r.Command])) {
			t.Errorf("%s: row %d exit=%d output=%q executed=%v; want %s", c.Name, r.Row, r.Exit, r.Output, ran[r.Command], c.Desk)
		}
		if c.Marked && refused && (!strings.Contains(r.Output, "replace it") || strings.Contains(r.Output, "mark the real command")) {
			t.Errorf("%s: a marked comment must be told to replace it, got %q", c.Name, r.Output)
		}
	}
}

// TestShellExecS1HeadingNotPass — review S1's cells through the REAL executor:
// "`## Coverage`: confirm …" lifts whole, and sh runs it as the `:` no-op once
// the comment-only substitution expands to nothing, exit 0. The runner must
// record a non-zero result instead of that pass.
// regression: #2684
func TestShellExecS1HeadingNotPass(t *testing.T) {
	root := t.TempDir()
	rel := writeCellBrief(t, root, "## Verify\n\n| # | Class | Command | Expect |\n|---|-------|---------|--------|\n"+
		"| 1 | check | `## Coverage`: confirm every angle is answered | review evidence present |\n"+
		"| 2 | check | `#coverage`: present | review evidence present |\n"+
		"| 3 | check | `test -d .` | exit 0 |\n\n## Evidence\n")
	res := runBriefRows(root, loopengine.Item{BriefPath: rel}, shellExec)
	if len(res) != 3 {
		t.Fatalf("want 3 row results, got %+v", res)
	}
	for _, r := range res[:2] {
		if r.Exit == 0 || !strings.Contains(r.Output, "shell comment") {
			t.Errorf("row %d: an inline-heading cell must not record a pass, got exit=%d output=%q", r.Row, r.Exit, r.Output)
		}
	}
	if res[2].Exit != 0 {
		t.Errorf("positive control: a real command must still pass through shellExec, got %+v", res[2])
	}
}
