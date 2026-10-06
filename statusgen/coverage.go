package main

// coverage.go — the evidence coverage rule (graph-execution/03).
//
// THE HOLE IT CLOSES. A brief can be marked `verified` while a mandatory check
// was never run, errored, or ran against a different revision of the work: the
// tools already RECORD what happened (verifyrun.go's witnesses, evidenceactor.go's
// attribution, the reviewer App's review object) but until this file no rule said
// what MUST have happened before the next step is allowed. This is that rule:
// deterministic, offline, and expressed as one function every consumer reads —
// exactly the same shape eligibility.go's evaluateEligibility already is for the
// depends:/gates: graph.
//
// WHAT "COVERAGE" MEANS. For one brief, the set of MANDATORY CLAIMS is:
//
//	(a) every Verify row the brief file declares — a Verify row is an obligation
//	    the brief itself took on, so every one of them is mandatory by
//	    construction, independent of any pattern;
//	(b) when the brief is bound to a workflow-pattern-v1 node (Task item 1's
//	    "when a pattern applies"), the union of (a) with that node's
//	    `evidence[].mandatory: true` entries — the node's OWN declared claims,
//	    which may name things no Verify row states (a review object, an
//	    observed signal).
//
// A brief is `released` only when EVERY mandatory claim resolves to `pass` AT
// THE ITEM'S REVISION. Anything else — missing, error, could-not-check,
// wrong-revision, or an explicit fail — HOLDS the brief, and the reason names
// the claim. This mirrors docs/three-state-instrument-rule.md at the aggregate
// level: "I could not establish this" and "this is a claim with no evidence
// behind it" must never collapse into the same bucket as "this passed".
//
// NO BINDING PROVIDER EXISTS YET. The instance contract that will bind a brief
// to a specific pattern node (graph-execution/09) has not landed — this brief's
// own integration amendment says so explicitly: "the later instance contract
// (09) supplies bindings through an adapter; this core rule remains testable
// without a provider." So evaluateCoverage takes its pattern bindings as an
// explicit, OPTIONAL parameter (coverageBindings) rather than resolving them
// from brief frontmatter that does not exist yet. In production
// (runCoverage / autoFlipModel) that map is empty, which is exactly fact (a)
// alone — "a brief with no pattern uses (a) alone" holds for every brief in this
// tree today, by construction, not by special-casing. Tests exercise (b) and the
// join rule directly, by constructing a binding and a parsed pattern in memory —
// deterministic and provider-free, per the amendment.
//
// THE REVISION COMPARISON IS OFFLINE (revised 2026-09-25, review findings F1/F2
// on this PR, rounds 1 and 2). Task item 1 names the item's revision as "the
// merged SHA for a merged brief, the PR head for an open one"; resolving either
// would need a live PR/merge lookup — exactly the live-infrastructure contact
// the ground rules forbid, even read-only. This file does NOT implement that
// literal wording, and says so: the item's revision here is the tree checked
// out at --root (`git rev-parse HEAD`), whatever that is. On an open PR's
// branch checkout that is the PR head; on the model-lane flip job
// (`--auto-flip-model`, run at the fetched main tip after every push) it is the
// main TIP, which is generally LATER than the brief's merge SHA. spec/
// lifecycle-v1.md's coverage paragraph states this same rule.
//
// A witness, however, is committed as PART OF the Evidence write that records
// it, and the main tip keeps moving after that — so a witness can almost never
// name the item's revision exactly. classifyRevision therefore accepts a
// witness tree as matching the item's revision in either of two cases:
//
//  1. an exact prefix match (in-progress work, still on the same tree the
//     witness ran on — the common case a run BEFORE the Evidence commit sees);
//  2. the witness commit shares history with the item's revision (they
//     have a common ancestor) and no path the witness SPEAKS FOR differs
//     between the witness's tree and the item's (witnessTreeApplies,
//     witnessScope.invalidatedBy). This is a CONTENT comparison of the two
//     trees, not a commit-ancestry test (#2026): a squash merge discards the
//     branch commit a witness names, so ancestry said nothing about whether
//     the code the check read is still the code at the item's revision. The
//     witness commit need NOT be part of the item's history — a commit on a
//     branch that was squash-merged, or never merged, is accepted when its
//     tree matches in scope — but a commit with no common ancestor (an
//     unrelated root) is refused. Nothing here proves the witness's run
//     happened; the Evidence row is the only record of that, as before. The
//     witness commit must be in the object store; when it is not, the claim
//     is could-not-check, naming the missing commit. The witness's tree must
//     also have LANDED (witnessLanded, #2026 cause 1): some commit on the
//     item's history since the merge base — the base itself when the witness
//     is on that history, the squash commit when its branch was squash-merged
//     — has a tree identical to the witness's outside docs/streams/** and
//     STATUS.md. Otherwise the check ran against code the item never carried
//     as a whole, and the claim is `wrong-revision`. The merge bases and the
//     item's revision are compared first, so a witness on the item's own
//     history lands however many commits followed it; only the search for a
//     squash commit between them is bounded. A shallow clone, a git failure,
//     or a search past maxLandingCandidates is could-not-check. For a
//     `+dirty` / `+unknown` witness the landing check reads its BASE commit,
//     never the uncommitted edits it ran on, so such a witness never gets a
//     derived scope (classifyRevisionDetail) and its pass says only that the
//     base commit landed. What
//     a witness speaks for (round-2 F2, round-3 F2/F6, #2026 cause 2):
//     - with no dependency manifest (production today), the paths the row's
//       command READS, derived from its text by a closed shell grammar
//       (coverage_inputs.go, forVerifyRow), plus every `.gitattributes` on
//       the way to them — so a changelog fragment or a release stamp the row
//       does not read no longer holds it;
//     - conservatively, when the row's inputs cannot be derived (any command,
//       operator or path the grammar does not establish, a symbolic link or
//       submodule on an input's path, an input not tracked under its exact
//       path in both trees, another tracked name a case-insensitive checkout
//       opens as the same file, a non-ASCII input, a coverage root below the
//       repository's toplevel, an input verify and regen write, a `+dirty` /
//       `+unknown` witness) or a
//       manifest is supplied but incomplete: every path OUTSIDE the board's
//       bookkeeping surface: `docs/streams/**` (sibling briefs' Evidence in
//       the same verify batch, READMEs, verify-outcome logs) and the
//       regenerated `STATUS.md`, which move between ANY witness and the main
//       tip and say nothing about the code a check ran against. The reason
//       names why the row's inputs could not be derived;
//     - ONLY when a complete work-input dependency manifest is supplied for
//       the brief (see "WORK-INPUT DEPENDENCIES" below): the brief's declared
//       `files:` entries plus the row's source dependencies (a declared
//       directory covers everything under it; a trailing `/**` reads as that
//       directory; a one-segment glob matches a path or any parent of it).
//       The declaration is the UNION of the `files:` line now and as it stood
//       at the witness's base commit, so narrowing it afterwards (in the
//       brief's own, otherwise exempt, file) never shrinks the scope. Even
//       then the scope falls back to conservative when ANY entry does not
//       resolve to a real, non-exempt file at the witness's base commit or the
//       item's revision (a brace form, `.`, prose such as `n/a` or `(new)`, a
//       bare sibling name, a `**` inside a glob, an entry naming only
//       STATUS.md). An entry that names nothing would otherwise speak for
//       nothing, which is less conservative than no declaration at all. The
//       `files:` parser (extractContextDeclaredEntriesRaw) keeps every word
//       the author wrote, so prose can only widen the scope, never drop an
//       entry out of it;
//     - never the files verify and regen NECESSARILY write, even when
//       declared: `STATUS.md`, the verify outcome log (verifyOutcomesGlob), a
//       stream `README.md`, and brief files (isVerifyWrittenPath). Any OTHER
//       declared `docs/streams/` artifact stays guarded;
//     - never the brief's own file: its Verify rows are bound separately, by
//       the Command and Expect guards below.
//     The differing paths are read with `git diff --no-renames`, so a rename
//     or move reports its OLD path too, never only its destination. A path
//     the witness speaks for that differs between the two trees is a genuine
//     `wrong-revision`: the witness no longer speaks for today's code, and the
//     reason names the path.
//
// WORK-INPUT DEPENDENCIES (the 2026-09-30 work-input amendment, WI-2). A
// non-exact match is a REUSE: a result recorded at one revision is credited at
// another. WI-2 allows that only through an explicit applicability
// derivation, and says "file non-overlap alone is insufficient: shared APIs,
// generated inputs, build configuration, authority rules and transitive
// callers may invalidate an assumption outside the edited files". So:
//
//   - A `files:` declaration ALONE never narrows the scope any more. Without a
//     complete dependency manifest (coverageOptions.Dependencies), a claim
//     takes its row's derived read set when the command's text establishes
//     it (#2026 cause 2), and otherwise the conservative scope, whose
//     derivation is "nothing outside the board's bookkeeping differs". This
//     replaces the round-3 residual, where a change to a helper outside
//     `files:` let the old PASS stand.
//   - A manifest names each claim's dependencies by kind: source (joins the
//     witness scope), policy and build (compared by the git object id of the
//     path at the witness's base commit against the item's revision), and
//     environment (a recorded against a current fingerprint). A changed
//     dependency holds only the claims that depend on it, as `wrong-revision`,
//     and the reason names it. A dependency that cannot be fingerprinted is
//     `could-not-check`. These checks run on the exact path too.
//   - Only a manifest marked Complete licenses a scope narrower than the
//     conservative one; an incomplete manifest's entries are still checked.
//   - A reused pass says so: its reason names the revision it is reused at and
//     the derivation. Claim.Revision stays the witness's own revision, so the
//     old receipt is never retargeted to the new subject.
//   - The result vocabulary is unchanged. graph-execution/09's adapter
//     supplies manifests later. Production passes none today, so every row
//     whose read set its command does not establish takes the conservative
//     scope. That is the "start with conservative invalidation" posture WI-2
//     asks for.
//
// The residual the conservative scope still accepts: a change under
// `docs/streams/**` or to STATUS.md never invalidates a witness unless the
// brief declares that artifact, and an out-of-tree environment change is seen
// only when a manifest names it.
//
// A value that is not adequately established as one of the two READS as a
// definite mismatch (`wrong-revision`) only when both tokens are themselves
// well-formed and simply differ as VALUES with no git checkout to compare
// them in; in a usable checkout, a comparison that cannot be made (the
// witness commit absent, the item's revision unresolvable) is
// `could-not-check`. Neither ever invents a match. Anything coverage cannot
// establish at all — an empty or absent tree token on either side, or a token
// shorter than minRevisionTokenLen (F1: a 1-character token matched roughly 1
// commit in 16) — resolves `could-not-check`, never `pass`
// (three-state-instrument-rule.md): "I could not establish this" must never
// collapse into "this passed" merely because there was nothing to contradict it.
//
// THE +dirty TOLERANCE (a declared decision — round-2 security S3 / correctness
// A5). verifyrun's treeSHA appends `+dirty` whenever the working tree has ANY
// uncommitted change (and `+unknown` when it cannot tell): "this SHA is a
// neighbourhood, not an address". Coverage compares such a witness by its BASE
// commit (witnessBaseRevision), on both the exact and the tree-diff path. It
// therefore credits a run over uncommitted edits as a run at that base — edits
// that may never have landed. That is a real witness-trust gap, and it is
// accepted HERE deliberately rather than closed, because:
//
//   - `+dirty` is the ORDINARY shape, not an edge case: a verify batch writes
//     one brief's Evidence before running the next brief's rows, so every
//     witness after the first in a batch is `+dirty`; refusing it would re-halt
//     the model lane exactly as round-1 F2 did;
//   - the token cannot say WHAT was dirty, so no narrower rule is computable
//     from it; narrowing it needs verifyrun to record the dirty paths — a
//     witness-format change, out of this brief's scope;
//   - it is the same witness-trust gap as the standing `F-verify-self-attest`
//     finding (the witness is self-reported by whoever ran it), not a new trust
//     grant; every other guard — Command text, Expect at the base commit, the
//     witness scope — still binds.
//
// TestCoverageDirtyWitnessToleranceIsDeclared pins it, so narrowing it later is
// a visible, deliberate change (that test and spec/lifecycle-v1.md together).
//
// THE ACCEPTANCE-DEFINITION DIGEST (2026-09-18 integration amendment, revised
// 2026-09-25 per review finding F3). "Applicable evidence must bind the exact
// subject and acceptance-definition digest; a model assessment is never an
// execution witness." Three guards implement this:
//
//   - Only a row shaped like verifyrun's own witness table (isWitnessRow: an
//     `exit=` marker AND a `sha256:` marker, each in its own cell) is ever read
//     as an execution witness. Prose — including a model's own high-confidence
//     textual assessment of a row — never matches that shape, so it can never
//     supply a missing witness; the claim stays `missing`. This is
//     TestCoverageAdviceCannotSupplyWitness.
//   - The witness's Command cell must match the Verify row's CURRENT Command
//     text byte-for-byte (normalized for whitespace) before its Result is ever
//     read. This is TestCoverageAcceptanceDigestChanged.
//   - The row's Expect text is checked too, offline, against git history:
//     coverage reads the brief file's OWN Verify row as it stood at the
//     witness's BASE commit (`git show <base>:./<path>`, verifyRowAtRevision;
//     the `+dirty`/`+unknown` suffix stripped first — round-2 F3(a)) and
//     compares its Expect text to the row's CURRENT Expect. No new field is
//     added to the witness table for this — the witness table's Runner cell
//     already binds the revision, and the revision is enough to look the row's
//     OWN historical Expect text up directly, which is more precise than a
//     digest and needs no wire-format change. This guard is NEVER skipped: when
//     the historical row cannot be read (no git history, the brief or the row
//     id absent at that commit) the claim resolves `could-not-check`, not
//     `pass` (round-2 F3(b)) — the same "uncorroborated is never pass" rule the
//     revision comparison follows. The remedy is to commit the Verify row, then
//     re-run the check. This is TestCoverageExpectChangedSinceWitnessRan,
//     TestCoverageDirtyWitnessExpectChangedIsError,
//     TestCoverageBriefAbsentAtWitnessTreeIsCouldNotCheck and
//     TestCoverageRowAbsentAtWitnessTreeIsCouldNotCheck.
//
// Either acceptance-definition guard failing resolves `error` — the "witness
// present but unparseable" case (docs/streams/graph-execution/brief-03-evidence-coverage-rule.md's
// mapping table), read as "no longer parseable AS EVIDENCE FOR THIS CLAIM".
//
// Coverage never establishes WHO ran a check (evidenceactor.go's job, and it
// stays advisory — the brief's own consumers: entry marks it out-of-scope) or a
// protected acceptance/executor identity (also out of scope here); it only
// establishes WHETHER every mandatory claim has a passing result at this
// revision.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Coverage result vocabulary — the six values Task item 1 fixes, and the
// mapping from today's witness states (Context fact, brief-03):
//
//	verifyrun pass          -> covPass
//	verifyrun fail          -> covFail
//	verifyrun could-not-run -> covCouldNotCheck
//	no witness              -> covMissing
//	witness at a different revision than the item's -> covWrongRevision
//	witness present but unparseable (incl. a stale acceptance-definition,
//	  see this file's header) -> covError
const (
	covPass          = "pass"
	covFail          = "fail"
	covMissing       = "missing"
	covError         = "error"
	covCouldNotCheck = "could-not-check"
	covWrongRevision = "wrong-revision"
)

