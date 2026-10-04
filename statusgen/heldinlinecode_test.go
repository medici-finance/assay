package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Tests for #2100: the held/could-not-check scan (unroutedHeldLine, behind
// verifyPassHeldContradiction and closeVerifyHeldRefusal) excludes inline code
// spans, the same as fenced code, blockquotes and struck text. An inline code
// span is quoted tool output, not a verifier disposition.
//
// This narrows a fail-closed control, so most cases below are the refusals
// that must survive it: the word outside a span, an unterminated backtick, a
// span that would have to cross a table cell, backtick runs of different
// lengths, a span holding nothing but the marker word, and a span that would
// join a negation cue to a marker.

// heldEv wraps one Result cell in a two-row Evidence table carrying a strict
// **VERIFY: PASS** marker — the shape the verify-gate card reads.
func heldEv(result string) string {
	return "**VERIFY: PASS** (model) — row 1 green.\n\n" +
		"| # | Command | Exit | Result | Date | Runner |\n" +
		"|---|---------|------|--------|------|--------|\n" +
		"| 1 | `go test ./...` | 0 | ok | 2026-10-03 | fixture-verifier |\n" +
		"| 2 | `grantcheck --all` | 0 | " + result + " | 2026-10-03 | fixture-verifier |\n"
}

// heldScanCase is one Evidence body and whether the held scan must refuse it.
type heldScanCase struct {
	name     string
	evidence string
	wantHeld bool
}

func runHeldScanCases(t *testing.T, cases []heldScanCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			held, why := verifyPassHeldContradiction(tc.evidence)
			if held != tc.wantHeld {
				t.Errorf("verifyPassHeldContradiction held=%v, want %v (why=%q)", held, tc.wantHeld, why)
			}
			// closeVerifyHeldRefusal runs the same scan with no strict-marker
			// precondition; it must agree on every case.
			err := closeVerifyHeldRefusal("ic/01", "verified", tc.evidence)
			if (err != nil) != tc.wantHeld {
				t.Errorf("closeVerifyHeldRefusal err=%v, want refusal=%v", err, tc.wantHeld)
			}
		})
	}
}

// TestHeldScanInlineCodeExcluded: Evidence whose only HELD/could-not-check
// sits inside an inline code span does not contradict the PASS.
func TestHeldScanInlineCodeExcluded(t *testing.T) {
	runHeldScanCases(t, []heldScanCase{
		{"single-backtick quoted checker line",
			heldEv("ok — checker printed `grant-scope — no record is held under the data grant`"), false},
		{"could-not-check inside a span",
			heldEv("ok — `probe: could-not-check reported for an unrelated tenant` is expected output"), false},
		{"double-backtick span holding a backtick",
			heldEv("ok — ``log: record `x` is held for audit`` quoted from the run"), false},
		{"triple-backtick inline span",
			heldEv("ok — ```HELD by upstream lock, released``` quoted"), false},
		{"two spans on one line",
			heldEv("ok — `held: 0` and `could-not-check: none` from the summary"), false},
		{"span on a prose line",
			"**VERIFY: PASS** — all rows green; the grant checker's `no record is held` line is informational.\n", false},
	})
}

