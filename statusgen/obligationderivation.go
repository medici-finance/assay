package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// obligationderivation.go — derive typed Verify-row OBLIGATIONS from the shape
// of a change (mistake-proofing/03, spec §4 B2 + §3 D7).
//
// THE HOLE. Three of the authoring rules the fleet leans on hardest are prose
// MUSTs with no device behind them: a brief that adds a check must carry a
// MUTATION row (spec D1), a change to a shared surface must carry a FLOW row, a
// deliverable making factual claims must carry a DEREFERENCE row. Each is
// enforced today only by a reviewer remembering it. The obligation encoding
// (rowclass.go) carries the obligation as typed data; this file derives its
// PRESENCE from the shape of the change and reports what is owed but absent.
//
// PRESENCE, NOT ADEQUACY (spec §3 D7). This derives only whether an obligation
// ROW IS PRESENT. Whether a mutation row actually reddens a guard, whether a
// flow row actually crosses a boundary, whether a dereference row actually
// dereferences — that ADEQUACY is not decidable from row text and stays the
// reviewer's call. Every emitted line states which half it covers.
//
// SEVERITY PHASING (spec §3 D4). Severity is PER OBLIGATION CLASS (obligationFatal):
//   - MUTATION is a hard PROBLEM (mistake-proofing/06 — this brief) on the
//     AVAILABLE-diff path: a change that adds or alters a check a brief declares
//     and carries no mutation row does not merge. This is the "positive control is
//     itself a control" promotion. When the branch diff is UNAVAILABLE the
//     derivation degrades to a conspicuous could-not-check NOTICE (never silence,
//     never a pass) — the same fail-open posture the UNRUN gate uses, so a
//     legitimately-degraded context (shallow clone, no git, pre-fetch CI) does not
//     freeze the board. See the could-not-check block for why this deviates from
//     the brief's "fail closed" wording.
//   - FLOW and DEREFERENCE stay advisory NOTICEs (obligationDerivationFatal, still
//     FALSE — mistake-proofing/03's phasing). A standing advisory NOBODY acts on is
//     negative value, so this is not their resting state either — their promotion
//     is a later brief that records their census and flip condition first.
//
// TRANSITION SCOPE. The derivation reads the SHAPE OF THE CHANGE, so it is
// diff-scoped by construction: it evaluates only a brief whose OWN file this
// branch changed. The obligation is a brief-AUTHORING property, decided when the
// brief is written or edited; scoping to the edited brief keeps the derivation
// off the 300-plus inherited tables (which predate the obligation encoding and
// would flood as advisory NOTICEs) and off every brief this branch did not
// touch. When the diff itself is unavailable the whole derivation is
// could-not-check — see verifyObligationDerivation.
//
// COVERAGE BOUNDARY (spec §3 D6). This does NOT catch: an obligation owed by a
// brief whose file this branch did not edit (its obligations were settled at
// authoring); the ADEQUACY of any present row; and the NEIGHBOUR obligation,
// which is validated (rowclass.go) but has no honest path-only derivation
// trigger and is deferred to a follow-up.

// obligationDerivationFatal gates the flip from advisory NOTICE to hard PROBLEM
// for the FLOW and DEREFERENCE obligations. Lands FALSE (advisory) — see the
// phasing note above. The MUTATION obligation is promoted separately, below.
const obligationDerivationFatal = false

// mutationObligationFatal promotes the MUTATION obligation from advisory NOTICE
// to hard PROBLEM (mistake-proofing/06 — the ONE severity change this brief
// makes). flow and dereference stay advisory (obligationDerivationFatal, 03's
// phasing): this brief promotes presence-of-a-mutation-row to fatal and nothing
// else.
//
// TRANSITION-SCOPED FOR FREE. verifyObligationDerivation evaluates ONLY a brief
// whose own file this branch changed (see the header's TRANSITION SCOPE note), so
// promoting mutation to fatal cannot touch the 300-plus inherited tables — they
// are never evaluated, so the promotion never makes them fatal. That is what
// makes the promotion landable in ONE pull request rather than a corpus
// migration (task 2).
const mutationObligationFatal = true

