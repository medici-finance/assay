package main

// mergehold2254_test.go — #2254: the verifier's draft lanes (Evidence row, outcome record) on a
// forge whose default branch takes no direct write open the desk's merge-hold marker thread on
// the change they create, so deskflip's reviewer-approved condition has a hold to release. A
// hold-open failure is loud: the run fails and names the change that exists without its gate.

import (
	"errors"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestEvidenceDraftOpensHold(t *testing.T) {
	f, errBuf := setupFake(t)
	f.defaultBranch = "main"
	evidencePath := "docs/streams/x/brief.md"
	root := rootWithFile(t, evidencePath, "row\n")

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root})
	if code != deskkit.ExitOK {
		t.Fatalf("exit = %d, want 0 (stderr %q)", code, errBuf.String())
	}
	if len(f.changes) != 1 {
		t.Fatalf("expected exactly 1 draft change, got %d", len(f.changes))
	}
	if len(f.holds) != 1 || f.holds[0] != 4242 {
		t.Fatalf("merge-holds opened = %v, want exactly [4242] — the just-created Evidence change", f.holds)
	}
}

func TestEvidenceHoldFailIsLoud(t *testing.T) {
	f, errBuf := setupFake(t)
	f.defaultBranch = "main"
	f.holdErr = errors.New("503 the instance is unavailable")
	evidencePath := "docs/streams/x/brief.md"
	root := rootWithFile(t, evidencePath, "row\n")

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root})
	if code == deskkit.ExitOK {
		t.Fatal("exit = 0 — a merge-hold open failure must never read as a clean landing")
	}
	if !strings.Contains(errBuf.String(), "https://forge.example/change/4242") ||
		!strings.Contains(errBuf.String(), "merge-hold") {
		t.Fatalf("stderr does not name the change and its missing merge-hold: %q", errBuf.String())
	}
}

func TestOutcomeDraftOpensHold(t *testing.T) {
	f, errBuf := setupFake(t)
	f.defaultBranch = "main"
	line := `{"ts":"2026-09-07T02:00:00Z","brief":"example-stream/14","outcome":"verify-fail","sha":"0000002"}`
	recFile := writeRepoFile(t, "record.json", line+"\n")

	code := run([]string{"example-org/tracker", "main", "--outcome-record", recFile})
	if code != deskkit.ExitOK {
		t.Fatalf("exit = %d, want 0 (stderr %q)", code, errBuf.String())
	}
	if len(f.changes) != 1 {
		t.Fatalf("expected exactly 1 draft change, got %d", len(f.changes))
	}
	if len(f.holds) != 1 || f.holds[0] != 4242 {
		t.Fatalf("merge-holds opened = %v, want exactly [4242] — the just-created outcome-record change", f.holds)
	}
}
