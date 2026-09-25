package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- conformanceDescriptionIssue: hard per-skill description limit ---------

func TestConformanceDescriptionIssue_AtLimitPasses(t *testing.T) {
	desc := strings.Repeat("a", maxDescriptionChars)
	if msg := conformanceDescriptionIssue(desc); msg != "" {
		t.Errorf("description at exactly the limit (%d chars) must pass, got %q", maxDescriptionChars, msg)
	}
}

func TestConformanceDescriptionIssue_OverLimitFails(t *testing.T) {
	desc := strings.Repeat("a", maxDescriptionChars+1)
	msg := conformanceDescriptionIssue(desc)
	if msg == "" {
		t.Fatal("description one char over the limit must fail")
	}
	if !strings.Contains(msg, "1025") || !strings.Contains(msg, "1024") {
		t.Errorf("message %q does not name both the measured length and the limit", msg)
	}
}

// TestConformanceDescriptionIssue_MultibyteCountedInRunes is the regression
// case for Verify row 2: a description whose RUNE length is under the limit
// but whose BYTE length is over it must pass. Counting bytes instead of runes
// would false-positive on every multi-byte description under 1024 characters.
func TestConformanceDescriptionIssue_MultibyteCountedInRunes(t *testing.T) {
	desc := strings.Repeat("é", maxDescriptionChars) // 1024 runes, 2048 bytes
	if len(desc) <= maxDescriptionChars {
		t.Fatalf("fixture invariant broken: byte length %d is not over %d", len(desc), maxDescriptionChars)
	}
	if msg := conformanceDescriptionIssue(desc); msg != "" {
		t.Errorf("a 1024-RUNE description must pass even though its byte length is %d, got %q", len(desc), msg)
	}
}

// --- conformanceNameIssue: hard per-skill name length/pattern limit --------

func TestConformanceNameIssue_ValidNamesPass(t *testing.T) {
	for _, name := range []string{"a", "the-desk", "pr-review-desk", "skill9", "a1-b2-c3"} {
		if msg := conformanceNameIssue(name); msg != "" {
			t.Errorf("name %q should be valid, got issue %q", name, msg)
		}
	}
}

func TestConformanceNameIssue_TooLongFails(t *testing.T) {
	name := strings.Repeat("a", maxNameChars+1)
	msg := conformanceNameIssue(name)
	if msg == "" {
		t.Fatal("a name over the length limit must fail")
	}
	if !strings.Contains(msg, "65") || !strings.Contains(msg, "64") {
		t.Errorf("message %q does not name both the measured length and the limit", msg)
	}
}

func TestConformanceNameIssue_AtLengthLimitPasses(t *testing.T) {
	name := strings.Repeat("a", maxNameChars)
	if msg := conformanceNameIssue(name); msg != "" {
		t.Errorf("a name at exactly the length limit must pass, got %q", msg)
	}
}

// TestConformanceNameIssue_PatternViolations pins the agentskills name
// grammar: lowercase ascii letters/digits, hyphen-separated, no leading/
// trailing/consecutive hyphen.
func TestConformanceNameIssue_PatternViolations(t *testing.T) {
	cases := []string{
		"Bad-Name",  // uppercase
		"-leading",  // leading hyphen
		"trailing-", // trailing hyphen
		"double--hyphen",
		"has_underscore",
		"has space",
		"",
	}
	for _, name := range cases {
		if msg := conformanceNameIssue(name); msg == "" {
			t.Errorf("name %q violates the agentskills grammar and must fail", name)
		} else if !strings.Contains(msg, "pattern") {
			t.Errorf("message for %q = %q, want it to name the pattern", name, msg)
		}
	}
}

// --- conformanceBodyNotice: advisory per-skill soft budgets ----------------

func TestConformanceBodyNotice_UnderBudgetIsEmpty(t *testing.T) {
	raw := []byte("---\nname: s\ndescription: x\n---\n\nshort body\n")
	if n := conformanceBodyNotice("s/SKILL.md", raw); n != "" {
		t.Errorf("a small body must not earn a NOTICE, got %q", n)
	}
}

func TestConformanceBodyNotice_OverByteBudget(t *testing.T) {
	raw := []byte(strings.Repeat("a", bodyByteBudget+1))
	n := conformanceBodyNotice("s/SKILL.md", raw)
	if n == "" {
		t.Fatal("a body over the byte budget must earn a NOTICE")
	}
	if !strings.Contains(n, "NOTICE") || !strings.Contains(n, "bytes") {
		t.Errorf("notice %q does not name the byte budget", n)
	}
}

func TestConformanceBodyNotice_OverLineBudget(t *testing.T) {
	raw := []byte(strings.Repeat("x\n", bodyLineBudget+1))
	n := conformanceBodyNotice("s/SKILL.md", raw)
	if n == "" {
		t.Fatal("a body over the line budget must earn a NOTICE")
	}
	if !strings.Contains(n, "lines") {
		t.Errorf("notice %q does not name the line budget", n)
	}
}

