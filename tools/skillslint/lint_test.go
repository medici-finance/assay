package main

import (
	"os"
	"path/filepath"
	"testing"
)

// writeSkill materialises plugins/assay/skills/<dir>/SKILL.md under root with body.
func writeSkill(t *testing.T, root, dir, body string) {
	t.Helper()
	d := filepath.Join(root, "plugins", "assay", "skills", dir)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", d, err)
	}
	if err := os.WriteFile(filepath.Join(d, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
}

func TestLintSkills_Valid(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "the-desk", "---\nname: the-desk\ndescription: Boot the desk.\n---\n\n# TheDesk\n")
	writeSkill(t, root, "author-brief", "---\nname: author-brief\ndescription: >-\n  Author briefs.\n---\n\nbody\n")

	checked, issues, err := LintSkills(root)
	if err != nil {
		t.Fatalf("unexpected structural error: %v", err)
	}
	if checked != 2 {
		t.Errorf("checked = %d, want 2", checked)
	}
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %v", issues)
	}
}

func TestLintSkills_NameMismatch(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "the-desk", "---\nname: not-the-desk\ndescription: x\n---\n")

	_, issues, err := LintSkills(root)
	if err != nil {
		t.Fatalf("unexpected structural error: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("want 1 issue, got %d: %v", len(issues), issues)
	}
	if got := issues[0].Path; got != "plugins/assay/skills/the-desk/SKILL.md" {
		t.Errorf("issue path = %q", got)
	}
}

func TestLintSkills_EmptyDescription(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "s", "---\nname: s\ndescription: \"\"\n---\n")

	_, issues, err := LintSkills(root)
	if err != nil {
		t.Fatalf("unexpected structural error: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("want 1 issue, got %d: %v", len(issues), issues)
	}
}

func TestLintSkills_MissingFrontmatter(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "s", "# no frontmatter here\n")

	_, issues, err := LintSkills(root)
	if err != nil {
		t.Fatalf("unexpected structural error: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("want 1 issue, got %d: %v", len(issues), issues)
	}
}

// TestLintSkills_MutatedFileFails is the unit-level analogue of the brief's
// Verify row 5: a SKILL.md overwritten with garbage must produce an issue, so
// the lint demonstrably READS the file rather than merely listing it.
func TestLintSkills_MutatedFileFails(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "the-desk", "x\n")

	_, issues, err := LintSkills(root)
	if err != nil {
		t.Fatalf("unexpected structural error: %v", err)
	}
	if len(issues) == 0 {
		t.Fatal("a garbage SKILL.md produced no issue — the lint is a no-op")
	}
}

// TestLintSkills_BareUnforgeableClaimFails is the regression test for the
// overclaim rule: a skill file that asserts a review/App/gate is "unforgeable"
// (or "tamper-evident") with no qualifier on the same line is a false overclaim
// (the App is attribution, not authorization — anyone holding the key can mint
// it) and must fail the lint.
func TestLintSkills_BareUnforgeableClaimFails(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "the-desk", "---\nname: the-desk\ndescription: x\n---\n\n"+
		"the commit actor is the unforgeable desk App, not a shared account.\n")

	_, issues, err := LintSkills(root)
	if err != nil {
		t.Fatalf("unexpected structural error: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("want 1 issue, got %d: %v", len(issues), issues)
	}
}

// TestLintSkills_TamperEvidentBareClaimFails covers the sibling banned word:
// "tamper-evident" is retired for the same reason "unforgeable" is.
func TestLintSkills_TamperEvidentBareClaimFails(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "the-desk", "---\nname: the-desk\ndescription: x\n---\n\n"+
		"the commit actor is the tamper-evident desk App, not a shared account.\n")

	_, issues, err := LintSkills(root)
	if err != nil {
		t.Fatalf("unexpected structural error: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("want 1 issue, got %d: %v", len(issues), issues)
	}
}

// TestLintSkills_QualifiedUnforgeableClaimPasses proves the check does not
// false-positive on the sanctioned phrasing already live in pr-review-desk
// and verify-desk SKILL.md: a banned word that is explicitly negated or
// flagged as retired on the same line is not a bare overclaim.
func TestLintSkills_QualifiedUnforgeableClaimPasses(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "the-desk", "---\nname: the-desk\ndescription: x\n---\n\n"+
		"a distinct, auditable actor; advisory, not unforgeable.\n"+
		"section called the verdict \"unforgeable\"; that was wrong and is retired.\n")

	checked, issues, err := LintSkills(root)
	if err != nil {
		t.Fatalf("unexpected structural error: %v", err)
	}
	if checked != 1 {
		t.Errorf("checked = %d, want 1", checked)
	}
	if len(issues) != 0 {
		t.Errorf("expected no issues on qualified phrasing, got %v", issues)
	}
}

func TestLintSkills_EmptyRootFailsClosed(t *testing.T) {
	root := t.TempDir() // no plugins/assay/skills at all
	_, _, err := LintSkills(root)
	if err == nil {
		t.Fatal("an empty root must be a structural error, not a pass")
	}
}
