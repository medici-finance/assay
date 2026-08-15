package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestFixturesStayLive keeps testdata/ honest. Those payloads are the arguments of the
// brief's executable Verify rows, so a fixture that silently stops parsing — or stops
// producing the exit code its row asserts — would break the DoD from a direction no unit
// test looks at. Each fixture is exercised here with the same expectation its row carries.
func TestFixturesStayLive(t *testing.T) {
	cases := []struct {
		file     string
		prs      string
		wantExit int
		why      string
	}{
		{"actions-quiet.json", "", deskkit.ExitOK, "a fresh, scoped, complete, quiet board is a positively measured clean run"},
		{"actions-blind.json", "", deskkit.ExitUnverifiable, "an EMPTY scope sweeps nothing and reports nothing — could-not-check, never an empty board"},
		{"actions-mixed.json", "prs-mixed.json", deskkit.ExitOK, "actionable rows are measured, so the idle question is answered (NOT-IDLE)"},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			args := []string{"--actions", filepath.Join("testdata", tc.file), "--now", "2026-08-13T12:01:00Z"}
			if tc.prs != "" {
				args = append(args, "--prs", filepath.Join("testdata", tc.prs))
			}
			if _, err := os.Stat(filepath.Join("testdata", tc.file)); err != nil {
				t.Fatalf("fixture missing: %v — a Verify row's argument cannot vanish", err)
			}
			got := deskkit.ExitCodeOf(cmdPlan(args, io.Discard))
			if got != tc.wantExit {
				t.Fatalf("exit = %d, want %d (%s)", got, tc.wantExit, tc.why)
			}
		})
	}
}
