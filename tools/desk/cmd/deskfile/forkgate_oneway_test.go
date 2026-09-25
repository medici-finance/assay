package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// forkgate_oneway_test.go — the fail-CLOSED half of the fork-test gate: which filings may
// leave the driver's queue at all. The notice lane is admitted only on a POSITIVE reversible
// signal with no one-way term and no one-way caller label; the fewer-than-two-options
// refusal and `--no-fork` never steer a one-way item off the queue. Every lead in the
// one-way table below took the notice lane (labels [desk-decided], rc 0) before the gate
// failed closed — the red run is quoted on TestNoticeLaneRefusesOneWayClasses.

// noticeLaneBlock is a two-option block the driver still holds a gate on (`caught-by:
// draft-pr`), worded so that NOTHING in it is a one-way term — the fixture the notice lane
// is meant for. Paired with reversibleTitle (a content-bearing R-3 reversible example), it
// reaches the notice lane; every negative test below adds exactly one thing to it.
const noticeLaneBlock = `### Fork test

option: A — keep the current default | works-because: it is a one-line revert if wrong | consequence: no behaviour change today
option: B — flip the default | works-because: the draft PR catches a wrong flip before it lands | consequence: every caller sees the new default on the next build
default: A
caught-by: draft-pr — #777
ruled-check: searched the tracker for "the same question" → nothing on record
`

// reversibleTitle names a CONTENT-BEARING R-3 reversible example (docs wording), the kind of
// signal that admits the notice lane. A shape-only needle ("tool default") no longer admits
// on its own (TestNoticeLaneRefusesShapeOnlyReversibleSubject), so a fixture titled with one
// would pass every one-way test below for the wrong reason.
const reversibleTitle = "fix the docs wording of the --sla-days help text"

// neutralEvidence is an `### Evidence` fence carrying no one-way term (bodyWithEvidence's
// "cluster" is itself a live-infrastructure term, so it cannot isolate a one-way assertion).
// Any filing that stays on needs-decision still passes the blocker-evidence gate with it.
const neutralEvidence = "### Evidence\n\n```\n$ deskfile new --help\nusage: deskfile new -R <repo> --title <t> --body-file <f>\n```\n"

// oneWayLeads is one lead per one-way class the gate must keep on the driver's queue. The
// first twelve are the security review's probes, verbatim; the next five are the correctness
// review's. Each is filed with a reversible title (reversibleTitle), a
// well-formed two-option block and `caught-by: draft-pr` — the filer's claim — so the only
// thing keeping it on needs-decision is the one-way term itself.
var oneWayLeads = []struct{ name, lead string }{
	{"tag-release", "Should we cut the v1.2.0 release tag now or after the next batch?"},
	{"merge", "Merge this PR today or wait for the second approval?"},
	{"key-custody", "Who holds custody of the App signing key: the service or the platform?"},
	{"gate-human-unspaced", "gate:human on brief 07 - the driver or the desk?"},
	{"app-permission", "Should the reviewer App get actions: write so it can dispatch runs?"},
	{"funds", "Move funds from the vault to the pool now or at settlement?"},
	{"identity", "Change the identity provider realm config now or next sprint?"},
	{"branch-protection", "Disable the branch protection ruleset for the migration window?"},
	{"weaken-control", "Weaken the leak-sweep assertion to unblock CI or keep it red?"},
	{"external-send", "Send the report to the external vendor or keep it internal?"},
	{"rotate-keys", "Rotate the keys?"},
	{"self-approval", "Should the desk be allowed to approve its own PRs?"},
	{"pii", "Ship the PII export endpoint now or after review?"},
	{"release-v2", "Cut the v2.0.0 release tag now, or wait a week."},
	{"merge-main", "Merge the open PR into main now or hold it."},
	{"login", "Change the identity provider for the login flow."},
	{"money", "Move money from the pool to the vault."},
	{"external-service", "Send the report to an external service."},
	{"ready-flip", "Ready-flip the PR now or after one more review round?"},
	{"main-push", "Push the fix straight to main or open a PR?"},
	{"delete-data", "Overwrite the stored ledger snapshot or keep both copies?"},
	{"publication", "Is the publication of the design notes fine as drafted?"},
	{"live-infra", "Deploy the new runner image to the live cluster now?"},
	// HumanOnlySignals-only leads: no OneWayPatterns entry matches either, so they pin the
	// digest-list half of deskkit.OneWay (review finding cor-1688-C5).
	{"irreversible-only", "This change is irreversible."},
	{"budget-only", "It comes out of this quarter's budget."},
}

