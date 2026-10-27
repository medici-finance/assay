package main

// boardhonesty.go — the BOARD-HONESTY detector.
//
// THE DEFECT IT SURFACES. A full drain of the Next-up board found that the
// "todo + passes-the-freshness-gate" count overstates genuinely-dispatchable
// work by a wide margin: a large fraction of rows that read `todo` and clear the
// freshness gate (no open branch, no open PR, claim free) are NOT implementable
// from a dispatch scoped to this repo. The freshness gate is correct — it asks
// "is anyone already working this?" — but it sits BELOW the routing layer and so
// cannot see that a row's work has already landed, that its deliverable belongs
// in a sibling repo, that the stream README has retired it, or that a sequencing
// gate defers it. Those are ROUTING facts, and nothing was reading them.
//
// This detector reads them. For every `todo` row it runs a set of PHANTOM-CLASS
// detectors over the brief body, the stream README, and the local merge history,
// and — when one matches — surfaces the row as NON-DISPATCHABLE with the class
// that caught it and what clears it. The point is honesty: a reader (human or a
// dispatch loop) can subtract the phantom rows from the todo count and see the
// real dispatchable backlog, instead of pointing the fleet at work that cannot be
// done here.
//
// SEVERITY IS NOTICE, deliberately, and this follows a standing house precedent
// (mergedstatus.go #270, evidenceactor.go): arming a HARD exclusion against a
// corpus that is already carrying a backlog of drifted rows would red every
// unrelated PR on day one, which is how a gate teaches people to route around it.
// A phantom row is SURFACED, not silently dropped and not hard-gated; promotion
// to an eligibility exclusion is a later ruling, once the standing backlog is
// reconciled. Until then the honest count is the surfaced one, and the detector
// never changes the exit code.
//
// SIX PHANTOM CLASSES, each with its own detector signal:
//
//	already-merged-unflipped   a local first-parent merge names this brief id in
//	                           its branch or subject, but the row is still todo.
//	                           Read from git history (mergedPRsFromGit), NOT a
//	                           forge list-with-limit — a `--limit N` PR list drops
//	                           older PRs on a busy repo and misses the canonical
//	                           duplicate, which is exactly how a merged brief got
//	                           re-dispatched. The first-parent history has no such
//	                           recency window. Reuses briefRefsIn, the same id
//	                           matcher the #270 reconciliation and the build-time
//	                           brief-number collision gate use.
//	out-of-repo-deliverable    the brief body says its deliverable lands in
//	                           another repo — the work is authored here but the
//	                           artifact belongs elsewhere.
//	dehoused                   the brief body or the ROW's own board cell marks
//	                           the work de-housed to another repo by ruling —
//	                           keyed on the row, never the whole stream README
//	                           (statusgen #1516), the same row scope re-homed uses.
//	re-homed                   the ROW's own board cell carries a re-home marker
//	                           ([homed→…] / deliverable-repo: / do-not-re-implement)
//	                           and the brief file is gone — its record merged
//	                           elsewhere, only the pointer row remains. Keyed on the
//	                           row's own cell, never a stream-level README inference
//	                           (statusgen #709).
//	statusgen-source-elsewhere the brief's task is a statusgen SOURCE change, but
//	                           the banner records that source lives in another
//	                           repo and the change must be made there; here it is
//	                           consumed as a pinned binary only.
//	deferred-by-gate           the brief declares itself DEFERRED behind an unmet
//	                           sequencing gate — un-dispatchable until the gate
//	                           brief lands, but still showing todo.
//
// THE RULESET IS OPEN-ENDED. New classes/strings are wired in as they surface;
// the detector strings live in the package-level regexps below so a new signal is
// a one-line addition with its own positive/negative test.
//
// THREE-STATE. The class-1 signal is read from git, which can fail (no .git, a
// shallow clone, git missing); a failed read makes class 1 could-not-check —
// reported as itself, never rounded to "no phantom" — while the text detectors
// (classes 2–6), which need no git, still run. A brief file that cannot be read
// is could-not-check for that row's body arm. No detector reads the stream
// README any more (statusgen #1516): the dehoused class now keys on the row's own
// cell and body, like re-homed. An instrument that did not look has cleared
// nothing.

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// errNoGitForMerge is the sentinel the caller passes as mergedErr when the tree
// has no .git, so the already-merged-unflipped arm reports could-not-check rather
// than silently reading an empty merge set as "no phantom". The five text
// detectors still run.
var errNoGitForMerge = errors.New("no .git directory — merge history unavailable")

