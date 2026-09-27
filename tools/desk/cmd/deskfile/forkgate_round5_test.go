package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// forkgate_round5_test.go — the fifth round of fail-closed probes on the notice lane: the
// security re-review at head 8eb647757 (review 5331815778) on the driver's option-2 ruling
// (issuecomment-5858970703). Two conformance gaps remained against the ruling's clauses:
//
//   - clause 1 (bind the reversible signal to the DECLARED subject, never the body): the
//     fork-test section's `subject:` read still let a `>`-quoted line count, and let the LAST
//     of several `subject:` lines win, so trailing prose or a stray second line could override
//     an honest declared subject.
//   - clause 2 (a lint-level/notice-or-error example must never admit an item naming a CI
//     check or job): `ciCheckOrJobRe` recognised only the generic check/job nouns and this
//     codebase's own "<word>-sweep"/"<word> check" compounds, never a check's own name
//     (pin-consistency, skillslint, forge-surface, build-test, govulncheck, CodeQL all
//     admitted).
//
// The fix (internal/deskkit/noticelane.go, forktest.go): `lint level`, `lint severity`,
// `notice or error` and `port-or-drop`/`port or drop` moved into NoticeLaneShapeOnlyNeedles,
// so none of them ever admits on its own, named check or not; and parseForkTest now reads a
// declared subject only when the section carries EXACTLY ONE `subject:` line and it is not
// `>`-quoted — two or more, quoted or not, leaves no declared subject, the fail-closed default
// a missing one already gets. This also SUPERSEDES cor-1688-C7 (the correctness re-review's
// finding that ciCheckOrJobRe read only the last `subject:` line, not the title or every
// line): all four of that finding's cases are fixed by the same two mechanisms, not a third.
//
// FAIL-FIRST: against this head before the round-5 fix, every case in both tables below took
// the notice lane (rc 0, labels [desk-decided]) — captured with the production fix reverted
// and this file's probes kept, e.g.:
//
//	--- FAIL: TestNoticeLaneRefusesRound5SubjectAmbiguityProbes/quoted-trailing-subject
//	    "Let the bot LGTM its own PRs (typo-fix PRs only)?" filed with labels [desk-decided], want needs-decision and no desk-decided
//	--- FAIL: TestNoticeLaneRefusesRound5NamedCheckProbes/pattern-sweep-notice-title
//	    "Pattern-sweep findings: notice or error?" filed with labels [desk-decided], want needs-decision and no desk-decided

// round5SubjectAmbiguityProbes are the security re-review's clause-1 evidence rows, verbatim
// in substance: an otherwise-honest declared subject overridden by a second subject-shaped
// line the parser used to read anyway.
var round5SubjectAmbiguityProbes = []struct {
	name, title, block string
}{
	{
		name:  "quoted-trailing-subject",
		title: "Let the bot LGTM its own PRs (typo-fix PRs only)?",
		// A real, declared subject, then a `>`-quoted line of trailing prose (no heading in
		// between, so it is still inside the fork-test section) that itself reads as a
		// subject line and carries the actual admitting needle ("typo"). The quoted line
		// must never be read as, or count toward, the declared subject.
		block: strings.Replace(noticeLaneBlock, "### Fork test\n\n",
			"### Fork test\n\nsubject: let the bot LGTM its own PRs\n\n", 1) +
			"\n> Subject: Re: typo in the README\n",
	},
	{
		name:  "duplicate-real-subject-lines",
		title: "Let the bot LGTM its own PRs (typo-fix PRs only)?",
		// Two real, unquoted `subject:` lines. The parser used to keep only the last
		// ("fix a typo", an admitting needle); ambiguity must refuse instead.
		block: strings.Replace(noticeLaneBlock, "### Fork test\n\n",
			"### Fork test\n\nsubject: let the bot LGTM its own PRs\nsubject: fix a typo\n\n", 1),
	},
	{
		name:  "template-placeholder-plus-real-subject",
		title: "Typo-fix PRs only?",
		// A filer who left the grammar's own placeholder text in place, plus a real
		// subject line below it. Two lines, so still ambiguous — the placeholder is not
		// special-cased, it is just another `subject:` line.
		block: strings.Replace(noticeLaneBlock, "### Fork test\n\n",
			"### Fork test\n\nsubject: <one line naming what is actually being decided>\nsubject: typo-fix PRs only\n\n", 1),
	},
}

