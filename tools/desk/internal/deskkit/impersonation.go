package deskkit

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Impersonated human-ruling guard.
//
// THE SHAPE. An agent can post a comment headed "Decision (Alex, …)" on a shared
// issue, from `shared-agent` — the shared account Alex ALSO drives from their own
// CLI. A reviewer reads it as Alex's instruction and shapes a verdict around it.
// There is no forgery in the technical sense: `shared-agent` genuinely is
// shared between the human and every agent, so a `shared-agent` comment saying
// "I (Alex) decided X" is textually INDISTINGUISHABLE from Alex deciding X. Only
// `ada`, Alex's distinct human GitHub login (ASSAY_HUMAN_LOGIN_MAP: alex:ada), proves
// it. See the pr-review-desk SKILL.md dispatch template ("Only `ada` proves
// Alex; `shared-agent` proves nothing") for the process-side half of this fix — this
// file is the mechanical half.
//
// THE MECHANICAL FIX. Every desk tool that writes prose to GitHub — deskpost
// (comment/review), deskpr, deskfile, deskevidence, deskreply — routes its
// outward body through ScanSurface/BodyCheck (bodycheck.go) before it ever posts.
// None of those tools EVER posts as a human: they post as the reviewer App, or
// under the shared `shared-agent` login, never as `ada`. So a body that reads as
// a RULING/DECISION claim attributed BY NAME to a configured human is refused at
// write time — not because the scan can tell whether the claim is TRUE (it
// can't; that is unanswerable from the artifact, per the issue), but because the
// write path is STRUCTURALLY incapable of being that human, so the honest form of
// the sentence never needs writing it in their voice at all: relay it instead
// ("relaying <name>'s direction from <link>" — the dispatch-template's endorsed
// wording), pointing at the primary source. A gate keyed on "does this look like
// a ruling" is prose-based and gameable; what makes THIS gate mechanical is that
// it does not ask whether the claim is true — it refuses categorically, on every
// write this account can make, independent of content.
//
// SCOPE. This governs OUTWARD writes only (what this codebase's own tools post).
// It does not and cannot stop a human from typing quoted text into a comment by
// hand, and it does not replace the read-side discipline in the dispatch
// template (check the ACCOUNT of an existing comment, never its text) — that
// half cannot be mechanized while `shared-agent` remains genuinely shared (issue
// #45 control 4, tracked as the custody split in #44). What it closes is the
// half that CAN be mechanized today: this tree's own tools refusing to emit the
// impersonating shape in the first place.

// rulingWord matches the label that opens a ruling/decision claim.
const rulingWord = `(?:decision|ruling)`

// humanRulingRes compiles the three attribution shapes this guard recognises for
// a given configured human name. All three require the ruling/decision LABEL
// AND the name to co-occur in one of the specific attribution shapes seen in the
// wild or named by the issue — a bare "Alex" and a bare "Decision" appearing
// anywhere in the same body do NOT match; ordinary prose that discusses a past
// decision or mentions Alex by name is untouched.
//
//   - parenRe:       "Decision (Alex, …)" / "## Ruling (Alex)" — the
//     literal instance shape: label, then the name in parens within a short
//     span (a heading's trailing annotation).
//   - dashRe:        "Ruling: do X this way — Alex" / "Decision: … -- Alex" — an
//     em/en-dash (or double-hyphen) attribution trailing the label at
//     end-of-line, the shape named directly by the Verify checklist ("Ruling: ...
//     — Alex").
//   - firstPersonRe: "I (Alex) have decided X" / "I, Alex, ruled that …" — the
//     first-person form the issue also names ("I (Alex) have decided").
func humanRulingRes(name string) (parenRe, dashRe, firstPersonRe *regexp.Regexp) {
	q := regexp.QuoteMeta(name)
	parenRe = regexp.MustCompile(`(?i)\b` + rulingWord + `\b[^\n]{0,60}\(\s*` + q + `\b`)
	// [-\x{2013}\x{2014}]{1,2} covers a hyphen, an en dash, an em dash, or a
	// doubled hyphen ("--") used as an ASCII em-dash stand-in.
	dashRe = regexp.MustCompile(`(?im)\b` + rulingWord + `\b[^\n]{0,200}[-\x{2013}\x{2014}]{1,2}\s*` + q + `\s*$`)
	firstPersonRe = regexp.MustCompile(`(?i)\bI\s*(?:\(\s*` + q + `\s*\)|,\s*` + q + `\s*,?)\s*(?:have\s+|has\s+)?(?:decided|ruled?)\b`)
	return
}

// ImpersonatedRulingClaim scans text for a ruling/decision claim attributed BY
// NAME to one of the CONFIGURED human names (ASSAY_HUMAN_LOGIN_MAP — the same
// map trust.go's expectedID and the pr-review-desk dispatch template key off).
// Only configured names are checked: an unconfigured roster (or a roster naming
// nobody) has nothing to detect against and this always answers found=false,
// the same fail-shape as every other roster-gated check in this package (P1 —
// but here "closed" means "cannot detect", not "cannot be fooled": with no
// configured human there is no name this guard can recognise as impersonable).
//
// name is returned in its configured (lowercased) form, matching HumanLogins'
// keys and expectedID's lookup — callers that want a display form title-case it
// themselves (see the refusal message below).
//
// Iteration order over the configured names is irrelevant to correctness (any
// match refuses); it is still sorted for a deterministic refusal message when a
// body somehow names more than one configured human.
func ImpersonatedRulingClaim(text string) (name string, found bool) {
	humans := EffectiveConfig().HumanLogins
	if len(humans) == 0 {
		return "", false
	}
	names := make([]string, 0, len(humans))
	for n := range humans {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		parenRe, dashRe, firstPersonRe := humanRulingRes(n)
		if parenRe.MatchString(text) || dashRe.MatchString(text) || firstPersonRe.MatchString(text) {
			return n, true
		}
	}
	return "", false
}

// displayName title-cases a configured (lowercased) human name for the refusal
// message only — "alex" -> "Alex". Never used for comparison or lookup.
func displayName(name string) string {
	if name == "" {
		return name
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

// impersonationRefusal builds the refusal a caller (ScanSurface) returns when a
// body reads as a human-ruling claim attributed to name on surface.
func impersonationRefusal(surface, name string) error {
	disp := displayName(name)
	return Refused(fmt.Sprintf(
		"refused: %s reads as a RULING/DECISION claim attributed to %q — this write path never "+
			"posts as a human (issue #45: `shared-agent` proves nothing, only the login configured for "+
			"%q in ASSAY_HUMAN_LOGIN_MAP does), so text written in their voice is indistinguishable "+
			"from inventing it, whether or not it is true. Relay it instead: state what you observed "+
			"or were told and LINK where %s actually said it (\"relaying %s's direction from <link>\") "+
			"— never write \"Decision (%s, ...)\", \"Ruling: ... — %s\", or \"I (%s) have decided\" "+
			"from this account.",
		surface, disp, name, disp, disp, disp, disp, disp))
}
