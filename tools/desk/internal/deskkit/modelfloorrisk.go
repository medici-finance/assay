package deskkit

// modelfloorrisk.go — the model-capability floor is RISK-CONDITIONAL on an UNSTAMPED PR.
//
// WHAT THIS CLOSES. The floor's permissive branch — proceed-with-NOTICE — is right for an
// unattested lane on an unremarkable PR: a human-driven session and a pre-attestation
// dispatch must not be bricked, and an `any`-tier or aged-out stamp carries no strength
// claim, so all three read as UNSTAMPED-for-strength and proceed. But "unstamped therefore
// proceed" was BLANKET, and that left the floor strict against an honest below-tier
// attestation while permissive against NO attestation at all. On a RISK-CLASSED PR that is
// the wrong way round: an authority-bearing write there — a security-review-bearing verdict,
// a ready-flip — must carry a trustable attestation of the tier that produced it, and an
// unstamped one carries none. A stamp anyone could self-apply is not attestation; the ABSENCE
// of one is not attestation either.
//
// THE RULE. An unstamped NON-risk PR proceeds with a NOTICE, exactly as before. An unstamped
// RISK-CLASSED PR REFUSES. This is layered OVER the base floor rather than folded into it: the
// base floor decides the tier/attestation question (modelfloor.go), and this overlay converts
// its one permissive outcome — FloorNoticeAllow, the single signal the base floor already
// emits for all THREE unstamped-for-strength branches (no stamp, an `any` tier, a stamp aged
// out) — into a refusal when, and only when, the PR is risk-classed. Every other outcome
// passes through untouched: an attested strong PR still proceeds, an attested below-strong PR
// still refuses, a present-but-unreadable stamp still refuses, and the loud incident-recovery
// override still bypasses everything.
//
// WHY IT REUSES THE SECURITY-GATE SIGNAL. "Risk-classed" here is the SAME determination the
// ready-flip's security-review gate makes — RiskPathTriggered over the PR's changed files
// (path triggers, plus the visibility rule that makes every public-repo PR risk-classed). The
// floor does not invent a second notion of risk; it reads the one already in the tree, so a PR
// is risk-classed to the floor exactly when it is risk-classed to the gate that demands a
// `Security-Review: pass`. The reducer here is the seam that keeps that true.
//
// WHICH DIRECTION IT FAILS. FloorRisk's ZERO VALUE is Unknown, and Unknown REFUSES an
// unstamped PR — the fail-closed direction. A caller resolves readability FIRST (a diff it
// could not read in full is could-not-check, and it surfaces that with its own exit code
// BEFORE the floor), so FloorRiskOf is reached only with a diff read in full and returns a
// definite answer; the Unknown zero value is the backstop for a caller that forgets, never the
// normal path.

// FloorRisk is the risk classification the model-capability floor consumes to decide whether
// an UNSTAMPED PR proceeds-with-NOTICE or refuses. It is three-state on purpose: the zero
// value is the non-answer and fails closed, so a consumer that never resolved risk refuses an
// unstamped write rather than waving it through.
type FloorRisk int

const (
	// FloorRiskUnknown is the ZERO VALUE: the PR's risk class was not established. It fails
	// CLOSED — an unstamped write is REFUSED, never proceeded — so a caller that forgets to
	// resolve risk cannot silently reopen the hole this overlay closes.
	FloorRiskUnknown FloorRisk = iota
	// FloorNotRiskClassed: the diff was read in full and positively cleared the classifier.
	// This is the ONLY value that keeps the permissive NOTICE branch for an unstamped PR.
	FloorNotRiskClassed
	// FloorRiskClassed: the PR is risk-classed (a security path, a risk-classed visibility,
	// or the classifier's own fail-closed answer). An unstamped write on it refuses.
	FloorRiskClassed
)

func (r FloorRisk) String() string {
	switch r {
	case FloorNotRiskClassed:
		return "not-risk-classed"
	case FloorRiskClassed:
		return "risk-classed"
	default:
		return "unknown"
	}
}

// RefusesUnstamped reports whether this risk class turns the base floor's proceed-with-NOTICE
// outcome into a refusal. Only a POSITIVELY not-risk-classed PR keeps the NOTICE; a
// risk-classed PR and an unresolved (Unknown) one both refuse — the fail-closed line drawn
// once, as a question the wrapper asks rather than a comparison it open-codes.
func (r FloorRisk) RefusesUnstamped() bool { return r != FloorNotRiskClassed }

// FloorRiskOf reduces a repo and its FULLY-READ changed-file list to a FloorRisk, reusing the
// exact RiskClassifier the security-review gate consults (RiskPathTriggered). The caller must
// have already established that the diff was read in full — a failed or short read is
// could-not-check and the caller surfaces it with its own exit code BEFORE reaching here, so
// this reducer never has to represent that state and never returns FloorRiskUnknown itself.
//
// RiskPathTriggered is itself fail-closed (an unknown repo, a risk-classed visibility, an
// empty or blank-entry file list all answer true), so an empty changedFiles slice classifies
// rather than clears — the same answer the security gate would give.
func FloorRiskOf(repo string, changedFiles []string) FloorRisk {
	if RiskPathTriggered(repo, changedFiles) {
		return FloorRiskClassed
	}
	return FloorNotRiskClassed
}
