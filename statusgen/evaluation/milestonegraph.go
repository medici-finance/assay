package evaluation

import (
	"sort"
	"strconv"
	"strings"
)

// milestonegraph.go — the combined milestone and brief dependency graph
// (milestone-support/05, spec/milestone-v1.md §4–§5).
//
// Two graphs share one node set. Execution holds a brief's own `depends:` and
// its explicit `requires-milestones:` start requirements. Acceptance (here:
// completion) holds a milestone's `contributes:` and its own
// `requires-milestones:`. Edges point from a consumer to what it requires.
//
// The function is pure: it reads only the values it is handed and the injected
// read-only source port. It opens nothing, fetches nothing, and grants no
// dispatch, claim or acceptance. Acceptance is not observed in this generation,
// so a start requirement is never reported satisfied here: it is blocked when
// the milestone has known remaining work, unknown otherwise.
//
// What it refuses to do is as important as what it does:
//   - a brief's start gates are exactly its own requires-milestones; nothing is
//     inherited from the milestones it contributes to, and no milestone is
//     chained to another unless a record says so;
//   - an unknown identity, root, record or pinned revision is unknown, never
//     "no dependency" and never done;
//   - withdrawn or missing work never satisfies an edge;
//   - a cycle over the combined graph (a brief requiring its own containing
//     milestone, directly or across roots) is reported and every node on it is
//     refused.

// GraphState is the derived state of a milestone's completion or of a start gate.
type GraphState string

const (
	// GraphComplete: every contributor is done and every prerequisite is
	// complete. Completion is not acceptance.
	GraphComplete GraphState = "complete"
	// GraphBlocked: a known unsatisfied requirement exists.
	GraphBlocked GraphState = "blocked"
	// GraphUnknown: no known blocker, but something could not be established.
	GraphUnknown GraphState = "unknown"
	// GraphCycle: the node sits on a cycle of the combined graph.
	GraphCycle GraphState = "cycle"
)

// WorkPin is one brief reference: its logical key and the optional exact
// authored revision it pins (0 = unpinned).
type WorkPin struct {
	Key      string
	Revision uint64
	Spelling string
}

// MilestonePin is one exact versioned milestone reference. An empty Key means
// the reference could not be resolved; Why says why.
type MilestonePin struct {
	Key      string
	Version  uint64
	Spelling string
	Why      string
}

// Ref is the exact versioned spelling Key@Version, or the authored spelling
// when the reference did not resolve.
func (p MilestonePin) Ref() string {
	if p.Key == "" {
		return p.Spelling
	}
	return p.Key + "@" + strconv.FormatUint(p.Version, 10)
}

// GraphBrief is the observation of one brief at the selected source snapshot.
type GraphBrief struct {
	Key string // logical key: the bound uuid, else the canonical identity
	// Root and ID are the brief's owning source root and authored uuid, carried
	// through from the source port unchanged; the graph never mints either.
	Root     string
	ID       string
	Status   string
	Revision uint64         // authored revision observed; 0 = none recorded
	Depends  []string       // logical keys of the brief's own depends
	Requires []MilestonePin // its own explicit start requirements
}

// GraphMilestone is one loaded (identity, contract-version) record.
type GraphMilestone struct {
	Key         string
	Version     uint64
	Contributes []WorkPin
	Requires    []MilestonePin
	// Unknowns are coverage gaps the record loader kept visible for this record.
	Unknowns []string
}

// Ref is the exact versioned reference to this record.
func (m GraphMilestone) Ref() string { return m.Key + "@" + strconv.FormatUint(m.Version, 10) }

// PinObservation is the source port's answer for one pin.
type PinObservation struct {
	Found bool
	Brief GraphBrief
	Why   string // why it is unavailable, when !Found
}

// BriefLookup is the injected read-only source port. It resolves a pin at the
// selected source snapshot of the pin's own root, and returns !Found when the
// root, the brief or the exact pinned revision is unavailable. It must never
// substitute a different revision for a pinned one.
type BriefLookup func(pin WorkPin) PinObservation

