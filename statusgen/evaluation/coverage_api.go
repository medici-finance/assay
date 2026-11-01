package evaluation

import (
	"fmt"
	"sort"
	"strings"
)

// coverage_api.go — the typed public surface of the coverage rule: the
// immutable observations EvaluateCoverage reads, and the entry point.
//
// The caller supplies EVERYTHING the verdict depends on: the item's revision
// (never an implicit HEAD), each brief's Verify rows and recorded witnesses,
// declared inputs, pattern bindings, dependency manifests, and a Repo for
// history questions. The function reads no environment, clock, network,
// process or file, holds no global state and writes nothing. Coverage grants no
// publication: a Released verdict is advice for the caller's own flip logic.

const (
	// TreeSHALen is the length of the tree token a witness records, and the
	// shortest revision token that is ever informative.
	TreeSHALen = 12
	// VerifyOutcomesGlob names the verify-outcome logs verify and regen write.
	VerifyOutcomesGlob = "verify-outcomes*.jsonl"
	// ShellSh is the default Verify-row shell, whose command grammar the input
	// derivation reads.
	ShellSh = "sh"
	// ObligationFlow is the `+flow` obligation token: a Verify row exercising
	// the cross-component path end to end.
	ObligationFlow = "flow"
)

// Witness states, as a Verify-row witness records them. The empty State is a
// witness whose Result cell carries none of them (unparseable).
const (
	StatePass        = "pass"
	StateFail        = "fail"
	StateCouldNotRun = "could-not-run"
)

