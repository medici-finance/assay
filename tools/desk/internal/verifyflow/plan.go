// Package verifyflow owns pure verification candidate selection. Collectors supply
// statusgen's canonical rows and verifyloop's existing overlay observations. Neither
// observation nor selection grants a claim, dispatch, write or acceptance authority.
package verifyflow

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/medici-finance/assay/statusgen/verifyqueue"
	"sort"
	"strings"
	"time"
)

const Schema = "verify-observation-v1"
const Evaluator = "verify-selection-v1"
const (
	Awaiting = "awaiting"
	Repair   = "evidence-repair"
	Dispatch = "dispatch"
	Evidence = "dispatch-for-evidence"
	Unknown  = "could-not-check"
	Held     = "queue-held"
)

// Authority is the ceiling a proposal would carry if a caller dispatched it. A
// proposal itself grants nothing; the field keeps judgment work (flip-eligible)
// apart from Evidence-only work, which both reach "deferred" and "selected".
const (
	AuthorityFlip     = "flip"
	AuthorityEvidence = "evidence-only"
	AuthorityNone     = "none"
)

// Compose is the existing demotion-only base/overlay join. It cannot turn a
// held overlay into a dispatch, and Evidence-only never gains flip authority.
func Compose(overlay, reason string, row *verifyqueue.Row) (string, string) {
	if overlay != Dispatch && overlay != Evidence {
		return overlay, reason
	}
	if row == nil || !verifyqueue.Supported(row.QueueVersion, row.QueueClass) {
		return Unknown, "canonical queue: no supported classification"
	}
	if !row.QueueValid || row.QueueClass == verifyqueue.Unreadable {
		return Unknown, "canonical queue: unreadable — " + firstNonEmpty(row.QueueWhy, row.QueueNextAct)
	}
	switch row.QueueClass {
	case verifyqueue.Judgment:
		return overlay, reason
	case verifyqueue.Human:
		if overlay == Evidence {
			return overlay, reason
		}
	}
	return Held, fmt.Sprintf("canonical queue: %s (%s, owner %s): %s",
		row.QueueClass, firstNonEmpty(row.QueueBucket, "-"), firstNonEmpty(row.QueueOwner, "-"), row.QueueNextAct)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// joinReason joins two reason clauses without a dangling separator.
func joinReason(a, b string) string {
	if a == "" || b == "" {
		return a + b
	}
	return a + "; " + b
}

type Subject struct {
	Repository string `json:"repository"`
	Brief      string `json:"brief"`
	Revision   string `json:"revision"`
}

// Coverage is per authorized root and source lane. Unknown repair coverage is
// explicit until /51 wires discovery; an Awaiting-only scan cannot establish idle.
type Coverage struct {
	Repository string `json:"repository"`
	Lane       string `json:"lane"`
	State      string `json:"state"` // complete, skipped, unreadable, unknown
	Reason     string `json:"reason,omitempty"`
}
type Candidate struct {
	Subject Subject         `json:"subject"`
	Aliases []string        `json:"aliases"`
	Sources []string        `json:"sources"`
	Base    verifyqueue.Row `json:"base"`
	Overlay string          `json:"overlay"`
	Reason  string          `json:"reason,omitempty"`
	Order   int             `json:"order"` // existing collector order, retained as tie breaker
}
type Observation struct {
	Schema    string `json:"schema"`
	Evaluator string `json:"evaluator"`
	// Revision is ContentRevision of the observation. PlanCandidates recomputes it
	// and refuses a mismatch: a consistency check, not an authenticity control.
	Revision   string      `json:"revision"`
	Provenance string      `json:"provenance"`
	Collected  time.Time   `json:"collected"`
	Requested  []string    `json:"requested"`
	WholeScope bool        `json:"wholeScope"`
	Coverage   []Coverage  `json:"coverage"`
	Candidates []Candidate `json:"candidates"`
}
type Priority struct {
	Repository string `json:"repository"`
	Brief      string `json:"brief"`
	Score      int    `json:"score"`
}
type Policy struct {
	Owner      string     `json:"owner"`
	Revision   string     `json:"revision"`
	Evaluator  string     `json:"evaluator"`
	Unmapped   string     `json:"unmapped"`   // observed-score or hold
	Priorities []Priority `json:"priorities"` // nil is unavailable, [] is measured zero
}
type Decision struct {
	Candidate      Candidate `json:"candidate"`
	Disposition    string    `json:"disposition"`
	Authority      string    `json:"authority"` // flip, evidence-only or none; never granted by the plan
	Reason         string    `json:"reason"`
	Priority       int       `json:"priority"`
	PriorityMapped bool      `json:"priorityMapped"`
	Order          int       `json:"order"`
}
type Plan struct {
	Role        string      `json:"role"`
	Evaluator   string      `json:"evaluator"`
	Observation Observation `json:"observation"`
	Policy      Policy      `json:"policy"`
	Now         time.Time   `json:"now"`
	Oldest      time.Time   `json:"oldest"`
	Capacity    int         `json:"capacity"`
	Candidates  []Decision  `json:"candidates"`
	NotObserved []Priority  `json:"notObserved"`
	Gaps        []string    `json:"gaps"`
	Idle        string      `json:"idle"` // yes, no, unknown; no capacity is not idle
}

func key(repo, brief string) string { return repo + ":" + brief }

// ContentRevision is the sha256 digest of the observation's JSON with Revision and
// Collected cleared, so identical content collected at two times has one revision.
// It detects an observation edited without resealing; anyone holding the content
// can recompute it, so it is not a signature or an authenticity control.
func ContentRevision(o Observation) string {
	o.Revision = ""
	o.Collected = time.Time{}
	raw, err := json.Marshal(o)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("sha256:%x", sha256.Sum256(raw))
}

// PlanCandidates is a pure function over the complete authorized observation and
// explicit approved policy, clock/freshness cutoff and capacity. Approval is a
// caller-owned fact; this validates structure, never fabricates a human ruling.
// Known work survives a sibling coverage gap. Gaps cannot establish whole-desk idle.
func PlanCandidates(o Observation, p Policy, now, oldest time.Time, capacity int) (Plan, error) {
	plan := Plan{Role: "verify", Evaluator: Evaluator, Observation: o, Policy: p, Now: now, Oldest: oldest, Capacity: capacity,
		Candidates: []Decision{}, NotObserved: []Priority{}, Gaps: []string{}, Idle: "unknown"}
	if o.Schema != Schema || o.Evaluator != Evaluator || strings.TrimSpace(o.Revision) == "" || strings.TrimSpace(o.Provenance) == "" ||
		p.Evaluator != Evaluator || strings.TrimSpace(p.Owner) == "" || strings.TrimSpace(p.Revision) == "" || p.Priorities == nil ||
		(p.Unmapped != "observed-score" && p.Unmapped != "hold") || capacity < 0 || now.IsZero() || oldest.IsZero() || oldest.After(now) {
		return plan, fmt.Errorf("unsupported or incomplete observation/policy/time/capacity")
	}
	if o.Revision != ContentRevision(o) {
		return plan, fmt.Errorf("observation revision does not match its content")
	}
	fresh := !o.Collected.IsZero() && !o.Collected.Before(oldest) && !o.Collected.After(now)
	if !fresh {
		plan.Gaps = append(plan.Gaps, "observation outside supplied freshness interval")
	}
	if !o.WholeScope {
		plan.Gaps = append(plan.Gaps, "requested roots are not the whole authorized scope")
	}
	requested := map[string]bool{}
	for _, repo := range o.Requested {
		if repo == "" || requested[repo] {
			return plan, fmt.Errorf("missing or duplicate requested root")
		}
		requested[repo] = true
	}
	if len(requested) == 0 {
		plan.Gaps = append(plan.Gaps, "no authorized roots observed")
	}
	coverage := map[string]Coverage{}
	for _, c := range o.Coverage {
		k := key(c.Repository, c.Lane)
		if !requested[c.Repository] || (c.Lane != Awaiting && c.Lane != Repair) {
			return plan, fmt.Errorf("coverage outside requested roots/lanes")
		}
		if _, ok := coverage[k]; ok {
			return plan, fmt.Errorf("duplicate coverage")
		}
		if c.State != "complete" && c.State != "unknown" && c.State != "unreadable" && c.State != "skipped" {
			return plan, fmt.Errorf("unsupported coverage state")
		}
		coverage[k] = c
	}
	for _, repo := range o.Requested {
		for _, lane := range []string{Awaiting, Repair} {
			c, ok := coverage[key(repo, lane)]
			if !ok || c.State != "complete" {
				plan.Gaps = append(plan.Gaps, repo+" "+lane+": "+joinReason("coverage unknown or incomplete", c.Reason))
			}
		}
	}
	priorities := map[string]int{}
	for _, entry := range p.Priorities {
		k := key(entry.Repository, entry.Brief)
		if entry.Repository == "" || entry.Brief == "" {
			return plan, fmt.Errorf("incomplete priority subject")
		}
		if _, ok := priorities[k]; ok {
			return plan, fmt.Errorf("duplicate priority subject")
		}
		priorities[k] = entry.Score
	}
	// Union repeated aliases/source lanes before ranking. A conflicting source
	// revision, canonical result or overlay is unknown; never let arrival order win.
	population := []Candidate{}
	positions := map[string]int{}
	for _, c := range o.Candidates {
		k := key(c.Subject.Repository, c.Subject.Brief)
		if at, ok := positions[k]; ok {
			old := &population[at]
			if old.Subject != c.Subject || old.Base != c.Base || old.Overlay != c.Overlay || old.Reason != c.Reason {
				return plan, fmt.Errorf("conflicting observations for one candidate subject")
			}
			old.Sources = union(old.Sources, c.Sources)
			old.Aliases = union(old.Aliases, c.Aliases)
			if c.Order < old.Order {
				old.Order = c.Order
			}
		} else {
			c.Sources = union(nil, c.Sources)
			c.Aliases = union(nil, c.Aliases)
			positions[k] = len(population)
			population = append(population, c)
		}
	}
	seen := map[string]bool{}
	for _, c := range population {
		k := key(c.Subject.Repository, c.Subject.Brief)
		if c.Subject.Brief == "" || !requested[c.Subject.Repository] {
			return plan, fmt.Errorf("candidate outside requested scope or missing brief")
		}

		seen[k] = true
		score, mapped := priorities[k]
		if !mapped {
			score = c.Base.Score
		}
		d := Decision{Candidate: c, Priority: score, PriorityMapped: mapped, Disposition: "held", Authority: AuthorityNone, Reason: c.Reason}
		disposition, why := Compose(c.Overlay, c.Reason, &c.Base)
		d.Reason = why
		known := fresh && c.Subject.Revision != "" && len(c.Sources) > 0 && c.Base.Brief == c.Subject.Brief && (c.Base.Repo == "" || c.Base.Repo == c.Subject.Repository)
		for _, lane := range c.Sources {
			if coverage[key(c.Subject.Repository, lane)].State != "complete" {
				known = false
			}
		}
		switch {
		case !known || !verifyqueue.Supported(c.Base.QueueVersion, c.Base.QueueClass) || !c.Base.QueueValid || disposition == Unknown:
			d.Disposition = Unknown
			d.Reason = joinReason("candidate identity, source coverage or canonical classification unavailable", why)
			plan.Gaps = append(plan.Gaps, k+": "+d.Reason)
		case disposition == Held && (c.Base.QueueClass == verifyqueue.Paused || c.Base.QueueClass == verifyqueue.Parked):
			d.Disposition = "watch"
		case disposition == Dispatch || disposition == Evidence:
			d.Disposition = "deferred"
			d.Reason = "eligible proposal awaits capacity"
			d.Authority = AuthorityFlip
			if disposition == Evidence {
				d.Authority = AuthorityEvidence
			}
		}
		// The hold policy governs only eligible proposals. Watched (paused/parked),
		// held and unknown rows keep their disposition, so it cannot create owed work.
		if !mapped && p.Unmapped == "hold" && d.Disposition == "deferred" {
			d.Disposition = "held"
			d.Authority = AuthorityNone
			d.Reason = joinReason(d.Reason, "priority unmapped")
		}
		plan.Candidates = append(plan.Candidates, d)
	}
	sort.SliceStable(plan.Candidates, func(i, j int) bool {
		a, b := plan.Candidates[i], plan.Candidates[j]
		if a.Priority != b.Priority {
			return a.Priority > b.Priority
		}
		if a.Candidate.Order != b.Candidate.Order {
			return a.Candidate.Order < b.Candidate.Order
		}
		return key(a.Candidate.Subject.Repository, a.Candidate.Subject.Brief) < key(b.Candidate.Subject.Repository, b.Candidate.Subject.Brief)
	})
	remaining := capacity
	owed := false
	for i := range plan.Candidates {
		d := &plan.Candidates[i]
		d.Order = i + 1
		// Held human/continuation/repair work is owed work too, never a drained desk.
		if d.Disposition != "watch" && d.Disposition != Unknown {
			owed = true
		}
		if d.Disposition == "deferred" && remaining > 0 {
			d.Disposition = "selected"
			d.Reason = "selected by explicit priority within capacity"
			remaining--
		}
	}
	for _, entry := range p.Priorities {
		if !seen[key(entry.Repository, entry.Brief)] {
			plan.NotObserved = append(plan.NotObserved, entry)
		}
	}
	sort.Slice(plan.NotObserved, func(i, j int) bool {
		a, b := plan.NotObserved[i], plan.NotObserved[j]
		return key(a.Repository, a.Brief) < key(b.Repository, b.Brief)
	})
	if len(plan.NotObserved) > 0 {
		plan.Gaps = append(plan.Gaps, "priority subjects not observed")
	}
	if owed {
		plan.Idle = "no"
	} else if len(plan.Gaps) == 0 {
		plan.Idle = "yes"
	}
	return plan, nil
}

func union(a, b []string) []string {
	seen := map[string]bool{}
	for _, s := range a {
		seen[s] = true
	}
	for _, s := range b {
		seen[s] = true
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
