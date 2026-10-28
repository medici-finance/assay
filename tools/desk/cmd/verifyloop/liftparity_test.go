package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
	"github.com/medici-finance/assay/tools/desk/internal/verifycmd"
)

// TestShellExecLiftParity — every shared cell through the real parse,
// runBriefRows and shellExec, from an empty directory. A runs cell executes
// exactly statusgen's lift and keeps its own exit status; a refused cell is
// never executed and records a non-zero could-not-run. A span followed by a
// colon or a prose word (`false`: confirm) once ran whole under sh, where the
// substitution dropped the span's status and the row recorded a pass.
// regression: #3147
func TestShellExecLiftParity(t *testing.T) {
	cells, brief := commentCellBrief(t)
	root := t.TempDir()
	rel := writeCellBrief(t, root, brief)
	res := runBriefRows(root, loopengine.Item{BriefPath: rel}, shellExec)
	if len(res) != len(cells) {
		t.Fatalf("want %d row results, got %d", len(cells), len(res))
	}
	for i, c := range cells {
		r := res[i]
		refused := strings.HasPrefix(r.Output, "could-not-run: ")
		switch {
		case c.Desk == "runs" && (refused || r.Command != verifycmd.UnescapePipes(c.Lift)):
			t.Errorf("%s: row %d ran %q (refused=%v); want statusgen's lift %q executed, its escaped pipes un-escaped", c.Name, r.Row, r.Command, refused, c.Lift)
		case c.Desk == "runs" && (r.Exit == 0) != c.Passes:
			t.Errorf("%s: row %d exit=%d output=%q; want passes=%v", c.Name, r.Row, r.Exit, r.Output, c.Passes)
		case c.Desk != "runs" && (!refused || r.Exit == 0):
			t.Errorf("%s: row %d exit=%d output=%q; want a non-zero could-not-run (%s)", c.Name, r.Row, r.Exit, r.Output, c.Desk)
		}
	}
}

// TestLaneReadsWholeCommandCell — the online-lane and window derivation reads
// the whole Command cell, not the lift: a row whose prose after the command span
// names the live cluster or the shadow window is still bucketed, although the
// first-span lift (#3147) drops that prose from the command that runs.
// regression: #3147
func TestLaneReadsWholeCommandCell(t *testing.T) {
	online := fixtureBrief("", "| 1 | `make smoke` against the live cluster | exit 0 |")
	if disp, _ := classifyFixture(t, online); disp != dispAwaitingOnlineLane {
		t.Fatalf("a live-cluster row with the phrase after its span classified %v; want awaiting-online-lane", disp)
	}
	window := fixtureBrief("", "| 1 | `make report` once the shadow window has elapsed | exit 0 |")
	if disp, _ := classifyFixture(t, window); disp != dispDeferred {
		t.Fatalf("a shadow-window row with the phrase after its span classified %v; want deferred", disp)
	}
}
