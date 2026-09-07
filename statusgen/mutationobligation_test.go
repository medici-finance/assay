package main

import (
	"strings"
	"testing"
)

// Positive control for the MUTATION-obligation lint (mistake-proofing/06) — the
// rule applied to itself. This check is a check, so it owes exactly what it
// demands: a demonstration that an injected instance of the error it claims to
// stop actually reddens it. These tests ARE that demonstration, and they also
// keep this rule out of the firing audit's COLD (retirement-candidate) bucket by
// referencing its stable rule tag [verify-obligation] (lintaudit.go).

// TestMutationObligationFiresOnAddedCheck is the core positive control (Verify
// row 5): a diff that adds a check-shaped path with no `+mutation` row is a fatal
// PROBLEM; the same diff with the row present is silent; a diff that touches no
// check-shaped path is silent.
func TestMutationObligationFiresOnAddedCheck(t *testing.T) {
	// (a) INJECT the error: a check-shaped path changed, brief owns it, NO row.
	t.Run("added-check-no-row-is-fatal", func(t *testing.T) {
		root := t.TempDir()
		s, briefRel := streamForBrief(t, root, "01", []string{"statusgen/"}, "Extend the lint.", noObligationRows)
		withBranchChangedSet(t, func(string) (map[string]bool, bool) {
			return map[string]bool{briefRel: true, "statusgen/newcheck.go": true}, true
		})
		p, n := verifyObligationDerivation(root, []*Stream{s})
		if len(p) != 1 {
			t.Fatalf("an added check with no mutation row must be exactly one fatal PROBLEM, got problems=%v notices=%v", p, n)
		}
		for _, want := range []string{"+mutation", "statusgen/newcheck.go", "muhar", "[verify-obligation]", "PRESENCE"} {
			if !strings.Contains(p[0], want) {
				t.Fatalf("mutation PROBLEM must contain %q, got %q", want, p[0])
			}
		}
		// The floor must state it does not replace the reviewer (brief Context).
		if !strings.Contains(p[0], "does NOT replace the reviewer") {
			t.Fatalf("mutation PROBLEM must say it does not replace the reviewer, got %q", p[0])
		}
	})

	// (b) DISCHARGE the obligation: the same diff, `+mutation` row present → silent.
	t.Run("added-check-with-row-is-silent", func(t *testing.T) {
		root := t.TempDir()
		s, briefRel := streamForBrief(t, root, "01", []string{"statusgen/"}, "Extend the lint.", withObligationRow("check +mutation"))
		withBranchChangedSet(t, func(string) (map[string]bool, bool) {
			return map[string]bool{briefRel: true, "statusgen/newcheck.go": true}, true
		})
		p, n := verifyObligationDerivation(root, []*Stream{s})
		for _, m := range append(append([]string{}, p...), n...) {
			if strings.Contains(m, "+mutation") {
				t.Fatalf("a present +mutation row must be silent, still reported: %q", m)
			}
		}
	})

	// (c) NO trigger: a diff touching no check-shaped path → nothing owed, silent.
	t.Run("no-check-shaped-path-is-silent", func(t *testing.T) {
		root := t.TempDir()
		s, briefRel := streamForBrief(t, root, "01", []string{"alpha/one.txt"}, "Edit a data file.", noObligationRows)
		withBranchChangedSet(t, func(string) (map[string]bool, bool) {
			return map[string]bool{briefRel: true, "alpha/one.txt": true}, true
		})
		p, n := verifyObligationDerivation(root, []*Stream{s})
		if len(p) != 0 {
			t.Fatalf("a diff touching no check-shaped path must owe no mutation PROBLEM, got %v", p)
		}
		for _, m := range n {
			if strings.Contains(m, "+mutation") {
				t.Fatalf("a diff touching no check-shaped path must not mention mutation, got %q", m)
			}
		}
	})
}

