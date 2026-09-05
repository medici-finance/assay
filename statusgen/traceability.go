package main

// traceability.go — the three corpus-wide traceability checks registers-v1 §6.5
// deferred as "reserved, not gating", now landed advisory-first (sdlc/02):
//
//   - orphan-requirement   NOTICE  — a requirement the house has ACCEPTED that no
//                                     brief's `satisfies:` names. An accepted ask
//                                     with no work against it is the buyer's first
//                                     question ("what did you take on and drop?").
//   - untraced-brief       NOTICE  — a brief in-progress-or-later, in a stream that
//                                     OPTED IN (`traced: true`), that cites no
//                                     requirement. The opt-in matters: a corpus-wide
//                                     sweep over legacy briefs that predate the
//                                     register is noise (§4.5).
//   - dangling-satisfies   PROBLEM — a `satisfies:` ref naming an in-repo REQ id
//                                     that does not exist. Unlike the two NOTICEs it
//                                     cannot be legacy debt: the register is
//                                     append-only (§3.1/§3.3), so an in-repo id that
//                                     no entry defines can only be a typo or a
//                                     deleted entry, both of which a human must fix.
//
// The escalation posture copies registers-v1 §4.5 exactly: the two questions that a
// legacy corpus answers wrong (an uncited requirement, a brief citing none) land as
// advisory NOTICEs that never change the exit code, and only the one question a
// legacy corpus CANNOT answer wrong — a citation of a requirement that was never
// defined — is a hard PROBLEM.
//
// Everything here is pure over the tree: it reads the requirement register and the
// brief corpus already parsed by the rest of --lint, touches no network and shells
// out to nothing (the same offline envelope every other --lint check keeps).

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// tracedBrief is one brief file joined to its README board row, carrying only the
// three facts the traceability checks and the rollup read: which requirements it
// cites, what lifecycle state its row reports, and the body of its `## Evidence`
// section. It is the single corpus walk all three checks and the rollup share.
type tracedBrief struct {
	Key       string   // <stream>/<NN>
	Stream    string   // stream name
	Traced    bool     // the stream opted into the untraced-brief check
	Status    string   // README row status; "" when the file has no row (could-not-check)
	Satisfies []string // the brief's `satisfies:` refs, verbatim
	Evidence  string   // body of the `## Evidence` section
}

// briefTracedForward is the set of lifecycle states that count as "in-progress or
// later" for the untraced-brief check. `blocked` is a hold, not forward progress,
// and `todo` has not been dispatched — neither is expected to cite a requirement
// yet, so both are excluded rather than flagged.
var briefTracedForward = map[string]bool{
	"in-progress": true,
	"implemented": true,
	"verified":    true,
	"done":        true,
}

// scanTracedBriefs walks every brief file in every stream and joins it to its
// README board row. It returns one tracedBrief per parseable brief-v1/v2 file plus
// a could-not-check line for every brief file that failed to parse — a parse
// failure is surfaced by checkBriefFiles as a PROBLEM already, so here it is only a
// NOTICE-shaped note that this brief was NOT considered by the traceability checks,
// never rounded into "cites nothing" (docs/three-state-instrument-rule.md).
func scanTracedBriefs(streams []*Stream) (briefs []tracedBrief, couldNotCheck []string) {
	for _, s := range streams {
		// Index the stream's README rows by brief number so each file joins to the
		// row that carries its authoritative status.
		rowStatus := map[string]string{}
		for i := range s.Briefs {
			rowStatus[s.Briefs[i].Num] = s.Briefs[i].Status
		}
		for _, path := range briefFilePaths(s) {
			bf, ok, err := parseBriefFile(path)
			if err != nil {
				couldNotCheck = append(couldNotCheck,
					fmt.Sprintf("could-not-check traceability for %s: %v — this brief was not considered by the orphan/untraced/dangling checks", path, err))
				continue
			}
			if !ok {
				continue // legacy / opted-out brief: it carries no satisfies: to trace
			}
			_, num, okName := expectedBriefID(path)
			if !okName {
				continue // a malformed filename is checkBriefFiles' PROBLEM, not ours
			}
			briefs = append(briefs, tracedBrief{
				Key:       s.Name + "/" + num,
				Stream:    s.Name,
				Traced:    s.Traced,
				Status:    rowStatus[num],
				Satisfies: bf.Satisfies,
				Evidence:  bf.Evidence,
			})
		}
	}
	return briefs, couldNotCheck
}

// requirementRefKind classifies a well-formed requirement reference. A dangling
// check can only be run against an IN-REPO id (its existence is knowable from this
// root's register); a cross-repo `<alias>:REQ-<slug>` names a register in another
// repo this offline check cannot read, so it is could-not-check, never dangling.
// A malformed ref is neither — checkBriefFiles flags its grammar separately.
func requirementRefKind(ref string) (inRepoID string, crossRepo bool) {
	ref = strings.TrimSpace(ref)
	switch {
	case requirementRefInRepoRe.MatchString(ref):
		return ref, false
	case requirementRefCrossRepoRe.MatchString(ref):
		return "", true
	default:
		return "", false
	}
}

