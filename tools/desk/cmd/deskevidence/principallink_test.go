package main

import (
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Link and image syntax, and closed comments, read the way a renderer shows them: a renderer
// shows a link's text, an image's alt text or nothing, and an autolink's address, where the
// lint reads the brackets and destination as written; and a renderer hides a closed comment
// outside a code span. Every cell, heading and line a landing adds is checked under each of
// those readings, and an App-run Runner cell holding link syntax is refused outright.

// linkedLogin is renderApp's App login split by an empty link, so only the reduced reading
// shows the login.
const linkedLogin = "example-app[bo[](x)t] @ abc1234"

// regression: #4075
// TestStampRefusesLinkedRunner: a Runner cell the lint reads as no App runner that a renderer
// shows as App-run once its link, image or autolink syntax is reduced to the text it shows.
func TestStampRefusesLinkedRunner(t *testing.T) {
	const tail = " @ abc1234 (on-behalf-of human:colleague)"
	cases := []struct{ name, runner, extra string }{
		{"leading-empty-link", "[](x)" + renderOther, ""},
		{"link-wrapped-login", "[example-app[bot]](https://example.com)" + tail, ""},
		{"titled-link", "[example-app[bot]](<https://example.com> \"t\")" + tail, ""},
		{"reference-link", "[example-app[bot]][r]" + tail, "\n[r]: https://example.com\n"},
		{"collapsed-reference", "[example-app[bot]][]" + tail, "\n[example-app[bot]]: https://example.com\n"},
		{"leading-empty-image", "![](x)" + renderOther, ""},
		{"image-before-login", "![note](x) " + renderOther, ""},
		{"image-alt", "![example-app[bot]](x)" + tail, ""},
		{"autolink", "<ab:example-app[bot]>" + tail, ""},
		{"split-login", linkedLogin + " (on-behalf-of human:colleague)", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := setupStamp(t, true)
			code := landRendered(t, f, restampHead, restampHead+restampRow("2026-10-14", tc.runner)+tc.extra)
			wantRenderRefusal(t, f, code, "cannot read the way the lint does", "line 7")
		})
	}
}

// regression: #4075
// TestStampRefusesLinkedAppCell: an App-run Runner cell holding link, image or autolink
// syntax shows a reader other text than the lint reads, so it is refused before any stamp.
func TestStampRefusesLinkedAppCell(t *testing.T) {
	for _, runner := range []string{
		renderApp + " [colleague](https://example.com)",
		renderApp + " ![x](y)",
		renderApp + " [colleague][r]",
	} {
		t.Run(runner, func(t *testing.T) {
			f := setupStamp(t, true)
			code := landRendered(t, f, restampHead, restampHead+restampRow("2026-10-14", runner))
			wantRenderRefusal(t, f, code, "line 7", "Markdown link or image syntax")
		})
	}
}