// coverageResultOrder is the fixed ranking a Coverage's overall Released bit
// does NOT need (every non-pass result holds equally), but which coverageLine
// uses to pick the FIRST reason deterministically when several claims hold —
// same rank every run, regardless of map/slice iteration order upstream.
var coverageResultOrder = map[string]int{
	covFail:          0,
	covMissing:       1,
	covError:         2,
	covWrongRevision: 3,
	covCouldNotCheck: 4,
	covPass:          5,
}

// Claim is one mandatory claim's resolution.
type Claim struct {
	Claim    string `json:"claim"`
	Kind     string `json:"kind"` // command | review | witness | observe
	Result   string `json:"result"`
	Revision string `json:"revision,omitempty"`
	Released bool   `json:"released"`
	Reason   string `json:"reason,omitempty"`
}

// Coverage is one brief's full coverage verdict.
type Coverage struct {
	BriefID  string  `json:"brief"`
	Released bool    `json:"released"`
	Claims   []Claim `json:"claims"`
}

// coverageBinding is the (pattern, node) a brief is instantiated at — supplied
// by a caller (today: only a test; production has no adapter yet, see this
// file's header).
type coverageBinding struct {
	Pattern string
	Node    string
}

// coverageOptions is evaluateCoverage's optional input set.
type coverageOptions struct {
	// Bindings maps a brief id ("<stream>/<NN>") to the pattern node it is
	// instantiated at. nil/absent entries use Task item 1's fact-(a)-alone path.
	Bindings map[string]coverageBinding
	// Patterns is the parsed pattern-file set, keyed by `pattern:` name
	// (loadPatternDocs). Consulted only for a brief present in Bindings.
	Patterns map[string]patternDoc
	// Revision is the item's revision to compare witnesses against. "" resolves
	// to the current tree's HEAD SHA (currentTreeRevision) — see this file's
	// header ("THE REVISION COMPARISON IS OFFLINE") for how that relates to
	// "merged SHA / PR head" and why it is not the literal merge SHA.
	Revision string
	// Dependencies maps a brief id to its work-input dependency manifest (the
	// work-input amendment, WI-2). The later graph-execution/09 adapter
	// supplies it; production passes none today, so every claim takes the
	// conservative scope. See this file's header ("WORK-INPUT DEPENDENCIES").
	Dependencies map[string]dependencyManifest
}

