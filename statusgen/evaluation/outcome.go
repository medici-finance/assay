package evaluation

import "strings"

// VerificationOutcome is the overall judgment recorded for a brief's run.
// Row results such as "pass" belong to a different vocabulary.
type VerificationOutcome string

const (
	OutcomeUnknown      VerificationOutcome = ""
	OutcomeVerified     VerificationOutcome = "verified"
	OutcomeFailed       VerificationOutcome = "verify-fail"
	OutcomeBlocked      VerificationOutcome = "blocked"
	OutcomeNeedsContext VerificationOutcome = "needs-context"
)

// ParseVerificationOutcome accepts the wire spellings understood by existing
// queue readers. Normalization affects interpretation only. The record's bytes
// still identify the original observation.
func ParseVerificationOutcome(raw string) VerificationOutcome {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "verified":
		return OutcomeVerified
	case "verify-fail", "fail":
		return OutcomeFailed
	case "blocked":
		return OutcomeBlocked
	case "needs-context", "needs_context":
		return OutcomeNeedsContext
	default:
		return OutcomeUnknown
	}
}

// Held reports whether a blocker holds the run for context or action.
func (outcome VerificationOutcome) Held() bool {
	return outcome == OutcomeBlocked || outcome == OutcomeNeedsContext
}

// NeedsWake reports whether a failed or held run belongs to wake scheduling.
func (outcome VerificationOutcome) NeedsWake() bool {
	return outcome == OutcomeFailed || outcome.Held()
}
