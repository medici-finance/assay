package streamview

// milestones.go — the stream-view/v2 milestone section (milestone-support/11).
//
// The section projects the canonical milestone reader's and readiness
// evaluator's results; it owns no state of its own. Five facts stay distinct and
// are never derived from one another by a consumer:
//
//   - scope: the work refs a milestone promises (Scope);
//   - work remaining: how much of that scope is not done (Work);
//   - acceptance: whether the outcome was accepted (Acceptance) — zero work
//     remaining is NOT acceptance;
//   - availability: whether the milestone is currently supported (Availability);
//   - holds and prerequisites: what blocks it and why (Holds, Prerequisites).
//
// Missing is never zero: work and acceptance each carry an explicit unknown
// state with a reason, and the section's coverage lists every unknown.

import (
	"fmt"
	"strings"
)

// Milestone coverage values. Known means every record was read and every
// reference resolved; Unknown means at least one gap is listed in Unknowns.
const (
	CoverageKnown   = "known"
	CoverageUnknown = "unknown"
)

// Work states.
const (
	WorkKnown   = "known"
	WorkUnknown = "unknown"
)

// Acceptance states. NotDue means work remains, so acceptance is not yet the
// question; CouldNotCheck means no acceptance observation was available.
const (
	AcceptanceAccepted      = "accepted"
	AcceptanceOutstanding   = "outstanding"
	AcceptanceNotDue        = "not-due"
	AcceptanceCouldNotCheck = "could-not-check"
)

// Availability values of one milestone.
const (
	MilestoneSupported = "supported"
	MilestoneBlocked   = "blocked"
	MilestoneUnknown   = "unknown"
)

var validHoldKinds = map[string]bool{"work": true, "coverage": true, "cycle": true, "prerequisite": true, "acceptance": true}

// MilestoneWork is the work-remaining reading of one milestone.
type MilestoneWork struct {
	State string `json:"state"`
	// Remaining is meaningful only when State is known; zero then means no
	// work remains, which is not acceptance.
	Remaining int      `json:"remaining"`
	Pending   []string `json:"pending,omitempty"`
	Reason    string   `json:"reason,omitempty"`
}

// MilestoneAcceptance is the acceptance reading of one milestone.
type MilestoneAcceptance struct {
	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
}

// MilestoneHold is one reason a milestone is not supported.
type MilestoneHold struct {
	Ref    string `json:"ref"`
	Kind   string `json:"kind"`
	State  string `json:"state"`
	Reason string `json:"reason"`
}

// MilestoneItem is one milestone at one exact contract version.
type MilestoneItem struct {
	// Ref is the exact versioned reference (<key>@<version>).
	Ref   string `json:"ref"`
	Title string `json:"title"`
	Owner string `json:"owner,omitempty"`
	// Aliases are legacy references that denote this same milestone. They never
	// make a second row and never carry approval state.
	Aliases       []string            `json:"aliases,omitempty"`
	Scope         []string            `json:"scope"`
	Work          MilestoneWork       `json:"work"`
	Acceptance    MilestoneAcceptance `json:"acceptance"`
	Availability  string              `json:"availability"`
	Prerequisites []string            `json:"prerequisites,omitempty"`
	Holds         []MilestoneHold     `json:"holds,omitempty"`
	Evidence      []EvidenceRef       `json:"evidence,omitempty"`
}

// MilestoneMembership is the generated reverse index: one work item and the
// milestones that list it. Nothing authors this by hand.
type MilestoneMembership struct {
	Brief      string   `json:"brief"`
	Milestones []string `json:"milestones"`
}

// Milestones is the v2 milestone section. It is mandatory in a v2 view; a
// stream that did not opt in states not-assessed rather than omitting it.
type Milestones struct {
	Provenance Provenance `json:"provenance"`
	// Coverage is known|unknown when the section carries content, empty
	// otherwise.
	Coverage string `json:"coverage,omitempty"`
	// Unknowns lists every gap behind an unknown coverage reading.
	Unknowns   []string              `json:"unknowns,omitempty"`
	Items      []MilestoneItem       `json:"items,omitempty"`
	Membership []MilestoneMembership `json:"membership,omitempty"`
}

func (m Milestones) hasContent() bool {
	return m.Coverage != "" || len(m.Unknowns) > 0 || len(m.Items) > 0 || len(m.Membership) > 0
}

