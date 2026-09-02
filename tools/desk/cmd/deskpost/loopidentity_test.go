package main

// loopidentity_test.go — an outward verb refuses when the session presents no loop
// identity, because a STOP.<loop> flag a human is holding then has nothing to match
// against and the halt silently fails.
//
// FAIL-FIRST: observed RED before the check was added — the verb ran the whole comment
// path with $DESK_LOOP unset and returned 0.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestDeskLoopUnsetRefusesTheOutwardVerb(t *testing.T) {
	f, errBuf := setupFake(t)
	t.Setenv("DESK_LOOP", "")

	body := filepath.Join(t.TempDir(), "body.md")
	if err := os.WriteFile(body, []byte("a comment body\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rc := run([]string{"comment", exampleRepo, "1", "--body-file", body})
	if rc != deskkit.ExitRefused {
		t.Fatalf("rc = %d, want %d (refused)", rc, deskkit.ExitRefused)
	}
	if !strings.Contains(errBuf.String(), "DESK_LOOP") {
		t.Errorf("the refusal does not name the variable to export: %s", errBuf.String())
	}
	if n := f.postedCmt; n != 0 {
		t.Fatalf("the refusal still posted %d comment(s)", n)
	}
}
