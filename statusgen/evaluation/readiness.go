package evaluation

import (
	"sort"
	"strings"
)

// ReadinessSubject is an opaque exact subject token plus its prerequisite scope.
// Hosts derive Token from the complete domain subject, never from its display name.
type ReadinessSubject struct {
	Token       string
	Environment string
	Interface   string
	Profile     string
}

func (s ReadinessSubject) known() bool {
	return s.Token != "" && s.Environment != "" && s.Interface != "" && s.Profile != ""
}

type AcceptanceState string

const (
	AcceptanceAccepted    AcceptanceState = "accepted"
	AcceptanceOutstanding AcceptanceState = "outstanding"
	AcceptanceRevoked     AcceptanceState = "revoked"
	AcceptanceSuperseded  AcceptanceState = "superseded"
	AcceptanceUnknown     AcceptanceState = "unknown"
)

type Availability string

const (
	AvailabilitySupported Availability = "supported"
	AvailabilityBlocked   Availability = "blocked"
	AvailabilityUnknown   Availability = "unknown"
)

// AcceptanceObservation is the receipt owner's present assessment, not a receipt
// or an authority claim. Milestone includes the exact promise version.
type AcceptanceObservation struct {
	Milestone    string
	Subject      ReadinessSubject
	State        AcceptanceState
	Availability Availability
	HistoryCount int
	Blockers     []string
	Unknowns     []string
}

type MilestoneInput struct {
	Graph      MilestoneGraph
	Subjects   map[string]ReadinessSubject // explicitly selected current subjects
	Acceptance []AcceptanceObservation
	Unknowns   []string // unread roots/plans, including ones with no loaded record
}

type ReadinessHoldKind string

const (
	HoldWork         ReadinessHoldKind = "work"
	HoldCoverage     ReadinessHoldKind = "coverage"
	HoldCycle        ReadinessHoldKind = "cycle"
	HoldPrerequisite ReadinessHoldKind = "prerequisite"
	HoldAcceptance   ReadinessHoldKind = "acceptance"
)

type ReadinessHold struct {
	Ref    string            `json:"ref"`
	Kind   ReadinessHoldKind `json:"kind"`
	State  EligibilityState  `json:"state"`
	Reason string            `json:"reason"`
}

type MilestoneReadiness struct {
	Ref                   string           `json:"ref"`
	WorkRemaining         int              `json:"work_remaining"`
	Remaining             []string         `json:"remaining,omitempty"`
	WorkComplete          bool             `json:"work_complete"`
	AcceptanceOutstanding bool             `json:"acceptance_outstanding"`
	AcceptedForSubject    bool             `json:"accepted_for_subject"`
	Subject               ReadinessSubject `json:"subject"`
	Availability          Availability     `json:"availability"`
	HistoryCount          int              `json:"history_count"`
	IncompleteCoverage    bool             `json:"incomplete_coverage"`
	Holds                 []ReadinessHold  `json:"holds,omitempty"`
	BlockingHeads         []string         `json:"blocking_heads,omitempty"`
}

type ReadinessCoverage struct {
	Incomplete bool     `json:"incomplete"`
	Unknowns   []string `json:"unknowns,omitempty"`
}

type ReadinessResult struct {
	Eligibility map[string]Eligibility        `json:"eligibility"`
	Milestones  map[string]MilestoneReadiness `json:"milestones,omitempty"`
	Coverage    ReadinessCoverage             `json:"coverage"`
	StartGates  map[string][]EdgeVerdict      `json:"-"`
}

