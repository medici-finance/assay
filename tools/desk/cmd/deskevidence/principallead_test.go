package main

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// A renderer shows nothing for a character that renders as nothing, for an element a sanitizer
// drops with its text, or for one it hides by default; it trims the white space (of any kind)
// at either end of a cell; and it can typeset a math span as a symbol, as blank space or as the
// text inside it, with the white space in it dropped and a bracket control word shown as the
// bracket. It shows a dollar sign that is escaped, written as an entity or inside a code span
// as text, never as a math delimiter, and it can hide an element whose text holds nested
// elements, or whose hidden attribute follows a slash or a quote. A reader takes a leading word
// with no ASCII letter or digit, or one a renderer shows as a glyph, for something other than a
// name. So a cell that opens with such a character, element, word or span and then white
// space, or whose math span holds a login, shows a reader that login. Every cell below shows a reader an App login and then the
// annotation of a recognised human other than the resolved principal, and is not App-run to
// the lint, so a landing that carries one is refused wherever the checks read it.

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
	// A lead read past, followed by later text the lead's reading must not depend on.
	{"glyph-word-later-word", ":robot: example-app[bot]" + foldTail + " :sparkles:"},
	{"glyph-word-later-glued", ":robot: example-app[bot]" + foldTail + ":sparkles:"},
	{"glyph-word-later-inner", ":robot: example-app[bot] :sparkles:" + foldTail},
	{"glyph-word-later-numbers", ":robot: example-app[bot] @ abc1234 (on-behalf-of human:colleague, 09:15:30)"},
	{"glyph-word-sign-later-numbers", ":+1: example-app[bot]" + foldTail + " 23:59:59Z"},
	// A lead read past, in a cell whose end is white space and then a character that renders
	// as nothing.
	{"glyph-word-space-blank-end", ":robot: example-app[bot]" + foldTail + " \u2063"},
	{"glyph-word-nbsp-blank-end-entity", ":robot: example-app[bot]" + foldTail + "&#160;&#8291;"},
	{"glyph-word-wide-space-blank-end", ":white_check_mark: example-app[bot]" + foldTail + "\u3000\u200b"},
	{"typeset-symbol", "$\\bullet$ example-app[bot]" + foldTail},
	{"typeset-blank", "$\\;$ example-app[bot]" + foldTail},
	{"typeset-spaced", "$\\cdot \\cdot$ example-app[bot]" + foldTail},
	{"typeset-display", "$$\\ast$$ example-app[bot]" + foldTail},
	{"disclosure-summary", "<details><summary>&#x2714;</summary>y</details> example-app[bot]" + foldTail},
	{"disclosure-bare", "<details>y</details> example-app[bot]" + foldTail},
	{"line-break", "y <br> example-app[bot]" + foldTail},
	{"block-element", "<div>y</div> example-app[bot]" + foldTail},
	// A math span whose text a renderer typesets as text, after a command it typesets as blank
	// space or as nothing.
	{"typeset-text", "$\\qquad \\textrm{example-app[bot]}$" + foldTail},
	{"typeset-text-blank-argument", "$\\hphantom{y} \\mathrm{example-app[bot]}$" + foldTail},
	{"typeset-text-glued", "$\\hphantom{y}example-app[bot]$" + foldTail},
	{"typeset-text-bare-argument", "$\\hphantom y example-app[bot]$" + foldTail},
	{"typeset-text-command-end", "$\\hphantom y\\,example-app[bot]$" + foldTail},
	{"typeset-symbol-then-text", "$\\circ$ $\\qquad \\textrm{example-app[bot]}$" + foldTail},
	{"typeset-text-bare", "$\\qquad example-app[bot]$" + foldTail},
	{"typeset-text-whole-cell", "$\\qquad \\textrm{example-app[bot]" + foldTail + "}$"},
	{"typeset-text-code", "$`\\enspace \\textrm{example-app[bot]}`$" + foldTail},
	{"typeset-text-display", "$$\\qquad \\textrm{example-app[bot]}$$" + foldTail},
	{"typeset-text-entity-space", "$\\qquad&#x20;\\textrm{example-app[bot]}$" + foldTail},
	{"typeset-text-symbol-argument", "$\\hphantom(\\,example-app[bot]$" + foldTail},
	{"typeset-text-split", "$\\mathrm{example}\\mathrm{-app}\\mathrm{[bot]}$" + foldTail},
	// An element a renderer hides by default with its text.
	{"hidden-ruby-fallback", "<rp>(y)</rp> example-app[bot]" + foldTail},
	{"hidden-ruby-nested", "<ruby><rp>(z)</rp></ruby> example-app[bot]" + foldTail},
	{"hidden-attribute", "<span hidden>y</span> example-app[bot]" + foldTail},
	{"hidden-data-list", "<datalist>y</datalist> example-app[bot]" + foldTail},
	{"hidden-then-dropped", "<rp>(y)</rp> <style>example-app[bot]</style>" + foldTail},
	{"dropped-then-hidden", "<style>(y)</style> <span hidden>example-app[bot]</span>" + foldTail},
	{"hidden-and-dropped", "<rp>(y)</rp> <style>z</style> example-app[bot]" + foldTail},
	// An element a renderer hides by default, holding a nested element, or with its attribute
	// after a separator other than white space.
	{"hidden-attribute-nested", "<small hidden><u>y</u> z</small> example-app[bot]" + foldTail},
	{"hidden-attribute-nested-empty", "<small hidden><em></em>y z</small> example-app[bot]" + foldTail},
	{"hidden-attribute-slash", "<abbr/hidden>y z</abbr> example-app[bot]" + foldTail},
	{"hidden-attribute-after-quote", "<abbr title=\"t\"hidden>y z</abbr> example-app[bot]" + foldTail},
	{"hidden-ruby-fallback-nested", "<rp>(<rp>y</rp>) z</rp> example-app[bot]" + foldTail},
	{"hidden-data-list-nested", "<datalist><datalist>y</datalist> z</datalist> example-app[bot]" + foldTail},
	// A math span after a dollar sign a renderer does not pair with it or shows as text: one
	// written alone, escaped, as an entity or inside a code span.
	{"typeset-after-lone-dollar", "$ $\\enspace \\textsf{example-app[bot]}$" + foldTail},
	{"typeset-after-glyph-lone-dollar", ":robot: $ $\\qquad \\textrm{example-app[bot]}$" + foldTail},
	{"symbol-span-after-lone-dollar", "$ $\\star$ example-app[bot]" + foldTail},
	{"typeset-after-escaped-dollar", "\\$ $\\enspace \\textrm{example-app[bot]}$" + foldTail},
	{"symbol-span-after-escaped-dollar", "\\$ $\\diamond$ example-app[bot]" + foldTail},
	{"typeset-after-entity-dollar", "&#x24; $\\qquad \\textsf{example-app[bot]}$" + foldTail},
	{"typeset-after-padded-entity-dollar", "&#0036; $\\enspace \\mathrm{example-app[bot]}$" + foldTail},
	{"typeset-after-code-dollar", "`$$` $\\enspace \\mathrm{example-app[bot]}$" + foldTail},
	{"symbol-span-after-code-dollar", "`` $ `` $\\star$ example-app[bot]" + foldTail},
	// A math span whose text a renderer typesets without the white space in it, or with a
	// bracket it names by a control word.
	{"typeset-letters-spaced", "$\\enspace \\mathrm{e x a m p l e - a p p [ b o t ]}$" + foldTail},
	{"typeset-bracket-words", "$\\enspace \\mathrm{example-app\\lbrack bot\\rbrack}$" + foldTail},
	{"typeset-letters-spaced-after-span", "$\\hphantom{y(}$ $\\enspace \\mathrm{e x a m p l e - a p p [ b o t ]}$" + foldTail},
	{"typeset-bracket-words-spaced", "$\\qquad example-app \\lbrack bot \\rbrack$" + foldTail},
	// A math span holding a control word the tool neither names nor reads as a character code,
	// inside the tag: one that takes several arguments and typesets only one, or one that
	// typesets a letter of its own. The span fails closed.
	{"typeset-tag-unnamed-multi-argument", "$\\mathrm{example-app}\\lbrack{}\\mathrm{b}\\mathchoice{zz}{o}{zz}{zz}\\mathrm{t}\\rbrack$" + foldTail},
	{"typeset-tag-unnamed-glyph", "$\\mathrm{example-app}\\lbrack \\mathrm{b}\\ell \\mathrm{t}\\rbrack$" + foldTail},
	// A lead read past and a lone dollar sign, with a later dollar sign the lead's reading must
	// not pair it with.
	{"glyph-dollar-later-dollar", ":robot: $ example-app[bot]" + foldTail + " $"},
	{"glyph-dollar-later-amount", ":robot: $ example-app[bot]" + foldTail + " (cost $5)"},
	{"glyph-glued-dollar-later-dollar", ":robot:$ example-app[bot]" + foldTail + " $"},
	{"glyph-dollars-later-dollars", ":robot: $$ example-app[bot]" + foldTail + " $$"},
	{"typeset-dollar-later-dollar", "$\\circ$ $ example-app[bot]" + foldTail + " $"},
	{"glyph-dollar-later-dollar-escaped-tag", ":robot: $ example-app[bot\\]" + foldTail + " $"},
	// A math span between backtick runs a renderer does not pair as a code span: runs of
	// different lengths, or a run whose first backtick is escaped.
	{"code-runs-unequal", "`$` `` $\\qquad \\mathsf{example-app[bot]}$ `" + foldTail},
	{"code-run-escaped", "`$` \\` $\\qquad \\mathsf{example-app[bot]}$ `" + foldTail},
	{"code-runs-longer-open", "`$` ``` $\\qquad \\mathsf{example-app[bot]}$ ``" + foldTail},
	{"code-runs-shorter-open", "`$` ` $\\qquad \\mathsf{example-app[bot]}$ ``" + foldTail},
	// A math span whose tag is bracketed by control words the tool does not name, any of which a
	// renderer can typeset as a bracket, with or without another control word inside the tag.
	{"typeset-bracket-words-unnamed", "$\\qquad \\mathsf{example-app\\lang bot\\rang}$" + foldTail},
	{"typeset-corner-words", "$\\qquad \\mathsf{example-app\\ulcorner bot\\urcorner}$" + foldTail},
	{"typeset-moustache-words", "$\\qquad \\mathsf{example-app\\lmoustache bot\\rmoustache}$" + foldTail},
	{"typeset-bracket-words-word-inside", "$\\qquad \\mathsf{example-app\\lang b\\textrm{o}t\\rang}$" + foldTail},
	// A `>` inside a quoted attribute value, which ends no tag, before an element's text or
	// before the hidden attribute.
	{"quoted-bracket-single", "<em data-x='> y '>example-app[bot]</em>" + foldTail},
	{"quoted-bracket-double", "<q cite=\"a> b c\">example-app[bot]</q>" + foldTail},
	{"hidden-after-quoted-bracket", "<small data-n='>' hidden>y z</small> example-app[bot]" + foldTail},
	// A hidden element the narrow reading drops, then one with its hidden attribute after a
	// slash holding the login, which shows where a renderer hides only the first.
	{"hidden-then-hidden-slash", "<i hidden>y z</i> <b/hidden>example-app[bot]" + foldTail + "</b>"},
	// Two hidden elements written the same way, the login in the second, which shows where a
	// renderer hides only the first.
	{"hidden-slash-then-hidden-slash", "<b/hidden>y z</b> <i/hidden>example-app[bot]" + foldTail + "</i>"},
	// A hidden element left open before a dialog holding nested markup, which shows the login
	// where a renderer hides the dialog and not the element left open.
	{"hidden-open-then-dialog", "<b/hidden><dialog><u>q</u>y z</dialog> example-app[bot]" + foldTail},
	// An apostrophe inside an unquoted attribute value, which opens no quote, and a later quote
	// the tool could pair with it.
	{"quoted-bracket-unquoted-apostrophe", "<b x=it's>example-app[bot]</b> <i y='>" + foldTail},
	// A math span after a lone run of `$` whose text a bracket joins to the word before the
	// control word that leads the login.
	{"typeset-after-lone-dollar-bracket", "$ $y(\\quad{}example-app[bot]$" + foldTail},
	// A math span inside a code span, for a renderer that typesets it there.
	{"typeset-inside-code-span", "`$\\qquad \\mathsf{example-app[bot]}$`" + foldTail},
	// A span after a lone run of `$` whose text ends in a letter, read as a span that shows
	// only a symbol.
	{"symbol-span-letter-after-lone-dollar", "$ $q$ example-app[bot]" + foldTail},
	// A math span whose tag a renderer typesets from character codes, decimal or hex, with
	// optional arguments or not, for a bracket or for a letter, beside a bracket it names.
	{"typeset-code-words-decimal", "$\\enspace \\textsf{example-app\\unicode{91}bot\\unicode{93}}$" + foldTail},
	{"typeset-code-words-options", "$\\quad \\textrm{example-app\\unicode[sans-serif]{91}bot\\unicode[sans-serif]{93}}$" + foldTail},
	{"typeset-code-letter", "$\\quad \\mathrm{example-app[\\unicode{98}ot]}$" + foldTail},
	{"typeset-code-word-and-named", "$\\enspace \\textsf{example-app\\unicode{0x5b}bot\\rbrack}$" + foldTail},
	{"typeset-char-words", "$\\qquad \\mathsf{example-app\\char91 bot\\char93}$" + foldTail},
	// A math span whose tag one control word the tool does not name opens, closed by a bracket
	// it names, and one whose bracket words each take an argument a renderer can typeset into
	// the bracket.
	{"typeset-one-unnamed-word", "$\\qquad \\mathsf{example-app\\lang bot\\rbrack}$" + foldTail},
	{"typeset-bracket-words-arguments", "$\\enspace \\textrm{example-app\\leftbr{s}bot\\rightbr{s}}$" + foldTail},
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
		":tada: human:operator @ abc1234 (rerun 10:30:00 by example-app review) :sparkles:",
		"$5 human:operator @ abc1234 (cost $6)",
		"<details><summary>&#x2714;</summary>y</details> human:operator @ abc1234",
		"y <br> human:operator @ abc1234 (see example-app review)",
		"<rp>(</rp> human:operator @ abc1234",
		"<span hidden>y</span> human:operator @ abc1234 (cost $5, then $6)",
		"<small hidden><u>y</u> z</small> human:operator @ abc1234",
		"<em data-x='> y '>human:operator</em> @ abc1234",
		"<small data-n='>' hidden>y z</small> human:operator @ abc1234",
		"<i hidden>y z</i> <b/hidden>human:operator @ abc1234</b>",
		"<b/hidden><dialog><u>q</u>y z</dialog> human:operator @ abc1234",
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
// TestStampUnnamedMathLeadRefused: a lead or runner whose math span holds a control word the
// tool neither names nor reads as a character code fails closed, even where the word would
// typeset nothing or a blank: the tool cannot know what a renderer typesets for it.
func TestStampUnnamedMathLeadRefused(t *testing.T) {
	for _, runner := range []string{
		"$\\bullet$ human:operator @ abc1234",
		"$\\qquad \\textrm{human:operator}$ @ abc1234",
		"\\$5 $\\circ$ human:operator @ abc1234",
		"`$$` $\\qquad \\textrm{human:operator}$ @ abc1234",
		"`$` `` $\\qquad \\mathsf{human:operator}$ ` @ abc1234",
		"$\\lang \\rang$ human:operator @ abc1234 (see example-app\\lang x\\rang review)",
		"$ $y(\\quad{}human:operator$ @ abc1234",
	} {
		t.Run(strings.TrimSpace(runner[strings.IndexByte(runner, ' '):]), func(t *testing.T) {
			f := setupStamp(t, true)
			code := landRendered(t, f, restampHead, restampHead+restampRow("2026-10-14", runner))
			wantRenderRefusal(t, f, code, "cannot read the way the lint does", "line 7")
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
		// White space other than an ASCII space at the end of the cell, or white space and then
		// a character that renders as nothing, leaves the reading past the glyph lead as it is.
		for _, end := range []string{"\u00a0", "&#160;", "\u3000", "&#x3000;",
			" \u2063", "\u00a0\u2063", "&#160;&#8291;", "\u3000\u200b", " &#8203;"} {
			if cell := word + " example-app[bot]" + foldTail + end; !showsAppRun(cell) {
				t.Errorf("%q not read as App-run", cell)
			}
		}
		// What follows the lead is read as written, so a later word of the same shape, or later
		// digits that hold one, leaves the reading past the lead as it is.
		for _, later := range []string{" " + word, word, " :sparkles:", " 09:15:30", ", 23:59:59Z"} {
			if cell := word + " example-app[bot]" + foldTail + later; !showsAppRun(cell) {
				t.Errorf("%q not read as App-run", cell)
			}
			if cell := word + " example-app[bot]" + later + foldTail; !showsAppRun(cell) {
				t.Errorf("%q not read as App-run", cell)
			}
		}
		for _, cell := range []string{
			word + " human:operator @ abc1234",
			word + " human:operator @ abc1234 " + word,
			word + " human:operator @ abc1234 (rerun 09:15:30)",
			"x" + word + "x example-app[bot]" + foldTail,
			"x" + word + "x example-app[bot]" + foldTail + " " + word,
			"x" + word + "[abc] human:operator @ abc1234",
		} {
			if showsAppRun(cell) {
				t.Errorf("%q read as App-run", cell)
			}
		}
	}
}

// unnamedMathLeads are the TestShowsAppRunShownLead leads whose math span holds a control word
// the tool neither names nor reads as a character code.
var unnamedMathLeads = map[string]bool{
	"$\\bullet$":                     true,
	"$\\;$":                          true,
	"$\\cdot \\cdot$":                true,
	"$$\\ast$$":                      true,
	"($\\circ$)":                     true,
	"\u2713 $\\bullet$":              true,
	"$\\bullet$ $\\bullet$":          true,
	"$\\qquad \\textrm{y}$":          true,
	"$\\hphantom{y}$":                true,
	"$\\circ$ $":                     true,
	"$ $\\star$":                     true,
	"\\$ $\\star$":                   true,
	"&#x24; $\\star$":                true,
	"`` $ `` $\\star$":               true,
	"$ $\\qquad \\mathrm{y}$":        true,
	"`$` `` $\\qquad \\mathrm{y}$ `": true,
}

// regression: #4102
// TestShowsAppRunShownLead: a leading span a renderer can typeset as math as a symbol or blank
// space, a leading element a renderer hides by default, or a glyph word and a lone dollar sign
// with a later dollar sign in the cell, before an App login with white space between, and an
// App login after a line break or a block element (a disclosure element among them), where the
// text a reader sees starts a new line, are read as showing App-run text; before a human
// runner, with a letter or digit glued to the span, or after a word a reader sees, they are not,
// unless the lead's math span holds a control word the tool does not name, which fails closed.
func TestShowsAppRunShownLead(t *testing.T) {
	for _, lead := range []string{
		"$\\bullet$", "$\\;$", "$\\cdot \\cdot$", "$$\\ast$$", "$x$", "($\\circ$)", "\u2713 $\\bullet$",
		"$\\bullet$ $\\bullet$",
		"<details><summary>&#x2714;</summary>y</details>", "<details>y</details>",
		"<details open><summary>-</summary>y z</details>", "y <br>", "y<br/>", "<p>y</p>", "<div>y z</div>",
		"<details><summary>y</summary>z</details>",
		"$\\qquad \\textrm{y}$", "$\\hphantom{y}$", "<rp>y</rp>", "<span hidden>y z</span>", "<dialog>y</dialog>",
		":robot: $", ":robot:$", "$\\circ$ $",
		"$ $\\star$", "\\$ $\\star$", "&#x24; $\\star$", "`` $ `` $\\star$", "$ $\\qquad \\mathrm{y}$",
		"<small hidden><u>y</u> z</small>", "<abbr/hidden>y</abbr>", "<rp>(<rp>y</rp>) z</rp>",
		"<small data-n='>' hidden>y z</small>", "<em data-x='> y '></em>", "`$` `` $\\qquad \\mathrm{y}$ `",
	} {
		if cell := lead + " example-app[bot]" + foldTail; !showsAppRun(cell) {
			t.Errorf("%q not read as App-run", cell)
		}
		// A lead whose math span holds a control word the tool does not name fails closed, so
		// before a human runner it is read as showing App-run text too.
		if cell := lead + " human:operator @ abc1234"; showsAppRun(cell) != unnamedMathLeads[lead] {
			t.Errorf("%q read as App-run: %v, want %v", cell, !unnamedMathLeads[lead], unnamedMathLeads[lead])
		}
	}
	for _, cell := range []string{
		"y $\\textrm{example-app[bot]}$" + foldTail,
		"y \\$ $\\star$ example-app[bot]" + foldTail,
	} {
		if !showsAppRun(cell) {
			t.Errorf("%q not read as App-run (unnamed math word fails closed)", cell)
		}
	}
	for _, cell := range []string{
		"$x$y example-app[bot]" + foldTail,
		"y$x$ example-app[bot]" + foldTail,
		"$5 operator @ abc1234 (cost $6)",
		"<details><summary>y</summary>z example-app[bot]" + foldTail + "</details>",
		"y <b>z</b> example-app[bot]" + foldTail,
		"<rt>y</rt> example-app[bot]" + foldTail,
		"y <small hidden><u>z</u></small> example-app[bot]" + foldTail,
	} {
		if showsAppRun(cell) {
			t.Errorf("%q read as App-run", cell)
		}
	}
	// A dollar sign in the lead is read as written beside each math reading, so a later dollar
	// sign that a span could pair it with does not take the login out of the reading.
	for _, lead := range []string{":robot: $", ":robot:$", ":+1: $$", "\u2713 $", "$\\circ$ $"} {
		for _, later := range []string{" $", " $$", " (cost $5)", " $5 $6", "$"} {
			if cell := lead + " example-app[bot]" + foldTail + later; !showsAppRun(cell) {
				t.Errorf("%q not read as App-run", cell)
			}
		}
	}
	// Every trim of a shown text takes any white space, so a runner whose tag holds only login
	// characters stays human when white space other than an ASCII space leads it.
	for _, cell := range []string{
		"\u00a0release[draft] @ abc1234", "&#160;release[draft] @ abc1234", "&#x3000;release[draft] @ abc1234",
	} {
		if showsAppRun(cell) {
			t.Errorf("%q read as App-run", cell)
		}
	}
}

// regression: #4102
// TestShowsAppRunKeepsRefusalInContext: a cell that shows a reader App-run text still does with
// text before or after it that holds a `>` inside a quoted attribute value, an element with the
// hidden attribute, a run of `$` or a control word, and, for a cell with no backtick, backtick
// runs of different lengths or an escaped one. (A backtick run can close a code span a cell
// opens, whose text a renderer shows as code, so that context is left to cells without one.)
func TestShowsAppRunKeepsRefusalInContext(t *testing.T) {
	suffixes := []string{" <em data-x='>'>", " <i hidden>", " $", " $\\lang$"}
	prefixes := []string{"<em data-x='>'></em> ", "<i data-n='>'></i>"}
	for _, tc := range leadRunners {
		if !showsAppRun(tc.cell) {
			t.Errorf("%s: %q not read as App-run", tc.name, tc.cell)
			continue
		}
		sfx, pfx := suffixes, prefixes
		if !strings.Contains(tc.cell, "`") {
			sfx = append(sfx[:len(sfx):len(sfx)], " `` `", " \\`", "`")
			pfx = append(pfx[:len(pfx):len(pfx)], "`` ` ")
		}
		for _, x := range sfx {
			if cell := tc.cell + x; !showsAppRun(cell) {
				t.Errorf("%s: %q not read as App-run", tc.name, cell)
			}
		}
		for _, x := range pfx {
			if cell := x + tc.cell; !showsAppRun(cell) {
				t.Errorf("%s: %q not read as App-run", tc.name, cell)
			}
		}
	}
}

// regression: #4102
// TestShownTextsFailClosedPastCap: a text whose readings run past the limit is reported as
// showing what is asked for, never read with fewer readings, and the same human text under the
// tool's cap is not.
func TestShownTextsFailClosedPastCap(t *testing.T) {
	cell := strings.Repeat("<br>", 10) + strings.Repeat("$\\char120 x y$ ", 20) + " human:operator @ abc1234"
	never := func(string) bool { return false }
	if !eachShownTextWithin(cell, 1<<10, never) {
		t.Errorf("%q read within a 1 KiB limit", cell)
	}
	if eachShownTextWithin(cell, readingCap, never) || showsAppRun(cell) || showsAttribution(cell) {
		t.Errorf("%q refused under the cap", cell)
	}
	// With no line break or hidden element, the limit is reached among the math readings.
	math := strings.Repeat("$\\char120 x y$ ", 40) + " human:operator @ abc1234"
	if !eachShownTextWithin(math, 4<<10, never) {
		t.Errorf("%q read within a 4 KiB limit", math)
	}
	if eachShownTextWithin(math, readingCap, never) {
		t.Errorf("%q refused under the cap", math)
	}
}

// regression: #4102
// TestMathTextsCharacterCodes: a math text is read with each character code as its character
// and, beside that, with its control word read as one the tool does not name; a control word
// the tool does not name is read alone as an opening and as a closing bracket; and a bracket
// word's argument is read both shown and taken into the bracket.
func TestMathTextsCharacterCodes(t *testing.T) {
	for _, tc := range []struct{ math, want string }{
		{"x\\unicode{91}y", "x[y"},
		{"x\\unicode{91}y", "x91y"},
		{"x\\unicode{x5d}y", "x]y"},
		{"x\\unicode{0X5D}y", "x]y"},
		{"x\\unicode[a][b]{93}y", "x]y"},
		{"x\\unicode{98}y", "xby"},
		{"x\\char\"5Dy", "x]y"},
		{"x\\char'135y", "x]y"},
		{"x\\char93 y", "x]y"},
		{"x\\mathchar\"7162y", "xby"},
		{"a\\foo b", "a[ b"},
		{"a\\foo b", "a] b"},
		{"x\\foo{z}y\\bar w", "x[y]w"},
		{"x\\foo y\\bar{z}w", "x[y]w"},
		{"x\\foo{z}y\\bar{z}w", "x[y]w"},
		{"x\\foo[o]{z}y\\bar\"5Dw", "x[y]w"},
	} {
		found := false
		mathTexts(tc.math, func(m string) bool {
			found = found || m == tc.want
			return false
		})
		if !found {
			t.Errorf("%q not read as %q", tc.math, tc.want)
		}
	}
}

// regression: #4102
// TestMathTextsFailClosedOnUnnamedWord: a text read as math that holds a control word or symbol
// the tool neither names nor reads as a character code is reported as showing App-run text
// once its readings are taken, whatever add reports for them; one whose control words are all
// named or character codes is not.
func TestMathTextsFailClosedOnUnnamedWord(t *testing.T) {
	for _, tc := range []struct {
		math string
		want bool
	}{
		{"x\\foo y", true},
		{"x\\mathchoice{a}{b}{c}{d}y", true},
		{"b\\ell t", true},
		{"x\\(y", true},
		{"x&#92;foo y", true},
		{"x\\lbrack y\\rbrack", false},
		{"x\\unicode{91}y\\char93 z", false},
		{"base \\| tail", false},
		{"a_b^{c}", false},
	} {
		if got := mathTexts(tc.math, func(string) bool { return false }); got != tc.want {
			t.Errorf("mathTexts(%q) = %v, want %v", tc.math, got, tc.want)
		}
	}
}

// regression: #4102
// TestMathTextsBarIsNoBracket: an escaped bar, as a table cell writes a shell pipe, is read as
// a bar and never as a bracket, so a command cell piping one tool into another shows no tag.
func TestMathTextsBarIsNoBracket(t *testing.T) {
	bar := false
	mathTexts("base \\| tail -1) && test", func(m string) bool {
		if strings.ContainsAny(m, "[]") {
			t.Errorf("escaped bar read as a bracket: %q", m)
		}
		bar = bar || strings.Contains(m, "base | tail")
		return false
	})
	if !bar {
		t.Error("escaped bar not read as a bar")
	}
	cell := "TAG=$(ls registry.example/base \\| tail -1) && test -n \"$TAG\" && ls \"registry.example/base:$TAG\" >/dev/null"
	if showsAppRun(cell) {
		t.Errorf("command cell %q read as App-run", cell)
	}
}

// regression: #4102
// TestHiddenReadingsStopAtBudget: a text holding many hidden elements builds no more of its
// hidden readings once the limit is spent, whether the text itself spends it or the first few
// of its hidden readings do.
func TestHiddenReadingsStopAtBudget(t *testing.T) {
	cell := strings.Repeat("<i hidden>yyyyyy</i>", 2000) + " human:operator @ abc1234"
	for _, limit := range []int{1, 4 * len(cell)} {
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		refused := eachShownTextWithin(cell, limit, func(string) bool { return false })
		runtime.ReadMemStats(&after)
		if !refused {
			t.Errorf("text of %d bytes read within a %d-byte limit", len(cell), limit)
		}
		if got := after.TotalAlloc - before.TotalAlloc; got > 32<<20 {
			t.Errorf("%d bytes allocated reading a text of %d bytes within a %d-byte limit", got, len(cell), limit)
		}
	}
}
