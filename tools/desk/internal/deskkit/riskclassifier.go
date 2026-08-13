package deskkit

import "strings"

// The RiskClassifier seam.
//
// riskpath.go states the contract this file must preserve verbatim: classification
// may only ever WIDEN, adopter configuration is ADDITIVE-ONLY, and every uncertain
// input answers TRUE (C-10 fail-closed). Cutting a seam under the gate is exactly
// the kind of refactor that can quietly trade one of those away, so the split here
// is drawn along a specific line:
//
//   - the MECHANISM half — the fail-closed enumeration RiskPathTriggered's doc
//     comment lists — stays OUTSIDE every classifier, in riskClassify below. It is
//     not policy and it is not pluggable: no classifier, present or future, is
//     consulted on an unknown repo, a public or visibility-unstated repo, an empty
//     changed-file list, or a blank path entry, because on those inputs there is
//     nothing to classify and the answer is already true.
//
//   - the POLICY half — "do these changed files touch something sensitive" — is
//     what a RiskClassifier answers. Today there is exactly one implementation,
//     patternClassifier, carrying the compiled path-trigger logic unchanged.
//
// Union dispatch over several classifiers arrives later; this
// brief is behavior-identical by construction, and both exported accessors
// (RiskPathTriggered, RiskClassReason) are thin readings of the SAME call, so the
// decision and the reason it prints cannot drift apart.

// RiskClassifier decides the POLICY half of risk classification: whether a PR's
// changed files, on a repo that has already cleared the mechanism-side fail-closed
// preconditions, are sensitive enough to require a `Security-Review: pass` verdict
// before the desk may flip the PR ready.
//
// Implementations MUST FAIL CLOSED — return true — on ANY uncertainty. There is no
// error return and no third state on purpose: an implementation that cannot decide
// has exactly one correct answer, and it is "classed". Answering false is a positive
// claim that the gate may be waived for this diff, so it may only be made about
// input the implementation fully understood.
//
// reason is a short human-readable explanation carried into refusal messages. An
// implementation returning classed=false MUST return the exact string
// "not risk-classed" — RiskClassReason's contract is that it says "not risk-classed"
// exactly when RiskPathTriggered answers false, and a test walks every case through
// both accessors to hold it.
type RiskClassifier interface {
	Classify(repo string, changedFiles []string) (classed bool, reason string)
}

// notRiskClassed is the ONE string that means "the gate may be waived for this
// diff". It is a constant so the agreement between RiskPathTriggered and
// RiskClassReason is textual, not remembered.
const notRiskClassed = "not risk-classed"

// patternClassifier is today's classification, unchanged: a path-pattern match over
// the compiled universal base list, the repo's compiled additions, and any adopter
// additions from ASSAY_RISK_PATH_TRIGGERS_EXTRA — unioned by RiskPathTriggersFor,
// which is where the additive-only property lives. This type adds NO configuration
// and reads NO environment of its own.
type patternClassifier struct{}

var _ RiskClassifier = patternClassifier{}

// unionClassifier ORs its members: risk-classed if ANY member says so
// (the additive/only-widens design). This is where the additive/only-widens
// property becomes structural rather than a convention — there is no code path in
// Classify that lets one member's "not classed" answer suppress another member's
// "classed" one, and no member is ever consulted to VETO another's verdict.
//
// The reasons of every FIRING member are joined (not just the first), so a
// refusal message can say every mode that fired rather than picking one
// arbitrarily. A member is anything implementing RiskClassifier — most simply
// patternClassifier{} (never abstains: it always answers true or
// notRiskClassed) and calloutClassifier{} (abstains — contributes false,
// notRiskClassed — when ASSAY_RISK_CALLOUT is unset).
type unionClassifier struct {
	members []RiskClassifier
}

var _ RiskClassifier = unionClassifier{}

