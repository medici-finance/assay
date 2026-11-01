package main

// coverage.go — the statusgen ADAPTER for the evidence coverage rule
// (graph-execution/03). The rule itself — what the mandatory claims of a brief
// are, how a recorded witness resolves against the item's revision, how a
// witness is reused at a later revision, what holds a brief — lives in the
// statusgen/evaluation package since statusgen/15, which documents the full
// semantics. This file observes the tree (briefs, Verify rows, Evidence
// witnesses, the checked-out revision), maps those observations onto the
// library's immutable input types, supplies a git-backed Repo (coverage_repo.go)
// and hands them over. It holds no second implementation of any verdict; the
// CLI rendering (coverageLine, runCoverage) stays here because it is
// presentation, not evaluation.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/medici-finance/assay/statusgen/evaluation"
)

// The result vocabulary, verdict types and dependency manifest are the
// library's: one definition.
const (
	covPass          = evaluation.ResultPass
	covFail          = evaluation.ResultFail
	covMissing       = evaluation.ResultMissing
	covError         = evaluation.ResultError
	covCouldNotCheck = evaluation.ResultCouldNotCheck
	covWrongRevision = evaluation.ResultWrongRevision

	depSource      = evaluation.DepSource
	depPolicy      = evaluation.DepPolicy
	depBuild       = evaluation.DepBuild
	depEnvironment = evaluation.DepEnvironment

	ruleCoverageJoinMissingFlow = evaluation.RuleJoinMissingFlow
)

// Helpers the coverage rule shares with the verifyrun and rowclass code: ONE
// definition, in the library.
const (
	// treeSHALen is how much of the HEAD SHA a witness records. Longer than
	// git's 7-char default on purpose: this is the field a re-runner uses to
	// fetch the exact tree the output hash belongs to, so collision-resistance
	// beats brevity.
	treeSHALen         = evaluation.TreeSHALen
	verifyOutcomesGlob = evaluation.VerifyOutcomesGlob
	// classFlow: exercises the cross-component path end to end, not just the
	// changed site.
	classFlow = evaluation.ObligationFlow
	// rowShellSh: POSIX bash — the default, today's behaviour.
	rowShellSh = evaluation.ShellSh
)

// unescapePipes converts a markdown-table-escaped `\|` back into the pipe the
// author wrote (evaluation.UnescapePipes).
func unescapePipes(command string) string { return evaluation.UnescapePipes(command) }

// normalizeCommandText collapses whitespace so a reflowed table cell is not
// mistaken for an edited command.
func normalizeCommandText(s string) string { return evaluation.NormalizeCommandText(s) }

type (
	Claim              = evaluation.Claim
	Coverage           = evaluation.Coverage
	dependencyManifest = evaluation.DependencyManifest
	workInputDep       = evaluation.WorkInputDep
)

// maxLandingCandidates bounds the landing check's squash-commit search (the
// evaluation package's default); a variable so a test can shrink it.
var maxLandingCandidates = evaluation.DefaultMaxLandingCandidates

// coverageResultOrder is the fixed ranking coverageLine uses to pick the FIRST
// reason deterministically when several claims hold — same rank every run,
// regardless of map/slice iteration order upstream.
var coverageResultOrder = map[string]int{
	covFail:          0,
	covMissing:       1,
	covError:         2,
	covWrongRevision: 3,
	covCouldNotCheck: 4,
	covPass:          5,
}

// coverageBinding is the (pattern, node) a brief is instantiated at — supplied
// by a caller (today: only a test; production has no adapter yet).
type coverageBinding struct {
	Pattern string
	Node    string
}