// Work-input dependency kinds (WI-2): what a claim's result depends on
// besides the Verify row text itself.
const (
	depSource      = "source"      // repo code the claim exercises; checked by the tree diff
	depPolicy      = "policy"      // an authority or policy file; checked by fingerprint
	depBuild       = "build"       // build configuration or a generated input; checked by fingerprint
	depEnvironment = "environment" // a named out-of-tree value; checked by fingerprint
)

// dependencyManifest is one brief's declared work-input dependencies.
// Complete is the manifest's own statement that it names EVERY dependency of
// every claim. Only a complete manifest licenses selective reuse (a scope
// narrower than the conservative one). An incomplete manifest's entries are
// still checked, but its claims also keep the conservative scope.
type dependencyManifest struct {
	Complete bool
	Deps     []workInputDep
}

// workInputDep is one dependency. Rows lists the Verify row ids it applies
// to; empty means every row.
//
//   - source, policy, build: Path is repo-relative (a file or a directory).
//     A source dependency joins the claim's witness scope. A policy or build
//     dependency is compared by fingerprint: the git object id of Path at the
//     witness's base commit against the one at the item's revision.
//   - environment: Name identifies the value; Recorded is its fingerprint when
//     the witness ran and Current is its fingerprint now. Either one empty
//     means the dependency cannot be checked.
type workInputDep struct {
	Kind     string
	Path     string
	Name     string
	Recorded string
	Current  string
	Rows     []string
}

// appliesTo reports whether d is a dependency of Verify row rowID.
func (d workInputDep) appliesTo(rowID string) bool {
	if len(d.Rows) == 0 {
		return true
	}
	for _, r := range d.Rows {
		if r == rowID {
			return true
		}
	}
	return false
}

// label names d in a reason string.
func (d workInputDep) label() string {
	if d.Kind == depEnvironment {
		return fmt.Sprintf("%s dependency %q", d.Kind, d.Name)
	}
	return fmt.Sprintf("%s dependency %s", d.Kind, d.Path)
}

// ruleCoverageJoinMissingFlow is the stable [rule-tag] for the join's missing
// integration-check claim (Task item 2), printed in the same bracketed form
// every other rule tag in this binary uses (rulePattern*, ruleEligibility*).
const ruleCoverageJoinMissingFlow = "coverage-join-missing-integration-check"

// evaluateCoverage computes the coverage verdict for every brief across
// streams. root anchors the offline revision resolution (see header).
func evaluateCoverage(root string, streams []*Stream, opts coverageOptions) map[string]Coverage {
	revision := opts.Revision
	if revision == "" {
		revision = currentTreeRevision(root)
	}

	out := map[string]Coverage{}
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
			id := s.Name + "/" + num
			out[id] = evaluateOneCoverage(root, path, id, bf, opts, revision)
		}
	}
	return out
}

