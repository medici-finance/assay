package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// TestRunBriefRowsDoesNotPassACommentOnlyCommand — a Verify command that is only a
// shell comment (an inline `## Coverage` heading lifted as the command, #2684)
// exits 0 with no output. The runner must record it as a non-zero, unexecuted row,
// never hand it to the shell and turn the zero exit into a PASS; a real command in
// the same brief still reaches the executor (positive control).
// regression: #2684
func TestRunBriefRowsDoesNotPassACommentOnlyCommand(t *testing.T) {
	root := t.TempDir()
	rel := filepath.Join("docs", "streams", "demo", "brief-01-demo.md")
	if err := os.MkdirAll(filepath.Join(root, filepath.Dir(rel)), 0o755); err != nil {
		t.Fatal(err)
	}
	brief := "## Verify\n\n" +
		"| # | Class | Command | Expect |\n" +
		"|---|-------|---------|--------|\n" +
		"| 1 | check | `## Coverage` | review evidence present |\n" +
		"| 2 | check | Read `## Coverage`, then `cmd: true` | exit 0 |\n" +
		"| 3 | check | `true` | exit 0 |\n\n" +
		"## Evidence\n"
	if err := os.WriteFile(filepath.Join(root, rel), []byte(brief), 0o644); err != nil {
		t.Fatal(err)
	}
	var ran []string
	exec := func(_ string, command string) (int, string) {
		ran = append(ran, command)
		return 0, ""
	}
	res := runBriefRows(root, loopengine.Item{BriefPath: rel}, exec)
	if len(res) != 3 {
		t.Fatalf("want 3 row results, got %+v", res)
	}
	if res[0].Exit == 0 || !strings.Contains(res[0].Output, "shell comment") {
		t.Errorf("row 1 (comment-only) must be a non-zero, explained result, got %+v", res[0])
	}
	if strings.Join(ran, "|") != "true|true" {
		t.Errorf("only the two real commands may reach the executor, got %q", ran)
	}
	if res[1].Exit != 0 || res[2].Exit != 0 {
		t.Errorf("the marked and plain rows must keep their executor result, got %+v %+v", res[1], res[2])
	}
}
