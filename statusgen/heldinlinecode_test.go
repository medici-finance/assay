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