// evaluateOneCoverage resolves one brief's claim set and Released verdict. It
// reads the brief's Verify/Evidence bodies off the already-parsed BriefFile
// (bf.Verify / bf.Evidence, populated by parseBriefFile for every schema
// version) rather than re-reading the file — the same data checkBriefFiles and
// autoflip.go's Evidence-contradiction read already use.
func evaluateOneCoverage(root, briefPath, id string, bf *BriefFile, opts coverageOptions, revision string) Coverage {
	evidence := parseEvidenceRows(bf.Evidence)
	verifyRows := briefVerifyRows(bf.Verify)

	var claims []Claim

	// (a) every Verify row is a mandatory claim by construction.
	var man *dependencyManifest
	if m, ok := opts.Dependencies[id]; ok {
		man = &m
	}
	scope := newWitnessScope(root, briefPath, bf, man)
	for _, r := range verifyRows {
		result, reason, claimRev := resolveVerifyClaim(root, scope.forVerifyRow(r), r, evidence, revision)
		claims = append(claims, Claim{
			Claim:    fmt.Sprintf("Verify row #%s: %s", r.ID, r.Command),
			Kind:     "command",
			Result:   result,
			Revision: claimRev,
			Released: result == covPass,
			Reason:   reason,
		})
	}

	// (b) the pattern node's own mandatory evidence, when this brief is bound to
	// one (coverageBinding — see this file's header on why bindings are opt-in).
	var atJoin bool
	if binding, ok := opts.Bindings[id]; ok {
		if pat, ok := opts.Patterns[binding.Pattern]; ok {
			if node, ok := pat.NodeByID[binding.Node]; ok {
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
						Released: result == covPass,
						Reason:   reason,
					})
				}
				atJoin = binding.Node == pat.Join
			}
		}
	}

	// The join's integration check (Task item 2): at least one Verify row
	// classed `+flow` — individually passing rows do not release the join.
	if atJoin {
		hasFlow := false
		for _, r := range verifyRows {
			for _, ob := range r.Obligations {
				if ob == classFlow {
					hasFlow = true
					break
				}
			}
		}
		if !hasFlow {
			claims = append(claims, Claim{
				Claim:  "integration-check",
				Kind:   "command",
				Result: covMissing,
				Reason: fmt.Sprintf("[%s] the pattern's join node requires at least one Verify row classed `+flow` — individually passing rows do not release the join", ruleCoverageJoinMissingFlow),
			})
		}
	}

	if len(claims) == 0 {
		// Fail-closed (patterns.go / the three-state-instrument-rule convention
		// this whole file follows): a brief with no Verify rows and no pattern
		// binding has nothing coverage can corroborate, so it is HELD, not
		// vacuously released.
		claims = append(claims, Claim{
			Claim: "mandatory claims", Kind: "command", Result: covMissing,
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

// resolveVerifyClaim resolves ONE Verify row's mandatory claim against the
// brief's Evidence section, in the order the mapping table (this file's header)
// fixes. root and scope (the brief's path plus the paths its witnesses speak
// for) anchor the offline git-history reads classifyRevision and
// verifyRowAtRevision use; either may be empty (a bare testdata copy), which
// those functions treat as "nothing to corroborate", never a match.
func resolveVerifyClaim(root string, scope witnessScope, r verifyRow, evidence map[string][]evidenceRow, targetRevision string) (result, reason, revision string) {
	var latestText string
	for _, er := range evidence[r.ID] {
		if isWitnessRow(er.Text) {
			latestText = er.Text // last witness for the row wins, same as checkWitnesses
		}
	}
	if latestText == "" {
		return covMissing, "no execution witness in Evidence — run `statusgen verifyrun --brief <path>`", ""
	}

	// The acceptance-definition digest guard, Command half (this file's
	// header): the witness must still describe the CURRENT Command text, or it
	// is not evidence for this claim any more.
	if normalizeCommandText(witnessCommandOf(latestText)) != normalizeCommandText(r.Command) {
		return covError, "the witness records a different command than the Verify row now carries — its acceptance definition changed after it was run, so the recorded result proves nothing about the row as it stands today", ""
	}

	wrev := witnessTreeOf(latestText)
	switch witnessStateOf(latestText) {
	case statePass:
		// A pass recorded (by an older binary) on a row now flagged prose-led
		// (#1808) measured the mention, not the check — checkWitnesses demotes
		// the same witness to could-not-run, and coverage must agree: the pass
		// proves nothing, so it is never credited (security review S12).
		if r.ProseLed != "" {
			return covCouldNotCheck, proseLedNote(r.ProseLed), wrev
		}
		rel, exact, detail := classifyRevisionDetail(root, scope, wrev, targetRevision)
		switch rel {
		case revisionUnestablished:
			if detail != "" {
				return covCouldNotCheck, fmt.Sprintf("the witness ran at %s and cannot be reused at the item's revision %s: %s — a passing result is never credited while its dependencies cannot be established", displayRevision(wrev), displayRevision(targetRevision), detail), wrev
			}
			return covCouldNotCheck, fmt.Sprintf("the witness's revision could not be corroborated against the item's revision (witness=%s, item=%s) — a passing result is never credited without knowing which revision it ran at", displayRevision(wrev), displayRevision(targetRevision)), wrev
		case revisionMismatch:
			reason := fmt.Sprintf("witness ran at revision %s, the item's revision is %s", wrev, targetRevision)
			if detail != "" {
				reason += "; " + detail + ", so the old result cannot release the new revision"
			}
			return covWrongRevision, reason, wrev
		}
		// revisionMatch: the witness's tree is, or offline-corroborates as, the
		// item's revision. The acceptance-definition digest guard, Expect half
		// (this file's header, F3): an Expect-only tightening since the witness
		// ran is invisible to the Command check above, so read the row's OWN
		// historical Expect text at the witness's BASE commit (the `+dirty` /
		// `+unknown` suffix stripped — round-2 F3(a): `git show <sha>+dirty:…`
		// never resolves) and compare it to the row's current Expect. A row
		// that cannot be read there — no history, the brief or the row id absent
		// at that commit — is could-not-check, never a pass (round-2 F3(b)).
		base := witnessBaseRevision(wrev)
		hist, ok := verifyRowAtRevision(root, scope.briefPath, base, r.ID)
		if !ok {
			return covCouldNotCheck, fmt.Sprintf("the Verify row #%s could not be read as it stood at the witness's revision %s (no git history, or the brief or that row did not exist there) — a pass is never credited without confirming the witness answered the question the row asks today; commit the row, then re-run the check", r.ID, displayRevision(base)), wrev
		}
		if normalizeCommandText(hist.Expect) != normalizeCommandText(r.Expect) {
			return covError, "the row's Expect text changed after the witness ran — its acceptance definition changed, so the recorded result proves nothing about the row as it stands today", wrev
		}
		// The work-input dependencies (WI-2): a changed policy, build or
		// environment dependency holds the claim even when the tree diff is
		// clean for its scope. Checked on the exact path too, because an
		// environment value can move without any commit.
		fingerprinted := 0
		for _, d := range scope.deps {
			if d.Kind == depSource {
				continue
			}
			same, unknown := depFingerprintSame(root, d, base, witnessBaseRevision(targetRevision))
			if unknown != "" {
				return covCouldNotCheck, fmt.Sprintf("the %s could not be fingerprinted (%s) — a pass is never credited while a dependency of the claim is unknown", d.label(), unknown), wrev
			}
			if !same {
				return covWrongRevision, fmt.Sprintf("the %s changed since the witness ran at %s; the claim is held until it is revalidated at %s, and the old receipt keeps its revision", d.label(), wrev, targetRevision), wrev
			}
			fingerprinted++
		}
		if fingerprinted > 0 {
			detail += fmt.Sprintf("; %d policy, build or environment fingerprint(s) unchanged", fingerprinted)
		}
		if exact {
			if fingerprinted > 0 {
				return covPass, "witness matches the row and passed" + detail, wrev
			}
			return covPass, "witness matches the row and passed", wrev
		}
		return covPass, fmt.Sprintf("witness ran at %s and is reused at %s: %s; the receipt keeps revision %s", wrev, targetRevision, detail, wrev), wrev
	case stateFail:
		return covFail, "the witness records a failure", wrev
	case stateCouldNotRun:
		return covCouldNotCheck, "the witness records could-not-run — the row produced no verdict", wrev
	default:
		// A row shaped enough to pass witnessCells (an exit= marker and a
		// sha256: marker each in their own cell) but whose Result cell carries
		// none of pass/fail/could-not-run — genuinely unparseable.
		return covError, "the witness row's Result cell could not be parsed to a known state", wrev
	}
}

// displayRevision renders a possibly-absent revision token for a reason
// string — "(none recorded)" rather than a bare empty pair of quotes.
func displayRevision(rev string) string {
	if rev == "" {
		return "(none recorded)"
	}
	return rev
}

// resolvePatternEvidenceClaim resolves a pattern node's mandatory evidence
// entry that is NOT a Verify row — review/witness/observe claims a pattern
// node owes on top of (or instead of) the brief's own Verify table.
//
// review/witness: this file does not itself corroborate a live reviewer-App
// review object or a fresh execution witness beyond what the brief's own
// Verify/Evidence sections already supply — that is autoflip.go's / verifyrun's
// job, and duplicating it here would be a second, divergent reader of the same
// fact. Absent a pattern-level binding mechanism that supplies one (see this
// file's header), a review/witness-kind pattern claim resolves could-not-check:
// an unestablished corroboration is reported as itself, never rounded to pass.
//
// observe (Task item 3): filled from the named `source`. An unreadable source
// is could-not-check, by construction — the spec text this implements:
// "an unreadable source → could-not-check → held". `source` is read as a
// repo-relative path under --root's tree by convention (kept intentionally
// simple: population/period export and any real signal-reading protocol is
// brief-15's control-evidence scope, not this one's — see the integration
// amendment, "Control-profile population/period export belongs to 15").
func resolvePatternEvidenceClaim(ev patternEvidence) (result, reason string) {
	switch ev.Kind {
	case "observe":
		if ev.Source == "" {
			return covCouldNotCheck, "observe evidence declares no `source` to read the signal from"
		}
		return covCouldNotCheck, fmt.Sprintf("observe source %q not corroborated by this run — no reader is wired for it yet", ev.Source)
	default:
		return covCouldNotCheck, fmt.Sprintf("%s-kind pattern evidence %q is not corroborated by a binding mechanism yet (graph-execution/09)", ev.Kind, ev.Claim)
	}
}

// currentTreeRevision is the offline stand-in for "the item's revision" (this
// file's header): the HEAD SHA of the tree currently checked out at root,
// truncated to the same treeSHALen a witness itself records, so the two
// compare like-for-like. "" when root carries no git history to read (a bare
// testdata copy) — callers must treat that as "nothing to compare", never a
// match or a mismatch.
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

// revisionRelation is classifyRevision's three-state verdict — deliberately
// not a bool: "the two differ" and "we could not tell" must never collapse
// into the same answer (three-state-instrument-rule.md).
type revisionRelation int

const (
	revisionMatch revisionRelation = iota
	revisionMismatch
	revisionUnestablished
)

// minRevisionTokenLen is the shortest tree token classifyRevision ever treats
// as informative. Review finding F1: a 1-character token matched roughly 1 in
// 16 real SHAs by coincidence — treeSHALen is the same floor every witness
// comparison in this binary already uses for a positive match, so a token
// shorter than that proves nothing either way.
const minRevisionTokenLen = treeSHALen

// hexRevisionRe is the shape classifyRevision requires of a witness token
// (after witnessBaseRevision strips its `+dirty`/`+unknown` suffix) before it
// is ever passed to git as a revision expression (security pr1682-S8).
// `witnessTreeOf` lifts the Runner cell's `@ <tree>` token as free text, and
// git accepts far more than a commit SHA as a "revision" — an ancestry suffix
// (`HEAD~0`), a full ref name, an abbreviated ref — so a hand-edited Evidence
// row naming one of those can resolve to whatever the checked-out HEAD
// happens to be and stay `pass` forever: a symbolic token never goes stale
// the way a forged-but-real hex SHA does (it goes `wrong-revision` on the
// next in-scope change). `verifyrun` itself only ever writes a lowercase hex
// SHA (treeSHALen, §"treeSHALen" in verifyrun.go), so this shape check
// rejects nothing a real witness ever produces.
var hexRevisionRe = regexp.MustCompile(`^[0-9a-fA-F]{12,40}$`)

// witnessBaseRevision strips the `+dirty` / `+unknown` suffix verifyrun's
// treeSHA appends, leaving the commit the witness ran on top of. The suffix
// records the witness run's OWN cleanliness, not a different identity — see
// this file's header ("THE +dirty TOLERANCE") for what that does and does not
// license.
func witnessBaseRevision(tok string) string {
	return strings.TrimSuffix(strings.TrimSuffix(tok, "+dirty"), "+unknown")
}

// classifyRevision compares a witness's recorded tree token against the
// item's revision, by the witness's base commit (witnessBaseRevision). See
// this file's header ("THE REVISION COMPARISON IS OFFLINE") for the full
// rationale; in short:
//
//   - an empty, absent, or too-short token on EITHER side is unestablished —
//     never a match, never a mismatch;
//   - a same-length-prefix equal-fold match is a match, unconditionally;
//   - otherwise witnessTreeApplies compares the two trees: a match when the
//     two commits share history and no path the witness speaks for (scope)
//     differs — the ordinary shape of "run the check, then commit the
//     Evidence that records it, then keep landing unrelated work on main"
//     (F2), and of a witness written on a branch that was then squash-merged
//     (#2026); a mismatch naming the first differing path it speaks for, or
//     the absence of shared history; unestablished when the comparison cannot
//     be made. Without a git checkout the plain mismatch stands; nothing ever
//     invents a match.
func classifyRevision(root string, scope witnessScope, witnessTree, target string) revisionRelation {
	rel, _, _ := classifyRevisionDetail(root, scope, witnessTree, target)
	return rel
}

// classifyRevisionDetail is classifyRevision plus how the verdict was reached.
// exact is true for a same-token match, where nothing is being reused; it is
// false for a tree-diff match, which IS a reuse. detail is witnessTreeApplies's
// account: on a match, the applicability derivation that licenses the reuse;
// on a mismatch, the first differing path the witness speaks for; on
// unestablished, why the comparison could not be made.
func classifyRevisionDetail(root string, scope witnessScope, witnessTree, target string) (rel revisionRelation, exact bool, detail string) {
	w := witnessBaseRevision(witnessTree)
	t := witnessBaseRevision(target)
	if w == "" || t == "" || w == "no-git" || t == "no-git" {
		return revisionUnestablished, false, ""
	}
	if len(w) < minRevisionTokenLen || len(t) < minRevisionTokenLen {
		return revisionUnestablished, false, ""
	}
	if !hexRevisionRe.MatchString(w) || !hexRevisionRe.MatchString(t) {
		return revisionUnestablished, false, ""
	}
	n := len(w)
	if len(t) < n {
		n = len(t)
	}
	if strings.EqualFold(w[:n], t[:n]) {
		return revisionMatch, true, ""
	}
	// A `+dirty` / `+unknown` witness ran on uncommitted edits over w: the
	// landing check below compares w's tree, never the tree the row read, so
	// it is no layer behind a narrowed scope. Such a witness keeps the
	// conservative scope (#2026: narrowing only where both layers bind).
	landedNoun := "the witness's tree"
	if w != witnessTree {
		landedNoun = "the witness's base commit (not the uncommitted edits its " + strings.TrimPrefix(witnessTree, w) + " token marks)"
		if scope.derived() {
			scope.inputs = nil
			scope.conservative = true
			scope.whyConservative = "no dependency manifest, and the witness ran on a working tree with uncommitted edits (" + strings.TrimPrefix(witnessTree, w) + "), whose tree no commit holds, so the landing check cannot stand behind a scope narrowed to the row's derived inputs"
		}
	}
	rel, detail = witnessTreeApplies(root, scope, w, t, landedNoun)
	return rel, false, detail
}

// witnessScope is what a brief's witnesses SPEAK FOR — the paths whose change
// after the witness ran means the witness no longer describes the item
// (round-2 F2, round-3 F2/F6). See this file's header ("THE REVISION
// COMPARISON IS OFFLINE").
type witnessScope struct {
	briefPath string   // absolute path of the brief file ("" = none)
	briefRel  string   // briefPath relative to root, slash-separated
	declared  []string // the brief's `files:` entries (normalized) plus, per row, its source dependencies; nil = none
	// manifest is the brief's work-input dependency manifest (nil = none
	// supplied). deps are its entries that apply to the row being resolved
	// (set by forRow).
	manifest *dependencyManifest
	deps     []workInputDep
	// conservative selects the no-declaration scope: every path outside the
	// board's bookkeeping surface. It is set unless a COMPLETE dependency
	// manifest licenses narrowing (WI-2), when the brief and row declare
	// nothing, and, by atBase, whenever a declaration cannot be trusted to
	// name the surface (round-3 F6). It only ever WIDENS the scope.
	conservative bool
	// whyConservative says which of those set conservative, for the reason.
	whyConservative string
	// inputs, when set (and conservative is not), are the repo paths the
	// row's command reads, derived from its text (forVerifyRow,
	// coverage_inputs.go; #2026 cause 2): the witness speaks for exactly
	// these. Only set when no dependency manifest is supplied.
	inputs []string
}

// derived reports whether the scope is a row's derived read set.
func (sc witnessScope) derived() bool {
	return !sc.conservative && len(sc.inputs) > 0
}

// newWitnessScope builds a brief's witnessScope from its CURRENT parsed `files:`
// line (BriefFile.DeclaredEntriesRaw — every entry the label names, including a
// dotless one the mistake-proofing/01 path-shape filter drops; round-3 F6 /
// security pr1682-S6) and its dependency manifest (nil = none). This is only
// the current, brief-level half: forRow adds the row's own dependencies, and
// witnessTreeApplies widens the result with atBase before reading a diff.
func newWitnessScope(root, briefPath string, bf *BriefFile, man *dependencyManifest) witnessScope {
	sc := witnessScope{briefPath: briefPath, manifest: man}
	if root != "" && briefPath != "" {
		if rel, err := filepath.Rel(root, briefPath); err == nil {
			sc.briefRel = filepath.ToSlash(rel)
		}
	}
	if bf != nil && bf.DeclaredEntriesRawFound {
		sc.declared = appendDeclaredEntries(nil, bf.DeclaredEntriesRaw)
	}
	return sc.forRow("")
}

// forRow returns the scope for Verify row rowID: the brief's `files:` entries
// plus that row's source dependencies, with the row's policy, build and
// environment dependencies attached for fingerprinting. Without a complete
// manifest the scope stays conservative (WI-2: "incomplete dependency
// knowledge requires broader revalidation"; "file non-overlap alone is
// insufficient"). The declared entries still count on top of it, so a
// declared docs/streams artifact stays guarded.
func (sc witnessScope) forRow(rowID string) witnessScope {
	out := sc
	out.declared = append([]string(nil), sc.declared...)
	out.deps = nil
	if sc.manifest != nil && rowID != "" {
		for _, d := range sc.manifest.Deps {
			if !d.appliesTo(rowID) {
				continue
			}
			out.deps = append(out.deps, d)
			if d.Kind == depSource {
				out.declared = appendDeclaredEntries(out.declared, []string{d.Path})
			}
		}
	}
	switch {
	case sc.manifest == nil:
		out.conservative, out.whyConservative = true, "no dependency manifest"
	case !sc.manifest.Complete:
		out.conservative, out.whyConservative = true, "the dependency manifest is incomplete"
	case len(out.declared) == 0:
		out.conservative, out.whyConservative = true, "nothing is declared"
	default:
		out.conservative, out.whyConservative = false, ""
	}
	return out
}

// describe says what the scope covers, for a wrong-revision reason.
func (sc witnessScope) describe() string {
	if sc.derived() {
		return "the row's command reads only " + strings.Join(sc.inputs, ", ") + " (derived from its text)"
	}
	if sc.conservative {
		return sc.whyConservative + ", so the witness speaks for every path outside docs/streams/** and STATUS.md"
	}
	return "a complete dependency manifest scopes the witness to " + strings.Join(sc.declared, ", ")
}

// derivation is the applicability derivation that licenses reusing a witness
// at a later revision (WI-2), for a reused pass's reason.
func (sc witnessScope) derivation() string {
	if sc.derived() {
		return "none of the paths the row's command reads (" + strings.Join(sc.inputs, ", ") + "; derived from its text) differs between the witness's tree and the item's"
	}
	if sc.conservative {
		return "no path outside docs/streams/** and STATUS.md differs between the witness's tree and the item's (" + sc.whyConservative + ", so that whole surface is the input)"
	}
	return "applicability derived from a complete dependency manifest: none of " + strings.Join(sc.declared, ", ") + " differs between the witness's tree and the item's"
}

// appendDeclaredEntries normalizes and de-duplicates declared `files:` entries
// onto dst: a leading `./` is dropped and a trailing `/**` reads as the
// directory it names (`src/**` covers everything under src/, which is what the
// author meant and what the prefix match below already implements).
func appendDeclaredEntries(dst []string, entries []string) []string {
	seen := map[string]bool{}
	for _, d := range dst {
		seen[d] = true
	}
	for _, d := range entries {
		d = strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(d)), "./")
		if strings.HasSuffix(d, "/**") && !strings.Contains(strings.TrimSuffix(d, "/**"), "**") {
			d = strings.TrimSuffix(d, "**")
		}
		if d == "" || seen[d] {
			continue
		}
		seen[d] = true
		dst = append(dst, d)
	}
	return dst
}

// atBase returns the scope a witness whose BASE commit is base actually speaks
// for, read against the item's revision target (round-3 F6):
//
//   - the union of the brief's `files:` declaration NOW and AS IT STOOD AT base
//     (read from git history, the same way verifyRowAtRevision reads the Verify
//     row), so a `files:` line narrowed after the witness ran — in the brief's
//     own, otherwise exempt, file — can never shrink what the witness speaks
//     for. A brief with no declaration at base, or one that cannot be read
//     there, contributes the conservative scope;
//   - the conservative scope as well whenever ANY declared entry does not
//     resolve to a real path at base or at target (declaredEntryResolves): a
//     brace form, `.`, prose such as `n/a`, a bare sibling name, a `**` in the
//     middle of a glob. An entry that names nothing would otherwise speak for
//     nothing, which is LESS conservative than no declaration at all.
//
// It only ever widens sc; the resolved declared entries still count on top of
// the conservative scope (they keep a declared docs/streams artifact guarded).
func (sc witnessScope) atBase(root, base, target string) witnessScope {
	eff := sc
	eff.declared = append([]string(nil), sc.declared...)
	widen := func(why string) {
		if !eff.conservative {
			eff.conservative, eff.whyConservative = true, why
		}
	}
	if body, ok := briefBodyAtRevision(root, sc.briefPath, base); !ok {
		widen("the brief cannot be read at the witness's commit")
	} else if entries, found := extractContextDeclaredEntriesRaw(body); !found {
		widen("the brief declared no parseable files: at the witness's commit")
	} else {
		eff.declared = appendDeclaredEntries(eff.declared, entries)
	}
	for _, d := range eff.declared {
		if !sc.declaredEntryResolves(root, d, base, target) {
			widen(fmt.Sprintf("the declared entry %q names no checkable file", d))
			break
		}
	}
	return eff
}

// isBoardBookkeepingPath reports whether a repo-relative path is the board's
// own bookkeeping surface — stream docs (brief Evidence, READMEs, verify
// outcome logs, findings) and the generated STATUS.md. A verify batch lands
// Evidence for several briefs in one commit, and statusgen regenerates
// STATUS.md after every push, so this surface moves between ANY witness and
// the main tip the model-lane flip runs at (review finding F2, round 2). It is
// what the CONSERVATIVE scope leaves out.
func isBoardBookkeepingPath(p string) bool {
	return p == "STATUS.md" || strings.HasPrefix(p, "docs/streams/")
}

// isVerifyWrittenPath reports whether p is one of the files verify and regen
// NECESSARILY write between a witness and the main tip — so no witness can
// speak for it, even when a brief's `files:` names it (round-3 F2): the
// generated STATUS.md, the verify outcome log and its rotation shards, a
// stream's README (its status rows flip on every verified/done), and brief
// files (a verify batch writes several briefs' Evidence in one commit). Any
// OTHER docs/streams/ artifact a brief declares stays guarded.
func isVerifyWrittenPath(p string) bool {
	if p == "STATUS.md" {
		return true
	}
	for _, pat := range []string{
		"docs/streams/" + verifyOutcomesGlob,
		"docs/streams/*/README.md",
		"docs/streams/*/brief-*.md",
	} {
		if m, err := pathpkg.Match(pat, p); err == nil && m {
			return true
		}
	}
	return false
}

// invalidatedBy reports whether a change to repo-relative path p, after the
// witness ran, means the witness no longer speaks for the item:
//
//   - the brief's OWN file never does — its Verify rows are bound separately,
//     by the Command and Expect guards (this file's header);
//   - in the conservative scope, every path outside the board's bookkeeping
//     surface (isBoardBookkeepingPath) does;
//   - the files verify and regen necessarily write (isVerifyWrittenPath) never
//     do, declared or not;
//   - otherwise a declared entry does (declaredEntryMatches: a declared
//     directory covers everything under it; a glob matches a path or any of
//     its parent directories).
func (sc witnessScope) invalidatedBy(p string) bool {
	p = filepath.ToSlash(strings.TrimSpace(p))
	if p == "" || p == sc.briefRel {
		return false
	}
	if sc.derived() {
		for _, in := range sc.inputs {
			if inputCovers(in, p) {
				return true
			}
		}
		return false
	}
	if sc.conservative && !isBoardBookkeepingPath(p) {
		return true
	}
	if isVerifyWrittenPath(p) {
		return false
	}
	for _, d := range sc.declared {
		if declaredEntryMatches(d, p) {
			return true
		}
	}
	return false
}

// declaredEntryMatches reports whether declared entry d covers repo-relative
// path p: p is d, p is under d as a directory, or d (as a path.Match glob)
// matches p or one of p's parent directories.
func declaredEntryMatches(d, p string) bool {
	dd := strings.TrimSuffix(d, "/")
	if dd != "" && (p == dd || strings.HasPrefix(p, dd+"/")) {
		return true
	}
	for q := p; q != "" && q != "." && q != "/"; q = pathpkg.Dir(q) {
		if m, err := pathpkg.Match(d, q); err == nil && m {
			return true
		}
	}
	return false
}

// declaredEntrySupported reports whether d is an entry declaredEntryMatches can
// actually express. `.`, an absolute or parent-relative path, brace syntax
// (which path.Match lacks — and which the `files:` parser has already split at
// its comma), a `**` that appendDeclaredEntries did not rewrite, and a
// malformed glob are not: such an entry would silently match less than its
// author meant.
func declaredEntrySupported(d string) bool {
	if d == "" || d == "." || strings.HasPrefix(d, "/") || d == ".." || strings.HasPrefix(d, "../") {
		return false
	}
	if strings.ContainsAny(d, "{}") || strings.Contains(d, "**") {
		return false
	}
	if _, err := pathpkg.Match(d, "x"); err != nil {
		return false
	}
	return true
}

// declaredEntryResolves reports whether d is supported AND covers at least one
// real file in the tree at base or at target that a change can actually
// invalidate — a declaration that names nothing is not evidence of what the
// witness exercised. A file no witness can speak for (isVerifyWrittenPath, or
// the brief's own file) does not count (security S6): a `files:` line naming
// only `STATUS.md` would otherwise narrow the scope to nothing at all.
func (sc witnessScope) declaredEntryResolves(root, d, base, target string) bool {
	if !declaredEntrySupported(d) {
		return false
	}
	for _, rev := range []string{base, target} {
		for _, f := range treeFilesAt(root, rev) {
			if f == sc.briefRel || isVerifyWrittenPath(f) {
				continue
			}
			if declaredEntryMatches(d, f) {
				return true
			}
		}
	}
	return false
}

// depFingerprintSame compares a policy, build or environment dependency's
// fingerprint when the witness ran (at base) against now (at target). unknown
// is non-empty when either side cannot be read; the caller then resolves
// could-not-check, never pass.
func depFingerprintSame(root string, d workInputDep, base, target string) (same bool, unknown string) {
	switch d.Kind {
	case depEnvironment:
		if d.Name == "" || d.Recorded == "" || d.Current == "" {
			return false, "no recorded or current value"
		}
		return d.Recorded == d.Current, ""
	case depPolicy, depBuild:
		p := strings.TrimSuffix(strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(d.Path)), "./"), "/")
		if !declaredEntrySupported(p) || strings.ContainsAny(p, "*?[") {
			return false, "not a plain repo-relative path"
		}
		was, okW := gitObjectAt(root, base, p)
		now, okN := gitObjectAt(root, target, p)
		if !okW || !okN {
			return false, "git history unreadable"
		}
		if was == "" && now == "" {
			return false, "the path exists at neither revision"
		}
		return was == now, ""
	default:
		return false, fmt.Sprintf("unknown dependency kind %q", d.Kind)
	}
}

