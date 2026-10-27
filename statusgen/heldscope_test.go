package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Tests for the held scan's read scope (#1894, heldscope.go). Each case runs
// through runHeldScanCases, so the verify-gate card / autoflip read
// (verifyPassHeldContradiction) and the human done close
// (closeVerifyHeldRefusal) must agree on it. `statusgen verifyflip` is a
// fourth consumer: it calls closeVerifyHeldRefusal, so it reads the same scope.

const scopeHdr = "| # | Command | Expect | Observed | Date | Runner |\n" +
	"|---|---------|--------|----------|------|--------|\n"

// scopeRow is one results row: expect and observed are its two free cells.
func scopeRow(n, expect, observed string) string {
	return "| " + n + " | `tool check` | " + expect + " | " + observed + " | 2026-10-03 | fixture-verifier |\n"
}

// heldRun is an earlier verifier run whose row 2 was HELD in its result cell.
const heldRun = "### Run 1 — 2026-10-01\n\n" +
	"| # | Command | Exit | Result | Date | Runner |\n" +
	"|---|---------|------|--------|------|--------|\n" +
	"| 1 | `go test ./...` | 0 | ok | 2026-10-01 | fixture-verifier |\n" +
	"| 2 | `go test ./integration/...` | — | HELD — no runner online | 2026-10-01 | fixture-verifier |\n\n" +
	"**VERIFY: PASS** (model) — row 1 green.\n\n"

// passTable is a later run's results table, rows 1 and 2 green.
const passTable = scopeHdr + "| 1 | `tool check` | exit 0 | exit 0, ok | 2026-10-03 | fixture-verifier |\n" +
	"| 2 | `tool check` | exit 0 | exit 0, ok | 2026-10-03 | fixture-verifier |\n"

// passRun is a later run of its own with every row green and a closing verdict.
func passRun(verdict string) string {
	return "### Run 2 — 2026-10-03\n\n" + scopeHdr +
		scopeRow("1", "exit 0", "exit 0, ok") +
		scopeRow("2", "exit 0", "exit 0, ok") + "\n" + verdict + "\n"
}

// coverRun is a later run whose row 1 is green and whose row 2 has an empty
// Exit cell and the given Result cell, closing on a strict PASS.
func coverRun(exit, result string) string {
	return "### Run 2\n\n| # | Command | Exit | Result |\n|---|---|---|---|\n" +
		"| 1 | `go test ./...` | 0 | ok |\n" +
		"| 2 | `go test ./integration/...` | " + exit + " | " + result + " |\n\n**VERIFY: PASS**\n"
}

// coverTable is a later run whose results table has the given header and
// rows (row 1 is green) and closes on a strict PASS.
func coverTable(header string, rows ...string) string {
	sep := "|" + strings.Repeat("---|", strings.Count(header, "|")-1) + "\n"
	cells := strings.Count(header, "|") - 1
	row1 := "| 1 |" + strings.Repeat(" ok |", cells-1) + "\n"
	return "### Run 2\n\n" + header + "\n" + sep + row1 + strings.Join(rows, "\n") + "\n\n**VERIFY: PASS**\n"
}

// TestHeldScopeCoverWholeCell: a covering row is clean only when the WHOLE of
// each outcome cell reads as a clean outcome, every row of the key in the last
// entry is clean, and the outcome comes from an Exit or Result column, never
// an expectation (S1-r3, F1 round 3).
func TestHeldScopeCoverWholeCell(t *testing.T) {
	refuse := []string{
		"PASS (no re-run)", "PASS — no rerun this round", "PASS without re-running", "ok (never re-run)",
		"PASS (Run #1)", "PASS (r1)", "PASS (ditto)", "PASS (last run)", "PASS (from the first run)",
		"PASS (as last time)", "PASS (couldn’t reach runner)", "PASS — unable to reach host",
		"PASS (runner unavailable)", "PASS (unverified)", "ok — partial", "ok — inconclusive", "ok? unclear",
		"green (flaky)", "exit 0 (probe exit 6)", "exit 0; inner check exit 6",
		"ok (as in run #1)", "PASS (from 2026-10-01 run)", "ok (Run-1 output)", "ok (copied)",
		"ok — last run's result", "ok — didn’t re-run",
		"0", "0/3", "0/3 passed", "0 of 3 passed", "0 of 5 passed", "0 checks run", "0 rows executed",
		"0 — runner unavailable", "0 — cannot check",
		"PASS: 0 tests run", "PASS 2/3", "ok — 2 of 3 passed", "ok, 3 tests, 1 failed",
		"ok ~~ HELD ~~", "~~HELD ~~ ok",
	}
	var cases []heldScanCase
	for _, r := range refuse {
		cases = append(cases, heldScanCase{"result " + strconv.Quote(r), heldRun + coverRun("", r), true})
	}
	cases = append(cases,
		heldScanCase{"expected exit, empty result",
			heldRun + coverTable("| # | Command | Expected exit | Result |", "| 2 | `go test ./integration/...` | 0 |  |"), true},
		heldScanCase{"expected outcome, empty actual",
			heldRun + coverTable("| # | Command | Expected outcome | Actual |", "| 2 | `go test ./integration/...` | exit 0 |  |"), true},
		heldScanCase{"pass criteria, empty result",
			heldRun + coverTable("| # | Command | Pass criteria | Result |", "| 2 | `go test ./integration/...` | ok |  |"), true},
		heldScanCase{"note column says PASS, empty result",
			heldRun + coverTable("| # | Command | Note | Result |", "| 2 | `go test ./integration/...` | PASS |  |"), true},
		heldScanCase{"note column beside ok",
			heldRun + coverTable("| # | Exit | Result | Notes |", "| 2 | 0 | ok | not re-run |"), true},
		heldScanCase{"duplicate key, one line skipped",
			heldRun + coverTable("| # | Command | Exit | Result |",
				"| 2 | `go test ./integration/...` | — | skipped |", "| 2 | `go vet ./...` | 0 | ok |"), true},
		heldScanCase{"second table re-runs the key",
			heldRun + "### Run 2\n\n| # | Command | Exit | Result |\n|---|---|---|---|\n| 1 | `go test ./...` | 0 | ok |\n" +
				"| 2 | `go test ./integration/...` | — | not re-run |\n\n| # | Exit | Result |\n|---|---|---|\n| 2 | 0 | ok |\n\n**VERIFY: PASS**\n", true},
		heldScanCase{"table after a paragraph lists the key",
			heldRun + "### Run 2\n\n| # | Command | Exit | Result |\n|---|---|---|---|\n| 1 | `go test ./...` | 0 | ok |\n" +
				"| 2 | `go test ./integration/...` | 0 | ok |\n\nRow 2 again:\n| # | Exit | Result |\n|---|---|---|\n| 2 | — | skipped |\n\n**VERIFY: PASS**\n", true},
		heldScanCase{"unaligned row in the last entry",
			heldRun + coverTable("| # | Command | Exit | Result |",
				"| 2 | `go test ./integration/...` | 0 | ok |", "| 2 | `a | b` | — | skipped |"), true},
	)
	runHeldLayerCases(t, cases)
}

