package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLintSkills_RealRepoSkillsAreValid runs the lint against THIS repo's real
// plugins/assay/skills/ tree, not a fixture. It is the enforcing counterpart to
// the unit tests: those prove the rules fire on synthetic input; this proves the
// skill homes actually shipped in this repo satisfy them.
//
// It is a CROSS-MODULE READER — it reads plugins/assay/skills/ at the repo root,
// two directories above this module (tools/skillslint).
func TestLintSkills_RealRepoSkillsAreValid(t *testing.T) {
	// go test runs with the working directory set to the package directory
	// (tools/skillslint); the repo root is two levels up.
	const repoRoot = "../.."

	if _, err := os.Stat(filepath.Join(repoRoot, "plugins", "assay", "skills")); err != nil {
		t.Fatalf("cannot find %s/plugins/assay/skills — run from inside the assay checkout (%v)", repoRoot, err)
	}

	checked, issues, err := LintSkills(repoRoot)
	if err != nil {
		t.Fatalf("skills lint could not run over the real tree: %v", err)
	}
	if checked == 0 {
		t.Fatal("linted 0 real skill files — this check proved nothing")
	}
	for _, is := range issues {
		t.Errorf("%s: %s", is.Path, is.Msg)
	}
}
