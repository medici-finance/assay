package deskkit

// rulinggate.go — the (url, error) adapter over the ONE sign-off reader.
//
// Two desk tools gate their whole authority on a ruling in
// docs/streams/issue-flow/rulings.md: deskclose on R-1 (may it close an issue) and
// deskmerge on R-5 (may it bring a PR current with main). Both must ask "did a human
// sign this ruling?" and get the SAME answer — a second parser is a second answer, and
// the two disagree silently until one reads an ambiguous register as signed while the
// other declines.
//
// So there is exactly one parser, and it lives in signoff.go: ReadSignOff /
// ReadSignOffFromText, block-oriented, three-state (issue #1094). deskclose consumes it
// directly through its SignOff value. Some callers — deskmerge, and deskclose's own
// R-1 gate — want the older (url, error) contract instead of the three-state value.
// ReadRulingSignOff below is that ADAPTER: it delegates the whole determination to
// ReadSignOff and only maps the three states onto (url, error) exit codes plus the
// operator wording. It decides nothing about the register itself. A differential test —
// cmd/deskclose/rulinggate_diff_test.go — pins deskclose's mapping and this one together
// so they cannot drift.
//
// What this file does NOT do, deliberately: it never decides that a ruling is
// AUTHORIZED. It surfaces the URL the register CLAIMS is the sign-off. Turning that
// claim into authority requires FETCHING the artifact and verifying its author against
// the roster-pinned blessing authority, which is a network act each tool performs for
// itself. A file in the caller's own worktree can only change WHICH URL gets fetched.

import "regexp"

// CommentPermalinkRe parses a GitHub issue/PR comment permalink into (owner, repo,
// kind, item number, comment id). Anchorless URLs do not match: a link to a whole
// THREAD is not an authorization, because a thread's content is written by whoever
// shows up.
var CommentPermalinkRe = regexp.MustCompile(
	`^https://github\.com/([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+)/(issues|pull)/(\d+)#issuecomment-(\d+)$`)

// ReadRulingSignOff extracts rulingID's sign-off URL from the rulings register at path,
// for callers whose contract is (url, error) rather than the three-state SignOff value.
//
// It is a THIN ADAPTER over ReadSignOff — the one block-oriented reader in signoff.go —
// and decides nothing of its own. That is the "exactly one parser" invariant (issue
// #1094) made real: deskmerge's R-5 gate and deskclose's R-1 gate now reach the SAME
// determination code, so an ambiguous, multi-URL or continuation-wrapped register cannot
// read as signed to one gate and could-not-read to the other. What remains local here is
// only the mapping of the three states onto exit codes and the operator wording:
//
//   - SignOffGranted      → the URL, nil          the caller FETCHES it next
//   - SignOffUnsigned     → "", Refused (5)       the ruling is unsigned
//   - SignOffCouldNotRead → "", Unverifiable (6)  the register could not be read
//
// An unsigned ruling is a POSITIVE determination ("the human has not granted this"),
// which is why it is a refusal rather than could-not-check. An unreadable register is
// the opposite epistemic state and must never be reported as either signed or unsigned;
// ReadSignOff keeps those apart and this mapping preserves the distinction.
//
// tool names the caller, for the message text only; it never affects the outcome.
func ReadRulingSignOff(path, rulingID, tool string) (string, error) {
	so := ReadSignOff(path, rulingID)
	switch so.State {
	case SignOffGranted:
		return so.URL, nil
	case SignOffUnsigned:
		return "", Refused(
			"refused: " + so.Detail + " (" + StripControl(path) + ") — the authority " + tool +
				" runs on is UNSIGNED, so it holds none. " + tool + " is inert until a human records " +
				"an acceptance artifact on that line. This is not a bug to route around: it is the " +
				"gate working.")
	default:
		return "", Unverifiable(
			"could-not-check: "+so.Detail+" — "+tool+" cannot establish that "+rulingID+
				" granted anything, so it does nothing (pass --rulings <path> if your checkout root "+
				"differs)", nil)
	}
}
