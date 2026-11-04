package main

import (
	"strings"
	"testing"
)

// Regression tests for the maintainer's scope ruling on #4294: verifyflip's
// held / could-not-check scan and its run boundary read only the latest PASS
// run, so an older, superseded run cannot block the flip. Older Evidence is
// never amended to get there. These tests use only identifiers the code had
// before the fix, so they run red on it unchanged (the PR's Fail-first).

// vlOldSHA is the sha of the older, superseded run in these fixtures.
const vlOldSHA = "0123456789ab"

// vlOld returns the fixture rows re-dated to an older run at vlOldSHA.
func vlOld(rows ...string) string {
	t := witnessTableFor(rows...)
	t = strings.ReplaceAll(t, vfSHA, vlOldSHA)
	return strings.ReplaceAll(t, "2026-08-13", "2026-08-10")
}

// vlVerify is a Verify table whose rows both expect exit 0, so the latest
// run's results rows read clean to the cover (an exit=1 row does not).
const vlVerify = "| # | Command | Expect |\n|---|---------|--------|\n" +
	"| 1 | `true` | exit 0 |\n| 2 | `true` | exit 0 |\n"

// vlRow2 is row 2 of vlVerify, passing at the fixture's run.
const vlRow2 = "| 2 | `true` | pass exit=0 | sha256:aa11bb22cc33 | 2026-08-13 | " + vfRunner + " @ " + vfSHA + " |"

// vlOpts is vfDefaults over vlVerify.
func vlOpts() vfOpts {
	o := vfDefaults()
	o.verify = vlVerify
	return o
}

// vlLatest is the latest run as its own entry: both rows re-run clean in a
// dated witness table under an entry heading, closed by the strict PASS.
const vlLatest = "### Non-implementer re-run — 2026-08-13\n\n"

func vlLatestRun() string {
	return vlLatest + witnessTableFor(vfRow1, vlRow2) + "\n\n" + vfPass
}

// vlFlips asserts that verifyflip, run dry, admits the fixture's PASS.
func vlFlips(t *testing.T, o vfOpts) {
	t.Helper()
	root, _ := vfFixture(t, o)
	if code, out, errOut := runVF(t, root, "--dry-run"); code != verifyflipExitOK {
		t.Fatalf("exit = %d, want 0; out=%q err=%q", code, out, errOut)
	}
}

// vlHeldRow is an older run's results row that reads could-not-check: the
// first shape #4294 reports.
var vlHeldRow = "| 2 | `true` | could-not-check exit=-1 | sha256:aa11bb22cc33 | 2026-08-13 | " + vfRunner + " @ " + vfSHA + " |"

// vlOldHeldRow is vlHeldRow as older entries often wrote it: an unescaped
// "|" in the command cell splits the row, so no row key can be read from it
// and the row cover (rule 3a) cannot match it to the later re-run.
var vlOldHeldRow = "| 2 | `true | cat` | could-not-check exit=-1 | sha256:aa11bb22cc33 | 2026-08-13 | " + vfRunner + " @ " + vfSHA + " |"

// vlProbe is a side table an older entry may carry beside its results: its
// rows are keyed by a probe, not by a Verify row, so no later run re-runs
// them and the whole-entry cover (rule 3b) cannot discharge that entry.
const vlProbe = "| Probe | Observed | Date | Runner |\n|-------|----------|------|--------|\n" +
	"| row 2 at the parent commit | exit 0 | 2026-08-10 | " + vfRunner + " |"

// vlBlockedBullet is an older run's verdict bullet: the second shape.
const vlBlockedBullet = "- BLOCKED. Rows 1-6 and 8 pass; row 7 could-not-check (rc=2)."

// TestVerifyflipSupersededHeldRow: an older entry whose results row reads
// could-not-check, then a later entry that re-ran every row clean with Date
// and Runner and closes on the strict PASS, flips.
// regression: #4294
func TestVerifyflipSupersededHeldRow(t *testing.T) {
	o := vlOpts()
	o.evidence = "### Non-implementer run — 2026-08-10\n\n" + vlOld(vfRow1, vlOldHeldRow) +
		"\n\n**VERIFY: BLOCKED** — row 2 could not run.\n\n" + vlLatestRun()
	vlFlips(t, o)
}

// TestVerifyflipSupersededBlockedBullet: an older BLOCKED entry's verdict
// bullet, then a later full PASS re-run, flips.
// regression: #4294
func TestVerifyflipSupersededBlockedBullet(t *testing.T) {
	o := vlOpts()
	o.evidence = "### Non-implementer run — 2026-08-10\n\n" + vlOld(vfRow1, vlRow2) +
		"\n\n" + vlProbe + "\n\n" + vlBlockedBullet + "\n\nVERIFY: BLOCKED\n\n" + vlLatestRun()
	vlFlips(t, o)
}

