package reviewflow

import (
	"sort"
	"strings"
	"time"
)

const SelectionEvaluator = "review-selection-v1"

type UnmappedPolicy string

const (
	// UseObservedScore explicitly retains the existing review-domain default.
	UseObservedScore UnmappedPolicy = "observed-score"
	HoldUnmapped     UnmappedPolicy = "hold"
)

// PrioritySubject is a provider-qualified change, independent of its mutable head.
// Priorities rank observed work; they cannot establish that a change exists or is eligible.
type PrioritySubject struct {
	Repository Repository
	Number     int
}
type PriorityEntry struct {
	Subject PrioritySubject
	Score   int
}

// SelectionPolicy is supplied by the approved policy owner. This library validates
// structure/version, not approval or identity. It never reads a priority register.
type SelectionPolicy struct {
	Owner      string
	Revision   string
	Evaluator  string
	Unmapped   UnmappedPolicy
	Priorities []PriorityEntry // explicit empty is valid; nil means unavailable
}

type SelectionDisposition string

const (
	SelectionSelected SelectionDisposition = "selected"
	SelectionDeferred SelectionDisposition = "deferred"
	SelectionHeld     SelectionDisposition = "held"
	SelectionWatch    SelectionDisposition = "watch"
)

// CandidateDecision preserves the classifier's row, lanes and holds unchanged.
// Priority and Order belong to this selection, not to the stored observation.
// A proposal remains advisory, even when selected. Consumers enforce claims and
// all consequential checks at the existing effect boundary.
type CandidateDecision struct {
	Row            ObservationRow
	Priority       int
	PriorityMapped bool
	Order          int
	Disposition    SelectionDisposition
	Reason         string
	Proposal       *PlannedVerb
}
type PriorityGap struct {
	Subject PrioritySubject
	Reason  string
}

type SelectionPlan struct {
	Role                 string
	Evaluator            string
	PolicyOwner          string
	PolicyRevision       string
	ObservationSchema    string
	ObservationEvaluator string
	ObservationPolicy    string
	Collection           CollectionInterval
	Requested            []Repository
	Admitted             []Repository
	Population           []PopulationCoverage
	Now                  time.Time
	Oldest               time.Time
	Capacity             int
	Depths               ObservationDepths
	Idle                 IdleVerdict         // existing reviewer-dispatch idle meaning, never whole-desk completion
	Candidates           []CandidateDecision // complete population, including held/unmapped/deferred
	NotObserved          []PriorityGap       // policy entries without an observed candidate, never inferred work
}

