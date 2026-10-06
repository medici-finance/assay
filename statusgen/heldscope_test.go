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
// (closeVerifyHeldRefusal) must agree on it.

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
	runHeldScanCases(t, cases)
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
	runHeldScanCases(t, cases)
}

// TestHeldScopeCurrentHeldRefuses: a genuine held row in the current entry
// still refuses, whatever the scope rules admit around it.
func TestHeldScopeCurrentHeldRefuses(t *testing.T) {
	runHeldScanCases(t, []heldScanCase{
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
	runHeldScanCases(t, []heldScanCase{
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
	runHeldScanCases(t, []heldScanCase{
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

// TestHeldScopeFailClosed: every place the scope cannot positively classify
// text, it reads it as before.
func TestHeldScopeFailClosed(t *testing.T) {
	runHeldScanCases(t, []heldScanCase{
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
		{"earlier hold in prose",
			heldRun + "Row 2 is HELD until a runner is online.\n\n" + passRun("**VERIFY: PASS**"), true},
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
	runHeldScanCases(t, []heldScanCase{
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
