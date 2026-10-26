package contexthandoff

import (
	"fmt"
	"strconv"
)

func refuse(rule, path, format string, a ...any) *Error {
	return &Error{Rule: rule, Path: path, Msg: fmt.Sprintf(format, a...)}
}

func idx(path string, i int) string { return path + "[" + strconv.Itoa(i) + "]" }

// validateHandoff applies the cross-field rules of spec/context-handoff-v1.md
// §"Rules": the ones the schema states as conditionals, and the ones no JSON
// schema can state (membership, uniqueness, cross-record binding).
func validateHandoff(h *Handoff) error {
	if err := checkTarget(h.Target); err != nil {
		return err
	}
	if err := checkCoverage(h.Discovery); err != nil {
		return err
	}
	if err := checkImpacts(h); err != nil {
		return err
	}
	for i, w := range h.ActiveWork {
		if w.Treatment == "adjust" && !w.Acknowledged {
			return refuse(RuleActiveWork, idx("$.active_work", i)+".acknowledged",
				"an active claim adjusted in place must be acknowledged by its owner (%s) before the amendment lands", w.Owner)
		}
	}
	if err := checkReadiness(h); err != nil {
		return err
	}
	return checkProduct(h.Product, h.Readiness.DecidingRole)
}

func checkTarget(t Target) error {
	const p = "$.target"
	switch {
	case t.State == TargetApproved && t.Approval == nil:
		return refuse(RuleApproval, p+".approval", "an approved target records its approval and the approving revision")
	case t.State != TargetApproved && t.Approval != nil:
		return refuse(RuleApproval, p+".approval", "a %s target carries no approval; approval is a distinct state", t.State)
	case t.Approval != nil && t.Approval.Revision != t.Revision:
		return refuse(RuleApproval, p+".approval.revision",
			"approval is for revision %s, but the target is revision %s; an approval covers exactly the revision it names", t.Approval.Revision, t.Revision)
	}
	return nil
}

// checkCoverage makes every coverage gap explicit: each included source is
// either enumerated or listed unavailable with a reason, exactly once.
func checkCoverage(d Discovery) error {
	const p = "$.discovery"
	sources := map[string]Source{}
	for i, s := range d.Sources {
		if _, dup := sources[s.ID]; dup {
			return refuse(RuleDuplicateID, idx(p+".sources", i)+".id", "source %q is listed twice", s.ID)
		}
		sources[s.ID] = s
	}
	seen := map[string]string{}
	for i, id := range d.Coverage.Enumerated {
		at := idx(p+".coverage.enumerated", i)
		if err := coverageMember(sources, seen, id, at); err != nil {
			return err
		}
		if sources[id].Revision == nil {
			return refuse(RuleCoverage, at, "source %q is enumerated but records no revision; an enumeration is at a revision", id)
		}
	}
	for i, u := range d.Coverage.Unavailable {
		if err := coverageMember(sources, seen, u.Source, idx(p+".coverage.unavailable", i)+".source"); err != nil {
			return err
		}
	}
	for i, s := range d.Sources {
		if s.Membership == "included" && seen[s.ID] == "" {
			return refuse(RuleCoverage, idx(p+".sources", i),
				"included source %q is neither enumerated nor listed unavailable; an unread source is a stated gap, never an omission", s.ID)
		}
	}
	if d.Coverage.State == "complete" && len(d.Coverage.Unavailable) > 0 {
		return refuse(RuleCoverage, p+".coverage.state", "coverage with an unavailable source is partial, not complete")
	}
	for i, q := range d.Queries {
		if q.Truncated && d.Coverage.State == "complete" {
			return refuse(RuleCoverage, idx(p+".queries", i)+".truncated",
				"query %q was truncated, so coverage is partial, not complete", q.Rule)
		}
	}
	return nil
}

func coverageMember(sources map[string]Source, seen map[string]string, id, at string) error {
	s, ok := sources[id]
	switch {
	case !ok:
		return refuse(RuleCoverage, at, "source %q is not in the source universe", id)
	case s.Membership != "included":
		return refuse(RuleCoverage, at, "source %q is excluded, so it is neither enumerated nor unavailable", id)
	case seen[id] != "":
		return refuse(RuleCoverage, at, "source %q is already accounted for at %s", id, seen[id])
	}
	seen[id] = at
	return nil
}

func checkImpacts(h *Handoff) error {
	subjects := map[string]bool{}
	for i, im := range h.Impacts {
		if subjects[im.Subject] {
			return refuse(RuleDuplicateID, idx("$.impacts", i)+".subject", "impact on %s is listed twice", im.Subject)
		}
		subjects[im.Subject] = true
	}
	obligations := map[string]Amendment{}
	ids := map[string]bool{}
	for i, a := range h.Amendments {
		at := idx("$.amendments", i)
		if ids[a.ID] {
			return refuse(RuleDuplicateID, at+".id", "amendment obligation %q is listed twice", a.ID)
		}
		ids[a.ID] = true
		if _, dup := obligations[a.Subject]; dup {
			return refuse(RuleDuplicateID, at+".subject", "%s already has an amendment obligation", a.Subject)
		}
		if !subjects[a.Subject] {
			return refuse(RuleImpactUnbound, at+".subject", "amendment obligation %q names %s, which has no impact record", a.ID, a.Subject)
		}
		obligations[a.Subject] = a
	}
	unresolved := map[string]bool{}
	for _, u := range h.Readiness.Unresolved {
		unresolved[u] = true
	}
	for i, im := range h.Impacts {
		at := idx("$.impacts", i) + ".disposition"
		switch im.Disposition {
		case "amend", "split", "replace", "retire":
			a, ok := obligations[im.Subject]
			if !ok {
				return refuse(RuleImpactUnbound, at, "disposition %q on %s has no amendment obligation", im.Disposition, im.Subject)
			}
			if a.Action != im.Disposition {
				return refuse(RuleImpactUnbound, at, "disposition %q on %s, but its obligation %q says %q", im.Disposition, im.Subject, a.ID, a.Action)
			}
		case "unresolved":
			if !unresolved[im.Subject] {
				return refuse(RuleReadiness, at, "%s is unresolved but not listed in readiness.unresolved", im.Subject)
			}
		default:
			if a, ok := obligations[im.Subject]; ok {
				return refuse(RuleImpactUnbound, at, "disposition %q on %s, but obligation %q would change it", im.Disposition, im.Subject, a.ID)
			}
		}
	}
	return nil
}