// TestHeldScopeCoverOutcome: a covering row 2 re-runs the earlier HELD row
// only when its result reads as a recognised clean outcome. A placeholder, a
// carry-forward, an unrun outcome, a note, or a cell that renders empty
// leaves the earlier hold read (S1-r2, S2-r2).
func TestHeldScopeCoverOutcome(t *testing.T) {
	refuse := []string{
		"—", "-", "n/a", "pending", "unchanged", "same as Run 1", "see Run 1",
		"not re-run", "not run", "skipped", "pass (same as Run 1)", "ok, carried",
		"exit 6", "runner offline", "FAIL", "HELD — deferred to #12",
		"<!-- ok -->", "<br>", "&nbsp;", "<span></span>", "\\ ", "\u200b",
		"ok\u200b", "<b>ok</b>", "&#111;k", "~~ok~~", "ok<br>", "ok &nbsp;",
	}
	var cases []heldScanCase
	for _, r := range refuse {
		cases = append(cases, heldScanCase{"result " + strconv.Quote(r), heldRun + coverRun("", r), true})
	}
	cases = append(cases,
		heldScanCase{"exit 0 beside a dash", heldRun + coverRun("0", "—"), true},
		heldScanCase{"dash exit, empty result", heldRun + coverRun("—", ""), true},
		heldScanCase{"dash exit, BLOCKED", heldRun + coverRun("—", "BLOCKED — runner offline"), true},
		heldScanCase{"exit 1, FAIL", heldRun + coverRun("1", "FAIL"), true},
		heldScanCase{"exit 0 beside a note", heldRun + coverRun("0", "same as Run 1"), true},
		heldScanCase{"note table covers nothing",
			heldRun + "### Run 2\n\n| # | Note |\n|---|---|\n| 1 | ok |\n| 2 | runner still offline |\n\n**VERIFY: PASS**\n", true},
		heldScanCase{"key column renamed",
			heldRun + "### Run 2\n\n| Row | Command | Exit | Result |\n|---|---|---|---|\n| 1 | `go test ./...` | 0 | ok |\n" +
				"| 2 | `go test ./integration/...` | 0 | ok |\n\n**VERIFY: PASS**\n", true},
		heldScanCase{"result ok", heldRun + coverRun("", "ok"), false},
		heldScanCase{"PASS with counts and no failures", heldRun + coverRun("0", "PASS — 12 tests, 0 failures"), false},
		heldScanCase{"ok with no errors", heldRun + coverRun("", "ok (no errors)"), false},
		heldScanCase{"passed with a duration", heldRun + coverRun("0", "passed in 1.2s"), false},
		heldScanCase{"ok with a full ratio", heldRun + coverRun("", "ok — 3/3 passed"), false},
		heldScanCase{"PASS with n of n", heldRun + coverRun("", "PASS: 3 of 3 checks passed"), false},
		heldScanCase{"check mark and PASS", heldRun + coverRun("0", "✅ PASS"), false},
		heldScanCase{"expected exit beside a result",
			heldRun + coverTable("| # | Command | Expected exit | Result |", "| 2 | `go test ./integration/...` | 0 | ok |"), false},
		heldScanCase{"struck note beside ok", heldRun + coverRun("0", "ok ~~pending~~"), false},
		heldScanCase{"exit 0 alone", heldRun + coverRun("0", ""), false},
		heldScanCase{"bold PASS with detail", heldRun + coverRun("0", "**PASS**: 12 tests"), false},
		heldScanCase{"exit 0 and passed", heldRun + coverRun("exit 0", "passed (12 tests)"), false},
	)
	runHeldLayerCases(t, cases)
}

// expectHdrRun is a later run whose results table carries one extra header
// beside Exit and Output, with row 2 holding the given Exit, Output and
// extra-column cells; it closes on a strict PASS.
func expectHdrRun(header, exit, output, extra string) string {
	return "### Run 2\n\n| # | Command | Exit | Output | " + header + " |\n|---|---|---|---|---|\n" +
		"| 1 | `go test ./...` | 0 | ok | ok |\n" +
		"| 2 | `go test ./integration/...` | " + exit + " | " + output + " | " + extra + " |\n\n**VERIFY: PASS**\n"
}

// TestHeldScopeExpectHeaderExact: a header is an expectation column only when
// it names an expectation and nothing else. A header that merely contains
// "expect" or "criteri" records the actual verdict, so it is an unrecognised
// column: a non-empty cell there keeps the earlier hold read (S1 round 4).
func TestHeldScopeExpectHeaderExact(t *testing.T) {
	var cases []heldScanCase
	for _, h := range []string{
		"Verdict vs Expect", "Matches expected?", "Meets criteria", "As expected?",
		"Result vs expected", "Criteria met?", "Expectation met", "Unexpected result",
	} {
		for _, v := range []string{"**INCONCLUSIVE**: empty body, not confirmed", "**FAIL**: Expect 200, got 503", "not re-run, carried from Run 1", "no (partial)", "no"} {
			cases = append(cases,
				heldScanCase{h + " says " + strconv.Quote(v) + ", empty result",
					heldRun + expectHdrRun(h, "0", "", v), true},
				heldScanCase{h + " says " + strconv.Quote(v) + ", ok result",
					heldRun + expectHdrRun(h, "0", "ok", v), true})
		}
	}
	// The reviewer's repro shape: the same header in both entries.
	hdr := "| # | Command | Exit | Output | Verdict vs Expect | Date | Runner |\n|---|---|---|---|---|---|---|\n"
	cases = append(cases, heldScanCase{"verdict-vs-expect repro, both entries",
		"### Run 1\n\n" + hdr + "| 1 | `a` | 0 | ok | **PASS** | 2026-10-01 | r |\n| 2 | `b` | — | — | HELD — no runner online | 2026-10-01 | r |\n\n**VERIFY: PASS** (model)\n\n" +
			"### Run 2\n\n" + hdr + "| 1 | `a` | 0 | ok | **PASS** | 2026-10-03 | r |\n| 2 | `b` | 0 |  | **INCONCLUSIVE**: empty body, not confirmed | 2026-10-03 | r |\n\n**VERIFY: PASS**\n", true})
	// A pure expectation column is still skipped.
	for _, h := range []string{"Expect", "Expected", "Expected exit", "Expected exit code", "Expected result", "Expected output", "Pass criteria", "Acceptance criteria", "Criteria", "Criterion"} {
		cases = append(cases, heldScanCase{h + " column is skipped",
			heldRun + expectHdrRun(h, "0", "ok", "exit 0"), false})
	}
	runHeldLayerCases(t, cases)
}

// TestHeldScopeCurrentHeldRefuses: a genuine held row in the current entry
// still refuses, whatever the scope rules admit around it.
func TestHeldScopeCurrentHeldRefuses(t *testing.T) {
	runHeldLayerCases(t, []heldScanCase{
		{"held result cell in the only entry",
			"**VERIFY: PASS**\n\n" + scopeHdr + scopeRow("1", "exit 0", "HELD — no runner online"), true},
		{"held result cell in the latest entry",
			heldRun + "### Run 2\n\n" + scopeHdr +
				scopeRow("1", "exit 0", "ok") + scopeRow("2", "exit 0", "could-not-check — runner offline") +
				"\n**VERIFY: PASS**\n", true},
		{"held result beside a could-not-check Expect",
			"**VERIFY: PASS**\n\n" + scopeHdr + scopeRow("4", "could-not-check, exit 6", "HELD — diff unreadable"), true},
		{"verdict-line summary beside a table",
			"**VERIFY: PASS** — row 2 HELD, no runner online.\n\n" + scopeHdr + scopeRow("1", "exit 0", "ok"), true},
		{"exit mapping inside a result cell",
			"**VERIFY: PASS**\n\n" + scopeHdr + scopeRow("2", "exit 0", "could-not-check → exit 6"), true},
	})
}

// TestHeldScopeExpectCellAdmitted: a held word in a row's Expect or Command
// cell names the behaviour under test, not the row's disposition.
func TestHeldScopeExpectCellAdmitted(t *testing.T) {
	runHeldLayerCases(t, []heldScanCase{
		{"could-not-check in Expect",
			"**VERIFY: PASS**\n\n" + scopeHdr +
				scopeRow("4", "unreadable diff → could-not-check exit 6, never clean", "exit 0 PASS"), false},
		{"HELD in Expect",
			"**VERIFY: PASS**\n\n" + scopeHdr + scopeRow("5", "row stays HELD while the lock is taken", "PASS"), false},
		{"Expected / Observed header spelling",
			"**VERIFY: PASS**\n\n| # | Command (as run) | Expected | Observed | Date / Runner |\n|---|---|---|---|---|\n" +
				"| 3 | `probe` | NOTICE could-not-check, not a hard fail | as expected, PASS | 2026-10-03 fixture-verifier |\n", false},
		{"could-not-check in Command",
			"**VERIFY: PASS**\n\n| # | Command | Exit | Result | Date | Runner |\n|---|---|---|---|---|---|\n" +
				"| 2 | `go test -run TestCouldNotCheck` | 0 | ok | 2026-10-03 | fixture-verifier |\n", false},
	})
}

