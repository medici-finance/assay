package deskkit

// offlinecheck.go — the ONE sentence that tells an operator how to rehearse a write
// offline, and the one place it is written.
//
// WHY THIS EXISTS. Two thirds of one verb's refusals are body-SCHEMA refusals: a review
// body with no `Verdict:` line, a review body with no `## ` heading, a PR body with no
// `Brief:` / `Issue:` trailer, a missing required flag. Every one of them is decidable with
// no network at all, and the verb that reports them already ships an offline rehearsal
// (`--dry-run`: run every check, stop before the write, exit 0, audited as a result class
// neither write meter counts). Not one of the refusals mentioned it. Measured on one
// operating desk host over 32 days: 1,065 body-schema refusals against 261 rehearsals.
//
// The gap is not knowledge, it is PLACEMENT. The rehearsal flag is documented in the verb's
// `--help`, which is precisely the text an operator who is mid-write does not have open; the
// refusal is the text they are looking at. So the refusal carries the pointer.
//
// NO FREE TEXT. The hint takes a tool name and a flag name and returns a fixed template. It
// never interpolates a body, a path, an error message, a repo, or anything else the caller
// might be holding — a refusal that quoted the offending content would turn the safety
// message into the disclosure. Both parameters are compiled-in constants at every call site.
//
// ONE COPY. The template literal lives here and nowhere else, so a second copy cannot drift
// from the first; a test asserts the literal occurs exactly once in the tree.

import "strings"

// offlineHintTemplate is the fixed sentence. `%TOOL%` and `%FLAG%` are the only
// substitutions, and both are filled from compiled-in constants.
//
// It is a package-level constant rather than a format string so the single-occurrence test
// has one literal to count, and so no caller can pass it to a formatter with arguments of
// its own.
const offlineHintTemplate = " Rehearse this offline first: `%TOOL% … %FLAG%` runs every " +
	"check and stops before the write, so getting the shape right costs no attempt."

// OfflineCheckHint returns the sentence to append to a schema/body refusal, naming the
// offline check that would have caught it. It begins with a single leading space so a
// caller appends it directly to a refusal message that already ends in a full stop.
//
// tool is the verb's own name (e.g. "deskpost"); flag is the rehearsal flag INCLUDING its
// leading dashes (e.g. "--dry-run", "--check"). Both must be compiled-in constants: passing
// caller-supplied text through here is out of scope and is a finding.
//
// With either parameter empty the hint is empty — a pointer to a flag that was not named
// would be worse than none, and a silently malformed sentence in a refusal is exactly the
// kind of thing nobody reports.
func OfflineCheckHint(tool, flag string) string {
	if strings.TrimSpace(tool) == "" || strings.TrimSpace(flag) == "" {
		return ""
	}
	s := strings.ReplaceAll(offlineHintTemplate, "%TOOL%", tool)
	return strings.ReplaceAll(s, "%FLAG%", flag)
}

// SchemaRefusal builds a refusal for a SHAPE problem — a body, a flag, a trailer, a
// heading: something decidable with no network — with the offline-check hint appended.
//
// Every body/schema refusal in the write verbs is routed through this one constructor so
// the hint cannot be present on some and absent on others. A refusal about remote STATE
// (the head moved, the budget is spent, the repo is not allowed) is NOT a schema refusal
// and must not use this: rehearsing it offline would not have helped, and pointing at the
// rehearsal flag there would be advice that does not work.
func SchemaRefusal(tool, flag, msg string) *DeskError {
	return Refused(msg + OfflineCheckHint(tool, flag))
}
