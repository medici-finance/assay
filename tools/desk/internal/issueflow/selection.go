// Package issueflow selects issue populations from already-observed facts.
// It has no provider, clock, credentials, trust policy or effects.
package issueflow

import "strings"

type Projection string

const (
	Full      Projection = "full"
	WorkReady Projection = "work-ready"
	Intake    Projection = "intake"
	Inbox     Projection = "inbox"
)

func Valid(p Projection) bool { return p == Full || p == WorkReady || p == Intake || p == Inbox }

// Candidate is coarse selection only. Inclusion still requires the caller's
// canonical trust, label-history and classification decisions.
type Candidate struct {
	HasPlaceholder, Excluded, DecisionOwed, Addressed, HelpWanted bool
	Role                                                          string
}

func Select(p Projection, c Candidate, to string) bool {
	if to != "" && (!c.Addressed || !strings.EqualFold(c.Role, to)) {
		return false
	}
	switch p {
	case WorkReady:
		return !c.HasPlaceholder && !c.Excluded && !c.DecisionOwed && !c.Addressed && !c.HelpWanted
	case Inbox:
		return c.Addressed
	case Full, Intake:
		return true
	default:
		return false
	}
}

// InboxRank preserves the human decision inbox's exact, case-sensitive label
// union and ordering. This differs from an addressed desk-role inbox.
func InboxRank(names []string) int {
	best := 99
	for _, n := range names {
		for i, l := range []string{"urgent", "needs-decision", "question", "help wanted"} {
			if n == l && i < best {
				best = i
			}
		}
	}
	return best
}
