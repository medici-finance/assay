package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// forkgate_round7_test.go — the seventh round: a strict, fail-closed grammar replaces five
// rounds of shape-by-shape patches (see the file comment in forktest.go). These are the
// end-to-end probes, through `deskfile new` itself, for the shapes that used to need their own
// boundary marker plus the withheld variants named in the private review detail; the
// pure-parser-level pins for the grammar's three rules live in forktest_test.go.

// TestNoticeLaneRefusesFencedTrailingSubjectDirectlyUnderBlock — cor-1688-C12/sec-1688-S5: a
// fenced `Subject:` line placed DIRECTLY under a no-subject block (no blank line — the shape
// the round-6 fence-arm mutation (MF/MG in both re-reviews) showed had no test that could
// fail). stripFencedBlocks removes the fence and its content entirely before the section is
// even located, so there is nothing left to read.
func TestNoticeLaneRefusesFencedTrailingSubjectDirectlyUnderBlock(t *testing.T) {
	withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("FAKEGH_LABELS", labelsJSON(t, needsDecisionLabel, deskDecidedLabel))
	block := noticeLaneBlock + "```\nSubject: Re: typo in the README\n```\n"
	body := bodyFileWith(t, neutralEvidence+"\n"+block)

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "Let the bot LGTM its own PRs?", "--body-file", body, "--label", needsDecisionLabel})
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0 (filed, on needs-decision); out=%s", rc, out)
	}
	final := curForge.finalLabels()
	if !hasLabel(final, needsDecisionLabel) || hasLabel(final, deskDecidedLabel) {
		t.Errorf("filed with labels %v, want needs-decision and no desk-decided", final)
	}
	if curForge.filed != nil && strings.Contains(curForge.filed.Body, deskDecidedMarker) {
		t.Error("carries the notice-lane marker")
	}
}

// TestNewRefusesSubjectHiddenInHTMLComment — a withheld variant named in the private review
// detail: a
// `subject:` line hidden inside an HTML comment BETWEEN two of the block's own real key lines,
// invisible in the rendered issue. stripHTMLComments removes the whole comment (delimiters and
// content) before the section is even located, so the hidden line can never be read as the
// declared subject. Here the comment sits between `default:` and `caught-by:`, so stripping it
// also removes the newline that separated real fields, leaving a blank line in their place —
// the block loses structural completeness as a SAFE side effect: this filing is refused
// outright (never silently admitted), and the audited `--force-new --reason` bypass is the
// only way past it, exactly like any other malformed block. The load-bearing assertion either
// way is that the hidden text never appears anywhere in what gets filed.
func TestNewRefusesSubjectHiddenInHTMLComment(t *testing.T) {
	withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("FAKEGH_LABELS", labelsJSON(t, needsDecisionLabel, deskDecidedLabel))
	block := "### Fork test\n\n" +
		"option: A — keep the current default | works-because: it is a one-line revert if wrong | consequence: no behaviour change today\n" +
		"option: B — flip the default | works-because: the draft PR catches a wrong flip before it lands | consequence: every caller sees the new default on the next build\n" +
		"default: A\n" +
		"<!--\nsubject: " + reversibleTitle + "\n-->\n" +
		"caught-by: draft-pr — #777\n" +
		"ruled-check: searched the tracker for \"the same question\" → nothing on record\n"
	body := bodyFileWith(t, neutralEvidence+"\n"+block)

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "Let the bot LGTM its own PRs?", "--body-file", body, "--label", needsDecisionLabel})
	if rc != deskkit.ExitRefused {
		t.Fatalf("rc = %d, want %d (the comment splits two real key lines, so the block is malformed); out=%s", rc, deskkit.ExitRefused, out)
	}
	if curForge.filed != nil {
		t.Fatal("an issue was filed despite a malformed fork-test block")
	}
	if strings.Contains(out, reversibleTitle) {
		t.Errorf("the refusal message echoes the hidden subject text: %s", out)
	}
}