func (m Milestones) validate() []error {
	errs := m.Provenance.validate("milestones", m.hasContent())
	if !m.Provenance.Availability.valid() {
		return errs
	}
	if !m.Provenance.Availability.carriesContent() {
		return errs
	}
	switch m.Coverage {
	case CoverageKnown:
		if len(m.Unknowns) > 0 {
			errs = append(errs, fmt.Errorf("milestones: coverage is known but %d unknowns are listed", len(m.Unknowns)))
		}
		if m.Provenance.Availability == Partial {
			errs = append(errs, fmt.Errorf("milestones: a partial section cannot claim known coverage"))
		}
	case CoverageUnknown:
		if len(m.Unknowns) == 0 {
			errs = append(errs, fmt.Errorf("milestones: coverage is unknown but no unknown is listed"))
		}
	default:
		errs = append(errs, fmt.Errorf("milestones: coverage %q is not one of known|unknown", m.Coverage))
	}
	seen := map[string]string{}
	claim := func(ref, owner string) {
		if prev, dup := seen[ref]; dup {
			errs = append(errs, fmt.Errorf("milestones: %q is claimed by both %s and %s — one milestone, one row", ref, prev, owner))
			return
		}
		seen[ref] = owner
	}
	for i, it := range m.Items {
		where := fmt.Sprintf("milestones: items[%d]", i)
		if it.Ref == "" || it.Title == "" {
			errs = append(errs, fmt.Errorf("%s lacks ref or title", where))
			continue
		}
		claim(it.Ref, it.Ref)
		for _, a := range it.Aliases {
			if a == "" || a == it.Ref {
				errs = append(errs, fmt.Errorf("%s: alias %q is empty or the ref itself", where, a))
				continue
			}
			claim(a, it.Ref)
		}
		errs = append(errs, it.validate(where)...)
	}
	known := map[string]bool{}
	for _, it := range m.Items {
		known[it.Ref] = true
	}
	for i, mb := range m.Membership {
		if mb.Brief == "" || len(mb.Milestones) == 0 {
			errs = append(errs, fmt.Errorf("milestones: membership[%d] lacks brief or milestones", i))
		}
		for _, r := range mb.Milestones {
			if !known[r] {
				errs = append(errs, fmt.Errorf("milestones: membership[%d] names %q, which is not an item", i, r))
			}
		}
	}
	return errs
}

func (it MilestoneItem) validate(where string) []error {
	var errs []error
	switch it.Work.State {
	case WorkKnown:
		if it.Work.Remaining < 0 {
			errs = append(errs, fmt.Errorf("%s: work remaining is negative", where))
		}
		if it.Work.Remaining != len(it.Work.Pending) && len(it.Work.Pending) > 0 {
			errs = append(errs, fmt.Errorf("%s: work remaining %d does not match %d pending refs", where, it.Work.Remaining, len(it.Work.Pending)))
		}
	case WorkUnknown:
		if it.Work.Reason == "" {
			errs = append(errs, fmt.Errorf("%s: unknown work gives no reason", where))
		}
		if it.Work.Remaining != 0 || len(it.Work.Pending) > 0 {
			errs = append(errs, fmt.Errorf("%s: unknown work carries a count — missing must never read as a value", where))
		}
	default:
		errs = append(errs, fmt.Errorf("%s: work state %q is not one of known|unknown", where, it.Work.State))
	}
	switch it.Acceptance.State {
	case AcceptanceCouldNotCheck:
		if it.Acceptance.Reason == "" {
			errs = append(errs, fmt.Errorf("%s: could-not-check acceptance gives no reason", where))
		}
	case AcceptanceOutstanding:
		if it.Work.State != WorkKnown || it.Work.Remaining != 0 {
			errs = append(errs, fmt.Errorf("%s: acceptance outstanding requires known work with none remaining", where))
		}
	case AcceptanceNotDue:
		if it.Work.State != WorkKnown || it.Work.Remaining == 0 {
			errs = append(errs, fmt.Errorf("%s: acceptance not-due requires known work still remaining", where))
		}
	case AcceptanceAccepted:
		if it.Work.State != WorkKnown || it.Work.Remaining != 0 {
			errs = append(errs, fmt.Errorf("%s: accepted requires known work with none remaining", where))
		}
	default:
		errs = append(errs, fmt.Errorf("%s: acceptance state %q is not one of accepted|outstanding|not-due|could-not-check", where, it.Acceptance.State))
	}
	switch it.Availability {
	case MilestoneSupported:
		if len(it.Holds) > 0 {
			errs = append(errs, fmt.Errorf("%s: supported milestone lists holds", where))
		}
		if it.Acceptance.State != AcceptanceAccepted {
			errs = append(errs, fmt.Errorf("%s: supported requires accepted acceptance, not %q", where, it.Acceptance.State))
		}
	case MilestoneBlocked:
		if len(it.Holds) == 0 {
			errs = append(errs, fmt.Errorf("%s: blocked milestone names no hold", where))
		}
	case MilestoneUnknown:
	default:
		errs = append(errs, fmt.Errorf("%s: availability %q is not one of supported|blocked|unknown", where, it.Availability))
	}
	for j, h := range it.Holds {
		if h.Ref == "" || h.Reason == "" || h.State == "" || !validHoldKinds[h.Kind] {
			errs = append(errs, fmt.Errorf("%s: holds[%d] needs ref, state, reason and a kind of work|coverage|cycle|prerequisite|acceptance", where, j))
		}
	}
	for j, e := range it.Evidence {
		if err := e.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("%s: evidence[%d]: %w", where, j, err))
		}
	}
	for _, s := range append(append([]string{}, it.Scope...), it.Prerequisites...) {
		if s == "" || strings.TrimSpace(s) != s {
			errs = append(errs, fmt.Errorf("%s: scope or prerequisite entry %q is empty or padded", where, s))
		}
	}
	return errs
}