// TestHeldScopeSupersededAdmitted: a row HELD in an earlier entry is admitted
// once the last entry re-runs that row in a results table of its own and
// records its own strict PASS.
func TestHeldScopeSupersededAdmitted(t *testing.T) {
	runHeldLayerCases(t, []heldScanCase{
		{"held row in an earlier entry",
			heldRun + passRun("**VERIFY: PASS**"), false},
		{"held row in the preamble table",
			strings.TrimPrefix(heldRun, "### Run 1 — 2026-10-01\n\n") + passRun("**VERIFY: PASS** — all rows green."), false},
		{"three entries, last one passes",
			heldRun + heldRun + passRun("**VERIFY: PASS**"), false},
		{"heading straight after a paragraph",
			strings.TrimSuffix(heldRun, "\n") + passRun("**VERIFY: PASS**"), false},
		{"HTML comment in the preamble",
			"<!-- one entry per verifier run -->\n\n" + heldRun + passRun("**VERIFY: PASS**"), false},
		{"multi-line comment in the preamble",
			"<!--\none entry per verifier run\n-->\n\n" + heldRun + passRun("**VERIFY: PASS**"), false},
		{"later run re-keys the row",
			heldRun + "### Run 2\n\n| # | Command | Exit | Result | Date | Runner |\n|---|---|---|---|---|---|\n" +
				"| **2** | `go test ./integration/...` | 0 | ok | 2026-10-03 | fixture-verifier |\n\n**VERIFY: PASS**\n", false},
	})
}

// proseRun is an entry of rows 1..n under the "#" key, each row's Result cell
// given by result (an empty result reads "ok"), followed by note — prose of
// the entry — and verdict. Its table carries no held token of its own.
func proseRun(title string, n int, result func(int) string, note, verdict string) string {
	s := "### " + title + "\n\n| # | Command | Exit | Result |\n|---|---|---|---|\n"
	for r := 1; r <= n; r++ {
		res := result(r)
		if res == "" {
			res = "ok"
		}
		s += "| " + strconv.Itoa(r) + " | `tool check " + strconv.Itoa(r) + "` | 0 | " + res + " |\n"
	}
	return s + "\n" + note + "\n\n" + verdict + "\n\n"
}

// rc2 is the Result of an earlier run's row 7: a failure, with no held token.
func rc2(r int) string {
	if r == 7 {
		return "exit 2"
	}
	return ""
}

// allOK re-runs every row clean.
func allOK(int) string { return "" }

// skipRow drops row n from a proseRun: the last entry did not re-run it.
func skipRow(n int, result func(int) string) func(int) string {
	return func(r int) string {
		if r == n {
			return "—"
		}
		return result(r)
	}
}

