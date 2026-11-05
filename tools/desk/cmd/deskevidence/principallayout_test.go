package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The landed Evidence section must be a table both readings agree on (evidenceLayoutProblem,
// unreadEvidenceAddition, outsideEvidenceProblem): every line a landing adds is a blank line,
// a `|`-led row under a header whose delimiter row a GFM renderer accepts, or a note line
// neither reading takes as a row; only the cell the lint reads as Runner may show as an
// App-run Runner cell; and no row is added where a renderer shows Evidence the lint never
// reads. Each case lands over restampHead (or the base it names) and is refused, with nothing
// written.

// appRow is an added data row of restampHead's table whose Runner cell is runner.
func appRow(n, runner string) string {
	return "| " + n + " | `b` | 0 | ok | 2026-10-14 | " + runner + " |\n"
}

const humanRunner = "human:operator @ abc1234"

// regression: #4075
// TestStampRefusesLayoutRows: added lines in the Evidence section that the lint and a
// renderer read as different tables, or as a table and not a table.
func TestStampRefusesLayoutRows(t *testing.T) {
	cases := []struct{ name, base, added, line, why string }{
		{"row-before-blank-separator", restampHead, appRow("2", renderOther) + "| | | | | | |\n" + renderHuman,
			"line 7", "new table header"},
		{"row-before-dash-separator", restampHead, renderHuman + appRow("2", renderOther) + "|---|---|---|---|---|---|\n",
			"line 8", "new table header"},
		{"second-delimiter-row", restampHead, "|:-|-|-|-|-|-|\n" + renderHuman, "line 7", "stands inside a table"},
		{"separator-under-no-header", restampHead, renderHuman + "\n|---|---|\n", "line 9", "no table header"},
		{"row-under-moved-header", restampHead + renderHuman + "|---|---|---|---|---|---|\n",
			"| 5 | `e` | 0 | ok | 2026-10-14 | " + renderOther + " | " + humanRunner + " |\n", "line 9", "different headers"},
		{"wider-than-header", restampHead, "\n| # | Runner (App) | Date |\n|---|---|---|\n| 3 | " + renderOther +
			" | 2026-10-14 | " + humanRunner + " |\n", "line 10", "4 cells under a 3-column header"},
		{"app-cell-not-runner", restampHead, "\n| # | Runner (App) | Date | Checked by |\n|---|---|---|---|\n| 3 | " +
			renderOther + " | 2026-10-14 | " + humanRunner + " |\n", "line 10", "cell 2 shows as an App-run"},
		{"headerless-app-cell", "# Brief\n\n## Evidence\n", "| 3 | " + renderOther + " | " + humanRunner + " |\n",
			"line 4", "cell 2 shows as an App-run"},
		{"delimiter-without-dashes", restampHead, "\n| # | Runner | Date |\n| : | : | : |\n| 3 | " + humanRunner +
			" | 2026-10-14 |\n", "line 9", "delimiter row of the 3-column header"},
		{"delimiter-cell-count", restampHead, "\n| # | Runner | Date |\n|---|---|\n| 3 | " + humanRunner +
			" | 2026-10-14 |\n", "line 9", "delimiter row of the 3-column header"},
		{"note-line-attribution", restampHead, renderHuman + "Checked by " + renderOther + "\n", "line 8", "one-cell row"},
		{"comment-shares-line", restampHead, appRow("5", humanRunner+" <!--") + appRow("6", renderOther) + "-->\n",
			"line 7", "crosses a line break shares the line"},
		{"block-quoted-table", restampHead, renderHuman + "\n> | # | Runner |\n> |---|---|\n> | 3 | " + renderOther + " |\n",
			"line 9", "inside a block quote"},
		{"raw-html-table", restampHead, renderHuman + "\n<table><tr><td>" + renderOther + "</td></tr></table>\n",
			"line 9", "raw HTML table"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := setupStamp(t, true)
			code := landRendered(t, f, tc.base, tc.base+tc.added)
			wantRenderRefusal(t, f, code, "cannot read the way the lint does", tc.line, tc.why)
		})
	}
}

// regression: #4075
// TestStampRefusesShownComment: a Runner cell the lint reads as no App runner, because a
// closed comment removes its text, but that a renderer shows as App-run (the comment markers
// in code spans), is refused on the cell's bytes as landed.
func TestStampRefusesShownComment(t *testing.T) {
	f := setupStamp(t, true)
	code := landRendered(t, f, restampHead, restampHead+appRow("2", "`<!--` "+renderOther+" `-->`"))
	wantRenderRefusal(t, f, code, "cannot read the way the lint does", "line 7", "markup character ```")
}

