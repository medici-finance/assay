package main

// streammilestones.go — project the canonical milestone plan and readiness
// result into the stream-view/v2 milestone section (milestone-support/11).
//
// OWNER. This file owns only the mapping; the reader (milestones.go) owns what a
// record says and the evaluator (evaluation/readiness.go) owns work remaining,
// holds, acceptance and availability. streamMilestoneSection takes explicit
// values and returns a typed section: it never fetches, claims, approves,
// launches or mutates, and it derives no state of its own. In particular it
// never infers done or accepted from a zero work count.

import (
	"fmt"
	"sort"
	"time"

	"github.com/medici-finance/assay/statusgen/evaluation"
	"github.com/medici-finance/assay/statusgen/streamview"
)

// streamMilestoneSection builds the section for one stream. plan nil or not
// opted in is not-assessed (no fake milestones). rr nil means no readiness
// evaluation was supplied: every item states work and acceptance unknown.
func streamMilestoneSection(plan *MilestonePlan, rr *evaluation.ReadinessResult, repo string, src streamview.SourceRef, observed time.Time) streamview.Milestones {
	observed = observed.UTC()
	if plan == nil || !plan.OptedIn {
		return streamview.Milestones{Provenance: streamview.Provenance{
			Availability: streamview.NotAssessed, ObservedAt: observed,
			Reason: "stream did not opt in to milestone-v1",
		}}
	}
	var unknowns []string
	for _, d := range plan.Diagnostics {
		unknowns = append(unknowns, fmt.Sprintf("%s: %s", d.Path, d.Reason))
	}
	for _, u := range plan.Unknowns {
		unknowns = append(unknowns, fmt.Sprintf("%s: %s", u.Path, u.Reason))
	}
	if rr != nil {
		unknowns = append(unknowns, rr.Coverage.Unknowns...)
		if rr.Coverage.Incomplete && len(rr.Coverage.Unknowns) == 0 {
			unknowns = append(unknowns, "readiness coverage is incomplete")
		}
	}
	if plan.Coverage != MilestoneCoverageKnown && len(unknowns) == 0 {
		unknowns = append(unknowns, "milestone plan coverage is unknown")
	}
	sort.Strings(unknowns)

	if len(plan.Records) == 0 {
		if len(unknowns) > 0 {
			return streamview.Milestones{Provenance: streamview.Provenance{
				Availability: streamview.CouldNotCheck, ObservedAt: observed,
				Reason:      "the milestone plan could not be read",
				Diagnostics: unknowns,
			}}
		}
		return streamview.Milestones{
			Provenance: streamview.Provenance{Availability: streamview.Available, Sources: []streamview.SourceRef{src}, ObservedAt: observed},
			Coverage:   streamview.CoverageKnown,
		}
	}

	m := streamview.Milestones{Provenance: streamview.Provenance{Availability: streamview.Available, Sources: []streamview.SourceRef{src}, ObservedAt: observed}}
	if len(unknowns) > 0 {
		m.Provenance.Availability = streamview.Partial
		m.Provenance.Reason = "milestone coverage is incomplete"
		m.Coverage, m.Unknowns = streamview.CoverageUnknown, unknowns
	} else {
		m.Coverage = streamview.CoverageKnown
	}
	byBrief := map[string][]string{}
	for _, rec := range plan.Records {
		it := streamview.MilestoneItem{Ref: rec.Ref(), Title: rec.Title, Owner: rec.Owner, Scope: []string{}}
		for _, c := range rec.Contributes {
			it.Scope = append(it.Scope, c.LogicalKey)
			byBrief[c.LogicalKey] = append(byBrief[c.LogicalKey], it.Ref)
		}
		for _, r := range rec.RequiresMilestones {
			it.Prerequisites = append(it.Prerequisites, fmt.Sprintf("%s@%d", r.Identity.String(), r.Revision))
		}
		seen := map[string]bool{}
		for _, c := range rec.Criteria {
			for _, p := range c.Evidence {
				ref := streamview.EvidenceRef{Kind: streamview.EvidencePath, Repo: repo, Path: p}
				if !seen[p] && ref.Validate() == nil {
					seen[p] = true
					it.Evidence = append(it.Evidence, ref)
				}
			}
		}
		var read *evaluation.MilestoneReadiness
		if rr != nil {
			if r, ok := rr.Milestones[it.Ref]; ok {
				read = &r
			}
		}
		projectReadiness(&it, read, rr != nil)
		m.Items = append(m.Items, it)
	}
	briefs := make([]string, 0, len(byBrief))
	for b := range byBrief {
		briefs = append(briefs, b)
	}
	sort.Strings(briefs)
	for _, b := range briefs {
		refs := byBrief[b]
		sort.Strings(refs)
		m.Membership = append(m.Membership, streamview.MilestoneMembership{Brief: b, Milestones: refs})
	}
	return m
}