// TestNoticeLaneRefusesOneWayClasses — each one-way class stays on needs-decision whatever
// the filer's caught-by claim says: the lower layer (the one-way list) catches when the
// upper layer (the filer's claim) is wrong, for every class the skills name, not just the
// one needle row 5 exercises.
//
// FAIL-FIRST: against the gate as first submitted (the HumanOnlySignals substring list
// alone, admitting the notice lane on its absence), all 23 leads went red, e.g.:
//
//	--- FAIL: TestNoticeLaneRefusesOneWayClasses/tag-release
//	    one-way lead "Should we cut the v1.2.0 release tag now or after the next batch?" filed with labels [desk-decided], want needs-decision and no desk-decided
//	--- FAIL: TestNoticeLaneRefusesOneWayClasses/merge
//	    one-way lead "Merge this PR today or wait for the second approval?" filed with labels [desk-decided], want needs-decision and no desk-decided
//	--- FAIL: TestNoticeLaneRefusesOneWayClasses/gate-human-unspaced
//	    one-way lead "gate:human on brief 07 - the driver or the desk?" filed with labels [desk-decided], want needs-decision and no desk-decided
func TestNoticeLaneRefusesOneWayClasses(t *testing.T) {
	for _, tc := range oneWayLeads {
		t.Run(tc.name, func(t *testing.T) {
			withEnv(t)
			t.Setenv("FAKEGH_SEARCH_HITS", "[]")
			t.Setenv("FAKEGH_LABELS", labelsJSON(t, needsDecisionLabel, deskDecidedLabel))
			body := bodyFileWith(t, tc.lead+"\n\n"+neutralEvidence+"\n"+noticeLaneBlock)

			rc, out := runCapture([]string{"new", "-R", allowedRepo,
				"--title", reversibleTitle, "--body-file", body,
				"--label", needsDecisionLabel})
			if rc != deskkit.ExitOK {
				t.Fatalf("rc = %d, want 0 (filed, on needs-decision); out=%s", rc, out)
			}
			final := curForge.finalLabels()
			if !hasLabel(final, needsDecisionLabel) || hasLabel(final, deskDecidedLabel) {
				t.Errorf("one-way lead %q filed with labels %v, want needs-decision and no desk-decided", tc.lead, final)
			}
			if curForge.filed != nil && strings.Contains(curForge.filed.Body, deskDecidedMarker) {
				t.Errorf("one-way lead %q carries the notice-lane marker", tc.lead)
			}
		})
	}
}

// TestNoticeLaneFailsClosedWithoutReversibleSignal — two workable options and a held gate,
// no one-way term, but ALSO no R-3 reversible signal: an item the list cannot place stays
// with the human. Absence of a one-way match is not evidence of reversibility.
func TestNoticeLaneFailsClosedWithoutReversibleSignal(t *testing.T) {
	withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("FAKEGH_LABELS", labelsJSON(t, needsDecisionLabel, deskDecidedLabel))
	body := bodyFileWith(t, "Pick the retry backoff shape for the poller.\n\n"+neutralEvidence+"\n"+noticeLaneBlock)

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "pick the retry backoff shape for the poller", "--body-file", body,
		"--label", needsDecisionLabel})
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0; out=%s", rc, out)
	}
	if final := curForge.finalLabels(); !hasLabel(final, needsDecisionLabel) || hasLabel(final, deskDecidedLabel) {
		t.Errorf("an item with no reversible signal filed with %v, want needs-decision (fail closed)", final)
	}
}

