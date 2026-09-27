package main

import (
	"strings"
	"testing"
)

// forktest_test.go — table tests for the pure `### Fork test` block parser (forktest.go). No forge, no env, no CLI dispatch here; the integration behaviour
// (what `deskfile new` DOES with a parsed result) is forkgate_test.go.

// validForkTestBlock is a well-formed, two-option block with `caught-by: nothing` — an
// ordinary needs-decision filing that stays on the needs-decision label (no notice-lane
// relabel). Shared with blockerevidence_test.go, whose needs-decision fixtures must carry a
// well-formed fork-test block now that the fork-test gate runs before the evidence gate.
const validForkTestBlock = `### Fork test

option: A — keep the current default | works-because: it is a one-line revert if wrong | consequence: no behaviour change today
option: B — flip the default | works-because: the merge gate catches a wrong flip before it ships | consequence: every caller sees the new default next release
default: A
caught-by: nothing
ruled-check: searched the tracker for "the same question" → no prior ruling found
`

// onlyOneWorkableOptionBlock is the row-2 fixture: a port-opening question with a single
// counted option (the rejected alternative is prose, per the facts, not a second `option:`
// line) — otherwise structurally complete (default/caught-by/ruled-check all present and
// valid).
const onlyOneWorkableOptionBlock = `### Fork test

option: A — open the port | works-because: nothing else in the landed work can serve the request | consequence: the service becomes reachable
default: A
caught-by: draft-pr — #401
ruled-check: searched the tracker for the same routing question → no prior ruling on this port
`

func TestParseForkTestValidBlockIsStructuralAndWorkable(t *testing.T) {
	r := parseForkTest(validForkTestBlock)
	if !r.Found {
		t.Fatal("Found = false, want true")
	}
	if !r.Structural() {
		t.Fatalf("Structural() = false, want true; errors: %v", r.Errors)
	}
	if !r.Workable() {
		t.Fatalf("Workable() = false, want true (2 counted options); counted = %v", r.CountedOptions())
	}
	if len(r.Options) != 2 {
		t.Fatalf("len(Options) = %d, want 2", len(r.Options))
	}
	if r.Default != "A" {
		t.Fatalf("Default = %q, want A", r.Default)
	}
	if def := r.DefaultOption(); def == nil || def.Letter != "A" {
		t.Fatalf("DefaultOption() = %+v, want option A", def)
	}
	if r.CaughtByKind != caughtByNothing {
		t.Fatalf("CaughtByKind = %q, want %q", r.CaughtByKind, caughtByNothing)
	}
	if !r.HasRuledCheck || r.RuledCheckSearch == "" || r.RuledCheckResult == "" {
		t.Fatalf("ruled-check not parsed: %+v", r)
	}
}

func TestParseForkTestOneOptionIsStructuralButNotWorkable(t *testing.T) {
	r := parseForkTest(onlyOneWorkableOptionBlock)
	if !r.Structural() {
		t.Fatalf("Structural() = false, want true (the block itself is well-formed); errors: %v", r.Errors)
	}
	if r.Workable() {
		t.Fatal("Workable() = true, want false (only one counted option)")
	}
	if got := len(r.CountedOptions()); got != 1 {
		t.Fatalf("len(CountedOptions()) = %d, want 1", got)
	}
	if r.CaughtByKind != caughtByDraftPR || r.CaughtByDetail != "#401" {
		t.Fatalf("caught-by = %q/%q, want draft-pr/#401", r.CaughtByKind, r.CaughtByDetail)
	}
}

func TestParseForkTestNoBlockIsNotStructural(t *testing.T) {
	r := parseForkTest("just an ordinary body with no fork-test section at all")
	if r.Found {
		t.Fatal("Found = true, want false")
	}
	if r.Structural() {
		t.Fatal("Structural() = true, want false")
	}
	if len(r.Errors) == 0 {
		t.Fatal("Errors is empty, want at least one naming the missing block")
	}
}

func TestParseForkTestMissingLinesAreNamedIndividually(t *testing.T) {
	// Two options, no default/caught-by/ruled-check at all.
	body := "### Fork test\n\n" +
		"option: A — do X | works-because: reversible | consequence: low risk\n" +
		"option: B — do Y | works-because: matches precedent | consequence: slower\n"
	r := parseForkTest(body)
	if r.Structural() {
		t.Fatal("Structural() = true, want false (default/caught-by/ruled-check all missing)")
	}
	joined := strings.Join(r.Errors, " | ")
	for _, want := range []string{"default:", "caught-by:", "ruled-check:"} {
		if !strings.Contains(joined, want) {
			t.Errorf("Errors %q does not name the missing %q line", joined, want)
		}
	}
}

func TestParseForkTestDefaultNamingNoCountedOptionIsNotStructural(t *testing.T) {
	body := "### Fork test\n\n" +
		"option: A — do X | works-because: reversible | consequence: low risk\n" +
		"option: B — do Y (rejected, cannot actually work) |\n" + // uncounted: no works-because/consequence
		"default: B\n" +
		"caught-by: nothing\n" +
		"ruled-check: checked the tracker → nothing found\n"
	r := parseForkTest(body)
	if r.Structural() {
		t.Fatalf("Structural() = true, want false (default B is not counted); errors: %v", r.Errors)
	}
	if !strings.Contains(strings.Join(r.Errors, " "), "default: B") {
		t.Errorf("Errors does not name the bad default: %v", r.Errors)
	}
}

