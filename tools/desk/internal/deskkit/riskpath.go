package deskkit

import (
	"sort"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/topology"
)

// Risk-path classification: the shared computation behind the desk's ready gate (the
// security-review precondition on a flip) and the board's SECURITY-REVIEW-REQUIRED
// signal. It lives HERE in deskkit so the two tools share ONE path-trigger computation
// rather than each carrying its own copy that could drift.
//
// A PR is risk-classed (needs a security-review verdict before it can flip ready) when
// EITHER its repo is risk-classed on visibility alone (a PUBLIC repo always is — see
// VisibilityRiskClassed) OR any changed file touches one of the compiled-in path
// triggers below. This is the deterministic, positively-verifiable signal (the
// changed-file list comes from a GET-only API call); a fuzzy frontmatter/label signal
// ("gate: human OR any risk: yes") is a best-effort secondary that deskpost
// deliberately does NOT gate a flip on — a fuzzy signal would fail open in the wrong
// direction (it could WAIVE the gate on a mislabeled PR). Path triggers only tighten.
//
// Repo-aware, not repo-blind. A single global trigger list applied to every repo
// mis-serves a fleet whose repos have different layouts: a path that names a security
// surface in one repo may not exist in another, so a public repo could end up with
// strictly LESS mandatory scrutiny than a private one. Classification is therefore
// repo-aware, and public repos are risk-classed unconditionally.
//
// EVERY change here may only ever WIDEN what is risk-classed. That rule SURVIVES
// parameterisation rather than being traded away by it: adopter configuration is
// ADDITIVE-ONLY. ASSAY_RISK_PATH_TRIGGERS_EXTRA unions adopter paths ON TOP of the
// compiled sets, and there is no value syntax that removes a compiled entry — so a
// configured value can only move classification in the safe direction (more scrutiny).
// The variable is deliberately NOT named ASSAY_RISK_PATH_TRIGGERS: a name reading as
// "the triggers" invites the replacement implementation this design forbids, and a
// narrowing knob on a security gate is a waiver waiting to be set.
//
// The compiled base is a GENERIC, illustrative default. The real security surfaces of
// any particular deployment are its own — they are supplied by the adopter through
// ASSAY_RISK_PATH_TRIGGERS_EXTRA rather than baked into this shared tree, so no one
// deployment's layout travels with the tool. An adopter that ships this gate MUST set
// that variable to its real trigger paths (see docs), or the gate classes only on the
// generic default and on visibility.
//
// The circularity, stated and answered. Narrowing the trigger list is a REVIEWED code
// change; naive parameterisation would move it to a silent repo-settings edit, trading
// reviewability for configurability. Under additive-only semantics the only thing that
// leaves the reviewed-diff path is WIDENING — the safe direction, and echoed on every
// run (the effective-config echo). Narrowing the compiled set still requires a source
// edit, which is itself risk-classed (this package is in repoRiskPathTriggers below)
// and ships through release + pin bump under the review loop.
//
// An UNSET ASSAY_RISK_PATH_TRIGGERS_EXTRA is NOT an error and NOT a refusal: the
// compiled set is a complete configuration on its own. That is the one place in the
// config surface where "unset" does not refuse, and it is sound precisely because unset
// means compiled-set-only, never classify-nothing.

// baseRiskPathTriggers apply to EVERY repo. They are GENERIC, illustrative security
// surfaces — the kind of path whose change should not be able to flip a PR ready on a
// correctness review alone — and they name no particular deployment's layout. An
// adopter layers its own real trigger paths ON TOP through ASSAY_RISK_PATH_TRIGGERS_EXTRA
// (additive-only); this compiled default is deliberately minimal and house-agnostic.
//
//	secrets/
//	.github/workflows/
//	k8s/*/rbac.yaml
var baseRiskPathTriggers = []string{
	"secrets/",
	".github/workflows/",
	"k8s/*/rbac.yaml",
}

// repoRiskPathTriggersFor returns the ADDITIONAL triggers for one repo, layered on top
// of baseRiskPathTriggers. A repo the topology says nothing about gets the base list
// only. An adopter registers its own repos' triggers through
// ASSAY_RISK_PATH_TRIGGERS_EXTRA rather than in the topology file; what the topology
// states is one illustrative example of the per-repo mechanism plus this toolkit's own
// gate code, which travels with the code it protects.
//
// THE PER-REPO SET IS DECLARED ONCE, in `topology.yaml` (repos[].risk_path_triggers),
// with the rationale for each repo's surfaces in that file's `note:`. ground-truth/04
// retired the hand table that used to sit at this spot (#276); the values, their
// rationale and this gate's reading of them can no longer disagree without
// TestTopologyDriftRegistry going red and naming this site.
//
// Reading a per-repo VALUE is not the same as reading a per-repo POLICY: nothing here
// can narrow the base list, and a repo absent from the topology loses no protection —
// it simply gains none beyond the universal base.
func repoRiskPathTriggersFor(repo string) []string {
	r, ok := topology.Compiled().Repo(repo)
	if !ok {
		return nil
	}
	return r.RiskPathTriggers
}