// gitObjectAt returns the git object id of path p in rev's tree ("" when p is
// absent there). ok is false when the lookup itself could not run.
func gitObjectAt(root, rev, p string) (oid string, ok bool) {
	if root == "" || rev == "" {
		return "", false
	}
	out, err := coverageGit(root, "ls-tree", "-z", "--full-tree", "--end-of-options", rev, "--", p).Output()
	if err != nil {
		return "", false
	}
	for _, e := range splitNUL(out) {
		meta, name, found := strings.Cut(e, "\t")
		if !found || name != p {
			continue
		}
		if f := strings.Fields(meta); len(f) == 3 {
			return f[2], true
		}
	}
	return "", true
}

// treeFilesCache memoizes treeFilesAt per (root, commit): a commit's tree never
// changes, and a --coverage run asks for the same few revisions once per brief.
var (
	treeFilesMu    sync.Mutex
	treeFilesCache = map[string][]string{}
)

// treeFilesAt lists every file path in rev's tree (`git ls-tree -r -z`, so a
// path with unusual bytes is never quoted into something no declaration
// matches). nil when root or rev does not resolve — the caller then finds no
// match, which widens the scope, never narrows it.
func treeFilesAt(root, rev string) []string {
	if root == "" || rev == "" {
		return nil
	}
	key := root + "\x00" + rev
	treeFilesMu.Lock()
	files, ok := treeFilesCache[key]
	treeFilesMu.Unlock()
	if ok {
		return files
	}
	out, err := coverageGit(root, "ls-tree", "-r", "-z", "--name-only", "--full-tree", "--end-of-options", rev).Output()
	if err != nil {
		return nil
	}
	files = splitNUL(out)
	treeFilesMu.Lock()
	treeFilesCache[key] = files
	treeFilesMu.Unlock()
	return files
}

