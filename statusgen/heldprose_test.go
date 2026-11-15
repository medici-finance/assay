package main

import "testing"

// regression: #4721
//
// Ruling A on #4721: the hold scan reads only witness rows and lines that
// state a current hold (a row result, a HELD marker, a RISK-VALUE verdict);
// free prose naming a state word is not a hold. Each line below sits beside a
// strict **VERIFY: PASS** with no results table and records no current hold.
// The first four are quoted from #4721 (graph-execution/28 and /29) and #4200;
// the rest are one negative case per edge of the ruling.
func TestHeldProseNotHolds(t *testing.T) {
	const pass = "**VERIFY: PASS** — every original row passes.\n\n"
	var cases []heldScanCase
	for _, c := range []struct{ name, line string }{
		{"ge/28 zero tally", "Accounting: original obligations 9, passed 9 with mutation controls killed; observed failed obligations 0; could-not-check 0. The downstream consumption holds above stay with their owning briefs."},
		{"ge/29 mutation prose", "Each Expect cell's named mutation, applied to the scratch copy, made that row's exact command exit 1: the new adapter rounds could-not-check to pass."},
		{"ge/29 past flip hold", "Why this entry exists: the earlier PASS landed through #4602, but its board flip was held on #4536. This entry is a fresh, complete run of every row."},
		{"blind list held", "pass 2: blind list held the gone item"},
		// Edge 1a-i: lower-case held / could-not-check in prose is not a marker.
		{"edge 1 lower held", "The cache held the previous value until the next read."},
		{"edge 1 lower cnc", "A timeout maps to could-not-check, never to permitted."},
		{"edge 1 lower list item", "- smoke run: could-not-check"},
		// Edge 3a: a current-tense hold on a step, no row, marker or verdict.
		{"edge 3 step held", "The live smoke step was held."},
		{"edge 3 step cnc", "The live smoke step could-not-check on this host."},
		{"edge 3 step stays held", "The canary is still held behind the feature flag."},
	} {
		cases = append(cases, heldScanCase{c.name, pass + c.line + "\n", false})
	}
	runHeldLayerCases(t, cases)
}

// regression: #4721
//
// The positive controls: every line below still refuses a strict PASS, one
// per signal the ruling keeps read, so a loosened gate is seen.
func TestHeldProseKeepsSignals(t *testing.T) {
	const pass = "**VERIFY: PASS** — every original row passes.\n\n"
	var cases []heldScanCase
	for _, c := range []struct{ name, line string }{
		// Edge 1a-i: a state word not all in lower case is a HELD marker.
		{"edge 1 upper HELD", "Live cluster probe: HELD"},
		{"edge 1 title Held", "The live smoke step stays Held, no runner online."},
		{"edge 1 mixed hElD", "Smoke run: hElD"},
		{"edge 1 upper cnc", "Live probe COULD-NOT-CHECK 0 clusters reachable."},
		{"edge 1 title cnc", "The live smoke step is Could-Not-Check."},
		{"edge 1 mixed cnc", "Smoke run: could-NOT-check"},
		{"edge 1 sentence start", "Held: the live smoke step, no runner online."},
		{"labelled caps held", "Row 2 HELD: 0 runners online."},
		// Edge 2a: a prose line naming a row puts its state word on it.
		{"edge 2 row space", "Row 3 could-not-check: 0 clusters reachable."},
		{"edge 2 row lower", "row 3 held"},
		{"edge 2 row joined", "Row3 held, no runner online."},
		{"edge 2 r short", "R3 held"},
		{"edge 2 rows plural", "rows 2 and 3 could-not-check offline."},
		// A RISK-VALUE verdict line stays read at any case.
		{"risk verdict", "RISK-VALUE: DERIVED — smoke run = could-not-check @ runner.log:1 — no runner online."},
		// Witness rows stay read whatever the case.
		{"table result cell", "| # | Command | Result |\n|---|---|---|\n| 1 | `go test ./...` | held |"},
		{"pipe line", "Accounting: could-not-check 0; passed 8 | n/a."},
		{"html cell", "<td>held</td>"},
	} {
		cases = append(cases, heldScanCase{c.name, pass + c.line + "\n", true})
	}
	runHeldLayerCases(t, cases)
}
