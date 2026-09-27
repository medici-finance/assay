package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// forkgate_round4_test.go — the fourth round of fail-closed probes on the notice lane: the
// arbiter packet's round-cap ruling on sec-1688-S1 (medici-finance/assay#1688,
// issuecomment-5840449031), OPTION 2 — bind the reversible-signal read to the filing's
// DECLARED SUBJECT (the `### Fork test` block's `subject:` line — see noticelane.go and
// forktest.go), never the title or body prose, and never admit a lint-level/port-or-drop
// example whose subject names a CI check or job (round 5 replaced the CI-check/job NOUN scan
// for these needles with an unconditional refusal — see forkgate_round5_test.go — but the
// probes below still pass unchanged, since every one they cover also names a check by a
// generic noun this round's scan already recognised). Every probe below is the security
// review's own end-to-end evidence, verbatim (titles and, where given, the "extra" prose the
// review prepended to the body), reproduced here through deskfile's real `new` command and a
// well-formed fork-test block, rather than the review's own throwaway harness.
//
// FAIL-FIRST: against the previous head (076cd94ec, before this round's fix), 12 of these 13
// probes went red — each took the notice lane (rc 0, labels [desk-decided]) exactly as the
// arbiter packet's evidence table shows (the packet's positive control, "fix the docs wording
// in the README", is not repeated here — it is round 1's reversibleTitle fixture). The 13th,
// main-commit-plain, is this file's own control and was already correctly needs-decision at
// that head (see its doc comment below) — not a fail-first case. Excerpt:
//
//	--- FAIL: TestNoticeLaneRefusesRound4Probes/main-commit-wording
//	    "Tool default: let the desk commit to main when CI is green? Fix the help-text wording too." filed with labels [desk-decided], want needs-decision and no desk-decided
//	--- FAIL: TestNoticeLaneRefusesRound4Probes/control-sweep
//	    "Lint level for the control-sweep check: notice or error?" filed with labels [desk-decided], want needs-decision and no desk-decided
//	    ... (12 of 13 probes red)

