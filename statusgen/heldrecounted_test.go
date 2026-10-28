package main

import "testing"

// regression: #4721
//
// The lines below are quoted from #4721 (graph-execution/28 and /29) and #4200
// (desktools-go-git/06). Each sits beside a strict **VERIFY: PASS** and
// records no current hold: an accounting tally that closes on a zero, a
// sentence describing what a mutation does, a sentence recounting a past
// board-flip hold, and a sourced RISK-VALUE derivation that names a product
// state.
func TestHeldRecountedNotHolds(t *testing.T) {
	const pass = "**VERIFY: PASS** — every original row passes.\n\n"
	var cases []heldScanCase
	for _, c := range []struct{ name, line string }{
		{"ge/28 zero tally", "Accounting: original obligations 9, passed 9 with mutation controls killed; observed failed obligations 0; could-not-check 0. Supplemental row 10 executed exit 0 with 0 disproved and no acceptance credit; the downstream consumption holds above stay with their owning briefs. Executor envelope: a local, digest-unpinned golang image with --network none, plus a source-built verifyrun with container-netns. This is not the pinned harness image or a qualified installed Linux runner. If the gate requires that runner, this evidence routes back to the environment tracker, medici-finance/assay#1800."},
		{"ge/29 mutation prose", "Supplemental failure controls, no acceptance credit: `python3 statusgen/testdata/regressionfloor/mutate.py` exited 0 and printed KILLED for all 17 delivered mutations, each by its named failing test (inventory-name, uncertainty-pass, stale-difference). Each Expect cell's named mutation, applied to the scratch copy, made that row's exact command exit 1: row 1 (one inventory test name misspelled), row 2 (new adapter rounds could-not-check to pass), row 3 (staleness check disabled), row 4 (zero selection returns success)."},
		{"ge/29 past flip hold", "Why this entry exists: the earlier PASS at 0c292571a3e0794598cb11733d0b6c966c89aae0 landed through #4602, but its board flip was held on #4536. This entry is a fresh, complete run of every row."},
		{"#4200 RISK-VALUE mapping", "RISK-VALUE: DERIVED — listTimeout = 45 * time.Second @ tools/desk/internal/deskkit/preflight.go:1098 — carried over unchanged from the deleted dry-run probe's context.WithTimeout(45*time.Second); a timeout maps to could-not-check, never to permitted. Reversible operational knob, ranked last."},
		{"tally label only", "Summary: passed 8; could-not-check 0, held 0."},
		{"tally in parentheses", "Every row ran (could-not-check 0)."},
		{"tally with colon", "Accounting: passed 9; could-not-check: 0."},
		{"flip had been held", "The earlier flip had been held on medici-finance/assay#4536."},
		{"coerces mutation", "The mutation coerces could-not-check to green, and the row then fails."},
	} {
		cases = append(cases, heldScanCase{c.name, pass + c.line + "\n", false})
	}
	runHeldLayerCases(t, cases)
}

// regression: #4721
//
// Prose stays read: every shape below refuses a strict PASS, and each sits
// beside one of the three excused shapes so a loosened excuse is seen. The
// first group is what the reviewer of the first revision showed released.
func TestHeldRecountedKeepsHolds(t *testing.T) {
	const pass = "**VERIFY: PASS** — every original row passes.\n\n"
	var cases []heldScanCase
	for _, c := range []struct{ name, line string }{
		{"labelled caps held", "Live cluster probe: HELD"},
		{"labelled held", "Live cluster probe: held"},
		{"labelled list item", "- smoke run: could-not-check"},
		{"labelled reason dash", "Live cluster probe: could-not-check — no cluster contact permitted offline."},
		{"labelled reason comma", "- Smoke run: could-not-check, no runner online."},
		{"labelled held comma", "- Smoke run: held, no runner online."},
		{"was held", "The live smoke step was held."},
		{"was held semicolon", "The smoke test was held; the runner was offline."},
		{"stays Held", "The live smoke step stays Held, no runner online."},
		{"is Could-Not-Check", "The live smoke step is Could-Not-Check."},
		{"could-not-check on host", "The live smoke step could-not-check on this host."},
		{"could-not-check since", "The deploy probe could-not-check since no runner was online."},
		{"could-not-check parens", "The deploy probe could-not-check (offline)."},
		{"we could-not-check", "We could-not-check the TLS probe (runner offline)."},
		{"still held", "The canary is still held behind the feature flag."},
		{"note colon", "Note: the e2e suite could-not-check on macOS; Linux runner needed."},
		{"cluster step dash", "The cluster step was held — offline, no cluster contact."},
		{"exit mapping prose", "exit codes: could-not-check → exit 6."},
		{"policy sentence", "missing and could-not-check cannot release assessment."},
		{"zero then caps held", "Row 3 HELD 0 runners online."},
		{"zero then caps cnc", "Live probe COULD-NOT-CHECK 0 clusters reachable."},
		{"zero colon caps held", "Row 2 HELD: 0 runners online."},
		{"zero colon row cnc", "Row 3 could-not-check: 0 clusters reachable."},
		{"zero then noun", "Accounting: passed 8; could-not-check 0 clusters reachable."},
		{"zero after row", "Verify row 3 could-not-check 0."},
		{"zero beside short ratio", "Accounting: 1 of 2 rows passed; could-not-check 0."},
		{"zero tally caps", "Accounting: passed 8; COULD-NOT-CHECK 0."},
		{"zero tally mixed case", "Accounting: passed 8; Could-Not-Check 0."},
		{"nonzero tally", "Accounting: passed 8; could-not-check 2."},
		{"rounds caps", "The adapter rounds COULD-NOT-CHECK to pass."},
		{"rounds with row", "Row 2 could-not-check, and the adapter rounds it to pass."},
		{"rounds after held", "Row 2 is held, so the adapter rounds could-not-check to pass here and also holds row 3 could-not-check."},
		{"maps without verb", "Row 2 could-not-check to pass the gate."},
		{"flip held present", "Its board flip is held on #4536."},
		{"flip held no tracker", "Its board flip was held on the runner."},
		{"flip held and still", "Its board flip was held on #4536 and the row is still held."},
		{"flip held caps", "Its board flip was HELD on #4536."},
		{"flip held reason", "Its board flip was held on #4536 pending the runner."},
		{"row before", "Verify row 3 could-not-check on this host."},
		{"rows plural", "Rows 2 and 4 are held."},
		{"reason pending", "The cluster step is held pending a runner."},
		{"count before", "Accounting: 9 obligations, 8 passed, 1 could-not-check."},
		{"lead dash", "could-not-check — OFFLINE (no cluster contact permitted)."},
		{"lead emphasis", "**Held at `implemented` — the deliverable still carries a confirmed fail-open.**"},
		{"verdict blocked", "VERIFY: BLOCKED — the cluster rows could-not-check offline."},
		{"risk record", "RISK-VALUE: DERIVED — smoke run = \"could-not-check\" @ runner.log:1 — no runner online."},
		{"tally then second hold", "Accounting: passed 8; could-not-check 0; the live probe is held."},
		{"pipe line", "Accounting: could-not-check 0; passed 8 | n/a."},
		{"rounds without target", "The adapter rounds could-not-check down to fail."},
		{"tracker without flip", "The live smoke step was held on #4536."},
		{"struck span beside tally", "Accounting: ~~passed 7~~ passed 8; could-not-check 0."},
	} {
		cases = append(cases, heldScanCase{c.name, pass + c.line + "\n", true})
	}
	runHeldLayerCases(t, cases)
}