// coverageOptions is evaluateCoverage's optional input set.
type coverageOptions struct {
	// Bindings maps a brief id ("<stream>/<NN>") to the pattern node it is
	// instantiated at. nil/absent entries use the Verify rows alone.
	Bindings map[string]coverageBinding
	// Patterns is the parsed pattern-file set, keyed by `pattern:` name
	// (loadPatternDocs). Consulted only for a brief present in Bindings.
	Patterns map[string]patternDoc
	// Revision is the item's revision to compare witnesses against. "" resolves
	// to the current tree's HEAD SHA (currentTreeRevision).
	Revision string
	// Dependencies maps a brief id to its work-input dependency manifest.
	// Production passes none today, so every claim takes the conservative scope.
	Dependencies map[string]dependencyManifest
	// readSets observes each witness's recorded read set, its introduction
	// and the go compile-input floor (build-less-brittle/15). Set by
	// coverageOptionsFor only when read sets are honoured
	// (readSetRecorderShipped); nil judges as a build with no recorder: no
	// observation runs and no read-set token is judged, so a row with one
	// comes out as the same row without it (never a narrower scope).
	readSets *readSetSource
}

// coverageOptionsFor is the one source of coverage options for the callers
// that decide a brief (autoflip, the reconcile fold, --coverage): the parsed
// pattern files plus, once a recorder ships (readSetRecorderShipped), the
// read-set, introduction and go-floor observations. No caller builds an
// empty options value itself.
func coverageOptionsFor(root string, patterns map[string]patternDoc) coverageOptions {
	opts := coverageOptions{Patterns: patterns}
	if honourReadSets {
		opts.readSets = newReadSetSource(root)
	}
	return opts
}

// evaluateCoverage computes the coverage verdict for every brief across
// streams. root anchors the observation of the checked-out revision and of git
// history (coverageRepo); the verdicts are evaluation.EvaluateCoverage's.
func evaluateCoverage(root string, streams []*Stream, opts coverageOptions) map[string]Coverage {
	cov, _ := evaluateCoverageObserved(root, streams, opts)
	return cov
}

// observedBrief is one brief's coverage observation plus, per Verify row id,
// the raw text of each witness row in the order observed — what the
// `--coverage --json` witness field (coverage_json.go) reports from.
type observedBrief struct {
	Path         string
	Obs          evaluation.BriefObservation
	WitnessTexts map[string][]string
}

// evaluateCoverageObserved is evaluateCoverage that also returns the
// observations it judged, keyed by brief id.
func evaluateCoverageObserved(root string, streams []*Stream, opts coverageOptions) (map[string]Coverage, map[string]observedBrief) {
	observed := map[string]observedBrief{}
	revision := opts.Revision
	if revision == "" {
		revision = currentTreeRevision(root)
	}
	in := evaluation.CoverageInput{Revision: revision, MaxLandingCandidates: maxLandingCandidates}
	for _, s := range streams {
		for _, path := range briefFilePaths(s) {
			bf, ok, err := parseBriefFile(path)
			if err != nil || !ok {
				continue // malformed (reported elsewhere) or legacy/opted-out
			}
			_, num, okName := expectedBriefID(path)
			if !okName {
				continue
			}
			obs, texts := observeBriefTexts(root, path, s.Name+"/"+num, bf)
			if opts.readSets != nil {
				opts.readSets.observeBrief(path, obs.BriefRel, obs.Witnesses)
			}
			observed[obs.ID] = observedBrief{Path: path, Obs: obs, WitnessTexts: texts}
			in.Briefs = append(in.Briefs, obs)
		}
	}
	if len(opts.Bindings) > 0 {
		in.Bindings = map[string]evaluation.Binding{}
		for id, b := range opts.Bindings {
			in.Bindings[id] = evaluation.Binding{Pattern: b.Pattern, Node: b.Node}
		}
		in.Patterns = evaluationPatterns(opts.Patterns)
	}
	if len(opts.Dependencies) > 0 {
		in.Dependencies = map[string]evaluation.DependencyManifest{}
		for id, m := range opts.Dependencies {
			deps := make([]evaluation.WorkInputDep, len(m.Deps))
			for i, d := range m.Deps {
				d.Path = filepath.ToSlash(d.Path)
				deps[i] = d
			}
			in.Dependencies[id] = evaluation.DependencyManifest{Complete: m.Complete, Deps: deps}
		}
	}
	if opts.readSets != nil {
		in.ReadSetsRecorded = true
		in.GoFloors = opts.readSets
		in.ReadSetDeadline = opts.readSets.overdue
		defer opts.readSets.close()
	}
	var repo evaluation.Repo
	if root != "" {
		repo = newMemoRepo(newCoverageRepo(root))
	}
	return evaluation.EvaluateCoverage(in, repo), observed
}