// Phantom-class identifiers. Stable strings — the NOTICE text and the tests both
// select on these exact values, so a rename is a breaking change caught by the
// test that pins each one.
const (
	phantomMergedUnflipped = "already-merged-unflipped"
	phantomOutOfRepo       = "out-of-repo-deliverable"
	phantomDehoused        = "dehoused"
	phantomReHomed         = "re-homed"
	phantomStatusgenSource = "statusgen-source-elsewhere"
	phantomDeferredByGate  = "deferred-by-gate"
	// phantomSiblingMergeUnreconciled is the SEVENTH phantom class
	// (siblingmerge.go): a change naming this brief
	// already merged in a SIBLING repo and nobody has recorded what it
	// covered. Unlike the six classes above, it is wired and tested from its
	// own driver (siblingMergeNotices, siblingmerge.go) rather than through
	// classifyPhantom — it needs a live read of ANOTHER repo's checkout, a
	// dependency none of the tree-only classes above carry, so folding it into
	// the single-tree pure classifier would smuggle git-on-a-sibling into
	// every classifyPhantom call site and test. It shares this file's
	// severity discipline (NOTICE, never a PROBLEM here — see the package
	// comment) and its NON-DISPATCHABLE wording, and IS one of "the six
	// phantom classes" the package comment's ruleset describes — the comment
	// is not re-numbered to keep the diff each future class adds small; this
	// const block is the seventh entry and the source of truth for the count.
	phantomSiblingMergeUnreconciled = "sibling-merge-unreconciled"
)

// The detector strings, as the drain catalogued them. Each is a phrase a routing
// ruling or a brief banner actually leaves behind; matching is case-insensitive
// so an author's capitalisation cannot slip a phantom past the detector.
var (
	// out-of-repo: the brief body names another repo as where the deliverable lands.
	reOutOfRepo = regexp.MustCompile(`(?i)deliverable lands in`)
	// dehoused: the work was de-housed to another repo by ruling. Matched against
	// the brief body and the ROW's own board cell — never the whole stream README
	// (statusgen #1516), the same row scope reReHomed uses — so ordinary todo rows
	// of a stream whose README merely MENTIONS de-housing are not all flagged.
	reDehoused = regexp.MustCompile(`(?i)\bde-?housed\b`)
	// re-homed: the ROW's own re-home marker. Keyed on the row's own brief-table
	// cell (never the whole stream README — statusgen #709), so ordinary todo
	// rows of a stream whose README merely MENTIONS re-homing are not all flagged.
	// It matches either explicit retirement wording a human wrote into that one
	// cell (re-homed / do-not-re-implement) or the rendered row markers that name
	// where the record went: the `[homed→<owner>/<repo>]` marker emit.go writes,
	// or a `deliverable-repo:` note carried on the row.
	reReHomed = regexp.MustCompile(`(?i)\bre-?homed\b|do not re-implement|\[homed→|deliverable-repo:`)
	// statusgen-source-elsewhere: the banner that records the source moved out.
	reStatusgenSource = regexp.MustCompile(`(?i)statusgen SOURCE change must be made in|must be made in [^\n]*medici-finance/assay`)
	// deferred-by-gate: a self-declared DEFERRED status behind an unmet gate.
	reDeferredByGate = regexp.MustCompile(`(?i)status:[^\n]*deferred|do not dispatch ahead of|do not dispatch until`)
)