// TestHeldScopeWholeSuperseded: once the last entry, closing on its own live
// strict PASS, re-ran EVERY row key of EVERY earlier entry clean, the earlier
// entries are superseded as wholes — their prose, an entry-level VERIFY:
// BLOCKED and a held note that names no row are dropped (rule 3b). Partial
// coverage, an earlier row that cannot be keyed, no earlier keyed row at all,
// or a hold on a table line the scan cannot key keeps every entry read.
// regression: #3841
func TestHeldScopeWholeSuperseded(t *testing.T) {
	summary := "- BLOCKED. Rows 1-6 and 8 pass; row 7 could-not-check (rc=2)."
	note := "- could-not-check: the restore drill runner was offline for this run."
	blocked := "**VERIFY: BLOCKED** — could-not-check, see the note above."
	earlier := proseRun("Run 1 — 2026-10-01", 8, rc2, summary+"\n"+note, blocked)
	final := proseRun("Run 2 — 2026-10-03", 8, allOK, "All eight rows re-run on an online runner.", "**VERIFY: PASS**")
	rerun2 := proseRun("Run 2", 2, allOK, "", "**VERIFY: PASS**")
	two := func(extra string) string { return proseRun("Run 1", 2, allOK, extra, blocked) }
	eight := func(extra string) string { return proseRun("Run 1", 8, allOK, extra, blocked) }
	handTable := "### Run 2\n\n| # | Result |\n|---|---|\n" +
		"| 1 | ok |\n| 2 | ok |\n| 3 | ok |\n| 4 | ok |\n| 5 | ok |\n| 6 | ok |\n| 7 | ok |\n| 8 | ok |\n\n**VERIFY: PASS**\n"
	runHeldLayerCases(t, []heldScanCase{
		// (1) Full coverage: the earlier entries drop as wholes.
		{"BLOCKED verdict and row-less note, full re-run", earlier + final, false},
		{"summary naming rows, full re-run",
			proseRun("Run 1", 8, rc2, summary, "**VERIFY: BLOCKED**") + final, false},
		{"held cell and row-less prose", heldRun + "The integration run is HELD until a runner is online.\n\n" +
			passRun("**VERIFY: PASS**"), false},
		{"hold wrapped across lines",
			proseRun("Run 1", 8, rc2, "- BLOCKED. Rows 1-6 and 8 pass; row 7\n  could-not-check (rc=2).", blocked) + final, false},
		{"held note in a fence", heldRun + "```\nstatus: could-not-check\n```\n\n" + passRun("**VERIFY: PASS**"), false},
		{"note-only entry between runs",
			earlier + "### Run 1b\n\nStill could-not-check: runner offline.\n\n" + blocked + "\n\n" + final, false},
		{"three entries, last one passes", earlier + earlier + final, false},
		{"earlier table keyed by command",
			"### Run 1\n\n| Command | Result |\n|---|---|\n| `tool check` | exit 2 |\n\nCould-not-check on this runner.\n\n" +
				"### Run 2\n\n| Command | Result |\n|---|---|\n| `tool check` | ok |\n\n**VERIFY: PASS**\n", false},
		// (4) A hand table with no Date or Runner witness columns still covers.
		{"last entry is a hand table only", earlier + handTable, false},

		// (2) One earlier key not covered: every entry stays read.
		{"row the hold named not re-run", earlier + proseRun("Run 2", 8, skipRow(7, allOK), "", "**VERIFY: PASS**"), true},
		{"row the hold did not name not re-run", earlier + proseRun("Run 2", 8, skipRow(3, allOK), "", "**VERIFY: PASS**"), true},
		{"row re-run to a failure", earlier + proseRun("Run 2", 8, rc2, "", "**VERIFY: PASS**"), true},
		{"row missing from the last entry", earlier + proseRun("Run 2", 6, allOK, "", "**VERIFY: PASS**"), true},
		{"key of another earlier entry not re-run",
			proseRun("Run 0", 9, allOK, "", "**VERIFY: PASS**") + earlier + final, true},
		{"key column renamed in the last entry",
			earlier + strings.Replace(final, "| # |", "| Row |", 1), true},
		// (3) No live strict PASS in the last entry.
		{"loose PASS", proseRun("Run 1", 8, rc2, summary+"\n"+note, "**VERIFY: PASS** (model) — rows 1-6 and 8 green.") +
			proseRun("Run 2", 8, allOK, "", "Non-implementer run — VERIFY: PASS"), true},
		{"PASS in a fence", earlier + proseRun("Run 2", 8, allOK, "", "```\n**VERIFY: PASS**\n```"), true},
		{"BLOCKED after the PASS", earlier + proseRun("Run 2", 8, allOK, "**VERIFY: PASS**", "VERIFY: BLOCKED"), true},
		// An earlier entry the scan cannot key whole.
		{"earlier misaligned row",
			"### Run 1\n\n" + scopeHdr + scopeRow("1", "exit 0", "ok") +
				"| 2 | a | b | c | ok | 2026-10-01 | fixture-verifier |\n\nCould-not-check on row 2.\n\n" + passRun("**VERIFY: PASS**"), true},
		{"earlier empty key",
			"### Run 1\n\n" + scopeHdr + scopeRow("1", "exit 0", "ok") + scopeRow("", "exit 0", "ok") +
				"\nThe integration run is HELD.\n\n" + passRun("**VERIFY: PASS**"), true},
		{"no earlier keyed row", "### Run 1\n\nThe integration run is HELD.\n\n" + passRun("**VERIFY: PASS**"), true},
		{"hold in an earlier table of another shape",
			heldRun + "| Check | State |\n|---|---|\n| restore drill | HELD |\n\n" + passRun("**VERIFY: PASS**"), true},
		// The last entry's own holds, and a last entry it cannot align.
		{"hold in the last entry's prose", earlier + proseRun("Run 2", 8, allOK, "Row 7 was could-not-check in Run 1.", "**VERIFY: PASS**"), true},
		{"last entry misaligned row",
			earlier + strings.Replace(final, "| 8 | `tool check 8` | 0 | ok |", "| 8 | `a | b` | 0 | ok |", 1), true},

		// The ruled scope (#3841): under full coverage, held prose naming a row
		// absent from every keyed table or naming no row is dropped, and so is a
		// hold in a fence or an HTML comment. A row an earlier entry records in
		// a table the scan cannot key (no separator row, another shape, no
		// leading pipe, HTML) keeps every entry read, whether the hold sits on
		// the row or in prose naming it; so does a hold in a non-comment HTML
		// block.
		// Admitted: the ruled scope.
		{"row named only in prose", two("Row 3 HELD — the restore drill was never run.") + rerun2, false},
		{"untabled row in a note", eight("- row 9 could-not-check: runner offline.") + final, false},
		{"row-less note", two("- could-not-check: integration suite") + rerun2, false},
		{"hold in an HTML comment", two("<!-- could-not-check: runner offline -->") + rerun2, false},
		{"hold in a multi-line comment", two("<!--\nrow 3 HELD — runner offline\n-->") + rerun2, false},
		{"hold in a comment ended by --!>",
			two("<!--\nrow 3 HELD — runner offline\n--!>\n-->") + rerun2, false},
		{"hold in a fence", two("```\nrow 3 HELD — runner offline\n```") + rerun2, false},
		{"closed HTML block after a prose hold",
			two("- row 3 could-not-check.\n\n<div title=\"a\">note</div>") + rerun2, false},
		{"attribute closed in its own block",
			two("- row 3 could-not-check.\n\n<div title=\"\nx\" y>\n</div>") + rerun2, false},
		{"div block runs past a closed tag",
			two("- row 3 could-not-check.\n\n<div>a</div> <b title=\"\nx\" y>\n</b>") + rerun2, false},
		{"fenced table, row in prose",
			eight("```\n| # | Result |\n|---|---|\n| 9 | exit 2 |\n```\n\n- row 9 could-not-check.") + final, false},
		{"pipe in a code span, prose hold", two("- `a | b` could-not-check: runner offline.") + rerun2, false},
		{"escaped pipe, prose hold", two("- a \\| b could-not-check: runner offline.") + rerun2, false},

		// Refused by design (heldContainerDoubt over-refuses): a tag start in
		// a list item. Admitted before the container rule.
		{"inline tag in a list item",
			two("- row 3 could-not-check.\n- <b>x</b> done") + rerun2, true},

		// Refused: a row recorded in a table the scan cannot key.
		{"pipeless table row held",
			two("# | Command | Result\n--|--|--\n3 | drill | HELD — runner offline") + rerun2, true},
		{"pipeless table, hold in prose",
			two("Row | Check | Result\n--|--|--\n3 | drill | exit 2\n\n- row 3 could-not-check.") + rerun2, true},
		{"HTML table cell held",
			two("<table><tr><td>3</td><td>HELD — runner offline</td></tr></table>") + rerun2, true},
		{"HTML table, hold on its own line",
			two("<table>\n<tr><td>3</td>\n<td>\nHELD — runner offline\n</td></tr>\n</table>") + rerun2, true},
		{"HTML table, hold in prose",
			two("<table><tr><td>3</td><td>exit 2</td></tr></table>\n\n- row 3 could-not-check.") + rerun2, true},
		{"other-shape table, hold in prose",
			eight("| Check | Result |\n|---|---|\n| restore drill | exit 2 |\n\n- restore drill could-not-check") + final, true},
		{"no-separator table, prose hold",
			eight("| # | Result |\n| 9 | exit 2 |\n\n- row 9 could-not-check.") + final, true},
		{"blockquoted table row held",
			two("> | # | Result |\n> |---|---|\n> | 3 | HELD — runner offline |") + rerun2, true},
		{"lazy row after a keyed table",
			strings.Replace(two(""), "| 2 | `tool check 2` | 0 | ok |\n\n",
				"| 2 | `tool check 2` | 0 | ok |\n3 HELD — runner offline\n\n", 1) + rerun2, true},
		// An HTML table nested in a header or keyed-row cell renders live, so
		// the row it records keeps the prose hold naming that row read.
		{"HTML table in the header cell",
			strings.Replace(two("- row 3 could-not-check."), "| Result |",
				"| Result <table><tr><td>3</td><td>exit 2</td></tr></table> |", 1) + rerun2, true},
		{"HTML table in a keyed row's cell",
			strings.Replace(two("- row 3 could-not-check."), "| 0 | ok |",
				"| 0 | ok <table><tr><td>3</td><td>exit 2</td></tr></table> |", 1) + rerun2, true},
		// An HTML block that ends inside a tag, a quoted attribute value or a
		// comment makes the page swallow what follows, the last entry and its
		// PASS included, while the scan reads that entry live.
		{"open attribute hides the last entry",
			two("- row 3 could-not-check.\n\n<div title=\"") + rerun2, true},
		{"open tag hides the last heading",
			two("- row 3 could-not-check.\n\n<div") + rerun2, true},
		{"mid-line <!-- in an HTML block",
			two("- row 3 could-not-check.\n\n<div> <!--") + rerun2, true},
		{"open attribute in a list item",
			two("- row 3 could-not-check.\n\n- <div title=\"") + rerun2, true},
		{"open attribute in a blockquote",
			two("- row 3 could-not-check.\n\n> <div title=\"") + rerun2, true},
		{"open attribute inside <pre>",
			two("- row 3 could-not-check.\n\n<pre>\n<b title=\"\n</pre>") + rerun2, true},
		{"open attribute holding >",
			two("- row 3 could-not-check.\n\n<div title=\">") + rerun2, true},
		{"single-quoted attribute holding >",
			two("- row 3 could-not-check.\n\n<div title='>") + rerun2, true},
		{"attribute closed in a later block",
			two("- row 3 could-not-check.\n\n<div title=\"") + rerun2 + "<p>\"</p>\n", true},
		// The page ends an HTML block before the next blank line when the
		// opener meets its end condition on its own line, or when a line
		// leaves the blockquote or list item the block sits in. What follows
		// is Markdown there, so a later quote closes nothing on the page.
		{"comment closed on the opener line",
			two("- row 3 could-not-check.\n\n<!-- a --> <div title=\"\nx\" y>") + rerun2, true},
		{"<!--> closes on the opener line",
			two("- row 3 could-not-check.\n\n<!--> <b title=\"\nx\" y>") + rerun2, true},
		{"<pre> closed on the opener line",
			two("- row 3 could-not-check.\n\n<pre>a</pre> <b title=\"\nx\" y>") + rerun2, true},
		{"<? closed on the opener line",
			two("- row 3 could-not-check.\n\n<? a ?> <div title=\"\nx\" y>") + rerun2, true},
		{"declaration closed on its line",
			two("- row 3 could-not-check.\n\n<!X a> <div title=\"\nx\" y>") + rerun2, true},
		{"CDATA closed on the opener line",
			two("- row 3 could-not-check.\n\n<![CDATA[ a ]]> <div title=\"\nx\" y>") + rerun2, true},
		{"blockquote left on the next line",
			two("- row 3 could-not-check.\n\n> <div title=\"\nx\" y>") + rerun2, true},
		{"blockquote ended by a > line",
			two("- row 3 could-not-check.\n\n> <div title=\"\n>\n> x\" y>") + rerun2, true},
		{"inner blockquote left",
			two("- row 3 could-not-check.\n\n> > <div title=\"\n> x\" y>") + rerun2, true},
		{"list item left on the next line",
			two("- row 3 could-not-check.\n\n- <div title=\"\nx\" y>") + rerun2, true},
		{"ordered list item left",
			two("- row 3 could-not-check.\n\n1) <div title=\"\nx\" y>") + rerun2, true},
		{"list item by indent left",
			two("- row 3 could-not-check.\n\n  <div title=\"\nx\" y>") + rerun2, true},
		// A carriage return with no line feed after it ends a line on the
		// page but not in the scan's split, so the scan and the page can
		// disagree on fences and HTML blocks: nothing is superseded.
		{"> line ends a tag on the page",
			two("- row 3 could-not-check.\n\n<div\n> <!-- a> b") + rerun2, true},
		{"bare CR ends a fence",
			two("- row 3 could-not-check.\n\n```\nx\r```\n\n"+
				"<table><tr><td>3</td><td>exit 2</td></tr></table>\n\n```\nz\r```") + rerun2, true},
		{"bare CR opens an HTML block",
			two("- row 3 could-not-check.\n\nx\r<div title=\"") + rerun2, true},
		// Refused: a hold inside an HTML block that is not a comment.
		{"text after a comment's end",
			two("<!-- note --> row 3 HELD — runner offline") + rerun2, true},
		// Refused: an opener the renderer does not read as an open comment —
		// "<!-->" and "<!--->" close on their own line, and an opener indented
		// four spaces or by a tab is indented code — so what follows renders.
		{"empty comment, pipeless table after",
			two("<!-->\n\nRow | Result\n--|--\n3 | HELD runner offline\n\n-->") + rerun2, true},
		// The correctness review's Q4, Q10, Q11, Q12: a GFM table set off by
		// blank lines after an opener the renderer does not hold open, then a
		// later end marker; Q13 is the control, a comment closed on its line.
		{"Q4: indented opener, GFM table",
			two("    <!--\n\n| Check | State |\n|---|---|\n| drill | HELD |\n\n-->") + rerun2, true},
		{"Q10: <!--> then GFM table",
			two("<!-->\n\n| Check | State |\n|---|---|\n| drill | HELD |\n\n-->") + rerun2, true},
		{"Q11: <!---> then GFM table",
			two("<!--->\n\n| Check | State |\n|---|---|\n| drill | HELD |\n\n-->") + rerun2, true},
		{"Q12: tab opener, GFM table",
			two("\t<!--\n\n| Check | State |\n|---|---|\n| drill | HELD |\n\n-->") + rerun2, true},
		{"Q13: closed comment, GFM table",
			two("<!-- note -->\n\n| Check | State |\n|---|---|\n| drill | HELD |\n\n-->") + rerun2, true},
		// "--!>" also ends a comment on the page, while the HTML block runs on
		// to the next "-->", so what lies between renders live.
		{"--!> end, HTML table held",
			two("<!--\n--!>\n<table><tr><td>3</td><td>HELD runner offline</td></tr></table>\n-->") + rerun2, true},
		{"opener --!> end, HTML table",
			two("<!-- note --!>\n<table><tr><td>3</td><td>HELD runner offline</td></tr></table>\n-->") + rerun2, true},
		{"--!> end, table, prose hold",
			two("<!--\n--!>\n<table><tr><td>3</td><td>exit 2</td></tr></table>\n- row 3 could-not-check.\n-->") + rerun2, true},
		// A tag an earlier HTML block left open swallows a later "<!--" on
		// the page, so the table after it renders live.
		{"open <div, then <!--, HTML table",
			two("<div\n\n<!--\nx>\n<table><tr><td>3</td><td>HELD runner offline</td></tr></table>\n-->") + rerun2, true},
		{"open <div title=\", <!--, table",
			two("<div title=\"\n\n<!--\n\">\n<table><tr><td>3</td><td>HELD runner offline</td></tr></table>\n-->") + rerun2, true},
		// Refused, fail-closed: a table line inside a comment still turns
		// whole supersession off, since the page may not read the comment as
		// one (the two cases above). Round 3 had admitted this shape.
		{"table in a comment, now refused",
			two("<!--\n| Check | State |\n|---|---|\n| restore drill | HELD — runner offline |\n-->") + rerun2, true},
		{"text after a multi-line comment",
			two("<!--\nnote\n--> row 3 HELD runner offline") + rerun2, true},
		{"opener --!> end, hold after",
			two("<!-- note --!>\nrow 3 HELD runner offline\n-->") + rerun2, true},
		{"hold between --!> and -->",
			two("<!-- note --!> row 3 HELD runner offline -->") + rerun2, true},
		{"indented opener, pipeless table",
			two("    <!--\n\nRow | Result\n--|--\n3 | HELD runner offline\n\n    -->") + rerun2, true},
		{"tab-indented opener, pipeless table",
			two("\t<!--\n\nRow | Result\n--|--\n3 | HELD runner offline\n\n\t-->") + rerun2, true},
		{"empty comment, prose hold",
			two("<!-->\n\nRow | Result\n--|--\n3 | exit 2\n\n- row 3 could-not-check.\n\n-->") + rerun2, true},
		{"hold in an HTML details block",
			two("<details>\nrow 3 HELD — runner offline\n</details>") + rerun2, true},
		// Prose holds, no table: a mis-read comment would drop them, so these
		// pin the comment boundary now that a table refuses either way.
		{"hold on a details opener line",
			two("<details>row 3 HELD runner offline</details>") + rerun2, true},
		{"indented opener, prose hold",
			two("    <!--\n\nrow 3 HELD runner offline\n\n    -->") + rerun2, true},
		{"tab-indented opener, prose hold",
			two("\t<!--\n\nrow 3 HELD runner offline\n\n\t-->") + rerun2, true},
		// An indented opener left open now also refuses through the unclosed
		// HTML check, so these pin the comment indent on a one-line opener.
		{"indented one-line comment, hold",
			two("    <!-- row 3 HELD runner offline -->") + rerun2, true},
		{"tab one-line comment, hold",
			two("\t<!-- row 3 HELD runner offline -->") + rerun2, true},
		{"--!> mid-block, hold after",
			two("<!--\nnote --!>\nrow 3 HELD runner offline\n-->") + rerun2, true},
	})
}

