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
// — was still read as the block's declared subject when the block itself declared none.
//
// Round 7 replaced the per-shape boundary markers (heading/fence/blank-run) with a single
// rule: the block is the CONTIGUOUS run of key lines (forkKeyLineRe) starting at the first one
// after the heading, and the run ends at the first line that does not match — a fence
// delimiter, an indented line, a setext underline, and ordinary prose all fail that match the
// same way a blank line does, so none of the four shapes below needs its own boundary marker
// any more (see the file comment in forktest.go). All four probes are kept unchanged as
// regression pins.
//
// FAIL-FIRST (round 6, against the round-5 behaviour of running to the next ATX heading or
// EOF): every one of the four probes below showed HasSubject=true with the trailing text
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
// `subject:`-shaped line is `>`-quoted, alone.
//
// Round 7: the dedicated quote-check field in forkSubjectLineRe is gone — a `>`-quoted line is
// now excluded because forkKeyLineRe requires column zero and a lowercase key name with no
// decoration at all, so a line opening with `>` was never a key line to begin with. This test
// still pins the SAME outcome (a lone quoted subject-shaped line admits nothing), now as a
// consequence of the grammar rather than a dedicated exclusion; the round-7 mutation that
// would re-open this hole is loosening forkKeyLineRe to re-admit a `>`/bullet/blank-space
// prefix (see the mutation table in the PR body).
//
// FAIL-FIRST (round 5, mutation M28b — the old quote check `!strings.Contains(m[1], ">")`
// replaced with `true`, so a quoted line counted as unquoted; the count rule kept): HasSubject
// flipped to true and Subject became "fix a typo in the README".
func TestParseForkTestLoneQuotedSubjectDoesNotAdmit(t *testing.T) {
	body := strings.Replace(validForkTestBlock, "### Fork test\n\n",
		"### Fork test\n\n> subject: fix a typo in the README\n\n", 1)
	r := parseForkTest(body)
	if r.HasSubject {
		t.Errorf("HasSubject = true (Subject = %q), want false — a lone `>`-quoted subject line must admit nothing", r.Subject)
	}
}

// --- round 7: the strict grammar (correctness re-review cor-1688-C11/C12, security re-review
// sec-1688-S1/S5, and the withheld review-notes#169 variants) --------------------------------
//
// Rounds 1-6 each closed one more way trailing or embedded content could be read as the
// block's declared subject by teaching extractForkSection one more boundary marker. Round 7
// replaces the whole "scan forward for a boundary marker" design with three rules that need no
// further enumeration — see the file comment in forktest.go. The tests below pin each rule
// directly, rather than one more named shape.

// TestParseForkTestHeadingNotFollowedDirectlyIsMalformed — cor-1688-C11: a `### Fork test`
// heading followed by a blank line and then ordinary prose, before the first key line, is a
// MALFORMED block (the heading was found; no block was located after it) — never a silently
// empty section that just happens to be missing every field.
//
// FAIL-FIRST: at the pre-round-7 head, this exact shape parsed 2 options with no errors (the
// section ran to the next heading/fence/blank-with-no-following-key-line, which absorbed the
// intro sentence as ordinary section text the grammar simply ignored); at head f6a4e8358 (the
// round-6 fix, still present when this round started), the SAME shape returned Errors naming
// every field missing (no `option:`, `default:`, `caught-by:`, `ruled-check:` lines found)
// instead of naming the actual problem (prose before the first key line) — see the correctness
// re-review's cor-1688-C11 finding, item 1.
func TestParseForkTestHeadingNotFollowedDirectlyIsMalformed(t *testing.T) {
	body := "### Fork test\n\n" +
		"A sentence of context before the real block starts.\n\n" +
		"option: A — do X | works-because: reversible | consequence: low risk\n" +
		"option: B — do Y | works-because: matches precedent | consequence: slower\n" +
		"default: A\n" +
		"caught-by: nothing\n" +
		"ruled-check: checked the tracker → nothing found\n"
	r := parseForkTest(body)
	if !r.Found {
		t.Fatal("Found = false, want true — the heading itself was present")
	}
	if r.Structural() {
		t.Fatalf("Structural() = true, want false (prose sits between the heading and the first key line); errors: %v", r.Errors)
	}
	if len(r.Options) != 0 {
		t.Errorf("Options = %+v, want none read — the intro prose makes the whole block unlocatable, not merely incomplete", r.Options)
	}
	joined := strings.Join(r.Errors, " | ")
	if !strings.Contains(joined, "followed directly") {
		t.Errorf("Errors %q does not name the actual problem (heading not followed directly by the block)", joined)
	}
}

