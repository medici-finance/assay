package main

import (
	"sort"
	"strings"
)

// riskpathtriggers.go — the POLICY half of the desk's risk-path classifier,
// DUPLICATED into the lint so a brief's declared paths can be cross-read against
// the same classification a pull request is (mistake-proofing/01, spec §4 B3).
//
// WHY A DUPLICATE AND NOT AN IMPORT. The classifier lives in
// tools/desk/internal/deskkit/{riskpath,riskclassifier}.go. That is a SEPARATE Go
// module (github.com/medici-finance/assay/tools/desk) and the classifier's package
// is `internal/`, so there is no import path in either direction. The established
// pattern for exactly this shape is duplicate-the-reader and bind-the-two-with-a-
// shared-test-vector — statusgen/rosterconfig.go already duplicates deskkit's
// config reader and TestRosterCouplingVectors / TestScanIssuesTrustGateEnforced
// bind them over statusgen/testdata/roster_coupling.json. This file copies that
// pattern: risk_trigger_coupling.json is read by a TriggerCoupling test in BOTH
// modules, so a drift in either copy reddens both.
//
// POLICY HALF ONLY. deskkit's classifier has two halves with OPPOSITE semantics.
// The MECHANISM half (riskClassify's early returns) answers TRUE on every
// uncertain input — an unknown repo, a public or visibility-unstated repo, an
// empty changed-file list, a blank path entry — because on those inputs there is
// nothing to classify and the fail-closed answer is already "classed". That half
// is CORRECT for a pull request but WRONG here: a brief is not a pull request on a
// known repo, so feeding a brief's declared paths through the top-level accessor
// (RiskPathTriggered) would class EVERY brief in the corpus — most streams sit in
// a public repo — and make the cross-read useless in one commit. This file copies
// the POLICY half only: does a path match the trigger set. It has no notion of
// repo visibility and never fails a brief closed on an uncertain input; the
// cross-read's own could-not-check handles an unreadable declared-paths line.
//
// ADDITIVE-ONLY, like the original. The trigger set is a union — a compiled base
// list that applies to every repo, per-repo additions read from topology.yaml,
// and an adopter environment variable (ASSAY_RISK_PATH_TRIGGERS_EXTRA). There is
// no removal syntax, so the set can only ever WIDEN, which is the safe direction
// for a gate.
//
// COVERAGE BOUNDARY (spec §3 D6 — kept beside the code, not in a separate doc).
// A cross-read built on this set does NOT catch: a risky surface that matches no
// trigger (the base list is generic and adopter-agnostic); a brief whose declared
// paths are wrong or absent (that is a could-not-check, handled by the caller);
// and the adopter case where only the generic base list is configured — an
// adopter that has set no per-repo topology trigger and no
// ASSAY_RISK_PATH_TRIGGERS_EXTRA gets the generic base only, so a clean cross-read
// there is not a claim to have classified that adopter's real security surfaces.

// lintBaseRiskPathTriggers is a byte-for-byte DUPLICATE of deskkit's
// baseRiskPathTriggers (tools/desk/internal/deskkit/riskpath.go). They are the
// GENERIC, illustrative security surfaces that apply to every repo. A drift
// between this copy and deskkit's is caught by the shared TriggerCoupling vector.
var lintBaseRiskPathTriggers = []string{
	"secrets/",
	".github/workflows/",
	"k8s/*/rbac.yaml",
}

// lintRepoRiskPathTriggersFor returns the ADDITIONAL triggers topology.yaml
// declares for one repo (repos[].risk_path_triggers), layered on top of the base
// list. A repo the topology says nothing about gets the base list only — it loses
// no protection, it simply gains none beyond the universal base. This reads the
// statusgen-side topology DERIVATION (topologyvalues.go), bound to topology.yaml
// by TestTopologyValuesMatchSource exactly as the desk module's reading is bound
// by TestTopologyDriftRegistry.
func lintRepoRiskPathTriggersFor(repo string) []string {
	return topologyRiskPathTriggersByRepo[repo]
}

// lintConfiguredExtraTriggers is the ADOPTER half: additional trigger paths
// supplied through ASSAY_RISK_PATH_TRIGGERS_EXTRA, parsed into
// scanConfig.RiskExtra by the roster reader. It can only ever ADD.
func lintConfiguredExtraTriggers() []string {
	return scanEffectiveConfig().RiskExtra
}

// lintRiskPathTriggersFor returns the full sorted trigger set for repo: the
// universal base list, any repo-specific topology additions, and any adopter
// additions from ASSAY_RISK_PATH_TRIGGERS_EXTRA. The three are UNIONED — a
// configured value never replaces a compiled one. Mirrors deskkit's
// RiskPathTriggersFor.
func lintRiskPathTriggersFor(repo string) []string {
	return lintUnionSortedTriggers(lintBaseRiskPathTriggers, lintRepoRiskPathTriggersFor(repo), lintConfiguredExtraTriggers())
}

// lintUnionSortedTriggers merges trigger lists, de-duplicating and sorting. It
// never removes. Mirrors deskkit's unionSorted.
func lintUnionSortedTriggers(lists ...[]string) []string {
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

// lintMatchTrigger reports whether path matches trigger pattern. It is a
// byte-for-byte DUPLICATE of deskkit's matchTrigger: paths are compared as
// '/'-separated segments; a trailing '/' in a trigger means "directory prefix"
// (matches the trigger and anything beneath it); a '*' segment matches exactly
// one path segment; a trigger WITHOUT a trailing '/' is an exact file match (same
// number of segments). A drift from deskkit's rule is caught by the shared
// TriggerCoupling vector.
func lintMatchTrigger(pattern, path string) bool {
	dirPrefix := strings.HasSuffix(pattern, "/")
	pp := strings.Split(strings.Trim(pattern, "/"), "/")
	fp := strings.Split(strings.Trim(path, "/"), "/")

	if dirPrefix {
		if len(fp) < len(pp) {
			return false
		}
	} else {
		if len(fp) != len(pp) {
			return false
		}
	}
	for i, seg := range pp {
		if seg == "*" {
			continue
		}
		if fp[i] != seg {
			return false
		}
	}
	return true
}

// lintRiskTriggerHit is the POLICY-half decision for one path on one repo: does
// path match any trigger in repo's set. It returns the FIRST matching trigger so
// a finding can name it. A blank path never matches (there is nothing to compare)
// — the cross-read screens declared paths before calling this, and unlike the
// desk gate this is not a fail-closed surface: an unreadable declared-paths line
// is the caller's could-not-check, not a silent classification here.
func lintRiskTriggerHit(repo, path string) (trigger string, hit bool) {
	if strings.TrimSpace(path) == "" {
		return "", false
	}
	for _, pat := range lintRiskPathTriggersFor(repo) {
		if lintMatchTrigger(pat, path) {
			return pat, true
		}
	}
	return "", false
}