func TestParseForkTestDefaultNamingUnknownLetterIsNotStructural(t *testing.T) {
	body := "### Fork test\n\n" +
		"option: A — do X | works-because: reversible | consequence: low risk\n" +
		"option: B — do Y | works-because: matches precedent | consequence: slower\n" +
		"default: C\n" +
		"caught-by: nothing\n" +
		"ruled-check: checked the tracker → nothing found\n"
	r := parseForkTest(body)
	if r.Structural() {
		t.Fatal("Structural() = true, want false (default C names no parsed option)")
	}
}

func TestParseForkTestCaughtByAcceptsBareNothing(t *testing.T) {
	body := "### Fork test\n\n" +
		"option: A — do X | works-because: reversible | consequence: low risk\n" +
		"option: B — do Y | works-because: matches precedent | consequence: slower\n" +
		"default: A\n" +
		"caught-by: nothing\n" +
		"ruled-check: checked the tracker → nothing found\n"
	r := parseForkTest(body)
	if !r.Structural() {
		t.Fatalf("Structural() = false, want true; errors: %v", r.Errors)
	}
	if r.CaughtByKind != caughtByNothing {
		t.Fatalf("CaughtByKind = %q, want nothing", r.CaughtByKind)
	}
}

func TestParseForkTestCaughtByInvalidValueIsNotStructural(t *testing.T) {
	body := "### Fork test\n\n" +
		"option: A — do X | works-because: reversible | consequence: low risk\n" +
		"option: B — do Y | works-because: matches precedent | consequence: slower\n" +
		"default: A\n" +
		"caught-by: a vibe\n" +
		"ruled-check: checked the tracker → nothing found\n"
	r := parseForkTest(body)
	if r.Structural() {
		t.Fatal("Structural() = true, want false (caught-by value is not in the closed set)")
	}
}

func TestParseForkTestAcceptsAsciiArrowInRuledCheck(t *testing.T) {
	body := "### Fork test\n\n" +
		"option: A — do X | works-because: reversible | consequence: low risk\n" +
		"option: B — do Y | works-because: matches precedent | consequence: slower\n" +
		"default: A\n" +
		"caught-by: nothing\n" +
		"ruled-check: checked the tracker -> nothing found\n"
	r := parseForkTest(body)
	if !r.HasRuledCheck || r.RuledCheckResult != "nothing found" {
		t.Fatalf("ruled-check (ascii arrow) not parsed: %+v", r)
	}
}

func TestParseForkTestHeadingAnyLevelAccepted(t *testing.T) {
	body := strings.Replace(validForkTestBlock, "### Fork test", "## Fork test", 1)
	r := parseForkTest(body)
	if !r.Found {
		t.Fatal("Found = false, want true for a ## heading level")
	}
}

func TestParseForkTestSectionEndsAtNextHeading(t *testing.T) {
	body := validForkTestBlock + "\n### Something else\n\noption: Z — a stray line from another section | works-because: x | consequence: y\n"
	r := parseForkTest(body)
	if len(r.Options) != 2 {
		t.Fatalf("len(Options) = %d, want 2 (the stray option: line under the NEXT heading must not be absorbed)", len(r.Options))
	}
}

// TestParseForkTestDefaultAcceptsTrailingText — `default: A — keep the current default` (the
// letter plus text, the same shape `option:` and `caught-by:` lines take) is a default
// naming A, never "no `default:` line found".
func TestParseForkTestDefaultAcceptsTrailingText(t *testing.T) {
	body := strings.Replace(validForkTestBlock, "default: A\n", "default: A — keep the current default\n", 1)
	r := parseForkTest(body)
	if !r.Structural() {
		t.Fatalf("Structural() = false, want true; errors: %v", r.Errors)
	}
	if r.Default != "A" {
		t.Fatalf("Default = %q, want A", r.Default)
	}
}

// TestParseForkTestMalformedDefaultIsNamedNotAbsent — a `default:` line whose value is not a
// bare option letter is reported as malformed, not as missing.
func TestParseForkTestMalformedDefaultIsNamedNotAbsent(t *testing.T) {
	body := strings.Replace(validForkTestBlock, "default: A\n", "default: the first one\n", 1)
	r := parseForkTest(body)
	if r.Structural() {
		t.Fatal("Structural() = true, want false (default names no option letter)")
	}
	joined := strings.Join(r.Errors, "\n")
	if strings.Contains(joined, "no `default:` line found") {
		t.Errorf("a present-but-malformed default is reported as absent: %v", r.Errors)
	}
}