// homedInSupersedes is the set of phantom classes that a valid `homed-in:` field
// (statusgen/12) makes redundant: each is a HEURISTIC guess that a brief's
// deliverable moved to another repo, and `homed-in` is the explicit,
// author-declared successor that turns that guess into a fact — one that actually
// excludes the brief from Next-up and carries the target repo. When a row carries
// a valid `homed-in`, the guess must not ALSO fire: the explicit field is now
// doing precisely what the heuristic was estimating, so leaving both would
// double-report the same row. The git-derived already-merged-unflipped class and
// the sequencing-gate class are NOT here — they describe different facts (a landed
// row, an unmet gate) that `homed-in` does not speak to.
var homedInSupersedes = map[string]bool{
	phantomOutOfRepo:       true,
	phantomDehoused:        true,
	phantomReHomed:         true,
	phantomStatusgenSource: true,
}

// phantomRemediation maps a class to the one-line "what clears it" the surfaced
// board line carries, so a reader is told the resolution, not just the diagnosis.
var phantomRemediation = map[string]string{
	phantomMergedUnflipped:          "reconcile the row (verified->done + regen), or say in the PR why the merge did not advance it",
	phantomOutOfRepo:                "route the work to the owning repo and leave a pointer/archived row here so the board stops reading it as fresh todo",
	phantomDehoused:                 "leave a pointer/archived row here; the work is owned by the repo it was de-housed to",
	phantomReHomed:                  "the record already merged elsewhere; retire the row (do not re-implement)",
	phantomStatusgenSource:          "make the change in the repo that now owns the source; here the tool is a pinned binary",
	phantomDeferredByGate:           "hold until the gating brief lands; the row returns to the board by itself, nothing needs re-authoring",
	phantomSiblingMergeUnreconciled: "read the merged sibling change Task by Task and reconcile the row, or record a `delivery:` claim naming what it covers",
}

// phantomFlipHeldRemediation is the "what clears it" for an already-merged-unflipped
// row the reconcile's own lint hold refuses to write (reconcilehold.go). Telling
// that reader to "reconcile the row" sends them to the one step that is guaranteed
// not to move it; what clears it is the lint problem the hold named, so the fix
// names the two records the Status-keyed rules ask for.
const phantomFlipHeldRemediation = "the reconcile's lint hold refuses to write this row, so reconciling cannot clear it — " +
	"clear the problem the hold names: cite the approved design-decision record in the brief's `design:` " +
	"(lifecycle-v1 §4.4), or record the human ruling in `ruling:` (lifecycle-v1 §4.5); the next reconcile " +
	"then writes the row. Do not re-dispatch it"

// phantomFlipUncheckedRemediation is the fix for a delivered row whose flip the
// lint hold could not evaluate at all: no conclusion is drawn, so the reader is
// sent to the evaluation, never to a reconcile that may or may not write.
const phantomFlipUncheckedRemediation = "the reconcile's lint hold could not evaluate this row, so whether reconciling " +
	"clears it is unknown — run `statusgen reconcile --backfill` on a full checkout to see the verdict. Do not re-dispatch it"

