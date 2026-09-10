package main

import (
	"strings"
	"testing"
)

// The pre-existing exemption: a human:<name> stamp whose Reviewed cell is
// byte-identical to the SAME brief's row at the PR merge-base was authored and
// corroborated on its own PR. A board migration that re-emits whole tables (so the
// stamp lands on an "added" diff line even though nothing about it changed) must
// NOT re-gate it against THIS PR's reviews. A stamp that is NEW on the branch, or
// whose cell text was edited (date/name), stays fully gated.
//
// The four rows below are the exact cases the fix enumerates. Each builds a diff
// whose stamp lands on an added line, plus the base row set the merge-base would
// yield, then asserts the verdict AND whether the run would fail (only
// verdictMissing sets the exit-1 flag in runCorroborate).
func TestPreExistingExemption(t *testing.T) {
	// A stream-board status row carrying a stamp in its Reviewed (last) cell.
	// briefKey is the brief-NN number the row is matched on across a re-render.
	row := func(num, reviewed string) string {
		return "| " + num + " | [Some brief](brief-" + num + "-slug.md) | 0 | S | done | 2026-09-01 opus-4.8[1m]-verifier | " + reviewed + " |"
	}
	board := "docs/streams/windows-port/README.md"

	// diffAdding builds a unified diff that ADDS the given board line — the shape a
	// table re-render produces (the whole row is emitted as an added line).
	diffAdding := func(line string) string {
		return "diff --git a/" + board + " b/" + board + "\n" +
			"--- a/" + board + "\n" +
			"+++ b/" + board + "\n" +
			"@@ -40,7 +40,7 @@\n" +
			"+" + line + "\n"
	}

	cases := []struct {
		name        string
		reviewedNew string            // the Reviewed cell as it appears in THIS PR's diff
		briefNum    string            // the brief number the added row carries
		base        map[string]string // merge-base rows by brief key (nil => brief absent at base)
		wantVerdict verdict
		wantFail    bool // would this stamp fail the run (exit 1)?
	}{
		{
			// (1) Re-render only: the Reviewed cell is byte-identical to the base
			// row for the same brief. PRE-EXISTING, run does not fail.
			name:        "byte-identical re-render is pre-existing",
			reviewedNew: "2026-09-08 human:ian",
			briefNum:    "04",
			base:        map[string]string{"04": row("04", "2026-09-08 human:ian")},
			wantVerdict: verdictPreExisting,
			wantFail:    false,
		},
		{
			// (2) A NEW stamp: the brief did not carry this row at the base at all,
			// and there is no corroborating review/comment. MISSING, run fails.
			name:        "new stamp with no base row is gated",
			reviewedNew: "2026-09-10 human:ian",
			briefNum:    "05",
			base:        map[string]string{}, // brief 05 absent at the merge-base
			wantVerdict: verdictMissing,
			wantFail:    true,
		},
		{
			// (3) An EXISTING stamp whose cell was edited (the date moved) vs base.
			// The re-render is not byte-identical, so the stamp is re-gated. MISSING.
			name:        "edited cell vs base is gated",
			reviewedNew: "2026-09-10 human:ian",
			briefNum:    "04",
			base:        map[string]string{"04": row("04", "2026-09-08 human:ian")},
			wantVerdict: verdictMissing,
			wantFail:    true,
		},
		{
			// (4) The human:reviewer placeholder class, unchanged from base. The
			// byte-identical rule covers it generically — no special-casing — so a
			// migration no longer blocks on it. PRE-EXISTING, run does not fail.
			name:        "unchanged human:reviewer placeholder is pre-existing",
			reviewedNew: "2026-09-08 human:reviewer",
			briefNum:    "04",
			base:        map[string]string{"04": row("04", "2026-09-08 human:reviewer")},
			wantVerdict: verdictPreExisting,
			wantFail:    false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			diff := diffAdding(row(tc.briefNum, tc.reviewedNew))
			stamps := stampsInDiff("", diff)
			if len(stamps) != 1 {
				t.Fatalf("got %d stamps, want 1: %+v", len(stamps), stamps)
			}
			// Every case above carries a recognizable brief row, so provenance is recorded.
			if len(stamps[0].Rows) != 1 {
				t.Fatalf("stamp recorded %d rows, want 1 (a brief status row): %+v", len(stamps[0].Rows), stamps[0])
			}
			if stamps[0].Rows[0].BriefKey != tc.briefNum {
				t.Errorf("row brief key = %q, want %q", stamps[0].Rows[0].BriefKey, tc.briefNum)
			}

			// Mark pre-existing against the merge-base rows, exactly as runCorroborate
			// does. A nil base header map exercises the positional-index fallback these
			// same-shaped cases have always relied on.
			markPreExisting(stamps, map[string]map[string]string{board: tc.base}, nil)

			// No PR reviews/comments — corroboration can come ONLY from the
			// pre-existing exemption, isolating the behaviour under test.
			results := corroborateStamps(stamps, &ghPRData{}, "medici-finance/assay", 1, nil)
			if len(results) != 1 {
				t.Fatalf("got %d results, want 1", len(results))
			}
			if results[0].Verdict != tc.wantVerdict {
				t.Errorf("verdict = %v, want %v", results[0].Verdict, tc.wantVerdict)
			}
			if got := stampResultsFail(results); got != tc.wantFail {
				t.Errorf("run-fails = %v, want %v (only MISSING-CORROBORATION fails the run)", got, tc.wantFail)
			}
		})
	}
}