// TestHeldScopeContainerDoubt: an HTML opener in a container (a blockquote,
// a list item, an indented line), a fence in one, or an opener after a
// paragraph or a closed construct, can end where the scan does not expect,
// so the page swallows the last entry. Whole supersession is off for each.
func TestHeldScopeContainerDoubt(t *testing.T) {
	blocked := "**VERIFY: BLOCKED** — could-not-check, see the note above."
	rerun2 := proseRun("Run 2", 2, allOK, "", "**VERIFY: PASS**")
	two := func(x string) string {
		return proseRun("Run 1", 2, allOK, "- row 3 could-not-check.\n\n"+x, blocked) + rerun2
	}
	runHeldLayerCases(t, []heldScanCase{
		// Admitted controls: a top-level HTML block or fence the page reads
		// the way the scan does.
		{"C1: attribute closed in block", two("<div title=\"\nx\" y>\n</div>"), false},
		{"C2: paragraph, div block", two("<3\n<div title=\"\nx\" y>\n</div>"), false},
		{"top-level fence holds a tag", two("```\n<div title=\"\n```"), false},
		{"top-level fence holds a table",
			two("```\n<table><tr><td>3</td><td>exit 2</td></tr></table>\n```"), false},
		{"top-level fence, indented tag", two("```\n  <div title=\"\n```"), false},
		// Refused: an opener after a paragraph, in a container, or sharing
		// its line with a closed construct.
		{"paragraph, quoted div", two("<3\n> <div title=\"\nx\" y>"), true},
		{"paragraph, comment then div", two("<3\n<!-- a --> <div title=\"\nx\" y>"), true},
		{"inline tag line, listed div", two("<b>x</b> text\n- <div title=\"\nx\" y>"), true},
		{"paragraph, closed pre then b", two("<3\n<pre>a</pre> <b title=\"\nx\" y>"), true},
		{"link line, quoted div", two("<a href=\"u\">log</a> shows\n> <div title=\"\nx\" y>"), true},
		{"paragraph, pre block open attr", two("<3\n<pre>\n<b title=\"\n</pre>\nx\" y>"), true},
		// A <pre> after a paragraph runs past blank lines to its end tag.
		{"paragraph, pre past a blank", two("<3\n<pre>\n\na <b title=\"\n</pre>\nx\" y>"), true},
		{"paragraph, pre holds a hold", two("<3\n<pre>\n\nrow 3 HELD runner offline\n</pre>"), true},
		{"div block, pre line, open attr", two("<div>\n<pre>\n<b title=\"\n\nx\" y>\n</pre>"), true},
		{"paragraph, pre holds a heading",
			proseRun("Run 1", 2, allOK, "- row 3 could-not-check.", blocked) + "<3\n<pre>\n\n" +
				proseRun("Run 2", 2, allOK, "</pre>", "**VERIFY: PASS**"), true},
		{"paragraph, pre holds last entry",
			proseRun("Run 1", 2, allOK, "- row 3 could-not-check.\n\n<3\n<pre>", blocked) + rerun2 + "</pre>\n", true},
		// Refused: a fence in a list item ends with the item on the page.
		{"list fence, open attribute", two("- a\n  ```\n<div title=\"\n```"), true},
		{"list fence, HTML table",
			two("- a\n  ```\n<table><tr><td>3</td><td>exit 2</td></tr></table>\n```"), true},
		{"ordered list tilde fence", two("1. a\n   ~~~\n<div title=\"\n~~~"), true},
		{"list fence, blank before close", two("- a\n  ```\n<div title=\"\n\n```"), true},
		{"marker fence, open attribute", two("- ```\n<div title=\"\n```"), true},
		// Refused by design (over-refusal): any fence in a container.
		{"quoted fence, no tag", two("> ```\n> x\n> ```"), true},
	})
}

