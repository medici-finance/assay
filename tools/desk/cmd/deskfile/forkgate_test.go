package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// forkgate_test.go — integration tests for the fork-test gate on `deskfile new`: what the CLI DOES with a parsed `### Fork test` block. The pure
// parser itself is table-tested in forktest_test.go.

// portOpeningQuestionBody is the fixture named in Verify row 2: a port-opening question
// whose block counts ONE option — otherwise well-formed (default/caught-by/ruled-check all
// present and valid). Neutral, public-repo-safe: no house names, no private issue numbers
// (ground rules).
//
// It also carries a valid `### Evidence` section (neutralEvidence, which holds no one-way
// term). Without one, the separate blocker-evidence gate refuses the same filing with the
// same exit code, so a test on this fixture could stay red with the fork-test gate removed
// and prove nothing about it. With it, the fork-test gate is the only gate left to refuse.
const portOpeningQuestionBody = `Should the metrics endpoint open a new port for the sidecar, or share the existing one?

` + neutralEvidence + "\n" + onlyOneWorkableOptionBlock

// TestNeedsDecisionOneWorkableOptionRefused — Verify row 2. A needs-decision filing whose
// fork-test block counts one workable option refuses (exit 5), files nothing, and names all
// three --no-fork re-routes.
//
// The fixture carries a valid `### Evidence` section, and the refusal is asserted NOT to be
// the blocker-evidence gate's, so the only gate that can turn this row green is the
// fork-test gate. Every assertion is non-fatal, so a red run reports all of them at once.
//
// FAIL-FIRST: with the fork-test gate call in cmdNew short-circuited (`if false && ...`):
//
//   - on the earlier fixture (no `### Evidence` section) the blocker-evidence gate still
//     refused with exit 5, so the exit-code assertion stayed GREEN and only the re-route
//     names went red, with the evidence gate's message as the refusal:
//
//     refusal does not name the --no-fork re-route "brief-contradicts-artifact": ...
//     refused: a filing labelled "needs-decision" is a blocker claim and must carry a `### Evidence` section ...
//
//   - on this fixture nothing refuses, the item is filed, and the row goes red on the
//     fork-test gate's own absence: rc 0, an issue filed, a create attempted, and none of
//     the three --no-fork re-routes named.
//
// regression: #3153
func TestNeedsDecisionOneWorkableOptionRefused(t *testing.T) {
	if !bodyHasEvidenceBlock(portOpeningQuestionBody) {
		t.Fatal("fixture: portOpeningQuestionBody must carry a valid `### Evidence` section")
	}
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("FAKEGH_LABELS", labelsJSON(t, needsDecisionLabel))
	body := bodyFileWith(t, portOpeningQuestionBody)

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "open a new port for the sidecar metrics endpoint", "--body-file", body,
		"--label", needsDecisionLabel})
	if rc != deskkit.ExitRefused {
		t.Errorf("one-workable-option filing rc = %d, want %d (exit 5); out=%s", rc, deskkit.ExitRefused, out)
	}
	if curForge.filed != nil {
		t.Error("an issue was filed despite fewer than two workable options")
	}
	if createArgv(*calls) != nil {
		t.Errorf("a create was attempted despite fewer than two workable options: %v", ghCalls(*calls))
	}
	if strings.Contains(out, "blocker claim") {
		t.Errorf("refused by the blocker-evidence gate, not the fork-test gate: %s", out)
	}
	for _, want := range []string{noForkBriefContradicts, noForkWrongRepo, noForkToolFalsePositive} {
		if !strings.Contains(out, want) {
			t.Errorf("refusal does not name the --no-fork re-route %q: %s", want, out)
		}
	}
}