// Classify implements RiskClassifier by ORing every member's verdict.
func (u unionClassifier) Classify(repo string, changedFiles []string) (bool, string) {
	classed := false
	var reasons []string
	for _, m := range u.members {
		if m == nil {
			continue
		}
		c, r := m.Classify(repo, changedFiles)
		if !c {
			continue
		}
		classed = true
		if r != "" && r != notRiskClassed {
			reasons = append(reasons, r)
		}
	}
	if !classed {
		return false, notRiskClassed
	}
	if len(reasons) == 0 {
		// Defensive: a member answered classed=true without a distinguishing
		// reason. This must still render as classed, never silently as clean.
		reasons = []string{"risk-classed (no reason given)"}
	}
	return true, strings.Join(reasons, "; ")
}

// defaultRiskClassifier is the classifier the exported accessors dispatch to. It is
// a package-level var of interface type so tests can drive riskClassify with a stub;
// nothing outside this package can replace it, and no configuration selects it.
//
// It is the UNION of the compiled pattern classifier and
// the adopter-executable callout. calloutClassifier reads ASSAY_RISK_CALLOUT fresh
// from EffectiveConfig() on every call rather than capturing a path here, so this
// var does not need to change when the configured callout does — it stays the same
// union value across ReloadConfig() calls, and the union's own only-widens
// structure is what keeps an unset callout byte-identical to the unset baseline.
var defaultRiskClassifier RiskClassifier = unionClassifier{
	members: []RiskClassifier{patternClassifier{}, calloutClassifier{}},
}

// Classify implements RiskClassifier over the compiled + configured trigger set.
// It fails closed on a blank path entry: a malformed listing is not evidence of a
// clean file. (riskClassify screens blank entries before dispatch as well — the
// guard is kept here too because the interface REQUIRES each implementation to fail
// closed on its own, and because pathTriggerHit reaches this method directly.)
func (patternClassifier) Classify(repo string, changedFiles []string) (bool, string) {
	triggers := RiskPathTriggersFor(repo)
	for _, f := range changedFiles {
		if strings.TrimSpace(f) == "" {
			return true, pathTriggerReason(repo)
		}
		for _, pat := range triggers {
			if matchTrigger(pat, f) {
				return true, pathTriggerReason(repo)
			}
		}
	}
	return false, notRiskClassed
}

// pathTriggerReason is the wording a path hit reports. It is preserved VERBATIM
// from the pre-seam implementation — including for a blank path entry, which
// reported this same string before the seam was cut. Rewording it would be a
// behavior change, which this brief does not make.
func pathTriggerReason(repo string) string { return "touches a security path for " + repo }

// riskClassify is the whole gate: the mechanism-side fail-closed enumeration, then
// dispatch to the classifier. It returns the decision AND its reason from one pass,
// in the order RiskPathTriggered's doc comment states, so the two exported
// accessors cannot disagree.
//
// Each early return here is a case where NO classifier runs. That is deliberate:
// these are not "the pattern classifier said true", they are "there was nothing to
// classify", and a plug-in must never be in a position to answer them.
func riskClassify(c RiskClassifier, repo string, changedFiles []string) (bool, string) {
	// (1) Unknown repo → fail closed. Mirrors CIRequired: a repo outside the fixed set
	// has no policy, and no policy never means "waived".
	if !IsAllowedRepo(repo) {
		return true, "repo is outside the fixed desk set — no policy, fail closed"
	}
	// (2) Visibility alone can risk-class: public, or a policy entry with no stated
	// visibility (the zero value). Only a known-private repo continues to the paths.
	// VisibilityRiskClassed is the DECISION (everything except known-private); the
	// switch below only picks which of the two wordings to report.
	if VisibilityRiskClassed(repo) {
		if RepoVisibility(repo) == VisibilityPublic {
			return true, "PUBLIC repo — every PR on a public repo is risk-classed"
		}
		return true, "repo visibility is not stated in the compiled-in policy — fail closed"
	}
	// (3) No changed files → we learned nothing about the diff. Fail closed.
	if len(changedFiles) == 0 {
		return true, "no changed files could be read for this PR — fail closed"
	}
	// (4) A blank/empty path entry → a malformed listing is not evidence. Screened
	// before dispatch so this can never depend on a classifier choosing to check it.
	for _, f := range changedFiles {
		if strings.TrimSpace(f) == "" {
			return true, pathTriggerReason(repo)
		}
	}
	// (5) Policy half — and only now is a classifier consulted.
	return c.Classify(repo, changedFiles)
}
