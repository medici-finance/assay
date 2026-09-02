package attribution

import "sort"

// stage.go — the stage classifier (task item 3). A DETERMINISTIC, rule-based
// classifier applies the spec §6 stage tests over an assembled Dossier and returns
// one of spec / brief / implementation / untraceable, PLUS the `review-escape`
// overlay recorded for EVERY defect. The deterministic classifier is the default
// AND the fallback: it produces a defensible stage for every dossier on its own; an
// optional model-assisted hook only REFINES the call and records the dossier hash it
// decided from either way (spec §10 honest-claims — judgment-classified, never
// measured).

// Stage is the M3 stage-attribution outcome (spec §6). The string values are the
// on-disk contract the ledger and brief-12 key on.
type Stage string

const (
	// StageSpec — the change faithfully implements its brief AND the brief
	// faithfully reflects the spec as it stood: the requirement was wrong.
	StageSpec Stage = "spec"
	// StageBrief — the implementation matches the brief but the plan did not cover
	// the defect surface (or did not faithfully derive from the spec): the plan was
	// wrong.
	StageBrief Stage = "brief"
	// StageImplementation — the plan covered the defect surface and the change
	// violates it: the work was wrong.
	StageImplementation Stage = "implementation"
	// StageUntraceable — the provenance chain is broken (pre-provenance history, no
	// linkage, could-not-trace). Reported AS SUCH, never binned elsewhere (spec
	// §3.2).
	StageUntraceable Stage = "untraceable"
)

// ReviewEscape is the orthogonal overlay recorded for EVERY defect (spec §6): the
// review lanes that APPROVED the inducing PR at head. It is a frozen seam brief-12
// consumes (gate-yield accounting attributes each escape to these lanes), so its
// shape is stable. Lanes is sorted and de-duplicated.
type ReviewEscape struct {
	Lanes []string `json:"lanes"`
}

// StageCall is a recorded stage judgment (spec §10): the Stage, the review-escape
// overlay, the exact DossierHash it decided from (so a spot-audit re-derives the
// dossier and checks the call against it), a human-legible Rationale, and whether a
// model refined the deterministic base. Every field travels with the call — a bare
// stage with no dossier hash is never emitted.
type StageCall struct {
	Stage         Stage        `json:"stage"`
	ReviewEscape  ReviewEscape `json:"review_escape"`
	DossierHash   string       `json:"dossier_hash"`
	Rationale     string       `json:"rationale"`
	ModelAssisted bool         `json:"model_assisted"`
}

// Classify applies the deterministic §6 stage tests to a dossier and returns the
// stage call. It is a PURE function of the dossier — no clock, no I/O, no map order
// — so the same dossier always yields the same call, and the call is auditable
// against the fixed dossier it names.
//
// The rule order encodes the §6 tests directly:
//
//  1. A broken/absent provenance chain, or a non-traceable trace, is `untraceable`
//     up front — never a stage guess (spec §3.2).
//  2. defectCovered := DefectSurface ⊆ Brief.Coverage. If the plan did NOT cover the
//     defect surface, the plan was wrong: `brief` (plan-gap).
//  3. Otherwise the plan covered it. changeFaithful := InducingDiff.Surface ⊆
//     Brief.Coverage. If the change went OUTSIDE the plan it violated a plan that
//     covered the surface: `implementation` (the work was wrong).
//  4. Otherwise the change stayed faithful to a plan that covered the surface, yet
//     the defect exists. If the brief faithfully reflected the spec, the requirement
//     itself was wrong: `spec`. If it did not (or the signal is unknown), the plan
//     mis-derived from the spec: `brief` — the conservative call, never an
//     unevidenced `spec`.
func Classify(d Dossier) StageCall {
	call := StageCall{
		ReviewEscape: reviewEscape(d),
		DossierHash:  d.Hash,
	}

	if !d.Trace.Traceable() || d.Chain.Broken() {
		call.Stage = StageUntraceable
		call.Rationale = untraceableRationale(d)
		return call
	}

	defectCovered := subset(d.DefectSurface, d.Brief.Coverage)
	if !defectCovered {
		call.Stage = StageBrief
		call.Rationale = "plan-gap: the defect surface is not within the brief's declared coverage — the plan did not cover it"
		return call
	}

	changeFaithful := subset(d.InducingDiff.Surface, d.Brief.Coverage)
	if !changeFaithful {
		call.Stage = StageImplementation
		call.Rationale = "plan-violation: the brief covered the defect surface but the inducing change touched surface outside the brief's coverage — the work violated the plan"
		return call
	}

	if d.Brief.ReflectsSpec == SignalTrue {
		call.Stage = StageSpec
		call.Rationale = "requirement-fault: the change faithfully implements a brief that faithfully reflects the spec — the requirement itself was wrong"
		return call
	}
	call.Stage = StageBrief
	call.Rationale = "plan-derivation-fault: the change is faithful to the brief and the brief covers the surface, but the brief does not faithfully reflect the spec (or the signal is unrecorded) — the plan was wrong; not attributed to spec without positive evidence"
	return call
}

