package evaluation

import (
	"regexp"
	"strings"
)

// eligibility.go — the canonical eligibility evaluator (graph-execution/01,
// extracted by statusgen/15): `gates:`, `feathers:` and `depends:` edges become
// a dispatch verdict with an explainable reason. See
// docs/dependency-graph-design.md §3.6 for the semantics executed here.
//
// The evaluator is a pure function of the observations it is handed. It never
// reads the filesystem, the clock, the environment or the forge: a registry
// and any sibling checkout are supplied by the caller, and a forge-backed ref
// (`#<NNN>`, `<alias>#<NNN>`) is ALWAYS could-not-check here, whatever the
// caller can reach. Eligibility is advice for a dispatcher: it grants no
// admission, claim, merge or status change.

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
	// Deliberately excluded from JSON — the wire shape is exactly
	// {ref,type,reason,state} — but carried for callers that must name WHY a
	// hold is a registry or checkout gap and not just THAT it is.
	Why string `json:"-"`
}

// Eligibility is the evaluator's full verdict for one brief.
type Eligibility struct {
	ID      string             `json:"id"` // "<stream>/<NN>"
	Verdict EligibilityVerdict `json:"verdict"`
	Holds   []EdgeVerdict      `json:"holds,omitempty"`
	Notices []EdgeVerdict      `json:"notices,omitempty"`
}

// Edge is one declared gates:/feathers: entry.
type Edge struct {
	Ref    string
	Type   string
	Reason string
}

// Brief is the eligibility-relevant observation of one brief.
type Brief struct {
	Num      string
	Status   string
	Depends  []string
	Gates    []Edge
	Feathers []Edge
	// HumanGatePassRecorded is the observer's statement that this brief is
	// gate:human, sits at `implemented`, and carries a recorded passing verify
	// verdict. It satisfies a depends: edge, never a gates:/feathers: edge. The
	// evidence reading that produces it stays with the observer.
	HumanGatePassRecorded bool
}

// Stream is one stream's briefs.
type Stream struct {
	Name   string
	Briefs []Brief
}

// RegistryAlias is one graph-repos alias.
type RegistryAlias struct {
	Repo        string
	Unpublished bool
}

// Registry is the observed docs/streams/graph-repos.yaml alias table. A nil
// *Registry means "no registry observed": every cross-repo edge is then
// could-not-check.
type Registry struct {
	Aliases map[string]RegistryAlias
}

// Sibling is the caller's observation of the checkout of a published alias's
// repository next to the evaluated tree.
type Sibling struct {
	// Found is false when no sibling checkout exists.
	Found bool
	// Where names the checkout for a reason string (a path the caller chose to
	// display); it is never opened here.
	Where string
	// ReadError, when set, says the checkout exists but could not be read.
	ReadError string
	// Streams are the sibling checkout's streams.
	Streams []Stream
}

// SiblingLookup returns the observation for a published repo (as the registry
// names it, e.g. "owner/name"). It is called lazily, once per distinct
// resolution that needs it. A nil lookup observes no sibling at all.
type SiblingLookup func(repo string) Sibling

// EligibilityInput is everything EvaluateEligibility reads.
type EligibilityInput struct {
	Streams  []Stream
	Registry *Registry
	Siblings SiblingLookup
}

// Reference-grammar (§3.3) recognizers. The grammar is hierarchical and
// elision-based; each form below is one legal shape a ref may take. They are
// the single definition: the statusgen lint validates refs with the same
// expressions.
var (
	// <stream>/<NN>  — in-repo brief.
	RefInRepoBriefRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*/[0-9]+[a-z]?$`)
	// <alias>:<stream>/<NN>  — cross-repo brief.
	RefCrossRepoBriefRe = regexp.MustCompile(`^[a-z0-9]+:[a-z0-9][a-z0-9-]*/[0-9]+[a-z]?$`)
	// <cell>:<alias>:<stream>/<NN>  — cross-cell brief (reserved form).
	RefCrossCellBriefRe = regexp.MustCompile(`^[a-z0-9]+:[a-z0-9]+:[a-z0-9][a-z0-9-]*/[0-9]+[a-z]?$`)
	// #<issue>  — in-repo issue.
	RefInRepoIssueRe = regexp.MustCompile(`^#[0-9]+$`)
	// <alias>#<issue>  — cross-repo issue.
	RefCrossRepoIssueRe = regexp.MustCompile(`^[a-z0-9]+#[0-9]+$`)
)