// classifyPhantom is the pure classifier. Given a todo brief's id, its raw file
// body, the ROW's own brief-table cell, the set of brief ids a local merge already
// named, and whether the row's own brief file exists on disk, it returns the
// phantom class that caught the row and a human reason, or ok=false when the row
// is clean.
//
// rowCell is the raw brief-table cell for this row. The dehoused AND re-homed
// classes both key on IT (plus, for dehoused, the brief body), never on the
// stream README, because a stream README mentions de-housing or re-homing for
// reasons that have nothing to do with any one row — a scope/boundary note, a
// sibling brief ABOUT de-housing code, or a stream re-homed INTO this repo
// (statusgen #581) describing its own arrival. Keying such a class on the whole
// README flagged EVERY todo row of such a stream (re-homed whose brief file was
// absent: statusgen #709; dehoused, every todo row: statusgen #1516): a false
// positive that made the signal worthless. A genuine pointer row carries its OWN
// marker — the rendered `[homed→<owner>/<repo>]` marker, a `deliverable-repo:`
// note, a de-housed/retirement note — in its cell; the stream README is not
// evidence about the row.
//
// briefFilePresent is the ROW's own record. The re-homed class also keys on it: a
// present brief file is LIVE work, so re-homed fires only on a POINTER row — one
// whose OWN cell carries a re-home marker AND whose brief file is gone (the record
// moved elsewhere, leaving only the row behind).
//
// Precedence runs most-specific -> most-general and the FIRST match wins, so a
// row that is both already-merged and carries a banner is reported as merged
// (the strongest, git-derived fact) rather than by a weaker text match. The
// ordering is the detector's contract and the tests pin it.
func classifyPhantom(briefID, body, rowCell string, mergedIDs map[string]bool, briefFilePresent bool) (class, reason string, ok bool) {
	if mergedIDs[briefID] {
		return phantomMergedUnflipped,
			"a merged PR/commit already names this brief but the row is still todo — Next-up keeps offering work that has already landed",
			true
	}
	if reStatusgenSource.MatchString(body) {
		return phantomStatusgenSource,
			"the brief's task is a source change whose source now lives in another repo — un-implementable from a dispatch here",
			true
	}
	if reOutOfRepo.MatchString(body) {
		return phantomOutOfRepo,
			"the brief's deliverable lands in a sibling repo — the artifact is not produced here",
			true
	}
	// dehoused keys on the brief body and the ROW's OWN cell — never the stream
	// README (statusgen #1516), the same row scope re-homed uses below — so a
	// stream whose README merely MENTIONS de-housing does not flag its every row.
	if reDehoused.MatchString(body) || reDehoused.MatchString(rowCell) {
		return phantomDehoused,
			"the work was de-housed to another repo by ruling — owned there, not here",
			true
	}
	// re-homed fires only on a POINTER row: the ROW's OWN cell carries a re-home
	// marker AND the row's own brief file is gone. Keyed on rowCell, never the
	// stream README (statusgen #709) — a README mention of re-homing says nothing
	// about any one row, and a present brief file means live work (statusgen #581).
	if !briefFilePresent && reReHomed.MatchString(rowCell) {
		return phantomReHomed,
			"this row's own board cell carries a re-home marker ([homed→…] / deliverable-repo: / do-not-re-implement) and its brief file is gone — the record merged elsewhere, only the pointer row remains",
			true
	}
	if reDeferredByGate.MatchString(body) {
		return phantomDeferredByGate,
			"the brief declares itself deferred behind an unmet sequencing gate — un-dispatchable until the gate brief lands",
			true
	}
	return "", "", false
}

// boardHonestyNotices is the driver: it walks every `todo` row, loads the inputs
// each detector needs (the brief file body, the row's own board cell, the local
// merge set), classifies the row, and returns one surfaced NON-DISPATCHABLE
// NOTICE per phantom — plus could-not-check NOTICEs for any input it could not
// read.
//
// merged/mergedErr are the SAME first-parent merge read the #270 reconciliation
// performs (mergedPRsFromGit), threaded in so the git history is read once per
// run. A non-nil mergedErr makes the already-merged-unflipped arm could-not-check
// (one aggregate NOTICE); the text detectors, which need no git, still run — a
// git failure must not blind the five classes that do not depend on it.
//
// root is the board root the reconcile writes under (root/docs/streams/<name>):
// the already-merged-unflipped rows held on a Brief: trailer are judged by the
// reconcile's own lint hold there (reconcileFlipVerdicts), so the NOTICE never
// tells a reader to reconcile a row the reconcile refuses to write.
func boardHonestyNotices(root string, streams []*Stream, merged []mergedPR, mergedErr error) []string {
	finds, notices := phantomFindings(root, streams, merged, mergedErr)
	for _, f := range finds {
		reason, fix := f.Reason, phantomRemediation[f.Class]
		if !f.Flip.Clean() {
			// The hold's problem text often ends in a period of its own; the format
			// below adds one.
			reason, fix = strings.TrimRight(reason+"; "+f.Flip.Label(), "."), f.Flip.Remediation()
		}
		notices = append(notices, fmt.Sprintf("NON-DISPATCHABLE (%s): %s — %s. This todo row passes the freshness gate but is not "+
			"dispatchable from here; it inflates the Next-up count. Fix: %s. (board-honesty)",
			f.Class, f.ID, reason, fix))
	}
	sort.Strings(notices)
	return notices
}

