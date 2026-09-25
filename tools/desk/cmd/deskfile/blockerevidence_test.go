package main

import (
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// blockerevidence_test.go — the blocker-evidence gate and the correction-capture (skill-bug)
// composition on `deskfile new` (brief 07 of its tracking stream).
//
// The gate is the TOOL half of a two-layer rule (the desk skills carry the other): a `new`
// filing labelled with an escalation label (needs-decision / help wanted / question) is a
// blocker CLAIM, and a blocker claim with nothing to quote is not a blocker claim — so it
// REFUSES (exit 5) unless the body carries a `### Evidence` heading followed by a fenced
// block. `human-only` is not in the set (an act, not a claim) and `attach` is unaffected.
//
// The correction-capture path composes a skill-bug body from this session's last receipt,
// and REFUSES when no receipt was recorded in the window — a correction with nothing to
// correct is not a skill-bug.

const bodyWithEvidence = `The cluster is unreachable from this agent.

### Evidence

` + "```" + `
$ kubectl get pods
error: could not reach the API server
` + "```" + `
`

const bodyNoEvidence = "I believe the cluster is unreachable, but I did not run anything."

// bodyNoEvidenceForLabel is bodyNoEvidence, plus a well-formed `### Fork test` block when
// label is needs-decision — the fork-test gate runs BEFORE the
// blocker-evidence gate this file tests, so a needs-decision fixture with no fork-test block
// would be refused for the WRONG reason (missing fork-test, not missing evidence). The block
// here counts two options with `caught-by: nothing`, so it is otherwise an ordinary
// needs-decision filing (no notice-lane relabel) and isolates the evidence assertion.
func bodyNoEvidenceForLabel(label string) string {
	if strings.EqualFold(label, needsDecisionLabel) {
		return bodyNoEvidence + "\n\n" + validForkTestBlock
	}
	return bodyNoEvidence
}

// TestEscalationWithoutEvidenceRefused — Verify row 2 (negative path). Each of the three
// escalation labels refuses an evidence-less body (exit 5) with a message naming `### Evidence`.
func TestEscalationWithoutEvidenceRefused(t *testing.T) {
	for _, label := range []string{"needs-decision", "help wanted", "question"} {
		t.Run(label, func(t *testing.T) {
			calls := withEnv(t)
			t.Setenv("FAKEGH_SEARCH_HITS", "[]")
			t.Setenv("FAKEGH_LABELS", labelsJSON(t, label))
			body := bodyFileWith(t, bodyNoEvidenceForLabel(label))

			rc, out := runCapture([]string{"new", "-R", allowedRepo,
				"--title", "a blocker on the settlement path", "--body-file", body,
				"--label", label})
			if rc != deskkit.ExitRefused {
				t.Fatalf("evidence-less %q filing rc = %d, want %d (exit 5); out=%s",
					label, rc, deskkit.ExitRefused, out)
			}
			if !strings.Contains(out, "### Evidence") {
				t.Fatalf("refusal message for %q does not name `### Evidence`: %s", label, out)
			}
			// The gate must refuse BEFORE the create is reached — no issue may be filed.
			if curForge.filed != nil {
				t.Fatalf("an issue was filed despite the missing evidence for %q", label)
			}
			if createArgv(*calls) != nil {
				t.Fatalf("a create was attempted despite the missing evidence for %q", label)
			}
		})
	}
}

// TestEscalationWithEvidenceFencePasses — Verify row 2 (positive path). A `### Evidence`
// heading followed by a fenced block satisfies the gate: the filing proceeds (exit 0).
func TestEscalationWithEvidenceFencePasses(t *testing.T) {
	withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("FAKEGH_LABELS", labelsJSON(t, "needs-decision"))
	// + a well-formed fork-test block (the fork-test gate runs before this one; see
	// bodyNoEvidenceForLabel above for why a needs-decision fixture needs it too).
	body := bodyFileWith(t, bodyWithEvidence+"\n\n"+validForkTestBlock)

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "a blocker on the settlement path", "--body-file", body,
		"--label", "needs-decision"})
	if rc != deskkit.ExitOK {
		t.Fatalf("filing WITH an evidence fence rc = %d, want 0; out=%s", rc, out)
	}
	if curForge.filed == nil {
		t.Fatal("an evidence-carrying escalation filing was not filed")
	}
}

