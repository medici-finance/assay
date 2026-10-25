package main

import (
	"fmt"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// A renderer shows nothing for a character that renders as nothing, or for an element a
// sanitizer drops with its text, and trims the white space (of any kind) at either end of a
// cell, and a reader takes a leading word with no ASCII letter or digit, or one a renderer shows
// as a glyph, for something other than a name, so a cell that opens with such a character,
// element or word and then white space shows a reader the login that follows. Every cell below shows a reader an App
// login and then the annotation of a recognised human other than the resolved principal, and
// is not App-run to the lint, so a landing that carries one is refused wherever the checks read
// it.

// leadRunners are the Runner cells, by family of the character or word that leads them.
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
	{"symbol-raw", "\u2713 example-app[bot]" + foldTail},
	{"symbol-entity", "&#x2713; example-app[bot]" + foldTail},
	{"symbol-repeated", "\u2713 \u2713 example-app[bot]" + foldTail},
	{"symbol-after-blank", "\u2063 \u2713 example-app[bot]" + foldTail},
	{"letter", "\u00e9 example-app[bot]" + foldTail},
	{"digit", "\u0663 example-app[bot]" + foldTail},
	{"punctuation", "- example-app[bot]" + foldTail},
	{"symbol-nbsp", "\u2713\u00a0example-app[bot]" + foldTail},
	{"symbol-wide-space", "\u2713\u3000example-app[bot]" + foldTail},
	{"symbol-tab", "\u2713\texample-app[bot]" + foldTail},
	{"blank-nbsp", "\u2063\u00a0example-app[bot]" + foldTail},
	{"blank-tab", "\u2063\texample-app[bot]" + foldTail},
	{"nbsp-only", "\u00a0example-app[bot]" + foldTail},
	{"wide-space-only", "\u3000example-app[bot]" + foldTail},
	{"nbsp-space", "\u00a0 example-app[bot]" + foldTail},
	{"wide-space-space", "\u3000 example-app[bot]" + foldTail},
	{"nbsp-entity-space", "&nbsp; example-app[bot]" + foldTail},
	{"wide-space-entity-space", "&#x3000; example-app[bot]" + foldTail},
	{"tab-entity-space", "&#9; example-app[bot]" + foldTail},
	{"blank-nbsp-space", "\u2063\u00a0 example-app[bot]" + foldTail},
	{"blank-wide-space-space", "\u2063\u3000 example-app[bot]" + foldTail},
	{"blank-tab-space", "\u2063\t example-app[bot]" + foldTail},
	{"symbol-nbsp-space", "\u2713\u00a0 example-app[bot]" + foldTail},
	{"symbol-tab-space", "\u2713\t example-app[bot]" + foldTail},
	{"glyph-word", ":white_check_mark: example-app[bot]" + foldTail},
	{"glyph-word-sign", ":+1: example-app[bot]" + foldTail},
	{"glyph-word-plain", ":robot: example-app[bot]" + foldTail},
	{"glyph-word-paren", "(:robot:) example-app[bot]" + foldTail},
	{"glyph-word-entity", "&#58;robot&#58; example-app[bot]" + foldTail},
	{"glyph-word-escaped", ":white\\_check\\_mark: example-app[bot]" + foldTail},
	{"dropped-element", "<script>x</script> example-app[bot]" + foldTail},
	{"dropped-element-style", "<style>p</style> example-app[bot]" + foldTail},
	{"glyph-word-nbsp-end", ":robot: example-app[bot]" + foldTail + "&#160;"},
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
// TestShowsAppRunLeadWord: every code point that is not an ASCII letter, digit or white space
// (sampled in the planes 2 to 13, 15 and 16), put before an App login with white space between,
// raw or as a numeric entity, is read as showing App-run text, as is an App login after white
// space alone. A raw control is also refused in any row before this is read (see
// rowControlCharacter); its entity is not.
func TestShowsAppRunLeadWord(t *testing.T) {
	for _, sp := range []string{" ", "\t", "\u00a0", "\u3000"} {
		if cell := sp + "example-app[bot]" + foldTail; !showsAppRun(cell) {
			t.Errorf("%q not read as App-run", cell)
		}
		// White space of any kind beside an ASCII space, alone, after a character that renders
		// as nothing or after a symbol, is trimmed, so the login after it is read.
		for _, lead := range []string{"", "\u2063", "\u2713"} {
			for _, ws := range []string{sp + " ", " " + sp} {
				if cell := lead + ws + "example-app[bot]" + foldTail; !showsAppRun(cell) {
					t.Errorf("%q not read as App-run", cell)
				}
			}
		}
		// An ASCII lead that is no login ends at any white space, not only at a space.
		if cell := "(" + sp + "example-app[bot]" + foldTail; !showsAppRun(cell) {
			t.Errorf("%q not read as App-run", cell)
		}
	}
	type span struct{ lo, hi rune }
	var spans []span
	for lo := rune(0); lo <= utf8.MaxRune; lo += 0x4000 {
		spans = append(spans, span{lo, lo + 0x3FFF})
	}
	for _, sp := range spans {
		t.Run(fmt.Sprintf("U+%04X", sp.lo), func(t *testing.T) {
			checked, missed, first := 0, 0, []string(nil)
			for r := sp.lo; r <= sp.hi; r++ {
				if !utf8.ValidRune(r) || r == 0 || unicode.IsSpace(r) || asciiAlnum(r) {
					continue
				}
				// Planes 2 to 13 and the private-use planes hold no character that is not a
				// letter, an unassigned or a private-use code point: one in 257 of them, and
				// each plane's two noncharacters, is enough.
				if sparse := r >= 0x20000 && r < 0xE0000 || r >= 0xF0000; sparse && r%0x101 != 0 && r&0xFFFE != 0xFFFE {
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
			if missed > 0 {
				t.Fatalf("%d of %d cells with a lead and a space not read as App-run; first: %v", missed, 2*checked, first)
			}
		})
	}
}

// regression: #4102
// TestShowsTagBlankInside: every code point of a family that renders as nothing (sampled in the
// planes that hold no assigned character), put inside a `[bot]` tag that opens a line, raw or
// as a numeric entity, is read as showing App-run text, which only the reading with such
// characters dropped shows.
func TestShowsTagBlankInside(t *testing.T) {
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
		if sparse := r >= 0x40000 && r < 0xE0000 || r >= 0xF0000; sparse && r%0x101 != 0 && r&0xFFFE != 0xFFFE {
			continue
		}
		checked++
		for _, line := range []string{
			"[b" + string(r) + "ot] signed rows 2-3",
			fmt.Sprintf("[b&#x%X;ot] signed rows 2-3", r),
		} {
			if !showsAttribution(line) {
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
		t.Fatalf("%d of %d lines with a blank inside a tag not read as App-run; first: %v", missed, 2*checked, first)
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
		"\u2713 human:operator @ abc1234",
		"- operator @ abc1234 (see example-app review)",
		":tada: human:operator @ abc1234",
		"<script>x</script> human:operator @ abc1234",
		"human:operator @ abc1234 (rerun 10:30:00 by example-app review)",
	} {
		t.Run(strings.TrimSpace(runner[strings.IndexByte(runner, ' '):]), func(t *testing.T) {
			f := setupStamp(t, true)
			if code := landRendered(t, f, restampHead, restampHead+restampRow("2026-10-14", runner)); code != deskkit.ExitOK {
				t.Fatalf("exit = %d, want 0: %q", code, lastAudit(t).Detail)
			}
		})
	}
}

// regression: #4102
// TestShowsAppRunGlyphLead: a word a renderer shows as a glyph, before an App login with white
// space between, alone, repeated, in parentheses, after a symbol or with its colons as entities,
// is read as showing App-run text; one before a human runner, inside a word or later in the
// text, where the reading as written already shows what stands there, is not.
func TestShowsAppRunGlyphLead(t *testing.T) {
	for _, word := range []string{":robot:", ":+1:", ":-1:", ":100:", ":white_check_mark:", ":Robot:"} {
		for _, lead := range []string{
			word,
			word + " " + word,
			"(" + word + ")",
			"\u2713 " + word,
			strings.ReplaceAll(word, ":", "&#58;"),
		} {
			if cell := lead + " example-app[bot]" + foldTail; !showsAppRun(cell) {
				t.Errorf("%q not read as App-run", cell)
			}
		}
		// White space other than an ASCII space at the end of the cell is trimmed before the
		// glyph reading is compared with the text it reads past.
		for _, end := range []string{"\u00a0", "&#160;", "\u3000", "&#x3000;"} {
			if cell := word + " example-app[bot]" + foldTail + end; !showsAppRun(cell) {
				t.Errorf("%q not read as App-run", cell)
			}
		}
		for _, cell := range []string{
			word + " human:operator @ abc1234",
			"x" + word + "x example-app[bot]" + foldTail,
			"x" + word + "[abc] human:operator @ abc1234",
		} {
			if showsAppRun(cell) {
				t.Errorf("%q read as App-run", cell)
			}
		}
	}
}