// runHeldLayerCases runs cases through both consumers, then again with the
// open-element rule (heldElementCheck) off: every case these tests hold was
// decided the same way before that rule existed, so each earlier rule must
// still refuse on its own and the element rule cannot mask a regression in
// one of them.
func runHeldLayerCases(t *testing.T, cases []heldScanCase) {
	t.Helper()
	runHeldScanCasesOnce(t, cases)
	t.Run("element rule off", func(t *testing.T) {
		defer func(f func([]string, []bool, []int) bool) { heldElementCheck = f }(heldElementCheck)
		heldElementCheck = func([]string, []bool, []int) bool { return false }
		runHeldScanCasesOnce(t, cases)
	})
}

// runHeldScanCasesOnce is runHeldScanCases under the name the layered runner
// and the open-element test use.
func runHeldScanCasesOnce(t *testing.T, cases []heldScanCase) {
	t.Helper()
	runHeldScanCases(t, cases)
}

// TestHeldScopeParagraphTag: a comment or declaration that ends on its own
// line leaves the next line a paragraph, where an unfinished tag is plain
// text, so the block rule (heldHTMLUnclosed) finds no doubt there.
func TestHeldScopeParagraphTag(t *testing.T) {
	for _, x := range []string{"<!-- a -->\na <b title=\"", "<!DOCTYPE html>\na <b title=\""} {
		lines := strings.Split(x, "\n")
		if heldHTMLUnclosed(lines, make([]bool, len(lines))) {
			t.Errorf("%q: heldHTMLUnclosed = true, want false", x)
		}
	}
}

// TestHeldScopeReaderPath runs the held scan on Evidence read the way the
// production consumers read it: a whole brief file through
// parseBriefFileBytes (and briefSectionsOf, which must agree), which rewrite
// CRLF to LF before the scan sees the text. A carriage return with no line
// feed after it in the file ends a line on the page but not in the scan's
// split, so it must survive the rewrite and turn whole supersession off.
func TestHeldScopeReaderPath(t *testing.T) {
	blocked := "**VERIFY: BLOCKED** — could-not-check, see the note above."
	rerun2 := proseRun("Run 2", 2, allOK, "", "**VERIFY: PASS**")
	cr, lf, crlf := "\r", "\n", "\r\n"
	// Line endings built from CR and LF. The first two hold no stray CR;
	// each other one holds at least one CR with no LF straight after it.
	seps := []struct{ name, s string }{
		{"LF", lf}, {"CRLF", crlf},
		{"stray CR 1", cr}, {"stray CR 2", cr + cr}, {"stray CR 3", cr + crlf},
		{"stray CR 4", crlf + cr}, {"stray CR 5", cr + cr + crlf}, {"stray CR 6", lf + cr},
	}
	shapes := []struct{ name, a, b string }{
		{"attribute", "<div title=\"", "x\" y>\n</div>"},
		{"prose", "Runner notes follow.", "Nothing else."},
	}
	for _, eol := range []struct{ name, s string }{{"LF file", lf}, {"CRLF file", crlf}} {
		var cases []heldScanCase
		for _, sh := range shapes {
			for _, sp := range seps {
				name := eol.name + ", " + sh.name + ", " + sp.name
				ev := proseRun("Run 1", 2, allOK, "- row 3 could-not-check.\n\n"+sh.a+"\x00"+sh.b, blocked) + rerun2
				file := validBriefFM + "\n## Evidence\n\n" + ev
				file = strings.ReplaceAll(strings.ReplaceAll(file, lf, eol.s), "\x00", sp.s)
				bf, ok, err := parseBriefFileBytes("x.md", []byte(file))
				if err != nil || !ok || bf == nil {
					t.Fatalf("%s: parse ok=%v err=%v", name, ok, err)
				}
				if _, ev2 := briefSectionsOf(file); ev2 != bf.Evidence {
					t.Errorf("%s: briefSectionsOf Evidence differs from the brief reader's", name)
				}
				dir := t.TempDir()
				if err := os.WriteFile(filepath.Join(dir, "brief-01.md"), []byte(file), 0o644); err != nil {
					t.Fatal(err)
				}
				if _, _, aev, _ := awaitInputs(&Stream{Name: "x", Dir: dir, Root: dir}, &Brief{Num: "01"}); aev.Text != bf.Evidence {
					t.Errorf("%s: awaitInputs Evidence differs from the brief reader's", name)
				}
				// Refused exactly when the separator holds a stray CR, which
				// must still be in the Evidence the scan reads.
				stray := strings.Contains(strings.ReplaceAll(sp.s, crlf, ""), cr)
				if stray != strings.Contains(bf.Evidence, cr) {
					t.Errorf("%s: stray CR in file=%v, CR in Evidence=%v", name, stray, !stray)
				}
				cases = append(cases, heldScanCase{name, bf.Evidence, stray})
			}
		}
		runHeldLayerCases(t, cases)
	}
}

