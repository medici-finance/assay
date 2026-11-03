package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Rendered-versus-read shapes (principalstamp.go): an Evidence landing the lint reads one way
// and a Markdown renderer shows another can put an App-run row on screen whose attribution the
// lint, and so this tool, never reads. Each shape below is refused before any write when the
// landing ADDS it; the same shape already standing on the branch is not this landing's.

const renderApp = "example-app[bot] @ abc1234"

// renderOther is an App-run Runner cell naming a recognised roster human other than the
// resolved principal: the stamp refuses it whenever it reads it, so a landing that shows it
// without the stamp reading it is the hazard each test plants.
const renderOther = renderApp + " (on-behalf-of human:colleague)"

const renderHuman = "| 4 | `d` | 0 | ok | 2026-10-14 | human:operator @ abc1234 |\n"

// landRendered lands content over base by direct write and returns the exit code.
func landRendered(t *testing.T, f *fakeForge, base, content string) int {
	t.Helper()
	f.setFile(stampBrief, base)
	root := rootWithFile(t, stampBrief, content)
	return run([]string{stampPublicRepo, "main", "--evidence-file", stampBrief, "--root", root})
}

// wantRenderRefusal asserts a refused landing that wrote nothing and whose audit detail holds
// every one of want.
func wantRenderRefusal(t *testing.T, f *fakeForge, code int, want ...string) {
	t.Helper()
	if code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d (refused); landed:\n%q", code, deskkit.ExitRefused, f.putContent)
	}
	if f.putCalls != 0 {
		t.Fatalf("a refused landing still wrote %d time(s):\n%q", f.putCalls, f.putContent)
	}
	got := lastAudit(t).Detail
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Fatalf("refusal detail lacks %q: %q", w, got)
		}
	}
}

// regression: #4075
// TestStampRefusesPipelessRow: a renderer reads a line inside a table as a row whether or not
// it begins with `|`, but the lint reads only `|`-led lines and forgets the table's header at
// the first line that is not one. An added line that renders as a row the lint skips (a row
// or a header or a delimiter row without its leading pipe), and an added `|`-led row read
// after such a line (where the lint has lost the header the renderer still uses), are refused.
func TestStampRefusesPipelessRow(t *testing.T) {
	cases := []struct{ name, added, line string }{
		{"row-without-leading-pipe", "2 | `b` | 0 | ok | 2026-10-14 | " + renderOther + " |\n", "line 7"},
		{"pipeless-table", "\nnote\n\nn | Runner\n---|---\n3 | " + renderOther + "\n", "line 10"},
		{"pipeless-delimiter-row", "\n| # | Command | Runner | Date |\n---|---|---|---\n| 3 | `c` | " + renderApp +
			" | 2026-10-14 |\n", "line 9"},
		{"row-after-pipeless-line", "\n| # | Runner | Date |\n|---|---|---|\nVerification note\n| 3 | " + renderOther +
			" | 2026-10-14 |\n", "line 11"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := setupStamp(t, true)
			code := landRendered(t, f, restampHead, restampHead+tc.added)
			wantRenderRefusal(t, f, code, "cannot read the way the lint does", tc.line, "table row")
		})
	}
}

// regression: #4075
// TestStampRefusesSecondHeading: the lint reads only the first `## Evidence` section, ending it
// at the next `## ` line. A heading a renderer shows as Evidence anywhere else in the landed
// body (a second `## Evidence`, one in another case or level, one a lone CR puts on its own
// rendered line, or one a --brief-path fragment carries into the brief) shows rows under it
// as Evidence that are never checked or stamped, so a landing that adds one is refused.
func TestStampRefusesSecondHeading(t *testing.T) {
	second := "\n| # | Runner |\n|---|---|\n| 3 | " + renderOther + " |\n"
	cases := []struct{ name, added, line string }{
		{"second-evidence-heading", "\n## Evidence\n" + second, "line 9"},
		{"other-case-and-level", "\n## Notes\n\n### evidence\n" + second, "line 11"},
		{"cr-hidden-heading", "\n## Notes\n\nsee below\r## Evidence\r" + strings.ReplaceAll(second, "\n", "\r") + "\n", "line 11"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := setupStamp(t, true)
			code := landRendered(t, f, restampHead, restampHead+renderHuman+tc.added)
			wantRenderRefusal(t, f, code, "renderer shows as Evidence", tc.line)
		})
	}
	t.Run("via-brief-path", func(t *testing.T) {
		f := setupStamp(t, true)
		f.setFile(stampBrief, restampHead+renderHuman)
		local := writeRepoFile(t, "row.md", strings.Replace(renderHuman, "| 4 |", "| 5 |", 1)+"\n## Evidence\n"+second)
		code := run([]string{stampPublicRepo, "main", "--evidence-file", local, "--brief-path", stampBrief})
		wantRenderRefusal(t, f, code, "renderer shows as Evidence")
	})
}

