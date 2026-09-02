package main

// loopidentity_test.go — an outward verb refuses when the session presents no loop
// identity, because a STOP.<loop> flag a human is holding then has nothing to match
// against and the halt silently fails.
//
// FAIL-FIRST: observed RED before the check was added — with $DESK_LOOP unset the verb
// filed the issue and returned 0.

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestDeskLoopUnsetRefusesTheOutwardVerb(t *testing.T) {
	calls := withEnv(t)
	t.Setenv("DESK_LOOP", "")

	body := bodyFileWith(t, "something worth filing")
	rc, out := runCapture([]string{"new", "-R", allowedRepo, "--title", "a new finding", "--body-file", body})
	if rc != deskkit.ExitRefused {
		t.Fatalf("rc = %d, want %d (refused); out=%s", rc, deskkit.ExitRefused, out)
	}
	if !strings.Contains(out, "DESK_LOOP") {
		t.Errorf("the refusal does not name the variable to export: %s", out)
	}
	assertNoIssueCreate(t, *calls)
}
