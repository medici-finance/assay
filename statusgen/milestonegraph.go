package main

// milestonegraph.go — the statusgen ADAPTER for the combined milestone and
// brief dependency graph (milestone-support/05). The graph itself is
// evaluation.BuildMilestoneGraph; this file only maps what the scan already
// observed — stream rows, brief-v4 start requirements, the record loader's
// milestone plans and each root's selected source snapshot — onto its explicit
// input values. EvaluateReadiness supplies the typed eligibility gates.
//
// It replaces the blanket hold brief-v4 used to place on every
// requires-milestones entry (a could-not-check gate whatever the graph said).
// A start requirement now holds the brief as unsatisfied when the milestone has
// known remaining work, as could-not-check when something is unknown, and is
// refused when the brief and the milestone sit on one cycle. It is never
// satisfied here: milestone acceptance is not observed by this reader.
//
// Milestone records are read only for streams whose reader-requirements this
// statusgen supports (attachMilestonePlans); while milestone-v1 is unadvertised
// every requirement is therefore unknown, exactly as before, with the reason
// named. Nothing here reads the network, mints a key or changes a status.

import (
	"slices"
	"strings"

	"github.com/medici-finance/assay/statusgen/evaluation"
	"github.com/medici-finance/assay/statusgen/workref"
)

// milestoneStartPending is one brief-v4 row with start requirements, held for
// the post-scan graph.
type milestoneStartPending struct {
	path     string
	stream   *Stream
	row      *Brief
	requires []string
}

// milestoneRootOwner is one root's canonical owning context and registry.
type milestoneRootOwner struct {
	ctx *workref.Context
	reg workref.Registry
	why string
}