// TestHumanOnlyNotEvidenceGated — Verify row 3 (neighbour). `human-only` is an ACT, not a
// claim (brief 05 of its tracking stream), so it is NOT in the escalation set: an evidence-less
// human-only filing files (exit 0).
func TestHumanOnlyNotEvidenceGated(t *testing.T) {
	withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("FAKEGH_LABELS", labelsJSON(t, "human-only"))
	body := bodyFileWith(t, bodyNoEvidence)

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "a human act to perform on the roster repos", "--body-file", body,
		"--label", "human-only"})
	if rc != deskkit.ExitOK {
		t.Fatalf("evidence-less human-only filing rc = %d, want 0 (not evidence-gated); out=%s", rc, out)
	}
	if curForge.filed == nil {
		t.Fatal("the human-only filing was not filed")
	}
}

// TestAttachUnaffected — Verify row 3 (neighbour). The gate binds `new` only: an `attach`
// observation carries no label and is not a fresh blocker claim, so an evidence-less attach
// posts unimpeded (exit 0).
func TestAttachUnaffected(t *testing.T) {
	withEnv(t)
	t.Setenv("FAKEGH_ISSUE_STATE", "OPEN")
	body := bodyFileWith(t, bodyNoEvidence)

	rc, out := runCapture([]string{"attach", "-R", allowedRepo, "--to", "11", "--body-file", body})
	if rc != deskkit.ExitOK {
		t.Fatalf("evidence-less attach rc = %d, want 0 (attach is unaffected); out=%s", rc, out)
	}
	if len(curForge.comments) == 0 {
		t.Fatal("the attach observation was not posted")
	}
}

// TestEscalationForceNewBypassesEvidenceGate — the refusal takes the audited
// --force-new --reason bypass, as every deskfile refusal does. With it, an evidence-less
// escalation files (exit 0) — the escape hatch for a blocker whose evidence genuinely
// cannot be produced.
func TestEscalationForceNewBypassesEvidenceGate(t *testing.T) {
	withEnv(t)
	t.Setenv("FAKEGH_LABELS", labelsJSON(t, "question"))
	body := bodyFileWith(t, bodyNoEvidence)

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "an unanswerable question during an outage", "--body-file", body,
		"--label", "question", "--force-new", "--reason", "the probe endpoint is itself down"})
	if rc != deskkit.ExitOK {
		t.Fatalf("--force-new evidence-less escalation rc = %d, want 0 (bypass); out=%s", rc, out)
	}
	if curForge.filed == nil {
		t.Fatal("the force-new escalation was not filed")
	}
}

// --- correction capture (skill-bug composition) -----------------------------------

const (
	corrRestatement = "start the next batch now"
	corrMessage     = "no, that isn't what I asked — look again"
	corrSection     = "worker-desk HARD-GATE-idle-claims"
	corrReading     = "an idle claim needs a fresh sweep, not an assumption"
)

// seedReceipt writes one receipt to the test session's beacon, stamped ts (empty ts → now).
func seedReceipt(t *testing.T, ts string) {
	t.Helper()
	if _, err := deskkit.AppendAck(deskkit.SessionTag(), deskkit.AckRecord{
		TS:          ts,
		Role:        "worker-desk",
		Repo:        "assay",
		Restatement: corrRestatement,
	}); err != nil {
		t.Fatalf("seed receipt: %v", err)
	}
}