// MilestoneGraphInput is everything BuildMilestoneGraph reads.
type MilestoneGraphInput struct {
	// Briefs are the briefs whose start gates are evaluated (execution scope).
	Briefs     []GraphBrief
	Milestones []GraphMilestone
	// Lookup resolves contributors. nil observes nothing: every contributor is unknown.
	Lookup BriefLookup
}

// StartGate is one of a brief's own start requirements, resolved.
type StartGate struct {
	Milestone string // exact versioned ref
	Spelling  string
	State     GraphState // blocked, unknown or cycle; never satisfied in this generation
	Blockers  []string
	Unknowns  []string
}

// BriefStart is a brief's derived execution view.
type BriefStart struct {
	Key      string
	Root, ID string // carried through from the source port, never minted
	// Gates are exactly the brief's own requires-milestones, in authored order.
	Gates []StartGate
	// Membership is the derived, deduplicated list of milestones the brief
	// contributes to. It adds no start gate.
	Membership []string
	OnCycle    bool
}

// MilestoneNode is a milestone's derived completion view.
type MilestoneNode struct {
	Ref           string
	Contributors  []string // deduplicated logical brief keys
	Prerequisites []string
	Completion    GraphState
	Blockers      []string
	Unknowns      []string
}

// MilestoneGraph is the combined graph's typed result.
type MilestoneGraph struct {
	Briefs     map[string]BriefStart
	Milestones map[string]MilestoneNode
	// Cycles lists every cycle as its sorted node names ("brief <key>",
	// "milestone <ref>").
	Cycles [][]string
}

// ContributorCount counts the distinct briefs contributing to the named
// milestones: a brief shared by several of them is counted once.
func (g MilestoneGraph) ContributorCount(refs ...string) int {
	seen := map[string]bool{}
	for _, r := range refs {
		for _, k := range g.Milestones[r].Contributors {
			seen[k] = true
		}
	}
	return len(seen)
}

func briefNode(key string) string     { return "brief " + key }
func milestoneNode(ref string) string { return "milestone " + ref }