// TestHeldScopeOpenElement: an earlier entry that opens an HTML element and
// never closes it nests the last entry inside that element on the page, which
// may drop, collapse or flatten it, while the scan reads it live. Whole
// supersession is off unless every element an entry opens (void ones aside)
// is closed in that entry, by a close tag the page reads as one, and the last
// entry holds no element open across a line end. Each case runs on the raw
// text and on a whole brief file, LF and CRLF, read by parseBriefFileBytes.
func TestHeldScopeOpenElement(t *testing.T) {
	blocked := "**VERIFY: BLOCKED** — could-not-check, see the note above."
	rerun2 := proseRun("Run 2", 2, allOK, "", "**VERIFY: PASS**")
	two := func(x string) string {
		return proseRun("Run 1", 2, allOK, "- row 3 could-not-check.\n\n"+x, blocked) + rerun2
	}
	base := []heldScanCase{
		// Admitted controls: every element closed in its entry, or void.
		{"element closed on its line", two("<span>x</span>"), false},
		{"element closed later", two("<section>\n\nx\n\n</section>"), false},
		{"inline element in prose", two("see <b>x</b> here"), false},
		{"case-folded close", two("<SECTION>x</section>"), false},
		{"void elements", two("<br>\n<img src=\"u\">\n<hr/>\n<wbr>"), false},
		{"custom element closed", two("<x-panel>x</x-panel>"), false},
		{"opener in fenced code", two("```\n<span>\n```"), false},
		{"nested across blocks", two("<section>\n<span>\n\nx\n\n</span>\n</section>"), false},
		// Refused: an element left open, by opener shape.
		{"open block element", two("<section>"), true},
		{"open inline element", two("<span>"), true},
		{"open form element", two("<fieldset>"), true},
		{"open progress element", two("<progress>"), true},
		{"open custom element", two("<x-panel>"), true},
		{"open uppercase element", two("<SECTION>"), true},
		{"open mixed-case element", two("<SeCtIoN>"), true},
		{"open element with attribute", two("<section class=\"a\">"), true},
		{"self-closing non-void", two("<span/>"), true},
		{"opener shares a line", two("see <span> here"), true},
		{"open with a closed child", two("Log: <section><b>log</b>"), true},
		{"opener after the verdict",
			proseRun("Run 1", 2, allOK, "- row 3 could-not-check.", blocked) + "<section>\n\n" + rerun2, true},
		{"one of two closed", two("<section>\n<section>\n\n</section>"), true},
		{"crossed closes", two("<span><b>x</span></b>"), true},
		// Refused: a close tag the page does not read as one.
		{"close in a code span", two("<span> `</span>`"), true},
		{"close in a comment", two("<span> <!-- </span> -->"), true},
		{"close escaped", two("<span> \\</span>"), true},
		{"close in a link target", two("<span> [a](</span>)"), true},
		{"close in an image text", two("<span> ![</span>][r]\n\n[r]: u"), true},
		{"close in a shortcut image", two("<span> ![</span>]\n\n[</span>]: u"), true},
		// A comment or declaration that ends on its own line leaves the next
		// line a paragraph, where an unfinished tag is plain text; the rule
		// still counts the opener: over-refusal by design. The block rule
		// alone admits both (TestHeldScopeParagraphTag).
		{"comment closed, paragraph tag", two("<!-- a -->\na <b title=\""), true},
		{"declaration, paragraph tag", two("<!DOCTYPE html>\na <b title=\""), true},
		{"close in an attribute", two("<span><img alt=\"</span>\">"), true},
		{"close after a math span", two("<span> $a$ </span>"), true},
		{"close on a later line", two("<span>\n\n</span\n>"), true},
		{"close with an attribute", two("<span>x</span x>"), true},
		{"stray close tag", two("</span>"), true},
		{"close in a later paragraph", two("<span>\n\nx </span>"), true},
		{"paragraph opener, block close", two("x <span>\n\n</span>"), true},
		{"opener in a paragraph", two("<span> x <b>\n\n</b>\n</span>"), true},
		{"close after a pre block", two("<span>\n\n<pre>\n</pre>\nx </span>"), true},
		{"close in a link label", two("<span> [a][</span>]"), true},
		{"close in a table row", two("<span> x | </span>"), true},
		// Refused: the last entry holds an element across a line end.
		{"element wraps the last PASS",
			proseRun("Run 1", 2, allOK, "- row 3 could-not-check.", blocked) +
				proseRun("Run 2", 2, allOK, "<section>", "**VERIFY: PASS**") + "</section>\n", true},
		// Refused by design: text the page reads raw, and foreign content.
		{"raw-text element closed", two("<xmp>x</xmp>"), true},
		{"escapable raw text closed", two("<title>x</title>"), true},
	}
	cases := append([]heldScanCase(nil), base...)
	for _, eol := range []struct{ name, s string }{{"LF file", "\n"}, {"CRLF file", "\r\n"}} {
		for _, c := range base {
			file := strings.ReplaceAll(validBriefFM+"\n## Evidence\n\n"+c.evidence, "\n", eol.s)
			bf, ok, err := parseBriefFileBytes("x.md", []byte(file))
			if err != nil || !ok || bf == nil {
				t.Fatalf("%s, %s: parse ok=%v err=%v", c.name, eol.name, ok, err)
			}
			cases = append(cases, heldScanCase{c.name + ", " + eol.name, bf.Evidence, c.wantHeld})
		}
	}
	runHeldScanCasesOnce(t, cases)
}

// TestHeldScopeFailClosed: every place the scope cannot positively classify
// text, it reads it as before.
func TestHeldScopeFailClosed(t *testing.T) {
	runHeldLayerCases(t, []heldScanCase{
		{"last entry has only a loose PASS",
			heldRun + passRun("Non-implementer run — VERIFY: PASS"), true},
		{"last entry ends BLOCKED",
			heldRun + passRun("**VERIFY: PASS** on rows 1-2.\n\nVERIFY: BLOCKED (human-gate)."), true},
		{"last entry FAILs after its PASS",
			heldRun + passRun("**VERIFY: PASS**\n\n**VERIFY: FAIL** — row 2 regressed."), true},
		{"last entry PASS is struck",
			heldRun + passRun("~~**VERIFY: PASS**~~"), true},
		{"last entry PASS is quoted",
			heldRun + passRun("> **VERIFY: PASS**"), true},
		{"last entry PASS is fenced",
			heldRun + passRun("```\n**VERIFY: PASS**\n```"), true},
		{"later table, same entry",
			"**VERIFY: PASS**\n\n" + scopeHdr + scopeRow("2", "exit 0", "HELD — offline") +
				"\nOnline runner, same table:\n\n" + scopeHdr + scopeRow("2", "exit 0", "ok") + "\n**VERIFY: PASS**\n", true},
		{"heading inside a fence does not split",
			"### Run 1\n\n" + scopeHdr + scopeRow("2", "exit 0", "HELD — offline") +
				"\n```\n### Run 2\n```\n\n" + passTable + "\n**VERIFY: PASS**\n", true},
		// The later entry must re-run every row that held (S1).
		{"later entry re-ran nothing",
			heldRun + "### Run 2\n\n**VERIFY: PASS**\n", true},
		{"verdict heading after the run",
			strings.TrimSuffix(heldRun, "**VERIFY: PASS** (model) — row 1 green.\n\n") + "### Verdict\n\n**VERIFY: PASS**\n", true},
		{"notes heading after a preamble",
			"| # | Command | Exit | Result |\n|---|---|---|---|\n| 2 | `go test` | — | HELD — no runner online |\n\n### Notes\n\n**VERIFY: PASS**\n", true},
		{"FAIL then loose PASS after it",
			heldRun + passRun("**VERIFY: PASS**\n\nVERIFY: FAIL — row 2 regressed.\n\nVERIFY: PASS on re-run"), true},
		{"later entry re-ran row 1 only",
			heldRun + "### Run 2\n\n" + scopeHdr + scopeRow("1", "exit 0", "ok") + "\n**VERIFY: PASS**\n", true},
		{"later row records no result",
			heldRun + "### Run 2\n\n" + scopeHdr + scopeRow("1", "exit 0", "ok") + scopeRow("2", "exit 0", "~~ok~~") +
				"\n**VERIFY: PASS**\n", true},
		{"later table keyed by command",
			heldRun + "### Run 2\n\n| Command | Result |\n|---|---|\n| `go test ./integration/...` | ok |\n\n**VERIFY: PASS**\n", true},
		{"earlier hold in prose, last entry re-ran row 2 only",
			heldRun + "The integration run is HELD until a runner is online.\n\n" +
				"### Run 2\n\n" + scopeHdr + scopeRow("2", "exit 0", "exit 0, ok") + "\n**VERIFY: PASS**\n", true},
		{"earlier held row misaligned",
			"### Run 1\n\n" + scopeHdr + "| 2 | a | b | c | HELD — offline | 2026-10-01 | fixture-verifier |\n\n" +
				passRun("**VERIFY: PASS**"), true},
		// Heading, marker and covering table count only where they render (S2, S3).
		{"heading and PASS in one fence",
			heldRun + "```\n### Run 2\n\n" + passTable + "\n**VERIFY: PASS**\n```\n", true},
		{"level-4 heading with a PASS",
			heldRun + "#### Run 2\n\n" + passTable + "\n**VERIFY: PASS**\n", true},
		{"entry inside an HTML comment",
			heldRun + "<!--\n### Run 2\n\n" + passTable + "\n**VERIFY: PASS**\n-->\n", true},
		{"table inside an HTML comment",
			heldRun + "### Run 2\n\n<!--\n" + passTable + "-->\n\n**VERIFY: PASS**\n", true},
		{"PASS inside an HTML block",
			heldRun + "### Run 2\n\n" + passTable + "\n<div>\n**VERIFY: PASS**\n</div>\n", true},
		{"short fence inside a long one",
			heldRun + "````\n```\n### Run 2\n\n" + passTable + "\n**VERIFY: PASS**\n````\n", true},
		{"tilde line inside a backtick fence",
			heldRun + "```\n~~~\n### Run 2\n\n" + passTable + "\n**VERIFY: PASS**\n```\n", true},
		{"info string cannot close a fence",
			heldRun + "```\n```text\n### Run 2\n\n" + passTable + "\n**VERIFY: PASS**\n```\n", true},
		{"PASS continues a blockquote",
			heldRun + "### Run 2\n\n" + passTable + "\n> runner note\n**VERIFY: PASS**\n", true},
		{"PASS inside a code span",
			heldRun + "### Run 2\n\n" + passTable + "\n`quoted\n**VERIFY: PASS**`\n", true},
		{"PASS inside a strike",
			heldRun + "### Run 2\n\n" + passTable + "\n~~retracted\n**VERIFY: PASS**~~\n", true},
		{"PASS paragraph in a fence",
			heldRun + "### Run 2\n\n" + passTable + "\n```\n\n**VERIFY: PASS**\n```\n", true},
		{"PASS paragraph in a comment",
			heldRun + "### Run 2\n\n" + passTable + "\n<!--\n\n**VERIFY: PASS**\n-->\n", true},
		{"PASS as indented code",
			heldRun + "### Run 2\n\n" + passTable + "\n    **VERIFY: PASS**\n", true},
		{"PASS marker wraps a code span",
			heldRun + passRun("**VERIFY: PASS `tool** --x`"), true},
		{"table continues a blockquote",
			heldRun + "### Run 2\n\n> runner note\n" + passTable + "\n**VERIFY: PASS**\n", true},
		{"table as indented code",
			heldRun + "### Run 2\n\n    " + strings.ReplaceAll(strings.TrimSuffix(passTable, "\n"), "\n", "\n    ") + "\n\n**VERIFY: PASS**\n", true},
		{"quoted FAIL after the PASS",
			heldRun + passRun("**VERIFY: PASS**\n\n> VERIFY: FAIL — row 2 regressed."), true},
		{"misaligned row is read whole",
			"**VERIFY: PASS**\n\n" + scopeHdr +
				"| 4 | grep a | wc -l | could-not-check exit 6 | exit 0 | 2026-10-03 | fixture-verifier |\n", true},
		{"keyless table is read whole",
			"**VERIFY: PASS**\n\n| Check | Expect | Note |\n|---|---|---|\n| floor | could-not-check | ok |\n", true},
		{"table with no separator row",
			"**VERIFY: PASS**\n\n| # | Command | Expect | Observed |\n| 2 | `probe` | could-not-check | ok |\n", true},
		{"unrecognised table is read whole",
			"**VERIFY: PASS**\n\n| Check | Note |\n|---|---|\n| floor | expected could-not-check |\n", true},
		{"exit prose with no results table",
			"**VERIFY: PASS** — exit codes: could-not-check → exit 6.\n", true},
		{"longer header name is a result",
			"**VERIFY: PASS**\n\n| # | Command | Exit | Runner verdict |\n|---|---|---|---|\n| 2 | `probe` | 0 | HELD — offline |\n", true},
	})
}

