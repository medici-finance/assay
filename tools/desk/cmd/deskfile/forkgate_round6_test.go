package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// forkgate_round6_test.go — the sixth round of fail-closed probes on the notice lane: the
// correctness re-review at head 8953654d9 (review 5332061602, finding cor-1688-C7 residual)
// and the security re-review at the same head (review 5332050856, finding sec-1688-S1
// narrowed again, plus its S5 finding and its two advisories). Three gaps remained:
//
//   - sec-1688-S1, clause 1 residual: extractForkSection ran to the next heading or EOF, so a
//     `subject:`-shaped line anywhere in the trailing body — fenced, indented, plain prose, or
//     past a Markdown setext heading — was still read as the declared subject when the block
//     itself declared none. Fixed in forktest.go (extractForkSection, forkGrammarKeyLine);
//     pinned at the pure-parser level in forktest_test.go
//     (TestParseForkTestSectionBoundedAgainstTrailingContent) since that is where the bug
//     lives. This file's TestNoticeLaneAdmitsUnaffectedByBoundedSectionFix is the end-to-end
//     control: an ordinary, well-formed subject still admits after the bound.
//   - cor-1688-C7 residual / sec-1688-S1's needle-veto gap: FirstNoticeLaneSignal only SKIPPED
//     the four lint-level/port-or-drop shape-only needles when scanning for an admitting
//     needle, instead of treating a match on one of them as a VETO — so a subject pairing a
//     shape-only phrase with an unrelated content-bearing needle ("wording") still admitted
//     through the content needle. Fixed in noticelane.go (FirstNoticeLaneSignal checks the
//     veto list first and returns nil on any match, before ever looking for a content needle).
//   - sec-1688-S5: the round-5 `>`-quote exclusion and the ciCheckOrJobRe backstop each had no
//     test that could fail when disarmed. Pinned below (TestParseForkTestLoneQuotedSubjectDoesNotAdmit,
//     TestNoticeLaneVerdictCICheckBackstopCatchesContentNeedle in noticelane_test.go).
//
// FAIL-FIRST for the two end-to-end suites in this file: with FirstNoticeLaneSignal reverted
// to the round-5 skip (`continue` instead of `return nil`) and this file's probes kept, both
// subjects in TestNoticeLaneRefusesRound6ShapeOnlyVetoProbes took the notice lane (rc 0,
// labels [desk-decided]) — captured with the production fix reverted:
//
//	--- FAIL: TestNoticeLaneRefusesRound6ShapeOnlyVetoProbes/wording-of-the-lint-level
//	    "wording of the pin-consistency lint level: notice or error" filed with labels [desk-decided], want needs-decision and no desk-decided
//	--- FAIL: TestNoticeLaneRefusesRound6ShapeOnlyVetoProbes/port-or-drop-forge-surface
//	    "port-or-drop forge-surface" filed with labels [desk-decided], want needs-decision and no desk-decided

// round6ShapeOnlyVetoProbes are the two exact cases named by both re-reviews: a subject that
// pairs a shape-only needle (never admitting on its own since round 5) with a genuine
// content-bearing needle. The shape-only phrase must still force needs-decision — it must
// never be outvoted by the paired content needle.
var round6ShapeOnlyVetoProbes = []struct {
	name, subject string
}{
	{
		name: "wording-of-the-lint-level",
		// "lint level" (shape-only) paired with "wording" (a real ReversibleSignals content
		// needle) — the exact residual from cor-1688-C7 and sec-1688-S1's throwaway probe.
		subject: "wording of the pin-consistency lint level: notice or error",
	},
	{
		name: "port-or-drop-forge-surface",
		// "port-or-drop" (shape-only) naming a real check ("forge-surface") by its bare name
		// — round 5 already refuses this (no other needle present), so this pins that the new
		// veto ordering does not change that outcome.
		subject: "port-or-drop forge-surface",
	},
}

// TestNoticeLaneRefusesRound6ShapeOnlyVetoProbes — both cases stay on needs-decision under
// the round-6 fix: a shape-only needle vetoes admission outright, rather than merely being
// skipped while a different needle in the same subject is free to admit.
func TestNoticeLaneRefusesRound6ShapeOnlyVetoProbes(t *testing.T) {
	for _, tc := range round6ShapeOnlyVetoProbes {
		t.Run(tc.name, func(t *testing.T) {
			withEnv(t)
			t.Setenv("FAKEGH_SEARCH_HITS", "[]")
			t.Setenv("FAKEGH_LABELS", labelsJSON(t, needsDecisionLabel, deskDecidedLabel))
			block := strings.Replace(noticeLaneBlock, "### Fork test\n\n", "### Fork test\n\nsubject: "+tc.subject+"\n", 1)
			body := bodyFileWith(t, neutralEvidence+"\n"+block)

			rc, out := runCapture([]string{"new", "-R", allowedRepo,
				"--title", tc.subject, "--body-file", body, "--label", needsDecisionLabel})
			if rc != deskkit.ExitOK {
				t.Fatalf("rc = %d, want 0 (filed, on needs-decision); out=%s", rc, out)
			}
			final := curForge.finalLabels()
			if !hasLabel(final, needsDecisionLabel) || hasLabel(final, deskDecidedLabel) {
				t.Errorf("%q filed with labels %v, want needs-decision and no desk-decided", tc.subject, final)
			}
			if curForge.filed != nil && strings.Contains(curForge.filed.Body, deskDecidedMarker) {
				t.Errorf("%q carries the notice-lane marker", tc.subject)
			}
		})
	}
}

// TestNoticeLaneAdmitsUnaffectedByBoundedSectionFix — end-to-end control for the
// extractForkSection bound (forktest.go): an ordinary, well-formed subject declared inside the
// block, with no trailing content at all, still admits the notice lane exactly as before. The
// bound narrows WHERE a subject can be read from, never whether a genuinely declared one
// admits.
func TestNoticeLaneAdmitsUnaffectedByBoundedSectionFix(t *testing.T) {
	withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("FAKEGH_LABELS", labelsJSON(t, needsDecisionLabel, deskDecidedLabel))
	body := bodyFileWith(t, neutralEvidence+"\n"+noticeLaneBlockWithSubject)

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", reversibleTitle, "--body-file", body, "--label", needsDecisionLabel})
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0; out=%s", rc, out)
	}
	final := curForge.finalLabels()
	if !hasLabel(final, deskDecidedLabel) {
		t.Errorf("filed with labels %v, want desk-decided (a genuinely declared, content-bearing subject must still admit)", final)
	}
}
