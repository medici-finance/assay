package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGuardrailsInSyncInRealRepo is the DIFF half of the derive-or-diff
// contract, run against THIS repo's real tree.
// .claude/guardrails/GUARDRAILS.md is the ONE declared source for each rule it
// lists; every copy in a plugins/assay/skills/*/SKILL.md is regenerated from it
// by `make guardrail-sync`, and this test is what makes a hand-edit of a copy
// fail rather than merge.
//
// It is a CROSS-MODULE READER — it reads .claude/ and plugins/ at the repo root,
// two directories above this module — so any CI wiring for it must cover BOTH
// paths or the guard is advisory.
//
// THREE-STATE: the report distinguishes checked-clean, checked-failed and
// could-not-check, and this test fails on all of unchecked, failed, and
// "compared nothing". A guardrail check that could not read a copy must not
// report clean — that is the fail-open the whole module exists to remove.
func TestGuardrailsInSyncInRealRepo(t *testing.T) {
	// go test runs with the working directory set to the package directory
	// (tools/skillslint); the repo root is two levels up.
	const repoRoot = "../.."

	if _, err := os.Stat(filepath.Join(repoRoot, ".claude", "guardrails", "GUARDRAILS.md")); err != nil {
		t.Fatalf("could-not-check: the declared guardrail source is missing (%v) — a check that cannot read its source is never a pass", err)
	}

	rep := CheckGuardrails(repoRoot)

	for _, is := range rep.Unchecked {
		t.Errorf("could-not-check %s: %s", is.Path, is.Msg)
	}
	for _, is := range rep.Failed {
		t.Errorf("drift %s: %s", is.Path, is.Msg)
	}
	if rep.Compared == 0 {
		t.Fatal("compared 0 guardrail copies — this check proved nothing")
	}
	if !rep.Clean() {
		return // the per-issue errors above already said why
	}
	t.Logf("checked-clean: %d guardrail copies byte-match the declared source", rep.Compared)
}

// TestGuardrailSourceCoversEveryDeclaredSite asserts every path the source names
// actually exists. A declared site that has been renamed or deleted would
// otherwise degrade the check quietly: fewer comparisons, still "clean".
func TestGuardrailSourceCoversEveryDeclaredSite(t *testing.T) {
	const repoRoot = "../.."

	src, err := ParseGuardrailSource(repoRoot)
	if err != nil {
		t.Fatalf("could-not-check: %v", err)
	}
	if len(src.Blocks) == 0 {
		t.Fatal("the declared source lists no guardrail blocks")
	}
	for _, b := range src.Blocks {
		if len(b.Sites) < 2 {
			t.Errorf("guardrail %q declares %d site(s) — a rule stated in fewer than two places is not a duplication this module should own; either drop it from the source or record the second site",
				b.ID, len(b.Sites))
		}
		for _, s := range b.Sites {
			if _, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(s.Path))); err != nil {
				t.Errorf("guardrail %q declares site %s (source line %d) which does not exist: %v", b.ID, s.Path, s.Line, err)
			}
		}
	}
}