// TestNoForkWrongRepoFiledWithoutDecisionLabel — Verify row 3. The "a brief that spans two
// repositories" shape (two prior routing-note instances, neutralised): re-filed with
// `--no-fork wrong-repo`, the created issue carries no needs-decision label, the
// `re-dispatch:` title prefix, and the worker-desk address (`to:worker` — the roster's role
// name for the worker-desk window; --to shares --raised-by's roster vocabulary, never the
// skill file name).
func TestNoForkWrongRepoFiledWithoutDecisionLabel(t *testing.T) {
	withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("FAKEGH_LABELS", labelsJSON(t, "to:worker"))
	body := bodyFileWith(t, "This brief's work belongs in example-org/other-repo, not here.")

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "this brief spans two repositories", "--body-file", body,
		"--no-fork", noForkWrongRepo})
	if rc != deskkit.ExitOK {
		t.Fatalf("--no-fork wrong-repo rc = %d, want 0; out=%s", rc, out)
	}
	if curForge.filed == nil {
		t.Fatal("the re-dispatch filing was not filed")
	}
	if !strings.HasPrefix(curForge.filed.Title, "re-dispatch:") {
		t.Errorf("title = %q, want the `re-dispatch:` prefix", curForge.filed.Title)
	}
	for _, l := range curForge.appliedLabel {
		if strings.EqualFold(l, needsDecisionLabel) {
			t.Fatalf("the re-dispatched issue carries %q, want no needs-decision label: %v", l, curForge.appliedLabel)
		}
	}
	foundTo := false
	for _, l := range curForge.appliedLabel {
		if strings.EqualFold(l, "to:worker") {
			foundTo = true
		}
	}
	if !foundTo {
		t.Errorf("applied labels %v do not carry the worker-desk address", curForge.appliedLabel)
	}
}

// TestCaughtByFilesNoticeWithMarker — Verify row 4. Two counted options plus
// `caught-by: draft-pr`, a reversible item (the title names a docs-wording change, an R-3 example)
// with no one-way term, files on the notice lane: the issue ends up labelled desk-decided
// (not needs-decision), and the body carries the shared `desk-r3-decision v1` marker and a
// `decision:` line equal to the default's text. The label writes are add-first: desk-decided
// lands alongside needs-decision, THEN needs-decision comes off.
func TestCaughtByFilesNoticeWithMarker(t *testing.T) {
	withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("FAKEGH_LABELS", labelsJSON(t, needsDecisionLabel, deskDecidedLabel))
	body := bodyFileWith(t, "A reversible docs-wording question.\n\n"+noticeLaneBlockWithSubject)

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", reversibleTitle, "--body-file", body,
		"--label", needsDecisionLabel})
	if rc != deskkit.ExitOK {
		t.Fatalf("notice-lane filing rc = %d, want 0; out=%s", rc, out)
	}
	if curForge.filed == nil {
		t.Fatal("the notice-lane filing was not filed")
	}
	if n := len(curForge.labelOps); n != 2 || len(curForge.labelOps[1].Remove) != 1 ||
		!strings.EqualFold(curForge.labelOps[1].Remove[0], needsDecisionLabel) {
		t.Errorf("label writes = %+v, want two: add (incl. desk-decided), then remove needs-decision", curForge.labelOps)
	}
	foundDecided, foundNeedsDecision := false, false
	for _, l := range curForge.finalLabels() {
		if strings.EqualFold(l, deskDecidedLabel) {
			foundDecided = true
		}
		if strings.EqualFold(l, needsDecisionLabel) {
			foundNeedsDecision = true
		}
	}
	if !foundDecided {
		t.Errorf("final labels %v do not carry %q", curForge.finalLabels(), deskDecidedLabel)
	}
	if foundNeedsDecision {
		t.Errorf("final labels %v still carry %q — the notice lane must remove it", curForge.finalLabels(), needsDecisionLabel)
	}
	if !strings.Contains(curForge.filed.Body, deskDecidedMarker) {
		t.Errorf("filed body does not carry the shared marker %q:\n%s", deskDecidedMarker, curForge.filed.Body)
	}
	if !strings.Contains(curForge.filed.Body, "decision: keep the current default") {
		t.Errorf("filed body's decision: line does not equal the default's text:\n%s", curForge.filed.Body)
	}
}

