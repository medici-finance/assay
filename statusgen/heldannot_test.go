package main

import (
	"strings"
	"testing"
)

// annHdr is the results-table header the #4294 annotated-key fixtures use.
const annHdr = "| # | Command | Exit | Result | Date | Runner |\n" +
	"|---|---------|------|--------|------|--------|\n"

// annRow is one results row of annHdr: key, exit and result are its free cells.
func annRow(key, exit, result string) string {
	return "| " + key + " | `bash tests/stream-brief/07.sh` | " + exit + " | " + result + " | 2026-10-07 | fixture-verifier |\n"
}

// annEntry is one Evidence entry: a heading, a results table of the given
// rows and a closing verdict line.
func annEntry(title, verdict string, rows ...string) string {
	return "### " + title + "\n\n" + annHdr + strings.Join(rows, "") + "\n" + verdict + "\n\n"
}

// annLater is the last entry: rows 1 to 3 re-ran clean under bare keys, closing
// on a strict PASS.
var annLater = annEntry("Verification — 2026-10-20", "**VERIFY: PASS**",
	annRow("1", "0", "ok"), annRow("2", "0", "ok"), annRow("3", "0", "PASS"))

// annHeld is an earlier entry whose annotated row 3 was HELD.
func annHeld(key string) string {
	return annEntry("Verification — 2026-10-07", "**VERIFY: BLOCKED**",
		annRow("1", "0", "ok"), annRow(key, "—", "HELD — no runner online"))
}

// annHeldMisaligned is annHeld for the exact #4294 row id plus a row that
// cannot be aligned, so whole-entry supersession (rule 3b) is off and only
// the row-by-row rule (3a) can drop the hold.
var annHeldMisaligned = strings.Replace(annHeld("3 (Verify 3, negative)"), "\n\n**VERIFY: BLOCKED**",
	"\n| 4 | `a | b` | 0 | ok | 2026-10-07 | fixture-verifier |\n\n**VERIFY: BLOCKED**", 1)

// TestHeldScopeAnnotatedKey: an earlier row keyed "3 (Verify 3, negative)"
// that a later strict PASS re-ran as the bare "3" is the same Verify row, so
// the later run supersedes it (rule 3a): the annotation names the Verify row
// it ran and nothing else. A second, planted instance under a Row header with
// a different annotation is admitted the same way (defect class).
// regression: #4294
func TestHeldScopeAnnotatedKey(t *testing.T) {
	rowHdr := "| Row | Command | Exit | Result |\n|---|---|---|---|\n"
	planted := "### Run 1\n\n" + rowHdr + "| 2 (verify 2) | `x` | — | HELD — runner offline |\n\n" +
		"### Run 2\n\n" + rowHdr + "| 2 | `x` | 0 | ok |\n\n**VERIFY: PASS**\n"
	runHeldLayerCases(t, []heldScanCase{
		{"annotated held row re-ran bare", annHeld("3 (Verify 3, negative)") + annLater, false},
		{"annotation with no number", annHeld("3 (negative)") + annLater, false},
		{"bold annotated key", annHeld("**3 (Verify 3, negative)**") + annLater, false},
		{"planted second instance, Row header", planted, false},
		{"row by row only (an earlier row cannot be aligned)", annHeldMisaligned + annLater, false},
	})
	if s := heldScanScope(annHeld("3 (Verify 3, negative)") + annLater); !s.whole {
		t.Errorf("annotated key: whole=%v, want the last entry to cover every earlier row", s.whole)
	}
	if s := heldScanScope(annHeldMisaligned + annLater); s.whole || s.from < 0 {
		t.Errorf("misaligned earlier row: whole=%v from=%d, want row-by-row supersession only", s.whole, s.from)
	}
}

