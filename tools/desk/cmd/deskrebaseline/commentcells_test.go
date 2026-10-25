package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/verifycmd"
)

// commentCellsPath is the shared table statusgen and verifycmd are held to; its
// "cells" section maps whole Command cells to a verdict (#2684 review S1, F2;
// #3147).
var commentCellsPath = filepath.Join("..", "..", "..", "..", "statusgen", "testdata", "shell-comment-vectors.json")

type commentCell struct {
	Name   string `json:"name"`
	Cell   string `json:"cell"`
	Marked bool   `json:"marked"`
	Lift   string `json:"lift"`
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
	if len(doc.Cells) < 30 {
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

// TestRebaselineCommentCells — every shared cell through deskrebaseline's real
// parse and gatherRowFacts: the row's command is statusgen's lift, a cell the
// shared table refuses is never probed (so a comment's or no-op's empty exit 0
// is never read as "the row still passes"), and a runs cell is probed unless its
// pinned path is missing (that row goes to the rename hop instead).
// regression: #2684, #3147
func TestRebaselineCommentCells(t *testing.T) {
	cells, brief := commentCellBrief(t)
	rows := parseVerifyRows(brief)
	if len(rows) != len(cells) {
		t.Fatalf("want %d rows, got %d", len(cells), len(rows))
	}
	var probed []string
	orig := runCommand
	runCommand = func(_ string, command string) (bool, int, string) {
		probed = append(probed, command)
		return true, 0, ""
	}
	t.Cleanup(func() { runCommand = orig })
	root := t.TempDir()
	for i, c := range cells {
		n := len(probed)
		f := gatherRowFacts(root, rows[i], false, "")
		// The row's command is the lift with its escaped pipes un-escaped (#1953).
		if want := verifycmd.UnescapePipes(c.Lift); rows[i].Command != want {
			t.Errorf("%s: row %d lifted %q, want statusgen's lift %q", c.Name, rows[i].Num, rows[i].Command, want)
		}
		want := c.Desk == "runs" && (f.PathRef == "" || f.PathExists)
		if f.CommandProbed != want || (len(probed) > n) != want {
			t.Errorf("%s: row %d CommandProbed=%v (lift %q, path %q); want %s", c.Name, rows[i].Num, f.CommandProbed, rows[i].Command, f.PathRef, c.Desk)
		}
	}
}
