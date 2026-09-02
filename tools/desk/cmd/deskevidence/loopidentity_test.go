package main

// loopidentity_test.go — an outward verb refuses when the session presents no loop
// identity, because a STOP.<loop> flag a human is holding then has nothing to match
// against and the halt silently fails.
//
// FAIL-FIRST: observed RED before the check was added — with $DESK_LOOP unset the verb
// landed the Evidence commit and returned 0.

import (
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestDeskLoopUnsetRefusesTheOutwardVerb(t *testing.T) {
	f, errBuf := setupFake(t)
	t.Setenv("DESK_LOOP", "")

	evidencePath := writeRepoFile(t, "docs/brief.md", "# Brief\n\n## Evidence\n| 1 | x | y |\n")
	f.setFile(evidencePath, "old", "old-sha")

	rc := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath})
	if rc != deskkit.ExitRefused {
		t.Fatalf("rc = %d, want %d (refused); stderr=%s", rc, deskkit.ExitRefused, errBuf.String())
	}
	if f.putCalls != 0 {
		t.Fatalf("the refusal still wrote %d time(s) to the contents API", f.putCalls)
	}
}