// TestHeldScopeAnnotatedKeyRefuses: the key normalisation never merges rows
// that may differ. Each case is a held row that must stay live.
// regression: #4294
func TestHeldScopeAnnotatedKeyRefuses(t *testing.T) {
	subRows := annEntry("Verification — 2026-10-07", "**VERIFY: BLOCKED**",
		annRow("3 (positive)", "0", "ok"), annRow("3 (negative)", "—", "HELD — no runner online"))
	bareAndSub := annEntry("Verification — 2026-10-07", "**VERIFY: BLOCKED**",
		annRow("3", "0", "ok"), annRow("3 (negative)", "—", "HELD — no runner online"))
	twoEntries := annHeld("3 (negative)") +
		annEntry("Verification — 2026-10-12", "**VERIFY: BLOCKED**", annRow("3 (positive)", "0", "ok")) + annLater
	rowHdr := annEntry("Verification — 2026-10-07", "**VERIFY: BLOCKED**")
	rowHdr = strings.Replace(rowHdr, "| # |", "| Row |", 1)
	rowHdr = strings.Replace(rowHdr, "\n\n**VERIFY", "\n"+annRow("3 (Verify 3)", "—", "HELD — no runner online")+"\n**VERIFY", 1)
	lastSub := annHeld("3 (Verify 3, negative)") + annEntry("Verification — 2026-10-20", "**VERIFY: PASS**",
		annRow("1", "0", "ok"), annRow("2", "0", "ok"), annRow("3 (Verify 3, positive)", "0", "ok"))
	runHeldLayerCases(t, []heldScanCase{
		{"annotation names another row", annHeld("3 (Verify 4)") + annLater, true},
		{"annotation names a date", annHeld("3 (2026-10-07)") + annLater, true},
		{"two numbers, one other", annHeld("3 (Verify 3, row 4)") + annLater, true},
		{"sub-rows of one key in an entry", subRows + annLater, true},
		{"bare and annotated in one entry", bareAndSub + annLater, true},
		{"two annotations across entries", twoEntries, true},
		{"lettered sub-row", annHeld("3a (negative)") + annLater, true},
		{"nested parentheses", annHeld("3 (Verify 3 (negative))") + annLater, true},
		{"text after the annotation", annHeld("3 (Verify 3) negative") + annLater, true},
		{"no leading number", annHeld("(Verify 3) 3") + annLater, true},
		{"Row header against #", rowHdr + annLater, true},
		{"later re-run not clean", annHeld("3 (Verify 3, negative)") + strings.Replace(annLater, "| 0 | PASS |", "| 0 | PASS (no re-run) |", 1), true},
		{"last entry annotates differently", lastSub, true},
		{"exact #4294 id, later run skipped row 3", annHeld("3 (Verify 3, negative)") + annEntry("Verification — 2026-10-20",
			"**VERIFY: PASS**", annRow("1", "0", "ok"), annRow("2", "0", "ok")), true},
		{"exact #4294 id, held in the last entry", annLater + annHeld("3 (Verify 3, negative)") +
			annEntry("Verification — 2026-10-21", "**VERIFY: PASS**", annRow("3 (Verify 3, negative)", "—", "HELD — no runner online")), true},
		{"exact #4294 id, later bare row held", annHeld("3 (Verify 3, negative)") + strings.Replace(annLater, "| 0 | PASS |", "| — | HELD — no runner online |", 1), true},
		{"exact #4294 id, row by row only, later not clean", annHeldMisaligned + strings.Replace(annLater, "| 0 | PASS |", "| 0 | same as Run 1 |", 1), true},
	})
}

// annObs is a single strict-PASS entry whose row 3 has the given Exit and
// Result cells.
func annObs(exit, result string) string {
	return annEntry("Verification — 2026-10-20", "**VERIFY: PASS**",
		annRow("1", "0", "ok"), annRow("3", exit, result))
}

// annQuote is the shape of the #4294 row: a passing negative case whose
// Result records that the product printed its could-not-check section.
const annQuote = "\"PASS: negative\" — the planted brief yields its named diagnostic, a lint PROBLEM and a could-not-check mission section with no content"

// TestHeldScopeObservedOutput: a row whose Exit cell is clean and whose
// Result opens on PASS reads "a could-not-check mission section" as what the
// product printed, not as the row's disposition. A second, planted instance
// under other header spellings is admitted the same way (defect class).
// regression: #4294
func TestHeldScopeObservedOutput(t *testing.T) {
	planted := "| Row | Command | Exit code | Observed |\n|---|---|---|---|\n" +
		"| 5 | `x` | exit 0 | **passed** — prints the could-not-check banner once |\n\n**VERIFY: PASS**\n"
	runHeldLayerCases(t, []heldScanCase{
		{"the #4294 row, latest entry", annObs("0", annQuote), false},
		{"the #4294 row, superseded entry", annEntry("Verification — 2026-10-07", "**VERIFY: PASS**",
			annRow("1", "0", "ok"), annRow("3 (Verify 3, negative)", "0", annQuote)) + annLater, false},
		{"ok lead, its section", annObs("exit 0", "ok — prints its could-not-check section with both rows"), false},
		{"planted second instance", planted, false},
	})
}

