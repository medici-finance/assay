package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/medici-finance/assay/statusgen/evaluation"
)

// eligibility.go — the statusgen ADAPTER for the eligibility evaluator
// (graph-execution/01). The evaluation itself — `gates:` and `feathers:`
// becoming GATING with an explainable reason (docs/dependency-graph-design.md
// §3.6) — lives in the statusgen/evaluation package since statusgen/15; this
// file only observes the tree (streams, the graph-repos registry, sibling
// checkouts), maps those observations onto the library's immutable input
// types, and hands them over. It holds no second implementation of any verdict.
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

// The wire and verdict types are the library's: one definition, the CLI and
// every other consumer read the same values.
type (
	EligibilityState   = evaluation.EligibilityState
	EligibilityVerdict = evaluation.EligibilityVerdict
	EdgeVerdict        = evaluation.EdgeVerdict
	Eligibility        = evaluation.Eligibility
)

const (
	StateSatisfied     = evaluation.StateSatisfied
	StateUnsatisfied   = evaluation.StateUnsatisfied
	StateCouldNotCheck = evaluation.StateCouldNotCheck

	VerdictEligible           = evaluation.VerdictEligible
	VerdictHeld               = evaluation.VerdictHeld
	VerdictEligibleWithNotice = evaluation.VerdictEligibleWithNotice
)

// evaluationStreams maps the loaded streams onto the library's input types.
// humanGatedVerifyPassed (the evidence reading) stays here; the library
// receives only its boolean result.
func evaluationStreams(streams []*Stream) []evaluation.Stream {
	out := make([]evaluation.Stream, 0, len(streams))
	for _, s := range streams {
		es := evaluation.Stream{Name: s.Name, Briefs: make([]evaluation.Brief, 0, len(s.Briefs))}
		for _, b := range s.Briefs {
			es.Briefs = append(es.Briefs, evaluation.Brief{
				Num:                   b.Num,
				Status:                b.Status,
				Depends:               b.Depends,
				Gates:                 evaluationEdges(b.Gates),
				Feathers:              evaluationEdges(b.Feathers),
				HumanGatePassRecorded: humanGatedVerifyPassed(b),
			})
		}
		out = append(out, es)
	}
	return out
}

func evaluationEdges(edges []GraphEdge) []evaluation.Edge {
	var out []evaluation.Edge
	for _, e := range edges {
		out = append(out, evaluation.Edge{Ref: e.Ref, Type: e.Type, Reason: e.Reason})
	}
	return out
}

// evaluationRegistry maps the loaded registry (nil = absent) onto the library.
func evaluationRegistry(reg *graphRepos) *evaluation.Registry {
	if reg == nil {
		return nil
	}
	out := &evaluation.Registry{Aliases: map[string]evaluation.RegistryAlias{}}
	for name, a := range reg.Aliases {
		out.Aliases[name] = evaluation.RegistryAlias{Repo: a.Repo, Unpublished: a.Unpublished}
	}
	return out
}

// siblingLookup observes the checkout of a published alias's repository next
// to root (a directory named after the repo's basename, e.g.
// "medici-finance/assay" -> "assay"). This is the filesystem half of
// cross-repo resolution; the verdict reasoning is the library's.
func siblingLookup(root string) evaluation.SiblingLookup {
	return func(repo string) evaluation.Sibling {
		name := repo
		if i := strings.LastIndexByte(name, '/'); i >= 0 {
			name = name[i+1:]
		}
		absRoot, err := filepath.Abs(root)
		if err != nil {
			absRoot = root
		}
		siblingRoot := filepath.Join(filepath.Dir(absRoot), name)
		if fi, err := os.Stat(siblingRoot); err != nil || !fi.IsDir() {
			return evaluation.Sibling{Where: siblingRoot}
		}
		streams, _, err := loadStreams(siblingRoot)
		if err != nil {
			return evaluation.Sibling{Found: true, Where: siblingRoot, ReadError: err.Error()}
		}
		return evaluation.Sibling{Found: true, Where: siblingRoot, Streams: evaluationStreams(streams)}
	}
}

// evaluateEligibility computes the eligibility verdict of every brief across
// streams by observing reg (the loaded docs/streams/graph-repos.yaml registry
// for THIS root; nil when absent) and any sibling checkout next to root, then
// asking evaluation.EvaluateEligibility. The rules are documented there.
func evaluateEligibility(streams []*Stream, reg *graphRepos, root string) map[string]Eligibility {
	return evaluation.EvaluateEligibility(evaluation.EligibilityInput{
		Streams:  evaluationStreams(streams),
		Registry: evaluationRegistry(reg),
		Siblings: siblingLookup(root),
	})
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