// TestHeldScanInlineCodeCard drives the same narrowing through the verify-gate
// card itself (verifyIssues Path B): a gate:human implemented brief whose only
// "held" is quoted in an inline code span opens its card, and the twin with the
// same word outside the span does not.
func TestHeldScanInlineCodeCard(t *testing.T) {
	const orig = "| 2 | `go test ./integration/...` | — | HELD — no runner online | 2026-07-10 | opus-verifier |"
	card := func(t *testing.T, row string) bool {
		t.Helper()
		root := loadCHRoot(t)
		p := filepath.Join(root, "docs/streams/ch/brief-04-implemented-held.md")
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), orig) {
			t.Fatalf("fixture drifted: ch/04 no longer carries %q", orig)
		}
		if err := os.WriteFile(p, []byte(strings.Replace(string(b), orig, row, 1)), 0o644); err != nil {
			t.Fatal(err)
		}
		streams, _, err := loadStreams(root)
		if err != nil {
			t.Fatal(err)
		}
		for _, iss := range verifyIssues(root, streams, map[string]bool{}) {
			if iss.Brief == "ch/04" {
				return true
			}
		}
		return false
	}
	inCode := "| 2 | `go test ./integration/...` | 0 | ok — checker printed `grant-scope — no record is held under the data grant` | 2026-07-10 | opus-verifier |"
	if !card(t, inCode) {
		t.Errorf("ch/04 whose only \"held\" is inside an inline code span must open its verify-gate card")
	}
	outside := "| 2 | `go test ./integration/...` | 0 | ok — no record is held under the data grant | 2026-07-10 | opus-verifier |"
	if card(t, outside) {
		t.Errorf("ch/04 with the same \"held\" outside any code span must still suppress its card")
	}
}

// TestHeldScanWordOutsideSpan: the same word outside the span still refuses,
// including on a line that also carries an excluded span.
func TestHeldScanWordOutsideSpan(t *testing.T) {
	runHeldScanCases(t, []heldScanCase{
		{"bare word, no span", heldEv("ok — no record is held under the data grant"), true},
		{"span quoted, then a live HELD after it",
			heldEv("checker printed `no record is held`; row HELD — no runner online"), true},
		{"live HELD before a span", heldEv("HELD — runner offline, see `grantcheck --all`"), true},
		{"live could-not-check between spans",
			heldEv("`step 1` could-not-check `step 2`"), true},
		{"HELD as its own cell after a code cell",
			"**VERIFY: PASS**\n\n| # | Command | Result |\n|---|---|---|\n| 3 | `go test ./x` | HELD |\n", true},
	})
}

// TestHeldScanUnterminatedTick: an unterminated backtick strips nothing — not
// the rest of its line, and never across a line break.
func TestHeldScanUnterminatedTick(t *testing.T) {
	runHeldScanCases(t, []heldScanCase{
		{"lone opener before HELD", heldEv("output was `partial — row HELD, no runner online"), true},
		{"closed span then a stray opener", heldEv("`ok` then `HELD — no runner online"), true},
		{"opener does not pair with the next line",
			"**VERIFY: PASS**\n\nrow 2 printed `partial\nrow 3 HELD — no runner online` end\n", true},
		{"escaped backtick cannot open", heldEv("printed \\`HELD — no runner` online"), true},
	})
}

// TestHeldScanTickRunMismatch: backtick runs of different lengths never pair.
func TestHeldScanTickRunMismatch(t *testing.T) {
	runHeldScanCases(t, []heldScanCase{
		{"double opener, single closer", heldEv("``HELD — no runner online` x"), true},
		{"single opener, double closer", heldEv("`HELD — no runner online`` x"), true},
		{"triple opener, double closer", heldEv("```HELD — no runner online`` x"), true},
		// A run of the right length after a wrong one still closes: CommonMark
		// pairs equal-length runs only, skipping the double in between.
		{"equal runs pair across a longer one", heldEv("`quoted `` held text` ok"), false},
	})
}

// TestHeldScanSpanVsTableRow: a span cannot swallow a real table-row
// disposition. GFM splits a table row on unescaped pipes before reading inline
// code, so a span never crosses one; the scan applies that on every line,
// which is stricter (never looser) than CommonMark outside a table.
func TestHeldScanSpanVsTableRow(t *testing.T) {
	runHeldScanCases(t, []heldScanCase{
		{"span would cross the cell holding HELD",
			"**VERIFY: PASS**\n\n| # | Command | Result | Note |\n|---|---|---|---|\n| 3 | `go test ./x | HELD | `ok` |\n", true},
		{"span would cross two cells",
			"**VERIFY: PASS**\n\n| # | Command | Result |\n|---|---|---|\n| 3 | `go test | HELD | x` |\n", true},
		{"escaped pipe inside a span stays one span",
			heldEv("ok — `grep held \\| wc -l` printed 0"), false},
		{"pipe in a prose span is a stated residual: still refuses",
			"**VERIFY: PASS** — the checker's `grep held | wc -l` printed 0.\n", true},
		// In prose the first span DOES cross the pipe, so the next backtick
		// closes it; pairing the cells on their own would mask the HELD.
		{"prose span across a pipe, then HELD",
			"**VERIFY: PASS** — ran `x | y` HELD — no runner online `z`.\n", true},
	})
}

