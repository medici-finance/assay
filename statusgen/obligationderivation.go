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
// SEVERITY PHASING (spec §3 D4 — task 4). obligationDerivationFatal gates the
// class. It lands FALSE — every owed-but-absent obligation and the
// could-not-check are advisory NOTICEs — exactly as the prior lints on this
// surface (risk×files cross-read, identifier dereference) were phased. A
// standing NOTICE nobody acts on is negative value, so this is NOT the resting
// state: the landing pull-request records the census and the flip condition. The
// MUTATION obligation's promotion to fatal is mistake-proofing/06 — NOT here.
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

// obligationDerivationFatal gates the flip from advisory NOTICE to hard PROBLEM.
// Lands FALSE (advisory) — see the phasing note above and mistake-proofing/06.
const obligationDerivationFatal = false

// ruleObligationDerivation is the stable [rule-tag] bracket token every emitted
// line carries, so lintaudit.go's firing audit can attribute the rule.
const ruleObligationDerivation = "verify-obligation"

// checkShapedPathRe matches a repo-relative CHECK-HOME source path in this repo —
// where guards/checks live (the stream README names the lint `statusgen/` and the
// desk tools `tools/desk/` as the canonical homes). A diff that adds or changes
// such a file is the MUTATION trigger.
//
// This deliberately OVER-fires: it matches any check-home source change, not only
// one that ADDS a control, because a path-only signal cannot tell an added guard
// from a refactor. That is acceptable while the check is advisory — the census in
// this brief's pull-request measures the over-fire, and mistake-proofing/06 (the
// promotion to fatal) must narrow this to "adds a control" before it can bite.
var checkShapedPathRe = regexp.MustCompile(`^(statusgen/[^/]+|tools/desk/.+)\.go$`)

// isCheckShapedPath reports whether p is a check-home source file (test files
// excluded — a test is not itself a guarded control).
func isCheckShapedPath(p string) bool {
	p = filepath.ToSlash(strings.TrimSpace(p))
	if strings.HasSuffix(p, "_test.go") {
		return false
	}
	return checkShapedPathRe.MatchString(p)
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
// (problems, notices); which one an owed-but-absent obligation lands in is gated
// by obligationDerivationFatal (advisory today).
func verifyObligationDerivation(root string, streams []*Stream) (problems, notices []string) {
	tag := "[" + ruleObligationDerivation + "]"
	emit := func(msg string) {
		if obligationDerivationFatal {
			problems = append(problems, tag+" "+msg)
		} else {
			notices = append(notices, tag+" "+msg)
		}
	}

	changed, ok := branchChangedSet(root)
	if !ok {
		// COULD-NOT-CHECK: the shape of the change is the derivation's only input,
		// and without the diff it cannot be computed. Reported AS ITSELF, never
		// rounded down to "nothing is owed" (docs/three-state-instrument-rule.md).
		emit("Verify-row obligation derivation COULD-NOT-CHECK — the branch diff against " + remoteMainRef +
			" is unavailable (no git dir, an unresolvable base, or a shallow clone), so the shape of the " +
			"change cannot be read and no obligation can be derived. This is could-not-check, not a pass: a " +
			"change whose diff cannot be read is not a change that owes nothing. This line checks the PRESENCE " +
			"of an obligation row, never its adequacy — that stays the reviewer's call (spec §3 D7) — mistake-proofing/03")
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
			owed := owedObligations(obligationInputs{
				declaredPaths:      bf.DeclaredPaths,
				declaredPathsFound: bf.DeclaredPathsFound,
				taskText:           extractSectionByPrefix(bf.Body, "Task"),
				changed:            changed,
			})
			present := presentObligations(bf.Verify)

			var missing []string
			for ob := range owed {
				if !present[ob] {
					missing = append(missing, ob)
				}
			}
			sort.Strings(missing)
			for _, ob := range missing {
				emit(fmt.Sprintf("brief %s (%s) owes a `+%s` Verify-row obligation — %s — but no Verify row declares one. "+
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