// EvaluateReadiness is S-eligibility's combined result. It neither accepts an
// outcome nor grants dispatch. Nil Milestones preserves the legacy projection.
func EvaluateReadiness(in EligibilityInput) ReadinessResult {
	out := ReadinessResult{}
	if in.Milestones == nil {
		out.Eligibility = evaluateEdges(in)
		return out
	}
	m := in.Milestones
	out.Milestones = map[string]MilestoneReadiness{}
	out.StartGates = map[string][]EdgeVerdict{}
	refs := map[string]bool{}
	for ref := range m.Graph.Milestones {
		refs[ref] = true
	}
	for _, b := range m.Graph.Briefs {
		for _, g := range b.Gates {
			refs[g.Milestone] = true
		}
	}
	for _, ref := range sortedRefs(refs) {
		r := milestoneReadiness(m, ref)
		out.Milestones[ref] = r
		for _, h := range r.Holds {
			if h.State == StateCouldNotCheck {
				out.Coverage.Unknowns = append(out.Coverage.Unknowns, h.Ref+": "+h.Reason)
			}
		}
	}
	out.Coverage.Unknowns = dedupe(append(out.Coverage.Unknowns, m.Unknowns...))
	sort.Strings(out.Coverage.Unknowns)
	out.Coverage.Incomplete = len(out.Coverage.Unknowns) > 0
	// Copy before appending: supplied observations remain immutable, including
	// shared backing arrays in a caller's Gates slice.
	in.Streams = append([]Stream(nil), in.Streams...)
	for i := range in.Streams {
		in.Streams[i].Briefs = append([]Brief(nil), in.Streams[i].Briefs...)
		for j := range in.Streams[i].Briefs {
			b := &in.Streams[i].Briefs[j]
			b.Gates = append([]Edge(nil), b.Gates...)
			for _, g := range m.Graph.Briefs[b.Key].Gates {
				r := out.Milestones[g.Milestone]
				state := StateCouldNotCheck
				if r.Availability == AvailabilityBlocked || g.State == GraphCycle {
					state = StateUnsatisfied
				}
				if r.Availability == AvailabilitySupported && r.AcceptedForSubject && g.State != GraphCycle {
					state = StateSatisfied
				}
				why := readinessReason(r)
				if g.State == GraphCycle {
					why = strings.Join(g.Blockers, "; ")
				}
				e := EdgeVerdict{Ref: g.Spelling, Type: "behavioural-gate", Reason: "requires-milestones: milestone " + string(r.Availability), State: state, Why: why}
				if e.Ref == "" {
					e.Ref = g.Milestone
				}
				out.StartGates[b.Key] = append(out.StartGates[b.Key], e)
				b.Gates = append(b.Gates, Edge{Ref: e.Ref, Type: e.Type, Reason: e.Reason, Resolved: e.State, Why: e.Why})
			}
		}
	}
	out.Eligibility = evaluateEdges(in)
	return out
}