// TestHeldScanBareTokenSpan: a span holding nothing but the marker word is the
// verifier's own status token in code formatting, not quoted output — kept.
func TestHeldScanBareTokenSpan(t *testing.T) {
	runHeldScanCases(t, []heldScanCase{
		{"`HELD` as a Result cell", heldEv("`HELD`"), true},
		{"`could-not-check` as a Result cell", heldEv("`could-not-check` — no runner"), true},
		{"padded and emphasised", heldEv("`` **HELD:** ``"), true},
	})
}

// TestHeldScanSpanNoCueJoin: removing a span never joins a negation cue to a
// marker, and a cue inside a span never negates a marker outside it.
// The cases sit at line start, where a cue with nothing before it excuses, so
// a span that vanished into whitespace would read as a negation.
func TestHeldScanSpanNoCueJoin(t *testing.T) {
	const pass = "**VERIFY: PASS**\n\n"
	runHeldScanCases(t, []heldScanCase{
		{"cue, span, marker", pass + "no `see log` HELD\n", true},
		{"zero cue, span, marker", pass + "- 0 `rows` HELD\n", true},
		{"cue inside the span", pass + "`no` HELD\n", true},
		{"span before a cue", pass + "`row 3` no HELD\n", true},
		{"span before a zero cue", pass + "`summary:` 0 HELD\n", true},
	})
}

// TestHeldScanPriorHygieneKept: the fence, blockquote, strike, negation and
// routing cases behave as before.
func TestHeldScanPriorHygieneKept(t *testing.T) {
	runHeldScanCases(t, []heldScanCase{
		{"HELD inside a fence", "**VERIFY: PASS**\n\n```\nrow 2 HELD\n```\n", false},
		{"HELD inside a tilde fence", "**VERIFY: PASS**\n\n~~~\nrow 2 HELD\n~~~\n", false},
		{"HELD in a blockquote", "**VERIFY: PASS**\n\n> row 2 HELD — quoted from the old run\n", false},
		{"HELD struck through", "**VERIFY: PASS**\n\n~~row 2 HELD — no runner~~ re-run green\n", false},
		{"negated count", "**VERIFY: PASS** — summary: 0 HELD, 2 PASS.\n", false},
		{"routed with the reference in code",
			heldEv("HELD — deferred to follow-up `verify-integrity/05`"), false},
		{"live HELD after a fence closes",
			"**VERIFY: PASS**\n\n```\nok\n```\nrow 2 HELD — no runner online\n", true},
		{"strike markers inside code do not strike",
			heldEv("`~~` HELD — no runner online `~~`"), true},
	})
}

// TestHeldScanSpanAcrossLines: a span may run across the lines of one
// paragraph (CommonMark joins them before reading code spans), so a backtick
// on a later line can CLOSE a span rather than open one. A line that may begin
// inside such a span masks nothing, and neither does any later line of the
// paragraph; a blank line ends the paragraph and the state with it.
func TestHeldScanSpanAcrossLines(t *testing.T) {
	const pass = "**VERIFY: PASS**\n\n"
	runHeldScanCases(t, []heldScanCase{
		{"closer of a span opened on the line above",
			pass + "row 2 printed `partial\noutput` then HELD — no runner online `x`\n", true},
		{"state carries through the paragraph",
			pass + "a `b\nc` d `e\nf` HELD — no runner online `g`\n", true},
		{"table row after an open prose line",
			pass + "printed `partial\n| 3 | x` HELD — no runner `ok` |\n", true},
		{"blank line ends the paragraph",
			pass + "printed `partial\n\nchecker printed `no record is held` ok\n", false},
	})
}

