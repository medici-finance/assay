package main

import (
	"strings"
	"testing"
)

// Regression tests for #5541: a passing verifyrun witness whose row expects a
// non-zero exit records its Result cell as "pass exit=N" (N the observed
// code). The held cover read only "pass exit=0" as a clean outcome, so a
// latest PASS run that re-ran such a row covered nothing for it: rules 3a,
// 3b and 3c all stayed off, and an earlier entry's HELD heading refused the
// flip forever. These tests use only identifiers the code had before the
// fix, so they run red on it unchanged.

// nzOldHeld is an earlier BLOCKED entry shaped like the #5541 report: the
// heading itself says a row was held, row 2 recorded fail exit=1 because its
// Expect was unnumbered, and the entry closes BLOCKED.
func nzOldHeld() string {
	failed := strings.Replace(vfRow2, "pass exit=1", "fail exit=1", 1)
	return "### Non-implementer run — 2026-08-10 (row 1 passes, row 2 held on its check definition)\n\n" +
		vlOld(vfRow1, failed) +
		"\n\n**VERIFY: BLOCKED** — row 2 held on its check definition.\n"
}

// nzLatest is the later full re-run: row 2 expects exit 1 (vfVerify) and the
// witness records pass exit=1, then the strict PASS. The heading follows the
// earlier verdict line with no blank line between, as in the report.
func nzLatest(row2 string) string {
	return vlLatest + witnessTableFor(vfRow1, row2) + "\n\n" + vfPass
}

// TestHeldWitnessNonZeroPass: the cover reads verifyrun's passing Result
// cell as clean whatever exit code it observed, and nothing wider.
// regression: #5541
func TestHeldWitnessNonZeroPass(t *testing.T) {
	for cell, want := range map[string]heldCellState{
		"pass exit=0":                         heldCellPass,
		"pass exit=2":                         heldCellPass,
		" pass exit=3 ":                       heldCellPass,
		"pass exit=1 sandbox=unshare":         heldCellPass,
		"pass exit=4 sandbox=container-netns": heldCellPass,
		"fail exit=2":                         heldCellOther,
		"could-not-run exit=-":                heldCellOther,
		"pass exit=-":                         heldCellOther,
		"PASS exit=2":                         heldCellOther,
		"passed exit=2":                       heldCellOther,
		"pass exit=2 (no re-run)":             heldCellOther,
		"pass exit=2 sandbox=other":           heldCellOther,
		"pass exit=2, HELD":                   heldCellOther,
	} {
		if got := heldOutcome(cell, false); got != want {
			t.Errorf("heldOutcome(%q) = %v, want %v", cell, got, want)
		}
	}
	// In an Exit column the cell is not verifyrun's Result cell.
	if got := heldOutcome("pass exit=2", true); got != heldCellOther {
		t.Errorf("heldOutcome(exit column) = %v, want other", got)
	}
}

// TestVerifyflipNonZeroSuperseded: the #5541 shape flips (rule 3c).
// regression: #5541
func TestVerifyflipNonZeroSuperseded(t *testing.T) {
	o := vfDefaults()
	o.evidence = nzOldHeld() + nzLatest(vfRow2)
	vlFlips(t, o)
}

// TestHeldNonZeroAllConsumers: the same Evidence is admitted by the readers
// that do not read rule 3c (close-verify, the card and autoflip's pre-check),
// through rules 3a and 3b, because the latest run now covers row 2.
// regression: #5541
func TestHeldNonZeroAllConsumers(t *testing.T) {
	ev := nzOldHeld() + nzLatest(vfRow2)
	if held, why := verifyPassHeldContradiction(ev); held {
		t.Errorf("verifyPassHeldContradiction held: %q", why)
	}
	for _, st := range []string{"implemented", "verified"} {
		if err := closeVerifyHeldRefusal("vf/01", st, ev); err != nil {
			t.Errorf("closeVerifyHeldRefusal(%s) = %v, want nil", st, err)
		}
	}
}

// TestHeldNonZeroFailStillRefuses: the control. A latest run whose row 2
// records fail exit=1 covers nothing, so the earlier hold still refuses on
// every reader.
// regression: #5541
func TestHeldNonZeroFailStillRefuses(t *testing.T) {
	failed := strings.Replace(vfRow2, "pass exit=1", "fail exit=1", 1)
	ev := nzOldHeld() + nzLatest(failed)
	if err := flipHeldRefusal("vf/01", "implemented", ev); err == nil {
		t.Error("flipHeldRefusal admitted a latest run whose row 2 failed")
	}
	if err := closeVerifyHeldRefusal("vf/01", "implemented", ev); err == nil {
		t.Error("closeVerifyHeldRefusal admitted a latest run whose row 2 failed")
	}
}
