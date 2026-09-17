package main

import (
	"os"
	"path/filepath"
	"strings"
)

// eligibility.go — the eligibility evaluator (graph-execution/01): `gates:`
// and `feathers:` become GATING, with an explainable reason, instead of the
// "reserved, not gating" no-op checkBriefV2Semantics (briefv2.go) left them
// at. See docs/dependency-graph-design.md §3.6 for the semantics executed
// here.
//
// The evaluator is the SINGLE deterministic verdict every consumer reads:
// Next-up (eligibleBase, nextup.go), the drive frontier (briefFrontierState,
// drivefrontier.go) and the `--eligibility` CLI all ask the same question of
// the same map, so a brief the board hides can never be offered by the
// frontier and vice versa — the reviewer question this brief's Review
// section asks.
//
// OFFLINE BY CONSTRUCTION: this file never opens a network connection, never
// shells to `gh`, and never consults the run's --forge state. A forge-backed
// ref (`#<NNN>`, `<alias>#<NNN>`) is ALWAYS could-not-check here, regardless
// of whether the run passed --forge — --forge governs OTHER checks' forge
// reads (stale-issue alarms etc.), never this evaluator, which must produce
// its verdicts "from the tree alone" (brief-01 Context, fact 4).

// EligibilityState is the three-state resolution of one edge
// (docs/three-state-instrument-rule.md): could-not-check is reported, never
// collapsed into satisfied or unsatisfied.
type EligibilityState string

const (
	StateSatisfied     EligibilityState = "satisfied"
	StateUnsatisfied   EligibilityState = "unsatisfied"
	StateCouldNotCheck EligibilityState = "could-not-check"
)

// EligibilityVerdict is the three-way dispatch verdict for one brief.
type EligibilityVerdict string

const (
	// VerdictEligible: no holds at all — nothing stands between this brief and
	// dispatch (from the graph's point of view; the caller's own status/claim/
	// wave rules still apply on top).
	VerdictEligible EligibilityVerdict = "eligible"
	// VerdictHeld: at least one gates: edge (or depends: entry) is unsatisfied
	// or could-not-check. HELD is exclusion from Next-up.
	VerdictHeld EligibilityVerdict = "held"
	// VerdictEligibleWithNotice: no holds, but at least one feathers: edge is
	// unsatisfied or could-not-check. Offered, rendered with its notice.
	VerdictEligibleWithNotice EligibilityVerdict = "eligible-with-notice"
)

// EdgeVerdict is one resolved gates:/feathers:/depends: edge — its ref, its
// declared type, its reason (verbatim from the gates: entry; "" for a bare
// depends: hold and for a build-dep feather scalar), and the resolved state.
type EdgeVerdict struct {
	Ref    string           `json:"ref"`
	Type   string           `json:"type"`
	Reason string           `json:"reason,omitempty"`
	State  EligibilityState `json:"state"`
	// Why is the could-not-check explanation (registry/checkout/forge detail).
	// Deliberately excluded from --eligibility --json — Task item 1's shape is
	// exactly {ref,type,reason,state} — but read by the --lint NOTICE emitter
	// (eligibilityCouldNotCheckNotices) so a hold caused by a registry or
	// checkout gap names WHY, not just THAT.
	Why string `json:"-"`
}

// Eligibility is the evaluator's full verdict for one brief.
type Eligibility struct {
	ID      string             `json:"id"` // "<stream>/<NN>"
	Verdict EligibilityVerdict `json:"verdict"`
	Holds   []EdgeVerdict      `json:"holds,omitempty"`
	Notices []EdgeVerdict      `json:"notices,omitempty"`
}

