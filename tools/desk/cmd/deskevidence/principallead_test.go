package main

import (
	"fmt"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// A renderer shows nothing for a character that renders as nothing and trims the white space
// at either end of a cell, so a cell that opens with such a character and then a space shows a
// reader the login that follows. Every cell below shows a reader an App login and then the
// annotation of a recognised human other than the resolved principal, and is not App-run to the
// lint, so a landing that carries one is refused wherever the checks read it.

// leadRunners are the Runner cells, by family of the character that leads them.
var leadRunners = []struct{ name, cell string }{
	{"format-raw", "\u2063 example-app[bot]" + foldTail},
	{"format-entity", "&#x2063; example-app[bot]" + foldTail},
	{"format-named-entity", "&zwnj; example-app[bot]" + foldTail},
	{"control-entity", "&#157; example-app[bot]" + foldTail},
	{"mark", "\ufe0f example-app[bot]" + foldTail},
	{"filler", "\u115f example-app[bot]" + foldTail},
	{"blank-symbol", "\u2800 example-app[bot]" + foldTail},
	{"private-use", "\ue123 example-app[bot]" + foldTail},
	{"unassigned", "\u0530 example-app[bot]" + foldTail},
	{"repeated-lead", "\u2063 \u2063 example-app[bot]" + foldTail},
	{"lead-wide-tag", "\u2063 example-app\uff3bbot\uff3d" + foldTail},
}

// regression: #4102
// TestStampRefusesBlankLead: each family in the Runner column, in a headerless row's non-last
// cell, in a table after the Evidence section, and in a brief with no Evidence section.
func TestStampRefusesBlankLead(t *testing.T) {
	for _, tc := range leadRunners {
		t.Run(tc.name+"/runner", func(t *testing.T) {
			f := setupStamp(t, true)
			code := landRendered(t, f, restampHead, restampHead+restampRow("2026-10-14", tc.cell))
			wantRenderRefusal(t, f, code, "cannot read the way the lint does", "line 7")
		})
		t.Run(tc.name+"/headerless-cell", func(t *testing.T) {
			f := setupStamp(t, true)
			base := "# Brief\n\n## Evidence\n"
			code := landRendered(t, f, base, base+"| 3 | "+tc.cell+" | "+humanRunner+" |\n")
			wantRenderRefusal(t, f, code, "cell 2 shows as an App-run", "line 4")
		})
		t.Run(tc.name+"/after-section", func(t *testing.T) {
			f := setupStamp(t, true)
			code := landRendered(t, f, restampHead+renderHuman, restampHead+renderHuman+"\n## Notes\n\n| 3 | "+tc.cell+" |\n")
			wantRenderRefusal(t, f, code, "outside the Evidence section the lint reads", "line 11")
		})
		t.Run(tc.name+"/no-section", func(t *testing.T) {
			f := setupStamp(t, true)
			base := "# Brief\n\n## Results\n"
			code := landRendered(t, f, base, base+"\n| # | Runner |\n|---|---|\n| 3 | "+tc.cell+" |\n")
			wantRenderRefusal(t, f, code, "outside the Evidence section the lint reads", "line 7")
		})
	}
}

// regression: #4102
// TestStampRefusesBlankLeadNote: a line that closes a table without a blank line and shows an
// App login with a tag the `[bot]` substring check does not find: after a character that
// renders as nothing and a space, or in parentheses.
func TestStampRefusesBlankLeadNote(t *testing.T) {
	for _, tc := range []struct{ name, note string }{
		{"lead-wide-tag", "\u2063 example-app\uff3bbot\uff3d signed rows 2-3"},
		{"lead-lookalike-tag", "\u2064 example-app[b\u03bft] signed rows 2-3"},
		{"paren-wide-tag", "(example-app\uff3bbot\uff3d) signed rows 2-3"},
		{"paren-lookalike-tag", "(example-app[b\u03bft]) signed rows 2-3"},
		{"paren-lead-wide-tag", "(\u2063 example-app\uff3bbot\uff3d) signed rows 2-3"},
		{"word-paren-wide-tag", "rows run (by example-app\uff3bbot\uff3d)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setupStamp(t, true)
			code := landRendered(t, f, restampHead+renderHuman, restampHead+renderHuman+tc.note+"\n")
			wantRenderRefusal(t, f, code, "closes a table without a blank line", "line 8")
		})
	}
}

// regression: #4102
// TestShowsAppRunBlankLead: every code point of a family that renders as nothing (sampled in the
// planes that hold no assigned character), put before an App login with a space between, raw or
// as a numeric entity, is read as showing App-run text. A raw control is also refused in any row
// before this is read (see rowControlCharacter); its entity is not.
func TestShowsAppRunBlankLead(t *testing.T) {
	families := []*unicode.RangeTable{unicode.Cc, unicode.Cf, unicode.Co, unicode.Mn, unicode.Me,
		unicode.Other_Default_Ignorable_Code_Point, unicode.Noncharacter_Code_Point}
	blank := func(r rune) bool {
		if unicode.In(r, families...) || r == '\u2800' {
			return true
		}
		// Unassigned: in no general category (Go's unicode.C holds them, so it is not used).
		return !unicode.In(r, unicode.L, unicode.M, unicode.N, unicode.P, unicode.S, unicode.Z,
			unicode.Cc, unicode.Cf, unicode.Co, unicode.Cs)
	}
	checked, missed, first := 0, 0, []string(nil)
	for r := rune(1); r <= utf8.MaxRune; r++ {
		if !utf8.ValidRune(r) || unicode.IsSpace(r) || !blank(r) {
			continue
		}
		// Planes 4 to 13 and the private-use planes hold only unassigned and private-use code
		// points: one in 257 of them, and each plane's two noncharacters, is enough.
		if sparse := r >= 0x40000 && r < 0xE0000 || r >= 0xF0000; sparse && r%0x101 != 0 && r&0xFFFE != 0xFFFE {
			continue
		}
		checked++
		for _, cell := range []string{
			string(r) + " example-app[bot]" + foldTail,
			fmt.Sprintf("&#x%X; example-app[bot]", r) + foldTail,
		} {
			if !showsAppRun(cell) {
				if missed++; len(first) < 5 {
					first = append(first, fmt.Sprintf("U+%04X", r))
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no code point checked")
	}
	if missed > 0 {
		t.Fatalf("%d of %d cells with a blank lead and a space not read as App-run; first: %v", missed, 2*checked, first)
	}
}

// regression: #4102
// TestStampBlankLeadAllowed: a character that renders as nothing before a human runner, or
// before a word that is not a login and tag, still lands.
func TestStampBlankLeadAllowed(t *testing.T) {
	for _, runner := range []string{
		"\u2063 human:operator @ abc1234",
		"\u2800 release[draft] @ abc1234",
		"\ufe0f operator @ abc1234 (see example-app review)",
	} {
		t.Run(strings.TrimSpace(runner[strings.IndexByte(runner, ' '):]), func(t *testing.T) {
			f := setupStamp(t, true)
			if code := landRendered(t, f, restampHead, restampHead+restampRow("2026-10-14", runner)); code != deskkit.ExitOK {
				t.Fatalf("exit = %d, want 0: %q", code, lastAudit(t).Detail)
			}
		})
	}
}
