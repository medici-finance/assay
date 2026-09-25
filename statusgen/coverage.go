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
//  2. the witness tree is a git ANCESTOR of the item's revision, and no path
//     the witness SPEAKS FOR changed in between (ancestorNoOtherChanges,
//     witnessScope.invalidatedBy). What a witness speaks for (round-2 F2):
//     - the brief's declared `files:` paths when it declares them (a declared
//       directory covers everything under it) — the brief's own statement of
//       the surface its checks exercise;
//     - absent a declaration, conservatively, every path OUTSIDE the board's
//       bookkeeping surface — `docs/streams/**` (sibling briefs' Evidence in
//       the same verify batch, READMEs, verify-outcome logs) and the
//       regenerated `STATUS.md`, which move between ANY witness and the main
//       tip and say nothing about the code a check ran against;
//     - never the brief's own file: its Verify rows are bound separately, by
//       the Command and Expect guards below.
//     A change to a path the witness speaks for after it ran is a genuine
//     `wrong-revision`: the witness no longer speaks for today's code. The
//     residual this scope accepts, by design: a declared-`files:` brief's
//     witness is not invalidated by a change OUTSIDE its declaration (a shared
//     helper the declared files call, say) — the declaration is the contract,
//     and an under-declared brief is fixed by declaring, not by this rule
//     guessing at a dependency graph it cannot compute offline.
//
// A value that is not adequately established as one of the two READS as a
// definite mismatch (`wrong-revision`) only when both tokens are themselves
// well-formed and simply differ as VALUES — an ancestor check that could not
// even run (no git, an unresolvable SHA) never upgrades that plain difference
// into a match, but it also never invents one. Anything coverage cannot
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
// commit (witnessBaseRevision), on both the exact and the ancestor path. It
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
	"fmt"
	"os"
	"os/exec"
	pathpkg "path"
	"path/filepath"
	"sort"
	"strings"
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
	scope := newWitnessScope(root, briefPath, bf)
	for _, r := range verifyRows {
		result, reason, claimRev := resolveVerifyClaim(root, scope, r, evidence, revision)
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
		switch classifyRevision(root, scope, wrev, targetRevision) {
		case revisionUnestablished:
			return covCouldNotCheck, fmt.Sprintf("the witness's revision could not be corroborated against the item's revision (witness=%s, item=%s) — a passing result is never credited without knowing which revision it ran at", displayRevision(wrev), displayRevision(targetRevision)), wrev
		case revisionMismatch:
			return covWrongRevision, fmt.Sprintf("witness ran at revision %s, the item's revision is %s", wrev, targetRevision), wrev
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
		return covPass, "witness matches the row and passed", wrev
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
//   - otherwise the two values plainly differ, which is a mismatch UNLESS a
//     git ancestor check (ancestorNoOtherChanges) corroborates that the
//     witness tree is an ancestor of the item's revision with no path the
//     witness speaks for (scope) touched since — the ordinary shape of "run
//     the check, then commit the Evidence that records it, then keep landing
//     unrelated work on main" (F2). An ancestor check that could not even run
//     leaves the plain mismatch standing; it never invents a match it could
//     not corroborate.
func classifyRevision(root string, scope witnessScope, witnessTree, target string) revisionRelation {
	w := witnessBaseRevision(witnessTree)
	t := witnessBaseRevision(target)
	if w == "" || t == "" || w == "no-git" || t == "no-git" {
		return revisionUnestablished
	}
	if len(w) < minRevisionTokenLen || len(t) < minRevisionTokenLen {
		return revisionUnestablished
	}
	n := len(w)
	if len(t) < n {
		n = len(t)
	}
	if strings.EqualFold(w[:n], t[:n]) {
		return revisionMatch
	}
	if ancestorNoOtherChanges(root, scope, w, t) {
		return revisionMatch
	}
	return revisionMismatch
}

// witnessScope is what a brief's witnesses SPEAK FOR — the paths whose change
// after the witness ran means the witness no longer describes the item
// (round-2 F2). See this file's header ("THE REVISION COMPARISON IS OFFLINE").
type witnessScope struct {
	briefPath string   // absolute path of the brief file ("" = none)
	briefRel  string   // briefPath relative to root, slash-separated
	declared  []string // the brief's `files:` paths; nil = no declaration
}

// newWitnessScope builds a brief's witnessScope from its parsed `files:` line
// (BriefFile.DeclaredPaths). A missing/unparseable declaration leaves declared
// nil, which selects the conservative fallback in invalidatedBy.
func newWitnessScope(root, briefPath string, bf *BriefFile) witnessScope {
	sc := witnessScope{briefPath: briefPath}
	if root != "" && briefPath != "" {
		if rel, err := filepath.Rel(root, briefPath); err == nil {
			sc.briefRel = filepath.ToSlash(rel)
		}
	}
	if bf != nil && bf.DeclaredPathsFound {
		for _, d := range bf.DeclaredPaths {
			d = strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(d)), "./")
			if d != "" {
				sc.declared = append(sc.declared, d)
			}
		}
	}
	return sc
}

