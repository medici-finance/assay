package deskkit

// rulinggate.go — the DECLARED SOURCE for "did a human sign ruling R-N?".
//
// Two desk tools now gate their whole authority on a ruling in
// docs/streams/issue-flow/rulings.md: deskclose on R-1 (may it close an issue) and
// deskmerge on R-5 (may it bring a PR current with main). The parse is small and the
// temptation to re-write it per tool is large — which is exactly the derive-or-diff
// failure mode: two hand-maintained copies of one fact, drifting apart until one of
// them reads an unsigned ruling as signed.
//
// So the parse lives here, once. deskclose still carries its own copy (migrating a
// landed, verified tool's 1200-line suite is a separate change); that copy is pinned to
// THIS one by a differential test — cmd/deskclose/rulinggate_diff_test.go feeds both
// implementations the same corpus and requires identical (url, exit-code) outcomes. A
// drift between them reddens CI rather than quietly widening someone's authority.
//
// What this file does NOT do, deliberately: it never decides that a ruling is
// AUTHORIZED. It extracts the URL the register CLAIMS is the sign-off. Turning that
// claim into authority requires FETCHING the artifact and verifying its author against
// the roster-pinned blessing authority, which is a network act each tool performs for
// itself. A file in the caller's own worktree can only change WHICH URL gets fetched.

import (
	"os"
	"regexp"
	"strings"
)

// SignOffLineRe matches a ruling's Sign-off line and captures its remainder.
var SignOffLineRe = regexp.MustCompile(`(?i)^\s*\*\*Sign-?off:?\*\*\s*(.*)$`)

// CommentPermalinkRe parses a GitHub issue/PR comment permalink into (owner, repo,
// kind, item number, comment id). Anchorless URLs do not match: a link to a whole
// THREAD is not an authorization, because a thread's content is written by whoever
// shows up.
var CommentPermalinkRe = regexp.MustCompile(
	`^https://github\.com/([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+)/(issues|pull)/(\d+)#issuecomment-(\d+)$`)

var httpsURLRe = regexp.MustCompile(`https://[^\s)\]>,"']+`)

// FirstHTTPSURL returns the first https:// URL in s, or "".
func FirstHTTPSURL(s string) string { return httpsURLRe.FindString(s) }

// ReadRulingSignOff extracts rulingID's sign-off URL from the rulings register at path.
//
// Three states, and they are three DIFFERENT answers — this is the three-state
// instrument invariant applied to an authority question:
//
//   - the file or the section cannot be read      → Unverifiable (6), could-not-check
//   - the section is there and the line is EMPTY  → Refused (5), the ruling is unsigned
//   - a URL is present                            → returned, for the caller to FETCH
//
// An unsigned ruling is a POSITIVE determination ("the human has not granted this"),
// which is why it is a refusal rather than could-not-check. An unreadable register is
// the opposite epistemic state and must never be reported as either signed or unsigned.
//
// tool names the caller, for the refusal text only; it never affects the outcome.
func ReadRulingSignOff(path, rulingID, tool string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", Unverifiable(
			"could-not-check: cannot read the rulings register at "+path+
				" — "+tool+" cannot establish that "+rulingID+" granted anything, so it does nothing "+
				"(pass --rulings <path> if your checkout root differs)", err)
	}
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	inSection := false
	for _, ln := range lines {
		if strings.HasPrefix(ln, "## ") {
			// Section boundaries: enter on the ruling's heading, LEAVE on the next
			// one. Without the leave arm a LATER ruling's signed sign-off would be
			// read as this one's — the bug that turns "one ruling was signed" into
			// "all of them were". R-5 sits last in the register today, so for
			// deskmerge this arm is what stops R-1..R-4's lines being mistaken for
			// R-5's; for deskclose it is the mirror image.
			inSection = strings.HasPrefix(ln, "## "+rulingID+" ") || strings.TrimSpace(ln) == "## "+rulingID
			continue
		}
		if !inSection {
			continue
		}
		m := SignOffLineRe.FindStringSubmatch(ln)
		if m == nil {
			continue
		}
		url := FirstHTTPSURL(strings.TrimSpace(m[1]))
		if url == "" {
			return "", Refused(
				"refused: " + rulingID + " carries no sign-off URL in " + path + " — the authority " +
					tool + " runs on is UNSIGNED, so it holds none. " + tool + " is inert until a human " +
					"records an acceptance artifact on that line. This is not a bug to route around: " +
					"it is the gate working.")
		}
		return url, nil
	}
	return "", Unverifiable(
		"could-not-check: "+path+" has no `## "+rulingID+"` section with a Sign-off line — "+
			"the register "+tool+" implements is not in the shape it expects, so no grant is established", nil)
}