// TestPreExistingReshapedBaseResolvesByHeader is the regression for the brief-v2
// migration column-shift bug (#769, follow-on to #770): the migration RE-SHAPES the
// Briefs table (the Gate column is dropped header-and-cells, columns re-ordered after
// Reviewed), so the Reviewed cell's positional INDEX differs between the base table
// and the branch table. #770's cell-level compare read the base cell at the BRANCH's
// index — the wrong column on a re-shaped base — so a sign-off whose cell text is
// byte-identical read MISSING again.
//
// This case models exactly that: the branch table has the post-migration shape (7
// columns, Reviewed last at index 6), while the base table carries an EXTRA "Gate"
// column before Reviewed (8 columns, Reviewed at index 7). The Reviewed cell text is
// identical on both sides. Resolving the base cell by the branch column's HEADER NAME
// ("Reviewed") finds index 7 on the base and the sign-off reads PRE-EXISTING; the old
// positional lookup would read the base "Gate" cell at index 6 and report MISSING.
func TestPreExistingReshapedBaseResolvesByHeader(t *testing.T) {
	board := "docs/streams/windows-port/README.md"

	// Branch (this PR's diff): post-migration shape — no Gate column, Reviewed last.
	branchHeader := "| # | Brief | Wave | Effort | Status | Verified | Reviewed |"
	branchDelim := "|---|-------|------|--------|--------|----------|----------|"
	branchRow := "| 04 | [B](brief-04-s.md) | 0 | S | done | 2026-09-01 opus-4.8[1m]-verifier | 2026-09-08 human:ian |"

	// A full table re-render emits the header, its delimiter, and the row as ADDED
	// lines — the shape stampsInDiff reads the branch column's header name from.
	diff := "diff --git a/" + board + " b/" + board + "\n" +
		"--- a/" + board + "\n" +
		"+++ b/" + board + "\n" +
		"@@ -40,12 +40,11 @@\n" +
		"+" + branchHeader + "\n" +
		"+" + branchDelim + "\n" +
		"+" + branchRow + "\n"

	stamps := stampsInDiff("", diff)
	if len(stamps) != 1 {
		t.Fatalf("got %d stamps, want 1: %+v", len(stamps), stamps)
	}
	if len(stamps[0].Rows) != 1 {
		t.Fatalf("stamp recorded %d rows, want 1: %+v", len(stamps[0].Rows), stamps[0])
	}
	// The branch column name must have been captured off the branch header — it is the
	// key the base lookup resolves by. Reviewed is the last (index 6) branch cell.
	if got := stamps[0].Rows[0].Header; got != "Reviewed" {
		t.Fatalf("branch column header = %q, want %q", got, "Reviewed")
	}
	if got := stamps[0].Rows[0].CellIndex; got != 6 {
		t.Fatalf("branch CellIndex = %d, want 6 (Reviewed is the last branch column)", got)
	}

	// Base (merge-base): PRE-migration shape — an EXTRA Gate column before Reviewed, so
	// the Reviewed cell sits at index 7, one past the branch's index 6. Its text is
	// byte-identical to the branch's Reviewed cell.
	baseHeaderLine := "| # | Brief | Wave | Effort | Status | Verified | Gate | Reviewed |"
	baseRow := "| 04 | [B](brief-04-s.md) | 0 | S | done | 2026-09-01 opus-4.8[1m]-verifier | human | 2026-09-08 human:ian |"
	baseRows := map[string]string{"04": baseRow}
	baseHeader := briefHeaderIndex(baseHeaderLine)
	if baseHeader["reviewed"] != 7 {
		t.Fatalf("base header index for Reviewed = %d, want 7", baseHeader["reviewed"])
	}

	markPreExisting(stamps,
		map[string]map[string]string{board: baseRows},
		map[string]map[string]int{board: baseHeader})

	results := corroborateStamps(stamps, &ghPRData{}, "medici-finance/assay", 1, nil)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Verdict != verdictPreExisting {
		t.Errorf("verdict = %v, want PRE-EXISTING — the base cell must be resolved by header NAME, not the branch's shifted positional index", results[0].Verdict)
	}
	if stampResultsFail(results) {
		t.Errorf("run-fails = true, want false — a byte-identical re-render across a re-shaped table must not re-gate the historical sign-off")
	}
}