// splitNUL splits git `-z` output into its non-empty entries.
func splitNUL(out []byte) []string {
	var parts []string
	for _, f := range strings.Split(string(out), "\x00") {
		if f != "" {
			parts = append(parts, f)
		}
	}
	return parts
}

// witnessTreeApplies decides whether a witness recorded at commit
// witnessTree still applies at the item's revision target, when the two are
// not the same commit. It judges by CONTENT, not by commit ancestry (#2026):
// the two trees are compared directly (`git diff witnessTree target`), and the
// witness applies when no path it speaks for (atBase's scope: a complete
// dependency manifest's declared scope, or the conservative scope of every
// path outside the board's bookkeeping) differs between them. Ancestry is
// deliberately not required — a squash merge discards the branch commit a
// witness names, so "is it an ancestor of main" says nothing about whether
// the code the check read is still the code at main; the tree comparison
// does. TestNoAncestryWitnessJudge is the class guard that keeps it that way.
//
// The witness commit must still share history with the item's revision (a
// common ancestor, `git merge-base`): a commit from an unrelated root is not
// a witness for this work, whatever its tree holds, and is refused. Both
// tokens are resolved once to full object ids (resolveCommitToken), and must
// resolve AS object ids — a hex-shaped ref name is refused — so every later
// read names the same commit.
//
// Results: revisionMatch with the applicability derivation; revisionMismatch
// naming the first differing path the witness speaks for, or the absence of
// shared history; and revisionUnestablished, with the reason, whenever the
// comparison itself cannot be made in a usable git checkout (the witness
// commit is not in the object store — the ordinary shape of a squash-merged
// branch commit that was never fetched — a token is not a commit, the item's
// revision does not resolve, or git fails). Where there is no usable git
// checkout at all the two plainly different tokens stay a mismatch, as
// before.
func witnessTreeApplies(root string, scope witnessScope, witnessTree, target, landedNoun string) (rel revisionRelation, detail string) {
	if root == "" || scope.briefRel == "" {
		return revisionMismatch, ""
	}
	if coverageGit(root, "rev-parse", "--git-dir").Run() != nil {
		return revisionMismatch, ""
	}
	t, why := resolveCommitToken(root, target)
	if why != "" {
		return revisionUnestablished, fmt.Sprintf("the item's revision %s %s", target, why)
	}
	w, why := resolveCommitToken(root, witnessTree)
	if why == tokenAbsent {
		why += " (a squash merge discards the branch commit a witness names; fetch it, e.g. the pull request's head ref), so the tree the check ran against cannot be compared"
	}
	if why != "" {
		return revisionUnestablished, fmt.Sprintf("the witness's commit %s %s", witnessTree, why)
	}
	if why := shallowCloneWhy(root); why != "" {
		return revisionUnestablished, why
	}
	if err := coverageGit(root, "merge-base", "--end-of-options", w, t).Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() == 1 {
			return revisionMismatch, fmt.Sprintf("the witness's commit %s shares no history with the item's revision %s (no common ancestor), so it is not a witness for this work", witnessTree, target)
		}
		return revisionUnestablished, "the witness's commit and the item's revision could not be related (git merge-base failed)"
	}
	eff := scope
	if scope.derived() {
		eff, why = scope.derivedAtBase(root, w, t)
		if why != "" {
			return revisionUnestablished, why
		}
	}
	if !eff.derived() {
		eff = eff.atBase(root, w, t)
	}
	// --no-renames (round-3 F6): with rename detection on, a rename or move
	// prints only its DESTINATION, so a declared file renamed away — or code
	// moved into docs/streams/ — never showed up as a change to its old path.
	// -z keeps every path byte-exact (no core.quotePath quoting).
	out, err := coverageGit(root, "diff", "--name-only", "--no-renames", "-z", "--end-of-options", w, t, "--").Output()
	if err != nil {
		return revisionUnestablished, "the witness's tree and the item's could not be compared (git diff failed)"
	}
	for _, p := range splitNUL(out) {
		if eff.invalidatedBy(p) {
			return revisionMismatch, fmt.Sprintf("%s differs between the witness's tree and the item's and the witness speaks for it (%s)", p, eff.describe())
		}
	}
	// #2026 cause 1: the paths the witness speaks for are unchanged, but the
	// tree it ran on must also have LANDED — be, outside the board's
	// bookkeeping, the tree of some commit on the item's history — or the
	// check ran against code the item never carried as a whole.
	landedAt, landed, why := witnessLanded(root, w, t)
	if why != "" {
		return revisionUnestablished, why
	}
	if !landed {
		return revisionMismatch, fmt.Sprintf("the witness's tree never landed: no commit on the item's history since the merge base has a tree identical to it outside docs/streams/** and STATUS.md (a branch squash-merged after its base moved, or never merged), so the check did not run against code %s carries", target)
	}
	return revisionMatch, fmt.Sprintf("%s; %s landed at %s", eff.derivation(), landedNoun, landedAt[:treeSHALen])
}