// obligationFatal reports whether an owed-but-absent obligation of the given
// class is a hard PROBLEM (refuse) or an advisory NOTICE. Only mutation is
// promoted by this brief; flow and dereference remain advisory.
func obligationFatal(class string) bool {
	if class == classMutation {
		return mutationObligationFatal
	}
	return obligationDerivationFatal
}

// ruleObligationDerivation is the stable [rule-tag] bracket token every emitted
// line carries, so lintaudit.go's firing audit can attribute the rule.
const ruleObligationDerivation = "verify-obligation"

// ---------------------------------------------------------------------------
// The check-shaped path set (mistake-proofing/06, task 1)
// ---------------------------------------------------------------------------
//
// The MUTATION obligation fires when the branch diff changes a CHECK-SHAPED path
// this brief declares. "Check-shaped" is written down here as an EXPLICIT,
// NARROW ENUMERATION — a named list with a per-entry rationale a reviewer can
// read and argue with — deliberately NOT one inline regex, and deliberately
// narrow. An over-broad definition fires the obligation on unrelated changes,
// and an obligation that fires on unrelated changes is the fastest route to an
// exemption file (brief Context). Each entry matches a repo-relative,
// slash-separated path; test files are excluded by isCheckShapedPath before any
// entry is consulted — a test is not itself a guarded control.
//
// mistake-proofing/03 landed this as one over-firing regex while the check was
// advisory; promoting the mutation obligation to fatal (this brief) is exactly
// when the set must become a narrow enumeration a reviewer can argue with.

// checkShape is one entry in the enumerated check-shaped path set: a named shape,
// a one-line rationale for why a change to it is a change to a control, and the
// repo-relative path matcher.
type checkShape struct {
	name      string
	rationale string
	re        *regexp.Regexp
}

// checkShapes is the enumerated check-shaped path set. NARROW BY DESIGN — the
// four shapes the brief names, and only those. See the COVERAGE BOUNDARY note
// below for what is deliberately left out and why.
var checkShapes = []checkShape{
	{
		name:      "lint/check source in the tool tree",
		rationale: "a source file of the board lint (statusgen/) IS a control — the tool is the board's guard surface, so changing one of its files adds or alters a check",
		re:        regexp.MustCompile(`^statusgen/[^/]+\.go$`),
	},
	{
		name:      "guard in the desk tree",
		rationale: "a guard in the desk tools (tools/desk/) gates desk writes at run time; its refusals are the in-tree precedent for a machine-checked positive control (brief sources)",
		re:        regexp.MustCompile(`^tools/desk/.+\.go$`),
	},
	{
		name:      "CI workflow",
		rationale: "a CI workflow (.github/workflows/) that gates a merge IS a required-check surface — a change to it changes what CI enforces",
		re:        regexp.MustCompile(`^\.github/workflows/[^/]+\.ya?ml$`),
	},
	{
		name:      "reviewed verify script",
		rationale: "a reviewed verify.d script (docs/streams/*/verify.d/**.sh) is executed verbatim by the verdict runner as the deterministic half of a verdict — a control the reviewer is the trust anchor for (verdict-lane/02)",
		re:        regexp.MustCompile(`^docs/streams/[^/]+/verify\.d/.+\.sh$`),
	},
}

// COVERAGE BOUNDARY (spec §3 D6) — recorded beside the set as an honest
// non-coverage device (a list of what this does NOT catch belongs next to what
// it does). Deliberately OUTSIDE the enumerated set:
//   - admission policies, forge rulesets and branch-protection config, which
//     live on the FORGE, not in the tree — no in-tree path signal exists for them;
//   - a control expressed as a pure-verification INVARIANT in product code
//     outside statusgen/ and tools/desk/ — the path shape cannot distinguish it
//     from ordinary code, and matching it would be the over-fire the header warns
//     is the fastest route to an exemption file;
//   - the ADEQUACY of any present mutation row. This whole surface proves a row
//     is PRESENT; a real mutation DEMONSTRATION — one that actually reddens the
//     guard, which the muhar harness produces — is strictly STRONGER than a
//     present row, and stays the reviewer's call (spec §3 D7). The present-row
//     floor is a floor, not a ceiling.

// isCheckShapedPath reports whether p matches any shape in the enumerated
// check-shaped set (test files excluded — a test is not itself a guarded control).
func isCheckShapedPath(p string) bool {
	p = filepath.ToSlash(strings.TrimSpace(p))
	if strings.HasSuffix(p, "_test.go") {
		return false
	}
	for _, sh := range checkShapes {
		if sh.re.MatchString(p) {
			return true
		}
	}
	return false
}