// TestParseForkTestHeadingWithNothingAfterIsMalformed — the degenerate case of C11: the
// heading (plus any number of blank lines) has NOTHING after it at all before EOF.
func TestParseForkTestHeadingWithNothingAfterIsMalformed(t *testing.T) {
	r := parseForkTest("### Fork test\n\n\n")
	if !r.Found {
		t.Fatal("Found = false, want true")
	}
	if r.Structural() {
		t.Fatal("Structural() = true, want false (nothing follows the heading)")
	}
}

// TestParseForkTestIgnoresHeadingInsideFencedExample — the sibling occurrence security review
// 5332392502 named (present since before this round, not itself a new finding, but closed as a
// direct consequence of round 7's design): extractForkSection used to take the FIRST
// `### Fork test` heading in the body, including one quoted inside an earlier fenced example.
// Stripping every fenced block from the WHOLE body before the heading search runs (round 7,
// stripFencedBlocks) means a quoted example — heading, its own decoy `subject:`/`option:`
// lines, all inside the fence — is gone before the real heading is ever searched for, so the
// REAL block later in the body is the one parsed.
//
// FAIL-FIRST (mutation: stripFencedBlocks replaced with `return body`, a no-op): the fenced
// example's own truncated heading is found FIRST, its two contiguous lines
// (`subject:`/`option:`) are read as the whole section, and the real block afterwards is never
// reached — Structural() flips to false and Options drops from 2 to 0.
func TestParseForkTestIgnoresHeadingInsideFencedExample(t *testing.T) {
	decoy := "```\n### Fork test\n\nsubject: fix a typo in the README\noption: X — a decoy option | works-because: y | consequence: z\n```\n\n"
	body := decoy + validForkTestBlock
	r := parseForkTest(body)
	if !r.Structural() {
		t.Fatalf("Structural() = false, want true (the real block, after the fenced decoy, must still parse); errors: %v", r.Errors)
	}
	if len(r.Options) != 2 {
		t.Fatalf("len(Options) = %d, want 2 (the real block's two options, not the fenced decoy's one)", len(r.Options))
	}
	if r.HasSubject {
		t.Errorf("HasSubject = true (Subject = %q), want false — the decoy's `subject:` line is inside a fence and must never be read", r.Subject)
	}
}

// TestParseForkTestSubjectHiddenInHTMLCommentDoesNotAdmit — the withheld variant from
// review-notes#169: a `subject:` line hidden inside an HTML comment BETWEEN two of the block's
// own real key lines, invisible in the rendered issue. stripHTMLComments removes the whole
// comment (delimiters and content) from the body before extractForkSection ever runs, so the
// hidden line can never be read as the declared subject.
//
// The comment sits between `default:` and `caught-by:`, so stripping it also removes the
// newline that separated them from the newline that separated the comment from what follows,
// leaving a blank line in their place — the block loses structural completeness as a SAFE side
// effect (forcing the audited `--force-new --reason` bypass rather than an invisible bypass of
// the gate itself). Either way, the load-bearing assertion is the same: the hidden text is
// never read as a declared subject.
//
// FAIL-FIRST (mutation: stripHTMLComments replaced with `return body`, a no-op): HasSubject
// flips to true and Subject becomes the hidden text.
func TestParseForkTestSubjectHiddenInHTMLCommentDoesNotAdmit(t *testing.T) {
	body := "### Fork test\n\n" +
		"option: A — keep the current default | works-because: it is a one-line revert if wrong | consequence: no behaviour change today\n" +
		"option: B — flip the default | works-because: the merge gate catches a wrong flip before it ships | consequence: every caller sees the new default next release\n" +
		"default: A\n" +
		"<!--\nsubject: a hidden line that must never be read as declared\n-->\n" +
		"caught-by: nothing\n" +
		"ruled-check: searched the tracker for \"the same question\" → no prior ruling found\n"
	r := parseForkTest(body)
	if r.HasSubject {
		t.Errorf("HasSubject = true (Subject = %q), want false — a subject hidden inside an HTML comment must never be read", r.Subject)
	}
}

