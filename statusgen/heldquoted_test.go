package main

import (
	"strings"
	"testing"
)

// Tests for the flip read's double-quoted output rule (heldscope.go, rule 7).
//
// A verifier records what a row's command printed — a test's failure message
// under a fail-first mutation, a diagnostic — in double quotes in prose or in
// a Result cell. The marker words inside such a span are the product's
// output, not the row's disposition. The flip read (flipHeldRefusal:
// verifyflip and the reconcile fold, both of which then require a passing
// execution witness for every Verify row) skips an occurrence inside one. The
// verify-gate card and close-verify, which run no witness check, read it as
// before, so every admitted case below is named in flipAdmits and still
// refuses there.

// quotedFailFirst is a fail-first note of the shape seen in the field: a row
// named in prose, the mutated test's own failure message quoted.
const quotedFailFirst = "Fail-first: with the guard disabled, row 2's command exits 1 with " +
	"`--- FAIL: TestBucketUnreadable (0.00s)` and \"brief file unreadable: bucket = none, want could-not-check\"."

// TestHeldQuotedFlipAdmits: an occurrence inside a double-quoted span is
// admitted by the flip read only; a genuine hold beside it still refuses.
// regression: #4721
func TestHeldQuotedFlipAdmits(t *testing.T) {
	admitted := map[string]bool{
		"quoted fail-first message":    true,
		"quoted output in result cell": true,
		"second quoted span":           true,
		"capitalised quoted marker":    true,
	}
	runHeldScanCasesFlip(t, []heldScanCase{
		// Admitted by the flip read.
		{"quoted fail-first message", heldEv("ok") + "\n" + quotedFailFirst + "\n", true},
		{"quoted output in result cell",
			heldEv(`ok — probe printed "tenant b: could-not-check, no grant"`), true},
		{"second quoted span",
			heldEv("ok") + "\nRow 2 printed \"ok\" and then \"HELD for audit: record 7\".\n", true},
		{"capitalised quoted marker",
			heldEv("ok") + "\nRow 2 base run prints \"COULD-NOT-CHECK: delivery not an ancestor\".\n", true},

		// Controls: refused by every read, the flip read included.
		{"genuinely held result cell", heldEv("could-not-check — no runner online"), true},
		{"marker outside the quotes",
			heldEv("ok") + "\nRow 2 printed \"ok\" but is HELD: no runner.\n", true},
		{"quote closes before marker",
			heldEv("ok") + "\nRow 2 printed \"ok\" then could-not-check.\n", true},
		{"bare marker in quotes", heldEv("ok") + "\nRow 2 is \"HELD\".\n", true},
		{"bare marker, punctuation", heldEv("ok") + "\nRow 2: \"**could-not-check:**\"\n", true},
		{"unpaired quote", heldEv("ok") + "\nRow 2 printed \"see log, could-not-check\n", true},
		// Three quotes, the marker inside the first, pairable, span: only
		// the odd-count guard refuses it.
		{"odd quote count",
			heldEv("ok") + "\nRow 2 printed \"tenant b: could-not-check\" then \"more.\n", true},
		{"escaped quote in the span",
			heldEv("ok") + "\nRow 2 printed \"a \\\" could-not-check b\".\n", true},
		// Even counts with an escaped quote: only the escaped-quote guard
		// refuses them.
		{"escaped quotes, even count",
			heldEv("ok") + "\nRow 2 printed \\\"tenant b: could-not-check\\\".\n", true},
		{"escaped pair inside even span",
			heldEv("ok") + "\nRow 2 printed \"a \\\" b \\\" could-not-check, no grant\".\n", true},
		// The quotes pair across the paragraph, not per line: HELD renders
		// between two spans, each of which crosses a line.
		{"span crossing a line",
			heldEv("ok") + "\nRow 2 printed \"first part\nsecond part\" and row 3 is HELD pending \"third part\nfourth\".\n", true},
		// The same with every quote flanked as a quotation mark, so only the
		// paragraph check refuses it.
		{"flanked span crossing a line",
			heldEv("ok") + "\nRow 2 printed \"first part\nsecond part.\" Row 3 is HELD pending \"(third part\nfourth\".\n", true},
		// A pipe on a prose line splits nothing when rendered.
		{"pipe on a prose line",
			heldEv("ok") + "\nRow 2 printed \"a | b\" and row 3 is HELD pending \" end.\n", true},
		// The same, flanked, so only the whole-line reading refuses it.
		{"flanked pipe on a prose line",
			heldEv("ok") + "\nRow 2 printed \"a | b .\" and row 3 is HELD pending \"(end.\n", true},
		// Inch marks are not quotation marks (flanking).
		{"inch marks",
			heldEv("ok") + "\nMeasured a 3\" gap; row 3 HELD pending the 5\" retest.\n", true},
		// A genuine hold in a result cell that also quotes output.
		{"held cell beside quoted output",
			heldEv(`could-not-check — runner printed "ok"`), true},
		{"span would cross a cell",
			heldEv(`ok "x | could-not-check | y"`), true},
		{"curly quotes not read",
			heldEv("ok") + "\nRow 2 printed “HELD for audit: record 7”.\n", true},
		{"link definition title",
			heldEv("ok") + "\n[x]: /u \"row 2 HELD for audit\"\n", true},
	}, admitted)
}