// TestPreExistingBranchColumnAbsentFromBaseFailsClosed is the regression for the
// header-resolution fix's fail-closed direction (#769, follow-on to #785): when the
// stamp's BRANCH column name is ABSENT from the base table's header entirely, the base
// cannot carry that column at all, so a byte-identical re-render can never be proven —
// the exemption must fail CLOSED (the stamp stays gated), NOT silently fall back to the
// branch's positional index and read whatever base cell happens to sit there.
//
// The scenario models a migration that ADDS a sign-off column ("Approved") the base
// never had, while both tables keep a "Reviewed" column (so both headers are
// recognized). The human:<name> stamp lands in the branch-only "Approved" column. The
// base row is deliberately shaped so its cell AT THE BRANCH'S POSITIONAL INDEX is
// byte-identical to the branch stamp cell: a naive index-fallback would therefore read
// that base cell and WRONGLY report PRE-EXISTING. Resolving by header NAME finds no
// "Approved" column on the base and fails closed — the stamp reads MISSING and stays
// gated.
func TestPreExistingBranchColumnAbsentFromBaseFailsClosed(t *testing.T) {
	board := "docs/streams/windows-port/README.md"

	// Branch (this PR's diff): the migration added an "Approved" sign-off column after
	// "Reviewed". The human stamp lands in "Approved" (index 7); "Reviewed" (index 6)
	// holds only a placeholder dash. "Reviewed" is present so the branch header is
	// recognized and each stamp carries its own column NAME.
	branchHeader := "| # | Brief | Wave | Effort | Status | Verified | Reviewed | Approved |"
	branchDelim := "|---|-------|------|--------|--------|----------|----------|----------|"
	branchRow := "| 04 | [B](brief-04-s.md) | 0 | S | done | 2026-09-01 opus-4.8[1m]-verifier | — | 2026-09-08 human:ian |"

	// A full table re-render emits the header, its delimiter, and the row as ADDED
	// lines — the shape stampsInDiff reads the branch column's header name from.
	diff := "diff --git a/" + board + " b/" + board + "\n" +
		"--- a/" + board + "\n" +
		"+++ b/" + board + "\n" +
		"@@ -40,12 +40,11 @@\n" +
		"+" + branchHeader + "\n" +
		"+" + branchDelim + "\n" +
		"+" + branchRow + "\n"

	stamps := stampsInDiff("", diff)
	if len(stamps) != 1 {
		t.Fatalf("got %d stamps, want 1: %+v", len(stamps), stamps)
	}
	if len(stamps[0].Rows) != 1 {
		t.Fatalf("stamp recorded %d rows, want 1: %+v", len(stamps[0].Rows), stamps[0])
	}
	// The stamp's column name was captured off the branch header — it is the key the
	// base lookup resolves by, and it is the column the base does NOT carry.
	if got := stamps[0].Rows[0].Header; got != "Approved" {
		t.Fatalf("branch column header = %q, want %q", got, "Approved")
	}
	if got := stamps[0].Rows[0].CellIndex; got != 7 {
		t.Fatalf("branch CellIndex = %d, want 7 (Approved is the last branch column)", got)
	}

	// Base (merge-base): NO "Approved" column. It keeps "Reviewed" (so the header is
	// recognized and briefHeaderIndex returns a non-empty map) and carries a "Gate"
	// column at the branch's index 7 whose row text is byte-identical to the branch's
	// Approved cell — the bait a positional index-fallback would bite on.
	baseHeaderLine := "| # | Brief | Wave | Effort | Status | Verified | Reviewed | Gate |"
	baseRow := "| 04 | [B](brief-04-s.md) | 0 | S | done | 2026-09-01 opus-4.8[1m]-verifier | — | 2026-09-08 human:ian |"
	baseRows := map[string]string{"04": baseRow}
	baseHeader := briefHeaderIndex(baseHeaderLine)
	if _, ok := baseHeader["approved"]; ok {
		t.Fatalf("base header unexpectedly carries an Approved column: %v", baseHeader)
	}
	if baseHeader["gate"] != 7 {
		t.Fatalf("base header index for Gate = %d, want 7 (the positional-fallback bait sits at the branch's index)", baseHeader["gate"])
	}

	markPreExisting(stamps,
		map[string]map[string]string{board: baseRows},
		map[string]map[string]int{board: baseHeader})

	results := corroborateStamps(stamps, &ghPRData{}, "medici-finance/assay", 1, nil)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Verdict != verdictMissing {
		t.Errorf("verdict = %v, want MISSING-CORROBORATION — the branch column is absent from the base header, so the exemption must fail closed rather than fall back to the positional index and match the base's Gate cell", results[0].Verdict)
	}
	if !stampResultsFail(results) {
		t.Errorf("run-fails = false, want true — a stamp whose column the base lacks must stay gated (fail closed)")
	}
}

