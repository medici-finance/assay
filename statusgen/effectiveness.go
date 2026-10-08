package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Corrective-action effectiveness record (iso-9001/03).
//
// A findings entry records that a corrective action was taken; `resolved: yes`
// says the work landed, not that the failure mode can no longer occur. The record
// closes that gap with three keys, REQUIRED TOGETHER OR NOT AT ALL (the parked
// triple's shape):
//
//	effectiveness:      the command, verbatim, that re-establishes the failure mode is gone
//	effectiveness-date: YYYY-MM-DD the command was run
//	effectiveness-by:   who ran it — `human:<name>` or a runner identity
//
// WHAT THIS CHECK COVERS. It is a presence floor: the record EXISTS, is complete,
// and is dated and attributed. It does NOT establish that the named command would
// genuinely fail if the failure mode returned — adequacy is the reviewer's
// question, asked at a different time by a different actor (docs/brief-rules.md
// rule 16's mutation row is the worked example of a good record). Every message
// below says so, so a clean lint is never read as a demonstrated control.
//
// OUT OF SCOPE, on purpose: a `cause:` field. A presence check on a stated cause
// proves nothing (a cause restating the symptom passes it), so an obligation on it
// would be proofing judgement, not a mechanical check.
//
// SEVERITY IS TRANSITION-SCOPED by the finding's own `date:` against
// effectivenessBoundary: a resolved finding dated on or after the boundary owes
// the record as a PROBLEM; an earlier one owes it as a NOTICE, so the inherited
// register is not made fatal by this change. A partial record is a PROBLEM at
// any date.
//
// effectivenessBoundary is the date the field was introduced. It is a named
// constant, not an inline literal, so a reviewer can find and argue with it.
// Moving it later widens the advisory window; moving it earlier makes more of the
// inherited corpus fatal.
const effectivenessBoundary = "2026-10-08"

// Stable [rule-tag]s for the firing audit (lintaudit.go extracts the first
// bracket token of each emitted line).
const (
	effTagPartial = "effectiveness-partial"
	effTagMissing = "effectiveness-missing"
	effTagUnfired = "finding-control-unfired"
)

// effectivenessBoundaryTime parses effectivenessBoundary. The constant is a
// compile-time literal, so a parse failure is a programming error.
func effectivenessBoundaryTime() time.Time {
	t, err := time.Parse("2006-01-02", effectivenessBoundary)
	if err != nil {
		panic("effectivenessBoundary is not YYYY-MM-DD: " + err.Error())
	}
	return t
}

// effectivenessPresence returns which of the three keys are non-blank.
func effectivenessPresence(f Finding) (cmd, date, by bool) {
	return strings.TrimSpace(f.Effectiveness) != "",
		strings.TrimSpace(f.EffectivenessDate) != "",
		strings.TrimSpace(f.EffectivenessBy) != ""
}

// effectivenessAbsent reports that none of the three keys is present.
func effectivenessAbsent(f Finding) bool {
	c, d, b := effectivenessPresence(f)
	return !c && !d && !b
}

func effectivenessFindingID(f Finding) string {
	if id := strings.TrimSpace(f.ID); id != "" {
		return id
	}
	return "(unidentified finding)"
}

// effectivenessBoundaryNote is the shared honesty tail of every message: it states
// which half the check covers and points at the worked example.
const effectivenessBoundaryNote = "This check verifies the record's PRESENCE and attribution only, not whether the named command re-establishes anything (adequacy stays with the reviewer: does it genuinely fail if the failure mode returned?). For a worked example of a good one, see the mutation row in docs/brief-rules.md rule 16: revert the fix, run the check, confirm it goes RED."