// tokenAbsent is resolveCommitToken's why for an object it cannot find.
const tokenAbsent = "is not in this clone, or is an ambiguous abbreviation"

// resolveCommitToken resolves a hex revision token from Evidence (or the
// item's revision) to the full id of the commit it names. why is "" on
// success, otherwise the rest of a sentence saying why it could not be used:
// the object is absent (or the abbreviation is ambiguous), the token resolved
// through a ref NAME rather than as an object id (a branch or tag literally
// named like a hex prefix), or the object is not a commit. An annotated tag's
// own id peels to the commit it tags.
func resolveCommitToken(root, tok string) (full, why string) {
	out, err := coverageGit(root, "rev-parse", "--verify", "--quiet", "--end-of-options", tok).Output()
	if err != nil {
		return "", tokenAbsent
	}
	if id := strings.TrimSpace(string(out)); !strings.HasPrefix(id, strings.ToLower(tok)) {
		return "", fmt.Sprintf("resolves through a ref name to %s rather than as an object id, so it does not name the object it spells", id)
	}
	out, err = coverageGit(root, "rev-parse", "--verify", "--quiet", "--end-of-options", tok+"^{commit}").Output()
	if err != nil {
		return "", "names a git object that is not a commit"
	}
	return strings.TrimSpace(string(out)), ""
}