// evaluateEligibility computes the eligibility verdict of every brief across
// streams (not just todo ones — a done brief's verdict is inert but still
// reported, so a caller need not special-case status before indexing the
// map). reg is the loaded docs/streams/graph-repos.yaml registry for THIS
// root (nil when absent — every gates:/feathers: edge on any brief in
// streams is then could-not-check by construction, since a cross-repo alias
// cannot resolve without it; an in-repo ref still resolves against streams
// alone). root anchors any sibling-checkout resolution a cross-repo ref
// needs.
//
// Rules (brief-01 Task item 1), in order:
//   - a gates: entry whose target is not done/verified → unsatisfied → HELD
//   - a gates: entry that cannot be resolved (unpublished alias, absent
//     sibling checkout, forge-backed target) → could-not-check → HELD
//   - a feathers: entry that is unsatisfied or could-not-check → NOTICE only
//     (verdict eligible-with-notice), never a hold
//   - an unsatisfied depends: entry is reported as a hold with type
//     build-dep, so one output explains every reason a brief is not offered
//   - eligible only when holds is empty
func evaluateEligibility(streams []*Stream, reg *graphRepos, root string) map[string]Eligibility {
	out := map[string]Eligibility{}
	for _, s := range streams {
		for _, b := range s.Briefs {
			id := s.Name + "/" + b.Num
			e := Eligibility{ID: id}
			for _, dep := range b.Depends {
				if !depIsSatisfied(streams, dep) {
					e.Holds = append(e.Holds, EdgeVerdict{
						Ref:    dep,
						Type:   "build-dep",
						State:  StateUnsatisfied,
						Reason: "depends: target is not done/verified",
					})
				}
			}
			for _, edge := range b.Gates {
				state, why := resolveGraphEdge(edge.Ref, streams, reg, root)
				if state != StateSatisfied {
					e.Holds = append(e.Holds, EdgeVerdict{
						Ref: edge.Ref, Type: edge.Type, Reason: edge.Reason, State: state, Why: why,
					})
				}
			}
			for _, edge := range b.Feathers {
				state, why := resolveGraphEdge(edge.Ref, streams, reg, root)
				if state != StateSatisfied {
					e.Notices = append(e.Notices, EdgeVerdict{
						Ref: edge.Ref, Type: edge.Type, Reason: edge.Reason, State: state, Why: why,
					})
				}
			}
			switch {
			case len(e.Holds) > 0:
				e.Verdict = VerdictHeld
			case len(e.Notices) > 0:
				e.Verdict = VerdictEligibleWithNotice
			default:
				e.Verdict = VerdictEligible
			}
			out[id] = e
		}
	}
	return out
}

// eligibilityForStreams is the convenience entry point every in-tree consumer
// (nextup.go, drivefrontier.go) uses: it groups streams by their own Root
// (multi-root runs may carry more than one), loads that root's
// docs/streams/graph-repos.yaml once, and evaluates each group. A registry
// load error demotes to "no registry" (every cross-repo edge in that root's
// group is then could-not-check) rather than panicking a board build —
// checkBriefFiles (via checkBriefV2Semantics) is the place a malformed
// registry is already a hard --lint PROBLEM.
func eligibilityForStreams(streams []*Stream) map[string]Eligibility {
	byRoot := map[string][]*Stream{}
	order := []string{}
	for _, s := range streams {
		if _, ok := byRoot[s.Root]; !ok {
			order = append(order, s.Root)
		}
		byRoot[s.Root] = append(byRoot[s.Root], s)
	}
	out := map[string]Eligibility{}
	for _, root := range order {
		reg, _, _ := loadGraphRepos(root)
		for id, e := range evaluateEligibility(byRoot[root], reg, root) {
			out[id] = e
		}
	}
	return out
}