// TestHeldScanAngleConstruct: an autolink or raw HTML tag outranks a code span
// when it starts first, so a backtick inside one is literal. The scan does not
// parse HTML: it masks nothing from a "<" that could open one, and nothing on
// any later line (an HTML block can run across blank lines).
func TestHeldScanAngleConstruct(t *testing.T) {
	const pass = "**VERIFY: PASS**\n\n"
	runHeldScanCases(t, []heldScanCase{
		{"backtick inside an autolink",
			pass + "see <https://x.example/a`b> then HELD — no runner online `c`\n", true},
		{"backtick inside a raw HTML tag",
			pass + "<span title=\"`\">HELD — no runner online</span> `x`\n", true},
		{"HTML block across a blank line",
			pass + "<pre>\n\n`checker: row HELD` printed\n</pre>\n", true},
		{"angle bracket inside a span still masks",
			heldEv("ok — `checker <tenant>: no record is held` printed"), false},
		{"a less-than that opens nothing",
			heldEv("ok — 3 < 5 and `no record is held` printed"), false},
		{"one-line HTML comment right above a table",
			"**VERIFY: PASS**\n\n<!-- contract comment -->\n| 2 | `grantcheck` | ok — `no record is held` |\n", false},
		{"multi-line HTML comment ends at its marker",
			"**VERIFY: PASS**\n\n<!-- contract\ncomment -->\n\n| 2 | `grantcheck` | ok — `no record is held` |\n", false},
	})
}

// TestHeldScanStrikeNeverWidens: a struck span is honoured only where the
// line reads as struck both with and without its code masked, so masking a
// "~" inside code never forms a strike the unmasked line did not have.
func TestHeldScanStrikeNeverWidens(t *testing.T) {
	const pass = "**VERIFY: PASS**\n\n"
	runHeldScanCases(t, []heldScanCase{
		{"tilde in code no longer breaks a strike",
			pass + "~~ `~` HELD — no runner online ~~\n", true},
		{"plain strike still strikes",
			pass + "~~row 2 HELD — no runner~~ `rerun` green\n", false},
	})
}

// TestHeldScanLinkConstruct: CommonMark reads an inline link's destination
// and title, and a full reference's label, when it reaches "](" or "][", so a
// backtick inside them is literal and cannot pair with a later one. The scan
// skips such a tail whole only when it closes on the line with nothing in it
// that could change where it ends or hold a backtick; any other "](" or "]["
// masks nothing from there and marks the paragraph open (a title can wrap).
func TestHeldScanLinkConstruct(t *testing.T) {
	const pass = "**VERIFY: PASS**\n\n"
	runHeldScanCases(t, []heldScanCase{
		{"backtick in a link destination",
			pass + "See [log](/runs/a`b) row 3 HELD `z`\n", true},
		{"backtick in a link title",
			pass + "See [log](/runs \"t`\") row 3 HELD `z`\n", true},
		{"backtick in an image destination",
			pass + "See ![shot](/i/a`b.png) row 3 HELD `z`\n", true},
		{"backtick in a pointy destination",
			pass + "See [log](</runs/a`b>) row 3 HELD `z`\n", true},
		{"backtick in a parenthesised destination",
			pass + "See [log](/runs/(a`b)) row 3 HELD `z`\n", true},
		{"backtick in a parenthesised title",
			pass + "See [log](/runs (t`)) row 3 HELD `z`\n", true},
		// Each case below puts a ")" inside the tail ahead of its real
		// closer, then a backtick: a tail cut at its first ")" would pair
		// that backtick with a later one and mask the HELD.
		{"paren pair in a destination, then a backtick",
			pass + "See [log](/runs/(a)b`c) row 3 HELD `z`\n", true},
		{"paren in a double-quoted title, then a backtick",
			pass + "See [log](/runs \"t)`\") row 3 HELD `z`\n", true},
		{"paren in a single-quoted title, then a backtick",
			pass + "See [log](/runs 't)`') row 3 HELD `z`\n", true},
		{"escaped paren in a parenthesised title, then a backtick",
			pass + "See [log](/runs (t\\)`)) row 3 HELD `z`\n", true},
		{"escaped paren in a destination, then a backtick",
			pass + "See [log](/runs/a\\)b`c) row 3 HELD `z`\n", true},
		{"paren in a pointy destination, then a backtick",
			pass + "See [log](</a)b`c>) row 3 HELD `z`\n", true},
		{"backtick in a tail that is not a link",
			pass + "See [log](a b`c) d` row 3 HELD `z`\n", true},
		{"title wraps onto the next line",
			pass + "See [log](/runs \"first\nsecond`b\") row 3 HELD `z`\n", true},
		{"link in a table cell",
			heldEv("see [log](/runs/a`b) row 3 HELD `z`"), true},
		{"backtick in a full reference label",
			pass + "See [log][r`x] row 3 HELD `z`\n\n[r`x]: https://x.example/runs\n", true},
		{"plain link before a span still masks",
			heldEv("ok — [run](https://x.example/runs/7) printed `no record is held`"), false},
		{"plain full reference before a span still masks",
			pass + "ok — [run][r] printed `no record is held`\n\n[r]: https://x.example/runs/7\n", false},
		{"shortcut reference: the span wins",
			pass + "ok — [checker `no record is held`] printed\n", false},
	})
}

