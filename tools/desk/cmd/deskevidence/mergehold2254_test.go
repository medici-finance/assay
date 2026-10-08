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

func TestOutcomeHoldFailIsLoud(t *testing.T) {
	f, errBuf := setupFake(t)
	f.defaultBranch = "main"
	f.holdErr = errors.New("503 the instance is unavailable")
	line := `{"ts":"2026-09-07T02:00:00Z","brief":"example-stream/14","outcome":"verify-fail","sha":"0000002"}`
	recFile := writeRepoFile(t, "record.json", line+"\n")

	code := run([]string{"example-org/tracker", "main", "--outcome-record", recFile})
	if code == deskkit.ExitOK {
		t.Fatal("exit = 0 — a merge-hold open failure must never read as a clean outcome-record landing")
	}
	if len(f.holds) != 1 || f.holds[0] != 4242 {
		t.Fatalf("merge-hold attempts = %v, want exactly [4242]", f.holds)
	}
	if !strings.Contains(errBuf.String(), "https://forge.example/change/4242") ||
		!strings.Contains(errBuf.String(), "merge-hold") {
		t.Fatalf("stderr does not name the change and its missing merge-hold: %q", errBuf.String())
	}
}

func TestDraftChangeLabelNeverHashZero(t *testing.T) {
	cases := []struct {
		pr   deskkit.PullRef
		want string
	}{
		{deskkit.PullRef{Number: 7, URL: "https://forge.example/change/7"}, "draft change #7"},
		{deskkit.PullRef{URL: "https://forge.example/change/x"}, "draft change https://forge.example/change/x"},
		{deskkit.PullRef{}, "a draft change the forge returned no number for"},
	}
	for _, c := range cases {
		pr := c.pr
		if got := draftChangeLabel(&pr); got != c.want {
			t.Errorf("draftChangeLabel(%+v) = %q, want %q", c.pr, got, c.want)
		}
	}
}
