package main

import (
	"fmt"
	"strings"
)

// The brief ruling-link lane: a pull request that ADDS or CHANGES a brief's
// frontmatter ruling: link (or the decision-issue: it points into) has that link
// resolved on the forge, whatever the brief's gate or status. The decision-gate
// hold resolves a ruling only when the change moves, relabels or drops a
// gate: human brief; a ruling link recorded in a change of its own was read by
// no one, so a bot's, an edited or a missing comment could stand in the brief as
// a human ruling. Resolution is the hold's own (resolveBriefRuling →
// resolveRulingOn): one resolver, one set of named reasons (registers-v1 §7.5).
// A link that cannot be resolved is refused, never skipped.
//
// The lane reads the two board snapshots the hold already built (the base as
// committed, the working tree as it stands), so it adds no Git read of its own.

// briefRulingSpecRef is the citation every refusal of this lane carries.
const briefRulingSpecRef = "lifecycle-v1 §4.5; registers-v1 §7.5"

// sameBriefRuling reports whether two readings of a brief record the same
// ruling link into the same decision issue.
func sameBriefRuling(a, b *gateBrief) bool {
	return a.Ruling == b.Ruling && a.DecisionIssue == b.DecisionIssue
}

// briefRulingSingle reports whether gb is one readable brief file whose
// frontmatter parsed, so its Ruling and DecisionIssue are that file's own.
func briefRulingSingle(gb *gateBrief) bool {
	return gb != nil && gb.Path != "" && gb.Ambiguous == "" && gb.Unreadable == "" && !gb.FMErr
}

// briefRulingRecordLinks is every ruling: value the board id's brief-file
// records carry. An ambiguous board id holds no single ruling (buildGateSnapshot
// clears it), so its records are the only place a link added to one of its
// files shows.
func briefRulingRecordLinks(gb *gateBrief) map[string]bool {
	out := map[string]bool{}
	if gb == nil {
		return out
	}
	for _, r := range gb.Records {
		if !strings.HasPrefix(r, "file ") {
			continue
		}
		_, rest, ok := strings.Cut(r, " ruling=")
		if !ok {
			continue
		}
		if i := strings.LastIndex(rest, " fmerr="); i >= 0 {
			rest = rest[:i]
		}
		if rest != "" {
			out[rest] = true
		}
	}
	return out
}

// briefRulingIsNew reports whether the brief h, after the change, cites a
// ruling link the base did not already carry for it. The same board id with
// the same link and decision issue is unchanged (a move into done/ keeps the
// board id); a board id whose brief file the base lacks is unchanged when a
// brief file the change removed carried that link and issue (a renumber).
func briefRulingIsNew(base, head gateSnapshot, h *gateBrief) bool {
	if b := base[h.Key]; b != nil && b.Path != "" {
		return !(briefRulingSingle(b) && sameBriefRuling(b, h))
	}
	for _, k := range sortedGateKeys(base) {
		d := base[k]
		if (head[k] == nil || head[k].Path == "") && briefRulingSingle(d) && sameBriefRuling(d, h) {
			return false
		}
	}
	return true
}

// briefRulingLane judges every ruling link the working tree at root adds or
// changes against base. Each such link must pass layer one (a decision issue
// recorded, a well-formed issue-comment URL on it) and then resolve on the
// forge through resolveBriefRuling. The forge client, login map and aliases are
// read only when a link reaches layer two. A record the board cannot read is
// left to the hold, which refuses every one a change leaves (judgeDecisionGate).
func briefRulingLane(root, repo, base string, baseSnap, headSnap gateSnapshot) []decisionGateResult {
	var (
		out     []decisionGateResult
		c       *ghClient
		logins  map[string]string
		aliases []string
		ready   bool
	)
	refuse := func(h *gateBrief, format string, a ...any) {
		out = append(out, decisionGateResult{Board: h.Board, Refused: true,
			Detail: fmt.Sprintf("%s: ", h.Path) + fmt.Sprintf(format, a...) + " (" + briefRulingSpecRef + ")"})
	}
	for _, k := range sortedGateKeys(headSnap) {
		h := headSnap[k]
		if strings.HasPrefix(k, gateScopeKeyPrefix) || h.Path == "" || h.Unreadable != "" {
			continue
		}
		b := baseSnap[k]
		if b != nil && b.fingerprint() == h.fingerprint() {
			continue // no brief file of this board id changed a field the lane reads
		}
		if h.FMErr {
			refuse(h, "the brief's frontmatter does not parse, so whether this change adds a ruling link cannot be judged — fail-closed")
			continue
		}
		if h.Ambiguous != "" {
			had := briefRulingRecordLinks(b)
			for link := range briefRulingRecordLinks(h) {
				if !had[link] {
					refuse(h, "this change cites ruling: %s on board id %s, which resolves to more than one record (%s), so no single brief's decision issue can bind the link — fail-closed", link, h.Board, h.Ambiguous)
					break
				}
			}
			continue
		}
		if h.Ruling == "" || !briefRulingIsNew(baseSnap, headSnap, h) {
			continue
		}
		if h.DecisionIssue == 0 {
			refuse(h, "this change cites ruling: %s, but no decision issue is recorded (frontmatter decision-issue: is absent), so the link has no issue to rule on", h.Ruling)
			continue
		}
		link, ok := parseRulingURL(h.Ruling)
		if !ok {
			refuse(h, "this change cites ruling: %q, which is not an issue-comment URL of the form %s — %s", h.Ruling, rulingURLGrammar, rulingMalformed)
			continue
		}
		if link.Issue != h.DecisionIssue {
			refuse(h, "this change cites ruling: %s, a comment on #%d, not on the recorded decision issue #%d — %s", h.Ruling, link.Issue, h.DecisionIssue, rulingUnrelatedIssue)
			continue
		}
		if repo == "" {
			refuse(h, "this change cites ruling: %s; the repository could not be determined from the origin remote, so the link cannot be resolved — %s", h.Ruling, rulingUnresolvable)
			continue
		}
		if !ready {
			c, logins, aliases = decisionGateClientFn(), decisionGateLoginsFn(), decisionGateAliases(root, base, repo)
			ready = true
		}
		var briefIDs []string
		if h.BriefID != "" {
			briefIDs = []string{h.BriefID}
		}
		names := h.Spellings
		if len(names) == 0 {
			names = []string{h.Board}
		}
		o := resolveBriefRuling(c, repo, h.Path, h.Ruling, names, briefIDs, logins, aliases)
		if o.Present && o.Reason == rulingOK {
			out = append(out, decisionGateResult{Board: h.Board,
				Detail: fmt.Sprintf("%s: ruling: link added or changed by this change, ruled by %s (%s)", h.Path, o.Author, o.Detail)})
			continue
		}
		refuse(h, "this change cites ruling: %s, which does not stand as a human ruling on decision issue #%d: %s — %s", h.Ruling, h.DecisionIssue, o.Reason, o.Detail)
	}
	return out
}