// TestOneWayTermOverridesCaughtBy — Verify row 5 (the negative-path row for the notice
// lane). Same fixture as row 4, plus a one-way term in the body ("security" — one of
// deskkit.HumanOnlySignals, shared with deskdigest's classifier) → filed under
// needs-decision regardless of the filer's caught-by claim: the lower layer (the one-way
// check) still catches when the upper layer (the filer's claim) is wrong. This row pins one
// needle; TestNoticeLaneRefusesOneWayClasses (forkgate_oneway_test.go) pins every one-way
// class the skills name, and TestNoticeLaneFailsClosedWithoutReversibleSignal pins that an
// item neither list recognises stays on the queue too.
func TestOneWayTermOverridesCaughtBy(t *testing.T) {
	withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("FAKEGH_LABELS", labelsJSON(t, needsDecisionLabel))
	// A one-way item stays on needs-decision, which is STILL a blocker claim: it needs the
	// SEPARATE `### Evidence` fence the blocker-evidence gate requires of every
	// needs-decision filing, one-way or not. neutralEvidence, NOT bodyWithEvidence: the
	// latter's "cluster" is itself a live-infrastructure OneWayPattern, which would let this
	// row pass with the HumanOnlySignals half of deskkit.OneWay disarmed. With it, the
	// `security` needle is the ONLY one-way term in the filing.
	//
	// FAIL-FIRST (review finding cor-1688-C5): with deskkit.OneWay's HumanOnlySignals branch
	// disarmed (`if s := ...; false && s != nil`), this row went red once switched to
	// neutralEvidence:
	//
	//	forkgate_test.go: applied labels [needs-decision desk-decided] dropped needs-decision despite the one-way term
	//
	// Round 5 (review finding cor-1688-C8): the fixture below used noticeLaneBlock (no
	// declared subject) until this round, so the filing failed closed on the missing subject
	// whether or not the one-way check ran — this row pinned nothing about the one-way layer.
	// noticeLaneBlockWithSubject gives it a real reversible subject, so disarming
	// deskkit.OneWayExempting (mutation M27) now turns this row red on its own — see
	// forkgate_oneway_test.go's noticeLaneBlockWithSubject comment for the shared FAIL-FIRST
	// evidence, captured once for all four affected tests.
	body := bodyFileWith(t, "This filing also touches a security control on the ledger boundary.\n\n"+
		neutralEvidence+"\n\n"+noticeLaneBlockWithSubject)

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", reversibleTitle, "--body-file", body,
		"--label", needsDecisionLabel})
	if rc != deskkit.ExitOK {
		t.Fatalf("one-way-term filing rc = %d, want 0; out=%s", rc, out)
	}
	if curForge.filed == nil {
		t.Fatal("the one-way-term filing was not filed")
	}
	foundNeedsDecision, foundDecided := false, false
	for _, l := range curForge.finalLabels() {
		if strings.EqualFold(l, needsDecisionLabel) {
			foundNeedsDecision = true
		}
		if strings.EqualFold(l, deskDecidedLabel) {
			foundDecided = true
		}
	}
	if !foundNeedsDecision {
		t.Errorf("applied labels %v dropped needs-decision despite the one-way term", curForge.appliedLabel)
	}
	if foundDecided {
		t.Errorf("applied labels %v took the notice lane despite the one-way term", curForge.appliedLabel)
	}
	if strings.Contains(curForge.filed.Body, deskDecidedMarker) {
		t.Errorf("filed body carries the notice-lane marker despite the one-way term:\n%s", curForge.filed.Body)
	}
}

// TestAttachUnaffectedByForkTest — Verify row 6 (neighbour). The fork-test gate binds `new`
// only: an `attach` observation on a needs-decision issue carries no fork-test requirement.
func TestAttachUnaffectedByForkTest(t *testing.T) {
	withEnv(t)
	t.Setenv("FAKEGH_ISSUE_STATE", "OPEN")
	body := bodyFileWith(t, "an observation with no fork-test block at all")

	rc, out := runCapture([]string{"attach", "-R", allowedRepo, "--to", "11", "--body-file", body})
	if rc != deskkit.ExitOK {
		t.Fatalf("attach rc = %d, want 0 (attach is unaffected by the fork-test gate); out=%s", rc, out)
	}
	if len(curForge.comments) == 0 {
		t.Fatal("the attach observation was not posted")
	}
}

