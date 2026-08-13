package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// witnessGateFixture writes a one-brief stream whose Verify table has two rows
// and whose Evidence carries whatever witness rows the caller supplies.
func witnessGateFixture(t *testing.T, status string, evidence string) []*Stream {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "wg")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	brief := `---
brief: wg/01
title: A brief whose cell is derived from its witness
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [284]
schema: brief-v1
authored: 2026-08-13 by fixture
sources: ["fixture"]
---

# Brief 01

## Verify
| # | Command | Expect |
|---|---------|--------|
| 1 | ` + "`true`" + ` | exit 0 |
| 2 | ` + "`false`" + ` | exit 1 |

## Evidence
` + evidence + `

## Review
Gate: model.
`
	if err := os.WriteFile(filepath.Join(dir, "brief-01-derived.md"), []byte(brief), 0o644); err != nil {
		t.Fatal(err)
	}
	return []*Stream{{Name: "wg", Dir: dir, Root: root, Status: "active",
		Briefs: []Brief{{Num: "01", Status: status}}}}
}

const (
	witnessRowOnePass = "| 1 | `true` | pass exit=0 | sha256:6f1a0b3c9d22 | 2026-08-13 | human:alex @ b988d175ab12 |"
	witnessRowOneFail = "| 1 | `true` | fail exit=3 | sha256:6f1a0b3c9d22 | 2026-08-13 | human:alex @ b988d175ab12 |"
	witnessRowTwoPass = "| 2 | `false` | pass exit=1 | sha256:aa11bb22cc33 | 2026-08-13 | human:alex @ b988d175ab12 |"
)

func witnessTableFor(rows ...string) string {
	return witnessHeader + "\n" + strings.Join(rows, "\n")
}

// withBaseClosures substitutes closedAtBase for the duration of a test, so the
// transition scoping can be exercised without building a git repository per
// case (the same seam unrunGateChecks' tests use).
func withBaseClosures(t *testing.T, set map[string]bool, ok bool) {
	t.Helper()
	prev := closedAtBase
	closedAtBase = func(string, []*Stream) (map[string]bool, bool) { return set, ok }
	t.Cleanup(func() { closedAtBase = prev })
}

func TestWitnessGateBlocksAClosureThisBranchMadeOverAFailingWitness(t *testing.T) {
	withBaseClosures(t, map[string]bool{}, true)
	streams := witnessGateFixture(t, "verified", witnessTableFor(witnessRowOneFail, witnessRowTwoPass))

	problems, notices := witnessGateChecks("/repo", streams)
	if len(problems) != 1 {
		t.Fatalf("got %d problems, want 1: %v", len(problems), problems)
	}
	for _, want := range []string{"wg/brief-01", "cannot close as verified", "#1", "brief-rule 30"} {
		if !strings.Contains(problems[0], want) {
			t.Errorf("problem does not mention %q: %s", want, problems[0])
		}
	}
	// Row 2 passed, so it must not be named as a failure.
	if strings.Contains(problems[0], "#2") {
		t.Errorf("a passing row was reported as failing: %s", problems[0])
	}
	if len(notices) != 0 {
		t.Errorf("a blocked closure must not also notice: %v", notices)
	}
}

func TestWitnessGateGrandfathersAnInheritedClosure(t *testing.T) {
	withBaseClosures(t, map[string]bool{"wg/01": true}, true)
	streams := witnessGateFixture(t, "done", witnessTableFor(witnessRowOneFail))

	problems, notices := witnessGateChecks("/repo", streams)
	if len(problems) != 0 {
		t.Fatalf("an inherited closure must never be a PROBLEM: %v", problems)
	}
	if len(notices) != 1 || !strings.Contains(notices[0], "grandfathered") {
		t.Fatalf("want one grandfathered NOTICE, got %v", notices)
	}
	if !strings.Contains(notices[0], "brief-rule 31") {
		t.Errorf("the NOTICE must route the fix to the causing PR (rule 31): %s", notices[0])
	}
}

// The degraded path is the could-not-check state of this instrument: with no
// resolvable base there is no observable transition, so nothing may be reported
// as a closure this branch made — and the run must SAY it is degraded rather
// than render as a clean gate.
func TestWitnessGateSaysSoWhenTheBaseIsUnresolvable(t *testing.T) {
	withBaseClosures(t, nil, false)
	streams := witnessGateFixture(t, "verified", witnessTableFor(witnessRowOneFail))

	problems, notices := witnessGateChecks("/repo", streams)
	if len(problems) != 0 {
		t.Fatalf("an unresolvable base must not produce PROBLEMs: %v", problems)
	}
	joined := strings.Join(notices, "\n")
	if !strings.Contains(joined, "degraded") {
		t.Errorf("a degraded run must announce itself: %v", notices)
	}
}

func TestWitnessGateSilentOnAPassingWitness(t *testing.T) {
	withBaseClosures(t, map[string]bool{}, true)
	streams := witnessGateFixture(t, "verified", witnessTableFor(witnessRowOnePass, witnessRowTwoPass))

	problems, notices := witnessGateChecks("/repo", streams)
	if len(problems) != 0 || len(notices) != 0 {
		t.Fatalf("a corroborated cell must be quiet: problems=%v notices=%v", problems, notices)
	}
}

// THE MEASURED BLAST RADIUS, as a test. On 2026-08-13, 319 of 320 brief files
// carried no witness at all. If absence were folded into this check it would
// fire on essentially the whole corpus; witnessNotices owns that state, rolled
// up per stream. This test is what stops a later edit from quietly widening the
// check into the inherited backlog.
func TestWitnessGateIgnoresAMissingWitness(t *testing.T) {
	withBaseClosures(t, map[string]bool{}, true)
	streams := witnessGateFixture(t, "verified", "No witness table here — just prose.")

	problems, notices := witnessGateChecks("/repo", streams)
	if len(problems) != 0 || len(notices) != 0 {
		t.Fatalf("absence is witnessNotices' business: problems=%v notices=%v", problems, notices)
	}
}

func TestWitnessGateIgnoresBriefsThatMakeNoClosureClaim(t *testing.T) {
	withBaseClosures(t, map[string]bool{}, true)
	for _, status := range []string{"todo", "in-progress", "implemented"} {
		streams := witnessGateFixture(t, status, witnessTableFor(witnessRowOneFail))
		problems, notices := witnessGateChecks("/repo", streams)
		if len(problems) != 0 || len(notices) != 0 {
			t.Errorf("status %q claims nothing about the rows: problems=%v notices=%v", status, problems, notices)
		}
	}
}

// Evidence is an append-only LOG and the LAST witness for a row wins, so a
// re-run that goes green after a red one clears the cell. The inverse — a green
// run followed by a red one — is the case the whole check exists for.
func TestWitnessGateReadsTheLatestWitnessForARow(t *testing.T) {
	withBaseClosures(t, map[string]bool{}, true)

	greenLast := witnessGateFixture(t, "verified", witnessTableFor(witnessRowOneFail, witnessRowTwoPass)+"\n\n"+witnessTableFor(witnessRowOnePass))
	if problems, _ := witnessGateChecks("/repo", greenLast); len(problems) != 0 {
		t.Errorf("a later passing re-run must clear the cell: %v", problems)
	}

	redLast := witnessGateFixture(t, "verified", witnessTableFor(witnessRowOnePass, witnessRowTwoPass)+"\n\n"+witnessTableFor(witnessRowOneFail))
	if problems, _ := witnessGateChecks("/repo", redLast); len(problems) != 1 {
		t.Errorf("a later failing re-run must redden the cell, got %v", problems)
	}
}
