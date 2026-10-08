package main

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Finding→control closure (coder-skills-review/03). Adapts procoder's blocking
// lessons-loop: a recurring bug class must land a PERMANENT adaptation (a
// lint/check, a pinned test, a rubric line) before the class counts closed. The
// house records findings rigorously, but finding→landed-control was pure
// convention — a recurring-class finding could sit acknowledged forever with no
// control landing and nothing surfaced the debt.
//
// This is the surfacing check. It is ADVISORY (NOTICE, never a hard PROBLEM) this
// phase, deliberately — the same escalation posture as the gate-why worklist
// NOTICE. A hard gate over an unclassified legacy backlog would only manufacture
// false-positives: the backlog must be classifiable (findings opting in via
// `class: recurring`) before the debt can honestly be forced. Escalation to a
// hard error is a later, separate decision, not something this check pre-empts.
//
// The two frontmatter fields it reads are both OPTIONAL (Finding.Class,
// Finding.Control); an absent `class:` reads as one-off and is never flagged, so
// the entire legacy register stays silent until an author opts a finding into the
// recurring class.

// ESCALATION (iso-9001/03). The advisory posture above governs the OPEN-finding
// surface (findingControlNotices). For the class this file already covers, the
// RESOLVED side is promoted from "a control landed" to "a control was shown to
// fire": findingControlUnfiredMessages raises a PROBLEM (NOTICE for a finding dated
// before effectivenessBoundary) when a recurring-class, resolved finding with a
// landed control carries no effectiveness record. It is a PRESENCE floor — it never
// judges the ADEQUACY of the named command — and one-off / unclassified findings
// stay exactly as before.

const recurringClass = "recurring"

// briefRefNumRe matches the second segment of a "<stream>/<NN>" control reference
// as a brief number (brief-v1 Num form: digits with an optional trailing letter,
// e.g. "04", "12a"). It is what distinguishes a brief reference from a
// check-name / pinned-test path that also contains a slash (e.g.
// "tools/skillslint" or "statusgen/humanstamp_test.go"): those do NOT have a
// brief-number tail, so they are treated as already-landed artifacts, not as a
// board row to be resolved.
var briefRefNumRe = regexp.MustCompile(`^[0-9]{1,3}[a-z]?$`)

// controlLanded reports, for a finding's `control:` reference, whether the named
// control counts as a LANDED adaptation (closure the check trusts):
//
//   - A brief reference "<stream>/<NN>" is landed ONLY when that brief is `done`
//     on the board. A todo/in-progress/implemented/verified brief is a tracked
//     closure vehicle that has not yet landed, so the finding stays listed until
//     the brief is done — read from the same board data (`streams`) the rest of
//     the run reads.
//   - A brief reference that resolves to no known brief is treated as NOT landed
//     (a dangling control must not silence the finding).
//   - Any other reference shape — a lint/check name, a pinned-test path — is
//     trusted as already landed. The check cannot cheaply prove a check or test
//     exists and stays freshness-blind about it deliberately: this instrument
//     targets the MISSING control (no `control:` at all, or a control still
//     riding an unfinished brief), not an audit of named artifacts.
func controlLanded(control string, streams []*Stream) bool {
	control = strings.TrimSpace(control)
	if control == "" {
		return false
	}
	parts := strings.SplitN(control, "/", 2)
	if len(parts) != 2 {
		// Not "<stream>/<NN>" shaped — a bare check name or rubric reference.
		// Trusted as landed (see the doc comment).
		return true
	}
	streamName := strings.TrimSpace(parts[0])
	num := strings.TrimPrefix(strings.TrimSpace(parts[1]), "brief-")
	if !briefRefNumRe.MatchString(num) {
		// Has a slash but no brief-number tail (e.g. "tools/skillslint",
		// "statusgen/humanstamp_test.go") — a check name or pinned-test path, not a
		// brief reference. Trusted as landed.
		return true
	}
	// A genuine brief reference: landed iff the brief exists and is `done`.
	for _, s := range streams {
		if s.Name != streamName {
			continue
		}
		for i := range s.Briefs {
			if s.Briefs[i].Num == num {
				return s.Briefs[i].Status == "done"
			}
		}
	}
	// Brief reference to an unknown stream/brief — not landed.
	return false
}

