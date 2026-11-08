package main

// outcomeonly.go — the outcome-only landing mode (#5759).
//
// Every new outcome record names the commit at which this run's Evidence landed
// (validateOutcomeEvidenceBinding). The verdict gate (verdictgate.go) refuses
// that Evidence append for a BLOCKED or could-not-check re-run of a verified or
// done brief, so such a run has no Evidence to name, and without this mode its
// outcome could not be recorded at all.
//
// `--outcome-record <file> --outcome-only` records it. The record carries no
// Evidence sha: its `sha` is the admitted observation commit (the receipt's
// brief source), and it states why it names no Evidence with
//
//	"evidence_withheld": "non-decisive-verdict-on-verified-brief"
//
// The mode is accepted ONLY when the verdict gate would refuse this run's
// Evidence append: a non-decisive outcome (blocked or needs-context, never a
// verify-fail) on a brief whose board status is verified or done, read by the
// same statusgen check. Every other record keeps the Evidence binding, and a
// record that states evidence_withheld without the mode is refused, so the
// reason key can never stand in for landed Evidence.

import (
	"encoding/json"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const (
	evidenceWithheldKey    = "evidence_withheld"
	evidenceWithheldReason = "non-decisive-verdict-on-verified-brief"
)

// evidenceWithheld returns the record's evidence_withheld value and whether the
// key is present at all.
func evidenceWithheld(raw []byte) (string, bool) {
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return "", false
	}
	v, ok := m[evidenceWithheldKey]
	if !ok {
		return "", false
	}
	var s string
	if json.Unmarshal(v, &s) != nil {
		return string(v), true
	}
	return s, true
}

// nonDecisiveVerdictWord maps a non-decisive outcome to the Evidence verdict
// word a run with that outcome records; "" for a decisive or unknown outcome.
func nonDecisiveVerdictWord(outcome string) string {
	switch strings.ToLower(strings.TrimSpace(outcome)) {
	case "blocked":
		return "BLOCKED"
	case "needs_context", "needs-context":
		return "NEEDS-CONTEXT"
	}
	return ""
}

// checkOutcomeOnly admits an outcome-only record, or refuses it naming why.
// root is the tree the board status is read at (the landed closure root).
func checkOutcomeOnly(root string, receipt deskkit.VerifierReceipt, outcome deskkit.WakeReceipt, raw []byte) error {
	const mode = "refused: --outcome-only records a non-decisive re-run of a verified or done brief, whose Evidence append is refused"
	reason, ok := evidenceWithheld(raw)
	if !ok || reason != evidenceWithheldReason {
		return deskkit.Refused(mode + "; the record must state \"" + evidenceWithheldKey + "\":\"" + evidenceWithheldReason + "\"")
	}
	word := nonDecisiveVerdictWord(outcome.Outcome)
	if word == "" {
		return deskkit.Refused(mode + "; outcome " + `"` + outcome.Outcome + `"` +
			" is decisive or unknown — a verify-fail or verified outcome lands its Evidence first")
	}
	source := receipt.BriefSource()
	if source == "" || !strings.EqualFold(outcome.SHA, source) {
		return deskkit.Refused(mode + "; the record names no Evidence, so its sha must be the admitted observation commit " +
			source + ", got " + outcome.SHA)
	}
	brief := receipt.Binding.Brief
	if brief == "" {
		return deskkit.Refused(mode + "; the admitted receipt names no brief")
	}
	verdict, why, err := evidenceVerdictCheckFn(root, brief, nil, nil, false, word)
	if err != nil {
		return err
	}
	if verdict != closureNotAccepted {
		msg := mode + "; an Evidence append of VERIFY: " + word + " to " + brief + " is not refused here"
		if why != "" {
			msg += " (" + why + ")"
		}
		return deskkit.Refused(msg + " — land the Evidence first, then its outcome record")
	}
	return nil
}

// gateOutcomeEvidence is the outcome record's Evidence step: the Evidence binding
// for an ordinary record, checkOutcomeOnly under --outcome-only.
func gateOutcomeEvidence(outcomeOnly bool, evidenceRoot, landedRoot string, fg deskkit.Forge, fr deskkit.ForgeRepo, closureRef string, receipt deskkit.VerifierReceipt, outcome deskkit.WakeReceipt, raw []byte) error {
	if outcomeOnly {
		return checkOutcomeOnly(landedRoot, receipt, outcome, raw)
	}
	if _, ok := evidenceWithheld(raw); ok {
		return deskkit.Refused("refused: the outcome record states " + evidenceWithheldKey +
			" but was not landed with --outcome-only; every other outcome record names its landed Evidence")
	}
	return outcomeEvidenceBindingFn(evidenceRoot, fg, fr, closureRef, receipt, outcome.SHA)
}