// TestMutationObligationCouldNotCheckIsNotSilence (Verify row 6): an unavailable
// diff does NOT pass silently — it emits a conspicuous, greppable COULD-NOT-CHECK
// NOTICE, distinguishable in the output from "nothing owed" (which is silent).
// The two failures are kept distinct and are never collapsed. (The could-not-check
// degrades to a NOTICE rather than a PROBLEM — the same fail-open posture the UNRUN
// gate uses — so a shallow clone or a no-git context does not freeze the board;
// the merge gate is the owed-but-absent mutation PROBLEM on the available-diff path.)
func TestMutationObligationCouldNotCheckIsNotSilence(t *testing.T) {
	root := t.TempDir()
	s, _ := streamForBrief(t, root, "01", []string{"statusgen/"}, "Extend the lint.", noObligationRows)

	// Diff UNAVAILABLE → could-not-check → a conspicuous NOTICE, never silence.
	withBranchChangedSet(t, func(string) (map[string]bool, bool) { return nil, false })
	pCNC, nCNC := verifyObligationDerivation(root, []*Stream{s})
	if len(pCNC) != 0 {
		t.Fatalf("could-not-check must degrade to a NOTICE, not freeze the board with a PROBLEM, got %v", pCNC)
	}
	if len(nCNC) != 1 || !strings.Contains(nCNC[0], "COULD-NOT-CHECK") {
		t.Fatalf("unavailable diff must emit one could-not-check NOTICE, got %v", nCNC)
	}

	// Diff AVAILABLE but touching no check-shaped path → nothing owed → SILENT.
	// (Its own file is not in the diff, so it is out of scope entirely.)
	withBranchChangedSet(t, func(string) (map[string]bool, bool) {
		return map[string]bool{"unrelated/file.txt": true}, true
	})
	pOwe, nOwe := verifyObligationDerivation(root, []*Stream{s})
	if len(pOwe) != 0 || len(nOwe) != 0 {
		t.Fatalf("nothing-owed must be silent, got problems=%v notices=%v", pOwe, nOwe)
	}

	// The two are DISTINCT and must never be collapsed: could-not-check emits a
	// greppable COULD-NOT-CHECK marker; nothing-owed emits nothing at all.
	if len(nCNC) == len(nOwe) {
		t.Fatalf("could-not-check and nothing-owed must be distinguishable in output; both produced %d notices", len(nCNC))
	}
}

// TestMutationObligationInheritedCorpusStaysAdvisory (Verify row 7): the
// promotion is transition-scoped — a brief whose own file this branch did NOT
// edit is not evaluated, so the inherited corpus is never made fatal by the
// promotion. The contrast (same brief, its file IN the diff) proves the scope is
// what protects the corpus, not a blanket advisory.
func TestMutationObligationInheritedCorpusStaysAdvisory(t *testing.T) {
	root := t.TempDir()
	// An inherited brief owing mutation: declares a check-shaped home, no row.
	s, briefRel := streamForBrief(t, root, "01", []string{"statusgen/"}, "Extend the lint.", noObligationRows)

	// This branch changed a check-shaped path but NOT this brief's own file →
	// out of transition scope → not evaluated → no PROBLEM.
	withBranchChangedSet(t, func(string) (map[string]bool, bool) {
		return map[string]bool{"statusgen/newcheck.go": true}, true
	})
	pInherited, nInherited := verifyObligationDerivation(root, []*Stream{s})
	if len(pInherited) != 0 || len(nInherited) != 0 {
		t.Fatalf("an inherited brief this branch did not edit must not be made fatal, got problems=%v notices=%v", pInherited, nInherited)
	}

	// Contrast: the SAME brief, once its own file is in the diff, DOES become a
	// fatal PROBLEM — proving the promotion bites on this branch's own changes.
	withBranchChangedSet(t, func(string) (map[string]bool, bool) {
		return map[string]bool{briefRel: true, "statusgen/newcheck.go": true}, true
	})
	pTouched, _ := verifyObligationDerivation(root, []*Stream{s})
	if len(pTouched) != 1 || !strings.Contains(pTouched[0], "+mutation") {
		t.Fatalf("a brief this branch DID edit must be a fatal mutation PROBLEM, got %v", pTouched)
	}
}