// TestQuestionLabelUnaffectedByForkTest — Verify row 6 (neighbour). The fork-test gate binds
// `needs-decision` only: a `question` filing with no fork-test block is untouched by it (it
// still passes through the SEPARATE blocker-evidence gate, so this fixture carries an
// Evidence fence to isolate the fork-test assertion).
func TestQuestionLabelUnaffectedByForkTest(t *testing.T) {
	withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("FAKEGH_LABELS", labelsJSON(t, "question"))
	body := bodyFileWith(t, bodyWithEvidence) // no fork-test block anywhere

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "an open question with no fork test at all", "--body-file", body,
		"--label", "question"})
	if rc != deskkit.ExitOK {
		t.Fatalf("question-labelled filing rc = %d, want 0 (fork-test gate binds needs-decision only); out=%s", rc, out)
	}
	if curForge.filed == nil {
		t.Fatal("the question filing was not filed")
	}
}

// --- structural refusal + --force-new bypass (own coverage, not a named Verify row) --------

// TestForkTestMissingBlockRefusesNamingMissingLines proves the OTHER refusal shape the facts
// name: no block / an unparseable block / a missing line / a default naming no counted
// option all refuse citing what is missing, distinct from the "fewer than two options"
// re-route message.
func TestForkTestMissingBlockRefusesNamingMissingLines(t *testing.T) {
	withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("FAKEGH_LABELS", labelsJSON(t, needsDecisionLabel))
	body := bodyFileWith(t, "a needs-decision filing with no fork-test section at all")

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "a decision with no fork test block", "--body-file", body,
		"--label", needsDecisionLabel})
	if rc != deskkit.ExitRefused {
		t.Fatalf("rc = %d, want %d; out=%s", rc, deskkit.ExitRefused, out)
	}
	if strings.Contains(out, noForkWrongRepo) {
		t.Errorf("a missing-block refusal must not read as the fewer-than-two-options re-route: %s", out)
	}
	if !strings.Contains(out, "Fork test") {
		t.Errorf("refusal does not name the Fork test block: %s", out)
	}
}

// TestForkTestForceNewBypassesTheGate — the audited --force-new --reason escape hatch
// applies to the fork-test gate exactly as it does to dedupe and the evidence gate.
func TestForkTestForceNewBypassesTheGate(t *testing.T) {
	withEnv(t)
	t.Setenv("FAKEGH_LABELS", labelsJSON(t, needsDecisionLabel))
	body := bodyFileWith(t, "no fork-test block, filed under an outage")

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "an urgent decision during an outage", "--body-file", body,
		"--label", needsDecisionLabel, "--force-new", "--reason", "search + fork-test both unreachable"})
	if rc != deskkit.ExitOK {
		t.Fatalf("--force-new bypass rc = %d, want 0; out=%s", rc, out)
	}
	if curForge.filed == nil {
		t.Fatal("the force-new filing was not filed")
	}
}

// --- --no-fork brief-contradicts-artifact: the composed addressee and title prefix -------

// amendBody carries what --no-fork brief-contradicts-artifact requires of a body — a
// brief-id-shaped token and, backtick-quoted, the artifact path it contradicts — and no
// one-way term, so the re-route's own mapping is the only thing these tests exercise.
const amendBody = "example-stream/07 says the setup guide has three steps; `docs/guide/setup.md` has four.\n"