// phantomFinding is one classified todo row: the pure classifier's verdict plus
// where the row lives and whether the evidence is STRONG enough to hold it out of
// dispatch (applyPhantomHolds), or only enough to surface a NOTICE.
type phantomFinding struct {
	Stream *Stream
	Index  int // index of the row in Stream.Briefs
	ID     string
	Class  string
	Reason string
	// Hold is the dispatch-exclusion label ("<class> (<evidence>)") when the
	// evidence is strong, "" when the finding is notice-only.
	Hold string
	// Flip is the reconcile lint hold's verdict on writing this row to
	// implemented — set only for an already-merged-unflipped row held on a
	// Brief: trailer, the one class the reconcile is told to clear.
	Flip *flipVerdict
}

// phantomFlipState is the three-state reading of a flipVerdict. Every render of a
// verdict (the hold label, the NOTICE, the Next-up board) goes through it, so a
// could-not-check can never be shown as the definite refusal it is not: each
// state carries its own board heading and its own fix, and a new state cannot
// be added without defining both.
type phantomFlipState uint8

const (
	phantomFlipClean     phantomFlipState = iota // the reconcile would write the row
	phantomFlipHeld                              // the reconcile refuses: the lint PROBLEM(s) it names
	phantomFlipUnchecked                         // the hold could not be evaluated: no conclusion drawn
)

// heading is the board line's name for the state's rows.
func (s phantomFlipState) heading() string {
	if s == phantomFlipUnchecked {
		return "flip hold could-not-check"
	}
	return "flip held by the reconcile's lint hold"
}

// remediation is the "what clears it" for the state's rows.
func (s phantomFlipState) remediation() string {
	if s == phantomFlipUnchecked {
		return phantomFlipUncheckedRemediation
	}
	return phantomFlipHeldRemediation
}

// flipVerdict is what `statusgen reconcile --apply` would do with one delivered
// row, read from the reconcile's own decision path (planReconcileWrites) rather
// than re-derived: admitted, held with the lint PROBLEM(s) the move would add, or
// could-not-check with why. Unchecked is never a pass.
type flipVerdict struct {
	Problems  []string // the hold's PROBLEM lines; empty when admitted or unchecked
	Unchecked string   // why the hold could not be evaluated; "" when it was
}

// State is the verdict's three-state reading. A nil verdict (the row was not
// evaluated) is phantomFlipClean: nothing to say beyond the plain label.
func (v *flipVerdict) State() phantomFlipState {
	switch {
	case v == nil:
		return phantomFlipClean
	case v.Unchecked != "":
		return phantomFlipUnchecked
	case len(v.Problems) > 0:
		return phantomFlipHeld
	}
	return phantomFlipClean
}

// Clean reports whether the reconcile would write the row.
func (v *flipVerdict) Clean() bool { return v.State() == phantomFlipClean }

// Label is the suffix the hold label and the NOTICE carry for a row the
// reconcile will not write: the same PROBLEM text the reconcile's hold reports.
func (v *flipVerdict) Label() string {
	if v.State() == phantomFlipUnchecked {
		return "flip hold could-not-check: " + v.Unchecked
	}
	return "flip held: " + strings.Join(v.Problems, "; ")
}

// Remediation is the fix for a row the reconcile will not write.
func (v *flipVerdict) Remediation() string { return v.State().remediation() }