// TestHeldQuotedRealEvidence: the committed graph-execution/02 Evidence, whose
// only live occurrence is the last entry's base-tree diagnostic in double
// quotes (b02Quote), flips; close-verify and the card still refuse it. The
// same Evidence with a genuine hold appended refuses on every read.
// regression: #4721
func TestHeldQuotedRealEvidence(t *testing.T) {
	ev := b02Evidence(t)
	runHeldLayerCasesFlip(t, []heldScanCase{
		{"real Evidence as committed", ev, true},
		{"real Evidence, hold appended",
			ev + "\n### Non-implementer verifier re-run — 2026-10-26\n\n- Row 3: could-not-check, no Linux runner online.\n\n**VERIFY: BLOCKED**\n", true},
	}, map[string]bool{"real Evidence as committed": true})
}

// TestHeldQuotedRenderDiverge: quote characters the source pairs around a
// hold that a renderer does not show as a quotation give no span, so every
// read refuses: quotes inside HTML comments, a character reference beside
// ASCII quotes, spaced ditto or inch marks, and inch marks flanked by a
// no-break space. Each shape is refused by exactly one of the guards.
// regression: #4721
func TestHeldQuotedRenderDiverge(t *testing.T) {
	runHeldScanCasesFlip(t, []heldScanCase{
		{"quotes in HTML comments",
			heldEv("ok") + "\nRow 2 printed <!--\"--> row 3 is HELD <!--\"--> as noted.\n", true},
		{"quote entity beside quotes",
			heldEv("ok") + "\nRow 2 printed &quot;a .\" row 3 is HELD \". b&quot; on rerun.\n", true},
		{"spaced inch marks",
			heldEv("ok") + "\nMeasured a 3 \" gap; row 3 is HELD pending the 5 \" retest.\n", true},
		{"NBSP-flanked inch marks",
			heldEv("ok") + "\nMeasured a 3\u00a0\"\u00a0gap; row 3 is HELD pending the 5\u00a0\"\u00a0retest.\n", true},
	}, nil)
}

// TestHeldQuotedGuardPins: each case pairs as a quoted span on its own line
// and is refused only by the guard it names, so loosening that guard admits
// it: a bare carriage return (a CommonMark line ending; doubled, it ends the
// paragraph between the quotes), a struck-through quote (the struck text is
// blanked before the scan, its quote still rendered), and one quoted
// neighbour line in a two-line paragraph (the quotes pair across the lines).
// regression: #4721
func TestHeldQuotedGuardPins(t *testing.T) {
	runHeldScanCasesFlip(t, []heldScanCase{
		{"bare CR between the quotes",
			heldEv("ok") + "\nRow 2 printed \"tenant b\r\rrow 3 is HELD\r\rno grant\".\n", true},
		{"struck quote on the line",
			heldEv("ok") + "\nRow 2 printed ~~\"old~~ new.\" row 3 is HELD \"(done\n", true},
		{"one quoted neighbour line",
			heldEv("ok") + "\nRow 2 printed \"tenant b output\nends here.\" Row 3 is HELD pending \"(retest\n", true},
	}, nil)
}

// TestHeldQuotedWitnessCell: rule 7 is not applied to a verifyrun witness
// row. The flip credits a witness row by the first state word anywhere in its
// Result cell (witnessStateRe), so a quoted hold ahead of "pass exit=0" would
// otherwise be skipped by the held scan and credited as a pass. Every read
// refuses both orders, as before rule 7.
// regression: #4721
func TestHeldQuotedWitnessCell(t *testing.T) {
	row2 := "| `tool check 2` | pass exit=0"
	cell := func(result string) string {
		table := strings.Replace(witTable(2, "", witPass, ""), row2, "| `tool check 2` | "+result, 1)
		if !strings.Contains(table, result) {
			t.Fatalf("fixture: row 2 not rewritten")
		}
		return witEntry("Run 1", table, "", "**VERIFY: PASS** — 2/2 rows")
	}
	runHeldScanCasesFlip(t, []heldScanCase{
		{"quoted hold before the state", cell(`"tenant b: could-not-check, no grant" pass exit=0`), true},
		{"quoted hold after the state", cell(`pass exit=0 "tenant b: could-not-check, no grant"`), true},
	}, nil)
}

