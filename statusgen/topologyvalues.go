package main

import "strings"

// topologyvalues.go — statusgen's DERIVATION of `topology.yaml`, the ONE declared
// source for org/repo/App/product topology.
//
// WHY A DERIVATION RATHER THAN A READ. statusgen ships as a pinned release binary
// and runs against an arbitrary `--root`; the root it is pointed at is a CONSUMER
// repo, not necessarily this one, so there is no `topology.yaml` on disk to read
// at run time. Compiling the values in is legal (brief ground-truth/04) — the
// #276 defect was five HAND-SYNCED copies with no generator and no cross-check,
// not compilation.
//
// WHY IT IS NOT A HAND TABLE. `TestTopologyValuesMatchSource`
// (topologyvalues_test.go) parses `../topology.yaml` and fails if this file
// disagrees, naming the field. The desk module carries the twin derivation
// (tools/desk/internal/topology/compiled.go) bound to the same source by
// `TestTopologyDriftRegistry`. That is the stream's derive-or-diff convention:
// exactly one declared source, every other occurrence diffed against it in CI.
//
// THE DRIFT THIS ENDS. `scanExcludedLabels` here and `excludedLabels` in
// tools/desk/cmd/issueboard/board.go were two hand-synced copies of one set, kept
// equal by a comment. They were NOT equal: issueboard was missing
// `review-request` (#829), so the board asked for a placeholder on exactly the
// class of issue this scanner deliberately excludes.
//
// TO CHANGE A VALUE: edit `topology.yaml` FIRST, then mirror it here, then run
// `cd statusgen && go test -run TestTopologyValuesMatchSource ./...`.

// topologySourceFile is the repo-relative path of the declared source, relative
// to the repo root (statusgen's own directory is one level below it).
const topologySourceFile = "topology.yaml"

// topologySchema is the schema version this derivation was taken from. A source
// declaring anything else means the schema moved without this file moving with
// it, and the diff test refuses rather than comparing across versions.
const topologySchema = "topology-v1"

// topologySystemStateLabels is `labels.system_state` from the declared source, in
// source order. Each names a CLOSEABLE STATE the desk machinery emits, not a unit
// of work; the rationale for each entry lives beside it in topology.yaml, which
// is where a new one is added.
var topologySystemStateLabels = []string{
	"verify-gate",
	"live-verify",
	"needs-decision",
	"review-request",
	"needs-human",
}

// topologyDecisionOwedLabels is `labels.decision_owed` from the declared source.
// statusgen does not gate on it today; it is derived here so the schema has one
// derivation per module rather than a partial one that the next consumer widens
// by hand.
var topologyDecisionOwedLabels = []string{
	"needs-decision",
	"question",
	"needs-human",
}

// topologyReleaseRepo is `release_repo` from the declared source.
var topologyReleaseRepo = "medici-finance/assay"

// topologyRiskPathTriggersByRepo is `repos[].risk_path_triggers` from the
// declared source — the ADDITIONAL risk-classing path prefixes each repo layers
// on top of the universal base list, keyed by owner/name slug. Only repos that
// state at least one trigger appear (a repo the topology says nothing about gets
// the base list only). It is the statusgen half of the mistake-proofing/01
// cross-read: the lint duplicates the desk classifier's POLICY half
// (riskpathtriggers.go) and reads the per-repo additions from HERE. Bound to the
// source field-for-field by TestTopologyValuesMatchSource, exactly as the desk
// module's compiled.go is bound by TestTopologyDriftRegistry — edit topology.yaml
// FIRST, then mirror it here, then run
// `cd statusgen && go test -run TestTopologyValuesMatchSource ./...`.
var topologyRiskPathTriggersByRepo = map[string][]string{
	"example-org/example-k8s": {
		"base/",
		"deploy/",
		"admin/",
		"hack/",
		".github/workflows/",
	},
	"medici-finance/assay": {
		"tools/desk/internal/deskkit/",
		"tools/desk/cmd/deskpost/",
		"tools/desk/cmd/deskboard/",
		"tools/desk/cmd/desktoken/",
		"tools/desk/cmd/deskpushguard/",
		"tools/desk/cmd/writeguard/",
	},
}

// labelSetOf renders a label list as the lowercase lookup map the matchers use.
func labelSetOf(names []string) map[string]bool {
	out := make(map[string]bool, len(names))
	for _, n := range names {
		out[strings.ToLower(strings.TrimSpace(n))] = true
	}
	return out
}
