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
//	dehoused                   the brief (or its README) marks the work de-housed
//	                           to another repo by ruling.
//	re-homed                   the stream README has retired the row (re-homed /
//	                           do-not-re-implement) — its record merged elsewhere.
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
// is could-not-check for that row's body arm; a stream README that cannot be read
// is could-not-check for that stream's README arm. An instrument that did not
// look has cleared nothing.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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
)

// The detector strings, as the drain catalogued them. Each is a phrase a routing
// ruling or a brief banner actually leaves behind; matching is case-insensitive
// so an author's capitalisation cannot slip a phantom past the detector.
var (
	// out-of-repo: the brief body names another repo as where the deliverable lands.
	reOutOfRepo = regexp.MustCompile(`(?i)deliverable lands in`)
	// dehoused: the work was de-housed to another repo by ruling.
	reDehoused = regexp.MustCompile(`(?i)\bde-?housed\b`)
	// re-homed: the stream README retired the row; its record merged elsewhere.
	reReHomed = regexp.MustCompile(`(?i)\bre-?homed\b|do not re-implement`)
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
	phantomMergedUnflipped: "reconcile the row (verified->done + regen), or say in the PR why the merge did not advance it",
	phantomOutOfRepo:       "route the work to the owning repo and leave a pointer/archived row here so the board stops reading it as fresh todo",
	phantomDehoused:        "leave a pointer/archived row here; the work is owned by the repo it was de-housed to",
	phantomReHomed:         "the record already merged elsewhere; retire the row (do not re-implement)",
	phantomStatusgenSource: "make the change in the repo that now owns the source; here the tool is a pinned binary",
	phantomDeferredByGate:  "hold until the gating brief lands; the row returns to the board by itself, nothing needs re-authoring",
}

// classifyPhantom is the pure classifier. Given a todo brief's id, its raw file
// body, its stream README text, and the set of brief ids a local merge already
// named, it returns the phantom class that caught the row and a human reason, or
// ok=false when the row is clean.
//
// Precedence runs most-specific -> most-general and the FIRST match wins, so a
// row that is both already-merged and carries a banner is reported as merged
// (the strongest, git-derived fact) rather than by a weaker text match. The
// ordering is the detector's contract and the tests pin it.
func classifyPhantom(briefID, body, readme string, mergedIDs map[string]bool) (class, reason string, ok bool) {
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
	if reDehoused.MatchString(body) || reDehoused.MatchString(readme) {
		return phantomDehoused,
			"the work was de-housed to another repo by ruling — owned there, not here",
			true
	}
	if reReHomed.MatchString(readme) {
		return phantomReHomed,
			"the stream README has retired this row (re-homed / do-not-re-implement) — its record merged elsewhere",
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
// each detector needs (the brief file body, the stream README, the local merge
// set), classifies the row, and returns one surfaced NON-DISPATCHABLE NOTICE per
// phantom — plus could-not-check NOTICEs for any input it could not read.
//
// merged/mergedErr are the SAME first-parent merge read the #270 reconciliation
// performs (mergedPRsFromGit), threaded in so the git history is read once per
// run. A non-nil mergedErr makes the already-merged-unflipped arm could-not-check
// (one aggregate NOTICE); the text detectors, which need no git, still run — a
// git failure must not blind the five classes that do not depend on it.
func boardHonestyNotices(streams []*Stream, merged []mergedPR, mergedErr error) []string {
	var notices []string
	add := func(format string, a ...any) { notices = append(notices, fmt.Sprintf(format, a...)) }

	// Build the merged-id set from the same signal #270 uses. A read error leaves
	// the set nil and records that class 1 could not be checked.
	var mergedIDs map[string]bool
	if mergedErr != nil {
		add("could-not-check: board-honesty could not read the merge history (%v), so the "+
			"already-merged-unflipped class was NOT checked — no conclusion is drawn about it. The "+
			"other five phantom classes, which need no git, still ran.", mergedErr)
	} else {
		mergedIDs = map[string]bool{}
		for _, pr := range merged {
			for _, id := range pr.Briefs {
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
		for _, b := range s.Briefs {
			if b.Status == "todo" {
				todo[b.Num] = true
				if b.HomedIn != "" {
					homedIn[b.Num] = true
				}
			}
		}
		if len(todo) == 0 {
			continue
		}

		// The stream README, read once. A read failure is could-not-check for the
		// README-keyed arms (re-homed, dehoused) of every todo row in this stream —
		// reported as itself, and the body-keyed arms still run with readme "".
		readme := ""
		readmePath := filepath.Join(s.Dir, "README.md")
		if raw, err := os.ReadFile(readmePath); err != nil {
			add("could-not-check: board-honesty could not read %s (%v), so the README-keyed "+
				"phantom classes (re-homed, dehoused) were not checked for stream %s.", readmePath, err, s.Name)
		} else {
			readme = string(raw)
		}

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
			if path, ok := pathByNum[num]; ok {
				if raw, err := os.ReadFile(path); err != nil {
					add("could-not-check: board-honesty could not read %s (%v), so the body-keyed "+
						"phantom classes were not checked for %s.", path, err, id)
				} else {
					body = string(raw)
				}
			}
			class, reason, ok := classifyPhantom(id, body, readme, mergedIDs)
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
			add("NON-DISPATCHABLE (%s): %s — %s. This todo row passes the freshness gate but is not "+
				"dispatchable from here; it inflates the Next-up count. Fix: %s. (board-honesty)",
				class, id, reason, phantomRemediation[class])
		}
	}

	sort.Strings(notices)
	return notices
}