// TestNoticeLaneRefusesSubjectHiddenInTrailingHTMLComment — the same withheld variant, placed
// AFTER every required field instead of splitting two of them: the block stays structurally
// complete (every required line is present before the comment), so this isolates the security
// property cleanly — the hidden `subject:` line is stripped along with its comment before
// parsing, so the filing has no declared subject at all and stays on needs-decision, never
// desk-decided.
func TestNoticeLaneRefusesSubjectHiddenInTrailingHTMLComment(t *testing.T) {
	withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("FAKEGH_LABELS", labelsJSON(t, needsDecisionLabel, deskDecidedLabel))
	block := noticeLaneBlock + "<!--\nsubject: " + reversibleTitle + "\n-->\n"
	body := bodyFileWith(t, neutralEvidence+"\n"+block)

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "Let the bot LGTM its own PRs?", "--body-file", body, "--label", needsDecisionLabel})
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0; out=%s", rc, out)
	}
	final := curForge.finalLabels()
	if !hasLabel(final, needsDecisionLabel) || hasLabel(final, deskDecidedLabel) {
		t.Errorf("filed with labels %v, want needs-decision and no desk-decided (a subject hidden in an HTML comment must never admit)", final)
	}
	if curForge.filed != nil && strings.Contains(curForge.filed.Body, deskDecidedMarker) {
		t.Error("carries the notice-lane marker")
	}
}

// TestNewRefusesForkTestHeadingNotFollowedDirectly — cor-1688-C11, end to end: prose between
// the heading and the first key line is a malformed block, refused (exit 5) naming the actual
// problem, not a pile of "missing line" errors that misdescribe what is wrong.
func TestNewRefusesForkTestHeadingNotFollowedDirectly(t *testing.T) {
	withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("FAKEGH_LABELS", labelsJSON(t, needsDecisionLabel))
	block := "### Fork test\n\nA sentence of context before the real block starts.\n\n" +
		strings.SplitN(validForkTestBlock, "### Fork test\n\n", 2)[1]
	body := bodyFileWith(t, block)

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "a decision with prose before the fork-test block", "--body-file", body,
		"--label", needsDecisionLabel})
	if rc != deskkit.ExitRefused {
		t.Fatalf("rc = %d, want %d; out=%s", rc, deskkit.ExitRefused, out)
	}
	if curForge.filed != nil {
		t.Fatal("an issue was filed despite a malformed fork-test block")
	}
	if !strings.Contains(out, "followed directly") {
		t.Errorf("refusal does not name the actual problem (heading not followed directly): %s", out)
	}
}

// TestNoticeLaneRefusesDirectlyAdjacentNonKeyTrailer — sec-1688-S1 (round 6 residual, review
// 5332050856's row "directly under the block, no blank line: `From: a reporter` /
// `Subject: Re: typo in the README`"): content directly adjacent to the block, with no blank
// line, still cannot supply the declared subject when the first such line is not itself key
// shaped — the contiguous run ends there, before the `Subject:` line one line further down is
// ever reached.
func TestNoticeLaneRefusesDirectlyAdjacentNonKeyTrailer(t *testing.T) {
	withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("FAKEGH_LABELS", labelsJSON(t, needsDecisionLabel, deskDecidedLabel))
	block := noticeLaneBlock + "From: a reporter\nSubject: " + reversibleTitle + "\n"
	body := bodyFileWith(t, neutralEvidence+"\n"+block)

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "Let the bot LGTM its own PRs?", "--body-file", body, "--label", needsDecisionLabel})
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0; out=%s", rc, out)
	}
	final := curForge.finalLabels()
	if !hasLabel(final, needsDecisionLabel) || hasLabel(final, deskDecidedLabel) {
		t.Errorf("filed with labels %v, want needs-decision and no desk-decided", final)
	}
}