// orphanRequirementNotices returns the orphan-requirement advisory lines: every
// `accepted` requirement that no brief's `satisfies:` names, sorted by impact
// descending (the §3.5 ordered axis) then by id, each carrying the requirement's
// age and impact. A `proposed` requirement is not yet orphan-eligible — the house
// has not committed to it — and a `satisfied`/`withdrawn` one is closed.
func orphanRequirementNotices(root string, streams []*Stream, now time.Time) []string {
	entries, err := parseRequirementsDir(root)
	if err != nil || len(entries) == 0 {
		// An unreadable register is already the register's own PROBLEM
		// (requirementRegisterProblems); re-reporting it here would double-count.
		return nil
	}
	briefs, _ := scanTracedBriefs(streams)
	cited := map[string]bool{}
	for _, b := range briefs {
		for _, ref := range b.Satisfies {
			if id, cross := requirementRefKind(ref); id != "" && !cross {
				cited[id] = true
			}
		}
	}
	type orphan struct {
		entry requirementEntry
		rank  int
	}
	var orphans []orphan
	for _, e := range entries {
		if strings.TrimSpace(e.Status) != "accepted" {
			continue
		}
		if cited[e.ID] {
			continue
		}
		rank, _ := requirementImpactRank(strings.TrimSpace(e.Impact))
		orphans = append(orphans, orphan{entry: e, rank: rank})
	}
	sort.SliceStable(orphans, func(i, j int) bool {
		if orphans[i].rank != orphans[j].rank {
			return orphans[i].rank > orphans[j].rank // impact descending
		}
		return orphans[i].entry.ID < orphans[j].entry.ID
	})
	var out []string
	for _, o := range orphans {
		out = append(out, fmt.Sprintf(
			"orphan-requirement: %s (impact %s, age %s) is accepted but no brief's satisfies: names it — an accepted ask with no work against it (registers-v1 §6.5, advisory)",
			o.entry.ID, strings.TrimSpace(o.entry.Impact), requirementAge(o.entry.Date, now)))
	}
	return out
}

// requirementAge renders the coarse age of a requirement from its date field, or
// "unknown age" when the date does not parse — never a fabricated 0.
func requirementAge(date string, now time.Time) string {
	t, err := time.Parse("2006-01-02", strings.TrimSpace(date))
	if err != nil {
		return "unknown age"
	}
	return renderAge(now.Sub(t))
}

// untracedBriefNotices returns the untraced-brief advisory lines: every brief that
// is in-progress-or-later, in a stream that opted in (`traced: true`), and names no
// `satisfies:`. The stream opt-in is the whole point — without it this would fire
// over every legacy brief that predates the register (§4.5).
func untracedBriefNotices(streams []*Stream) []string {
	briefs, _ := scanTracedBriefs(streams)
	var keys []string
	for _, b := range briefs {
		if !b.Traced {
			continue
		}
		if !briefTracedForward[strings.TrimSpace(b.Status)] {
			continue
		}
		if len(nonEmptyStrings(b.Satisfies)) > 0 {
			continue
		}
		keys = append(keys, b.Key)
	}
	sort.Strings(keys)
	var out []string
	for _, k := range keys {
		out = append(out, fmt.Sprintf(
			"untraced-brief: %s is in-progress or later in a traced stream but names no satisfies: — it cites no requirement it was written against (registers-v1 §6.5, advisory)", k))
	}
	return out
}

// danglingSatisfiesProblems returns the hard dangling-satisfies PROBLEMs: every
// `satisfies:` ref naming an IN-REPO REQ id that this root's register does not
// define. It is a PROBLEM, not a NOTICE, because the register is append-only: an
// in-repo id no entry defines can only be a typo or a deleted entry.
//
// It checks ONLY well-formed in-repo refs. A grammar-malformed ref is already
// checkBriefFiles' PROBLEM (double-reporting it would say the same fault twice),
// and a cross-repo ref names a register in another repo this offline check cannot
// read — could-not-check, never dangling.
func danglingSatisfiesProblems(root string, streams []*Stream) []string {
	entries, err := parseRequirementsDir(root)
	if err != nil {
		// Unreadable register: the register's own PROBLEM already fired. Reporting
		// every satisfies ref as dangling on top of it would bury the real cause.
		return nil
	}
	exists := map[string]bool{}
	for _, e := range entries {
		if id := strings.TrimSpace(e.ID); id != "" {
			exists[id] = true
		}
	}
	briefs, _ := scanTracedBriefs(streams)
	var problems []string
	for _, b := range briefs {
		for _, ref := range b.Satisfies {
			id, cross := requirementRefKind(ref)
			if cross || id == "" {
				continue // cross-repo = could-not-check; malformed = checkBriefFiles' PROBLEM
			}
			if !exists[id] {
				problems = append(problems, fmt.Sprintf(
					"%s: satisfies names %s, which no requirement in docs/streams/requirements/ defines — dangling-satisfies (the register is append-only, so an in-repo id no entry defines is a typo or a deleted entry, registers-v1 §6.5)",
					b.Key, id))
			}
		}
	}
	sort.Strings(problems)
	return problems
}