// TestHeldScanStopLeavesTicks: when masking stops early at a "<", a backtick
// or a link tail after the stop is never examined, so it may open a span (or a
// title) that a later line closes. The paragraph is marked open.
func TestHeldScanStopLeavesTicks(t *testing.T) {
	const pass = "**VERIFY: PASS**\n\n"
	runHeldScanCases(t, []heldScanCase{
		{"backtick after a closed comment",
			pass + "Ran it <!-- note --> `open\nclose` row 3 HELD `z`\n", true},
		{"wrapping title after a closed comment",
			pass + "Ran it <!-- note --> [log](/runs \"t\nx`\") row 3 HELD `z`\n", true},
	})
}

// TestHeldScanPseudoFence: a line the scan toggles a fence on, but that
// CommonMark reads as paragraph text (a backtick fence whose info string
// holds a backtick), never reaches the masker, so its backticks are invisible
// to the paragraph state. Such a line marks the paragraph open.
func TestHeldScanPseudoFence(t *testing.T) {
	const pass = "**VERIFY: PASS**\n\n"
	runHeldScanCases(t, []heldScanCase{
		{"backtick in a backtick fence's info string",
			pass + "```a`\n```b`\nclose` row 3 HELD `z`\n", true},
		{"valid backtick fence leaves masking on",
			pass + "```sh\nrow HELD\n```\nchecker printed `no record is held`\n", false},
		{"fence indented four spaces inside a paragraph",
			pass + "text\n    ~~~\n`open\n    ~~~\nclose` row 3 HELD `z`\n", true},
		{"backtick in a tilde fence's info string is valid",
			pass + "~~~a`\nrow HELD\n~~~\nchecker printed `no record is held`\n", false},
	})
}

// TestHeldScanBlankLine: CommonMark treats a line as blank only when it
// holds spaces and tabs alone. A line of other whitespace continues the
// paragraph, and with it a span left open above.
func TestHeldScanBlankLine(t *testing.T) {
	const pass = "**VERIFY: PASS**\n\n"
	runHeldScanCases(t, []heldScanCase{
		{"no-break-space line",
			pass + "printed `partial\n \nclose` row 3 HELD — no runner `z`\n", true},
		{"ideographic-space line",
			pass + "printed `partial\n　\nclose` row 3 HELD — no runner `z`\n", true},
		{"form-feed line",
			pass + "printed `partial\n\f\nclose` row 3 HELD — no runner `z`\n", true},
		{"spaces-and-tab line is blank",
			pass + "printed `partial\n \t \nchecker printed `no record is held` ok\n", false},
		{"CRLF blank line is blank",
			pass + "printed `partial\r\n\r\nchecker printed `no record is held` ok\r\n", false},
	})
}

