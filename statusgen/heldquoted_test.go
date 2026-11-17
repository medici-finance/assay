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

// TestHeldQuotedNBSPSide: a no-break space on one side of one quote, before
// the opening quote or after the closing one, gives no span, so every read
// refuses. Each side is pinned alone; the twin with an ASCII space in its
// place pairs as a quoted span, so only the flip read admits it.
// regression: #4721
func TestHeldQuotedNBSPSide(t *testing.T) {
	runHeldScanCasesFlip(t, []heldScanCase{
		{"NBSP before the open quote",
			heldEv("ok") + "\nRow 2 printed \"row 3 is HELD for audit\" on rerun.\n", true},
		{"NBSP after the close quote",
			heldEv("ok") + "\nRow 2 printed \"row 3 is HELD for audit\" on rerun.\n", true},
		{"space before the open quote",
			heldEv("ok") + "\nRow 2 printed \"row 3 is HELD for audit\" on rerun.\n", true},
		{"NBSP before, in a Result cell",
			heldEv("printed \"row 3 could-not-check, no grant\" on rerun"), true},
		{"NBSP after, in a Result cell",
			heldEv("printed \"row 3 could-not-check, no grant\" on rerun"), true},
	}, map[string]bool{"space before the open quote": true})
}

// TestHeldQuotedWitnessFirst: a comment that joins two source lines into one
// witness row marks every joined line, the first included. Here the hold
// sits on the first line and the comment opens after the digest, inside
// inline code: the page shows the opener as code, so the scan's line holds
// no comment syntax, while the comment strip the witness check reads joins
// the two lines into a passing witness row. A mark on the last joined line
// alone would let the flip skip the hold.
// regression: #4721
func TestHeldQuotedWitnessFirst(t *testing.T) {
	base := witEntry("Run 1", witTable(2, "", witPass, ""), "", "**VERIFY: PASS** — 2/2 rows")
	row2 := "| 2 | `tool check 2` | pass exit=0 | sha256:0123456789ab | 2026-10-22 | fixture-verifier[bot] @ 0123456789ab |"
	if !strings.Contains(base, row2) {
		t.Fatalf("fixture: row 2 not found")
	}
	ev := strings.Replace(base, row2,
		"| 2 | `tool check 2` | \"tenant b: HELD, no grant\" pass exit=0 | sha256:0123456789ab `<!--` | x | y |\n"+
			"--> | 2026-10-22 | fixture-verifier[bot] @ 0123456789ab |", 1)
	run, _ := stripRowComments(ev)
	rows := parseEvidenceRows(run)["2"]
	if len(rows) == 0 || witnessStateOf(rows[len(rows)-1].Text) != statePass {
		t.Fatalf("fixture: stripped row 2 is not a passing witness")
	}
	runHeldScanCasesFlip(t, []heldScanCase{{"hold on the first joined line", ev, true}}, nil)
}

// TestHeldQuotedCrossCell: an HTML comment that opens in one cell, or on an
// earlier line, and closes in the cell holding the quoted hold hides a quote
// the cell's own "<" check never sees. HTML comment syntax anywhere on the
// line gives no span: every read refuses.
// regression: #4721
func TestHeldQuotedCrossCell(t *testing.T) {
	hdr := "**VERIFY: PASS** (model) — row 1 green.\n\n" +
		"| # | Command | Exit | Result | Date | Runner |\n" +
		"|---|---------|------|--------|------|--------|\n"
	runHeldScanCasesFlip(t, []heldScanCase{
		{"comment from the Command cell",
			hdr + "| 2 | `grantcheck --all` <!-- | 0 | --> \"tenant b: could-not-check, no grant\" | 2026-10-03 | fixture-verifier |\n", true},
		{"comment from the line above",
			hdr + "| 1 | `go test ./...` | 0 | ok | 2026-10-03 | fixture-verifier | <!--\n" +
				"| 2 | `grantcheck --all` | 0 | --> \"tenant b: could-not-check, no grant\" | 2026-10-03 | fixture-verifier |\n", true},
		{"closer alone before the span",
			hdr + "| 2 | `grantcheck --all` | 0 | a --> \"tenant b: could-not-check, no grant\" | 2026-10-03 | fixture-verifier |\n", true},
		{"no comment: quoted output",
			hdr + "| 2 | `grantcheck --all` | 0 | \"tenant b: could-not-check, no grant\" | 2026-10-03 | fixture-verifier |\n", true},
	}, map[string]bool{"no comment: quoted output": true})
}

