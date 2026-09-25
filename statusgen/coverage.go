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
// THE REVISION COMPARISON IS OFFLINE. "The merged SHA for a merged brief, the PR
// head for an open one" (Task item 1) would ordinarily need a live PR/merge
// lookup — exactly the live-infrastructure contact the ground rules forbid, even
// read-only. The offline equivalent this file uses instead: the tree currently
// checked out at --root IS the item's revision, whichever of the two states it
// is in — `git rev-parse HEAD` on a merged brief's checkout names the merge SHA,
// and on an open PR's branch checkout it names the PR head. So `wrong-revision`
// here means "a witness was recorded against a commit that is not the one
// presently checked out", which is the same fact the merged/open distinction
// was reaching for, read off a signal that needs no network. A tree with no git
// history at all (a bare testdata copy) yields "" and coverage skips the
// revision comparison for that run — never fabricates a match or a mismatch
// against an SHA it could not establish (three-state-instrument-rule.md).
//
// THE ACCEPTANCE-DEFINITION DIGEST (2026-09-18 integration amendment). "Applicable
// evidence must bind the exact subject and acceptance-definition digest; a model
// assessment is never an execution witness." Two separate guards implement this:
//
//   - Only a row shaped like verifyrun's own witness table (isWitnessRow: an
//     `exit=` marker AND a `sha256:` marker, each in its own cell) is ever read
//     as an execution witness. Prose — including a model's own high-confidence
//     textual assessment of a row — never matches that shape, so it can never
//     supply a missing witness; the claim stays `missing`. This is
//     TestCoverageAdviceCannotSupplyWitness.
//   - The witness's Command cell must match the Verify row's CURRENT Command
//     text byte-for-byte (normalized for whitespace) before its Result is ever
//     read. The row's Command+Expect text IS its acceptance definition; when
//     either changes after the witness ran, the witness no longer describes the
//     row it is credited to, and the claim resolves `error` — the "witness
//     present but unparseable" case (docs/streams/graph-execution/brief-03-evidence-coverage-rule.md's
//     mapping table), read as "no longer parseable AS EVIDENCE FOR THIS CLAIM".
//     This is TestCoverageAcceptanceDigestChanged.
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
	// header for why that is the offline equivalent of "merged SHA / PR head".
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
			out[id] = evaluateOneCoverage(id, bf, opts, revision)
		}
	}
	return out
}

// evaluateOneCoverage resolves one brief's claim set and Released verdict. It
// reads the brief's Verify/Evidence bodies off the already-parsed BriefFile
// (bf.Verify / bf.Evidence, populated by parseBriefFile for every schema
// version) rather than re-reading the file — the same data checkBriefFiles and
// autoflip.go's Evidence-contradiction read already use.
func evaluateOneCoverage(id string, bf *BriefFile, opts coverageOptions, revision string) Coverage {
	evidence := parseEvidenceRows(bf.Evidence)
	verifyRows := briefVerifyRows(bf.Verify)

	var claims []Claim

	// (a) every Verify row is a mandatory claim by construction.
	for _, r := range verifyRows {
		result, reason, claimRev := resolveVerifyClaim(r, evidence, revision)
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
// fixes. targetRevision == "" skips the revision comparison entirely (an
// unreadable/absent tree revision is never treated as a match OR a mismatch).
func resolveVerifyClaim(r verifyRow, evidence map[string][]evidenceRow, targetRevision string) (result, reason, revision string) {
	var latestText string
	for _, er := range evidence[r.ID] {
		if isWitnessRow(er.Text) {
			latestText = er.Text // last witness for the row wins, same as checkWitnesses
		}
	}
	if latestText == "" {
		return covMissing, "no execution witness in Evidence — run `statusgen verifyrun --brief <path>`", ""
	}

	// The acceptance-definition digest guard (this file's header): the witness
	// must still describe the CURRENT Command text, or it is not evidence for
	// this claim any more.
	if normalizeCommandText(witnessCommandOf(latestText)) != normalizeCommandText(r.Command) {
		return covError, "the witness records a different command than the Verify row now carries — its acceptance definition changed after it was run, so the recorded result proves nothing about the row as it stands today", ""
	}

	wrev := witnessTreeOf(latestText)
	switch witnessStateOf(latestText) {
	case statePass:
		if targetRevision != "" && wrev != "" && !revisionMatches(wrev, targetRevision) {
			return covWrongRevision, fmt.Sprintf("witness ran at revision %s, the item's revision is %s", wrev, targetRevision), wrev
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

// revisionMatches compares a witness's recorded tree token against the item's
// revision, ignoring the `+dirty`/`+unknown` suffix (that suffix records the
// witness run's OWN cleanliness, not a different identity) and comparing only
// the shorter of the two prefixes, case-insensitively — the same tolerance
// treeSHALen already bakes into every witness comparison in this binary.
func revisionMatches(witnessTree, target string) bool {
	w := strings.TrimSuffix(strings.TrimSuffix(witnessTree, "+dirty"), "+unknown")
	t := strings.TrimSuffix(strings.TrimSuffix(target, "+dirty"), "+unknown")
	if w == "" || t == "" {
		return true // nothing to compare against
	}
	n := len(w)
	if len(t) < n {
		n = len(t)
	}
	if n == 0 {
		return true
	}
	return strings.EqualFold(w[:n], t[:n])
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