// TestSkillBugRequiresRecentReceipt — Verify row 3a (mutation). Composition REFUSES (exit 5)
// when no receipt was recorded in the window: no beacon at all, and a receipt older than the
// window both refuse.
func TestSkillBugRequiresRecentReceipt(t *testing.T) {
	t.Run("no receipt at all", func(t *testing.T) {
		withEnv(t)
		t.Setenv("FAKEGH_LABELS", labelsJSON(t, skillBugLabel))
		rc, out := runCapture([]string{"new", "-R", allowedRepo, "--label", skillBugLabel,
			"--correction", corrMessage, "--section", corrSection, "--reading", corrReading})
		if rc != deskkit.ExitRefused {
			t.Fatalf("compose with no receipt rc = %d, want %d (exit 5); out=%s", rc, deskkit.ExitRefused, out)
		}
		if curForge.filed != nil {
			t.Fatal("a skill-bug was filed despite no receipt to correct")
		}
	})
	t.Run("stale receipt outside the window", func(t *testing.T) {
		withEnv(t)
		t.Setenv("FAKEGH_LABELS", labelsJSON(t, skillBugLabel))
		stale := time.Now().Add(-(skillBugReceiptWindow + time.Hour)).UTC().Format(time.RFC3339)
		seedReceipt(t, stale)
		rc, out := runCapture([]string{"new", "-R", allowedRepo, "--label", skillBugLabel,
			"--correction", corrMessage, "--section", corrSection, "--reading", corrReading})
		if rc != deskkit.ExitRefused {
			t.Fatalf("compose with a stale receipt rc = %d, want %d (exit 5); out=%s", rc, deskkit.ExitRefused, out)
		}
		if curForge.filed != nil {
			t.Fatal("a skill-bug was filed against a stale receipt")
		}
	})
}

// TestSkillBugBodyComposed — Verify row 3a. With a fresh receipt the composition succeeds
// (exit 0) and the filed body carries all five fields — receipt line, correction verbatim,
// $DESK_LOOP, skill+section, the desk's reading — and the title is composed from the section.
func TestSkillBugBodyComposed(t *testing.T) {
	withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("FAKEGH_LABELS", labelsJSON(t, skillBugLabel))
	seedReceipt(t, "") // now

	rc, out := runCapture([]string{"new", "-R", allowedRepo, "--label", skillBugLabel,
		"--correction", corrMessage, "--section", corrSection, "--reading", corrReading})
	if rc != deskkit.ExitOK {
		t.Fatalf("compose with a fresh receipt rc = %d, want 0; out=%s", rc, out)
	}
	if curForge.filed == nil {
		t.Fatal("the skill-bug was not filed")
	}
	filed := curForge.filed
	// Title is composed from the section.
	if want := "skill-bug: " + corrSection; filed.Title != want {
		t.Fatalf("composed title = %q, want %q", filed.Title, want)
	}
	// All five fields present in the composed body.
	for _, want := range []string{
		corrRestatement,              // 1. the receipt line
		corrMessage,                  // 2. the correction, verbatim
		"**Desk loop:** worker-desk", // 3. $DESK_LOOP
		corrSection,                  // 4. the skill + section
		corrReading,                  // 5. the desk's reading
	} {
		if !strings.Contains(filed.Body, want) {
			t.Fatalf("composed body is missing %q\n--- body ---\n%s", want, filed.Body)
		}
	}
}

// TestSkillBugModeRejectsTitleAndBodyFile — the tool composes the title and body in
// skill-bug mode, so passing --title or --body-file is a caller error (exit 5): the
// composition owns them.
func TestSkillBugModeRejectsTitleAndBodyFile(t *testing.T) {
	t.Run("--title rejected", func(t *testing.T) {
		withEnv(t)
		rc, out := runCapture([]string{"new", "-R", allowedRepo, "--label", skillBugLabel,
			"--correction", corrMessage, "--section", corrSection, "--reading", corrReading,
			"--title", "hand-written"})
		if rc != deskkit.ExitRefused {
			t.Fatalf("--title with --correction rc = %d, want %d; out=%s", rc, deskkit.ExitRefused, out)
		}
	})
	t.Run("--body-file rejected", func(t *testing.T) {
		withEnv(t)
		body := bodyFileWith(t, "hand-written")
		rc, out := runCapture([]string{"new", "-R", allowedRepo, "--label", skillBugLabel,
			"--correction", corrMessage, "--section", corrSection, "--reading", corrReading,
			"--body-file", body})
		if rc != deskkit.ExitRefused {
			t.Fatalf("--body-file with --correction rc = %d, want %d; out=%s", rc, deskkit.ExitRefused, out)
		}
	})
	t.Run("--correction requires --label skill-bug", func(t *testing.T) {
		withEnv(t)
		rc, out := runCapture([]string{"new", "-R", allowedRepo,
			"--correction", corrMessage, "--section", corrSection, "--reading", corrReading})
		if rc != deskkit.ExitRefused {
			t.Fatalf("--correction without --label skill-bug rc = %d, want %d; out=%s", rc, deskkit.ExitRefused, out)
		}
	})
}