// effectivenessTripleProblems raises one hard PROBLEM per finding that carries one
// or two of the three effectiveness keys, naming which are missing or invalid.
// All three present and well-formed, or none, is silent. Mirrors parkFieldProblems.
func effectivenessTripleProblems(findings []Finding) []string {
	var problems []string
	for _, f := range findings {
		hasCmd, hasDate, hasBy := effectivenessPresence(f)
		if !hasCmd && !hasDate && !hasBy {
			continue // no record — the closure obligation (not this check) decides
		}
		var missing []string
		if !hasCmd {
			missing = append(missing, "effectiveness (the command, verbatim, that re-establishes the failure mode is gone)")
		}
		if !hasDate {
			missing = append(missing, "effectiveness-date (the YYYY-MM-DD the command was run)")
		} else if _, err := time.Parse("2006-01-02", strings.TrimSpace(f.EffectivenessDate)); err != nil {
			missing = append(missing, fmt.Sprintf("a parseable effectiveness-date (got %q, want YYYY-MM-DD)", strings.TrimSpace(f.EffectivenessDate)))
		}
		if !hasBy {
			missing = append(missing, "effectiveness-by (who ran it: human:<name> or a runner identity)")
		}
		if len(missing) == 0 {
			continue
		}
		problems = append(problems, fmt.Sprintf(
			"[%s] findings register: %s: malformed effectiveness record — missing/invalid: %s. effectiveness, effectiveness-date and effectiveness-by are REQUIRED TOGETHER, or none: a partial record is worse than no record because it reads as one. %s",
			effTagPartial, effectivenessFindingID(f), strings.Join(missing, "; "), effectivenessBoundaryNote))
	}
	sort.Strings(problems)
	return problems
}

// effectivenessOwed classifies a resolved finding that carries NO effectiveness
// record against the transition boundary. fatal=true means it is dated on or after
// the boundary; scoped=false means the date could not be read (three-state: that is
// reported as could-not-scope, never rounded to either side).
func effectivenessOwed(f Finding) (fatal, scoped bool) {
	opened, ok := findingOpenDate(f)
	if !ok {
		return false, false
	}
	return !opened.Before(effectivenessBoundaryTime()), true
}

// effectivenessRoute routes a message to the
// PROBLEM or NOTICE list per the transition scoping (effectivenessOwed).
func effectivenessRoute(f Finding, msg string, problems, notices *[]string) {
	fatal, scoped := effectivenessOwed(f)
	switch {
	case fatal:
		*problems = append(*problems, msg)
	case scoped:
		*notices = append(*notices, msg+fmt.Sprintf(" (advisory: dated before %s, the day the record was introduced — the inherited register is not made fatal)", effectivenessBoundary))
	default:
		*notices = append(*notices, msg+fmt.Sprintf(" (advisory: the finding's date %q is not YYYY-MM-DD, so it could not be placed against the %s boundary — could-not-check, not a pass)", strings.TrimSpace(f.Date), effectivenessBoundary))
	}
}

// effectivenessClosureMessages is the generic closure obligation: a `resolved: yes`
// finding with no effectiveness record owes one — a PROBLEM when dated on or after
// effectivenessBoundary, a NOTICE before it. It skips the finding class that
// findingControlUnfiredMessages (findingcontrol.go) owns, so one defect is reported
// once, and it skips a partial record (effectivenessTripleProblems owns that).
func effectivenessClosureMessages(findings []Finding, streams []*Stream) (problems, notices []string) {
	for _, f := range findings {
		if !f.Resolved || !effectivenessAbsent(f) {
			continue
		}
		if recurringLandedControl(f, streams) {
			continue // owned by findingControlUnfiredMessages
		}
		msg := fmt.Sprintf(
			"[%s] findings register: %s: resolved: yes but no effectiveness record (missing effectiveness, effectiveness-date, effectiveness-by). A finding closes on a fired control, not on a fix commit: record the command, verbatim, that re-establishes the failure mode is gone, the date it was run, and who ran it. %s",
			effTagMissing, effectivenessFindingID(f), effectivenessBoundaryNote)
		effectivenessRoute(f, msg, &problems, &notices)
	}
	return problems, notices
}