// regression: #4075
// TestStampRefusesShownHeading: a heading that shows a reader `Evidence` once entities are
// decoded or tags read, one whose letters include a non-ASCII homoglyph, and rows added under
// such a heading already on the branch.
func TestStampRefusesShownHeading(t *testing.T) {
	second := "\n| # | Runner |\n|---|---|\n| 3 | " + renderOther + " |\n"
	withNotes := restampHead + renderHuman + "\n## Notes\n\n"
	cases := []struct{ name, base, content, want, line string }{
		{"entity-heading", restampHead, withNotes + "### Evid&#101;nce\n" + second, "renderer shows as Evidence", "line 11"},
		{"html-heading", restampHead, withNotes + "<h3>Evidence</h3>\n" + second, "renderer shows as Evidence", "line 11"},
		{"homoglyph-heading", restampHead, withNotes + "### Evidеnce\n" + second, "non-ASCII letter or mark U+0435", "line 11"},
		{"rows-under-branch-heading", withNotes + "### Evidence\n", withNotes + "### Evidence\n" + second,
			"adds line 13 under it", "line 11"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := setupStamp(t, true)
			code := landRendered(t, f, tc.base, tc.content)
			wantRenderRefusal(t, f, code, tc.want, tc.line)
		})
	}
}

// regression: #4075
// TestStampHeaderOnBranchRows: a header already on the branch is still checked when the
// landing adds rows under it, since those rows are read under it.
func TestStampHeaderOnBranchRows(t *testing.T) {
	base := restampHead + renderHuman + "\n| # | **Runner** | Command | Date |\n|---|---|---|---|\n"
	f := setupStamp(t, true)
	code := landRendered(t, f, base, base+"| 3 | "+renderOther+" | `c` | 2026-10-14 |\n")
	wantRenderRefusal(t, f, code, "cannot read the way the lint does", "header at line 9", "renders as Runner")
}

// regression: #4075
// TestStampRefusesOutsideRows: an App-run row, or a raw HTML table (one between backtick runs
// of different lengths among them), added outside the one section the lint reads, whether or
// not the brief has such a section.
func TestStampRefusesOutsideRows(t *testing.T) {
	cases := []struct{ name, base, added, line string }{
		{"no-evidence-section", "# Brief\n\n## Results\n", "\n| # | Runner |\n|---|---|\n| 3 | " + renderOther + " |\n", "line 7"},
		{"row-after-section", restampHead + renderHuman, "\n## Notes\n\n| 3 | " + renderOther + " |\n", "line 11"},
		{"html-after-section", restampHead + renderHuman, "\n## Notes\n\n<table>\n<tr><td>" + renderOther + "</td></tr>\n</table>\n",
			"line 11"},
		// Backtick runs of different lengths open no code span, so the tag between them is markup.
		{"html-between-unequal-backticks", restampHead + renderHuman, "\n## Notes\n\nA `` <table><tr><td>" + renderOther +
			"</td></tr></table> `\n", "line 11"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := setupStamp(t, true)
			code := landRendered(t, f, tc.base, tc.base+tc.added)
			wantRenderRefusal(t, f, code, "outside the Evidence section the lint reads", tc.line)
		})
	}
}

// regression: #4075
// TestStampLayoutShapesAllowed: what the layout check leaves alone still lands — a plain row
// under no header, prose naming a runner after a blank line, rows under an Evidence
// sub-heading inside the section the lint reads, and an escaped pipe in a cell.
func TestStampLayoutShapesAllowed(t *testing.T) {
	cases := []struct{ name, base, added string }{
		{"headerless-plain-row", "# Brief\n\n## Evidence\n", "| 1 | `true` | 0 | ok | 2026-10-06 | " + humanRunner + " |\n"},
		{"prose-attribution", restampHead, renderHuman + "\nRun by " + renderApp + " (on-behalf-of human:operator).\n"},
		{"subheading-in-section", restampHead + renderHuman + "\n### Evidence\n", "\n| # | Runner |\n|---|---|\n| 5 | " +
			humanRunner + " |\n"},
		{"escaped-pipe", restampHead, "| 2 | `grep -c 'a\\|b'` | 0 | ok | 2026-10-14 | " + humanRunner + " |\n"},
		{"fenced-heading", restampHead + renderHuman + "\n## Notes\n", "\n```\n## Evidence\n```\n"},
		{"fenced-row-outside", restampHead + renderHuman + "\n## Notes\n", "\n```\n| 3 | " + renderOther + " |\n```\n"},
		{"code-span-tag-outside", restampHead + renderHuman + "\n## Notes\n", "\nA `<table>` tag is shown as code here.\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := setupStamp(t, true)
			if code := landRendered(t, f, tc.base, tc.base+tc.added); code != deskkit.ExitOK {
				t.Fatalf("exit = %d, want 0: %q", code, lastAudit(t).Detail)
			}
		})
	}
}