// milestoneGraphInput maps the scan onto the pure graph's input. It returns the
// input, each scanned row's logical key, and — for a row with no usable key —
// why its start requirements must be held could-not-check instead.
//
// A row has no usable key when its owning root could not be established, or
// when its logical key is claimed by more than one scanned row (two roots whose
// registries resolve to one cell and self alias, sharing a stream and number).
// A collision never picks a winner: every claimant is left out of the graph and
// the source port, so no row can receive another row's gates or status.
func milestoneGraphInput(streams []*Stream, pending []milestoneStartPending, snaps map[string]briefSnapshot, regFor func(string) *graphRepos) (evaluation.MilestoneGraphInput, map[*Brief]string, map[*Brief]string) {
	owners := map[string]milestoneRootOwner{}
	ownerOf := func(root string) milestoneRootOwner {
		if o, ok := owners[root]; ok {
			return o
		}
		ctx, reg, why := milestoneOwner(regFor(root))
		o := milestoneRootOwner{ctx: ctx, reg: reg, why: why}
		owners[root] = o
		return o
	}
	requiresOf := map[*Brief][]string{}
	for _, p := range pending {
		requiresOf[p.row] = p.requires
	}

	// Every claimant of each logical key, before any row is admitted.
	claims := map[string][]string{}
	for _, s := range streams {
		if o := ownerOf(s.Root); o.ctx != nil {
			for _, row := range s.Briefs {
				k, at := briefGraphKey(o.ctx, s.Name, row.Num), s.Root+" "+s.Name+"/"+row.Num
				if !slices.Contains(claims[k], at) { // one row seen twice is one claimant
					claims[k] = append(claims[k], at)
				}
			}
		}
	}
	collision := func(k string) string {
		if len(claims[k]) < 2 {
			return ""
		}
		return "logical key " + k + " is claimed by more than one scanned brief (" + strings.Join(claims[k], ", ") + "); no claimant is chosen"
	}
	records := map[string][]string{}
	for _, s := range streams {
		if s.Milestones == nil {
			continue
		}
		for _, rec := range s.Milestones.Records {
			ref, at := (evaluation.GraphMilestone{Key: rec.Key, Version: rec.ContractVersion}).Ref(), s.Root+" "+s.Name+" "+rec.Path
			if !slices.Contains(records[ref], at) {
				records[ref] = append(records[ref], at)
			}
		}
	}
	recordCollision := func(ref string) string {
		if len(records[ref]) < 2 {
			return ""
		}
		return "milestone " + ref + " is declared by more than one loaded record (" + strings.Join(records[ref], ", ") + "); no record is chosen"
	}

	var in evaluation.MilestoneGraphInput
	keyOf := map[*Brief]string{}
	heldWhy := map[*Brief]string{}
	index := map[string]evaluation.GraphBrief{}
	for _, s := range streams {
		o := ownerOf(s.Root)
		for i := range s.Briefs {
			row := &s.Briefs[i]
			if o.ctx == nil {
				heldWhy[row] = "the owning root could not be established: " + o.why
				continue
			}
			key := briefGraphKey(o.ctx, s.Name, row.Num)
			if why := collision(key); why != "" {
				heldWhy[row] = why
				continue
			}
			snap := snaps[s.Root][s.Name+"/"+row.Num]
			b := evaluation.GraphBrief{
				Key:      key,
				Root:     s.Root,
				ID:       snap.ID,
				Status:   row.Status,
				Revision: uint64(max(snap.Version, 0)),
			}
			for _, d := range row.Depends {
				if st, num, ok := strings.Cut(d, "/"); ok {
					b.Depends = append(b.Depends, briefGraphKey(o.ctx, st, num))
				}
			}
			for _, m := range requiresOf[row] {
				res, err := workref.Resolve(m, o.ctx, o.reg, workref.TypedMilestone)
				if err != nil {
					b.Requires = append(b.Requires, evaluation.MilestonePin{Spelling: m, Why: err.Error()})
					continue
				}
				b.Requires = append(b.Requires, evaluation.MilestonePin{Key: res.Identity.String(), Version: res.Revision, Spelling: m})
			}
			keyOf[row] = b.Key
			index[b.Key] = b
			// Every scanned brief is in the execution scope, so an indirect
			// cycle through a brief with no requirement of its own is still seen.
			in.Briefs = append(in.Briefs, b)
		}
		if s.Milestones == nil {
			continue
		}
		for _, rec := range s.Milestones.Records {
			m := evaluation.GraphMilestone{Key: rec.Key, Version: rec.ContractVersion}
			if why := recordCollision(m.Ref()); why != "" {
				// The same class as a brief key collision: no record is chosen,
				// so every gate on this version stays unknown.
				if !slices.ContainsFunc(in.Milestones, func(x evaluation.GraphMilestone) bool { return x.Ref() == m.Ref() }) {
					m.Unknowns = []string{why}
					in.Milestones = append(in.Milestones, m)
				}
				continue
			}
			for _, c := range rec.Contributes {
				m.Contributes = append(m.Contributes, evaluation.WorkPin{Key: c.LogicalKey, Revision: c.Revision, Spelling: c.Spelling})
			}
			for _, r := range rec.RequiresMilestones {
				m.Requires = append(m.Requires, evaluation.MilestonePin{Key: r.Identity.String(), Version: r.Revision, Spelling: r.Spelling})
			}
			for _, u := range s.Milestones.Unknowns {
				if u.Path == rec.Path || u.Path == "" || u.Path == "README.md" {
					m.Unknowns = append(m.Unknowns, u.Reason)
				}
			}
			in.Milestones = append(in.Milestones, m)
		}
	}
	// The read-only source port: a brief at its root's selected snapshot. The
	// revision rule is bindBriefSelector's — an unversioned pin binds what is
	// observed, a pinned one only an exactly matching revision, never another.
	in.Lookup = func(pin evaluation.WorkPin) evaluation.PinObservation {
		b, ok := index[pin.Key]
		if !ok {
			if why := collision(pin.Key); why != "" {
				return evaluation.PinObservation{Why: why}
			}
			return evaluation.PinObservation{Why: "brief " + pin.Key + " is not in a loaded root's source snapshot"}
		}
		bound := bindBriefSelector(BriefRefSelector{Key: pin.Key, Ref: workref.Reference{Revision: pin.Revision}}, briefSnapshot{pin.Key: {Version: int(b.Revision), ID: b.ID}})
		if !bound.Bound {
			return evaluation.PinObservation{Why: bound.Reason}
		}
		return evaluation.PinObservation{Found: true, Brief: b}
	}
	return in, keyOf, heldWhy
}