// TestHeldQuotedWitnessStripped: the witness check reads Evidence rows only
// after complete HTML comments are removed (parseEvidenceRows), so whether
// rule 7 may skip a line is decided on that same stripped text. Each case
// hides row 2's witness shape from its source line with an invisible
// comment, while the stripped row is a witness the check credits as a pass:
// a comment inside the digest, the same with a HELD marker, a comment
// carrying a pipe in the header and every Command cell, a comment that
// joins two source lines into one row, and a comment that only the second
// strip on the flip path removes. Every read refuses each one.
// regression: #4721
func TestHeldQuotedWitnessStripped(t *testing.T) {
	base := witEntry("Run 1", witTable(2, "", witPass, ""), "", "**VERIFY: PASS** — 2/2 rows")
	row2 := "| 2 | `tool check 2` | pass exit=0 | sha256:0123456789ab |"
	edit := func(s string, pairs ...string) string {
		for k := 0; k < len(pairs); k += 2 {
			if !strings.Contains(s, pairs[k]) {
				t.Fatalf("fixture: %q not found", pairs[k])
			}
			s = strings.Replace(s, pairs[k], pairs[k+1], 1)
		}
		return s
	}
	cnc := `"tenant b: could-not-check, no grant" pass exit=0`
	held := `"tenant b: HELD, no grant" pass exit=0`
	cases := []heldScanCase{
		{"comment inside the digest", edit(base, row2,
			"| 2 | `tool check 2` | "+cnc+" | sha256:<!-- -->0123456789ab |"), true},
		{"digest comment, held marker", edit(base, row2,
			"| 2 | `tool check 2` | "+held+" | sha256:<!-- -->0123456789ab |"), true},
		{"comment pipe shifts columns", edit(base,
			"| # | Command |", "| # | Command <!-- | --> |",
			"|---|---------|", "|---|---------|---|",
			"`tool check 1` |", "`tool check 1` <!-- | --> |",
			row2, "| 2 | `tool check 2` <!-- | --> | "+cnc+" | sha256:0123456789ab |"), true},
		{"comment joins two lines", edit(base, row2,
			"| 2 | `tool check 2` <!--\n--> | "+held+" | sha256:0123456789ab |"), true},
		// The flip strips the run once and the witness check strips it
		// again, so a comment the first strip assembles is removed too.
		{"comment built by the strip", edit(base, row2,
			"| 2 | `tool check 2` | "+cnc+" | sha256:<!<!-- -->-- -->0123456789ab |"), true},
	}
	for _, tc := range cases {
		// The shape under test: no source line holding the hold is a
		// witness row, but the stripped row 2 is one and reads pass.
		for _, l := range strings.Split(tc.evidence, "\n") {
			if strings.Contains(l, "tenant b") && isWitnessRow(l) {
				t.Fatalf("%s: fixture: the source line is a witness row", tc.name)
			}
		}
		run, _ := stripRowComments(tc.evidence) // the flip's run view
		rows := parseEvidenceRows(run)["2"]
		if len(rows) == 0 || witnessStateOf(rows[len(rows)-1].Text) != statePass {
			t.Fatalf("%s: fixture: stripped row 2 is not a passing witness", tc.name)
		}
	}
	runHeldScanCasesFlip(t, cases, nil)
}

// TestHeldQuotedLinkTitles: a quote inside markdown link or image syntax (a
// title in single quotes, or a bare destination) is not rendered as text, so
// two of them pair in the source around a hold the page shows unquoted. Link
// or image syntax in the region gives no span, in prose and in a Result cell.
// regression: #4721
func TestHeldQuotedLinkTitles(t *testing.T) {
	runHeldScanCasesFlip(t, []heldScanCase{
		{"two link titles",
			heldEv("ok") + "\nSee [out](/a '\"') then row 3 is HELD per [out](/b '\"') above.\n", true},
		{"two image titles",
			heldEv("ok") + "\nSee ![out](/a.png '\"') then row 3 is HELD per ![out](/b.png '\"') above.\n", true},
		{"quote in a bare destination",
			heldEv("ok") + "\nSee [out](/a/\") then row 3 is HELD per (\".)\n", true},
		{"link titles in a Result cell",
			heldEv("ok [out](/a '\"') row 3 could-not-check [out](/b '\"')"), true},
	}, nil)
}

// TestHeldQuotedCtrlSpace: a form feed or vertical tab renders as white
// space, so a mark flanked by them on both sides is a spaced ditto or inch
// mark, exactly as with spaces, in prose and in a Result cell.
// regression: #4721
func TestHeldQuotedCtrlSpace(t *testing.T) {
	runHeldScanCasesFlip(t, []heldScanCase{
		{"form-feed-flanked marks",
			heldEv("ok") + "\nMeasured a 3\f\"\fgap; row 3 is HELD pending the 5\f\"\fretest.\n", true},
		{"vertical-tab-flanked marks",
			heldEv("ok") + "\nMeasured a 3\v\"\vgap; row 3 is HELD pending the 5\v\"\vretest.\n", true},
		{"form feeds in a Result cell",
			heldEv("3\f\"\fgap; row 3 could-not-check, 5\f\"\fretest"), true},
	}, nil)
}