// TestParseForkTestDuplicateOptionLettersRefused — two `option:` lines with the same letter
// are one option padded into two; the block is not structural and the duplicate is named.
func TestParseForkTestDuplicateOptionLettersRefused(t *testing.T) {
	body := strings.Replace(validForkTestBlock, "option: B —", "option: a —", 1)
	r := parseForkTest(body)
	if r.Structural() {
		t.Fatalf("Structural() = true, want false (option letter A used twice); options: %+v", r.Options)
	}
	if !strings.Contains(strings.Join(r.Errors, "\n"), "duplicate") {
		t.Errorf("errors do not name the duplicate letter: %v", r.Errors)
	}
}

// round6TrailingSubjectBlock is validForkTestBlock (no `subject:` line of its own) plus a
// single trailer appended after it, with no heading in between — the shape the old,
// unbounded extractForkSection absorbed wholesale.
const round6TrailingSubjectBlockBase = validForkTestBlock

// TestParseForkTestSectionBoundedAgainstTrailingContent — security review sec-1688-S1 (round
// 6, review 5332050856): extractForkSection used to run to the next ATX heading or EOF, so a
// `subject:`-shaped line anywhere in the trailing body — fenced, indented, in ordinary prose,
// or past a Markdown setext heading, none of which the old scan recognised as ending anything
// — was still read as the block's declared subject when the block itself declared none. The
// section now ends at the first heading, fence, or blank line after the block's last
// recognised grammar key line, so none of the four shapes below is ever picked up.
//
// FAIL-FIRST: with extractForkSection reverted to the round-5 behaviour (next ATX heading or
// EOF) and these four probes kept, every one showed HasSubject=true with the trailing text
// read as the subject:
//
//	--- FAIL: TestParseForkTestSectionBoundedAgainstTrailingContent/fenced-subject-outside-block
//	    HasSubject = true (Subject = "Re: typo in the README"), want false
//	--- FAIL: TestParseForkTestSectionBoundedAgainstTrailingContent/indented-subject-outside-block
//	    HasSubject = true (Subject = "Re: typo in the README"), want false
//	--- FAIL: TestParseForkTestSectionBoundedAgainstTrailingContent/setext-heading-then-subject
//	    HasSubject = true (Subject = "fix a typo"), want false
//	--- FAIL: TestParseForkTestSectionBoundedAgainstTrailingContent/prose-subject-outside-block
//	    HasSubject = true (Subject = "Re: typo in the README"), want false
func TestParseForkTestSectionBoundedAgainstTrailingContent(t *testing.T) {
	for _, tc := range []struct{ name, trailer string }{
		{
			// the reviewer's own case: a fenced excerpt quoting an unrelated mail header,
			// directly after the block (one blank line, no other prose in between).
			name:    "fenced-subject-outside-block",
			trailer: "\n\n```\nSubject: Re: typo in the README\n```\n",
		},
		{
			// a code-block-style (4-space) indented line: the grammar's own prefix classes
			// (`[ \t>*_-]*`) tolerate leading whitespace for bullet/quote formatting and
			// cannot themselves distinguish this from a legitimate indented subject line.
			name:    "indented-subject-outside-block",
			trailer: "\n\n    Subject: Re: typo in the README\n",
		},
		{
			// a Markdown setext heading (text + an underline of `=`/`-`): anyHeadingRe only
			// recognises ATX (`#`-prefixed) headings, so the old scan never treated this as
			// ending anything either.
			name:    "setext-heading-then-subject",
			trailer: "\n\nNotes\n-----\n\nsubject: fix a typo\n",
		},
		{
			// ordinary prose, no code fence or heading involved at all.
			name:    "prose-subject-outside-block",
			trailer: "\n\nA general note about the request.\n\nSubject: Re: typo in the README\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := parseForkTest(round6TrailingSubjectBlockBase + tc.trailer)
			if r.HasSubject {
				t.Errorf("HasSubject = true (Subject = %q), want false — the trailing text is outside the bounded fork-test section", r.Subject)
			}
		})
	}
}

// TestParseForkTestLoneQuotedSubjectDoesNotAdmit — sec-1688-S5: the round-5 `>`-quote
// exclusion in parseForkTest had no test that could fail when disarmed, because the only
// existing quoted-subject fixture (quoted-trailing-subject, forkgate_round5_test.go) also
// carries a real unquoted subject line, so the exactly-one-line count rule refuses it whether
// or not the quote check exists. This fixture has NO other subject line: the block's sole
// `subject:` line is `>`-quoted, alone.
//
// FAIL-FIRST (mutation M28b — the quote check `!strings.Contains(m[1], ">")` replaced with
// `true`, so a quoted line counts as unquoted; the count rule kept): HasSubject flips to true
// and Subject becomes "fix a typo in the README".
func TestParseForkTestLoneQuotedSubjectDoesNotAdmit(t *testing.T) {
	body := strings.Replace(validForkTestBlock, "### Fork test\n\n",
		"### Fork test\n\n> subject: fix a typo in the README\n\n", 1)
	r := parseForkTest(body)
	if r.HasSubject {
		t.Errorf("HasSubject = true (Subject = %q), want false — a lone `>`-quoted subject line must admit nothing", r.Subject)
	}
}