// redPreflightOutput is the shape of a red `deskroster preflight` run as deskboot captures it
// (config banner stripped): the summary line carrying each blocking check's detail and
// remediation verbatim.
const redPreflightOutput = "preflight role=worker RED 4/6 checked-clean · ambient-identity=checked-failed: " +
	"the ambient gh login is mallory, a non-blessing login → fix: switch the interactive gh identity [#1527] " +
	"· write-transport=could-not-check: no remote named origin in /home/someone/checkout → fix: run the landing probe by hand"

// TestPreflightAlarmBodyPassesBlockerEvidenceGate is the cross-tool contract for deskboot's
// red-preflight alarm: the body deskboot composes (deskkit.PreflightAlarmBody) under the label
// it files with (deskkit.PreflightAlarmLabel) must be ACCEPTED by this tool's blocker-evidence
// gate. deskboot's own tests stub deskfile, so they can only check its argv; this is where the
// gate that actually decides lives.
//
// FAIL-FIRST: the alarm body shipped before this contract opened its fence under the prose
// line "The roster's own verdict, verbatim:" with no `### Evidence` heading, so this filing
// was refused exit 5 and the alarm never reached anyone.
func TestPreflightAlarmBodyPassesBlockerEvidenceGate(t *testing.T) {
	withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("FAKEGH_LABELS", labelsJSON(t, deskkit.PreflightAlarmLabel))
	body := bodyFileWith(t, deskkit.PreflightAlarmBody("worker", redPreflightOutput))

	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", deskkit.PreflightAlarmTitle("worker", "2026-01-02"), "--body-file", body,
		"--label", deskkit.PreflightAlarmLabel})
	if rc != deskkit.ExitOK {
		t.Fatalf("the composed alarm body was refused by deskfile (rc %d, want 0) — deskboot's alarm "+
			"would never file; out=%s", rc, out)
	}
	if curForge.filed == nil {
		t.Fatal("the composed alarm body was not filed")
	}
}

// TestDedupeRefusalCarriesTheSharedPrefix pins the other half of the alarm contract: the
// title-dedupe refusal is the ONE exit 5 deskboot may read as "already filed", and it
// recognises it by deskkit.DedupeRefusalPrefix. Any other exit-5 refusal must not carry it.
func TestDedupeRefusalCarriesTheSharedPrefix(t *testing.T) {
	withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", searchHitsJSON(t, "oracle price feed goes stale"))
	body := bodyFileWith(t, "the oracle price feed is going stale under load")
	rc, out := runCapture([]string{"new", "-R", allowedRepo,
		"--title", "oracle price feed goes stale", "--body-file", body})
	if rc != deskkit.ExitRefused || !strings.Contains(out, deskkit.DedupeRefusalPrefix) {
		t.Fatalf("dedupe refusal rc=%d, want 5 carrying %q; out=%s", rc, deskkit.DedupeRefusalPrefix, out)
	}

	// The blocker-evidence refusal is ALSO exit 5 — it must be distinguishable.
	withEnv(t)
	t.Setenv("FAKEGH_SEARCH_HITS", "[]")
	t.Setenv("FAKEGH_LABELS", labelsJSON(t, "help wanted"))
	body = bodyFileWith(t, bodyNoEvidence)
	rc, out = runCapture([]string{"new", "-R", allowedRepo,
		"--title", "a blocker on the settlement path", "--body-file", body, "--label", "help wanted"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("evidence-less escalation rc=%d, want 5; out=%s", rc, out)
	}
	if strings.Contains(out, deskkit.DedupeRefusalPrefix) {
		t.Fatalf("the evidence-gate refusal carries the dedupe prefix, so deskboot would read it as "+
			"'already filed'; out=%s", out)
	}
}
