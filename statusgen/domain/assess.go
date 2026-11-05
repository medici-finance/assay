package domain

import "github.com/medici-finance/assay/statusgen/evaluation"

// Observations are already normalized by an adapter. The evaluator's exported
// input types remain their single owner. Repo and Siblings must answer from
// frozen observations for a pure assessment. No adapter is installed here.
// The envelope binds these observations to the same work, candidate and generation.
// EvaluationContext supplies complete applicability; adapters still establish current authority.
type Observations struct {
	Work              WorkIdentity
	Candidate         Candidate
	Generation        uint64
	AcceptanceVersion uint64
	AcceptanceDigest  string
	Eligibility       evaluation.EligibilityInput
	Coverage          evaluation.CoverageInput
	Repo              evaluation.Repo
}

type FactHold struct {
	Fact  string       `json:"fact"`
	State Completeness `json:"state"`
}

// Assessment reports support, never an acceptance decision or publication grant.
// Missing observations leave the corresponding evaluator result absent, not green.
type Assessment struct {
	Work              WorkIdentity            `json:"work"`
	Candidate         Candidate               `json:"candidate"`
	AcceptanceVersion uint64                  `json:"acceptance-version"`
	AcceptanceDigest  string                  `json:"acceptance-digest"`
	Generation        uint64                  `json:"generation"`
	ObservedAt        string                  `json:"observed-at"`
	Evaluator         evaluation.Identity     `json:"evaluator"`
	State             string                  `json:"state"`
	Holds             []FactHold              `json:"holds"`
	Eligibility       *evaluation.Eligibility `json:"eligibility,omitempty"`
	Coverage          *evaluation.Coverage    `json:"coverage,omitempty"`
}

// Assess composes completeness, binding and the existing evaluator. It neither
// infers acceptance from execution nor modifies the caller's progress projection.
// semantic: S-verification-domain
func Assess(r Record, o Observations) (Assessment, error) {
	if err := r.Validate(); err != nil {
		return Assessment{}, err
	}
	if err := r.Generation.CheckCurrent(); err != nil {
		return Assessment{}, err
	}
	if o.Work != r.Work || o.Candidate != r.Candidate {
		return Assessment{}, refuse("observation-subject")
	}
	if o.Generation != r.Generation.Requested {
		return Assessment{}, refuse("stale-generation")
	}
	out := Assessment{Work: r.Work, Candidate: r.Candidate, Generation: r.Generation.Recorded, ObservedAt: r.Facts.ObservedAt, Evaluator: evaluation.CurrentIdentity(), State: "held", Holds: []FactHold{}}
	if r.Acceptance == nil {
		out.Holds = append(out.Holds, FactHold{"acceptance", Missing})
		return out, nil
	}
	if o.AcceptanceVersion != r.Acceptance.Version || o.AcceptanceDigest != r.Acceptance.Digest {
		return Assessment{}, refuse("observation-acceptance")
	}
	out.AcceptanceVersion = r.Acceptance.Version
	out.AcceptanceDigest = r.Acceptance.Digest
	facts := map[string]Completeness{}
	for _, f := range r.Facts.Entries {
		facts[f.ID] = f.State
	}
	for _, id := range r.Acceptance.RequiredFacts {
		state, ok := facts[id]
		if !ok {
			state = Missing
		}
		if state != Present {
			out.Holds = append(out.Holds, FactHold{id, state})
		}
	}
	// Neither evaluator sees a partial fact set: an absent prerequisite is held
	// explicitly even if the remaining observations would release coverage.
	if len(out.Holds) > 0 {
		return out, nil
	}
	q, err := qualified(r.Work.Brief)
	if err != nil {
		return Assessment{}, err
	}
	localID := q.Stream + "/" + q.NN
	count := 0
	for _, s := range o.Eligibility.Streams {
		if s.Name == q.Stream {
			for _, b := range s.Briefs {
				if b.Num == q.NN {
					count++
				}
			}
		}
	}
	if count != 1 {
		return Assessment{}, refuse("eligibility-subject")
	}
	if len(o.Coverage.Briefs) != 1 || o.Coverage.Briefs[0].ID != localID {
		return Assessment{}, refuse("coverage-subject")
	}
	// The candidate tree is the coverage revision. Never substitute its commit,
	// a branch, a path or an adapter's ambient HEAD.
	in := o.Coverage
	in.Revision = r.Candidate.Tree
	coverage := evaluation.EvaluateCoverage(in, o.Repo)[localID]
	eligibility := evaluation.EvaluateEligibility(o.Eligibility)[localID]
	out.Coverage = &coverage
	out.Eligibility = &eligibility
	if coverage.Released && eligibility.Verdict != evaluation.VerdictHeld {
		out.State = "supported"
	}
	return out, nil
}

// CheckCurrent refuses a historical result at a new subject, definition or
// reevaluation generation. This is a binding check, not authentication of a result.
func (a Assessment) CheckCurrent(r Record) error {
	if err := r.Validate(); err != nil {
		return err
	}
	if err := r.Generation.CheckCurrent(); err != nil {
		return err
	}
	if a.Generation != r.Generation.Requested {
		return refuse("stale-generation")
	}
	if a.Work != r.Work || a.Candidate != r.Candidate {
		return refuse("assessment-subject")
	}
	if r.Acceptance == nil || a.AcceptanceVersion != r.Acceptance.Version || a.AcceptanceDigest != r.Acceptance.Digest {
		return refuse("assessment-acceptance")
	}
	if a.Evaluator != evaluation.CurrentIdentity() {
		return unknown("unsupported-evaluator")
	}
	return nil
}
