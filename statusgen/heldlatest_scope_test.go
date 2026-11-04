package main

import (
	"strings"
	"testing"
)

// TestHeldLatestScope pins rule 3c's own conditions on the held scan: the
// scan reads only the last entry exactly when that entry is a strict PASS
// run, dated, clean, and bounded by a heading the page surely draws.
// regression: #4294
func TestHeldLatestScope(t *testing.T) {
	undated := strings.Replace(vlRow2, "| 2026-08-13 |", "|  |", 1)
	failed := strings.Replace(vlRow2, "pass exit=0", "fail exit=1", 1)
	cases := []struct {
		name string
		ev   string
		want bool
	}{
		{"clean latest run", vlOldHeld() + vlLatestRun(), true},
		{"single entry", vlLatestRun(), false},
		{"not strict", vlOldHeld() + vlLatest + witnessTableFor(vfRow1, vlRow2) + "\n\nVERIFY: PASS. green.", false},
		{"blocked", vlOldHeld() + vlLatest + witnessTableFor(vfRow1, vlRow2) + "\n\n**VERIFY: BLOCKED**", false},
		{"verdict after pass", vlOldHeld() + vlLatestRun() + "\n\n**VERIFY: FAIL**", false},
		{"undated", vlOldHeld() + vlLatest + witnessTableFor(vfRow1, undated) + "\n\n" + vfPass, false},
		{"no witness table", vlOldHeld() + vlLatest + "Both rows re-ran.\n\n" + vfPass, false},
		{"failed row", vlOldHeld() + vlLatest + witnessTableFor(vfRow1, failed) + "\n\n" + vfPass, false},
		{"held key", vlOldHeld() + vlLatest + witnessTableFor(vfRow1, strings.Replace(vlRow2, "| 2 |", "| 2 (HELD) |", 1)) + "\n\n" + vfPass, false},
		{"row run-on", vlOldHeld() + vlLatest + witnessTableFor(vfRow1, vlRow2) + "\nre-ran at the parent\n\n" + vfPass, false},
		{"open attribute", vlOldHeld() + "<span title=\"a\n\n" + vlLatestRun(), false},
		{"open details", vlOldHeld() + "<details>\n\n" + vlLatestRun(), false},
		{"open strike", vlOldHeld() + "<s>\n\n" + vlLatestRun(), false},
		{"closed strike", vlOldHeld() + "<s>old</s>\n\n" + vlLatestRun(), true},
		{"open comment", vlOldHeld() + "<!-- note\n\n" + vlLatestRun(), false},
		{"fence in quote", vlOldHeld() + "> ```\n\n" + vlLatestRun(), false},
		{"same-run BLOCKED heading", "### Non-implementer run — VERIFY: BLOCKED — 2026-08-13\n\n" +
			witnessTableFor(vfRow1, vlRow2) + "\n\n" + vlBlockedBullet + "\n\n" + vlLatest + witnessTableFor(vfRow1) + "\n\n" + vfPass, false},
		// The BLOCKED line bounds the run verifyflip credits, so 3c may stop
		// at the heading; the flip then refuses on row 2's missing witness
		// (TestVerifyflipSameRunBlockedRefuses).
		{"same-run BLOCKED line", "### Non-implementer run — 2026-08-13\n\n" + witnessTableFor(vfRow1, vlRow2) + "\n\n" +
			vlBlockedBullet + "\n\n**VERIFY: BLOCKED** — env.\n\n" + vlLatest + witnessTableFor(vfRow1) + "\n\n" + vfPass, true},
		{"split run", "### Non-implementer run part 1 — 2026-08-13\n\n" + witnessTableFor(vfRow1, vlRow2) + "\n\n" +
			vlBlockedBullet + "\n\n### Non-implementer run part 2 — 2026-08-13\n\n" + witnessTableFor(vfRow1) + "\n\n" + vfPass, false},
		{"skipped row, own run", vlOldHeld() + vlLatest + witnessTableFor(vfRow1) + "\n\n" + vfPass, true},
		{"unrendered heading", vlOldHeld() + vlLatest + witnessTableFor(vfRow1, vlRow2) + "\n\n<div>\n### Aside\n</div>\n\n" + vfPass, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := heldScanScope(c.ev, true).latest; got != c.want {
				t.Fatalf("latest = %v, want %v\n%s", got, c.want, c.ev)
			}
			if heldScanScope(c.ev, false).latest {
				t.Fatal("latest = true on the read close-verify and the card use, which never reads rule 3c")
			}
		})
	}
	// With the witness gate held true, every other condition must still
	// decide each case on its own, so none of them is masked by the gate. The
	// cases the gate alone refuses must then admit: the gate is what holds them.
	gateOnly := map[string]bool{"same-run BLOCKED heading": true, "split run": true}
	t.Run("witness gate held true", func(t *testing.T) {
		defer func(f func(string) bool) { heldWitnessInLatest = f }(heldWitnessInLatest)
		heldWitnessInLatest = func(string) bool { return true }
		for _, c := range cases {
			want := c.want || gateOnly[c.name]
			if got := heldScanScope(c.ev, true).latest; got != want {
				t.Errorf("%s: latest = %v, want %v", c.name, got, want)
			}
		}
	})
}

// TestHeldCarryElement: an element the page carries across blocks, left
// open, is boundary doubt; a closed one, or one inside fenced code, is not.
// regression: #4294
func TestHeldCarryElement(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{"<s>", true},
		{"<strike>old", true},
		{"<details><summary>x</summary>", true},
		{"<template>", true},
		{"<s>old</s>", false},
		{"<details>\n\n</details>", false},
		{"plain `<name>` placeholder", false},
		{"```\n<s>\n```", false},
	}
	for _, c := range cases {
		lines := strings.Split(c.text, "\n")
		inFence := make([]bool, len(lines))
		f := false
		for i, l := range lines {
			if heldFenceToggle(l) {
				f, inFence[i] = !f, true
				continue
			}
			inFence[i] = f
		}
		if got := heldCarryElement(lines, inFence); got != c.want {
			t.Errorf("heldCarryElement(%q) = %v, want %v", c.text, got, c.want)
		}
	}
}

// TestHeldContainerFence: a fence opened inside a blockquote or list item
// may end where the scan does not; a fence at column 0 is not doubt.
// regression: #4294
func TestHeldContainerFence(t *testing.T) {
	for text, want := range map[string]bool{
		"> ```":          true,
		"- ~~~":          true,
		"```\ncode\n```": false,
		"> quoted prose": false,
	} {
		if got := heldContainerFence(strings.Split(text, "\n")); got != want {
			t.Errorf("heldContainerFence(%q) = %v, want %v", text, got, want)
		}
	}
}