func TestConformanceBodyNotice_OverTokenBudget(t *testing.T) {
	// Approx tokens = bytes / approxBytesPerToken, so crossing the (higher, in
	// bytes) token budget needs bodyTokenBudget*approxBytesPerToken+1 bytes —
	// well past the byte budget too, which the notice also names.
	raw := []byte(strings.Repeat("a", (bodyTokenBudget+1)*approxBytesPerToken))
	n := conformanceBodyNotice("s/SKILL.md", raw)
	if !strings.Contains(n, "approx tokens") {
		t.Errorf("notice %q does not name the approx-token budget", n)
	}
}

// --- conformanceBundleNotice: advisory bundle-wide budget ------------------

func TestConformanceBundleNotice_UnderBudgetIsEmpty(t *testing.T) {
	if n := conformanceBundleNotice(3, bundleDescriptionBudget); n != "" {
		t.Errorf("a sum AT the budget must not earn a NOTICE, got %q", n)
	}
}

func TestConformanceBundleNotice_OverBudget(t *testing.T) {
	n := conformanceBundleNotice(9, bundleDescriptionBudget+100)
	if n == "" {
		t.Fatal("a sum over the budget must earn a NOTICE")
	}
	if !strings.Contains(n, "NOTICE") || !strings.Contains(n, "8000") {
		t.Errorf("notice %q does not name the budget", n)
	}
}

// --- LintSkillsDir: the --skills-dir adopter-reach path --------------------

// writeDirSkill materialises <dir>/<skillName>/SKILL.md — the --skills-dir
// layout (no fixed plugins/assay/skills prefix).
func writeDirSkill(t *testing.T, dir, skillName, body string) {
	t.Helper()
	d := filepath.Join(dir, skillName)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", d, err)
	}
	if err := os.WriteFile(filepath.Join(d, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
}

func TestLintSkillsDir_ValidPasses(t *testing.T) {
	dir := t.TempDir()
	writeDirSkill(t, dir, "one", "---\nname: one\ndescription: A short description.\n---\n\nbody\n")
	writeDirSkill(t, dir, "two", "---\nname: two\ndescription: Another short description.\n---\n\nbody\n")

	checked, issues, err := LintSkillsDir(dir)
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

func TestLintSkillsDir_OverLongDescriptionFails(t *testing.T) {
	dir := t.TempDir()
	desc := strings.Repeat("a", maxDescriptionChars+1)
	writeDirSkill(t, dir, "one", "---\nname: one\ndescription: \""+desc+"\"\n---\n\nbody\n")

	_, issues, err := LintSkillsDir(dir)
	if err != nil {
		t.Fatalf("unexpected structural error: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("want 1 issue, got %d: %v", len(issues), issues)
	}
	if !strings.Contains(issues[0].Msg, "1024") {
		t.Errorf("issue %q does not name the limit", issues[0].Msg)
	}
}

func TestLintSkillsDir_BadNamePatternFails(t *testing.T) {
	dir := t.TempDir()
	writeDirSkill(t, dir, "Bad--Name", "---\nname: Bad--Name\ndescription: x\n---\n\nbody\n")

	_, issues, err := LintSkillsDir(dir)
	if err != nil {
		t.Fatalf("unexpected structural error: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("want 1 issue, got %d: %v", len(issues), issues)
	}
	if !strings.Contains(issues[0].Msg, "pattern") {
		t.Errorf("issue %q does not name the pattern violation", issues[0].Msg)
	}
}

// TestLintSkillsDir_EmptyDirFailsClosed is the unit-level analogue of Verify
// row expectations for the flag: zero matched files is exit 2, never a pass.
func TestLintSkillsDir_EmptyDirFailsClosed(t *testing.T) {
	dir := t.TempDir() // no <dir>/*/SKILL.md at all
	_, _, err := LintSkillsDir(dir)
	if err == nil {
		t.Fatal("an empty --skills-dir directory must be a structural error, not a pass")
	}
}

// --- ConformanceNoticesDir --------------------------------------------------

func TestConformanceNoticesDir_BundleOverBudget(t *testing.T) {
	dir := t.TempDir()
	desc := strings.Repeat("a", 900) // under the 1024 hard limit, individually
	for i := 0; i < 9; i++ {
		writeDirSkill(t, dir, "skill-"+string(rune('a'+i)), "---\nname: skill-"+string(rune('a'+i))+"\ndescription: \""+desc+"\"\n---\n\nbody\n")
	}

	checked, notices, err := ConformanceNoticesDir(dir)
	if err != nil {
		t.Fatalf("unexpected structural error: %v", err)
	}
	if checked != 9 {
		t.Errorf("checked = %d, want 9", checked)
	}
	found := false
	for _, n := range notices {
		if strings.Contains(n, "NOTICE") && strings.Contains(n, "8000") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a bundle-budget NOTICE naming 8000, got %v", notices)
	}
}

func TestConformanceNoticesDir_UnderBudgetIsClean(t *testing.T) {
	dir := t.TempDir()
	writeDirSkill(t, dir, "one", "---\nname: one\ndescription: short\n---\n\nbody\n")

	_, notices, err := ConformanceNoticesDir(dir)
	if err != nil {
		t.Fatalf("unexpected structural error: %v", err)
	}
	if len(notices) != 0 {
		t.Errorf("expected no notices for a small, single skill, got %v", notices)
	}
}