// TestVerifyflipVerdictFirstTail: an older entry that records its verdict at
// its top and its rows below it leaves those rows between that verdict and
// the latest PASS. They are another run's (another date and sha), so the run
// is read from the latest entry heading on, and the PASS flips.
// regression: #4294
func TestVerifyflipVerdictFirstTail(t *testing.T) {
	o := vlOpts()
	o.evidence = "### Non-implementer run — VERIFY: BLOCKED — 2026-08-10\n\n" + vlOld(vfRow1, vlRow2) +
		"\n\n" + vlLatestRun()
	vlFlips(t, o)
}

// TestVerifyflipLatestHoldRefuses: the same text inside the latest run still
// refuses, in either shape.
// regression: #4294
func TestVerifyflipLatestHoldRefuses(t *testing.T) {
	for name, ev := range map[string]string{
		"held row": vlLatest + witnessTableFor(vfRow1, vlHeldRow) + "\n\n" + vfPass,
		"blocked bullet": vlLatest + witnessTableFor(vfRow1, vlRow2) + "\n\n" + vlBlockedBullet +
			"\n\n" + vfPass,
	} {
		t.Run(name, func(t *testing.T) {
			o := vlOpts()
			o.evidence = "### Non-implementer run — 2026-08-10\n\n" + vlOld(vfRow1, vlRow2) +
				"\n\n**VERIFY: PASS**\n\n" + ev
			vfRefuses(t, o, "could-not-check")
		})
	}
}

// vlOldHeld is an older BLOCKED entry whose row 2 reads could-not-check in a
// row the row cover cannot key, so only the latest-run reading can discharge it.
func vlOldHeld() string {
	return "### Non-implementer run — 2026-08-10\n\n" + vlOld(vfRow1, vlOldHeldRow) +
		"\n\n**VERIFY: BLOCKED** — row 2 could not run.\n\n"
}

// TestVerifyflipLatestNotStrictRefuses: a latest run whose verdict is not a
// strict bold VERIFY: PASS still refuses, older hold or not.
// regression: #4294
func TestVerifyflipLatestNotStrictRefuses(t *testing.T) {
	o := vlOpts()
	o.evidence = vlOldHeld() + vlLatest + witnessTableFor(vfRow1, vlRow2) + "\n\nVERIFY: PASS. Both rows green."
	vfRefuses(t, o, "non-strict PASS")
}

// TestVerifyflipLatestUndatedRefuses: a latest run with a row missing its
// Date cell is not a PASS run: the run reader refuses it (the held scan's
// side of the same rule is TestHeldLatestScope's "undated" case).
// regression: #4294
func TestVerifyflipLatestUndatedRefuses(t *testing.T) {
	o := vlOpts()
	o.evidence = vlOldHeld() + vlLatest + witnessTableFor(vfRow1, strings.Replace(vlRow2, "| 2026-08-13 |", "|  |", 1)) +
		"\n\n" + vfPass
	vfRefuses(t, o, "no Date or Runner")
}

// TestVerifyflipLatestFailedRowRefuses: a latest run with a failed row reads
// no clean outcome, so the older hold stays read and refuses.
// regression: #4294
func TestVerifyflipLatestFailedRowRefuses(t *testing.T) {
	o := vlOpts()
	o.evidence = vlOldHeld() + vlLatest + witnessTableFor(vfRow1, strings.Replace(vlRow2, "pass exit=0", "fail exit=1", 1)) +
		"\n\n" + vfPass
	vfRefuses(t, o, "could-not-check")
}

// TestVerifyflipLatestRunSkippedRow: the latest run re-ran row 1 only. The
// older hold on row 2 is no longer read, but row 2 has no passing witness in
// the latest run, so the flip still refuses.
// regression: #4294
func TestVerifyflipLatestRunSkippedRow(t *testing.T) {
	o := vlOpts()
	o.evidence = vlOldHeld() + vlLatest + witnessTableFor(vfRow1) + "\n\n" + vfPass
	vfRefuses(t, o, "passing execution witness")
}

// TestVerifyflipAmbiguousBoundaryRefuses: an element left open before the
// latest heading (one the page may carry across it) makes the boundary
// uncertain, so neither the held scan nor the run reader narrows to it and
// the original refusal stands.
// regression: #4294
func TestVerifyflipAmbiguousBoundaryRefuses(t *testing.T) {
	for _, open := range []string{"<details>", "<s>"} {
		t.Run("held "+open, func(t *testing.T) {
			o := vlOpts()
			o.evidence = vlOldHeld() + open + "\n\n" + vlLatestRun()
			vfRefuses(t, o, "could-not-check")
		})
		t.Run("tail "+open, func(t *testing.T) {
			o := vlOpts()
			o.evidence = "### Non-implementer run — VERIFY: BLOCKED — 2026-08-10\n\n" + vlOld(vfRow1, vlRow2) +
				"\n\n" + open + "\n\n" + vlLatestRun()
			vfRefuses(t, o, "date mismatch")
		})
	}
}
