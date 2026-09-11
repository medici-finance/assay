package deskkit

// checkonlycr.go is the CANONICAL reader for the two marker lines that carry the
// check-only-CR exemption: `Blocked-On-Check:` on a CHANGES_REQUESTED, and
// `Cleared-Check-Run:` on the APPROVE that answers it.
//
// WHAT THE EXEMPTION IS FOR. deskflip refuses an APPROVE posted at an UNCHANGED head over a
// standing CHANGES_REQUESTED: nothing new exists to verify, so the re-approve cannot be a
// re-verification. That is the right default and it stays the default. But one legitimate
// case sat under it with no path out: a CR whose ONLY stated blocker was a required CHECK
// being red, where the check later turned green at the SAME head with no code push (a
// `changelog:skip` label applied by a human, a re-run of a flaked job). The reviewer's
// re-approve is then a true statement about a fact that genuinely changed — just not a fact
// that lives in the diff. Without an exemption the only ways forward were a no-op push
// (gaming the head-move rule) or a human dismissing the review by hand on every such PR.
//
// WHY MARKERS AND NOT PROSE. The narrowed exemption was authorized on the condition that
// "check-only" is established by the CR body's EXPLICIT SHAPE, not by heuristics over prose.
// A reader that tried to infer "this CR names no other finding" from English would be
// guessing, and guessing on a GRANT path is how a laundering hole opens: a CR carrying three
// findings and the word "changelog" would read as check-only. So the reviewer DECLARES it,
// on one line, in a fixed form. No declaration is no exemption — that is the whole of the
// detection, and it is why the absence of the line is not a soft signal but a full stop.
//
// BOTH READS ARE GRANT-DIRECTION, so both skip fenced code blocks (SkipFenced), matching
// HasSecurityReviewPass. A marker quoted inside a fence is documentation explaining the
// format — this file's own doc comment would otherwise qualify — and documentation is never
// a grant. The FAIL-direction asymmetry that HasSecurityReviewFail carries has no analogue
// here: neither of these lines can ever BLOCK anything, so there is no retraction that a
// fence could hide.

import "regexp"

// blockedOnCheck matches the CR-side declaration: the reviewer states that the ONLY finding
// in this CHANGES_REQUESTED is the named check being red.
//
//	Blocked-On-Check: changelog
//
// The capture is deliberately permissive about WHAT a check may be called (forge check names
// carry spaces, slashes, parentheses — `build (ubuntu-latest)` is an ordinary one) and
// strict about the line SHAPE: whole-line anchored, exactly as the `Security-Review:` markers
// are, so the same marker inside a sentence, a table cell, or a `> ` quote does not declare
// anything. The value may not be empty — a declaration that names no check establishes
// nothing.
var blockedOnCheck = regexp.MustCompile(`(?i)^[ \t]*Blocked-On-Check:[ \t]*(\S.*?)[ \t\r]*$`)

// clearedCheckRun matches the APPROVE-side citation: the specific check RUN the reviewer
// says turned green.
//
//	Cleared-Check-Run: 41234567890
//
// The value is digits only. It is a RUN identifier, not a check name: the point of citing it
// is that it names one EXECUTION, so the same citation cannot be satisfied by an older run of
// the same check that was green all along. A non-numeric value does not match at all, so a
// reviewer who writes the check's NAME here has cited nothing and the exemption withholds.
var clearedCheckRun = regexp.MustCompile(`(?i)^[ \t]*Cleared-Check-Run:[ \t]*(\d+)[ \t\r]*$`)

// BlockedOnCheckName returns the check a CHANGES_REQUESTED body declares as its SOLE
// finding, or "" when the body makes no such declaration (or makes two that disagree).
//
// "" is the answer that means "no exemption is claimed", and it is by far the common one:
// an ordinary CR carrying real findings has no reason to write this line.
func BlockedOnCheckName(body string) string {
	return SoleVerdictMarkerValue(body, blockedOnCheck, SkipFenced)
}

// ClearedCheckRunID returns the check-run id an APPROVED body cites as the run that turned
// green, or "" when the body cites none (or cites two that disagree).
//
// The returned string is compared against CheckRun.ID, which is "" for a run the forge gave
// no id — so a run without an identifier can never be the one a citation matched.
func ClearedCheckRunID(body string) string {
	return SoleVerdictMarkerValue(body, clearedCheckRun, SkipFenced)
}