// attestedHead is an Evidence section ending in a table and the pipe-less attestation line a
// verifier run writes directly under it.
const attestedHead = restampHead + renderHuman + "Verification-Attestation: example#1 run=abc source=def brief=b.md\n"

// regression: #4075
// TestStampAppendAfterAttest: a table appended after an attestation line lands, whether it
// repeats the table's header or brings its own: the merge starts a fresh table after a blank
// line, and a repeat of the table's own header reads the same to both readings even without
// one. Its own, different header directly under the attestation line is refused, with the fix.
func TestStampAppendAfterAttest(t *testing.T) {
	const appRunRow = "| 5 | `e` | 0 | ok | 2026-10-14 | " + renderApp + " |\n"
	same := "| # | Command | Exit | Result | Date | Runner |\n|---|---|---|---|---|---|\n" + appRunRow
	// Date sits immediately left of Runner, as the attribution preflight requires of an
	// execution row, so only the header differs from the base's.
	own := "| # | Command | Result | Exit | Date | Runner |\n|---|---|---|---|---|---|\n" +
		"| 5 | `e` | ok | 0 | 2026-10-14 | " + renderApp + " |\n"
	for _, tc := range []struct{ name, content string }{
		{"merged-same-header", mergedEvidence(t, attestedHead, same)},
		{"merged-own-header", mergedEvidence(t, attestedHead, own)},
		{"direct-same-header", attestedHead + same},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setupStamp(t, true)
			if code := landRendered(t, f, attestedHead, tc.content); code != deskkit.ExitOK {
				t.Fatalf("exit = %d, want 0: %q", code, lastAudit(t).Detail)
			}
			if !strings.Contains(f.putContent, renderApp+" (on-behalf-of human:") {
				t.Fatalf("the appended App-run row was not stamped:\n%s", f.putContent)
			}
		})
	}
	t.Run("direct-own-header", func(t *testing.T) {
		f := setupStamp(t, true)
		code := landRendered(t, f, attestedHead, attestedHead+own)
		wantRenderRefusal(t, f, code, "cannot read the way the lint does", "line 9", "no blank line separates it")
	})
}

// regression: #4075
// TestStampRowsAfterAttest: rows appended directly under an attestation line, with no header
// of their own, land and are stamped when they are as wide as the table's header and its
// Runner column is the last: both readings take the row's last cell as its Runner cell. A row
// of another width, or under a header whose Runner column is not the last, is still refused.
func TestStampRowsAfterAttest(t *testing.T) {
	t.Run("same-width-runner-last", func(t *testing.T) {
		f := setupStamp(t, true)
		if code := landRendered(t, f, attestedHead, attestedHead+appRow("5", renderApp)); code != deskkit.ExitOK {
			t.Fatalf("exit = %d, want 0: %q", code, lastAudit(t).Detail)
		}
		if !strings.Contains(f.putContent, renderApp+" (on-behalf-of human:") {
			t.Fatalf("the appended App-run row was not stamped:\n%s", f.putContent)
		}
	})
	runnerFirst := "# Brief\n\n## Evidence\n\n| # | Runner | Date |\n|---|---|---|\n| 4 | " + humanRunner +
		" | 2026-10-14 |\nVerification-Attestation: example#1 run=abc source=def brief=b.md\n"
	runnerless := "# Brief\n\n## Evidence\n\n| # | Checked by |\n|---|---|\n| 4 | " + humanRunner +
		" |\nVerification-Attestation: example#1 run=abc source=def brief=b.md\n"
	cases := []struct{ name, base, added, line string }{
		{"narrower-row-runnerless", runnerless, "| " + renderApp + " |\n", "line 9"},
		{"wider-row-runnerless", runnerless, "| 5 | " + humanRunner + " | " + renderApp + " |\n", "line 9"},
		{"narrower-row", attestedHead, "| 5 | `e` | 0 | ok | " + renderApp + " |\n", "line 9"},
		{"wider-row", attestedHead, "| 5 | `e` | 0 | ok | 2026-10-14 | " + humanRunner + " | " + renderApp + " |\n", "line 9"},
		{"runner-not-last", runnerFirst, "| 5 | " + humanRunner + " | " + renderApp + " |\n", "line 9"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := setupStamp(t, true)
			code := landRendered(t, f, tc.base, tc.base+tc.added)
			wantRenderRefusal(t, f, code, "cannot read the way the lint does", tc.line, "dropped that header")
		})
	}
}

// mergedEvidence is base with block appended the way a brief-path landing appends it.
func mergedEvidence(t *testing.T, base, block string) string {
	t.Helper()
	merged, _ := mergeEvidenceContent([]byte(base), []byte(block))
	if !strings.Contains(string(merged), "brief=b.md\n\n| #") {
		t.Fatalf("the merge did not start the appended table after a blank line:\n%s", merged)
	}
	return string(merged)
}