// findingControlNotices returns one advisory NOTICE per recurring-class finding
// whose class is not yet closed by a landed control. A finding is flagged when it
// is:
//
//   - unresolved (a resolved finding is closed — inert, like a tombstone), AND
//   - `class: recurring` (one-off / unclassified findings are never flagged), AND
//   - either names no `control:`, or names a control that has not landed
//     (a `<stream>/<NN>` brief reference whose brief is not `done`).
//
// The NOTICE lists the finding with its age (days open), so an old unclosed
// recurring class reads as older debt. Undated findings are surfaced without a
// day count rather than dropped (three-state: a bad date is could-not-check, not
// "no debt"). Advisory only — this function never returns a PROBLEM.
func findingControlNotices(findings []Finding, streams []*Stream, now time.Time) []string {
	var out []string
	for _, f := range findings {
		if f.Resolved {
			continue // closed — inert, like a tombstone
		}
		if !strings.EqualFold(strings.TrimSpace(f.Class), recurringClass) {
			continue // one-off or unclassified — never flagged (advisory-first)
		}
		control := strings.TrimSpace(f.Control)
		if control != "" && controlLanded(control, streams) {
			continue // class closed by a landed control — silent
		}

		age := ""
		if opened, ok := findingOpenDate(f); ok {
			age = fmt.Sprintf("open %d days", int(now.Sub(opened).Hours()/24))
		} else {
			age = "open (undated)"
		}

		if control == "" {
			out = append(out, fmt.Sprintf(
				"finding-without-control: %s (recurring, %s) names no landed control — a recurring bug class must land a permanent adaptation (a lint/check, a pinned test, or a tracked brief) before it counts closed: %s",
				f.ID, age, f.Title))
		} else {
			out = append(out, fmt.Sprintf(
				"finding-without-control: %s (recurring, %s) — its control %q is a tracked brief that is not yet done; the finding stays listed until the control lands: %s",
				f.ID, age, control, f.Title))
		}
	}
	return out
}

// recurringLandedControl reports the exact class findingcontrol was written for,
// seen from its closed side: a `class: recurring` finding that is resolved and
// names a control that has LANDED. A one-off or unclassified finding (an absent
// `class:` reads as one-off) is never in this class — widening it would be a second
// decision hiding inside the effectiveness change (iso-9001/03).
func recurringLandedControl(f Finding, streams []*Stream) bool {
	if !f.Resolved || !strings.EqualFold(strings.TrimSpace(f.Class), recurringClass) {
		return false
	}
	control := strings.TrimSpace(f.Control)
	return control != "" && controlLanded(control, streams)
}

// findingControlUnfiredMessages extends findingcontrol from "a control LANDED" to
// "a control was shown to FIRE" (iso-9001/03), for the class it already covers and
// no other: a `class: recurring` finding that is `resolved: yes` with a landed
// `control:` but no effectiveness record is exactly the gap this check exists to
// surface. It is a PROBLEM when the finding is dated on or after
// effectivenessBoundary and a NOTICE before it (transition-scoped, so the inherited
// register is not made fatal).
//
// The check is a presence floor: it confirms a record EXISTS (and, with
// effectivenessTripleProblems, is complete and attributed). It does not judge the
// ADEQUACY of the named command — see effectivenessBoundaryNote. A partial record
// is left to effectivenessTripleProblems so a defect is reported once.
func findingControlUnfiredMessages(findings []Finding, streams []*Stream) (problems, notices []string) {
	for _, f := range findings {
		if !recurringLandedControl(f, streams) || !effectivenessAbsent(f) {
			continue
		}
		msg := fmt.Sprintf(
			"[%s] findings register: %s: recurring-class finding is resolved: yes with landed control %q, but records no effectiveness (missing effectiveness, effectiveness-date, effectiveness-by): a control that landed is not a control shown to fire. Name the command, verbatim, that re-establishes the failure mode is gone, the date it was run, and who ran it. %s",
			effTagUnfired, effectivenessFindingID(f), strings.TrimSpace(f.Control), effectivenessBoundaryNote)
		effectivenessRoute(f, msg, &problems, &notices)
	}
	return problems, notices
}