// TestNoticeLaneRefusedByOneWayCallerLabel — a caller label that marks the item one-way
// (human-only, security, gate:human) keeps it on needs-decision even when the text is clean.
func TestNoticeLaneRefusedByOneWayCallerLabel(t *testing.T) {
	for _, lbl := range []string{"human-only", "security", "gate:human"} {
		t.Run(lbl, func(t *testing.T) {
			withEnv(t)
			t.Setenv("FAKEGH_SEARCH_HITS", "[]")
			t.Setenv("FAKEGH_LABELS", labelsJSON(t, needsDecisionLabel, deskDecidedLabel, lbl))
			body := bodyFileWith(t, "A reversible question.\n\n"+neutralEvidence+"\n"+noticeLaneBlock)

			rc, out := runCapture([]string{"new", "-R", allowedRepo,
				"--title", reversibleTitle, "--body-file", body,
				"--label", needsDecisionLabel, "--label", lbl})
			if rc != deskkit.ExitOK {
				t.Fatalf("rc = %d, want 0; out=%s", rc, out)
			}
			if final := curForge.finalLabels(); !hasLabel(final, needsDecisionLabel) || hasLabel(final, deskDecidedLabel) {
				t.Errorf("caller label %q: filed with %v, want needs-decision kept", lbl, final)
			}
		})
	}
}

// TestFewerThanTwoOptionsOneWayOffersNoReroute — a one-way item with one workable option is
// refused WITHOUT naming the --no-fork re-routes (every one of which files off the queue);
// the message offers only the audited --force-new --reason, which files it as needs-decision.
func TestFewerThanTwoOptionsOneWayOffersNoReroute(t *testing.T) {
	withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("FAKEGH_LABELS", labelsJSON(t, needsDecisionLabel))
	body := bodyFileWith(t, "The only way forward is to grant the App write access.\n\n"+onlyOneWorkableOptionBlock)

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "grant the App write access", "--body-file", body,
		"--label", needsDecisionLabel})
	if rc != deskkit.ExitRefused {
		t.Fatalf("rc = %d, want %d; out=%s", rc, deskkit.ExitRefused, out)
	}
	if curForge.filed != nil {
		t.Fatal("an issue was filed")
	}
	for _, v := range []string{noForkBriefContradicts, noForkWrongRepo, noForkToolFalsePositive} {
		if strings.Contains(out, v) {
			t.Errorf("a one-way refusal names the off-queue re-route %q: %s", v, out)
		}
	}
	if !strings.Contains(out, "--force-new --reason") {
		t.Errorf("the one-way refusal does not offer --force-new --reason: %s", out)
	}
}

// TestNoForkRefusesOneWayItem — --no-fork files off the queue, so it refuses an item that
// carries a one-way term (or a one-way caller label), for every re-route value.
func TestNoForkRefusesOneWayItem(t *testing.T) {
	cases := []struct {
		name, noFork, body string
		labels             []string
	}{
		{"wrong-repo-release", noForkWrongRepo, "Cutting the release tag belongs in example-org/other-repo.", nil},
		{"brief-contradicts-key", noForkBriefContradicts,
			"example-stream/07 says to rotate the signing key; `docs/keys/policy.md` says never.", nil},
		{"tool-false-positive-credential", noForkToolFalsePositive,
			"The scanner refused my credential rotation.\n\n```\nrefused: secret-shaped token\n```\n", nil},
		{"wrong-repo-human-only-label", noForkWrongRepo, "This belongs in example-org/other-repo.", []string{"human-only"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withEnv(t)
			t.Setenv("FAKEGH_SEARCH_HITS", "[]")
			t.Setenv("FAKEGH_LABELS", labelsJSON(t, append([]string{"to:worker", "to:desk", "bug"}, tc.labels...)...))
			body := bodyFileWith(t, tc.body)
			args := []string{"new", "-R", allowedRepo, "--title", "a routing note", "--body-file", body, "--no-fork", tc.noFork}
			for _, l := range tc.labels {
				args = append(args, "--label", l)
			}
			rc, out := runCapture(args)
			if rc != deskkit.ExitRefused {
				t.Fatalf("--no-fork %s on a one-way item: rc = %d, want %d; out=%s", tc.noFork, rc, deskkit.ExitRefused, out)
			}
			if curForge.filed != nil {
				t.Fatal("a one-way item was filed off the queue via --no-fork")
			}
			if !strings.Contains(out, "one-way") {
				t.Errorf("refusal does not say why (one-way): %s", out)
			}
		})
	}
}

