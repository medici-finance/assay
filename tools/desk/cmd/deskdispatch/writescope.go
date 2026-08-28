package main

import (
	"fmt"
	"io"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// writescope.go — the ADVISORY write-scope overlap echo.
//
// Before the durable claim is taken, deskdispatch echoes any WRITE-OVERLAP between the item
// about to be dispatched and the items already holding an in-flight dispatch claim for the
// same root, then PROCEEDS. This is a COORDINATION HINT, not a lock: it has no exit code, it
// never blocks or delays the claim, and it is emitted to stderr so it never pollutes the
// prompt on stdout. The moment this returned an error, gated a step, or changed an exit code
// on overlap it would be a lock — a different, rejected design.
//
// It is best-effort: the candidate's scopes come from its `--brief` (when given); the in-flight
// universe is the root repo's local `refs/dispatch/*` claims resolved to briefs under `--root`.
// A missing brief, a non-git root, or an underivable scope simply yields fewer (or no) lines —
// never a failed dispatch.

// echoWriteOverlap writes the advisory overlap warnings for the dispatch to w. It always
// returns nil-shaped behavior (no error, no exit code): the caller ignores its effect on
// control flow entirely.
func echoWriteOverlap(w io.Writer, o dispatchOpts) {
	// The candidate item, identified by the ORIGINAL item key (never the derived claim key —
	// the warning names the item the operator knows).
	cand := loopengine.Item{ID: o.item, WriteScopes: readBriefScopes(o.brief)}

	inflight := loopengine.InFlightClaimScopes(o.root)

	// Only the overlap lines matter at dispatch time; a candidate `could-not-derive` line is
	// suppressed here (deskdispatch dispatches one named item deliberately — the operator is
	// not scanning a queue), so an unparseable --brief does not emit a spurious advisory. The
	// plan surface is where could-not-derive is reported across the queue.
	if !cand.WriteScopes.Derivable {
		return
	}
	for _, line := range loopengine.WriteOverlapWarnings([]loopengine.Item{cand}, inflight) {
		fmt.Fprintln(w, line)
	}
}

// readBriefScopes derives the write-scope set from a brief file path. An empty path or an
// unreadable file yields the could-not-derive zero value.
func readBriefScopes(path string) loopengine.WriteScopeSet {
	if path == "" {
		return loopengine.WriteScopeSet{}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return loopengine.WriteScopeSet{}
	}
	return loopengine.DeriveWriteScopes(string(raw))
}
