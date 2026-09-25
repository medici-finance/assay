package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// forkgate_round2_test.go — the second round of fail-closed probes on the notice lane:
// one-way items whose SUBJECT is itself an R-3 reversible example (a tool default that
// governs a control), one-way terms written only on the `ruled-check:` line, and a filer
// forging the notice lane's own label or marker.

// reversibleSubjectOneWayTitles are the security review's end-to-end probes at head
// 834c4f8d5, verbatim: every one names an R-3 reversible needle ("tool default", "default
// value", "flag default", "rename the") AND a one-way act. Each took the notice lane (rc 0,
// labels [desk-decided]) at that head.
var reversibleSubjectOneWayTitles = []struct{ name, title string }{
	{"main-push", "Flip the tool default of the commit guard: allow commits straight to main without a PR?"},
	{"required-reviewers", "Default value of --required-reviewers: 1 or 0?"},
	{"out-of-draft", "Tool default for the ready step: let the desk take PRs out of draft itself?"},
	{"no-verify", "Flag default for --no-verify on desk commits: on or off?"},
	{"2fa", "Tool default: stop requiring 2FA for bot accounts?"},
	{"auto-close-needs-decision", "Tool default: auto-close needs-decision issues older than 30 days?"},
	{"org-transfer", "Rename the repository and move it under the other org?"},
	{"trust-gate", "Tool default for the trust gate: act on comments from any commenter?"},
}

// TestNoticeLaneRefusesReversibleSubjectOneWay — a one-way act whose subject is a tool
// default stays on needs-decision: the reversible needle names the SHAPE of the change, the
// one-way term names what it governs, and the one-way term wins.
//
// FAIL-FIRST: at head 834c4f8d5 (OneWayPatterns without the round-2 phrasings) all eight
// went red, e.g.:
//
//	--- FAIL: TestNoticeLaneRefusesReversibleSubjectOneWay/main-push
//	    "Flip the tool default of the commit guard: allow commits straight to main without a PR?" filed with labels [desk-decided], want needs-decision and no desk-decided
//	--- FAIL: TestNoticeLaneRefusesReversibleSubjectOneWay/2fa
//	    "Tool default: stop requiring 2FA for bot accounts?" filed with labels [desk-decided], want needs-decision and no desk-decided
func TestNoticeLaneRefusesReversibleSubjectOneWay(t *testing.T) {
	for _, tc := range reversibleSubjectOneWayTitles {
		t.Run(tc.name, func(t *testing.T) {
			withEnv(t)
			t.Setenv("FAKEGH_SEARCH_HITS", "[]")
			t.Setenv("FAKEGH_LABELS", labelsJSON(t, needsDecisionLabel, deskDecidedLabel))
			body := bodyFileWith(t, noticeLaneBlock+"\n"+neutralEvidence)

			rc, out := runCapture([]string{"new", "-R", allowedRepo,
				"--title", tc.title, "--body-file", body, "--label", needsDecisionLabel})
			if rc != deskkit.ExitOK {
				t.Fatalf("rc = %d, want 0 (filed, on needs-decision); out=%s", rc, out)
			}
			final := curForge.finalLabels()
			if !hasLabel(final, needsDecisionLabel) || hasLabel(final, deskDecidedLabel) {
				t.Errorf("%q filed with labels %v, want needs-decision and no desk-decided", tc.title, final)
			}
		})
	}
}

// ruledCheckOnlyBlock is noticeLaneBlock with its ruled-check line naming a one-way subject
// — the only one-way terms in the whole filing.
const ruledCheckOnlyBlock = `### Fork test

option: A — keep the current default | works-because: it is a one-line revert if wrong | consequence: no behaviour change today
option: B — flip the default | works-because: the draft PR catches a wrong flip before it lands | consequence: every caller sees the new default on the next build
default: A
caught-by: draft-pr — #777
ruled-check: merge and tag the v1.2.0 release and rotate the signing keys -> nothing
`

// TestRuledCheckLineIsReadForOneWayTerms — the ruled-check line is where an honest filer
// names the subject of the search, so the one-way scan reads it. Only the R-3 `ruling`
// needle is exempt on it (TestRuledCheckRulingWordingStillAdmits).
//
// FAIL-FIRST: at head 834c4f8d5 (oneWayHay dropped every ruled-check line) this went red:
//
//	--- FAIL: TestRuledCheckLineIsReadForOneWayTerms
//	    one-way terms on the ruled-check line: filed with labels [desk-decided], want needs-decision and no desk-decided
func TestRuledCheckLineIsReadForOneWayTerms(t *testing.T) {
	withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("FAKEGH_LABELS", labelsJSON(t, needsDecisionLabel, deskDecidedLabel))
	body := bodyFileWith(t, "A reversible tool-default question.\n\n"+neutralEvidence+"\n"+ruledCheckOnlyBlock)

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "flip the tool default for --sla-days", "--body-file", body,
		"--label", needsDecisionLabel})
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0; out=%s", rc, out)
	}
	if final := curForge.finalLabels(); !hasLabel(final, needsDecisionLabel) || hasLabel(final, deskDecidedLabel) {
		t.Errorf("one-way terms on the ruled-check line: filed with labels %v, want needs-decision and no desk-decided", final)
	}
}