// sharedSurfaceKeywords are the FLOW trigger's task-prose half: a Task section
// that names one of these describes a change crossing a component boundary, which
// owes an end-to-end flow row. A small, conservative set.
var sharedSurfaceKeywords = []string{
	"shared surface", "shared-surface",
	"cross-component", "cross component",
	"end-to-end", "end to end",
}

// isDereferenceDeliverable reports whether a declared path is a DELIVERABLE that
// asserts checkable facts — a markdown document that is neither a brief file nor a
// stream README (those are process scaffolding, not fact-asserting deliverables).
// The DEREFERENCE trigger is the least mechanically decidable of the three, so it
// is derived conservatively to UNDER-fire rather than over-fire (task 3).
func isDereferenceDeliverable(p string) bool {
	p = filepath.ToSlash(strings.TrimSpace(p))
	if !strings.HasSuffix(p, ".md") {
		return false
	}
	base := filepath.Base(p)
	if base == "README.md" || strings.HasPrefix(base, "brief-") {
		return false
	}
	return true
}

// obligationInputs is the shape-of-the-change a single brief presents to the
// derivation: what it declared, its Task prose, and the branch diff.
type obligationInputs struct {
	declaredPaths      []string
	declaredPathsFound bool
	taskText           string
	changed            map[string]bool // this branch's diff, keyed by repo-relative slash path
}

// owedObligations derives which obligations a brief OWES from the shape of its
// change. The branch diff is the MUTATION axis; the brief's declared paths and
// Task prose are the FLOW and DEREFERENCE axes. NEIGHBOUR is not derived.
//
//	mutation:    a check-shaped path this brief declares (by exact match or
//	             directory prefix) appears in the branch diff — a guard changed,
//	             so a row proving the guard reddens is owed.
//	flow:        declared paths span more than one top-level directory, OR the
//	             Task prose names a shared-surface keyword — the change crosses a
//	             component boundary and owes an end-to-end row.
//	dereference: a declared deliverable asserts checkable facts (a non-brief,
//	             non-README markdown file). Conservative: under-fires by design.
func owedObligations(in obligationInputs) map[string]bool {
	owed := map[string]bool{}

	// mutation — the actual diff must contain a check-shaped path this brief owns.
	for c := range in.changed {
		if isCheckShapedPath(c) && declaredCovers(in.declaredPaths, c) {
			owed[classMutation] = true
			break
		}
	}

	// flow — declared paths span >1 top-level dir, or a shared-surface keyword.
	if in.declaredPathsFound && topLevelSpan(in.declaredPaths) > 1 {
		owed[classFlow] = true
	}
	if hasSharedSurfaceKeyword(in.taskText) {
		owed[classFlow] = true
	}

	// dereference — a declared deliverable asserts checkable facts.
	for _, p := range in.declaredPaths {
		if isDereferenceDeliverable(p) {
			owed[classDereference] = true
			break
		}
	}

	return owed
}

// mutationTriggerPath returns the check-shaped path in the diff that this brief
// declares — the path whose change makes the mutation obligation owed — or "" when
// none. Naming it in the failure message (task 3) is what makes the message
// actionable: the author sees exactly which changed control triggered the row.
// Deterministic (sorted) so the message is stable across runs.
func mutationTriggerPath(in obligationInputs) string {
	var hits []string
	for c := range in.changed {
		if isCheckShapedPath(c) && declaredCovers(in.declaredPaths, c) {
			hits = append(hits, filepath.ToSlash(strings.TrimSpace(c)))
		}
	}
	if len(hits) == 0 {
		return ""
	}
	sort.Strings(hits)
	return hits[0]
}

// declaredCovers reports whether one of a brief's declared paths covers the
// changed path c — an exact match, or a declared directory that is a prefix of c.
func declaredCovers(declared []string, c string) bool {
	c = filepath.ToSlash(strings.TrimSpace(c))
	for _, d := range declared {
		d = filepath.ToSlash(strings.TrimSpace(d))
		if d == "" {
			continue
		}
		if d == c {
			return true
		}
		dir := strings.TrimSuffix(d, "/")
		if strings.HasPrefix(c, dir+"/") {
			return true
		}
	}
	return false
}