// TestPreExistingRequiresAllRows pins the anti-evasion property: a (name,file)
// stamp is exempt ONLY when EVERY board row it appears on is byte-identical to the
// base. A pre-existing row and a genuinely NEW, uncorroborated row that happen to
// share the human name must NOT let the new one ride in on the old one's exemption.
func TestPreExistingRequiresAllRows(t *testing.T) {
	board := "docs/streams/windows-port/README.md"
	row := func(num, reviewed string) string {
		return "| " + num + " | [B](brief-" + num + "-s.md) | 0 | S | done | 2026-09-01 opus-4.8[1m]-verifier | " + reviewed + " |"
	}
	diff := "diff --git a/" + board + " b/" + board + "\n" +
		"--- a/" + board + "\n" +
		"+++ b/" + board + "\n" +
		"@@ -40,9 +40,9 @@\n" +
		"+" + row("04", "2026-09-08 human:ian") + "\n" + // pre-existing (matches base)
		"+" + row("07", "2026-09-10 human:ian") + "\n" // NEW row, same name, no base match

	stamps := stampsInDiff("", diff)
	if len(stamps) != 1 {
		t.Fatalf("got %d stamps, want 1 deduped (name,file): %+v", len(stamps), stamps)
	}
	if len(stamps[0].Rows) != 2 {
		t.Fatalf("dedup dropped row provenance: got %d rows, want 2", len(stamps[0].Rows))
	}

	base := map[string]string{"04": row("04", "2026-09-08 human:ian")} // brief 07 absent at base
	markPreExisting(stamps, map[string]map[string]string{board: base}, nil)

	results := corroborateStamps(stamps, &ghPRData{}, "medici-finance/assay", 1, nil)
	if results[0].Verdict != verdictMissing {
		t.Errorf("verdict = %v, want MISSING-CORROBORATION — a new uncorroborated row must not hide behind a pre-existing one", results[0].Verdict)
	}
}