// round4Probes are the arbiter packet's 12 fresh probes plus its own control case, verbatim.
// Each pairs the probe's real title/extra with a `subject:` line — a clean, single-clause
// statement of what the item is actually about, exactly what an honest filer completing the
// fork-test block's (optional) `subject:` line would write.
//
// Two different mechanisms close these, and the split is deliberate (see noticelane.go):
//
//   - main-commit-wording, bot-lgtm-typo, fork-heads-typo, contents-write-wording,
//     close-decisions-rephrasing, table-column-envdump: the reversible needle sits in an
//     INCIDENTAL secondary clause of the title (or, for print-env-wording and
//     git-reset-rephrasing, only in the "extra" prose — never even a candidate for the
//     subject). subject omits that clause, so FirstNoticeLaneSignal finds no needle at all.
//     This is what binding admission to a declared subject is FOR.
//   - control-sweep, pattern-sweep-notice, leak-check-severity, pattern-sweep-job: the
//     reversible needle and the CI-check/job name are the SAME clause — there is no
//     incidental part to drop, the subject genuinely is "the lint level / port-or-drop
//     disposition of this named check". subject keeps it whole; ciCheckOrJobRe is what
//     refuses these, whichever reversible needle matched.
//
// main-commit-plain is the packet's own negative control: the same one-way act as
// main-commit-wording, with no reversible needle anywhere. It was already correctly
// needs-decision before this round (no admission mechanism ever reached it) — pinned here as
// a regression check, not a fail-first case.
var round4Probes = []struct {
	name, title, extra, subject string
}{
	{
		name:    "main-commit-wording",
		title:   "Tool default: let the desk commit to main when CI is green? Fix the help-text wording too.",
		subject: "let the desk commit to main when CI is green",
	},
	{
		name:    "bot-lgtm-typo",
		title:   "Let the bot LGTM its own PRs (typo-fix PRs only)?",
		subject: "let the bot LGTM its own PRs",
	},
	{
		name:    "fork-heads-typo",
		title:   "Build untrusted fork heads in CI for typo-only PRs?",
		subject: "build untrusted fork heads in CI",
	},
	{
		name:    "contents-write-wording",
		title:   "Give the worker App contents: write for wording fixes?",
		subject: "give the worker App contents: write",
	},
	{
		name:    "close-decisions-rephrasing",
		title:   "Close stale decision issues after 30 days? (rephrasing of an older ask)",
		subject: "close stale decision issues after 30 days",
	},
	{
		name:    "print-env-wording",
		title:   "Tool default: print the environment to the job log on failure?",
		extra:   "Context: the current wording of the runbook is ambiguous.\n",
		subject: "print the environment to the console on tool failure",
	},
	{
		name:    "git-reset-rephrasing",
		title:   "Tool default: git reset --hard the shared checkout on resync?",
		extra:   "Note: rephrasing the older ask.\n",
		subject: "git reset --hard the shared checkout on resync",
	},
	{
		name:    "control-sweep",
		title:   "Lint level for the control-sweep check: notice or error?",
		subject: "lint level for the control-sweep check: notice or error",
	},
	{
		name:    "pattern-sweep-notice",
		title:   "Pattern-sweep findings: notice or error?",
		subject: "pattern-sweep findings: notice or error",
	},
	{
		name:    "leak-check-severity",
		title:   "Lint severity of the leak check: warn only?",
		subject: "lint severity of the leak check: warn only",
	},
	{
		name:    "pattern-sweep-job",
		title:   "Port-or-drop the pattern-sweep job?",
		subject: "port-or-drop the pattern-sweep job",
	},
	{
		name:    "table-column-envdump",
		title:   "Table column: add the raw environment dump to the digest?",
		subject: "add the raw environment dump to the digest",
	},
	{
		name:    "main-commit-plain",
		title:   "Let the desk commit to main when CI is green?",
		subject: "let the desk commit to main when CI is green",
	},
}

// TestNoticeLaneRefusesRound4Probes — every round-4 probe stays on needs-decision: the
// reversible signal is read from the fork-test block's declared subject alone, never the
// title or body prose (closing the incidental-clause probes), and a subject naming a CI check
// or job never admits on a lint-level/port-or-drop example either (closing the
// named-check/job probes — round 5 made this refusal unconditional for the four needles;
// see TestNoticeLaneRefusesRound5NamedCICheckSubjects for the case that distinguishes them,
// a check named by itself rather than a generic noun). See round4Probes for which mechanism
// closes which probe.
func TestNoticeLaneRefusesRound4Probes(t *testing.T) {
	for _, tc := range round4Probes {
		t.Run(tc.name, func(t *testing.T) {
			withEnv(t)
			t.Setenv("FAKEGH_SEARCH_HITS", "[]")
			t.Setenv("FAKEGH_LABELS", labelsJSON(t, needsDecisionLabel, deskDecidedLabel))
			block := strings.Replace(noticeLaneBlock,
				"### Fork test\n\n", "### Fork test\n\nsubject: "+tc.subject+"\n\n", 1)
			body := bodyFileWith(t, tc.extra+"\n\n"+neutralEvidence+"\n"+block)

			rc, out := runCapture([]string{"new", "-R", allowedRepo,
				"--title", tc.title, "--body-file", body, "--label", needsDecisionLabel})
			if rc != deskkit.ExitOK {
				t.Fatalf("rc = %d, want 0 (filed, on needs-decision); out=%s", rc, out)
			}
			final := curForge.finalLabels()
			if !hasLabel(final, needsDecisionLabel) || hasLabel(final, deskDecidedLabel) {
				t.Errorf("%q filed with labels %v, want needs-decision and no desk-decided", tc.title, final)
			}
			if curForge.filed != nil && strings.Contains(curForge.filed.Body, deskDecidedMarker) {
				t.Errorf("%q carries the notice-lane marker", tc.title)
			}
		})
	}
}