// runAmend files amendBody with --no-fork brief-contradicts-artifact under title, plus any
// extra arguments, against a repo carrying both addressee labels.
func runAmend(t *testing.T, title string, extra ...string) (int, string, *[][]string) {
	t.Helper()
	calls := withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("FAKEGH_LABELS", labelsJSON(t, "to:desk", "to:worker"))
	body := bodyFileWith(t, amendBody)
	args := append([]string{"new", "-R", allowedRepo, "--title", title, "--body-file", body,
		"--no-fork", noForkBriefContradicts}, extra...)
	rc, out := runCapture(args)
	return rc, out, calls
}

// TestNoForkAmendAddressesDesk — --no-fork brief-contradicts-artifact addresses the filing
// to the desk (`to:desk`) whether or not the caller passes --to, and a matching --to (any
// case) is accepted rather than refused.
//
// FAIL-FIRST: with the value's mapping removed from cmdNew (no addressee composed), the
// no-to case files with no `to:desk` label and goes red. The two --to cases pin that a
// matching --to is accepted; they supply the addressee themselves, so they stay green.
//
// regression: #3153
func TestNoForkAmendAddressesDesk(t *testing.T) {
	for _, tc := range []struct {
		name  string
		extra []string
	}{
		{"no-to", nil},
		{"to-desk", []string{"--to", "desk"}},
		{"to-Desk", []string{"--to", "Desk"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rc, out, _ := runAmend(t, "the setup guide step count", tc.extra...)
			if rc != deskkit.ExitOK {
				t.Fatalf("rc = %d, want 0; out=%s", rc, out)
			}
			if curForge.filed == nil {
				t.Fatal("the amend-brief filing was not filed")
			}
			if !hasLabel(curForge.appliedLabel, "to:desk") {
				t.Errorf("applied labels %v do not carry to:desk", curForge.appliedLabel)
			}
			if hasLabel(curForge.appliedLabel, "to:worker") {
				t.Errorf("applied labels %v carry to:worker, want only to:desk", curForge.appliedLabel)
			}
		})
	}
}

// TestNoForkAmendRefusesOtherTo — an explicit --to naming any role but the desk conflicts
// with the addressee the re-route composes: refused (exit 5) before anything is written,
// and the refusal names the addressee it requires.
//
// FAIL-FIRST: with the value's mapping removed from cmdNew, --to worker files the item
// addressed to the worker (rc 0) and this test goes red.
//
// regression: #3153
func TestNoForkAmendRefusesOtherTo(t *testing.T) {
	rc, out, calls := runAmend(t, "the setup guide step count", "--to", "worker")
	if rc != deskkit.ExitRefused {
		t.Fatalf("rc = %d, want %d; out=%s", rc, deskkit.ExitRefused, out)
	}
	if curForge.filed != nil || createArgv(*calls) != nil {
		t.Fatalf("a conflicting --to was filed anyway: %v", ghCalls(*calls))
	}
	if !strings.Contains(out, "--to desk") {
		t.Errorf("refusal does not name the required addressee --to desk: %s", out)
	}
}

// TestNoForkAmendPrefixesTitle — the `amend brief:` title prefix is composed by the tool,
// not trusted to free text: a title without it is filed WITH it (the tool prepends it; it
// does not refuse), and a title that already carries it, in any case, is not prefixed twice.
//
// FAIL-FIRST: with the value's mapping removed from cmdNew, the unprefixed title is filed
// as given and the first case goes red.
//
// regression: #3153
func TestNoForkAmendPrefixesTitle(t *testing.T) {
	for _, tc := range []struct{ title, want string }{
		{"the setup guide step count", "amend brief: the setup guide step count"},
		{"Amend brief: the setup guide step count", "Amend brief: the setup guide step count"},
	} {
		rc, out, _ := runAmend(t, tc.title)
		if rc != deskkit.ExitOK {
			t.Fatalf("title %q: rc = %d, want 0; out=%s", tc.title, rc, out)
		}
		if curForge.filed == nil {
			t.Fatalf("title %q: not filed", tc.title)
		}
		if got := curForge.filed.Title; got != tc.want {
			t.Errorf("title %q filed as %q, want %q", tc.title, got, tc.want)
		}
	}
}