// observeBrief reads one brief file's coverage-relevant observations: its
// Verify rows, its Evidence witnesses (genuine verifyrun execution rows only —
// prose never qualifies, isWitnessRow) and its declared `files:` entries.
func observeBrief(root, path, id string, bf *BriefFile) evaluation.BriefObservation {
	obs, _ := observeBriefTexts(root, path, id, bf)
	return obs
}

// observeBriefTexts is observeBrief plus each witness row's raw text, index
// for index with obs.Witnesses.
func observeBriefTexts(root, path, id string, bf *BriefFile) (evaluation.BriefObservation, map[string][]string) {
	texts := map[string][]string{}
	obs := evaluation.BriefObservation{ID: id, Witnesses: map[string][]evaluation.Witness{}}
	if root != "" && path != "" {
		if rel, err := filepath.Rel(root, path); err == nil {
			obs.BriefRel = filepath.ToSlash(rel)
		}
	}
	if bf.DeclaredEntriesRawFound {
		obs.DeclaredFilesFound = true
		for _, e := range bf.DeclaredEntriesRaw {
			obs.DeclaredFiles = append(obs.DeclaredFiles, filepath.ToSlash(e))
		}
	}
	plan, refusal := protectedAcceptance(bf.Verify, bf.Evidence, path)
	if refusal != "" {
		// Retain visible rows and an explicit held observation. Empty rows
		// alone could be released by a separate pattern binding's claims.
		obs.AcceptanceProblem = refusal
		plan.Verify = bf.Verify
	}
	obs.AcceptanceRevision = plan.Original
	obs.AcceptanceRevisions = plan.Revisions
	obs.DeclaredInputs, obs.DeclaredInputsFound, obs.DeclaredInputsProblem = evaluation.ParseInputsDeclaration(extractSectionByPrefix(bf.Body, "Context"))
	rows, classCells := briefVerifyRowsWithClassCells(plan.Verify)
	for i, r := range rows {
		row := evaluation.VerifyRow{
			ID: r.ID, Command: r.Command, Expect: r.Expect,
			Shell: r.Shell, Obligations: r.Obligations, ProseLed: r.ProseLed,
			ClassCell: classCells[i],
		}
		if r.ProseLed != "" {
			row.ProseLedNote = proseLedNote(r.ProseLed)
		}
		obs.VerifyRows = append(obs.VerifyRows, row)
	}
	for rowID, rows := range parseEvidenceRows(plan.Evidence) {
		for _, er := range rows {
			if !isWitnessRow(er.Text) {
				continue
			}
			cells := witnessCells(er.Text)
			obs.Witnesses[rowID] = append(obs.Witnesses[rowID], evaluation.Witness{
				Command: witnessCommandOf(er.Text),
				State:   witnessStateOf(er.Text),
				Tree:    witnessTreeOf(er.Text),
				Runner:  strings.TrimSpace(cells[witnessCellRunner]),
				Result:  strings.TrimSpace(cells[witnessCellResult]),
				Output:  strings.TrimSpace(cells[witnessCellOutput]),
				Date:    strings.TrimSpace(cells[witnessCellDate]),
			})
			texts[rowID] = append(texts[rowID], er.Text)
		}
	}
	return obs, texts
}

// evaluationPatterns maps the parsed pattern documents onto the library's
// minimal pattern view.
func evaluationPatterns(docs map[string]patternDoc) map[string]evaluation.Pattern {
	out := map[string]evaluation.Pattern{}
	for name, d := range docs {
		p := evaluation.Pattern{Join: d.Join, Nodes: map[string]evaluation.PatternNode{}}
		for id, n := range d.NodeByID {
			node := evaluation.PatternNode{}
			for _, ev := range n.Evidence {
				node.Evidence = append(node.Evidence, evaluation.PatternEvidence{
					Kind: ev.Kind, Claim: ev.Claim, Mandatory: ev.Mandatory, Source: ev.Source,
				})
			}
			p.Nodes[id] = node
		}
		out[name] = p
	}
	return out
}