// TestHeldScopeObservedOutputRefuses: the observed-output reading stays
// narrow. Every case records a hold, or a shape the scan cannot tell from one.
// regression: #4294
func TestHeldScopeObservedOutputRefuses(t *testing.T) {
	noExit := "| # | Command | Result |\n|---|---|---|\n| 3 | `x` | " + annQuote + " |\n\n**VERIFY: PASS**\n"
	note := "| # | Command | Exit | Result | Note |\n|---|---|---|---|---|\n" +
		"| 3 | `x` | 0 | PASS | passed — prints a could-not-check mission section |\n\n**VERIFY: PASS**\n"
	misaligned := "| # | Command | Exit | Result |\n|---|---|---|---|\n" +
		"| 3 | a | b | 0 | " + annQuote + " |\n\n**VERIFY: PASS**\n"
	runHeldLayerCases(t, []heldScanCase{
		{"exit non-zero", annObs("1", annQuote), true},
		{"exit placeholder", annObs("—", annQuote), true},
		{"exit empty", annObs("", annQuote), true},
		{"explicit HELD in the cell", annObs("0", annQuote+"; row HELD"), true},
		{"BLOCKED in the cell", annObs("0", "PASS: a could-not-check mission section, BLOCKED on runner"), true},
		{"BLOCKED lead", annObs("0", "BLOCKED — a could-not-check mission section"), true},
		{"FAIL in the cell", annObs("0", "PASS: a could-not-check mission section, then FAIL on re-run"), true},
		{"not run in the cell", annObs("0", "PASS: a could-not-check mission section; row 4 not run"), true},
		{"skipped in the cell", annObs("0", annQuote+"; row 4 skipped"), true},
		{"unrun in the cell", annObs("0", annQuote+"; 1 unrun"), true},
		{"deferred in the cell", annObs("0", annQuote+"; deferred"), true},
		{"pending in the cell", annObs("0", annQuote+"; review pending"), true},
		{"awaiting in the cell", annObs("0", annQuote+"; awaiting a runner"), true},
		{"waiting in the cell", annObs("0", annQuote+"; waiting on a runner"), true},
		{"not re-run in the cell", annObs("0", annQuote+"; row 4 not re-run"), true},
		{"no runner in the cell", annObs("0", annQuote+"; no runner for row 4"), true},
		{"no PASS lead", annObs("0", "yields a could-not-check mission section"), true},
		{"lead not a word", annObs("0", "PASSWORD: prints a could-not-check mission section"), true},
		{"not attributive", annObs("0", "PASS: could-not-check, no runner"), true},
		{"no article", annObs("0", "PASS: prints then could-not-check mission section"), true},
		{"disposition noun", annObs("0", "PASS: a could-not-check result"), true},
		{"row noun", annObs("0", "PASS: the could-not-check row"), true},
		{"marker ends the cell", annObs("0", "PASS: a could-not-check"), true},
		{"hold reason after", annObs("0", "PASS: a could-not-check because the runner was offline"), true},
		{"HELD attributive", annObs("0", "PASS: prints a HELD mission section"), true},
		{"struck text on the row", annObs("0", "PASS ~~old~~: prints a could-not-check mission section"), true},
		{"no Exit column", noExit, true},
		{"note column", note, true},
		{"misaligned row", misaligned, true},
		{"live held row", annObs("—", "HELD — no runner online"), true},
		{"prose line, not a cell", annObs("0", "ok") + "PASS: a could-not-check mission section\n", true},
	})
}

// TestHeldScopeZeroRatio: a summary that counts every row passed and zero
// could-not-check ("Rows: 2 of 2 required rows passed; 0 could-not-check.")
// is a count, not a disposition; a ratio that did not pass every row, or a
// zero count that gives a hold reason, still refuses.
// regression: #4294
func TestHeldScopeZeroRatio(t *testing.T) {
	ev := func(line string) string { return annObs("0", "ok") + line + "\n" }
	runHeldLayerCases(t, []heldScanCase{
		{"the comment's line", ev("Rows: 2 of 2 required rows passed; 0 could-not-check."), false},
		{"on the verdict line", ev("**VERIFY: PASS** — 7 of 7 rows passed, 0 HELD"), false},
		{"checks ran", ev("3 of 3 checks ran; 0 could-not-check"), false},
		{"ratio short", ev("Rows: 1 of 2 required rows passed; 0 could-not-check."), true},
		{"zero ratio", ev("Rows: 0 of 0 rows passed; 0 could-not-check."), true},
		{"reason after", ev("Rows: 2 of 2 rows passed; 0 could-not-check pending runner"), true},
		{"two qualifiers", ev("Rows: 2 of 2 required verify rows passed; 0 could-not-check."), true},
		{"not a count noun", ev("Rows: 2 of 2 required exits passed; 0 could-not-check."), true},
	})
}

