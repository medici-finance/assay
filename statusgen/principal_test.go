package main

import "testing"

// TestOnBehalfOfPrincipalOfTakesLastOccurrence is the security-lane finding
// (multi-principal/01 review): a cell carrying more than one on-behalf-of annotation
// must resolve to the LAST one, not the first — the reading that cannot be shadowed by
// an earlier, spoofed occurrence. In normal operation a cell carries at most one (see
// AppendOnBehalfOf's planted-line stripping on the write side); this is the read-side
// defense in depth for the case where it somehow does not.
func TestOnBehalfOfPrincipalOfTakesLastOccurrence(t *testing.T) {
	cell := "assay-verifier-app[bot] @ abc1234 (on-behalf-of human:evil) (on-behalf-of human:ada)"
	login, ok := onBehalfOfPrincipalOf(cell)
	if !ok {
		t.Fatalf("onBehalfOfPrincipalOf(%q) ok = false, want true", cell)
	}
	if login != "ada" {
		t.Fatalf("onBehalfOfPrincipalOf(%q) = %q, want the LAST occurrence %q", cell, login, "ada")
	}
}

// TestOnBehalfOfPrincipalOfSingleOccurrence is the ordinary case: one annotation
// resolves to itself, unaffected by the last-occurrence change.
func TestOnBehalfOfPrincipalOfSingleOccurrence(t *testing.T) {
	cell := "assay-verifier-app[bot] @ abc1234 (on-behalf-of human:ada)"
	login, ok := onBehalfOfPrincipalOf(cell)
	if !ok || login != "ada" {
		t.Fatalf("onBehalfOfPrincipalOf(%q) = (%q, %v), want (\"ada\", true)", cell, login, ok)
	}
}