func briefGraphKey(ctx *workref.Context, stream, num string) string {
	return workref.Identity{Kind: workref.Brief, Cell: ctx.Cell, Repo: ctx.Repo, Stream: stream, Number: num}.String()
}

// applyMilestoneStartGates builds the combined graph after the scan and writes
// each brief-v4 start requirement onto its row via EvaluateReadiness.
// A cycle through a milestone is a PROBLEM: a brief requiring its own
// containing milestone, directly or across roots, is refused.
func applyMilestoneStartGates(add, notice func(string, ...any), streams []*Stream, pending []milestoneStartPending, snaps map[string]briefSnapshot, regFor func(string) *graphRepos) {
	if len(pending) == 0 && !anyMilestoneRecords(streams) {
		return
	}
	in, keyOf, heldWhy := milestoneGraphInput(streams, pending, snaps, regFor)
	g := evaluation.BuildMilestoneGraph(in)
	for _, c := range g.Cycles {
		if milestoneCycle(c) {
			add("milestone graph: dependency cycle %s — a brief cannot require a milestone its own work completes; refused", strings.Join(c, " -> "))
		}
	}
	observations := &evaluation.MilestoneInput{Graph: g}
	for _, p := range pending {
		if why := heldWhy[p.row]; why != "" {
			observations.Unknowns = append(observations.Unknowns, why)
		}
	}
	for _, s := range streams {
		if s.Milestones != nil {
			for _, u := range s.Milestones.Unknowns {
				observations.Unknowns = append(observations.Unknowns, s.Name+": "+u.Reason)
			}
		}
	}
	applyMilestoneReadiness(notice, streams, pending, keyOf, heldWhy, observations)

}

// applyMilestoneReadiness maps explicit graph/receipt observations through the
// canonical owner. Production supplies no selector/subjects/acceptance yet.
func applyMilestoneReadiness(notice func(string, ...any), streams []*Stream, pending []milestoneStartPending, keyOf map[*Brief]string, heldWhy map[*Brief]string, milestones *evaluation.MilestoneInput) {
	es := evaluationStreams(streams)
	for i, s := range streams {
		for j := range s.Briefs {
			es[i].Briefs[j].Key = keyOf[&s.Briefs[j]]
		}
	}
	result := evaluation.EvaluateReadiness(evaluation.EligibilityInput{Streams: es, Milestones: milestones})
	for _, s := range streams {
		s.MilestoneReadiness = &result
	}
	for _, p := range pending {
		row := p.row
		gates := append([]GraphEdge(nil), row.Gates...)
		key := keyOf[row]
		if why := heldWhy[row]; why != "" || key == "" {
			if why == "" {
				why = "the row has no logical key in the combined graph"
			}
			for _, m := range p.requires {
				gates = append(gates, GraphEdge{Ref: m, Type: "behavioural-gate", Reason: "requires-milestones", Resolved: StateCouldNotCheck, Why: why})
				notice("%s: requires-milestones %s — held could-not-check: %s", p.path, m, why)
			}
		} else {
			for _, gate := range result.StartGates[key] {
				gates = append(gates, GraphEdge{Ref: gate.Ref, Type: gate.Type, Reason: gate.Reason, Resolved: gate.State, Why: gate.Why})
				if gate.State != StateSatisfied {
					notice("%s: requires-milestones %s — held %s (%s): %s", p.path, gate.Ref, gate.State, strings.TrimPrefix(gate.Reason, "requires-milestones: "), gate.Why)
				}
			}
		}
		row.Gates = gates
	}
}

func anyMilestoneRecords(streams []*Stream) bool {
	for _, s := range streams {
		if s.Milestones != nil && len(s.Milestones.Records) > 0 {
			return true
		}
	}
	return false
}

// milestoneCycle reports whether a cycle passes through a milestone; pure brief
// depends cycles are the existing wave lint's.
func milestoneCycle(c []string) bool {
	for _, n := range c {
		if strings.HasPrefix(n, "milestone ") {
			return true
		}
	}
	return false
}
