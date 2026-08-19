package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBoardProvenanceLine pins #186's teeth: whenever statusgen reports a red
// board, it must name WHICH binary produced that red, so a stale-oracle red
// (an unstamped local build that predates a check the pin already fixed, OR a
// stamped build that is behind the pin) is visible as such rather than read as
// ground truth. The stamp is the tell — but it is not self-certifying: neither
// branch may assert an authority the process cannot establish, because statusgen
// cannot read the consumer's pin from inside this process. So BOTH branches send
// the reader back to the pin before the red is trusted.
func TestBoardProvenanceLine(t *testing.T) {
	saved := statusgenVersion
	t.Cleanup(func() { statusgenVersion = saved })

	t.Run("unstamped dev build flags itself as a possible stale oracle", func(t *testing.T) {
		statusgenVersion = "dev"
		got := boardProvenanceLine(3)
		for _, want := range []string{
			"3 PROBLEM(s)",
			"UNSTAMPED local build",
			`"dev"`,
			".assay-versions",
			"#186",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("dev provenance line missing %q\n  got: %s", want, got)
			}
		}
		// A dev build must NOT present itself as an authoritative stamped source.
		if strings.Contains(got, "stamped release build") {
			t.Errorf("dev provenance line claims to be the stamped release build\n  got: %s", got)
		}
	})

	t.Run("stamped build names its tag but does not over-claim authority", func(t *testing.T) {
		statusgenVersion = "statusgen/v0.5.0"
		got := boardProvenanceLine(2)
		for _, want := range []string{
			"2 PROBLEM(s)",
			"statusgen/v0.5.0",
			// The load-bearing correction (#186 review finding 2): a stamped-but-
			// behind binary is itself a stale oracle, so the stamped branch must
			// still point the reader at the pin rather than assert its red is fact.
			".assay-versions",
			"BEHIND the pin",
			"#186",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("stamped provenance line missing %q\n  got: %s", want, got)
			}
		}
		// The over-claim the review flagged: a stamped build must NOT tell the
		// reader its red is authoritative ground truth with no hedge.
		if !strings.Contains(got, "before treating this red as ground truth") {
			t.Errorf("stamped provenance line over-claims authority (no re-check hedge)\n  got: %s", got)
		}
	})
}

// TestBoardProvenanceReachesStderrOnRedBoard pins the WIRING, not just the string
// constructor (#186 review finding 3): a red board must actually emit its
// provenance line to stderr. Deleting the emit call sites in main.go — which
// TestBoardProvenanceLine alone would not catch — fails here. The phantom-brief
// row is a blocking board-source red (see offboard_test.go), the primary red
// path the stamp guards.
func TestBoardProvenanceReachesStderrOnRedBoard(t *testing.T) {
	saved := statusgenVersion
	t.Cleanup(func() { statusgenVersion = saved })
	statusgenVersion = "dev"

	root := goodrepo(t)
	readme := filepath.Join(root, "docs/streams/alpha/README.md")
	raw, err := os.ReadFile(readme)
	if err != nil {
		t.Fatal(err)
	}
	phantom := strings.TrimRight(string(raw), "\n") +
		"\n| 07 | [Phantom work](brief-07.md) | 1 | done | 2026-07-20 someone | human:alex |\n"
	if err := os.WriteFile(readme, []byte(phantom), 0o644); err != nil {
		t.Fatal(err)
	}

	code := 0
	stderr := captureStderr(t, func() { code = run(root, "write", nil, nil, "") })
	if code != 1 {
		t.Fatalf("phantom brief row exited %d, want a red board (1)", code)
	}
	if !strings.Contains(stderr, "UNSTAMPED local build") {
		t.Errorf("a red board must carry its provenance line on stderr (#186); stderr was:\n%s", stderr)
	}
}
