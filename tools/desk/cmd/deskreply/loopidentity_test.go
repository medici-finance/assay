package main

// loopidentity_test.go — an outward verb refuses when the session presents no loop
// identity, because a STOP.<loop> flag a human is holding then has nothing to match
// against and the halt silently fails.
//
// FAIL-FIRST: observed RED before the check was added — with $DESK_LOOP unset the verb
// posted the comment and returned 0.

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestDeskLoopUnsetRefusesTheOutwardVerb(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	t.Setenv("DESK_LOOP", "")

	body := bodyFileWith(t, "Re-reviewed the delta.")
	rc := run([]string{"example-org/tracker", "7", "--body-file", body})
	if rc != deskkit.ExitRefused {
		t.Fatalf("rc = %d, want %d (refused)", rc, deskkit.ExitRefused)
	}
	for _, c := range *calls {
		if strings.Contains(strings.Join(c, " "), "pr comment") {
			t.Fatalf("the refusal still posted: %v", c)
		}
	}
}
