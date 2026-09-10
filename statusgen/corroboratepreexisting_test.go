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

			// Mark pre-existing against the merge-base rows, exactly as runCorroborate does.
			markPreExisting(stamps, map[string]map[string]string{board: tc.base})

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
	markPreExisting(stamps, map[string]map[string]string{board: base})

	results := corroborateStamps(stamps, &ghPRData{}, "medici-finance/assay", 1, nil)
	if results[0].Verdict != verdictMissing {
		t.Errorf("verdict = %v, want MISSING-CORROBORATION — a new uncorroborated row must not hide behind a pre-existing one", results[0].Verdict)
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
