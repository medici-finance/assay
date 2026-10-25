package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
	"github.com/medici-finance/assay/tools/desk/internal/verifycmd"
)

// TestRunnerGetsWholePipeline — #1953's class, its second instance: the deterministic
// runner's Verify parser split rows on every `|`, escaped or not, so a row whose Command is a
// pipeline reached the executor cut at the first `\|`. The runner must hand the executor the
// un-escaped pipeline.
//
// regression: #1953
func TestRunnerGetsWholePipeline(t *testing.T) {
	root := t.TempDir()
	brief := "# B\n\n## Verify\n\n| # | Class | Command | Expect |\n|---|---|---|---|\n" +
		"| 1 | check | `grep -c a f.txt \\| wc -l \\|\\| true` | exit 0 |\n"
	if err := os.WriteFile(filepath.Join(root, "b.md"), []byte(brief), 0o644); err != nil {
		t.Fatal(err)
	}
	var ran []string
	exec := func(_, command string) (int, string) { ran = append(ran, command); return 0, "" }
	runBriefRows(root, loopengine.Item{BriefPath: "b.md"}, exec)
	if want := "grep -c a f.txt | wc -l || true"; len(ran) != 1 || ran[0] != want {
		t.Fatalf("executor ran %q, want exactly [%q]", ran, want)
	}
}

// TestEvidenceKeepsPipedCommand — the Evidence table renderEvidence writes re-parses to the
// command that ran: its `|` is escaped, so the Command cell is not split at the pipe.
func TestEvidenceKeepsPipedCommand(t *testing.T) {
	cmd := "grep -c a f.txt | wc -l || true"
	md := renderEvidence(loopengine.Result{Rows: []loopengine.EvidenceRow{{Command: cmd, Output: "1"}}}, "2026-01-01")
	lines := strings.Split(md, "\n")
	cells := verifycmd.SplitRow(lines[2])
	if len(cells) != 6 {
		t.Fatalf("Evidence row %q split into %d cells, want 6", lines[2], len(cells))
	}
	if got := verifycmd.UnescapePipes(verifycmd.Lift(strings.TrimSpace(cells[1]))); got != cmd {
		t.Fatalf("Evidence Command re-parses to %q, want %q", got, cmd)
	}
}

// TestStreamTableHonoursEscapedPipe — a stream README row whose title cell carries an
// escaped pipe keeps its later cells in their columns.
func TestStreamTableHonoursEscapedPipe(t *testing.T) {
	readme := "---\nstream: s\n---\n\n| # | Brief | Status | Verified | Reviewed |\n|---|---|---|---|---|\n" +
		"| 01 | a \\| b | implemented | ✓ | — |\n"
	_, rows := parseStreamTable(readme, "s")
	if len(rows) != 1 || rows[0].Num != "01" || rows[0].Status != "implemented" {
		t.Fatalf("rows = %+v, want one row 01 with status implemented", rows)
	}
}