// TestHeldScopeCoverKeyDirection: a bare earlier row is covered only by a
// later row under the same bare key. A later row whose key carries an
// annotation is a different key and covers nothing.
// regression: #4294
func TestHeldScopeCoverKeyDirection(t *testing.T) {
	lastKeyed := func(key string) string {
		return annEntry("Verification — 2026-10-20", "**VERIFY: PASS**",
			annRow("1", "0", "ok"), annRow("2", "0", "ok"), annRow(key, "0", "PASS"))
	}
	runHeldLayerCases(t, []heldScanCase{
		{"later key annotated with a word", annHeld("3") + lastKeyed("3 (negative)"), true},
		{"later key annotated with several words", annHeld("3") + lastKeyed("3 (positive not run)"), true},
		{"later key annotated with a hold word", annHeld("3") + lastKeyed("3 (HELD)"), true},
		{"later key annotated with its own number", annHeld("3") + lastKeyed("3 (Verify 3)"), true},
		{"bare later key", annHeld("3") + lastKeyed("3"), false},
		{"earlier annotated, later bare", annHeld("3 (Verify 3, negative)") + lastKeyed("3"), false},
	})
}

// TestHeldScopeObservedOutputClosed: the observed-output reading needs a
// positive match. A Result cell that opens on a pass but says anything else
// about the marker is read as before.
// regression: #4294
func TestHeldScopeObservedOutputClosed(t *testing.T) {
	runHeldLayerCases(t, []heldScanCase{
		{"remainder awaits", annObs("0", "PASS (rows 1-2) — the could-not-check remainder awaits a runner"), true},
		{"portion still open", annObs("0", "ok — the could-not-check portion is still open"), true},
		{"half needs a runner", annObs("0", "PASS on positive; the could-not-check negative half needs a runner"), true},
		{"offline wording", annObs("0", "ok — the could-not-check section is offline"), true},
		{"unavailable wording", annObs("0", "PASS: prints its could-not-check section, runner unavailable"), true},
		{"not exercised wording", annObs("0", "PASS: prints a could-not-check section; row 4 not exercised"), true},
		{"unknown word after the noun", annObs("0", "PASS: prints a could-not-check section quarantined"), true},
		{"no output verb", annObs("0", "PASS: a lint PROBLEM and a could-not-check mission section"), true},
		{"verb in an earlier clause", annObs("0", "PASS: prints the banner; a could-not-check mission section"), true},
		{"printed section", annObs("0", "PASS: prints its could-not-check mission section"), false},
		{"printed section, quoted lead", annObs("0", annQuote), false},
		{"no known noun after the marker", annObs("0", "PASS: prints a could-not-check with no content"), true},
	})
	// A hold word in an earlier clause of the cell is read even when the
	// clause that holds the marker is well formed.
	for _, hold := range []string{"BLOCKED on a runner", "row 4 failed", "row 4 skipped", "1 unrun", "row 4 deferred",
		"review pending", "awaiting a runner", "row 4 awaits a runner", "waiting on a runner", "row 4 needs a runner",
		"a runner is needed", "row 4 still to do", "row 4 is open", "row 4 remains", "row 4 outstanding",
		"incomplete run", "partial run", "runner offline", "runner unavailable", "runner unreachable",
		"row 4 unverified", "row 4 not yet run", "row 4 not exercised", "row 4 not executed", "row 4 not available",
		"row 4 not checked", "row 4 not re-run", "row 4 not run", "no runner for row 4", "cannot reach the runner",
		"can't reach the runner", "unable to reach the runner", "row 4 todo"} {
		runHeldLayerCases(t, []heldScanCase{
			{"earlier clause: " + hold, annObs("0", "PASS: "+hold+"; prints a could-not-check mission section"), true},
		})
	}
}
