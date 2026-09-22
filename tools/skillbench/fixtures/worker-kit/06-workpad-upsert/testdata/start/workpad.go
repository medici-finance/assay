// Package workpad is a tiny, standalone stand-in for the one-workpad-per-PR upsert rule —
// small enough to fix in one sitting, deterministic, and offline.
package workpad

// Upsert returns the comment list with the running agent's own progress recorded as a
// SINGLE entry: if the agent's own newest unresolved comment already exists (its index is
// ownIndex, -1 if none), it is replaced in place; otherwise the new body is appended as a
// new comment. The input slice is never mutated in place.
func Upsert(comments []string, ownIndex int, body string) []string {
	// BUG: always appends — an agent re-dispatched onto the same PR scatters a new
	// comment every time instead of editing its one workpad in place.
	out := make([]string, len(comments))
	copy(out, comments)
	return append(out, body)
}