// isBoardBookkeepingPath reports whether a repo-relative path is the board's
// own bookkeeping surface — stream docs (brief Evidence, READMEs, verify
// outcome logs, findings) and the generated STATUS.md. A verify batch lands
// Evidence for several briefs in one commit, and statusgen regenerates
// STATUS.md after every push, so this surface moves between ANY witness and
// the main tip the model-lane flip runs at (review finding F2, round 2).
func isBoardBookkeepingPath(p string) bool {
	return p == "STATUS.md" || strings.HasPrefix(p, "docs/streams/")
}

// invalidatedBy reports whether a change to repo-relative path p, after the
// witness ran, means the witness no longer speaks for the item:
//
//   - the brief's OWN file never does — its Verify rows are bound separately,
//     by the Command and Expect guards (this file's header);
//   - when the brief declares `files:`, exactly those paths do (a declared
//     directory covers everything under it; a glob is matched as one) — the
//     brief's own statement of the surface its checks exercise;
//   - otherwise, conservatively, every path outside the board's bookkeeping
//     surface (isBoardBookkeepingPath) does.
func (sc witnessScope) invalidatedBy(p string) bool {
	p = filepath.ToSlash(strings.TrimSpace(p))
	if p == "" || p == sc.briefRel {
		return false
	}
	if sc.declared == nil {
		return !isBoardBookkeepingPath(p)
	}
	for _, d := range sc.declared {
		dd := strings.TrimSuffix(d, "/")
		if p == dd || strings.HasPrefix(p, dd+"/") {
			return true
		}
		if m, err := pathpkg.Match(d, p); err == nil && m {
			return true
		}
	}
	return false
}

// ancestorNoOtherChanges reports whether witnessTree is a git ancestor of
// target with no path the witness speaks for (scope.invalidatedBy) changed in
// between. false whenever root is not a usable git checkout, or either token
// does not resolve there as a real commit — the caller (classifyRevision) then
// keeps whatever plain-value comparison it already made, rather than promoting
// an unverifiable claim to a match. Every revision argument follows
// `--end-of-options`, so an option-shaped token from Evidence text can never be
// read as a flag, independent of call order.
func ancestorNoOtherChanges(root string, scope witnessScope, witnessTree, target string) bool {
	if root == "" || scope.briefRel == "" {
		return false
	}
	if exec.Command("git", "-C", root, "rev-parse", "--git-dir").Run() != nil {
		return false
	}
	if exec.Command("git", "-C", root, "cat-file", "-e", "--end-of-options", witnessTree+"^{commit}").Run() != nil {
		return false
	}
	if exec.Command("git", "-C", root, "cat-file", "-e", "--end-of-options", target+"^{commit}").Run() != nil {
		return false
	}
	if exec.Command("git", "-C", root, "merge-base", "--is-ancestor", "--end-of-options", witnessTree, target).Run() != nil {
		return false // not an ancestor, or the check itself could not run
	}
	out, err := exec.Command("git", "-C", root, "diff", "--name-only", "--end-of-options", witnessTree, target, "--").Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if scope.invalidatedBy(line) {
			return false // a path the witness speaks for changed since it ran
		}
	}
	return true
}

// verifyRowAtRevision reads briefPath's OWN Verify row for rowID as it stood
// at git revision rev (`git show <rev>:./<path>`), for the Expect half of the
// acceptance-definition digest guard (this file's header, F3). rev must be a
// BASE revision (witnessBaseRevision) — a `+dirty` token is not a git object.
// ok is false whenever root/briefPath do not resolve to a usable git object at
// rev, the brief has no Verify section there, or no row with that id exists
// there — the caller resolves that could-not-check, never "unchanged".
func verifyRowAtRevision(root, briefPath, rev, rowID string) (verifyRow, bool) {
	if root == "" || briefPath == "" || rev == "" {
		return verifyRow{}, false
	}
	rel, err := filepath.Rel(root, briefPath)
	if err != nil {
		return verifyRow{}, false
	}
	out, err := exec.Command("git", "-C", root, "show", "--end-of-options", rev+":./"+filepath.ToSlash(rel)).Output()
	if err != nil {
		return verifyRow{}, false
	}
	content := strings.ReplaceAll(string(out), "\r\n", "\n")
	body := content
	if first, _, _ := strings.Cut(content, "\n"); strings.TrimSpace(first) == "---" {
		if _, b, ferr := splitFrontmatter(content); ferr == nil {
			body = b
		}
	}
	verify := extractSectionByPrefix(body, "Verify")
	for _, hr := range briefVerifyRows(verify) {
		if hr.ID == rowID {
			return hr, true
		}
	}
	return verifyRow{}, false
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
