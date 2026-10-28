// Package issueflow is the compatibility facade over the public policy owner.
package issueflow

import shared "github.com/medici-finance/assay/deskcore/issueflow"

type Projection = shared.Projection
type Candidate = shared.Candidate

const (
	Full      = shared.Full
	WorkReady = shared.WorkReady
	Intake    = shared.Intake
	Inbox     = shared.Inbox
	Scanner   = shared.Scanner
)

func Valid(p Projection) bool                          { return shared.Valid(p) }
func Select(p Projection, c Candidate, to string) bool { return shared.Select(p, c, to) }
func InboxRank(names []string) int                     { return shared.InboxRank(names) }
func CandidateForLabels(c Candidate, labels []string) Candidate {
	return shared.CandidateForLabels(c, labels)
}