// RiskPathTriggers returns the UNIVERSAL (base) risk path-trigger patterns — the
// compiled set that applies to every repo, PLUS any adopter additions from
// ASSAY_RISK_PATH_TRIGGERS_EXTRA. Use RiskPathTriggersFor to see one repo's full set.
// Nothing narrows the compiled list: there is no removal syntax.
func RiskPathTriggers() []string {
	out := make([]string, 0, len(baseRiskPathTriggers))
	out = append(out, baseRiskPathTriggers...)
	return unionSorted(out, configuredExtraTriggers())
}

// configuredExtraTriggers is the ADOPTER half of C8: additional trigger paths supplied
// through ASSAY_RISK_PATH_TRIGGERS_EXTRA. It can only ever ADD.
func configuredExtraTriggers() []string {
	return EffectiveConfig().RiskExtra
}

// unionSorted merges trigger lists, de-duplicating and sorting. It never removes.
func unionSorted(lists ...[]string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, 8)
	for _, l := range lists {
		for _, p := range l {
			p = strings.TrimSpace(p)
			if p == "" || seen[p] {
				continue
			}
			seen[p] = true
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// RiskPathTriggersFor returns the full sorted trigger set for repo: the universal base
// list, any repo-specific compiled additions, and any adopter additions from
// ASSAY_RISK_PATH_TRIGGERS_EXTRA (for help text, banners, and tests). The three are
// UNIONED — a configured value never replaces a compiled one. A repo outside the allowed
// set gets the base list — but note that such a repo is risk-classed outright by
// RiskPathTriggered, so the list is informational there.
func RiskPathTriggersFor(repo string) []string {
	return unionSorted(baseRiskPathTriggers, repoRiskPathTriggersFor(repo), configuredExtraTriggers())
}

// RiskPathTriggered reports whether a PR on repo is risk-classed — i.e. whether it needs
// a `Security-Review: pass` verdict at head before the desk may flip it ready.
//
// It answers TRUE on every uncertain input (C-10 fail-closed). Enumerated, the only way
// to get FALSE is: a repo that IS in the compiled-in allowed set, AND is compiled in as
// VisibilityPrivate, AND a non-empty changed-file list every entry of which is a
// non-blank path matching none of that repo's triggers. Specifically:
//
//   - repo outside the allowed set        → true (unknown policy, like CIRequired)
//   - repo public, or visibility unstated → true (the public-repo risk rule policy call)
//   - changed-file list empty             → true (we did not see the diff; a PR always
//     changes at least one file, so "no files" means the read told us nothing)
//   - a blank/empty path entry            → true (a malformed listing is not evidence)
//   - any file matches any trigger        → true
//
// The changed-file list must be the COMPLETE list. A caller that could not read it in
// full must not call this with the partial result — it must fail closed itself
// (deskpost's listFiles error path returns exit 6).
//
// Paths are compared as '/'-separated segments; a trailing '/' in a trigger means
// "directory prefix" (matches the trigger and anything beneath it) and a '*' segment
// matches exactly one path segment. A trigger WITHOUT a trailing '/' (e.g.
// k8s/*/rbac.yaml) is an exact file match (same number of segments).
//
// The decision itself now lives in riskClassify (riskclassifier.go): the fail-closed
// preconditions above are the MECHANISM half and run before any classifier, and the
// path half is the patternClassifier behind the RiskClassifier seam. This function
// keeps its exported signature and every call site unchanged.
func RiskPathTriggered(repo string, changedFiles []string) bool {
	classed, _ := riskClassify(defaultRiskClassifier, repo, changedFiles)
	return classed
}

// RiskClassReason returns a short human-readable reason for the classification, in the
// SAME order RiskPathTriggered decides, so a refusal message can say why. It returns
// "not risk-classed" exactly when RiskPathTriggered returns false — the two read the
// same riskClassify call, so the agreement is structural rather than maintained, and is
// covered by a test that walks every case through both.
func RiskClassReason(repo string, changedFiles []string) string {
	_, reason := riskClassify(defaultRiskClassifier, repo, changedFiles)
	return reason
}

// pathTriggerHit is the PATH half of the classification, split out so it can be tested
// on its own — on a public repo RiskPathTriggered short-circuits before ever reaching
// it, which would otherwise make example-k8s's triggers untestable through the front
// door. It fails closed on a blank path entry.
//
// It is a thin reading of patternClassifier.Classify, NOT a second copy of the match
// loop: a test exercising this must exercise the same code the gate dispatches to.
func pathTriggerHit(repo string, changedFiles []string) bool {
	hit, _ := patternClassifier{}.Classify(repo, changedFiles)
	return hit
}

// matchTrigger reports whether path matches trigger pattern. See RiskPathTriggered for
// the matching rules. A '*' pattern segment is a single-segment wildcard.
func matchTrigger(pattern, path string) bool {
	dirPrefix := strings.HasSuffix(pattern, "/")
	pp := strings.Split(strings.Trim(pattern, "/"), "/")
	fp := strings.Split(strings.Trim(path, "/"), "/")

	if dirPrefix {
		// The pattern's segments must be a prefix of the path's segments — the file
		// lives under the trigger directory.
		if len(fp) < len(pp) {
			return false
		}
	} else {
		// Exact file match — same segment count.
		if len(fp) != len(pp) {
			return false
		}
	}
	for i, seg := range pp {
		if seg == "*" { // single-segment wildcard
			continue
		}
		if fp[i] != seg {
			return false
		}
	}
	return true
}
