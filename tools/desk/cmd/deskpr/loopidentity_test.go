package main

// loopidentity_test.go — an outward verb refuses when the session presents no loop
// identity, because a STOP.<loop> flag a human is holding then has nothing to match
// against and the halt silently fails.
//
// FAIL-FIRST: observed RED before the check was added — with $DESK_LOOP unset the verb ran
// the whole create path, pushed, and returned 0.

import (
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestDeskLoopUnsetRefusesTheOutwardVerb(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	t.Setenv("DESK_LOOP", "")

	rc := run([]string{"create", "--title", "add feature", "--body-min", "does the thing\nBrief: fixture/01"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("rc = %d, want %d (refused)", rc, deskkit.ExitRefused)
	}
	if got := ghCalls(*calls); len(got) != 0 {
		t.Fatalf("the refusal still made %d forge call(s): %v", len(got), got)
	}
	if anyCall(gitCalls(*calls), "push") {
		t.Fatal("the refusal still pushed")
	}
}
