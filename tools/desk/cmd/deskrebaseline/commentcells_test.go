package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

// TestRebaselineCommentCells — every shared cell through deskrebaseline's real
// parse and gatherRowFacts: the behaviour probe runs a cell iff the shared table
// says the executors run it, so a comment-led cell's empty exit 0 is never read
// as "the row still passes".
// regression: #2684
func TestRebaselineCommentCells(t *testing.T) {
	cells, brief := commentCellBrief(t)
	rows := parseVerifyRows(brief)
	if len(rows) != len(cells) {
		t.Fatalf("want %d rows, got %d", len(cells), len(rows))
	}
	var probed []string
	orig := runCommand
	runCommand = func(_ string, command string) (bool, int) {
		probed = append(probed, command)
		return true, 0
	}
	t.Cleanup(func() { runCommand = orig })
	root := t.TempDir()
	for i, c := range cells {
		n := len(probed)
		f := gatherRowFacts(root, rows[i], false, "")
		if want := c.Desk == "runs"; f.CommandProbed != want || (len(probed) > n) != want {
			t.Errorf("%s: row %d CommandProbed=%v (lift %q); want %s", c.Name, rows[i].Num, f.CommandProbed, rows[i].Command, c.Desk)
		}
	}
}
