package main

import "testing"

// ALIAS-HELD rows (another spelling of the same PR) are not occupied slots.
func TestHeldClaimID_ignores_alias_rows(t *testing.T) {
	for _, tc := range []struct {
		line, id string
		ok       bool
	}{
		{"dispatch-claim: HELD a--pr-1 — x at=t age=1m", "a--pr-1", true},
		{"HELD a--pr-1 — x", "a--pr-1", true},
		{"dispatch-claim: ALIAS-HELD b--pr-1 — x at=t age=1m (same PR as a--pr-1)", "", false},
		{"dispatch-claim: FREE a--pr-1 (no ref)", "", false},
	} {
		id, ok := heldClaimID(tc.line)
		if id != tc.id || ok != tc.ok {
			t.Errorf("%q: got (%q,%v), want (%q,%v)", tc.line, id, ok, tc.id, tc.ok)
		}
	}
}
