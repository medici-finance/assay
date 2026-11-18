package main

import (
	"strings"
	"testing"
)

// Tests for two render-divergence shapes in the held scan (heldscope.go):
//
//   - An emphasis delimiter between a digit and a double quote. The page
//     renders **5**" as a bold 5 and an inch mark, but the source shows the
//     quote beside "*", so rule 7's inch-mark check (a quote right after a
//     letter or digit) never sees the digit and the flip read pairs the two
//     marks as a quoted span around a hold.
//   - A HELD or could-not-check marker that an HTML comment splits
//     ("HE<!-- -->LD"). The page renders the marker whole; the scan reads the
//     line with the comment in place and sees only its pieces. Every read
//     shares one rule: the Evidence is stripped of complete comments until
//     it stops changing (heldCommentJoins), and a marker a strip forms is a
//     hold on the line it lands on, wherever that line sits, with no
//     negation, routing or quoting excuse.
//
// heldReadsAll runs one case at every read: the verify-gate card and the
// model autoflip's pre-check (verifyPassHeldContradiction), the autoflip's
// witnessed read (verifyPassHeldWitnessed; with no brief path it keeps the
// plain read's answer), close-verify (closeVerifyHeldRefusal) and the flip
// read (flipHeldRefusal, verifyflip and the reconcile fold).
func heldReadsAll(t *testing.T, cases []heldScanCase) {
	t.Helper()
	runHeldScanCasesFlip(t, cases, nil)
	for _, tc := range cases {
		t.Run(tc.name+"/autoflip", func(t *testing.T) {
			if held, why := verifyPassHeldWitnessed(tc.evidence, ""); held != tc.wantHeld {
				t.Errorf("verifyPassHeldWitnessed held=%v, want %v (why=%q)", held, tc.wantHeld, why)
			}
		})
	}
}

// TestHeldEmphasisInch: an emphasis delimiter ("*", "**" or "_") between a
// digit and a double quote renders as nothing, so the quote is an inch mark
// on the page. Rule 7 gives no span to a quote beside a delimiter, in prose
// and in a Result cell, so every read refuses the hold between the marks.
// regression: #6789
func TestHeldEmphasisInch(t *testing.T) {
	prose := func(d string) string {
		return heldEv("ok") + "\nMeasured a " + d + "3" + d + "\" gap; row 3 is HELD pending the " + d + "5" + d + "\" retest.\n"
	}
	cell := func(d string) string {
		return heldEv(d + "3" + d + "\" gap; row 3 could-not-check, " + d + "5" + d + "\" retest")
	}
	var cases []heldScanCase
	for _, d := range []string{"*", "**", "_"} {
		cases = append(cases,
			heldScanCase{"prose " + d, prose(d), true},
			heldScanCase{"result cell " + d, cell(d), true})
	}
	// The delimiter after the closing mark: "...retest"_ is not a span.
	cases = append(cases, heldScanCase{"delimiter after the quote",
		heldEv("ok") + "\nRow 2 printed \"row 3 is HELD for audit\"_ on rerun_.\n", true})
	heldReadsAll(t, cases)

	// Control: the same quoted output with no delimiter beside a mark is
	// still a span, so only the flip read admits it.
	runHeldScanCasesFlip(t, []heldScanCase{
		{"no delimiter", heldEv("ok") + "\nRow 2 printed \"row 3 is HELD for audit\" on **rerun**.\n", true},
	}, map[string]bool{"no delimiter": true})
}

// TestHeldCommentSplit: a marker an HTML comment splits renders whole, so a
// results row, a verifyrun witness row and a prose line holding one refuse
// at every read, the flip read included: a comment inside the word, one that
// spans two lines, and one only a second strip removes. A comment that forms
// no marker changes nothing.
// regression: #6789
func TestHeldCommentSplit(t *testing.T) {
	wit := witEntry("Run 1", witTable(2, "", witPass, ""), "", "**VERIFY: PASS** — 2/2 rows")
	row2 := "| 2 | `tool check 2` | pass exit=0 | sha256:0123456789ab |"
	if !strings.Contains(wit, row2) {
		t.Fatalf("fixture: witness row 2 not found")
	}
	witRow := func(result string) string {
		return strings.Replace(wit, row2, "| 2 | `tool check 2` | "+result+" | sha256:0123456789ab |", 1)
	}
	prose := func(line string) string { return heldEv("ok") + "\n" + line + "\n" }
	heldReadsAll(t, []heldScanCase{
		// Results rows (the card, close-verify and autoflip reads).
		{"row, comment in HELD", heldEv("ok — HE<!-- note -->LD pending runner"), true},
		{"row, comment in could-not-check", heldEv("could-<!-- -->not-check, no runner"), true},
		{"row, comment across lines", heldEv("HE<!--\n-->LD pending runner"), true},
		{"row, second strip", heldEv("HE<!<!-- -->-- -->LD pending runner"), true},
		{"row, short comment form", heldEv("HE<!-->LD pending runner"), true},
		// A verifyrun witness row.
		{"witness row, comment in HELD", witRow("HE<!-- -->LD exit=0"), true},
		// Prose lines, the flip side included.
		{"prose, comment in HELD", prose("Row 3 is HE<!-- x -->LD: no runner online."), true},
		{"prose, comment in could-not-check", prose("Row 3: could-not-<!-- -->check, no runner."), true},
		{"prose, negated split marker", prose("No row is HE<!-- -->LD."), true},
		{"prose, second strip", prose("Row 3 is HE<!<!-- -->-- -->LD."), true},
		// Controls: no strip forms a marker.
		{"comment beside a word", heldEv("ok <!-- reviewer note -->"), false},
		{"comment between words", prose("Row 3 ran <!-- x --> clean."), false},
		{"whole marker after a comment", prose("Row 3 ran <!-- x --> clean.\n\nThe run had no could-not-check rows."), false},
		// A marker inside a comment still refuses, as before.
		{"marker inside a comment", heldEv("ok <!-- was HELD, re-ran clean -->"), true},
	})
}

// TestHeldCommentUnsettled: Evidence whose strip still changes after
// heldRestripCap strips, with a marker the last strip forms, refuses at every
// read: the join cannot be mapped to a line, so the read fails closed.
// regression: #6789
func TestHeldCommentUnsettled(t *testing.T) {
	nest := "HE<!-- -->LD"
	for i := 0; i < 12; i++ {
		nest = strings.Replace(nest, "<!--", "<!<!-- -->--", 1)
	}
	heldReadsAll(t, []heldScanCase{
		{"nested past the cap", heldEv("ok") + "\nRow 3 is " + nest + ".\n", true},
	})
}