// TestPreExistingNonBoardOccurrenceFailsClosed is the fail-open the reviewer of PR
// #770 saw RED: a NEW, uncorroborated human:<name> stamp on a NON-board line (prose,
// an `authorized-by:` line, an Evidence-table cell, frontmatter) shares (name,file)
// with a PRE-EXISTING board row that matches base. Dedup folds them into one stamp,
// and a non-board occurrence contributes NO Row, so the all-rows check alone cannot
// see it — before the fix the stamp was reported PRE-EXISTING and the uncorroborated
// non-board stamp rode the board row's exemption. The Unresolved fail-closed gate
// closes it: any non-board occurrence forces the whole (name,file) stamp to stay
// gated. Every row below MUST report MISSING-CORROBORATION and fail the run.
func TestPreExistingNonBoardOccurrenceFailsClosed(t *testing.T) {
	board := "docs/streams/windows-port/README.md"
	row := func(num, reviewed string) string {
		return "| " + num + " | [B](brief-" + num + "-s.md) | 0 | S | done | 2026-09-01 opus-4.8[1m]-verifier | " + reviewed + " |"
	}
	// A pre-existing board row for brief 04, byte-identical to base.
	preExistingRow := row("04", "2026-09-08 human:alex")
	base := map[string]string{"04": preExistingRow}

	nonBoard := []struct {
		name string
		line string
	}{
		{"prose", "Signed off in review by human:alex."},
		{"authorized-by", "authorized-by: human:alex"},
		{"evidence-cell", "| 1 | go test ./... | PASS | human:alex (non-implementer) |"},
		{"frontmatter", `decided-by: "human:alex"`},
	}
	for _, nb := range nonBoard {
		t.Run(nb.name, func(t *testing.T) {
			diff := "diff --git a/" + board + " b/" + board + "\n" +
				"--- a/" + board + "\n" +
				"+++ b/" + board + "\n" +
				"@@ -40,9 +40,10 @@\n" +
				"+" + preExistingRow + "\n" +
				"+" + nb.line + "\n"
			stamps := stampsInDiff("", diff)
			if len(stamps) != 1 {
				t.Fatalf("got %d stamps, want 1 deduped (name,file): %+v", len(stamps), stamps)
			}
			if !stamps[0].Unresolved {
				t.Fatalf("stamp not marked Unresolved despite a non-board occurrence (%q): %+v", nb.line, stamps[0])
			}
			markPreExisting(stamps, map[string]map[string]string{board: base}, nil)
			results := corroborateStamps(stamps, &ghPRData{}, "medici-finance/assay", 1, nil)
			if results[0].Verdict != verdictMissing {
				t.Errorf("verdict = %v, want MISSING-CORROBORATION — a NEW non-board stamp must not ride a pre-existing board row's exemption", results[0].Verdict)
			}
			if !stampResultsFail(results) {
				t.Errorf("run-fails = false, want true — the fail-open let an uncorroborated non-board stamp pass")
			}
		})
	}
}

