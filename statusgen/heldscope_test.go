package main

import (
	"os"
	"path/filepath"
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

// heldRun is an earlier verifier run whose row 2 was HELD.
const heldRun = "### Run 1 — 2026-10-01\n\n" +
	"| # | Command | Exit | Result | Date | Runner |\n" +
	"|---|---------|------|--------|------|--------|\n" +
	"| 1 | `go test ./...` | 0 | ok | 2026-10-01 | fixture-verifier |\n" +
	"| 2 | `go test ./integration/...` | — | HELD — no runner online | 2026-10-01 | fixture-verifier |\n\n" +
	"Row 2 is HELD until a runner is online.\n\n" +
	"**VERIFY: PASS** (model) — row 1 green.\n\n"

// passRun is a later run of its own with every row green and a closing verdict.
func passRun(verdict string) string {
	return "### Run 2 — 2026-10-03\n\n" + scopeHdr +
		scopeRow("1", "exit 0", "exit 0, ok") +
		scopeRow("2", "exit 0", "exit 0, ok") + "\n" + verdict + "\n"
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

// TestHeldScopeSupersededAdmitted: HELD in an earlier entry is admitted once
// a later entry records its own strict PASS.
func TestHeldScopeSupersededAdmitted(t *testing.T) {
	runHeldScanCases(t, []heldScanCase{
		{"held row and prose in an earlier entry",
			heldRun + passRun("**VERIFY: PASS**"), false},
		{"held preamble before the first entry",
			"Row 2 HELD — no runner online.\n\n" + passRun("**VERIFY: PASS** — all rows green."), false},
		{"three entries, last one passes",
			heldRun + heldRun + passRun("**VERIFY: PASS**"), false},
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
				"\n```\n### Run 2\n```\n\n**VERIFY: PASS**\n", true},
		{"misaligned row is read whole",
			"**VERIFY: PASS**\n\n" + scopeHdr +
				"| 4 | grep a | wc -l | could-not-check exit 6 | exit 0 | 2026-10-03 | fixture-verifier |\n", true},
		{"unrecognised table is read whole",
			"**VERIFY: PASS**\n\n| Check | Note |\n|---|---|\n| floor | expected could-not-check |\n", true},
		{"exit prose with no results table",
			"**VERIFY: PASS** — exit codes: could-not-check → exit 6.\n", true},
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
