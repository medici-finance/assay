package main

import (
	"strings"
	"testing"
)

// TestApplyRebaselineScopedToVerifySection pins the F-applyrebaseline-unscoped fix (PR #1511):
// applyRebaseline must rewrite the row in the `## Verify` table, NOT an earlier pipe table that
// happens to carry the same row number and the same command substring. The brief below has a
// decoy `## Facts` table whose first data row is `| 4 | … test -f old/path.go … |` sitting
// ABOVE the real Verify row `| 4 | check | test -f old/path.go | exists |`.
//
// FAIL-FIRST: against the pre-fix applyRebaseline (which scanned the whole brief and rewrote
// the first pipe line whose first cell equalled the row number), this test REDS — the decoy
// Facts row is rewritten and the Verify row is left stale, so the "decoy untouched" assertion
// fails. With the scan confined to the Verify section it passes.
func TestApplyRebaselineScopedToVerifySection(t *testing.T) {
	const decoy = "| 4 | example | run `test -f old/path.go` here |"
	content := strings.Join([]string{
		"# Brief 05",
		"",
		"## Facts",
		"",
		"| # | note | detail |",
		"|---|------|--------|",
		decoy,
		"",
		"## Verify",
		"",
		"| # | Class | Command | Expect |",
		"|---|-------|---------|--------|",
		"| 4 | check | `test -f old/path.go` | exists |",
		"",
	}, "\n")

	row := verifyRow{Num: 4, Class: "check", Command: "test -f old/path.go", Expect: "exists"}
	newContent, _, newLine, ok := applyRebaseline(content, row, "test -f new/path.go", "exists")
	if !ok {
		t.Fatalf("applyRebaseline should locate and rewrite the Verify-section row 4")
	}
	if !strings.Contains(newLine, "new/path.go") {
		t.Fatalf("the rewritten line should be the Verify row carrying the new path; got %q", newLine)
	}
	if !strings.Contains(newContent, decoy) {
		t.Fatalf("the earlier Facts decoy row (same row number + command substring) must be UNTOUCHED — "+
			"a whole-brief scan would have rewritten it first. newContent=\n%s", newContent)
	}
	// Exactly one stale `old/path.go` should survive: the decoy's. The Verify row's was rewritten.
	if n := strings.Count(newContent, "old/path.go"); n != 1 {
		t.Fatalf("expected exactly the decoy's old/path.go to survive (1), got %d\n%s", n, newContent)
	}
}

// TestApplyRebaselineNoVerifySection — a brief with no `## Verify` section is not rewritable:
// applyRebaseline returns ok=false rather than falling back to scanning the whole document.
func TestApplyRebaselineNoVerifySection(t *testing.T) {
	content := strings.Join([]string{
		"# Brief",
		"",
		"## Facts",
		"",
		"| # | Command | Expect |",
		"|---|---------|--------|",
		"| 1 | `test -f old/path.go` | exists |",
		"",
	}, "\n")
	row := verifyRow{Num: 1, Class: "check", Command: "test -f old/path.go", Expect: "exists"}
	if _, _, _, ok := applyRebaseline(content, row, "test -f new/path.go", "exists"); ok {
		t.Fatalf("applyRebaseline must refuse (ok=false) when there is no ## Verify section")
	}
}
