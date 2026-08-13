package main

import (
	"strings"
	"testing"
)

// TestGhRefusesWithoutMintedToken hardens the #562 finding (a candidate root cause
// raised during the investigation: `deskreply` might fall back to an ambient/keyring gh
// identity — e.g. a stale or invalid `assay-desk-app[bot]` keyring entry — instead of the
// freshly minted worker token). Tracing every gh() call site shows they all run strictly
// after mintWorkerToken succeeds, so ghToken is never actually empty at a gh() call in
// production — but gh() itself must refuse outright rather than silently falling through
// to os.Environ()'s ambient gh identity if that invariant is ever violated by a future
// change, so the guard is tested directly here rather than left implicit.
func TestGhRefusesWithoutMintedToken(t *testing.T) {
	old := ghToken
	ghToken = ""
	t.Cleanup(func() { ghToken = old })

	_, err := gh(".", "pr", "view", "1")
	if err == nil {
		t.Fatal("gh() with no minted worker token succeeded; it must refuse rather than fall back to the ambient gh identity/keyring")
	}
	if !strings.Contains(err.Error(), "minted worker token") {
		t.Fatalf("gh() error = %q, want it to name the missing minted token", err.Error())
	}
}