// TestNoticeLaneRefusesRound5SubjectAmbiguityProbes — every clause-1 probe stays on
// needs-decision: a declared subject counts only when the section has exactly one
// `subject:` line and it is not `>`-quoted.
func TestNoticeLaneRefusesRound5SubjectAmbiguityProbes(t *testing.T) {
	for _, tc := range round5SubjectAmbiguityProbes {
		t.Run(tc.name, func(t *testing.T) {
			withEnv(t)
			t.Setenv("FAKEGH_SEARCH_HITS", "[]")
			t.Setenv("FAKEGH_LABELS", labelsJSON(t, needsDecisionLabel, deskDecidedLabel))
			body := bodyFileWith(t, neutralEvidence+"\n"+tc.block)

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

// round5NamedCheckProbes are cor-1688-C7's four exact cases (correctness re-review at
// 8eb647757): a title naming a CI check or job by name, paired with a subject that either
// omits the name entirely or splits it across two `subject:` lines, so the old
// last-subject-only ciCheckOrJobRe scan never saw the name. Every one is now refused by the
// combined round-5 fix (the lint-level/port-or-drop needles never admit at all; a two-line
// subject is ambiguous), not by teaching ciCheckOrJobRe the title or every line — see the
// file comment.
var round5NamedCheckProbes = []struct {
	name, title, subject, secondSubject string
}{
	{
		name:    "pattern-sweep-notice-title",
		title:   "Pattern-sweep findings: notice or error?",
		subject: "lint level: notice or error",
	},
	{
		name:    "control-sweep-check-title",
		title:   "Lint level for the control-sweep check: notice or error?",
		subject: "lint level: notice or error",
	},
	{
		name:    "pattern-sweep-job-title",
		title:   "Port-or-drop the pattern-sweep job?",
		subject: "port-or-drop the legacy helper scripts",
	},
	{
		name:          "two-subject-lines-first-names-job",
		title:         "Port-or-drop question",
		subject:       "port-or-drop the pattern-sweep job",
		secondSubject: "port-or-drop the legacy helper scripts",
	},
}

// TestNoticeLaneRefusesRound5NamedCheckProbes — cor-1688-C7's four cases stay on
// needs-decision under the round-5 fix.
func TestNoticeLaneRefusesRound5NamedCheckProbes(t *testing.T) {
	for _, tc := range round5NamedCheckProbes {
		t.Run(tc.name, func(t *testing.T) {
			withEnv(t)
			t.Setenv("FAKEGH_SEARCH_HITS", "[]")
			t.Setenv("FAKEGH_LABELS", labelsJSON(t, needsDecisionLabel, deskDecidedLabel))
			subjectLines := "subject: " + tc.subject + "\n"
			if tc.secondSubject != "" {
				subjectLines += "subject: " + tc.secondSubject + "\n"
			}
			block := strings.Replace(noticeLaneBlock, "### Fork test\n\n", "### Fork test\n\n"+subjectLines+"\n", 1)
			body := bodyFileWith(t, neutralEvidence+"\n"+block)

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

// round5NamedCICheckSubjects are the security re-review's clause-2 evidence rows: a subject
// that is genuinely, single-clause about a lint level / lint severity / notice-or-error /
// port-or-drop question, naming this repository's own required checks BY NAME rather than a
// generic noun ciCheckOrJobRe would catch.
var round5NamedCICheckSubjects = []string{
	"lint level for pin-consistency: notice or error?",
	"lint severity for skillslint findings: warn only?",
	"port-or-drop forge-surface?",
	"lint level for the build-test step: notice or error?",
	"govulncheck findings: notice or error?",
	"lint level for the CodeQL scan: notice or error?",
}

// TestNoticeLaneRefusesRound5NamedCICheckSubjects — end-to-end through `deskfile new`: every
// named-check subject above stays on needs-decision. TestNoticeLaneVerdictRefusesNamedCheckByLintOrPortNeedle
// (internal/deskkit) pins the same shapes at the deskkit.NoticeLaneVerdict level and asserts
// none of them matches ciCheckOrJobRe, so this is the shape-only floor doing the work.
func TestNoticeLaneRefusesRound5NamedCICheckSubjects(t *testing.T) {
	for _, subject := range round5NamedCICheckSubjects {
		t.Run(subject, func(t *testing.T) {
			withEnv(t)
			t.Setenv("FAKEGH_SEARCH_HITS", "[]")
			t.Setenv("FAKEGH_LABELS", labelsJSON(t, needsDecisionLabel, deskDecidedLabel))
			block := strings.Replace(noticeLaneBlock, "### Fork test\n\n", "### Fork test\n\nsubject: "+subject+"\n\n", 1)
			body := bodyFileWith(t, neutralEvidence+"\n"+block)

			rc, out := runCapture([]string{"new", "-R", allowedRepo,
				"--title", subject, "--body-file", body, "--label", needsDecisionLabel})
			if rc != deskkit.ExitOK {
				t.Fatalf("rc = %d, want 0 (filed, on needs-decision); out=%s", rc, out)
			}
			final := curForge.finalLabels()
			if !hasLabel(final, needsDecisionLabel) || hasLabel(final, deskDecidedLabel) {
				t.Errorf("%q filed with labels %v, want needs-decision and no desk-decided", subject, final)
			}
		})
	}
}