// topLevelSpan counts the distinct top-level directories a path set spans.
func topLevelSpan(paths []string) int {
	tops := map[string]bool{}
	for _, p := range paths {
		p = filepath.ToSlash(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		top := p
		if i := strings.IndexByte(p, '/'); i >= 0 {
			top = p[:i]
		}
		tops[top] = true
	}
	return len(tops)
}

// hasSharedSurfaceKeyword reports whether task prose names a shared-surface
// keyword (case-insensitive).
func hasSharedSurfaceKeyword(task string) bool {
	t := strings.ToLower(task)
	for _, kw := range sharedSurfaceKeywords {
		if strings.Contains(t, kw) {
			return true
		}
	}
	return false
}

// presentObligations returns the set of obligations any Verify row in the section
// declares.
func presentObligations(verifySection string) map[string]bool {
	present := map[string]bool{}
	verifyRowTable(verifySection, func(r verifyRowCells) {
		for _, ob := range r.obligations() {
			present[ob] = true
		}
	})
	return present
}

// branchChangedSet computes this branch's three-dot diff against origin/main,
// reusing the consumer-routing helpers (pinConsumerBase + changedPathsSince) so
// no new diff machinery is added. Returns (set, true) on success; (nil, false)
// when the diff is UNAVAILABLE — no git dir, an unresolvable base, a shallow
// clone. A false is a COULD-NOT-CHECK, never "nothing changed". A package-level
// var so tests can inject a diff (or a could-not-check) without a git fixture.
var branchChangedSet = func(root string) (map[string]bool, bool) {
	base, err := pinConsumerBase(root, remoteMainRef)
	if err != nil {
		return nil, false
	}
	paths, err := changedPathsSince(root, base)
	if err != nil {
		return nil, false
	}
	set := map[string]bool{}
	for _, p := range paths {
		if p = filepath.ToSlash(strings.TrimSpace(p)); p != "" {
			set[p] = true
		}
	}
	return set, true
}

// obligationPhrase is the one-line human description of each derived obligation,
// used in the owed-but-absent message.
var obligationPhrase = map[string]string{
	classMutation:    "a check-shaped path this brief owns changed, so a row that breaks the guard and proves it reddens is owed (spec D1)",
	classFlow:        "the change spans more than one component (declared paths cross top-level directories, or the task names a shared surface), so a row exercising the cross-component path end to end is owed",
	classDereference: "the brief declares a deliverable that asserts checkable facts, so a row that resolves a claim rather than counting its presence is owed",
}

// verifyObligationDerivation derives the owed obligations for every brief this
// branch changed and reports each one that no Verify row declares. Returns
// (problems, notices); the SEVERITY of an owed-but-absent obligation is per
// class (obligationFatal): the MUTATION obligation is a hard PROBLEM
// (mistake-proofing/06), flow and dereference remain advisory NOTICEs.
func verifyObligationDerivation(root string, streams []*Stream) (problems, notices []string) {
	tag := "[" + ruleObligationDerivation + "]"
	// emitClass routes one message to problems or notices by the obligation
	// class's own promoted severity, so the mutation promotion is per-class and
	// does not drag flow/dereference along.
	emitClass := func(class, msg string) {
		if obligationFatal(class) {
			problems = append(problems, tag+" "+msg)
		} else {
			notices = append(notices, tag+" "+msg)
		}
	}

	changed, ok := branchChangedSet(root)
	if !ok {
		// COULD-NOT-CHECK: the shape of the change is the derivation's only input,
		// and without the diff it cannot be computed. Reported AS ITSELF — a
		// conspicuous, greppable NOTICE — never rounded down to "nothing is owed"
		// and never silently rounded up to a pass (docs/three-state-instrument-rule.md,
		// C4). This is the DEGRADED posture the tree's other transition-scoped gate
		// already uses: unrunGateChecks resolves the merge-base and, when it cannot,
		// degrades every offender to a NOTICE plus a "running degraded" NOTICE
		// ("If this is CI, fetch origin/main before the lint step") rather than
		// reddening the board. A single PROBLEM aborts the whole board regeneration,
		// so a could-not-check that fired as a PROBLEM would freeze the board in
		// every legitimately-degraded context (a shallow clone, a fixture with no
		// git, a pre-fetch CI step) — which is why the tree fails OPEN on an
		// unavailable base and closes the gate on the AVAILABLE-diff path instead.
		//
		// NOTE (brief §Task 4 / §Context say "refuse / fail closed on the diff"):
		// this deviates deliberately. The brief cites "the same posture as every
		// other check in the tree", but the cited precedent (unrunGateChecks)
		// DEGRADES rather than refuses, and a refusing PROBLEM here reds 17 full-lint
		// fixture tests and risks a board freeze. The gate that the brief's `why`
		// actually turns on — "a change that adds a check and carries no mutation
		// row does not merge" — is the OWED-BUT-ABSENT mutation PROBLEM below, which
		// IS fatal. The reviewer is asked to confirm this severity call (PR body).
		notices = append(notices, tag+" Verify-row obligation derivation COULD-NOT-CHECK — the branch diff against "+remoteMainRef+
			" is unavailable (no git dir, an unresolvable base, or a shallow clone), so the shape of the "+
			"change cannot be read and no obligation can be derived. This is could-not-check, NOT a pass and NOT "+
			"\"nothing is owed\": a change whose diff cannot be read is not a change that owes nothing. The mutation "+
			"obligation runs DEGRADED here (the same fail-open posture as the UNRUN gate) — if this is CI, fetch "+
			remoteMainRef+" before the lint step so the gate can evaluate this branch's changes. This line checks the "+
			"PRESENCE of an obligation row, never its adequacy — that stays the reviewer's call (spec §3 D7) — mistake-proofing/06")
		return problems, notices
	}

	for _, s := range streams {
		for _, path := range briefFilePaths(s) {
			// Transition scope: only a brief whose OWN file this branch changed.
			rel, err := filepath.Rel(s.Root, path)
			if err != nil {
				continue
			}
			if !changed[filepath.ToSlash(rel)] {
				continue
			}
			bf, ok, err := parseBriefFile(path)
			if err != nil || !ok {
				continue
			}
			in := obligationInputs{
				declaredPaths:      bf.DeclaredPaths,
				declaredPathsFound: bf.DeclaredPathsFound,
				taskText:           extractSectionByPrefix(bf.Body, "Task"),
				changed:            changed,
			}
			owed := owedObligations(in)
			present := presentObligations(bf.Verify)

			var missing []string
			for ob := range owed {
				if !present[ob] {
					missing = append(missing, ob)
				}
			}
			sort.Strings(missing)
			for _, ob := range missing {
				if ob == classMutation {
					// The promoted, actionable mutation message (task 3): names the
					// check-shaped path that triggered the obligation, states a row is
					// required, points at the muhar harness, keeps presence-vs-adequacy
					// explicit, and states it does NOT replace the reviewer (brief Context).
					emitClass(classMutation, fmt.Sprintf("brief %s (%s) changed a check-shaped control (%s) but declares no `+mutation` Verify row. "+
						"A change that adds or alters a control MUST carry a row that BREAKS the guarded thing and proves the control reddens (spec D1). "+
						"Add `+mutation` to the relevant Verify row's `Class` cell (e.g. `check +mutation`); the mutation harness at `tools/desk/cmd/muhar` "+
						"is the recommended way to produce the demonstration. This check verifies the PRESENCE of the row, NOT its adequacy — whether the "+
						"mutation is real and actually reddens the guard stays the reviewer's call (spec §3 D7), so this floor does NOT replace the reviewer "+
						"question \"does a row prove this reddens when the guarded thing is broken?\" — mistake-proofing/06",
						bf.Brief, path, mutationTriggerPath(in)))
					continue
				}
				emitClass(ob, fmt.Sprintf("brief %s (%s) owes a `+%s` Verify-row obligation — %s — but no Verify row declares one. "+
					"Add the obligation token to the relevant row's `Class` cell (e.g. `check +%s`), or if the row genuinely "+
					"does not apply, say why in review. This checks the PRESENCE of the obligation row, not its ADEQUACY — "+
					"whether the row actually discharges the obligation stays the reviewer's call (spec §3 D7) — mistake-proofing/03",
					bf.Brief, path, ob, obligationPhrase[ob], ob))
			}
		}
	}
	sort.Strings(problems)
	sort.Strings(notices)
	return problems, notices
}