// strongPhantom decides whether a classified row may be HELD from dispatch, or is
// notice-only. The text classes are HEURISTIC mentions — a brief ABOUT de-housing,
// or one that quotes "deliverable lands in" while explaining a design, matches
// them — and a mention is not an ownership declaration (the explicit declaration
// is `homed-in:`, which eligibleBase already honours). So only two kinds of
// evidence hold a row:
//
//   - already-merged-unflipped, when a merged PR's `Brief:` trailer names the brief
//     (pr.deliveryOf == mergeDelivered) and the brief declares no partial delivery;
//     a subject/branch-only match is unconfirmed and an Authors: trailer is not
//     delivery;
//   - re-homed, which is already row-scoped (the row's own cell carries the marker
//     AND its brief file is gone).
//
// flip is the reconcile lint hold's verdict for a delivered row (nil when it was
// not evaluated). A row the reconcile will not write keeps its hold — the work
// still landed — but the label names the hold's PROBLEM, so "reconcile the row"
// is never the reading of a row the reconcile refuses.
func strongPhantom(class, id string, delivering map[string]mergedPR, partial bool, flip *flipVerdict) string {
	switch class {
	case phantomMergedUnflipped:
		if pr, ok := delivering[id]; ok && !partial {
			evidence := "merged PR, Brief: trailer"
			if pr.Number > 0 {
				evidence = fmt.Sprintf("merged PR #%d, Brief: trailer", pr.Number)
			}
			if !flip.Clean() {
				evidence += "; " + flip.Label()
			}
			return fmt.Sprintf("%s (%s)", class, evidence)
		}
	case phantomReHomed:
		return fmt.Sprintf("%s (pointer row, brief file gone)", class)
	}
	return ""
}

// applyPhantomHolds runs the board-honesty classifier and stamps Brief.PhantomHeld
// on every TODO row it caught on strong evidence, so eligibleBase excludes it in
// every path that builds Next-up. One classifier, no separate allowlist. A failed
// merge read (mergedErr != nil) holds no merged-delivery row: absent forge/history
// state is could-not-check, never "delivered". A re-homed pointer row is still held,
// because its evidence is the row and its missing brief file, not the history.
func applyPhantomHolds(root string, streams []*Stream, merged []mergedPR, mergedErr error) {
	finds, _ := phantomFindings(root, streams, merged, mergedErr)
	for _, f := range finds {
		if f.Hold != "" {
			f.Stream.Briefs[f.Index].PhantomHeld = f.Hold
			f.Stream.Briefs[f.Index].PhantomFlip = f.Flip.State()
		}
	}
}

// planFlipWrites is the reconcile's read-only write path the planner asks. It is
// a variable only so a test can count and shape the evaluations.
var planFlipWrites = planReconcileWrites

// flipMemo holds the flip verdicts of one statusgen invocation, keyed by root and
// row. Evaluating the hold costs a whole-board baseline plus a move per row, and
// run() asks twice (boardHonestyNotices, then applyPhantomHolds) about the same
// rows of the same tree; a verdict is reused, never recomputed, for the life of
// the invocation. Nil outside run(): a caller that holds no invocation (a test, a
// tree it has just edited) always gets a fresh evaluation, so no verdict outlives
// the run that read the tree.
var flipMemo map[string]*flipVerdict

// beginFlipMemo opens the invocation's verdict memo and returns its close, which
// restores whatever scope was open before (run() may nest under another run()).
func beginFlipMemo() (end func()) {
	prev := flipMemo
	flipMemo = map[string]*flipVerdict{}
	return func() { flipMemo = prev }
}

