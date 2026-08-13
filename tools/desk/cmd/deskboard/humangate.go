package main

// humangate.go — the machine-readable HUMAN-GATE declaration.
//
// The defect: `classify()` reads any App APPROVED at head plus green CI as
// flip-and-merge-eligible, and the ONLY brake is the risk-path/security-review pair.
// "Risk-classed" is a SECURITY-SURFACE list; "human gate" is a different set — an
// integrity-check change, an irreversible operation, a statusgen gate-semantics change
// can be human-gate while touching no security path at all (#223 is the live instance:
// title `[HUMAN GATE]`, shown as MERGE-NOW). For that whole class the board was
// actively instructing a desk to merge, and ranking it FIRST while doing so.
//
// The declaration has to be derivable from the PR ALONE, because the board reads PRs.
// Three forms are accepted, any one of which is sufficient:
//
//	label   human-gate            (case-insensitive; also `gate:human` / `gate: human`)
//	title   [HUMAN GATE]          (case-insensitive; hyphen or space)
//	body    Gate: human           (a line of its own; mirrors the owning brief's key)
//
// Deliberately NOT prose-sniffing. "This is a human gate" in a paragraph does not
// count: a gate that describes intent rather than asserting a machine-checkable
// property is the exact defect #223 exists to close, and reproducing it here would be
// the same mistake one level up.
//
// What the board then does is NARROWER than the issue's suggested fix, and the
// difference is deliberate: the issue proposes an action that "never resolves to
// MERGE-NOW/FLIP", but the standing house rule is that a human gate bars the MERGE and
// the done-flip, NOT the desk's ready-flip (the desk still flips a human-gate PR ready
// once its artifacts are complete — that is what "ready for HUMAN review" means). So
// HUMAN-GATE replaces MERGE-NOW/FLIP as the ACTION, never reads as "merge now", and is
// excluded from the MERGE-NOW count and decay banner — while its note tells the desk
// plainly that a ready-flip is still its call and the merge never is.

import "strings"

// humanGateLabels are the label spellings that declare a human gate.
var humanGateLabels = map[string]bool{
	"human-gate":  true,
	"human gate":  true,
	"gate:human":  true,
	"gate: human": true,
}

// humanGateTitleMarkers are the bracketed title markers that declare a human gate.
var humanGateTitleMarkers = []string{"[human gate]", "[human-gate]"}

// humanGateDeclared reports whether a PR carries a machine-readable human-gate
// declaration, and which form carried it (for the row note — a desk told to stop is
// owed the reason, and a wrong stop must be traceable to the thing that caused it).
func humanGateDeclared(title, body string, labels []string) (bool, string) {
	for _, l := range labels {
		if humanGateLabels[strings.ToLower(strings.TrimSpace(l))] {
			return true, "label " + strings.TrimSpace(l)
		}
	}
	lt := strings.ToLower(title)
	for _, m := range humanGateTitleMarkers {
		if strings.Contains(lt, m) {
			return true, "title marker " + m
		}
	}
	for _, ln := range strings.Split(body, "\n") {
		s := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(ln, "\r")))
		s = strings.TrimSpace(strings.TrimPrefix(s, ">")) // a quoted declaration still declares
		if s == "gate: human" || s == "gate:human" {
			return true, "body line 'Gate: human'"
		}
	}
	return false, ""
}

// humanGateNote is the row note for a HUMAN-GATE action. It states the two halves
// separately, because conflating them is what produced the defect: the desk MAY still
// flip a draft ready; the desk may NEVER merge, and an App approve is not a human.
// ciPhrase (#400 N9) is how the caller described the CI state: "CI green" only when a
// check actually passed, and the no-CI-configured wording when the greenness was vacuous.
// Passed in rather than re-derived so the human-gate row can never assert a verdict the
// row above it withheld.
func humanGateNote(draft bool, reason, ciPhrase string) string {
	head := "HUMAN GATE declared (" + reason + ") — "
	if draft {
		return head + reviewerBotDisplay() + " APPROVED at head, " + ciPhrase + ": the desk may flip it ready, then STOP. " +
			"The MERGE is the human's — never merge on an App approve alone (#241)."
	}
	return head + reviewerBotDisplay() + " APPROVED at head, " + ciPhrase + ", already ready: awaiting the HUMAN. " +
		"The MERGE is the human's — never merge on an App approve alone (#241)."
}
