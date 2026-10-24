package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// A reader takes a cell that opens with an App login as an App-run cell whatever character
// follows the login; the lint splits the cell at the first ASCII space or `(` and wants the
// token to end in `[bot]`. Every cell below opens, to a reader, with an App login and then the
// annotation of a recognised human other than the resolved principal, and is not App-run to
// the lint, so a landing that carries one is refused wherever the checks read it.

const foldTail = " @ abc1234 (on-behalf-of human:colleague)"

// foldedRunners are the Runner cells the checks must refuse. The name says what keeps the
// lint from reading the cell as App-run.
var foldedRunners = []struct{ name, cell string }{
	{"nbsp-raw", "example-app[bot] @ abc1234 (on-behalf-of human:colleague)"},
	{"nbsp-entity", "example-app[bot]&nbsp;@ abc1234 (on-behalf-of human:colleague)"},
	{"ideographic-space", "example-app[bot]　@ abc1234 (on-behalf-of human:colleague)"},
	{"zero-width-raw", "example-app[bot]​@ abc1234 (on-behalf-of human:colleague)"},
	{"zero-width-entity", "example-app[bot]&#8203;@ abc1234 (on-behalf-of human:colleague)"},
	{"zero-width-in-tag", "example-app[bo​t]" + foldTail},
	{"tab", "example-app[bot]\t@ abc1234 (on-behalf-of human:colleague)"},
	{"lookalike-letter", "example-app[bоt]" + foldTail},
	{"fullwidth-brackets", "example-app［bot］" + foldTail},
	{"punctuation-after", "example-app[bot]:" + foldTail},
	{"comma-after", "example-app[bot]," + foldTail},
	{"glued-after", "example-app[bot]@ abc1234 (on-behalf-of human:colleague)"},
	{"shortcut-escaped", "[example-app\\[bot\\]]" + foldTail},
	{"shortcut-entity", "[example-app&#91;bot&#93;]" + foldTail},
	{"shortcut-image", "![example-app\\[bot\\]]" + foldTail},
	{"shortcut-whole", "[example-app\\[bot\\]" + foldTail + "]"},
}

// regression: #4075
// TestStampRefusesFoldedRunner: the Runner column of an added row.
func TestStampRefusesFoldedRunner(t *testing.T) {
	for _, tc := range foldedRunners {
		t.Run(tc.name, func(t *testing.T) {
			f := setupStamp(t, true)
			code := landRendered(t, f, restampHead, restampHead+restampRow("2026-10-14", tc.cell)+
				"\n[example-app\\[bot\\]]: https://example.com\n[example-app&#91;bot&#93;]: https://example.com\n")
			wantRenderRefusal(t, f, code, "cannot read the way the lint does", "line 7")
		})
	}
}

// regression: #4075
// TestStampRefusesFoldedLayout: the same cells as a non-Runner cell of a headerless row, in a
// table outside the Evidence section, and in a brief with no Evidence section at all.
func TestStampRefusesFoldedLayout(t *testing.T) {
	for _, tc := range foldedRunners {
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

// regression: #4075
// TestStampFoldShapesAllowed: what the identity-region reading leaves alone still lands — Runner cells
// that carry `[bot]` or a bracket tag without opening with an App login, and a human name
// with a non-ASCII letter.
func TestStampFoldShapesAllowed(t *testing.T) {
	for _, runner := range []string{
		"human:operator @ abc1234 (via example-app[bot] review)",
		"human:operator @ abc1234 (model example[1m])",
		"José García @ abc1234",
		"human:operator  @ abc1234",
		"see [the run](https://example.com) @ abc1234",
		"[1m] @ abc1234",
	} {
		t.Run(runner, func(t *testing.T) {
			f := setupStamp(t, true)
			if code := landRendered(t, f, restampHead, restampHead+restampRow("2026-10-14", runner)); code != deskkit.ExitOK {
				t.Fatalf("exit = %d, want 0: %q", code, lastAudit(t).Detail)
			}
		})
	}
}

// regression: #4075
// TestStampRefusesFoldedNoteLine: a line that closes a table without a blank line shows as a
// one-cell row, and shows App-run or on-behalf-of text under the same reading.
func TestStampRefusesFoldedNoteLine(t *testing.T) {
	for _, tc := range foldedRunners {
		if tc.name == "tab" {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			f := setupStamp(t, true)
			// Without the on-behalf-of text, only the `[bot]` tag shows App-run text.
			note := strings.Replace(tc.cell, " (on-behalf-of human:colleague)", "", 1)
			code := landRendered(t, f, restampHead+renderHuman, restampHead+renderHuman+note+"\n")
			wantRenderRefusal(t, f, code, "closes a table without a blank line", "line 8")
		})
	}
}