// reconcileFlipVerdicts asks the reconcile's OWN decision path what it would do
// with each delivered row: planReconcileWrites, the read-only form of
// `reconcile --apply` (same cells, same lint hold, nothing written). One
// evaluator, so the planner's label and the reconcile's write can never
// disagree about whether reconciling clears a row.
//
// Each id is evaluated as a trailer-witnessed todo -> implemented move, the
// cell the reconcile builds for a merged Brief:-trailer PR. Ids are fed in
// sorted order; the reconcile evaluates its witnessed cells in its own order
// with earlier admitted moves kept, so a verdict that depends on another row
// moving first can differ at the margin — that is the reconcile's ordering,
// not a second rule. A row the reconcile has no verdict for (no README row it
// can rewrite) and an evaluation error are both could-not-check, never clean.
//
// Inside an invocation (beginFlipMemo) only the rows not yet judged under root
// are evaluated, in one batch.
func reconcileFlipVerdicts(root string, ids map[string]string, prs map[string]int) map[string]*flipVerdict {
	out := map[string]*flipVerdict{}
	if len(ids) == 0 {
		return out
	}
	keys := make([]string, 0, len(ids))
	for k := range ids {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	memoKey := func(k string) string { return root + "\x00" + k }
	var fresh []string
	for _, k := range keys {
		if v, ok := flipMemo[memoKey(k)]; ok {
			out[ids[k]] = v
			continue
		}
		fresh = append(fresh, k)
	}
	for k, v := range evaluateFlipVerdicts(root, fresh, prs) {
		if flipMemo != nil {
			flipMemo[memoKey(k)] = v
		}
		out[ids[k]] = v
	}
	return out
}

// evaluateFlipVerdicts runs the reconcile's read-only write path once over keys
// (sorted "<stream dir>/<num>") and reads a verdict for each.
func evaluateFlipVerdicts(root string, keys []string, prs map[string]int) map[string]*flipVerdict {
	out := map[string]*flipVerdict{}
	if len(keys) == 0 {
		return out
	}
	cells := make([]BriefCell, 0, len(keys))
	for _, k := range keys {
		cells = append(cells, BriefCell{ID: k, Cell: "implemented", Source: "pr", Witness: fmt.Sprintf("PR #%d", prs[k])})
	}
	rows, held, err := planFlipWrites(root, cells)
	if err != nil {
		for _, k := range keys {
			out[k] = &flipVerdict{Unchecked: "the reconcile's hold evaluation failed (" + err.Error() + ")"}
		}
		return out
	}
	for _, r := range rows {
		out[r.ID] = &flipVerdict{}
	}
	for _, h := range held {
		out[h.ID] = heldFlipVerdict(h.Problems)
	}
	for _, k := range keys {
		if _, ok := out[k]; !ok {
			out[k] = &flipVerdict{Unchecked: "the reconcile found no todo row for " + k + " under " +
				filepath.ToSlash(filepath.Join("docs", "streams", strings.SplitN(k, "/", 2)[0], "README.md")) + " to write"}
		}
	}
	return out
}

// heldFlipVerdict reads one held row's PROBLEM lines. The reconcile reports a
// write it could not evaluate (the lint's stream model has no such row) in the
// same list as a refusal, marked with couldNotCheckPrefix; a row held ONLY on such
// lines is could-not-check here, never a refusal. Any real PROBLEM keeps the
// row held, with every line named.
func heldFlipVerdict(problems []string) *flipVerdict {
	for _, p := range problems {
		if !isCouldNotCheckProblem(p) {
			return &flipVerdict{Problems: problems}
		}
	}
	return &flipVerdict{Unchecked: strings.Join(problems, "; ")}
}

// phantomFindings is the walk boardHonestyNotices always did, returning structured
// findings next to the could-not-check notices. Every already-merged-unflipped
// row it holds on a Brief: trailer is then judged by the reconcile's own lint
// hold under root (reconcileFlipVerdicts), so its label says whether
// reconciling can clear it.
func phantomFindings(root string, streams []*Stream, merged []mergedPR, mergedErr error) ([]phantomFinding, []string) {
	var notices []string
	var finds []phantomFinding
	add := func(format string, a ...any) { notices = append(notices, fmt.Sprintf(format, a...)) }

	// Build the merged-id set from the same signal #270 uses. A read error leaves
	// the set nil and records that class 1 could not be checked.
	var mergedIDs map[string]bool
	delivering := map[string]mergedPR{}
	if mergedErr != nil {
		add("could-not-check: board-honesty could not read the merge history (%v), so the "+
			"already-merged-unflipped class was NOT checked — no conclusion is drawn about it. The "+
			"other five phantom classes, which need no git, still ran.", mergedErr)
	} else {
		mergedIDs = map[string]bool{}
		for _, pr := range merged {
			for _, id := range pr.Briefs {
				switch pr.deliveryOf(id) {
				case mergeAuthoring:
					continue // an Authors:-only PR WROTE the brief; it delivered nothing
				case mergeDelivered:
					if _, dup := delivering[id]; !dup {
						delivering[id] = pr
					}
				}
				mergedIDs[id] = true
			}
		}
	}

	for _, s := range streams {
		// Which brief numbers are todo — the only rows this detector judges.
		// homedIn records which todo rows carry a valid `homed-in:` (wired onto
		// the Brief row by checkBriefFiles, which runs before this detector), so
		// the explicit field can suppress the heuristic classes it supersedes.
		todo := map[string]bool{}
		homedIn := map[string]bool{}
		// cellByNum records each todo row's OWN brief-table cell, the only
		// row-scoped signal the re-homed class may key on (statusgen #709).
		cellByNum := map[string]string{}
		idxByNum := map[string]int{}
		partialByNum := map[string]bool{}
		for i, b := range s.Briefs {
			if b.Status == "todo" {
				idxByNum[b.Num] = i
				for _, d := range b.Delivery {
					if d.Covers == "partial" {
						partialByNum[b.Num] = true
					}
				}
				todo[b.Num] = true
				cellByNum[b.Num] = b.RawCell
				if b.HomedIn != "" {
					homedIn[b.Num] = true
				}
			}
		}
		if len(todo) == 0 {
			continue
		}

		// No stream README is read here: every phantom class now keys on either the
		// git merge set, the brief body, or the row's own cell (dehoused went
		// row-scoped in statusgen #1516; re-homed already was, statusgen #709). A
		// stream-wide README match flagged every todo row of a stream that merely
		// mentioned de-housing, so the README is no longer evidence about any row.

		// Map todo brief number -> its file path, for the body read.
		pathByNum := map[string]string{}
		for _, path := range briefFilePaths(s) {
			if _, num, ok := expectedBriefID(path); ok {
				pathByNum[num] = path
			}
		}

		// Iterate todo rows in a deterministic order so the NOTICE list is stable.
		nums := make([]string, 0, len(todo))
		for num := range todo {
			nums = append(nums, num)
		}
		sort.Strings(nums)

		for _, num := range nums {
			id := s.Name + "/" + num
			body := ""
			// briefFilePresent is the ROW's own record: whether a brief file for
			// this todo row exists on disk. It gates the re-homed class so a live
			// row of a stream re-homed INTO this repo (its cell may carry a marker)
			// is still recognised as live work, not a retired pointer (statusgen #581).
			path, briefFilePresent := pathByNum[num]
			if briefFilePresent {
				if raw, err := readFileMemo(path); err != nil {
					add("could-not-check: board-honesty could not read %s (%v), so the body-keyed "+
						"phantom classes were not checked for %s.", path, err, id)
				} else {
					body = string(raw)
				}
			}
			class, reason, ok := classifyPhantom(id, body, cellByNum[num], mergedIDs, briefFilePresent)
			if !ok {
				continue
			}
			// A valid `homed-in:` is the explicit successor to the heuristic
			// work-moved classes (statusgen/12): when it is present, do not ALSO
			// fire the guess it replaces — the field already excludes the brief
			// and names the target. Classes it does not speak to (already-merged,
			// deferred-by-gate) still surface.
			if homedIn[num] && homedInSupersedes[class] {
				continue
			}
			finds = append(finds, phantomFinding{
				Stream: s, Index: idxByNum[num], ID: id, Class: class, Reason: reason,
				Hold: strongPhantom(class, id, delivering, partialByNum[num], nil),
			})
		}
	}

	// The rows a reader is told to reconcile: delivered on a Brief: trailer and
	// held. Keyed by the stream's directory name — the key the reconcile maps to
	// docs/streams/<key>/README.md — and mapped back to the finding's id.
	flipIDs, flipPRs := map[string]string{}, map[string]int{}
	for _, f := range finds {
		if f.Class == phantomMergedUnflipped && f.Hold != "" {
			key := filepath.Base(f.Stream.Dir) + "/" + f.Stream.Briefs[f.Index].Num
			flipIDs[key], flipPRs[key] = f.ID, delivering[f.ID].Number
		}
	}
	verdicts := reconcileFlipVerdicts(root, flipIDs, flipPRs)
	for i := range finds {
		f := &finds[i]
		if v, ok := verdicts[f.ID]; ok {
			f.Flip = v
			f.Hold = strongPhantom(f.Class, f.ID, delivering, false, v)
		}
	}

	sort.Strings(notices)
	return finds, notices
}