// StageRefiner is the OPTIONAL model-assisted hook (task item 3). It receives the
// dossier and the deterministic base call and MAY refine the stage; it records
// nothing itself — ClassifyWith stamps the dossier hash and the model-assisted flag.
// A refiner is never trusted to invent an attribution for a broken chain: see
// ClassifyWith.
type StageRefiner interface {
	Refine(d Dossier, base StageCall) (Stage, string, error)
}

// ClassifyWith runs the deterministic classifier and then, when a refiner is
// supplied, lets it refine the stage — EXCEPT for an untraceable base, which stands:
// a broken chain yields `untraceable`, never a model guess (spec §3.2, brief task
// item 3). The returned call always records the dossier hash it decided from; when
// the refiner changed the stage the call is marked model-assisted and the base stage
// is preserved in the rationale for the spot-audit trail. A refiner error is
// non-fatal: the deterministic base is returned unchanged (the classifier is the
// fallback), with the error noted in the rationale.
func ClassifyWith(d Dossier, refiner StageRefiner) StageCall {
	base := Classify(d)
	if refiner == nil || base.Stage == StageUntraceable {
		return base
	}
	stage, rationale, err := refiner.Refine(d, base)
	if err != nil {
		base.Rationale = base.Rationale + " [model refinement errored, deterministic base kept: " + err.Error() + "]"
		return base
	}
	if stage == StageUntraceable {
		// A refiner may not manufacture untraceability for a chain the deterministic
		// classifier resolved; ignore that and keep the base.
		return base
	}
	if stage == base.Stage {
		return base
	}
	base.ModelAssisted = true
	base.Rationale = "model-refined to " + string(stage) + " (deterministic base: " + string(base.Stage) + "): " + rationale
	base.Stage = stage
	return base
}

// reviewEscape builds the overlay: the sorted, de-duplicated set of lanes that
// approved the inducing PR at head. A lane with a non-approving verdict is NOT an
// escape lane (it did not let the defect through). Recorded for every defect — an
// empty Lanes list (nothing approved, or no verdicts recorded) is honest, not a gap.
func reviewEscape(d Dossier) ReviewEscape {
	seen := map[string]struct{}{}
	var lanes []string
	for _, r := range d.Reviews {
		if !r.Approved || r.Lane == "" {
			continue
		}
		if _, dup := seen[r.Lane]; dup {
			continue
		}
		seen[r.Lane] = struct{}{}
		lanes = append(lanes, r.Lane)
	}
	sort.Strings(lanes)
	if lanes == nil {
		lanes = []string{}
	}
	return ReviewEscape{Lanes: lanes}
}

func untraceableRationale(d Dossier) string {
	if !d.Trace.Traceable() {
		return "chain broken: brief-07 trace is " + d.Trace.TraceState + " — no reachable inducing change to attribute (never binned into a stage)"
	}
	return "chain broken: the provenance chain does not resolve to a requirement rung (issue/brief) — not attributable to a stage (never binned)"
}

// subset reports whether every token in sub is present in super. An EMPTY sub is
// vacuously a subset (nothing to cover); this is deliberate — a defect with no
// recorded surface cannot drive a plan-gap claim on its own, so it falls through to
// the later tests rather than reddening as `brief`. Callers that need a surface
// present must record one.
func subset(sub, super []string) bool {
	if len(sub) == 0 {
		return true
	}
	set := make(map[string]struct{}, len(super))
	for _, s := range super {
		set[s] = struct{}{}
	}
	for _, s := range sub {
		if _, ok := set[s]; !ok {
			return false
		}
	}
	return true
}