// currentTreeRevision is the offline stand-in for "the item's revision": the
// HEAD SHA of the tree currently checked out at root, truncated to the same
// treeSHALen a witness itself records, so the two compare like-for-like. ""
// when root carries no git history to read (a bare testdata copy) — callers
// must treat that as "nothing to compare", never a match or a mismatch.
func currentTreeRevision(root string) string {
	sha := gitCurrentSHA(root)
	if sha == "" {
		return ""
	}
	if len(sha) > treeSHALen {
		sha = sha[:treeSHALen]
	}
	return sha
}

// coverageLine renders one brief's --coverage (non-JSON) line: `<id>
// released|held <n-claims> <first-reason>`.
func coverageLine(c Coverage) string {
	verdict := "held"
	if c.Released {
		verdict = "released"
	}
	line := fmt.Sprintf("%s %s %d", c.BriefID, verdict, len(c.Claims))
	if reason := firstHoldingReason(c); reason != "" {
		line += " " + reason
	}
	return line
}

// firstHoldingReason picks the first non-pass claim's reason, in a fixed,
// deterministic order (coverageResultOrder) so the printed line never depends
// on map/slice iteration order.
func firstHoldingReason(c Coverage) string {
	var best *Claim
	for i := range c.Claims {
		cl := &c.Claims[i]
		if cl.Released {
			continue
		}
		if best == nil || coverageResultOrder[cl.Result] < coverageResultOrder[best.Result] {
			best = cl
		}
	}
	if best == nil {
		return ""
	}
	return fmt.Sprintf("%s: %s", best.Claim, best.Reason)
}

// coverageRefusalReason renders EVERY non-released claim's reason for a held
// brief — autoflip.go's refusal (Task item 4) names every gap at once, unlike
// coverageLine's single "first reason" (a --coverage run lists one line per
// BRIEF; a flip refusal is read by a human deciding what to fix next, so every
// gap is named up front rather than found one dry-run at a time).
func coverageRefusalReason(c Coverage) string {
	var reasons []string
	for _, cl := range c.Claims {
		if cl.Released {
			continue
		}
		reasons = append(reasons, fmt.Sprintf("%s (%s): %s", cl.Claim, cl.Result, cl.Reason))
	}
	if len(reasons) == 0 {
		return "coverage is not released"
	}
	return "coverage is not released — " + strings.Join(reasons, "; ")
}

// ---------------------------------------------------------------------------
// --coverage CLI
// ---------------------------------------------------------------------------

// coverageExit* mirrors every other self-contained emitter in this binary
// (--eligibility, --next-up): exit 0 on any verdict (held is not itself a
// command failure), exit 2 only when the tree could not be read at all.
const (
	coverageExitOK       = 0
	coverageExitCouldNot = 2
)

// runCoverage is the `statusgen --coverage [--json] --root <root>` entry point.
// Offline and STATUS.md-free, same discipline as runEligibility. No bindings
// are supplied here — see this file's header on why that is fact (a) alone for
// every brief today, not a scope cut.
func runCoverage(root string, jsonMode bool) int {
	streams, _, err := loadHydratedStreams(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return coverageExitCouldNot
	}
	patterns, _ := loadPatternDocs(root)
	cov, observed := evaluateCoverageObserved(root, streams, coverageOptionsFor(root, patterns))

	ids := make([]string, 0, len(cov))
	for id := range cov {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	if jsonMode {
		out, err := json.Marshal(coverageJSONRows(root, ids, cov, observed))
		if err != nil {
			fmt.Fprintln(os.Stderr, "statusgen:", err)
			return 1
		}
		fmt.Println(string(out))
		return coverageExitOK
	}

	for _, id := range ids {
		fmt.Println(coverageLine(cov[id]))
	}
	return coverageExitOK
}
