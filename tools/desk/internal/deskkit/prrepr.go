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

// RepresentedPR is one brief's representing PR reduced to the two facts a scheduler needs to route
// it: the PR NUMBER (to name it) and whether it is MERGED. Merged=false is an OPEN PR — the only
// other state prRepresents admits (a closed-unmerged PR represents nothing). The distinction is
// load-bearing for `fanoutloop plan`: a brief with a MERGED PR is landed-unreconciled (its board
// cell just never flipped) and must NOT be re-offered, while a brief with an OPEN PR is started work
// to RESUME, not fresh dispatch.
type RepresentedPR struct {
	Number int
	Merged bool
}

// RepresentingPRsByBrief maps each brief id (`<stream>/<NN>`, lower-cased) to EVERY OPEN or MERGED
// PR that names it in a `Brief:` link trailer, in list order. It is the multi-valued form a caller
// needs when it may set one representing PR aside — the briefs-AUTHORING exemption (#1339,
// briefauthoring.go) drops a PR that only wrote the brief, and the PR behind it must then still be
// seen rather than hidden by a first-wins reduction. A PR with no parseable `Brief:` trailer, or one
// carrying an `Issue:` or `Authors:` link, contributes nothing — neither names a DELIVERED brief.
func RepresentingPRsByBrief(prs []PRRef) map[string][]RepresentedPR {
	out := map[string][]RepresentedPR{}
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
		merged := strings.EqualFold(strings.TrimSpace(pr.State), "MERGED")
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
			out[id] = append(out[id], RepresentedPR{Number: pr.Number, Merged: merged})
		}
	}
	return out
}

// RepresentedBriefPRs maps each brief id (`<stream>/<NN>`, lower-cased) to the OPEN or MERGED PR
// that delivers it — its number AND its state — keyed on the PR body's `Brief:` link trailer, NEVER a
// branch name. When several PRs represent one brief the FIRST in list order wins (deterministic; the
// caller needs only one to report). It is RepresentingPRsByBrief reduced to each brief's first PR,
// and the single source RepresentedBriefs and RepresentedBriefSet reduce from, so the open/merged
// split can never drift from the number map.
func RepresentedBriefPRs(prs []PRRef) map[string]RepresentedPR {
	all := RepresentingPRsByBrief(prs)
	out := make(map[string]RepresentedPR, len(all))
	for id, list := range all {
		out[id] = list[0]
	}
	return out
}

// RepresentedBriefs maps each brief id (`<stream>/<NN>`, lower-cased) to the number of an OPEN or
// MERGED PR that delivers it. It is RepresentedBriefPRs reduced to just the number — the shape a
// caller (deskdispatch's phantom check) that needs only "which PR" consumes.
func RepresentedBriefs(prs []PRRef) map[string]int {
	m := RepresentedBriefPRs(prs)
	out := make(map[string]int, len(m))
	for id, rp := range m {
		out[id] = rp.Number
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