// NormalizeCommandText collapses whitespace so a reflowed table cell is not
// mistaken for an edited command.
func NormalizeCommandText(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// UnescapePipes converts a markdown-table-escaped `\|` back into the pipe the
// author wrote.
func UnescapePipes(command string) string {
	return strings.ReplaceAll(command, `\|`, `|`)
}

// Binding is the (pattern, node) a brief is instantiated at.
type Binding struct {
	Pattern string
	Node    string
}

// PatternEvidence is one evidence entry of a pattern node.
type PatternEvidence struct {
	Kind      string
	Claim     string
	Mandatory bool
	Source    string
}

// PatternNode is the part of a workflow-pattern node coverage reads.
type PatternNode struct {
	Evidence []PatternEvidence
}

// Pattern is the part of a workflow pattern coverage reads: its nodes by id
// and its join node's id.
type Pattern struct {
	Join  string
	Nodes map[string]PatternNode
}

// VerifyRow is one declared Verify row.
type VerifyRow struct {
	ID      string
	Command string // as authored: `\|`-escaped, code span lifted
	Expect  string
	// Shell is the resolved per-row shell; "" and ShellSh are the default.
	Shell string
	// Obligations are the row's known `+`-prefixed obligation tokens.
	Obligations []string
	// ProseLed, when set, says the Command cell is a mention rather than a
	// command (a pass recorded on it measured the mention). ProseLedNote is the
	// could-not-check reason the caller words for it.
	ProseLed     string
	ProseLedNote string
	// ClassCell is the row's Class cell as authored (trimmed), the text a v2
	// record's class_sha256 digests.
	ClassCell string
}

// Witness is one recorded execution witness for a Verify row, as read off the
// brief's Evidence table. Callers pass only genuine execution-witness rows,
// in document order; prose is never a witness.
type Witness struct {
	Command string // the Command cell, code span lifted
	State   string // StatePass, StateFail, StateCouldNotRun, or "" (unparseable)
	Tree    string // the tree token after the Runner cell's `@`, "" when absent
	// Runner, Result, Output and Date are the row's Runner, Result, output-digest
	// and Date cells as written (trimmed). A v2 cell (`[w2]`) is judged from
	// them (witness.go); a v1 cell reads only Runner's read-set token
	// (readset.go). An empty Runner is read from Tree alone.
	Runner string
	Result string
	Output string
	Date   string
	// ReadSet is the caller's observation of the read-set file the Runner
	// cell's `(reads sha256:…)` token names, and of the commit that
	// introduced this row (readset.go); nil when none was observed.
	ReadSet *ReadSetObservation
}

// BriefObservation is everything coverage reads about one brief.
type BriefObservation struct {
	// ID is "<stream>/<NN>".
	ID string
	// BriefRel is the brief file's slash-separated path from the repository
	// root, "" when it has none.
	BriefRel string
	// DeclaredFiles are the brief's `files:` entries; DeclaredFilesFound is
	// false when the brief declares none.
	DeclaredFiles      []string
	DeclaredFilesFound bool
	// DeclaredInputs are the brief's `inputs:` entries (ParseInputsDeclaration);
	// DeclaredInputsFound is false when the field is absent or unparseable (a
	// v2 claim is then whole-tree scope). DeclaredInputsProblem is the parser's
	// lint problem, never read by coverage.
	DeclaredInputs        []string
	DeclaredInputsFound   bool
	DeclaredInputsProblem string
	VerifyRows            []VerifyRow
	// AcceptanceRevision is an optional immutable original-table revision.
	// The observer must validate protected delivery/baseline/original provenance
	// before populating it. It governs only the historical Expect definition
	// read; the witness keeps its real execution tree and all applicability and
	// dependency checks. Empty retains the ordinary witness-base definition.
	AcceptanceRevision string
	// AcceptanceRevisions overrides only explicitly approved rows; other rows retain the original definition.
	AcceptanceRevisions map[string]string
	// AcceptanceProblem holds coverage even if a pattern supplies other passing
	// claims. An invalid protected definition is never an empty, releasable set.
	AcceptanceProblem string
	// Witnesses maps a Verify row id to its recorded witnesses.
	Witnesses map[string][]Witness
}

// WorkInputDep is one declared work-input dependency (WI-2). Rows lists the
// Verify row ids it applies to; empty means every row.
//
//   - source, policy, build: Path is repo-relative (a file or a directory).
//     A source dependency joins the claim's witness scope. A policy or build
//     dependency is compared by fingerprint: the git object id of Path at the
//     witness's base commit against the one at the item's revision.
//   - environment: Name identifies the value; Recorded is its fingerprint when
//     the witness ran and Current is its fingerprint now. Either one empty
//     means the dependency cannot be checked.
type WorkInputDep struct {
	Kind     string
	Path     string
	Name     string
	Recorded string
	Current  string
	Rows     []string
}

// DependencyManifest is one brief's declared work-input dependencies.
// Complete is the manifest's own statement that it names EVERY dependency of
// every claim. Only a complete manifest licenses selective reuse (a scope
// narrower than the conservative one). An incomplete manifest's entries are
// still checked, but its claims also keep the conservative scope.
type DependencyManifest struct {
	Complete bool
	Deps     []WorkInputDep
}

// CoverageInput is everything EvaluateCoverage reads besides the Repo.
type CoverageInput struct {
	// Revision is the item's revision to compare witnesses against. It is
	// explicit: "" is "no revision known", and every witness comparison then
	// resolves could-not-check — never an implicit HEAD.
	Revision string
	Briefs   []BriefObservation
	// Bindings maps a brief id to the pattern node it is instantiated at; an
	// absent entry uses the Verify rows alone. A binding naming an unknown
	// pattern or node is ignored.
	Bindings map[string]Binding
	// Patterns is the pattern set, keyed by pattern name. Consulted only for a
	// brief present in Bindings.
	Patterns map[string]Pattern
	// Dependencies maps a brief id to its dependency manifest; absent entries
	// keep the conservative scope.
	Dependencies map[string]DependencyManifest
	// MaxLandingCandidates bounds the squash-commit search; 0 means
	// DefaultMaxLandingCandidates.
	MaxLandingCandidates int
	// WitnessPolicy is the environment rule a v2 witness is judged by; the
	// zero value is the ruled default (option 1: the pinned network-less
	// container runner, or a host or back-filled run whose row is not `+env`
	// and whose readable command names no network tool).
	WitnessPolicy WitnessPolicy
	// Forge is the optional code-hosting seam a v2 landing check consults; nil
	// is offline.
	Forge Forge
	// GoFloors answers the compile-input floor of a build-family `go-run`
	// entry of a witness's read set. It is asked only for a read set the
	// library has admitted (digest, header and introduction binding all
	// hold), so a read set that is never admitted never reaches the go tool.
	// A read set with such an entry and no floor here (nil, or an unanswered
	// entry) is could-not-check, never narrowed.
	GoFloors GoFloorSource
}

// EvaluateCoverage computes the coverage verdict of every observed brief,
// keyed by brief id. repo answers the history questions; nil is "no
// repository" (every revision comparison that needs one is then a plain
// mismatch or could-not-check, never a pass).
//
// semantic: S-evidence-coverage
func EvaluateCoverage(in CoverageInput, repo Repo) map[string]Coverage {
	x := newEvalCtx(repo, in.MaxLandingCandidates)
	x.policy, x.forge = in.WitnessPolicy, in.Forge
	x.goFloors = in.GoFloors
	x.prefetchV2Records(in)
	out := map[string]Coverage{}
	for _, b := range in.Briefs {
		out[b.ID] = evaluateOneCoverage(x, b, in)
	}
	return out
}

// evaluateOneCoverage resolves one brief's claim set and Released verdict.
func evaluateOneCoverage(x *evalCtx, b BriefObservation, in CoverageInput) Coverage {
	id := b.ID
	revision := in.Revision
	var claims []Claim
	if b.AcceptanceProblem != "" {
		claims = append(claims, Claim{Claim: "protected acceptance definition", Kind: "command", Result: ResultCouldNotCheck, Reason: b.AcceptanceProblem})
	}

	// (a) every Verify row is a mandatory claim by construction.
	var man *DependencyManifest
	if m, ok := in.Dependencies[id]; ok {
		man = &m
	}
	scope := newWitnessScope(b.BriefRel, b.DeclaredFiles, b.DeclaredFilesFound, man)
	scope.briefID = id
	scope.v2inputs, scope.v2inputsFound = b.DeclaredInputs, b.DeclaredInputsFound
	for _, r := range b.VerifyRows {
		definition := b.AcceptanceRevision
		if amended, ok := b.AcceptanceRevisions[r.ID]; ok {
			if amended == "" {
				claims = append(claims, Claim{Claim: fmt.Sprintf("Verify row #%s: %s", r.ID, r.Command), Kind: "command", Result: ResultCouldNotCheck, Reason: "approved acceptance revision is empty"})
				continue
			}
			definition = amended
		}
		result, reason, claimRev := resolveVerifyClaim(x, scope.forVerifyRow(r), r, b.Witnesses[r.ID], revision, definition)
		claims = append(claims, Claim{
			Claim:    fmt.Sprintf("Verify row #%s: %s", r.ID, r.Command),
			Kind:     "command",
			Result:   result,
			Revision: claimRev,
			Released: result == ResultPass,
			Reason:   reason,
		})
	}

	// (b) the pattern node's own mandatory evidence, when this brief is bound to
	// one.
	var atJoin bool
	if binding, ok := in.Bindings[id]; ok {
		if pat, ok := in.Patterns[binding.Pattern]; ok {
			if node, ok := pat.Nodes[binding.Node]; ok {
				for _, ev := range node.Evidence {
					if !ev.Mandatory {
						continue
					}
					result, reason := resolvePatternEvidenceClaim(ev)
					claims = append(claims, Claim{
						Claim:    ev.Claim,
						Kind:     ev.Kind,
						Result:   result,
						Revision: revision,
						Released: result == ResultPass,
						Reason:   reason,
					})
				}
				atJoin = binding.Node == pat.Join
			}
		}
	}

	// The join's integration check: at least one Verify row classed `+flow` —
	// individually passing rows do not release the join.
	if atJoin {
		hasFlow := false
		for _, r := range b.VerifyRows {
			for _, ob := range r.Obligations {
				if ob == ObligationFlow {
					hasFlow = true
					break
				}
			}
		}
		if !hasFlow {
			claims = append(claims, Claim{
				Claim:  "integration-check",
				Kind:   "command",
				Result: ResultMissing,
				Reason: fmt.Sprintf("[%s] the pattern's join node requires at least one Verify row classed `+flow` — individually passing rows do not release the join", RuleJoinMissingFlow),
			})
		}
	}

	if len(claims) == 0 {
		// Fail-closed: a brief with no Verify rows and no pattern binding has
		// nothing coverage can corroborate, so it is HELD, not vacuously released.
		claims = append(claims, Claim{
			Claim: "mandatory claims", Kind: "command", Result: ResultMissing,
			Reason: "brief declares no Verify rows and binds no pattern node — nothing to release against",
		})
	}

	released := true
	for _, c := range claims {
		if !c.Released {
			released = false
			break
		}
	}
	return Coverage{BriefID: id, Released: released, Claims: claims}
}

// SortedIDs returns the keys of a coverage map in a fixed order.
func SortedIDs(m map[string]Coverage) []string {
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