// BuildMilestoneGraph derives the combined graph from explicit values. It feeds
// the S-eligibility owner (EvaluateEligibility) pre-resolved start gates; it is
// not a second eligibility implementation and carries no semantic marker.
func BuildMilestoneGraph(in MilestoneGraphInput) MilestoneGraph {
	g := MilestoneGraph{Briefs: map[string]BriefStart{}, Milestones: map[string]MilestoneNode{}}

	records := map[string]GraphMilestone{}
	for _, m := range in.Milestones {
		records[m.Ref()] = m
	}

	// Observed briefs: the execution scope, then every contributor the port finds.
	briefs := map[string]GraphBrief{}
	for _, b := range in.Briefs {
		briefs[b.Key] = b
	}
	contributor := map[string]PinObservation{} // milestone ref + "\x00" + pin key
	for _, m := range in.Milestones {
		for _, pin := range m.Contributes {
			obs := PinObservation{Why: "no source port: contributor " + pin.Key + " was not observed"}
			if in.Lookup != nil {
				obs = in.Lookup(pin)
			}
			contributor[m.Ref()+"\x00"+pin.Key] = obs
			if obs.Found {
				if _, ok := briefs[obs.Brief.Key]; !ok {
					briefs[obs.Brief.Key] = obs.Brief
				}
			}
		}
	}

	// The combined edge set.
	edges := map[string][]string{}
	link := func(from, to string) { edges[from] = append(edges[from], to) }
	for _, b := range briefs {
		for _, d := range b.Depends {
			link(briefNode(b.Key), briefNode(d))
		}
		for _, r := range b.Requires {
			if r.Key != "" {
				link(briefNode(b.Key), milestoneNode(r.Ref()))
			}
		}
	}
	for _, m := range in.Milestones {
		for _, pin := range m.Contributes {
			link(milestoneNode(m.Ref()), briefNode(pin.Key))
		}
		for _, r := range m.Requires {
			if r.Key != "" {
				link(milestoneNode(m.Ref()), milestoneNode(r.Ref()))
			}
		}
	}
	onCycle := map[string]bool{}
	for _, scc := range stronglyConnected(edges) {
		if len(scc) == 1 && !hasEdge(edges, scc[0], scc[0]) {
			continue
		}
		sort.Strings(scc)
		g.Cycles = append(g.Cycles, scc)
		for _, n := range scc {
			onCycle[n] = true
		}
	}
	sort.Slice(g.Cycles, func(i, j int) bool { return strings.Join(g.Cycles[i], ",") < strings.Join(g.Cycles[j], ",") })

	// Milestone completion, derived recursively over prerequisites.
	var complete func(ref string, visiting map[string]bool) MilestoneNode
	complete = func(ref string, visiting map[string]bool) MilestoneNode {
		if n, ok := g.Milestones[ref]; ok {
			return n
		}
		m := records[ref]
		n := MilestoneNode{Ref: ref}
		if onCycle[milestoneNode(ref)] {
			n.Completion = GraphCycle
			n.Blockers = append(n.Blockers, "milestone "+ref+" is on a dependency cycle")
		}
		visiting[ref] = true
		seen := map[string]bool{}
		for _, pin := range m.Contributes {
			if seen[pin.Key] {
				continue
			}
			seen[pin.Key] = true
			n.Contributors = append(n.Contributors, pin.Key)
			obs := contributor[ref+"\x00"+pin.Key]
			switch {
			case !obs.Found:
				n.Unknowns = append(n.Unknowns, "contributor "+pinSpelling(pin)+" is unknown: "+obs.Why)
			case obs.Brief.Status == "withdrawn":
				n.Blockers = append(n.Blockers, "contributor "+pin.Key+" is withdrawn; withdrawn work never satisfies a contribution")
			case obs.Brief.Status != "done":
				n.Blockers = append(n.Blockers, "contributor "+pin.Key+" is "+statusOrUnset(obs.Brief.Status))
			}
		}
		for _, r := range m.Requires {
			n.Prerequisites = append(n.Prerequisites, r.Ref())
			b, u := requirement(r, records, visiting, complete)
			n.Blockers = append(n.Blockers, b...)
			n.Unknowns = append(n.Unknowns, u...)
		}
		n.Unknowns = append(n.Unknowns, m.Unknowns...)
		delete(visiting, ref)
		switch {
		case n.Completion == GraphCycle:
		case len(n.Blockers) > 0:
			n.Completion = GraphBlocked
		case len(n.Unknowns) > 0:
			n.Completion = GraphUnknown
		default:
			n.Completion = GraphComplete
		}
		g.Milestones[ref] = n
		return n
	}
	refs := make([]string, 0, len(records))
	for r := range records {
		refs = append(refs, r)
	}
	sort.Strings(refs)
	for _, r := range refs {
		complete(r, map[string]bool{})
	}

	// Reverse membership, derived and deduplicated.
	membership := map[string][]string{}
	for _, r := range refs {
		for _, k := range g.Milestones[r].Contributors {
			membership[k] = append(membership[k], r)
		}
	}

	// Start gates: each brief's own requirements only.
	for _, b := range in.Briefs {
		s := BriefStart{Key: b.Key, Root: b.Root, ID: b.ID, Membership: membership[b.Key], OnCycle: onCycle[briefNode(b.Key)]}
		for _, r := range b.Requires {
			gate := StartGate{Milestone: r.Ref(), Spelling: r.Spelling}
			if r.Key != "" && onCycle[briefNode(b.Key)] && onCycle[milestoneNode(r.Ref())] && sameCycle(g.Cycles, briefNode(b.Key), milestoneNode(r.Ref())) {
				gate.State = GraphCycle
				gate.Blockers = []string{"brief " + b.Key + " and milestone " + r.Ref() + " are on one dependency cycle"}
				s.Gates = append(s.Gates, gate)
				continue
			}
			gate.Blockers, gate.Unknowns = requirement(r, records, map[string]bool{}, complete)
			switch {
			case len(gate.Blockers) > 0:
				gate.State = GraphBlocked
			default:
				gate.State = GraphUnknown
				if len(gate.Unknowns) == 0 {
					gate.Unknowns = []string{"milestone " + r.Ref() + " has no remaining work; its acceptance is not observed by this reader"}
				}
			}
			s.Gates = append(s.Gates, gate)
		}
		g.Briefs[b.Key] = s
	}
	return g
}