// TestHeldScanExtendedLink: GitHub recognises a bare URL (a scheme then "://")
// or a "www." domain as an extended autolink when it reaches the trigger, so a
// backtick later in the same whitespace-delimited run is a literal part of the
// link and cannot open a span. Such a backtick masks nothing from there and
// marks the paragraph open. A backtick that opens a span BEFORE the trigger
// still wins, as in GFM.
func TestHeldScanExtendedLink(t *testing.T) {
	const pass = "**VERIFY: PASS**\n\n"
	runHeldScanCases(t, []heldScanCase{
		{"backtick in a bare https URL",
			pass + "See https://x.example/a`b row 3 HELD `z`\n", true},
		{"backtick in a bare http URL",
			pass + "See http://x.example/`b row 3 HELD `z`\n", true},
		{"backtick in a bare ftp URL",
			pass + "See ftp://x.example/`b row 3 HELD `z`\n", true},
		{"backtick in a www domain",
			pass + "See www.x.example/a`b row 3 HELD `z`\n", true},
		{"backtick in a www domain after a paren",
			pass + "See (www.x.example/`b) row 3 HELD `z`\n", true},
		{"upper-case scheme",
			pass + "See HTTPS://X.example/`b row 3 HELD `z`\n", true},
		{"URL runs through a pipe in prose",
			pass + "See https://x.example/a|`b row 3 HELD `z`\n", true},
		{"backtick in a bare URL in a table cell",
			heldEv("see https://x.example/a`b row 3 HELD `z`"), true},
		{"backtick in a mailto URI (fail-closed choice)",
			pass + "See mailto:ops@x.example`b row 3 HELD `z`\n", true},
		{"URL then a span after a space still masks",
			heldEv("ok — see https://x.example/runs/7 then `no record is held` printed"), false},
		{"URL inside a span still masks",
			heldEv("ok — `curl https://x.example/runs` printed `no record is held`"), false},
	})
}

// TestHeldScanBareCR: a carriage return not followed by a line feed is a line
// ending in CommonMark, so a line holding one before its last byte is several
// lines to a renderer: a blank line or a heading inside it ends the paragraph.
// Such a line masks nothing and leaves the paragraph open. A trailing "\r"
// (a CRLF ending) is not a bare CR.
func TestHeldScanBareCR(t *testing.T) {
	const pass = "**VERIFY: PASS**\n\n"
	runHeldScanCases(t, []heldScanCase{
		{"bare-CR blank line",
			pass + "ok `a\r\rrow 3 HELD — no runner `\n", true},
		{"bare-CR space-only blank line",
			pass + "ok `a\r \rrow 3 HELD — no runner `\n", true},
		{"bare-CR line opens a heading",
			pass + "ok `a\r# row 3 HELD — no runner `\n", true},
		{"bare-CR paragraph left open",
			pass + "p `a\r\rq` r\nclose` row 3 HELD `z`\n", true},
		{"CRLF line ending is not a bare CR",
			pass + "checker printed `no record is held` ok\r\nsecond line\r\n", false},
	})
}

// TestHeldScanLinkDefTitle: a link reference definition's title is not
// rendered as text, so a backtick inside it pairs with nothing. A line that
// may be a definition masks nothing and leaves the paragraph open (a title can
// wrap onto the next line).
func TestHeldScanLinkDefTitle(t *testing.T) {
	const pass = "**VERIFY: PASS**\n\n"
	runHeldScanCases(t, []heldScanCase{
		{"backticks in a definition title",
			pass + "[x]: /u \"a ` row 3 HELD ` b\"\n", true},
		{"definition title wraps onto the next line",
			pass + "[x]: /u \"a `\nrow 3 HELD ` b\"\n", true},
		{"definition-like line that is paragraph text",
			pass + "[x]: /u \"a ` b\" junk\nclose` row 3 HELD `z`\n", true},
		{"definition then a blank line still masks",
			pass + "[r]: https://x.example/runs/7\n\nok — `no record is held` printed\n", false},
	})
}
