package main

// milestonegraph.go — the statusgen ADAPTER for the combined milestone and
// brief dependency graph (milestone-support/05). The graph itself is
// evaluation.BuildMilestoneGraph; this file only maps what the scan already
// observed — stream rows, brief-v4 start requirements, the record loader's
// milestone plans and each root's selected source snapshot — onto its explicit
// input values, and writes the typed result back as eligibility gates.
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
// input and each scanned row's logical key ("" when the row's owning root could
// not be established).
func milestoneGraphInput(streams []*Stream, pending []milestoneStartPending, snaps map[string]briefSnapshot, regFor func(string) *graphRepos) (evaluation.MilestoneGraphInput, map[*Brief]string, map[*Brief]milestoneRootOwner) {
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

	var in evaluation.MilestoneGraphInput
	keyOf := map[*Brief]string{}
	ownerByRow := map[*Brief]milestoneRootOwner{}
	index := map[string]evaluation.GraphBrief{}
	for _, s := range streams {
		o := ownerOf(s.Root)
		for i := range s.Briefs {
			row := &s.Briefs[i]
			ownerByRow[row] = o
			if o.ctx == nil {
				continue
			}
			snap := snaps[s.Root][s.Name+"/"+row.Num]
			b := evaluation.GraphBrief{
				Key:      briefGraphKey(o.ctx, s.Name, row.Num),
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
			return evaluation.PinObservation{Why: "brief " + pin.Key + " is not in a loaded root's source snapshot"}
		}
		bound := bindBriefSelector(BriefRefSelector{Key: pin.Key, Ref: workref.Reference{Revision: pin.Revision}}, briefSnapshot{pin.Key: {Version: int(b.Revision), ID: b.ID}})
		if !bound.Bound {
			return evaluation.PinObservation{Why: bound.Reason}
		}
		return evaluation.PinObservation{Found: true, Brief: b}
	}
	return in, keyOf, ownerByRow
}

func briefGraphKey(ctx *workref.Context, stream, num string) string {
	return workref.Identity{Kind: workref.Brief, Cell: ctx.Cell, Repo: ctx.Repo, Stream: stream, Number: num}.String()
}

// applyMilestoneStartGates builds the combined graph after the scan and writes
// each brief-v4 start requirement onto its row as a resolved eligibility gate.
// A cycle through a milestone is a PROBLEM: a brief requiring its own
// containing milestone, directly or across roots, is refused.
func applyMilestoneStartGates(add, notice func(string, ...any), streams []*Stream, pending []milestoneStartPending, snaps map[string]briefSnapshot, regFor func(string) *graphRepos) {
	if len(pending) == 0 && !anyMilestoneRecords(streams) {
		return
	}
	in, keyOf, owners := milestoneGraphInput(streams, pending, snaps, regFor)
	g := evaluation.BuildMilestoneGraph(in)
	for _, c := range g.Cycles {
		if milestoneCycle(c) {
			add("milestone graph: dependency cycle %s — a brief cannot require a milestone its own work completes; refused", strings.Join(c, " -> "))
		}
	}
	for _, p := range pending {
		row := p.row
		gates := append([]GraphEdge(nil), row.Gates...)
		key := keyOf[row]
		if key == "" {
			why := "the owning root could not be established: " + owners[row].why
			for _, m := range p.requires {
				gates = append(gates, GraphEdge{Ref: m, Type: "behavioural-gate", Reason: "requires-milestones", Resolved: StateCouldNotCheck, Why: why})
				notice("%s: requires-milestones %s — held could-not-check: %s", p.path, m, why)
			}
			row.Gates = gates
			continue
		}
		for _, gate := range g.Briefs[key].Gates {
			state, why := StateCouldNotCheck, strings.Join(gate.Unknowns, "; ")
			if gate.State == evaluation.GraphBlocked || gate.State == evaluation.GraphCycle {
				state, why = StateUnsatisfied, strings.Join(gate.Blockers, "; ")
				if len(gate.Unknowns) > 0 {
					why += "; also unknown: " + strings.Join(gate.Unknowns, "; ")
				}
			}
			gates = append(gates, GraphEdge{Ref: gate.Spelling, Type: "behavioural-gate", Reason: "requires-milestones: milestone " + string(gate.State), Resolved: state, Why: why})
			notice("%s: requires-milestones %s — held %s (milestone %s): %s", p.path, gate.Spelling, state, gate.State, why)
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