func milestoneReadiness(in *MilestoneInput, ref string) MilestoneReadiness {
	r := MilestoneReadiness{Ref: ref, Subject: in.Subjects[ref]}
	add := func(ref string, kind ReadinessHoldKind, state EligibilityState, why string) {
		r.Holds = append(r.Holds, ReadinessHold{Ref: ref, Kind: kind, State: state, Reason: why})
	}
	remaining, visited := map[string]bool{}, map[string]bool{}
	complete := true
	var walk func(string)
	walk = func(at string) {
		if visited[at] {
			return
		}
		visited[at] = true
		n, ok := in.Graph.Milestones[at]
		if !ok {
			complete = false
			add(at, "coverage", StateCouldNotCheck, "not a loaded record version")
			return
		}
		if n.Completion == GraphCycle {
			complete = false
			add(at, "cycle", StateUnsatisfied, "dependency cycle")
		}
		for _, why := range n.Coverage {
			complete = false
			add(at, "coverage", StateCouldNotCheck, why)
		}
		for _, w := range n.Work {
			if !w.Found || !knownWorkStatus(w.Status) {
				complete = false
				add(w.Key, "coverage", StateCouldNotCheck, "contributor "+w.Key+" is unknown: "+w.Why)
			} else if w.Status != "done" {
				complete = false
				remaining[w.Key] = true
				add(w.Key, "work", StateUnsatisfied, "contributor "+w.Key+" is "+w.Status)
			}
		}
		// A graph built by an older producer cannot silently mean empty work.
		if len(n.Work) != len(n.Contributors) {
			complete = false
			add(at, "coverage", StateCouldNotCheck, "contributor observations incomplete")
		}
		for _, prereq := range n.Prerequisites {
			walk(prereq)
			obs, state, why := currentAcceptance(in, prereq)
			scope := in.Subjects[prereq]
			if !r.Subject.known() || !scope.known() || scope.Environment != r.Subject.Environment || scope.Interface != r.Subject.Interface || scope.Profile != r.Subject.Profile {
				state, why = StateCouldNotCheck, "prerequisite subject/profile does not match consuming milestone"
			}
			if state != StateSatisfied {
				complete = false
				add(prereq, "prerequisite", state, why)
			}
			for _, why := range obs.Unknowns {
				add(prereq, "coverage", StateCouldNotCheck, why)
			}
		}
	}
	walk(ref)
	for _, why := range in.Unknowns {
		complete = false
		add(ref, "coverage", StateCouldNotCheck, why)
	}
	obs, state, why := currentAcceptance(in, ref)
	r.HistoryCount = obs.HistoryCount
	r.AcceptedForSubject = state == StateSatisfied
	r.WorkComplete = complete
	r.AcceptanceOutstanding = complete && !r.AcceptedForSubject
	if state != StateSatisfied {
		add(ref, "acceptance", state, why)
	}
	for _, why := range obs.Unknowns {
		add(ref, "coverage", StateCouldNotCheck, why)
	}
	r.Remaining = sortedRefs(remaining)
	r.WorkRemaining = len(r.Remaining)
	r.Availability = AvailabilitySupported
	seen := map[ReadinessHold]bool{}
	holds := r.Holds
	r.Holds = nil
	for _, h := range holds {
		if seen[h] {
			continue
		}
		seen[h] = true
		r.Holds = append(r.Holds, h)
		if h.State == StateCouldNotCheck {
			r.IncompleteCoverage = true
			if r.Availability != AvailabilityBlocked {
				r.Availability = AvailabilityUnknown
			}
		}
		if h.State == StateUnsatisfied {
			r.Availability = AvailabilityBlocked
		}
	}
	// Blocking heads are the nearest unfinished contributors / unaccepted
	// prerequisite promises, rather than every path through a shared lattice.
	n := in.Graph.Milestones[ref]
	for _, w := range n.Work {
		if !w.Found || w.Status != "done" {
			r.BlockingHeads = append(r.BlockingHeads, w.Key)
		}
	}
	for _, p := range n.Prerequisites {
		reachable := map[string]bool{}
		var visit func(string)
		visit = func(at string) {
			if reachable[at] {
				return
			}
			reachable[at] = true
			node := in.Graph.Milestones[at]
			for _, w := range node.Work {
				reachable[w.Key] = true
			}
			for _, next := range node.Prerequisites {
				visit(next)
			}
		}
		visit(p)
		for _, h := range r.Holds {
			if reachable[h.Ref] {
				r.BlockingHeads = append(r.BlockingHeads, p)
				break
			}
		}
	}
	r.BlockingHeads = dedupe(r.BlockingHeads)
	sort.Strings(r.BlockingHeads)
	return r
}

func currentAcceptance(in *MilestoneInput, ref string) (AcceptanceObservation, EligibilityState, string) {
	subject := in.Subjects[ref]
	var matches []AcceptanceObservation
	for _, a := range in.Acceptance {
		if a.Milestone == ref && a.Subject == subject {
			matches = append(matches, a)
		}
	}
	if !subject.known() || len(matches) != 1 {
		return AcceptanceObservation{}, StateCouldNotCheck, "exact current subject acceptance observation unavailable or ambiguous"
	}
	a := matches[0]
	if a.State == AcceptanceRevoked || a.State == AcceptanceSuperseded || a.State == AcceptanceOutstanding || a.Availability == AvailabilityBlocked || len(a.Blockers) > 0 {
		return a, StateUnsatisfied, "acceptance " + string(a.State) + ": " + strings.Join(a.Blockers, "; ")
	}
	if a.State != AcceptanceAccepted || a.Availability != AvailabilitySupported || len(a.Unknowns) != 0 {
		return a, StateCouldNotCheck, "current acceptance unavailable: " + strings.Join(a.Unknowns, "; ")
	}
	return a, StateSatisfied, ""
}

func sortedRefs(seen map[string]bool) []string {
	var out []string
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func readinessReason(r MilestoneReadiness) string {
	var reasons []string
	for _, h := range r.Holds {
		reasons = append(reasons, h.Reason)
	}
	return strings.Join(capDetail(dedupe(reasons), "holds"), "; ")
}

func knownWorkStatus(status string) bool {
	switch status {
	case "todo", "in-progress", "implemented", "verified", "done", "blocked", "withdrawn":
		return true
	default:
		return false
	}
}