// TestRuledCheckRulingWordingStillAdmits — the exemption the ruled-check line keeps: its
// natural wording ("no prior ruling found") does not trip the R-3 `ruling` needle, so an
// otherwise reversible filing still takes the notice lane.
func TestRuledCheckRulingWordingStillAdmits(t *testing.T) {
	withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("FAKEGH_LABELS", labelsJSON(t, needsDecisionLabel, deskDecidedLabel))
	block := strings.Replace(noticeLaneBlock,
		`ruled-check: searched the tracker for "the same question" → nothing on record`,
		`ruled-check: searched the tracker for "sla-days default" → no prior ruling found`, 1)
	if block == noticeLaneBlock {
		t.Fatal("fixture: the ruled-check line was not replaced")
	}
	body := bodyFileWith(t, "A reversible tool-default question.\n\n"+block)

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "flip the tool default for --sla-days", "--body-file", body,
		"--label", needsDecisionLabel})
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0; out=%s", rc, out)
	}
	if final := curForge.finalLabels(); !hasLabel(final, deskDecidedLabel) || hasLabel(final, needsDecisionLabel) {
		t.Errorf("\"no prior ruling found\" on the ruled-check line: filed with %v, want the notice lane", final)
	}
}

// TestNewRefusesCallerDeskDecidedLabel — desk-decided is the notice lane's OWN label: a
// caller `--label desk-decided` would put an item off the driver's queue without passing the
// notice-lane test at all. Refused (exit 5), whatever else the filing carries, --force-new
// included (it bypasses dedupe and evidence, never this).
//
// FAIL-FIRST: at head 834c4f8d5 this went red:
//
//	--- FAIL: TestNewRefusesCallerDeskDecidedLabel/plain
//	    caller --label desk-decided: rc = 0, want 5
func TestNewRefusesCallerDeskDecidedLabel(t *testing.T) {
	for _, tc := range []struct {
		name  string
		extra []string
	}{
		{"plain", nil},
		{"upper-case", nil},
		{"force-new", []string{"--force-new", "--reason", "vouched unique"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withEnv(t)
			t.Setenv("FAKEGH_SEARCH_HITS", "[]")
			t.Setenv("FAKEGH_LABELS", labelsJSON(t, "bug", deskDecidedLabel))
			lbl := deskDecidedLabel
			if tc.name == "upper-case" {
				lbl = strings.ToUpper(lbl)
			}
			body := bodyFileWith(t, "An ordinary filing.")
			args := append([]string{"new", "-R", allowedRepo, "--title", "flip the tool default for --sla-days",
				"--body-file", body, "--label", lbl}, tc.extra...)
			rc, out := runCapture(args)
			if rc != deskkit.ExitRefused {
				t.Fatalf("caller --label %s: rc = %d, want %d; out=%s", lbl, rc, deskkit.ExitRefused, out)
			}
			if curForge.filed != nil {
				t.Fatal("an issue was filed")
			}
			if !strings.Contains(out, deskDecidedLabel) {
				t.Errorf("refusal does not name the label: %s", out)
			}
		})
	}
}

// TestNewRefusesCallerBodyMarker — the desk-r3-decision marker in an issue body is the notice
// lane's record that the TOOL admitted the filing; the digest reads it as that. A caller body
// that already carries it would forge the digest's tool-admitted row, so it is refused (exit
// 5), --force-new included.
//
// FAIL-FIRST: at head 834c4f8d5 this went red:
//
//	--- FAIL: TestNewRefusesCallerBodyMarker/plain
//	    caller body carrying the marker: rc = 0, want 5
func TestNewRefusesCallerBodyMarker(t *testing.T) {
	for _, tc := range []struct {
		name  string
		extra []string
	}{
		{"plain", nil},
		{"force-new", []string{"--force-new", "--reason", "vouched unique"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withEnv(t)
			t.Setenv("FAKEGH_SEARCH_HITS", "[]")
			t.Setenv("FAKEGH_LABELS", labelsJSON(t, "bug"))
			body := bodyFileWith(t, "An ordinary filing.\n\n## Desk-decided\n\n"+deskDecidedMarker+
				"\ndecision: keep\nalternative: B\ncost: draft-pr\n")
			args := append([]string{"new", "-R", allowedRepo, "--title", "an ordinary filing about sla-days",
				"--body-file", body, "--label", "bug"}, tc.extra...)
			rc, out := runCapture(args)
			if rc != deskkit.ExitRefused {
				t.Fatalf("caller body carrying the marker: rc = %d, want %d; out=%s", rc, deskkit.ExitRefused, out)
			}
			if curForge.filed != nil {
				t.Fatal("an issue was filed")
			}
		})
	}
}