func checkReadiness(h *Handoff) error {
	const p = "$.readiness"
	r := h.Readiness
	approved := h.Target.State == TargetApproved
	if r.Facts.ProposalApproved != approved {
		return refuse(RuleReadiness, p+".facts.proposal_approved",
			"proposal_approved is %t but the target state is %s", r.Facts.ProposalApproved, h.Target.State)
	}
	if r.NextAct == "publish" || r.NextAct == "dispatch" {
		if err := h.RequireApproved(); err != nil {
			e := err.(*Error)
			e.Path = p + ".next_act"
			e.Msg = fmt.Sprintf("next act %q needs an approved target: %s", r.NextAct, e.Msg)
			return e
		}
		if len(r.Unresolved) > 0 {
			return refuse(RuleReadiness, p+".unresolved", "next act %q with %d unresolved item(s)", r.NextAct, len(r.Unresolved))
		}
	}
	return nil
}

// RequireApproved is the check a consumer runs before acting on a target that
// must be approved (publishing amendments, dispatching work). A draft or
// proposed target never satisfies it, and neither does an approval that does
// not record the target's own revision.
func (h *Handoff) RequireApproved() error {
	t := h.Target
	if t.State != TargetApproved {
		return refuse(RuleNotApproved, "$.target.state", "target is %s, not approved", t.State)
	}
	if t.Approval == nil || t.Approval.Revision == "" || t.Approval.Revision != t.Revision {
		return refuse(RuleNotApproved, "$.target.approval", "target is marked approved but records no approval of revision %s", t.Revision)
	}
	return nil
}

// validateManifest applies the manifest's own cross-field rules.
func validateManifest(m *Manifest) error {
	seen := map[string]bool{}
	for i, e := range m.Entries {
		at := idx("$.entries", i)
		if seen[e.Obligation] {
			return refuse(RuleDuplicateID, at+".obligation", "obligation %q has more than one entry", e.Obligation)
		}
		seen[e.Obligation] = true
		if why := checkBinding(e.Binding); why != "" {
			return refuse(RuleAmendUnbound, at+".binding", "%q obligation %q is unbound: %s", e.Action, e.Obligation, why)
		}
	}
	return nil
}

// checkBinding is the single amendment-binding rule: exactly one of a patch
// carrying its base revision and patch digest, or a follow-up brief in the
// four-segment form. It returns why a binding is unbound, or "".
func checkBinding(b Binding) string {
	switch {
	case b.Patch != nil && b.FollowUp != nil:
		return "a binding is exactly one of a patch or a follow-up brief, not both"
	case b.Patch == nil && b.FollowUp == nil:
		return "no patch and no follow-up brief"
	case b.Patch != nil:
		if !revisionRe.MatchString(b.Patch.BaseRevision) || !digestRe.MatchString(b.Patch.Digest) {
			return "the patch binding needs a base revision (7-64 lower-case hex) and a patch digest (sha256:<64 hex>)"
		}
	default:
		if *b.FollowUp == "" {
			return "the follow-up reference is empty"
		}
		if _, err := ResolveQualifiedID(*b.FollowUp, nil); err != nil {
			return "the follow-up reference is not a four-segment brief ID: " + err.Error()
		}
	}
	return ""
}

// CheckManifest binds a manifest to its handoff: the manifest names the
// handoff's canonical digest, carries exactly one entry per amendment
// obligation with the same brief and action, and no entry for anything else.
func CheckManifest(h *Handoff, m *Manifest) error {
	if err := validateManifest(m); err != nil {
		return err
	}
	if d := h.Digest(); m.Handoff != d {
		return refuse(RuleManifestMismatch, "$.handoff", "manifest names handoff %s, but the handoff's canonical digest is %s", m.Handoff, d)
	}
	if m.DeclaredBy != h.DeclaredBy {
		return refuse(RuleManifestMismatch, "$.declared_by", "manifest is declared by %s, the handoff by %s", m.DeclaredBy, h.DeclaredBy)
	}
	entries := map[string]int{}
	for i, e := range m.Entries {
		entries[e.Obligation] = i
	}
	obligations := map[string]bool{}
	for i, a := range h.Amendments {
		obligations[a.ID] = true
		j, ok := entries[a.ID]
		if !ok {
			return refuse(RuleManifestMismatch, idx("$.amendments", i), "amendment obligation %q has no manifest entry", a.ID)
		}
		e := m.Entries[j]
		if e.Brief != a.Subject || e.Action != a.Action {
			return refuse(RuleManifestMismatch, idx("$.entries", j),
				"entry for %q says %s %s, the obligation says %s %s", a.ID, e.Action, e.Brief, a.Action, a.Subject)
		}
	}
	for i, e := range m.Entries {
		if !obligations[e.Obligation] {
			return refuse(RuleManifestMismatch, idx("$.entries", i)+".obligation", "entry names %q, which is not an obligation of this handoff", e.Obligation)
		}
	}
	return nil
}