// TestNoticeLaneAddsBeforeRemoving — the notice lane never leaves a filing on NEITHER
// label: desk-decided is applied alongside needs-decision first, and needs-decision comes
// off only in a second write. A failed second write exits 6 with the item still on the
// driver's queue.
func TestNoticeLaneAddsBeforeRemoving(t *testing.T) {
	withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("FAKEGH_LABELS", labelsJSON(t, needsDecisionLabel, deskDecidedLabel))
	t.Setenv("FAKEGH_LABEL_REMOVE_FAIL", "1")
	body := bodyFileWith(t, "A reversible docs-wording question.\n\n"+noticeLaneBlock)

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", reversibleTitle, "--body-file", body,
		"--label", needsDecisionLabel})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("rc = %d, want %d (the remove write failed, loudly); out=%s", rc, deskkit.ExitUnverifiable, out)
	}
	final := curForge.finalLabels()
	if !hasLabel(final, needsDecisionLabel) {
		t.Errorf("after a failed remove the issue carries %v — it must still be on needs-decision", final)
	}
	if !hasLabel(final, deskDecidedLabel) {
		t.Errorf("after a failed remove the issue carries %v — desk-decided was applied first", final)
	}
}

// TestNoticeLaneAuditRecordsLane — the local audit line says which lane the filing took, so
// "the tool took this off the human queue" is on the local trail, not only on the forge.
func TestNoticeLaneAuditRecordsLane(t *testing.T) {
	withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("FAKEGH_LABELS", labelsJSON(t, needsDecisionLabel, deskDecidedLabel))
	body := bodyFileWith(t, "A reversible docs-wording question.\n\n"+noticeLaneBlock)
	if rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", reversibleTitle, "--body-file", body,
		"--label", needsDecisionLabel}); rc != deskkit.ExitOK {
		t.Fatalf("rc = %d; out=%s", rc, out)
	}
	entries := readAudit(t)
	last := entries[len(entries)-1]
	if !strings.Contains(last.Detail, "lane=desk-decided") {
		t.Errorf("audit detail %q does not record lane=desk-decided", last.Detail)
	}
}

// TestNoForkAuditRecordsValue — the --no-fork value the filing used is on its audit line.
func TestNoForkAuditRecordsValue(t *testing.T) {
	withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("FAKEGH_LABELS", labelsJSON(t, "to:worker"))
	body := bodyFileWith(t, "This brief's work belongs in example-org/other-repo, not here.")
	if rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "this brief spans two repositories", "--body-file", body,
		"--no-fork", noForkWrongRepo}); rc != deskkit.ExitOK {
		t.Fatalf("rc = %d; out=%s", rc, out)
	}
	entries := readAudit(t)
	last := entries[len(entries)-1]
	if !strings.Contains(last.Detail, "no-fork="+noForkWrongRepo) {
		t.Errorf("audit detail %q does not record no-fork=%s", last.Detail, noForkWrongRepo)
	}
}

// TestDeskDecidedLabelCreatedOnFirstUse — the repo does not carry desk-decided yet: the
// filing still succeeds, and the tool's ensure-exists write names a colour (every other
// ensure-exists caller does; an empty colour is not a valid create).
func TestDeskDecidedLabelCreatedOnFirstUse(t *testing.T) {
	withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("FAKEGH_LABELS", labelsJSON(t, needsDecisionLabel)) // no desk-decided
	body := bodyFileWith(t, "A reversible docs-wording question.\n\n"+noticeLaneBlock)
	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", reversibleTitle, "--body-file", body,
		"--label", needsDecisionLabel})
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0 (desk-decided is created on first use); out=%s", rc, out)
	}
	var spec *deskkit.LabelSpec
	for i := range curForge.labelSpecs {
		if curForge.labelSpecs[i].Name == deskDecidedLabel {
			spec = &curForge.labelSpecs[i]
		}
	}
	if spec == nil {
		t.Fatalf("desk-decided was never applied: %v", curForge.appliedLabel)
	}
	if len(spec.Color) != 6 {
		t.Errorf("desk-decided create colour = %q, want 6 hex digits", spec.Color)
	}
}
