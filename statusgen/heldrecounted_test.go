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
		{"tally in parentheses", "Every check ran (could-not-check 0)."},
		{"tally with colon", "Accounting: passed 9; could-not-check: 0."},
		{"flip had been held", "The earlier flip had been held on medici-finance/assay#4536."},
		{"mutation sentence", "The mutation rounds could-not-check to pass, and the test then fails."},
	} {
		cases = append(cases, heldScanCase{c.name, pass + c.line + "\n", false})
	}
	runHeldLayerCases(t, cases)
}

// regression: #4721
//
// Prose stays read: every shape below refuses a strict PASS, and each sits
// beside one of the three excused shapes so a loosened excuse is seen. The
// first group is what the reviewer of the first revision showed released; the
// later groups are what the reviewers of the second showed released, and one
// line per guard of each excused shape, built so that guard alone keeps it read.
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

		// The second revision's reviewers: real holds the wider excuses released.
		{"converted offline", "Row 3: the runner was offline, so I converted could-not-check to pass."},
		{"promoted no runner", "We promoted could-not-check to pass for the live smoke row; no runner was online."},
		{"coerced to green", "The live probe never reached the cluster; the verifier coerced could-not-check to green."},
		{"rounds runner absent", "Row 4 passed only because verifyrun rounds could-not-check to pass when the runner is absent."},
		{"upgraded to a pass", "The harness upgraded could-not-check to a pass on this host (no cluster contact)."},
		{"rounded into success", "Row 2 smoke: rounded could-not-check into success, runner offline."},
		{"converts row label", "Row 3: the gate converts could-not-check to pass, so row 3 stays unproven."},
		{"rounded re-run", "Smoke run: rounded could-not-check to green; re-run needed."},
		{"flip row named", "Row 3's flip was held on #4536."},
		{"flip verified", "The verified flip was held on #4536."},
		{"flip this entry", "This entry's flip had been held on medici-finance/assay#4536."},
		{"flip comma still is", "The flip was held on #4536, and still is."},
		{"flip comma pending", "Its board flip was held on #4536, pending the runner."},
		{"flip semicolon reason", "The verified flip was held on #4536; the runner is offline."},
		{"flip had been comma", "This brief's flip had been held on medici-finance/assay#4536, awaiting the runner."},
		{"flip paren reason", "Its board flip was held on #4536) and the runner is offline."},
		{"tally paren after row", "Verify row 3 (could-not-check 0)."},
		{"tally comma after row", "Row 3, could-not-check 0."},
		{"tally colon after row", "Row 3 (could-not-check: 0)."},
		{"td tally", "<td>(could-not-check 0)</td>"},
		{"tally beside td", "Accounting: passed 8; could-not-check 0. <td>x</td>"},

		// Zero tally: one guard each.
		{"zero then noun", "Accounting: passed 8; could-not-check 0 clusters reachable."},
		{"zero no clause break", "The smoke step could-not-check 0."},
		{"zero after row", "Verify row 3 could-not-check 0."},
		{"zero beside short ratio", "Accounting: 1 of 2 rows passed; could-not-check 0."},
		{"zero tally caps", "Accounting: passed 8; COULD-NOT-CHECK 0."},
		{"zero tally mixed case", "Accounting: passed 8; Could-Not-Check 0."},
		{"nonzero tally", "Accounting: passed 8; could-not-check 2."},
		{"tally then second hold", "Accounting: passed 8; could-not-check 0; the live probe is held."},
		{"pipe line", "Accounting: could-not-check 0; passed 8 | n/a."},
		{"struck span beside tally", "Accounting: ~~passed 7~~ passed 8; could-not-check 0."},
		{"count before", "Accounting: 9 obligations, 8 passed, 1 could-not-check."},

		// Mutation sentence: one guard each.
		{"rounds caps", "The mutation rounds COULD-NOT-CHECK to pass."},
		{"mutation verb missing", "The mutation maps could-not-check to pass."},
		{"mutation past verb", "The mutation rounded could-not-check to pass."},
		{"mutation target green", "The mutation rounds could-not-check to green."},
		{"mutation no target", "The mutation rounds could-not-check down to fail."},
		{"verb without mutation", "The verifier rounds could-not-check to pass."},
		{"mutation offline", "The mutation rounds could-not-check to pass while the runner is offline."},
		{"mutation row outside", "Row 4: the mutation rounds could-not-check to pass."},
		{"mutation row inside", "Mutations: (row 2 rounds could-not-check to pass)."},
		{"mutation paren open", "Mutations: row 2 (the adapter rounds could-not-check to pass on this host) failed."},
		{"rounds with row", "Row 2 could-not-check, and the adapter rounds it to pass."},
		{"rounds after held", "Row 2 is held, so the adapter rounds could-not-check to pass here and also holds row 3 could-not-check."},
		{"maps without verb", "Row 2 could-not-check to pass the gate."},

		// Past flip hold: one guard each.
		{"flip held present", "The earlier flip is held on #4536."},
		{"flip held no tracker", "The earlier flip was held on the runner."},
		{"flip held caps", "The earlier flip was HELD on #4536."},
		{"flip held reason", "The earlier flip was held on #4536 pending the runner."},
		{"flip earlier comma", "The earlier flip was held on #4536, pending the runner."},
		{"flip earlier semicolon", "The earlier flip was held on #4536; the runner is offline."},
		{"flip earlier paren", "The earlier flip was held on #4536) and the runner is offline."},
		{"flip held and still", "The earlier flip was held on #4536 and the row is still held."},
		{"flip still next", "The earlier flip was held on #4536. It still is."},
		{"flip no earlier", "Its board flip was held on #4536."},
		{"flip earlier row", "Row 3's earlier flip was held on #4536."},
		{"flip earlier verified", "The earlier verified flip was held on #4536."},
		{"tracker without flip", "The earlier smoke step was held on #4536."},

		// Other prose.
		{"row before", "Verify row 3 could-not-check on this host."},
		{"rows plural", "Rows 2 and 4 are held."},
		{"reason pending", "The cluster step is held pending a runner."},
		{"lead dash", "could-not-check — OFFLINE (no cluster contact permitted)."},
		{"lead emphasis", "**Held at `implemented` — the deliverable still carries a confirmed fail-open.**"},
		{"verdict blocked", "VERIFY: BLOCKED — the cluster rows could-not-check offline."},
		{"risk record", "RISK-VALUE: DERIVED — smoke run = \"could-not-check\" @ runner.log:1 — no runner online."},
	} {
		cases = append(cases, heldScanCase{c.name, pass + c.line + "\n", true})
	}
	runHeldLayerCases(t, cases)
}
