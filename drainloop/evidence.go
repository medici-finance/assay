package drainloop

import "fmt"

// WorkEvidence is an opt-in probe consulted before an item is claimed: has this work already
// been done elsewhere (a merged change, a landed artifact)? It returns:
//
//   - (true,  why, nil) — the work is already done; the engine lands the item without
//     dispatching it, recording why.
//   - (false, "",  nil) — no evidence; proceed to claim + dispatch as normal.
//   - (false, "",  err) — could-not-check; the engine SKIPS the item this pass rather than
//     dispatch a possible duplicate. A probe that cannot tell is never rounded up to "free".
//
// It is a deskkit-free func type: the generic shape lives in the public core, and its
// board/PR-reading implementation is house-side. nil ⇒ not performed.
type WorkEvidence func(Item) (taken bool, why string, err error)

// CheckAuthorRunner is the author-not-runner structural guard: an item whose Implementer
// equals runnerID must not be dispatched to that runner — the author of a change cannot be
// the runner that verifies it. It is a pure typed guard over Item.Implementer and the
// configured RunnerID; an empty runnerID (or an item with no Implementer) disables it, so it
// is opt-in by construction. It returns a non-nil error when the guard trips.
func CheckAuthorRunner(it Item, runnerID string) error {
	if runnerID == "" || it.Implementer == "" {
		return nil
	}
	if it.Implementer == runnerID {
		return fmt.Errorf("author-not-runner: item %s was authored by %q, which is this runner — refusing to dispatch it to its own author", it.ID, it.Implementer)
	}
	return nil
}