// resolveGraphEdge resolves one gates:/feathers: ref to its state, using the
// §3.3 grammar forms already recognized in briefv2.go (refInRepoBriefRe
// etc). It never touches the network or a forge.
func resolveGraphEdge(ref string, streams []*Stream, reg *graphRepos, root string) (EligibilityState, string) {
	switch {
	case refInRepoBriefRe.MatchString(ref):
		return resolveInRepoBriefRef(ref, streams)
	case refInRepoIssueRe.MatchString(ref):
		return StateCouldNotCheck, "forge-backed gate " + ref + " — the evaluator reads the tree only, never the forge"
	case refCrossRepoIssueRe.MatchString(ref):
		return StateCouldNotCheck, "forge-backed cross-repo gate " + ref + " — the evaluator reads the tree only, never the forge"
	case refCrossRepoBriefRe.MatchString(ref):
		i := strings.IndexByte(ref, ':')
		alias, target := ref[:i], ref[i+1:]
		return resolveCrossRepoBriefRef(alias, target, reg, root)
	case refCrossCellBriefRe.MatchString(ref):
		return StateCouldNotCheck, "cross-cell reference " + ref + " — no cross-cell resolution exists in this evaluator"
	default:
		return StateCouldNotCheck, "ref " + ref + " does not match a recognized §3.3 reference form"
	}
}

// resolveInRepoBriefRef resolves a `<stream>/<NN>` ref against the in-hand
// streams. A ref that cannot be found (unknown stream or brief number) is
// could-not-check, never a silent unsatisfied — the target may simply not
// have been loaded into this run's scope.
func resolveInRepoBriefRef(ref string, streams []*Stream) (EligibilityState, string) {
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) != 2 {
		return StateCouldNotCheck, "malformed in-repo ref " + ref
	}
	for _, s := range streams {
		if s.Name != parts[0] {
			continue
		}
		for _, b := range s.Briefs {
			if b.Num == parts[1] {
				if b.Status == "done" || b.Status == "verified" {
					return StateSatisfied, ""
				}
				return StateUnsatisfied, ""
			}
		}
	}
	return StateCouldNotCheck, "brief " + ref + " not found in this tree"
}

// resolveCrossRepoBriefRef resolves an `<alias>:<stream>/<NN>` ref. An
// unpublished (or unregistered) alias is could-not-check by design
// (docs/dependency-graph-design.md §3.3): the target repo is deliberately
// withheld from this tree, and the evaluator never treats "cannot look" as
// "looked and found satisfied". A published alias resolves against a SIBLING
// CHECKOUT next to root (a directory named after the target repo's basename,
// e.g. "medici-finance/assay" -> "assay"); an absent sibling checkout is the
// same could-not-check class as an unpublished alias (brief-01 Context, fact
// 4: "the same holds when the sibling checkout an alias resolves to is
// absent").
func resolveCrossRepoBriefRef(alias, target string, reg *graphRepos, root string) (EligibilityState, string) {
	if reg == nil {
		return StateCouldNotCheck, "no docs/streams/graph-repos.yaml registry — alias " + alias + " cannot be resolved"
	}
	entry, ok := reg.Aliases[alias]
	if !ok {
		return StateCouldNotCheck, "alias " + alias + " is not in docs/streams/graph-repos.yaml"
	}
	if entry.Unpublished || entry.Repo == "" {
		return StateCouldNotCheck, "alias " + alias + " is unpublished — its target repo is not resolvable from this tree"
	}
	siblingName := entry.Repo
	if i := strings.LastIndexByte(siblingName, '/'); i >= 0 {
		siblingName = siblingName[i+1:]
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		absRoot = root
	}
	siblingRoot := filepath.Join(filepath.Dir(absRoot), siblingName)
	if fi, err := os.Stat(siblingRoot); err != nil || !fi.IsDir() {
		return StateCouldNotCheck, "sibling checkout for " + entry.Repo + " not found at " + siblingRoot
	}
	siblingStreams, _, err := loadStreams(siblingRoot)
	if err != nil {
		return StateCouldNotCheck, "sibling checkout for " + entry.Repo + " at " + siblingRoot + " could not be read: " + err.Error()
	}
	state, why := resolveInRepoBriefRef(target, siblingStreams)
	if state == StateCouldNotCheck {
		return StateCouldNotCheck, "in the sibling checkout for " + entry.Repo + ": " + why
	}
	return state, why
}
