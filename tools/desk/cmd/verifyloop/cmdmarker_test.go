package main

import "testing"

// TestParseRowsHonoursCmdMarker — #1808 review, SR-1808-2: the deterministic
// runner lifts a marked Command cell's command (spec §4.4), and an unmarked cell
// lifts exactly as before. Before the fix the runner received the raw marked
// cell, so `cmd:` reached the shell as a command name (exit 127).
func TestParseRowsHonoursCmdMarker(t *testing.T) {
	brief := "# Demo\n\n## Verify\n\n| # | Class | Command | Expect |\n|---|---|---|---|\n" +
		"| 1 | check | In `PublicRepoGate` change it, then `cmd: go test ./x -count=1` | exit 0 |\n" +
		"| 2 | check | `cmd: true` | exit 0 |\n" +
		"| 3 | check | `go test ./y` | exit 0 |\n" +
		"| 4 | check | `go test ./z` <!-- `cmd: true` --> | exit 1 |\n" +
		"\n## Evidence\n"
	rows := parseVerifyRows(brief)
	want := []string{"go test ./x -count=1", "true", "go test ./y", "`go test ./z` <!-- `cmd: true` -->"}
	if len(rows) != len(want) {
		t.Fatalf("want %d rows, got %+v", len(want), rows)
	}
	for i, w := range want {
		if rows[i].Command != w {
			t.Errorf("row %d: Command = %q, want %q", i+1, rows[i].Command, w)
		}
	}
}
