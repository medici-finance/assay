package main

import "testing"

// TestParseRowsHonoursCmdMarker — #1808 review, SR-1808-2: the rebaseline probe
// runs the command a marked Command cell names (spec §4.4), and an unmarked
// cell lifts exactly as before.
func TestParseRowsHonoursCmdMarker(t *testing.T) {
	brief := "# Demo\n\n## Verify\n\n| # | Command | Expect |\n|---|---|---|\n" +
		"| 1 | In `PublicRepoGate` change it, then `cmd: go test ./x -count=1` | exit 0 |\n" +
		"| 2 | `go test ./y` | `exit 0` |\n" +
		"\n## Evidence\n"
	rows := parseVerifyRows(brief)
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %+v", rows)
	}
	if rows[0].Command != "go test ./x -count=1" {
		t.Errorf("row 1: Command = %q, want the marked command", rows[0].Command)
	}
	if rows[1].Command != "go test ./y" || rows[1].Expect != "exit 0" {
		t.Errorf("row 2: got %+v, want the legacy unwrap", rows[1])
	}
}