// EvaluateEligibility computes the eligibility verdict of every brief across
// in.Streams (not just todo ones — a done brief's verdict is inert but still
// reported, so a caller need not special-case status before indexing the map).
//
// semantic: S-eligibility
//
// Rules (brief-01 Task item 1), in order:
//   - a gates: entry whose target is not done/verified → unsatisfied → HELD
//   - a gates: entry that cannot be resolved (unpublished alias, absent
//     sibling checkout, forge-backed target) → could-not-check → HELD
//   - a feathers: entry that is unsatisfied or could-not-check → NOTICE only
//     (verdict eligible-with-notice), never a hold
//   - an unsatisfied depends: entry is reported as a hold with type
//     build-dep, so one output explains every reason a brief is not offered;
//     a depends: target is satisfied at done/verified, or when it is
//     gate:human at implemented with a recorded pass (TargetSatisfies) — the
//     latter never satisfies a gates: entry
//   - eligible only when holds is empty
func EvaluateEligibility(in EligibilityInput) map[string]Eligibility {
	out := map[string]Eligibility{}
	for _, s := range in.Streams {
		for _, b := range s.Briefs {
			id := s.Name + "/" + b.Num
			e := Eligibility{ID: id}
			for _, dep := range b.Depends {
				if !DependsSatisfied(in.Streams, dep) {
					e.Holds = append(e.Holds, EdgeVerdict{
						Ref:    dep,
						Type:   "build-dep",
						State:  StateUnsatisfied,
						Reason: "depends: target is not done/verified (nor gate:human at implemented with a recorded pass)",
					})
				}
			}
			for _, edge := range b.Gates {
				state, why := resolveGraphEdge(edge.Ref, in)
				if state != StateSatisfied {
					e.Holds = append(e.Holds, EdgeVerdict{
						Ref: edge.Ref, Type: edge.Type, Reason: edge.Reason, State: state, Why: why,
					})
				}
			}
			for _, edge := range b.Feathers {
				state, why := resolveGraphEdge(edge.Ref, in)
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

// DependsSatisfied reports whether the typed dep "<stream>/<NN>" satisfies a
// depends: edge — see TargetSatisfies(b, true). An unresolvable dep returns
// false, gating the dependent brief.
func DependsSatisfied(streams []Stream, dep string) bool {
	parts := strings.SplitN(dep, "/", 2)
	if len(parts) != 2 {
		return false
	}
	for _, s := range streams {
		if s.Name != parts[0] {
			continue
		}
		for _, b := range s.Briefs {
			if b.Num == parts[1] {
				return TargetSatisfies(b, true)
			}
		}
	}
	return false // unresolved dep → not satisfied
}

// TargetSatisfies is the ONE predicate for "does this target brief satisfy an
// edge pointing at it". done/verified satisfies every edge kind. A depends:
// edge (dependsEdge=true) is ALSO satisfied by a gate:human target that sits
// at `implemented` with a recorded passing verify run: the human gate binds
// approval, not implementation, so a dependent may build on a deliverable that
// has already passed verification while it waits for sign-off. A gates: or
// feathers: edge (dependsEdge=false) never takes that branch — an ordering or
// policy gate still waits for verified/done.
func TargetSatisfies(b Brief, dependsEdge bool) bool {
	if b.Status == "done" || b.Status == "verified" {
		return true
	}
	return dependsEdge && b.HumanGatePassRecorded
}

// resolveGraphEdge resolves one gates:/feathers: ref to its state, using the
// §3.3 grammar forms. It never touches the network or a forge.
func resolveGraphEdge(ref string, in EligibilityInput) (EligibilityState, string) {
	switch {
	case RefInRepoBriefRe.MatchString(ref):
		return resolveInRepoBriefRef(ref, in.Streams)
	case RefInRepoIssueRe.MatchString(ref):
		return StateCouldNotCheck, "forge-backed gate " + ref + " — the evaluator reads the tree only, never the forge"
	case RefCrossRepoIssueRe.MatchString(ref):
		return StateCouldNotCheck, "forge-backed cross-repo gate " + ref + " — the evaluator reads the tree only, never the forge"
	case RefCrossRepoBriefRe.MatchString(ref):
		i := strings.IndexByte(ref, ':')
		alias, target := ref[:i], ref[i+1:]
		return resolveCrossRepoBriefRef(alias, target, in)
	case RefCrossCellBriefRe.MatchString(ref):
		return StateCouldNotCheck, "cross-cell reference " + ref + " — no cross-cell resolution exists in this evaluator"
	default:
		return StateCouldNotCheck, "ref " + ref + " does not match a recognized §3.3 reference form"
	}
}

// resolveInRepoBriefRef resolves a `<stream>/<NN>` gates:/feathers: ref
// against the in-hand streams. A ref that cannot be found (unknown stream or
// brief number) is could-not-check, never a silent unsatisfied — the target
// may simply not have been loaded into this run's scope.
//
// It asks targetSatisfies with dependsEdge=false: a gate:human target at
// `implemented` with a recorded pass satisfies a depends: edge but NEVER a
// gates:/feathers: edge, which still waits for verified/done.
func resolveInRepoBriefRef(ref string, streams []Stream) (EligibilityState, string) {
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
				if TargetSatisfies(b, false) {
					return StateSatisfied, ""
				}
				if b.HumanGatePassRecorded {
					return StateUnsatisfied, "brief " + ref + " is gate:human at implemented with a recorded pass — that satisfies depends: edges only, never gates:/feathers:"
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
// "looked and found satisfied". A published alias resolves against the
// sibling checkout the caller observed for it; an absent sibling checkout is
// the same could-not-check class as an unpublished alias.
func resolveCrossRepoBriefRef(alias, target string, in EligibilityInput) (EligibilityState, string) {
	if in.Registry == nil {
		return StateCouldNotCheck, "no docs/streams/graph-repos.yaml registry — alias " + alias + " cannot be resolved"
	}
	entry, ok := in.Registry.Aliases[alias]
	if !ok {
		return StateCouldNotCheck, "alias " + alias + " is not in docs/streams/graph-repos.yaml"
	}
	if entry.Unpublished || entry.Repo == "" {
		return StateCouldNotCheck, "alias " + alias + " is unpublished — its target repo is not resolvable from this tree"
	}
	var sib Sibling
	if in.Siblings != nil {
		sib = in.Siblings(entry.Repo)
	}
	if !sib.Found {
		msg := "sibling checkout for " + entry.Repo + " not found"
		if sib.Where != "" {
			msg += " at " + sib.Where
		}
		return StateCouldNotCheck, msg
	}
	if sib.ReadError != "" {
		return StateCouldNotCheck, "sibling checkout for " + entry.Repo + " at " + sib.Where + " could not be read: " + sib.ReadError
	}
	state, why := resolveInRepoBriefRef(target, sib.Streams)
	if state == StateCouldNotCheck {
		return StateCouldNotCheck, "in the sibling checkout for " + entry.Repo + ": " + why
	}
	return state, why
}