// coverageGit runs git in root with replacement objects ignored
// (--no-replace-objects): what a witness is compared against is the object
// store's own content, never a `refs/replace/` substitution.
func coverageGit(root string, args ...string) *exec.Cmd {
	return exec.Command("git", append([]string{"--no-replace-objects", "-C", root}, args...)...)
}

// verifyRowAtRevision reads briefPath's OWN Verify row for rowID as it stood
// at git revision rev (`git show <rev>:./<path>`), for the Expect half of the
// acceptance-definition digest guard (this file's header, F3). rev must be a
// BASE revision (witnessBaseRevision) — a `+dirty` token is not a git object.
// ok is false whenever root/briefPath do not resolve to a usable git object at
// rev, the brief has no Verify section there, or no row with that id exists
// there — the caller resolves that could-not-check, never "unchanged".
func verifyRowAtRevision(root, briefPath, rev, rowID string) (verifyRow, bool) {
	body, ok := briefBodyAtRevision(root, briefPath, rev)
	if !ok {
		return verifyRow{}, false
	}
	verify := extractSectionByPrefix(body, "Verify")
	for _, hr := range briefVerifyRows(verify) {
		if hr.ID == rowID {
			return hr, true
		}
	}
	return verifyRow{}, false
}

// briefBodyAtRevision reads briefPath's markdown body (frontmatter stripped)
// as it stood at git revision rev (`git show <rev>:./<path>`). rev must be a
// BASE revision (witnessBaseRevision). ok is false whenever root/briefPath do
// not resolve to a usable git object at rev.
func briefBodyAtRevision(root, briefPath, rev string) (string, bool) {
	if root == "" || briefPath == "" || rev == "" {
		return "", false
	}
	rel, err := filepath.Rel(root, briefPath)
	if err != nil {
		return "", false
	}
	out, err := coverageGit(root, "show", "--end-of-options", rev+":./"+filepath.ToSlash(rel)).Output()
	if err != nil {
		return "", false
	}
	content := strings.ReplaceAll(string(out), "\r\n", "\n")
	body := content
	if first, _, _ := strings.Cut(content, "\n"); strings.TrimSpace(first) == "---" {
		if _, b, ferr := splitFrontmatter(content); ferr == nil {
			body = b
		}
	}
	return body, true
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
	cov := evaluateCoverage(root, streams, coverageOptions{Patterns: patterns})

	ids := make([]string, 0, len(cov))
	for id := range cov {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	if jsonMode {
		rows := make([]Coverage, 0, len(ids))
		for _, id := range ids {
			rows = append(rows, cov[id])
		}
		out, err := json.Marshal(rows)
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