// requirement resolves one required milestone (a prerequisite or a start
// requirement) into known blockers and unknowns. Completion is not acceptance:
// a complete prerequisite still leaves its acceptance unknown.
func requirement(r MilestonePin, records map[string]GraphMilestone, visiting map[string]bool, complete func(string, map[string]bool) MilestoneNode) (blockers, unknowns []string) {
	if r.Key == "" {
		return nil, []string{"milestone " + r.Spelling + " does not resolve: " + r.Why}
	}
	ref := r.Ref()
	if _, ok := records[ref]; !ok {
		return nil, []string{"milestone " + ref + " is not a loaded record version"}
	}
	if visiting[ref] {
		return []string{"milestone " + ref + " is on a dependency cycle"}, nil
	}
	n := complete(ref, visiting)
	switch n.Completion {
	case GraphCycle:
		return []string{"milestone " + ref + " is on a dependency cycle"}, nil
	case GraphComplete:
		return nil, []string{"milestone " + ref + " acceptance is not observed"}
	}
	for _, b := range n.Blockers {
		blockers = append(blockers, ref+": "+b)
	}
	for _, u := range n.Unknowns {
		unknowns = append(unknowns, ref+": "+u)
	}
	return blockers, unknowns
}

func pinSpelling(p WorkPin) string {
	if p.Spelling != "" {
		return p.Spelling
	}
	return p.Key
}

func statusOrUnset(s string) string {
	if s == "" {
		return "of unknown status"
	}
	return s
}

func hasEdge(edges map[string][]string, from, to string) bool {
	for _, t := range edges[from] {
		if t == to {
			return true
		}
	}
	return false
}

func sameCycle(cycles [][]string, a, b string) bool {
	for _, c := range cycles {
		var ha, hb bool
		for _, n := range c {
			ha = ha || n == a
			hb = hb || n == b
		}
		if ha && hb {
			return true
		}
	}
	return false
}

// stronglyConnected returns the strongly connected components of edges
// (Tarjan), deterministically ordered.
func stronglyConnected(edges map[string][]string) [][]string {
	nodes := map[string]bool{}
	for from, tos := range edges {
		nodes[from] = true
		for _, t := range tos {
			nodes[t] = true
		}
	}
	order := make([]string, 0, len(nodes))
	for n := range nodes {
		order = append(order, n)
	}
	sort.Strings(order)
	index, low := map[string]int{}, map[string]int{}
	onStack := map[string]bool{}
	var stack []string
	var out [][]string
	next := 0
	var visit func(v string)
	visit = func(v string) {
		index[v], low[v] = next, next
		next++
		stack = append(stack, v)
		onStack[v] = true
		tos := append([]string(nil), edges[v]...)
		sort.Strings(tos)
		for _, w := range tos {
			if _, seen := index[w]; !seen {
				visit(w)
				if low[w] < low[v] {
					low[v] = low[w]
				}
			} else if onStack[w] && index[w] < low[v] {
				low[v] = index[w]
			}
		}
		if low[v] == index[v] {
			var scc []string
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[w] = false
				scc = append(scc, w)
				if w == v {
					break
				}
			}
			out = append(out, scc)
		}
	}
	for _, v := range order {
		if _, seen := index[v]; !seen {
			visit(v)
		}
	}
	return out
}