// regression: #4075
// TestStampRefusesLinkedLayout: a non-Runner cell, or a line outside the section the lint
// reads, that shows App-run text only once link syntax is reduced or a closed comment hidden.
func TestStampRefusesLinkedLayout(t *testing.T) {
	appHeader := "\n| # | Runner (App) | Date | Checked by |\n|---|---|---|---|\n"
	cases := []struct{ name, base, added, want, line string }{
		{"headerless-linked-cell", "# Brief\n\n## Evidence\n", "| 3 | [](x)" + renderOther + " | " + humanRunner + " |\n",
			"cell 2 shows as an App-run", "line 4"},
		{"headed-linked-cell", restampHead, appHeader + "| 3 | ![](x)" + renderOther + " | 2026-10-14 | " + humanRunner + " |\n",
			"cell 2 shows as an App-run", "line 10"},
		{"hidden-comment-cell", restampHead, appHeader + "| 3 | <!-- note --> " + renderOther + " | 2026-10-14 | " +
			humanRunner + " |\n", "cell 2 shows as an App-run", "line 10"},
		{"multibyte-comment-before-cell", restampHead, appHeader + "| 3 <!-- — — --> | `<!--` " + renderOther +
			" `-->` | 2026-10-14 | " + humanRunner + " |\n", "cell 2 shows as an App-run", "line 10"},
		{"multibyte-comment-before-split-cell", restampHead, appHeader + "| 3 <!-- — — --> | x<!-- y -->example-app[bot] | " +
			"2026-10-14 | " + humanRunner + " |\n", "cell 2 shows as an App-run", "line 10"},
		{"multibyte-comment-before-unclosed-tag", restampHead, appHeader + "| 3 <!-- — — --> | <!-- example-app[b -->ot] | " +
			"2026-10-14 | " + humanRunner + " |\n", "cell 2 shows as an App-run", "line 10"},
		{"linked-row-after-section", restampHead + renderHuman, "\n## Notes\n\n| 3 | " + linkedLogin + " |\n",
			"outside the Evidence section the lint reads", "line 11"},
		{"comment-split-row-after-section", restampHead + renderHuman, "\n## Notes\n\n| 3 | example-app[b<!-- x -->ot] @ abc1234 |\n",
			"outside the Evidence section the lint reads", "line 11"},
		{"linked-row-no-section", "# Brief\n\n## Results\n", "\n| # | Runner |\n|---|---|\n| 3 | " + linkedLogin + " |\n",
			"outside the Evidence section the lint reads", "line 7"},
		{"linked-heading", restampHead, renderHuman + "\n## Notes\n\n### Evid[e](#x)nce\n\n| # | Runner |\n|---|---|\n| 3 | " +
			renderOther + " |\n", "renderer shows as Evidence", "line 11"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := setupStamp(t, true)
			code := landRendered(t, f, tc.base, tc.base+tc.added)
			wantRenderRefusal(t, f, code, tc.want, tc.line)
		})
	}
}

// regression: #4075
// TestStampLinkShapesAllowed: what the link reading leaves alone still lands — a model tag in
// brackets in an App-run Runner cell (stamped), and a link in a cell that shows no App run.
func TestStampLinkShapesAllowed(t *testing.T) {
	cases := []struct{ name, added string }{
		{"bracket-tag-app-cell", restampRow("2026-10-14", renderApp+" (model example[1m])")},
		{"link-in-command-cell", "| 2 | [docs](https://example.com) | 0 | ok | 2026-10-14 | " + humanRunner + " |\n"},
		{"link-in-note", renderHuman + "\nSee [the run](https://example.com).\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := setupStamp(t, true)
			if code := landRendered(t, f, restampHead, restampHead+tc.added); code != deskkit.ExitOK {
				t.Fatalf("exit = %d, want 0: %q", code, lastAudit(t).Detail)
			}
		})
	}
}

// regression: #4075
// TestStampCodeSpanTagReading: a code span's backticks are not part of the text a reader takes
// for a login or tag. A tag a code span closes or wraps (`example-app` plus `[bot]` in code) is
// read as App-run, and a bracketed word that a span follows or sits inside is not read as a
// wider tag beside a character outside the login alphabet, so a plain human cell still lands.
func TestStampCodeSpanTagReading(t *testing.T) {
	for _, runner := range []string{
		"example-app`[bot]`",
		"example-app`[bot]` @ abc1234",
		"op`x`[abc]",
	} {
		t.Run("refused/"+runner, func(t *testing.T) {
			f := setupStamp(t, true)
			code := landRendered(t, f, restampHead, restampHead+restampRow("2026-10-14", runner))
			wantRenderRefusal(t, f, code, "line 7")
		})
	}
	for _, runner := range []string{
		"op[abcd]`x`",
		"op`x[abcd]` @ abc1234",
	} {
		t.Run("allowed/"+runner, func(t *testing.T) {
			f := setupStamp(t, true)
			if code := landRendered(t, f, restampHead, restampHead+restampRow("2026-10-14", runner)); code != deskkit.ExitOK {
				t.Fatalf("exit = %d, want 0: %q", code, lastAudit(t).Detail)
			}
		})
	}
}
