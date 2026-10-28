package main

import "testing"

// regression: #4721
//
// The lines below are quoted from #4721 (graph-execution/28 and /29) and
// #4200 (desktools-go-git/06). Each sits beside a strict **VERIFY: PASS** and
// records no current hold: an accounting tally with a zero count, a sentence
// describing what a mutation does, a sentence recounting a past board-flip
// hold, and a sourced RISK-VALUE derivation that names a product state.
func TestHeldStructuredNotHolds(t *testing.T) {
	const pass = "**VERIFY: PASS** — every original row passes.\n\n"
	var cases []heldScanCase
	for _, c := range []struct{ name, line string }{
		{"ge/28 zero tally", "Accounting: original obligations 9, passed 9 with mutation controls killed; observed failed obligations 0; could-not-check 0. Supplemental row 10 executed exit 0 with 0 disproved and no acceptance credit; the downstream consumption holds above stay with their owning briefs. Executor envelope: a local, digest-unpinned golang image with --network none, plus a source-built verifyrun with container-netns. This is not the pinned harness image or a qualified installed Linux runner. If the gate requires that runner, this evidence routes back to the environment tracker, medici-finance/assay#1800."},
		{"ge/29 mutation prose", "Supplemental failure controls, no acceptance credit: `python3 statusgen/testdata/regressionfloor/mutate.py` exited 0 and printed KILLED for all 17 delivered mutations, each by its named failing test (inventory-name, uncertainty-pass, stale-difference). Each Expect cell's named mutation, applied to the scratch copy, made that row's exact command exit 1: row 1 (one inventory test name misspelled), row 2 (new adapter rounds could-not-check to pass), row 3 (staleness check disabled), row 4 (zero selection returns success)."},
		{"ge/29 past flip hold", "Why this entry exists: the earlier PASS at 0c292571a3e0794598cb11733d0b6c966c89aae0 landed through #4602, but its board flip was held on #4536. This entry is a fresh, complete run of every row."},
		{"#4200 RISK-VALUE mapping", "RISK-VALUE: DERIVED — listTimeout = 45 * time.Second @ tools/desk/internal/deskkit/preflight.go:1098 — carried over unchanged from the deleted dry-run probe's context.WithTimeout(45*time.Second); a timeout maps to could-not-check, never to permitted. Reversible operational knob, ranked last."},
	} {
		cases = append(cases, heldScanCase{c.name, pass + c.line + "\n", false})
	}
	// Three prose lines that refused before only because every prose
	// occurrence counted: each names a state and records no hold. They were
	// fail-closed fixtures of the cell-only and table-only excuses (#4294,
	// #4607); rule 6 reads none of them.
	cases = append(cases,
		heldScanCase{"prose line, not a cell", annObs("0", "ok") + "PASS: a could-not-check mission section\n", false},
		heldScanCase{"plain prose policy sentence", pass + "missing and could-not-check cannot release assessment.\n", false},
		heldScanCase{"exit prose with no results table", "**VERIFY: PASS** — exit codes: could-not-check → exit 6.\n", false},
	)
	runHeldLayerCases(t, cases)
}

// regression: #4721
//
// Every structured hold signal still refuses a strict PASS when it sits in
// prose: rule 6 narrows what prose counts, never what a disposition is.
func TestHeldStructuredKeepsHolds(t *testing.T) {
	const pass = "**VERIFY: PASS** — every original row passes.\n\n"
	var cases []heldScanCase
	for _, c := range []struct{ name, line string }{
		{"marker caps", "Row results recorded; the live smoke step is HELD."},
		{"row before", "Verify row 3 could-not-check on this host."},
		{"rows before plural", "Rows 2 and 4 are held."},
		{"row before adverb", "Verify row 4 is therefore could-not-check until that release is cut."},
		{"row before colon", "row 5: held"},
		{"row before code unmasked", "A stray ` opens this paragraph.\nVerify row 4 is therefore `could-not-check` until that release is cut."},
		{"row after", "could-not-check on row 3, the runner was offline."},
		{"row after paren", "The probe stays held (row 2)."},
		{"reason pending", "The cluster step is held pending a runner."},
		{"reason because", "The cluster step could-not-check because no cluster was reachable."},
		{"count before", "Accounting: 9 obligations, 8 passed, 1 could-not-check."},
		{"count word before", "One could-not-check remains on the live probe."},
		{"count after", "Accounting: passed 8; could-not-check 2."},
		{"count after colon", "Accounting: passed 8; could-not-check: 1."},
		{"lead dash", "could-not-check — OFFLINE (no cluster contact permitted)."},
		{"lead emphasis", "**Held at `implemented` — the deliverable still carries a confirmed fail-open.**"},
		{"lead plain", "Held at `implemented`: the deliverable still carries a confirmed fail-open."},
		{"status held at", "Verify pass (VERIFY: PASS — records Evidence; status held at `implemented`, human flip via the gate)."},
		{"risk record", "RISK-VALUE: DERIVED — smoke run = \"could-not-check\" @ runner.log:1 — no runner online."},
		{"verdict blocked", "VERIFY: BLOCKED — the cluster rows could-not-check offline."},
	} {
		cases = append(cases, heldScanCase{c.name, pass + c.line + "\n", true})
	}
	runHeldLayerCases(t, cases)
}
