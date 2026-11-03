// Package issueflow selects issue populations from already-observed facts.
// It has no provider, clock, credentials, trust policy or effects.
package issueflow

import "strings"

const AddressPrefix = "to:"

type Projection string

const (
	Full      Projection = "full"
	WorkReady Projection = "work-ready"
	Intake    Projection = "intake"
	Inbox     Projection = "inbox"
	Scanner   Projection = "scanner"
)

// Valid reports supported issueboard CLI projections. Scanner is library-only.
func Valid(p Projection) bool {
	return p == Full || p == WorkReady || p == Intake || p == Inbox
}

// Candidate is coarse selection only. Inclusion still requires the caller's
// canonical trust, label-history and classification decisions.
type Candidate struct {
	HasPlaceholder, Excluded, DecisionOwed, Addressed, HelpWanted bool
	AddressUnknown                                                bool
	Role                                                          string
}

func Select(p Projection, c Candidate, to string) bool {
	if to != "" && (!c.Addressed || !strings.EqualFold(c.Role, to)) {
		return false
	}
	switch p {
	case WorkReady:
		return !c.HasPlaceholder && !c.Excluded && !c.DecisionOwed && !c.Addressed && !c.AddressUnknown && !c.HelpWanted
	case Scanner:
		return !c.HasPlaceholder && !c.Excluded && !c.Addressed && !c.AddressUnknown
	case Inbox:
		return c.Addressed
	case Full, Intake:
		return true
	default:
		return false
	}
}

// InboxRank uses the human decision inbox's label union and ordering. Labels
// compare without case or surrounding space; their displayed spelling is kept.
// This differs from an addressed desk-role inbox.
func InboxRank(names []string) int {
	best := 99
	for _, n := range names {
		n = strings.ToLower(strings.TrimSpace(n))
		for i, l := range []string{"urgent", "needs-decision", "question", "help wanted"} {
			if n == l && i < best {
				best = i
			}
		}
	}
	return best
}

// Address reads historical labels without consulting the current roster. A malformed
// or conflicting stamp is unknown, never permission to enter ordinary work.
func Address(labels []string) (role string, unknown bool) { return Stamp(labels, AddressPrefix) }

// Stamp is the historical three-state label decoder reused by addressee and
// provenance adapters. A current roster is a write-side concern.
func Stamp(labels []string, prefix string) (role string, unknown bool) {
	for _, label := range labels {
		name := strings.ToLower(strings.TrimSpace(label))
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		next := strings.TrimSpace(strings.TrimPrefix(name, prefix))
		if next == "" || (role != "" && role != next) {
			return "", true
		}
		role = next
	}
	return role, false
}

// CandidateForLabels adds only the canonical addressee facts; caller-owned label,
// trust and eligibility policy remains outside this package.
func CandidateForLabels(c Candidate, labels []string) Candidate {
	c.Role, c.AddressUnknown = Address(labels)
	c.Addressed = c.Role != ""
	return c
}
