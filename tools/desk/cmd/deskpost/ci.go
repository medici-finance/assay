package main

// CI rollup evaluation. "Green" means no FAILURE/ERROR/PENDING
// among the combined commit-status contexts and check runs at the head, with
// SKIPPED/NEUTRAL conclusions ignored. The determination is deliberately
// fail-closed: anything we cannot positively read as green is refused/unverifiable
// rather than waved through.

import "github.com/medici-finance/assay/tools/desk/internal/deskkit"

type ciVerdict int

const (
	ciGreen   ciVerdict = iota // positively green — all relevant checks succeeded
	ciRed                      // positively NOT green (a check failed/errored) → refuse (exit 5)
	ciPending                  // not yet complete, OR the rollup could not be read IN FULL → unverifiable (exit 6)
	ciEmpty                    // no relevant checks at all → per-repo policy decides
)

// evalCI reduces the two rollups to a single verdict.
//
// The rollups it is handed MUST be the full, paginated ones (github.go walks every page).
// Since a truncated rollup and a green one are indistinguishable once flattened, evalCI
// ALSO reconciles the items it holds against the total_count GitHub reported at the head:
// short by even one → ciPending, never ciGreen. That closes this fail-open at the last gate — the
// count is the tool's own proof that it saw every check.
func evalCI(cs *combinedStatus, cr *checkRunsResp) ciVerdict {
	relevant := 0
	pending := false

	// A rollup shorter than the count GitHub asserts exists means we did NOT see every
	// check at this head. (A LONGER one is fine — treat total_count as a floor.)
	incomplete := len(cs.Statuses) < cs.TotalCount || len(cr.CheckRuns) < cr.TotalCount

	for _, s := range cs.Statuses {
		switch s.State {
		case "failure", "error":
			return ciRed
		case "pending":
			pending = true
			relevant++
		case "success":
			relevant++
		default:
			// Unknown status state → fail closed as not-yet-green.
			pending = true
			relevant++
		}
	}

	for _, run := range cr.CheckRuns {
		if run.Status != "completed" {
			pending = true
			relevant++
			continue
		}
		switch run.Conclusion {
		case "success":
			relevant++
		case "neutral", "skipped":
			// Ignored.
		case "failure", "cancelled", "timed_out", "action_required", "stale":
			return ciRed
		default:
			// Unknown conclusion on a completed run → fail closed as unverifiable.
			pending = true
			relevant++
		}
	}

	// A failure we DID see has already returned ciRed (the stronger refusal). What we could
	// NOT see keeps us from claiming green — and must not be mistaken for the empty rollup
	// of a CI-less repo when GitHub says checks exist at this head.
	if incomplete || pending {
		return ciPending
	}
	if relevant == 0 {
		return ciEmpty
	}
	return ciGreen
}

// ciRequired reports whether an EMPTY rollup for repo must be treated as unverifiable
// (true) rather than green (false).
//
// the drift anti-pattern: this is no longer a second, hand-maintained list (it held only example-org/examples
// while the desk's allowed set grew, so `ready` could never flip slides /
// example-reconciler / proposals). The policy now comes off the SAME compiled-in repo table as the
// allowed-repo gate — adding a repo to the desk set forces an explicit CI policy, so the
// two cannot drift apart again. An unknown repo is CI-required (fail closed).
func ciRequired(repo string) bool {
	return deskkit.CIRequired(repo)
}