// TestParseForkTestContiguityCutOnFirstNonKeyLine — cor-1688-C11 item 2 / sec-1688-S1 (round
// 6 residual, row "directly under the block, no blank line: `From: a reporter` /
// `Subject: Re: typo in the README`"): content directly adjacent to the block (no blank line
// separating it) still cannot become the declared subject when the FIRST such line is not
// itself key-shaped — `From: a reporter` opens with an uppercase letter, so it is not a key
// line (forkKeyLineRe requires a lowercase key name), and the contiguous run ends right there,
// before ever reaching the `Subject:` line one line further down.
//
// FAIL-FIRST (mutation: extractForkSection's contiguity loop changed to advance `end` to
// `len(lines)` unconditionally, ignoring forkKeyLineRe after the first key line): HasSubject
// flips to true.
func TestParseForkTestContiguityCutOnFirstNonKeyLine(t *testing.T) {
	body := validForkTestBlock + "From: a reporter\nSubject: Re: typo in the README\n"
	r := parseForkTest(body)
	if r.HasSubject {
		t.Errorf("HasSubject = true (Subject = %q), want false — `From: a reporter` is not key-shaped and must end the block", r.Subject)
	}
}

// TestParseForkTestDirectlyAdjacentKeyShapedLineJoinsTheBlock — documents the one residual
// both re-reviews flagged as advisory, not blocking (correctness re-review cor-1688-C11: "the
// grouping the comment deliberately allows"; security review sec-1688-S1: "the first shape...
// whether it counts as the filer's own declaration is that lane's call"): a key-shaped line
// placed DIRECTLY adjacent to the block, with no blank line breaking contiguity, is simply
// part of the same contiguous run — the grammar has no way to tell "the filer's own next
// line" from "an appended line with no separator" when both are syntactically identical key
// lines. Here that is SAFE by construction: this fixture's appended `subject:` line is the
// block's ONLY subject line, so it is read (HasSubject=true) — but had the block already
// declared one, a second contiguous `subject:` line would make the count two, and the
// exactly-one-line rule (forktest.go) refuses to pick either.
// TestParseForkTestQuotedIntroLineIsMalformedNotAKeyLine — pins forkKeyLineRe's own
// column-zero, no-decoration requirement in isolation, as distinct from the plain-prose C11
// case above: a `>`-quoted line directly after the heading, directly followed (no blank line)
// by the block's real key lines, is malformed because the QUOTED line itself is not a key
// line — not because prose sits in the way.
//
// FAIL-FIRST (mutation: forkKeyLineRe loosened to `^[ \t>*_-]*[a-z][a-z-]*: \S`, re-admitting
// the old bullet/blockquote prefix class): the quoted line becomes the section's first key
// line, the real option/default/caught-by/ruled-check lines directly beneath it join the same
// contiguous run (nothing separates them), and Structural() flips from false to true.
func TestParseForkTestQuotedIntroLineIsMalformedNotAKeyLine(t *testing.T) {
	body := "### Fork test\n\n> subject: fix a typo\n" +
		"option: A — do X | works-because: reversible | consequence: low risk\n" +
		"option: B — do Y | works-because: matches precedent | consequence: slower\n" +
		"default: A\n" +
		"caught-by: nothing\n" +
		"ruled-check: checked the tracker → nothing found\n"
	r := parseForkTest(body)
	if r.Structural() {
		t.Fatalf("Structural() = true, want false — a `>`-quoted line is not a key line, whatever directly follows it; errors: %v", r.Errors)
	}
	if len(r.Options) != 0 {
		t.Errorf("Options = %+v, want none — the quoted intro line makes the block unlocatable", r.Options)
	}
}

func TestParseForkTestDirectlyAdjacentKeyShapedLineJoinsTheBlock(t *testing.T) {
	body := validForkTestBlock + "subject: a directly adjacent line with no separating blank line\n"
	r := parseForkTest(body)
	if !r.HasSubject || r.Subject != "a directly adjacent line with no separating blank line" {
		t.Fatalf("HasSubject/Subject = %v/%q, want true/%q — a directly-adjacent key-shaped line joins the same contiguous run",
			r.HasSubject, r.Subject, "a directly adjacent line with no separating blank line")
	}
}