// TestHeldScopeExitProse: prose beside a results table that says what an
// exit code means is not a disposition.
func TestHeldScopeExitProse(t *testing.T) {
	tbl := scopeHdr + scopeRow("1", "exit 0", "ok")
	runHeldLayerCases(t, []heldScanCase{
		{"arrow mapping",
			"**VERIFY: PASS**\n\n" + tbl + "\nRISK-VALUE: DERIVED — could-not-check→exit 6 (ExitUnverifiable)@main.go:17.\n", false},
		{"ascii arrow and equals",
			"**VERIFY: PASS**\n\n" + tbl + "\nNote: HELD -> exit 3; could-not-check = exit code 6.\n", false},
		{"parenthesised meaning",
			"**VERIFY: PASS**\n\n" + tbl + "\nThe guard exits 0 (clean), exit 2 (could-not-check).\n", false},
		{"mapping that names a row",
			"**VERIFY: PASS**\n\n" + tbl + "\nrow 3 could-not-check → exit 6, runner offline.\n", true},
		{"mapping beside a live hold refuses",
			"**VERIFY: PASS**\n\n" + tbl + "\ncould-not-check → exit 6; row 3 HELD, runner offline.\n", true},
	})
}

// scopeEvidence is a two-entry Evidence body for the ch/ fixtures: an earlier
// run with row 2 HELD, then a run of its own closing on verdict.
func scopeEvidence(verdict string) string {
	return "### Run 1 — 2026-07-08\n\n" +
		"| # | Command | Exit | Result | Date | Runner |\n" +
		"|---|---------|------|--------|------|--------|\n" +
		"| 1 | `go test ./...` | 0 | ok | 2026-07-08 | opus-verifier |\n" +
		"| 2 | `go test ./integration/...` | — | HELD — no runner online | 2026-07-08 | opus-verifier |\n\n" +
		"### Run 2 — 2026-07-10\n\n" +
		"| # | Command | Exit | Result | Date | Runner |\n" +
		"|---|---------|------|--------|------|--------|\n" +
		"| 1 | `go test ./...` | 0 | ok | 2026-07-10 | opus-verifier |\n" +
		"| 2 | `go test ./integration/...` | 0 | ok | 2026-07-10 | opus-verifier |\n\n" +
		verdict
}

// rewriteCHEvidence replaces brief num's Evidence body in a fresh ch/ root:
// everything from its first table header through the line holding last.
func rewriteCHEvidence(t *testing.T, root, file, last, ev string) {
	t.Helper()
	p := filepath.Join(root, "docs/streams/ch", file)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	from := strings.Index(s, "| # | Command | Exit")
	to := strings.Index(s, last)
	if from < 0 || to < from {
		t.Fatalf("fixture drifted: %s no longer carries its table and %q", file, last)
	}
	if err := os.WriteFile(p, []byte(s[:from]+ev+s[to+len(last):]), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestHeldScopeCard: through verifyIssues, an implemented brief whose HELD row
// a later strict-PASS entry replaced opens its verify-gate card; the twin
// whose later entry has only a loose PASS still suppresses it.
func TestHeldScopeCard(t *testing.T) {
	card := func(t *testing.T, ev string) bool {
		t.Helper()
		root := loadCHRoot(t)
		rewriteCHEvidence(t, root, "brief-04-implemented-held.md", "**VERIFY: PASS** (model) — row 1 green.", ev)
		streams, _, err := loadStreams(root)
		if err != nil {
			t.Fatal(err)
		}
		for _, iss := range verifyIssues(root, streams, map[string]bool{}) {
			if iss.Brief == "ch/04" {
				return true
			}
		}
		return false
	}
	if !card(t, scopeEvidence("**VERIFY: PASS** (model) — all rows green.")) {
		t.Errorf("ch/04 whose HELD row a later strict-PASS entry replaced must open its verify-gate card")
	}
	// The earlier entry's strict PASS keeps hasVerifyPass true, so only the
	// scope decides the twin.
	twin := strings.Replace(scopeEvidence("Run 2 — VERIFY: PASS (model)."), "### Run 1 — 2026-07-08\n\n",
		"### Run 1 — 2026-07-08\n\n**VERIFY: PASS** (model) — row 1 green.\n\n", 1)
	if card(t, twin) {
		t.Errorf("ch/04 whose later entry has only a loose PASS must still suppress its card")
	}
}

// TestHeldScopeCloseVerify: the human done close on the verified path admits
// a HELD row a later strict-PASS entry replaced, and refuses the loose twin.
func TestHeldScopeCloseVerify(t *testing.T) {
	now := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	const last = "**VERIFY: PASS** (model) — row 1 green."

	root := loadCHRoot(t)
	rewriteCHEvidence(t, root, "brief-01-verified-held.md", last, scopeEvidence("**VERIFY: PASS** (model) — all rows green."))
	if err := closeVerify(root, "ch/01", now); err != nil {
		t.Errorf("closeVerify on a superseded hold refused: %v", err)
	}

	root = loadCHRoot(t)
	rewriteCHEvidence(t, root, "brief-01-verified-held.md", last, scopeEvidence("Run 2 — VERIFY: PASS (model)."))
	err := closeVerify(root, "ch/01", now)
	if err == nil || !strings.Contains(err.Error(), "HELD") {
		t.Errorf("closeVerify with only a loose later PASS: err=%v, want a HELD refusal", err)
	}
}