// regression: #4075
// TestStampRefusesRunnerMarkup: HTML or Markdown markup in an added App-run Runner cell
// changes what a renderer shows (a struck-through annotation, a title attribute, an entity
// or escape that renders as another character) from the text the lint reads, so the cell is
// refused. An App-run Runner cell is an App login, a commit and plain tags: none carries it.
func TestStampRefusesRunnerMarkup(t *testing.T) {
	cases := []struct{ name, runner, why string }{
		{"strikethrough-html", renderApp + " <s>(on-behalf-of human:operator)</s>", "markup character `<`"},
		{"title-attribute", `<span title="x">` + renderApp + " (on-behalf-of human:operator)</span>", "markup character `<`"},
		{"strikethrough-gfm", renderApp + " ~~(on-behalf-of human:operator)~~", "markup character `~`"},
		{"entity", renderApp + " (on-behalf-of human:&#111;perator)", "markup character `&`"},
		{"backslash-escape", renderApp + ` (on-behalf-of human:op\_erator)`, "markup character `\\`"},
		{"closing-angle", renderApp + " (on-behalf-of human:operator) >", "markup character `>`"},
		{"strong-emphasis", "**example-app[bot]** @ abc1234 (on-behalf-of human:colleague)", "markup character `*`"},
		{"underscore-emphasis", "_example-app[bot]_ @ abc1234 (on-behalf-of human:colleague)", "markup character `_`"},
		{"code-span", "`example-app[bot]` @ abc1234 (on-behalf-of human:colleague)", "markup character ```"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := setupStamp(t, true)
			code := landRendered(t, f, restampHead, restampHead+restampRow("2026-10-14", tc.runner))
			wantRenderRefusal(t, f, code, "cannot read the way the lint does", "line 7", tc.why)
		})
	}
}

// regression: #4075
// TestStampRefusesRunnerHeader: the lint takes the Runner column from the header cell that
// trims and lowercases to exactly `runner`, else the last cell. An added header whose cells a
// renderer shows differently (an invisible format or bidi character, a non-ASCII letter that
// looks like an ASCII one, markup around `Runner`, or two Runner columns) can make the column
// the reader takes as Runner not the one the lint reads, so it is refused.
func TestStampRefusesRunnerHeader(t *testing.T) {
	const human = "human:operator @ abc1234"
	cases := []struct{ name, header, row, why string }{
		{"bidi-isolated-runner", "| # | Command | Runner | \u2068Runner\u2069 |", "| 3 | `c` | " + human + " | " + renderOther + " |",
			"format character U+2068"},
		{"homoglyph-runner", "| # | Runn\u0435r | Command | Date |", "| 3 | " + renderOther + " | `c` | 2026-10-14 |",
			"non-ASCII letter or mark U+0435"},
		{"markup-runner", "| # | **Runner** | Command | Date |", "| 3 | " + renderOther + " | `c` | 2026-10-14 |", "renders as Runner"},
		{"two-runner-columns", "| # | Runner | Command | Runner |", "| 3 | " + renderOther + " | `c` | " + human + " |",
			"more than one Runner column"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := setupStamp(t, true)
			code := landRendered(t, f, restampHead, restampHead+"\n"+tc.header+"\n|---|---|---|---|\n"+tc.row+"\n")
			wantRenderRefusal(t, f, code, "cannot read the way the lint does", "header at line 8", tc.why)
		})
	}
}

// regression: #4075
// TestStampRenderShapesAllowed: the shapes the refusals above leave alone still land — a
// pipe-less note line ending a table, a pipe in prose after a blank line, an Evidence
// sub-heading of another title, a non-ASCII separator in a header cell, and each refused shape
// when it already stands on the branch.
func TestStampRenderShapesAllowed(t *testing.T) {
	cases := []struct{ name, base, added string }{
		{"note-line-ends-table", restampHead, renderHuman + "Verification-Attestation: example run=abc\n"},
		{"pipe-in-prose", restampHead, renderHuman + "\nran `grep x | wc -l`\n"},
		{"addendum-subheading", restampHead, renderHuman + "\n### Evidence addendum — re-run\n"},
		{"dot-in-header", restampHead, "\n| # | Date · Runner |\n|---|---|\n| 5 | human:operator @ abc1234 |\n"},
		{"pipeless-row-on-branch", restampHead + "x | y\n", "\n| # | Runner |\n|---|---|\n| 5 | human:operator @ abc1234 |\n"},
		{"second-heading-on-branch", restampHead + "\n## evidence\n", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := setupStamp(t, true)
			content := tc.base + tc.added
			if tc.name == "second-heading-on-branch" {
				content = restampHead + renderHuman + "\n## evidence\n"
			}
			if code := landRendered(t, f, tc.base, content); code != deskkit.ExitOK {
				t.Fatalf("exit = %d, want 0: %q", code, lastAudit(t).Detail)
			}
		})
	}
}
