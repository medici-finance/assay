package runnercmds

import (
	"strings"
	"testing"
)

// regression: example-stream/50 — a program named only by a row the runner never
// executes must not become a pinned instrument of the run.
func TestCommandsRunnerRowsOnly(t *testing.T) {
	brief := "# B\n\n## Verify\n\n" +
		"| # | Class | Command |\n|---|-------|---------|\n" +
		"| 1 | check | `statusgen --lint` |\n" +
		"| 2 | gate:model | `modelprog --help` |\n" +
		"| 3a | check | `suffixprog --help` |\n" +
		"| 4 | check:ci | `go test ./x \\| statusgen -v` |\n" +
		"| 5 | check | `# commentprog --help` |\n\n## Evidence\n\nPending.\n"
	got := Commands(brief)
	want := []string{"statusgen --lint", "go test ./x | statusgen -v"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("Commands = %q, want %q", got, want)
	}
}

func TestCommandsDefaultClassAndNoTable(t *testing.T) {
	legacy := "## Verify\n\n| # | Command |\n|---|---|\n| 1 | `true` |\n"
	if got := Commands(legacy); len(got) != 1 || got[0] != "true" {
		t.Fatalf("legacy table = %q", got)
	}
	if got := Commands("# no verify section\n"); got != nil {
		t.Fatalf("no table = %q", got)
	}
}