// TestHeldQuotedCtrlInch: a C0 control or DEL byte renders as nothing, so a
// quote right after "5\x01" is an inch mark the letter-or-digit rule cannot
// see. A control byte beside a quote gives no span, in prose and in a Result
// cell: every read refuses.
// regression: #4721
func TestHeldQuotedCtrlInch(t *testing.T) {
	runHeldScanCasesFlip(t, []heldScanCase{
		{"C0 byte before inch marks",
			heldEv("ok") + "\nMeasured a 3\x01\" gap; row 3 HELD pending the 5\x01\" retest.\n", true},
		{"DEL byte before inch marks",
			heldEv("ok") + "\nMeasured a 3\x7f\" gap; row 3 HELD pending the 5\x7f\" retest.\n", true},
		{"C0 byte after a close quote",
			heldEv("ok") + "\nRow 2 printed \"tenant b: HELD\"\x01x on rerun.\n", true},
		{"C0 inch marks in a Result cell",
			heldEv("3\x01\" gap; row 3 could-not-check, 5\x01\" retest"), true},
	}, nil)
}

// TestHeldQuotedFencedWit: the witness check reads table rows inside fenced
// code (parseEvidenceRows), so a fenced verifyrun witness row is credited by
// its first state word while the held scan skipped the fence. On the flip
// read a fenced witness row is read whole: a hold on it refuses, quoted or
// not. The card and close-verify run no witness check and keep skipping it.
// regression: #4721
func TestHeldQuotedFencedWit(t *testing.T) {
	row2 := "| 2 | `tool check 2` | pass exit=0 |"
	fenced := func(fence, result string) string {
		table := strings.Replace(witTable(2, "", witPass, ""), row2, "| 2 | `tool check 2` | "+result+" |", 1)
		if !strings.Contains(table, result) {
			t.Fatalf("fixture: row 2 not rewritten")
		}
		ev := witEntry("Run 1", fence+"\n"+table+fence+"\n", "", "**VERIFY: PASS** — 2/2 rows")
		p, err := flipLatestPass(ev)
		if err != nil {
			t.Fatalf("fixture: %v", err)
		}
		if rows := parseEvidenceRows(p.Run)["2"]; len(rows) == 0 || witnessStateOf(rows[len(rows)-1].Text) != statePass {
			t.Fatalf("fixture: fenced row 2 is not a passing witness")
		}
		return ev
	}
	for _, tc := range []struct {
		name   string
		ev     string
		wantFl bool
	}{
		{"fenced hold before the state", fenced("```", "HELD pass exit=0"), true},
		{"tilde-fenced hold", fenced("~~~", "HELD pass exit=0"), true},
		{"four-backtick fenced hold", fenced("````", "HELD pass exit=0"), true},
		{"fenced quoted hold", fenced("```", `"tenant b: could-not-check, no grant" pass exit=0`), true},
		{"fenced row with no hold", fenced("```", "pass exit=0"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if held, why := verifyPassHeldContradiction(tc.ev); held {
				t.Errorf("verifyPassHeldContradiction held, want not (why=%q)", why)
			}
			if err := closeVerifyHeldRefusal("ic/01", "verified", tc.ev); err != nil {
				t.Errorf("closeVerifyHeldRefusal err=%v, want none", err)
			}
			if err := flipHeldRefusal("ic/01", "verified", tc.ev); (err != nil) != tc.wantFl {
				t.Errorf("flipHeldRefusal err=%v, want refusal=%v", err, tc.wantFl)
			}
		})
	}
}