// projectReadiness copies the evaluator's present assessment onto an item.
// Anything the evaluator did not supply stays unknown with a reason.
func projectReadiness(it *streamview.MilestoneItem, r *evaluation.MilestoneReadiness, evaluated bool) {
	why := "no readiness evaluation was supplied"
	if evaluated {
		why = "the readiness evaluation has no result for this milestone"
	}
	if r == nil || r.IncompleteCoverage {
		if r != nil {
			why = "readiness coverage for this milestone is incomplete"
		}
		it.Work = streamview.MilestoneWork{State: streamview.WorkUnknown, Reason: why}
		it.Acceptance = streamview.MilestoneAcceptance{State: streamview.AcceptanceCouldNotCheck, Reason: why}
		it.Availability = streamview.MilestoneUnknown
		if r != nil {
			it.Holds = projectHolds(r)
		}
		return
	}
	pending := append([]string(nil), r.Remaining...)
	sort.Strings(pending)
	it.Work = streamview.MilestoneWork{State: streamview.WorkKnown, Remaining: r.WorkRemaining, Pending: pending}
	if len(pending) != r.WorkRemaining {
		it.Work.Pending = nil
	}
	it.Holds = projectHolds(r)
	switch {
	case r.AcceptedForSubject && r.WorkComplete:
		it.Acceptance = streamview.MilestoneAcceptance{State: streamview.AcceptanceAccepted}
	case r.Subject.Token == "" && !r.AcceptedForSubject:
		it.Acceptance = streamview.MilestoneAcceptance{State: streamview.AcceptanceCouldNotCheck, Reason: "no acceptance subject or observation was supplied"}
	case r.AcceptanceOutstanding && r.WorkComplete:
		it.Acceptance = streamview.MilestoneAcceptance{State: streamview.AcceptanceOutstanding}
	case !r.WorkComplete:
		it.Acceptance = streamview.MilestoneAcceptance{State: streamview.AcceptanceNotDue}
	default:
		it.Acceptance = streamview.MilestoneAcceptance{State: streamview.AcceptanceCouldNotCheck, Reason: "the acceptance reading is inconsistent with the work reading"}
	}
	switch {
	case r.Availability == evaluation.AvailabilitySupported && it.Acceptance.State == streamview.AcceptanceAccepted && len(it.Holds) == 0:
		it.Availability = streamview.MilestoneSupported
	case r.Availability == evaluation.AvailabilityBlocked && len(it.Holds) > 0:
		it.Availability = streamview.MilestoneBlocked
	default:
		it.Availability = streamview.MilestoneUnknown
	}
}

func projectHolds(r *evaluation.MilestoneReadiness) []streamview.MilestoneHold {
	var out []streamview.MilestoneHold
	for _, h := range r.Holds {
		reason := h.Reason
		if reason == "" {
			reason = "no reason recorded"
		}
		out = append(out, streamview.MilestoneHold{Ref: h.Ref, Kind: string(h.Kind), State: string(h.State), Reason: reason})
	}
	return out
}

// streamViewVersioned assembles the view at the version both sides support.
// A stream that did not opt in to milestone-v1 offers only v1 and gets the
// unchanged seed; an opted-in stream also offers v2 with the milestone section.
// A consumer that accepts no offered version gets an error, never a silent
// downgrade or a section it did not ask for.
func streamViewVersioned(s *Stream, rr *evaluation.ReadinessResult, defaultRepo, readmeRel, revision string, observed time.Time, ctx streamview.AccessContext, accepted []string) (*streamview.StreamView, error) {
	offered := []string{streamview.Version}
	optedIn := s.Milestones != nil && s.Milestones.OptedIn
	if optedIn {
		offered = []string{streamview.Version2, streamview.Version}
	}
	version, err := streamview.Negotiate(offered, accepted)
	if err != nil {
		return nil, err
	}
	v, err := streamViewSeed(s, defaultRepo, readmeRel, revision, observed, ctx)
	if err != nil {
		return nil, err
	}
	if version == streamview.Version {
		return v, nil
	}
	v.Contract = streamview.Version2
	src := streamview.SourceRef{Repo: v.Identity.Key.Repo, Path: readmeRel, Revision: revision}
	section := streamMilestoneSection(s.Milestones, rr, v.Identity.Key.Repo, src, observed)
	v.Milestones = &section
	if err := v.Validate(); err != nil {
		return nil, err
	}
	return v, nil
}
