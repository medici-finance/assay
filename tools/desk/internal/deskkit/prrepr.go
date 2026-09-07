package deskkit

import (
	"encoding/json"
	"strings"
)

// prrepr.go — the already-represented / phantom reconciliation.
//
// A dispatch plan and a dispatcher both need one answer: does the brief a row names ALREADY have a
// pull request? A worker spawned for a brief whose work is already in an open — or already merged —
// PR re-derives that fact at a large token cost and returns nothing. The naive check keys on a
// derived BRANCH name, but the live PR is branched under the claim-key form
// (`feat/<repo>--<stream>--<NN>`) while the plan derives `feat/<stream>-<NN>`, so a branch-name
// match misses it and the phantom slips through. The identity that does NOT drift is the PR body's
// `Brief:` link trailer (deskkit/trailer.go), which every desk PR is required to carry — so the
// reconciliation here keys on THAT, never on a branch spelling.

// PRRef is the minimal shape the reconciliation reads from a repo's PR list: the number, the state,
// and the body carrying the `Brief:` link trailer. It is exactly what one row of
// `gh pr list --json number,state,body` unmarshals into.
type PRRef struct {
	Number int    `json:"number"`
	State  string `json:"state"` // gh's uppercased state: OPEN | CLOSED | MERGED
	Body   string `json:"body"`
}

// ParsePRList unmarshals the JSON a `gh pr list --json number,state,body` prints. Empty or
// whitespace-only input is the empty list — the caller owns the could-not-read error path around
// the transport, so an empty RESULT and a failed READ are never conflated here.
func ParsePRList(raw []byte) ([]PRRef, error) {
	s := strings.TrimSpace(string(raw))
	if s == "" {
		return nil, nil
	}
	var prs []PRRef
	if err := json.Unmarshal([]byte(s), &prs); err != nil {
		return nil, err
	}
	return prs, nil
}

// prRepresents reports whether a PR in this state REPRESENTS its brief. Only an OPEN or a MERGED PR
// does: a CLOSED-unmerged PR is abandoned work, so the row it named is dispatchable again — which is
// the whole reason the check keys on state and not merely on a PR's existence. The ~1-in-6 already-
// MERGED reality is why MERGED counts here and a plain "is there an open PR" would still leak.
func prRepresents(state string) bool {
	switch strings.ToUpper(strings.TrimSpace(state)) {
	case "OPEN", "MERGED":
		return true
	}
	return false
}

// RepresentedBriefs maps each brief id (`<stream>/<NN>`, lower-cased) to the number of an OPEN or
// MERGED PR that delivers it, keyed on the PR body's `Brief:` link trailer — NEVER a branch name.
// When several PRs represent one brief the FIRST in list order wins (deterministic; the caller needs
// only one to report). A PR with no parseable `Brief:` trailer, or one carrying an `Issue:` link,
// contributes nothing — it names no brief.
func RepresentedBriefs(prs []PRRef) map[string]int {
	out := map[string]int{}
	for _, pr := range prs {
		if !prRepresents(pr.State) {
			continue
		}
		trs, err := ParseTrailers([]byte(pr.Body))
		if err != nil {
			// A body whose trailers do not parse (a multiplicity error) names no brief this check
			// can trust; it is skipped rather than guessed at.
			continue
		}
		for _, t := range trs {
			if t.Kind != TrailerBrief {
				continue
			}
			// Canonicalize the trailer value to the slash-form brief id through the SAME
			// reduction deskpr's create-time validation uses (SplitBriefTrailer). A PR
			// authored with the accepted colon form (`Brief: <stream>:<NN>`) must key on
			// `<stream>/<NN>` so it matches the plan's slash-form item id — otherwise a real
			// OPEN/MERGED colon-form PR is missed and a fresh worker is dispatched over it.
			id := CanonicalBriefID(t.Value)
			if id == "" {
				continue
			}
			if _, seen := out[id]; !seen {
				out[id] = pr.Number
			}
		}
	}
	return out
}

// BriefRepresentedPR reports whether briefID (`<stream>/<NN>`) already has an OPEN or MERGED PR in
// prs, and that PR's number. The match is case-insensitive on the trimmed id. An empty briefID never
// matches — the caller reduced a key it could not turn into a brief id, and "no brief id" is not a
// reason to claim a phantom.
func BriefRepresentedPR(briefID string, prs []PRRef) (int, bool) {
	id := strings.ToLower(strings.TrimSpace(briefID))
	if id == "" {
		return 0, false
	}
	n, ok := RepresentedBriefs(prs)[id]
	return n, ok
}

// RepresentedBriefSet is the lower-cased brief-id set (`<stream>/<NN>`) of every brief with an OPEN
// or MERGED PR — the form the plan's already-represented exclusion consumes, where the PR number is
// not needed. It is RepresentedBriefs reduced to its keys.
func RepresentedBriefSet(prs []PRRef) map[string]bool {
	m := RepresentedBriefs(prs)
	out := make(map[string]bool, len(m))
	for id := range m {
		out[id] = true
	}
	return out
}
