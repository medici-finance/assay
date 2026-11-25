package main

import (
	"strings"
	"testing"
)

// hc7578Row is a witness row verifyrun wrote into a brief's Evidence: a
// command that kept an unsubstituted placeholder, and a Result cell that
// quotes that placeholder back in prose. "<a verb ...>" reads as an <a>
// start tag, twice.
const hc7578Row = "| 2 | `ASSAY_REPO_ALIASES='not=a=valid=shape' <a verb whose manifest requires assay.roster.ext.repo-aliases>` | " +
	"could-not-run exit=- — unsubstituted placeholder(s) <a verb whose manifest requires assay.roster.ext.repo-aliases> — " +
	"the row cannot run as written | sha256:e3b0c44298fc | 2026-10-30 | assay-verifier-app[bot] @ c887e24dd952 " +
	"(on-behalf-of human:ian) (forge-identity) |"

// hc7578Code is the placeholder inside an inline code span in prose.
const hc7578Code = "Row 2 ran `ASSAY_REPO_ALIASES='not=a=valid=shape' <a verb whose manifest requires assay.roster.ext.repo-aliases>` as written."

// hc7578Carry runs heldCarryElement over text as heldScanScope would.
func hc7578Carry(text string) bool {
	lines := strings.Split(text, "\n")
	inFence := make([]bool, len(lines))
	f := false
	for i, l := range lines {
		if heldFenceToggle(l) {
			f, inFence[i] = !f, true
			continue
		}
		inFence[i] = f
	}
	return heldCarryElement(lines, inFence)
}

// hc7578Entry is an earlier, failed run whose witness table holds rows.
func hc7578Entry(rows ...string) string {
	return "### Non-implementer run — 2026-08-11\n\n" + witnessTableFor(append([]string{vfRow1}, rows...)...) +
		"\n\n**VERIFY: FAIL** — row 2 could not run.\n\n"
}

// TestHeldCarryWitnessRow: the witness row as verifyrun wrote it opens no
// element the page carries past its cell, so a clean latest run still bounds
// the scan (rule 3c) and verifyflip admits it.
// regression: #7578
func TestHeldCarryWitnessRow(t *testing.T) {
	table := "\n" + witnessTableFor(vfRow1, hc7578Row) + "\n"
	if hc7578Carry(table) {
		t.Fatalf("heldCarryElement = true on the verifyrun witness row\n%s", table)
	}
	ev := vlOldHeld() + hc7578Entry(hc7578Row) + vlLatestRun()
	if !heldScanScope(ev, true).latest {
		t.Fatalf("latest = false: the witness row's placeholder still doubts the run boundary\n%s", ev)
	}
	o := vlOpts()
	o.evidence = ev
	vlFlips(t, o)
}

// TestHeldCarryCodeSpan: a carried opener inside a certain inline code span
// opens nothing on the page, in prose or in a table cell written by hand.
// regression: #7578
func TestHeldCarryCodeSpan(t *testing.T) {
	hand := "| # | Command | Result |\n|---|---|---|\n| 2 | `ASSAY_REPO_ALIASES='x' <a verb whose manifest requires y>` | could not run |"
	for _, text := range []string{hc7578Code, hand, "`<s>` and ``<details>``", "`<a href=x>` then `</a>`"} {
		if hc7578Carry(text) {
			t.Errorf("heldCarryElement(%q) = true, want false", text)
		}
	}
	ev := vlOldHeld() + "### Non-implementer run — 2026-08-11\n\n" + hc7578Code + "\n\n**VERIFY: FAIL** — row 2.\n\n" + vlLatestRun()
	if !heldScanScope(ev, true).latest {
		t.Fatalf("latest = false: a placeholder in a code span still doubts the run boundary\n%s", ev)
	}
}

// TestHeldCarryProseStillDoubts: an opener the page can render as HTML still
// counts: in prose, in a span the scan cannot settle, inside an HTML block,
// and in a table that is not exactly verifyrun's or whose cell can escape.
// regression: #7578
func TestHeldCarryProseStillDoubts(t *testing.T) {
	escRow := strings.Replace(vlRow2, "| 2026-08-13 |", "| 2026-08-13 </table><s> |", 1)
	tplRow := strings.Replace(vlRow2, "| 2026-08-13 |", "| 2026-08-13 <template> |", 1)
	rawRow := strings.Replace(vlRow2, "| 2026-08-13 |", "| 2026-08-13 <textarea> |", 1)
	carryRow := strings.Replace(vlRow2, "| 2026-08-13 |", "| 2026-08-13 <s> |", 1)
	closeRow := strings.Replace(vlRow2, "| 2026-08-13 |", "| 2026-08-13 </s> |", 1)
	// object, applet and marquee push their own active-formatting marker, so
	// closing the cell keeps a formatting element opened before them open.
	markRow := func(tag string) string {
		return strings.Replace(vlRow2, "| 2026-08-13 |", "| 2026-08-13 <s><"+tag+"> |", 1)
	}
	header := strings.SplitN(witnessTableFor(vfRow1), "\n", 2)[0]
	cases := []struct{ name, text string }{
		{"open a href", "See <a href=x>the log"},
		{"open details", "Notes <details><summary>x</summary>"},
		{"placeholder in prose", "could-not-run — <a verb whose manifest requires x> — as written"},
		{"unpaired run before span", "x `` y `<a verb>`"},
		{"span opened on earlier line", "a `open\n`<a verb>` x"},
		{"span in HTML block", "<div>\n`<s>`\n</div>"},
		{"span after a bare CR", "a\rb\n\n`<s>`"},
		{"escape tag in witness row", "\n" + witnessTableFor(vfRow1, escRow)},
		{"template in witness row", "\n" + witnessTableFor(vfRow1, tplRow)},
		{"escape ends exemption", "\n" + witnessTableFor(vfRow1, escRow, carryRow)},
		{"raw tag in witness row", "\n" + witnessTableFor(vfRow1, rawRow)},
		{"witness table after text", "Rows:\n" + witnessTableFor(vfRow1, carryRow)},
		{"hand header", "\n| # | Command | Result | Output | Date |\n|---|---|---|---|---|\n" + carryRow},
		{"row not verifyrun's", "\n" + witnessTableFor(vfRow1, "| 3 | <s> | ran |")},
		{"after the table", "\n" + witnessTableFor(vfRow1) + "\n\n" + carryRow},
		{"header with no delimiter row", "\n" + header + "\n" + vlRow2 + "\n" + carryRow},
		{"fence ends the table", "\n" + witnessTableFor(vfRow1) + "\n```\nx\n```\n" + carryRow},
		{"closer in a witness row", "x <s>\n\n" + witnessTableFor(vfRow1, closeRow)},
		{"object in witness row", "\n" + witnessTableFor(vfRow1, markRow("object"))},
		{"applet in witness row", "\n" + witnessTableFor(vfRow1, markRow("applet"))},
		{"marquee in witness row", "\n" + witnessTableFor(vfRow1, markRow("MARQUEE"))},
	}
	for _, c := range cases {
		if !hc7578Carry(c.text) {
			t.Errorf("%s: heldCarryElement(%q) = false, want true", c.name, c.text)
		}
	}
	for _, c := range []struct{ name, text string }{
		{"open a href", "See <a href=x>the log\n\n"},
		{"open details", "<details>\n\n"},
	} {
		ev := vlOldHeld() + c.text + vlLatestRun()
		if heldScanScope(ev, true).latest {
			t.Errorf("%s: latest = true, want the run boundary doubted\n%s", c.name, ev)
		}
	}
}
