package main

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// A Runner cell is read for who ran the row by its identity region (see identityRegion), not by
// folding characters to the ones a check knows. Every cell below shows a reader an App login
// and then the annotation of a recognised human other than the resolved principal, and is not
// App-run to the lint, so a landing that carries one is refused wherever the checks read it.

const shapeTail = "\u00a0@ abc1234 (on-behalf-of human:colleague)"

// shapeRunners are the Runner cells, by family: a character that renders as nothing before the
// login, a character with no rendering of its own before it, a lookalike of a login character
// inside it, and visible variants of the login or its tag.
var shapeRunners = []struct{ name, cell string }{
	{"invisible-lead-raw", "\u2800example-app[bot]" + shapeTail},
	{"invisible-lead-entity", "&#x2800;example-app[bot]" + shapeTail},
	{"unrendered-lead-a", "\ue000example-app[bot]" + shapeTail},
	{"unrendered-lead-b", "\u0378example-app[bot]" + shapeTail},
	{"login-lookalike-a", "example\u2010app[bot]" + shapeTail},
	{"login-lookalike-b", "example\u2011app[bot]" + shapeTail},
	{"login-lookalike-c", "example\u2024app[bot]" + shapeTail},
	{"visible-variant-a", "@example-app[bot]" + shapeTail},
	{"visible-variant-b", "\"example-app[bot]" + shapeTail},
	{"visible-variant-c", "example-app[ bot ]" + shapeTail},
	{"visible-variant-d", "example-app\u2039bot\u203a" + shapeTail},
	{"visible-variant-e", "example-app[\u24d1ot]" + shapeTail},
	{"any-non-ascii-lead", "\u2022example-app[bot]" + shapeTail},
	{"symbol-tag", "\u2800[\u24d1\u24de\u24e3]" + shapeTail},
	{"filler-symbol-tag", "\u3164[\u24d1\u24de\u24e3]" + shapeTail},
	{"angle-tag", "example-app&lt; bot &gt;" + shapeTail},
}

// regression: #4102
// TestStampRefusesShapeFamily: each family in the Runner column, in a headerless row's
// non-last cell, and in a brief with no Evidence section.
func TestStampRefusesShapeFamily(t *testing.T) {
	for _, tc := range shapeRunners {
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
		t.Run(tc.name+"/no-section", func(t *testing.T) {
			f := setupStamp(t, true)
			base := "# Brief\n\n## Results\n"
			code := landRendered(t, f, base, base+"\n| # | Runner |\n|---|---|\n| 3 | "+tc.cell+" |\n")
			wantRenderRefusal(t, f, code, "outside the Evidence section the lint reads", "line 7")
		})
		t.Run(tc.name+"/note-line", func(t *testing.T) {
			f := setupStamp(t, true)
			// Without the on-behalf-of text, only the login and its tag show App-run text.
			note := "see " + strings.Replace(tc.cell, " (on-behalf-of human:colleague)", "", 1)
			code := landRendered(t, f, restampHead+renderHuman, restampHead+renderHuman+note+"\n")
			wantRenderRefusal(t, f, code, "closes a table without a blank line", "line 8")
		})
	}
}

// regression: #4102
// TestStampRefusesShownOnly: text that shows App-run or on-behalf-of text in only one shown
// reading (emphasis removed, a comment hidden, an image's alt text shown, or a bracket read as
// a shortcut reference link), so the cell or line as written does not.
func TestStampRefusesShownOnly(t *testing.T) {
	for _, tc := range []struct{ name, note, cell string }{
		{"strong-emphasis", "see on-behalf-**of** human:colleague", ""},
		{"underscore-emphasis", "see on-behalf-_of_ human:colleague", ""},
		{"hidden-comment", "see on-behalf<!--x-->-of human:colleague", ""},
		{"image-alt", "", "![example-app](x.png)[bot]: @ abc1234"},
		{"shortcut-link", "", "[example-app]\\[bot\\]: @ abc1234"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setupStamp(t, true)
			if tc.note != "" {
				code := landRendered(t, f, restampHead+renderHuman, restampHead+renderHuman+tc.note+"\n")
				wantRenderRefusal(t, f, code, "closes a table without a blank line", "line 8")
				return
			}
			base := "# Brief\n\n## Evidence\n"
			code := landRendered(t, f, base, base+"| 3 | "+tc.cell+" | "+humanRunner+" |\n")
			wantRenderRefusal(t, f, code, "cell 2 shows as an App-run", "line 4")
		})
	}
}

// regression: #4102
// TestStampShapeRegionAllowed: brackets that open no tag, an unclosed bracket, and a tag
// written only in the login alphabet still land in a human Runner cell.
func TestStampShapeRegionAllowed(t *testing.T) {
	for _, runner := range []string{
		"[21:24Z][ok-all] @ abc1234",
		"[[abc]] @ abc1234",
		"human:operator[draft @ abc1234 (see [notes])",
		"release[draft] @ abc1234",
		"v2[beta]-[ wip @ abc1234",
	} {
		t.Run(runner, func(t *testing.T) {
			f := setupStamp(t, true)
			if code := landRendered(t, f, restampHead, restampHead+restampRow("2026-10-14", runner)); code != deskkit.ExitOK {
				t.Fatalf("exit = %d, want 0: %q", code, lastAudit(t).Detail)
			}
		})
	}
}

// regression: #4102
// TestShowsAppRunAnyNonASCII: a non-ASCII character anywhere before or inside the login of a
// cell that shows an App login is read as showing App-run text, whatever the character is.
func TestShowsAppRunAnyNonASCII(t *testing.T) {
	missed, first := 0, []string(nil)
	for r := rune(0x80); r <= utf8.MaxRune; r++ {
		if !utf8.ValidRune(r) {
			continue
		}
		for _, cell := range []string{
			string(r) + "example-app[bot]" + shapeTail,
			"example" + string(r) + "app[bot]" + shapeTail,
		} {
			if !showsAppRun(cell) {
				if missed++; len(first) < 5 {
					first = append(first, fmt.Sprintf("U+%04X", r))
				}
			}
		}
	}
	if missed > 0 {
		t.Fatalf("%d cells with a non-ASCII character in the login not read as App-run; first: %v", missed, first)
	}
}

// regression: #4102
// TestStampLandsCJKTags: CJK text in brackets is not a login with a tag, so a note line that
// closes a table and a human Runner cell carrying it land.
func TestStampLandsCJKTags(t *testing.T) {
	for _, tc := range []struct{ name, base, content string }{
		{"note-square", restampHead + renderHuman, restampHead + renderHuman + "[東京都]\n"},
		{"note-lenticular", restampHead + renderHuman, restampHead + renderHuman + "【東京都】\n"},
		{"note-glued", restampHead + renderHuman, restampHead + renderHuman + "東京【東京都】 note\n"},
		{"runner-lenticular", restampHead, restampHead + restampRow("2026-10-14", "東京【東京都】 @ abc1234")},
		{"runner-square", restampHead, restampHead + restampRow("2026-10-14", "東京[東京都] @ abc1234")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setupStamp(t, true)
			if code := landRendered(t, f, tc.base, tc.content); code != deskkit.ExitOK {
				t.Fatalf("exit = %d, want 0: %q", code, lastAudit(t).Detail)
			}
		})
	}
}
