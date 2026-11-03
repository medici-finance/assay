package fleet

import (
	"fmt"
	"strings"
	"time"

	"github.com/medici-finance/assay/deskd/internal/adapter"
)

// classifyPR maps one open PR to a fleet [Token] and [State] (brief §"Design
// record"):
//
//	CONFLICTING mergeable                         → conflicting (fault)
//	no review yet, or REVIEW_REQUIRED, not draft  → awaiting-review (neutral)
//	everything else (draft, changes-requested,
//	  approved-and-landing)                       → working (healthy)
//
// "working" is deliberately the catch-all, mirroring web/panes.go's own
// paletteFor doctrine that a well-formed in-progress state renders CALM
// rather than alarming (§3.5) — a PR with changes requested is the author's
// next move, not a fault, and an approved PR still landing is progressing,
// not waiting.
func classifyPR(pr adapter.RawPullRequest, now time.Time) Token {
	state := StateWorking
	switch {
	case pr.Mergeable == "CONFLICTING":
		state = StateConflicting
	case !pr.Draft && (pr.ReviewDecision == "" || pr.ReviewDecision == "REVIEW_REQUIRED"):
		state = StateAwaitingReview
	}
	return Token{
		Kind:   KindPR,
		Key:    fmt.Sprintf("#%d", pr.Number),
		Title:  pr.Title,
		Holder: pr.Author,
		Age:    now.Sub(pr.UpdatedAt),
		State:  state,
	}
}

// classifyClaim maps one refs/dispatch/* ref to a fleet [Token]. A claim
// older than staleAfter is stale-claim (amber); everything else is working
// (healthy) — a claim has only these two states, unlike a PR's three,
// because a claim carries no independent "needs a human" or "broken"
// signal of its own the way mergeable/reviewDecision do for a PR.
func classifyClaim(c adapter.RawClaim, now time.Time, staleAfter time.Duration) Token {
	key := strings.TrimPrefix(c.RefName, "refs/dispatch/")
	state := StateWorking
	var age time.Duration
	if !c.ClaimedAt.IsZero() {
		age = now.Sub(c.ClaimedAt)
		if age > staleAfter {
			state = StaleClaimState
		}
	}
	return Token{
		Kind:   KindClaim,
		Key:    key,
		Holder: c.Holder,
		Age:    age,
		State:  state,
	}
}