// TestPreExistingNilMergeBaseFailsClosed pins the fail-closed direction for an
// unresolvable merge-base (no git, shallow clone, unreadable base file): the base
// row map is nil, so even a board row that would otherwise look byte-identical is
// NOT exempted — the exemption may never fire on an absence the check never observed.
func TestPreExistingNilMergeBaseFailsClosed(t *testing.T) {
	board := "docs/streams/windows-port/README.md"
	row := "| 04 | [B](brief-04-s.md) | 0 | S | done | 2026-09-01 opus-4.8[1m]-verifier | 2026-09-08 human:alex |"
	diff := "diff --git a/" + board + " b/" + board + "\n" +
		"--- a/" + board + "\n" +
		"+++ b/" + board + "\n" +
		"@@ -40,7 +40,7 @@\n" +
		"+" + row + "\n"
	stamps := stampsInDiff("", diff)
	if len(stamps) != 1 {
		t.Fatalf("got %d stamps, want 1: %+v", len(stamps), stamps)
	}
	// nil base map models an unresolvable merge-base / unreadable base file.
	markPreExisting(stamps, map[string]map[string]string{board: nil}, nil)
	results := corroborateStamps(stamps, &ghPRData{}, "medici-finance/assay", 1, nil)
	if results[0].Verdict != verdictMissing {
		t.Errorf("verdict = %v, want MISSING-CORROBORATION — an unresolvable merge-base must exempt nothing (fail-closed)", results[0].Verdict)
	}
}

// TestBriefRowKey covers the row-matching key: a brief status row is matched on its
// stable brief number (from the Brief-cell link, else the leading # cell), while an
// Evidence table row and non-table prose are NOT brief rows.
func TestBriefRowKey(t *testing.T) {
	cases := []struct {
		line string
		want string
	}{
		{"| 04 | [Windows CI leg](brief-04-windows-ci-leg.md) | 2 | M | done | 2026-09-06 host | 2026-09-08 human:reviewer |", "04"},
		{"| 01 | Some brief | 0 | S | done | 2026-07-10 opus | 2026-07-10 human:alex |", "01"}, // no link, numeric # cell
		{"| 1 | go test ./... | PASS | human:alex |", ""},                                      // Evidence row (4 cells)
		{"| 2 | go run statusgen | 0 | human:alex |", ""},                                      // Evidence row (4 cells)
		{"authorized-by: human:alex", ""},                                                      // not a table row
		{"", ""},
	}
	for _, tc := range cases {
		if got := briefRowKey(tc.line); got != tc.want {
			t.Errorf("briefRowKey(%q) = %q, want %q", tc.line, got, tc.want)
		}
	}
}

// TestBriefRowsByKey pins the base-side parse: a board file indexes only its brief
// status rows, keyed by brief number.
func TestBriefRowsByKey(t *testing.T) {
	content := strings.Join([]string{
		"| # | Brief | Wave | Effort | Status | Verified | Reviewed |",
		"|---|-------|------|--------|--------|----------|----------|",
		"| 04 | [B four](brief-04-x.md) | 2 | M | done | 2026-09-06 host | 2026-09-08 human:reviewer |",
		"| 05 | [B five](brief-05-y.md) | 2 | S | verified | 2026-09-07 opus | — |",
		"",
		"## Critical path",
		"prose, not a row",
	}, "\n")
	rows := briefRowsByKey(content)
	if len(rows) != 2 {
		t.Fatalf("indexed %d rows, want 2: %v", len(rows), rows)
	}
	if !strings.Contains(rows["04"], "human:reviewer") {
		t.Errorf("row 04 = %q, want it to carry the Reviewed cell", rows["04"])
	}
	if _, ok := rows["05"]; !ok {
		t.Errorf("row 05 missing from index")
	}
}