// PlanCandidates joins a validated full observation with explicit current policy,
// time/freshness and aggregate proposal capacity. It uses the existing classifier,
// action table, coalescer and idle owner through PlanObservation. It performs no IO.
// Capacity counts advisory operations, not permissions or reviewer-lane grants.
// Legacy command ordering is unchanged; adapters migrate in the existing follow-ons.
func PlanCandidates(o Observation, freshness ObservationPolicy, policy SelectionPolicy, capacity int) (SelectionPlan, error) {
	if capacity < 0 || strings.TrimSpace(policy.Owner) == "" || strings.TrimSpace(policy.Revision) == "" || policy.Evaluator != SelectionEvaluator {
		return SelectionPlan{}, invalid("selection requires owner, revision, supported evaluator and nonnegative capacity", nil)
	}
	if policy.Unmapped != UseObservedScore && policy.Unmapped != HoldUnmapped {
		return SelectionPlan{}, invalid("explicit supported unmapped-priority policy is required", nil)
	}
	if policy.Priorities == nil {
		return SelectionPlan{}, inputError(PartialError, "priority population is unavailable; supply an explicit empty list for measured zero")
	}
	priorities := map[PrioritySubject]int{}
	for _, entry := range policy.Priorities {
		if !validRepository(entry.Subject.Repository) || entry.Subject.Number < 1 {
			return SelectionPlan{}, invalid("noncanonical priority subject", nil)
		}
		if _, exists := priorities[entry.Subject]; exists {
			return SelectionPlan{}, invalid("duplicate priority subject", nil)
		}
		priorities[entry.Subject] = entry.Score
	}
	verified, err := PlanObservation(o, freshness)
	if err != nil {
		return SelectionPlan{}, err
	}
	proposals := map[PrioritySubject]PlannedVerb{}
	for _, v := range verified.Verbs {
		proposals[PrioritySubject{Repository: Repository{Provider: v.Key.Provider, Host: v.Key.Host, Repository: v.Key.Repo}, Number: v.Key.PR}] = v
	}
	plan := SelectionPlan{Role: "review", Evaluator: SelectionEvaluator, PolicyOwner: policy.Owner, PolicyRevision: policy.Revision,
		ObservationSchema: o.Schema, ObservationEvaluator: o.Inputs.Evaluator, ObservationPolicy: o.Inputs.Policy,
		Collection: o.Inputs.Collection, Requested: append([]Repository{}, o.Inputs.Requested...), Admitted: append([]Repository{}, o.Inputs.Admitted...), Population: append([]PopulationCoverage{}, o.Inputs.Population...), Now: freshness.Now, Oldest: freshness.Oldest,
		Capacity: capacity, Depths: verified.Result.Depths, Idle: verified.Idle, Candidates: []CandidateDecision{}, NotObserved: []PriorityGap{}}
	observed := map[PrioritySubject]bool{}
	for _, row := range verified.Result.Rows {
		subject := PrioritySubject{Repository: row.Identity.Repository, Number: row.Identity.Number}
		observed[subject] = true
		score, mapped := priorities[subject]
		if !mapped {
			score = row.Score
		}
		d := CandidateDecision{Row: row, Priority: score, PriorityMapped: mapped, Disposition: SelectionWatch, Reason: row.Note}
		if row.Disposition == DispositionWait.String() {
			d.Disposition = SelectionHeld
		}
		if v, ok := proposals[subject]; ok {
			d.Proposal = &v
			switch {
			case v.Suppressed:
				d.Disposition = SelectionHeld
				d.Reason = v.Why
			default:
				d.Disposition = SelectionDeferred
				d.Reason = "proposal awaits capacity"
			}
		}
		// A missing mapping never erases an existing domain hold or watch reason.
		if !mapped {
			if policy.Unmapped == HoldUnmapped {
				d.Disposition = SelectionHeld
				if d.Proposal == nil || !d.Proposal.Suppressed {
					d.Reason = row.Note
				}
				d.Reason += "; priority unmapped; explicit policy holds selection"
			} else {
				d.Reason += "; priority unmapped; existing observed-score default retained"
			}
		}
		plan.Candidates = append(plan.Candidates, d)
	}
	sort.Slice(plan.Candidates, func(i, j int) bool {
		a, b := plan.Candidates[i], plan.Candidates[j]
		if a.Priority != b.Priority {
			return a.Priority > b.Priority
		}
		return observationRowID(a.Row) < observationRowID(b.Row)
	})
	remaining := capacity
	for i := range plan.Candidates {
		d := &plan.Candidates[i]
		d.Order = i + 1
		if d.Disposition == SelectionDeferred && remaining > 0 {
			d.Disposition = SelectionSelected
			d.Reason = "selected by explicit priority within capacity"
			remaining--
			if !d.PriorityMapped {
				d.Reason += "; priority unmapped; existing observed-score default retained"
			}
		}
	}
	admitted := map[Repository]bool{}
	for _, repo := range o.Inputs.Admitted {
		admitted[repo] = true
	}
	for subject := range priorities {
		if !observed[subject] {
			reason := "priority subject not observed in the admitted candidate population"
			if !admitted[subject.Repository] {
				reason = "priority subject outside the admitted candidate scope; not observed"
			}
			plan.NotObserved = append(plan.NotObserved, PriorityGap{Subject: subject, Reason: reason})
		}
	}
	sort.Slice(plan.NotObserved, func(i, j int) bool {
		a, b := plan.NotObserved[i].Subject, plan.NotObserved[j].Subject
		if a.Repository != b.Repository {
			return a.Repository.String() < b.Repository.String()
		}
		return a.Number < b.Number
	})
	if len(plan.NotObserved) > 0 && plan.Idle.State == IdleYes {
		plan.Idle.State = IdleCouldNotCheck
		plan.Idle.BlindReasons = append(plan.Idle.BlindReasons, "priority subjects not observed in the admitted candidate population")
	}
	return plan, nil
}
