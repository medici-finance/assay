package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- committed .mdc rule is in sync with the committed resident source ---------

// TestCursorCommittedRuleMatchesSource is the cursor analogue of
// TestCodexCommittedManifestMatchesSource: the shipped cursor/assay.mdc must be
// exactly what the committed resident-rules.md would generate. If it drifts, this
// fails in the same suite CI runs (Verify: `cursor --check`).
func TestCursorCommittedRuleMatchesSource(t *testing.T) {
	if code := cursorCmd([]string{"--check", "--root", "../.."}); code != exitClean {
		t.Fatalf("cursor --check against the repo root returned %d, want %d (clean) — "+
			"the committed rule is out of sync with the resident source; run `go run ./tools/harnessgen cursor` and commit", code, exitClean)
	}
}

// --- the committed .mdc carries the `.cursor/rules` frontmatter contract -------

// TestCursorRuleHasMdcFrontmatter locks the Cursor-native shape: an `.mdc` rule
// begins with a YAML frontmatter block carrying `alwaysApply: true` (so the rules
// load every session) — the mechanism Cursor reads (HP/12 §2.1). A generator that
// dropped the frontmatter would ship a file Cursor treats as plain prose, not a rule.
func TestCursorRuleHasMdcFrontmatter(t *testing.T) {
	raw, err := os.ReadFile("../../plugins/assay/cursor/assay.mdc")
	if err != nil {
		t.Fatalf("read committed rule: %v", err)
	}
	body := string(raw)
	if !strings.HasPrefix(body, "---\n") {
		t.Fatalf("committed rule does not open with an `.mdc` frontmatter block; head:\n%.80s", body)
	}
	if !strings.Contains(body, "alwaysApply: true") {
		t.Fatalf("committed rule is missing `alwaysApply: true` — Cursor would not load it every session")
	}
}

// --- a planted rule hand-edit is caught as drift, naming the rule --------------

func TestCursorCheckDetectsDrift(t *testing.T) {
	bundle := writeMinimalCursorBundle(t)
	rule := filepath.Join(bundle, "cursor", "assay.mdc")
	raw, err := os.ReadFile(rule)
	if err != nil {
		t.Fatal(err)
	}
	// Hand-edit the generated rule (mirrors a stray edit that bypasses the generator).
	edited := string(raw) + "\nHAND-EDITED LINE\n"
	if err := os.WriteFile(rule, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	var code int
	stderr := captureStderr(t, func() { code = cursorCmd([]string{"--check", "--bundle", bundle}) })
	if code != exitDrift {
		t.Fatalf("check with a hand-edited rule returned %d, want %d (drift)", code, exitDrift)
	}
	if !strings.Contains(stderr, "assay.mdc") {
		t.Fatalf("drift report did not name the rule; stderr:\n%s", stderr)
	}
	// After restoring, --check passes again.
	if err := os.WriteFile(rule, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if code := cursorCmd([]string{"--check", "--bundle", bundle}); code != exitClean {
		t.Fatalf("check after restore returned %d, want %d (clean)", code, exitClean)
	}
}

// --- coverage: a skill on disk with no roster/exclusion entry is exit 2 --------

func TestCursorCoverageCatchesUnaccountedSkill(t *testing.T) {
	bundle := writeMinimalCursorBundle(t)
	if code := cursorCmd([]string{"--check", "--bundle", bundle}); code != exitClean {
		t.Fatalf("baseline check returned %d, want %d", code, exitClean)
	}
	probe := filepath.Join(bundle, "skills", "probe-skill")
	if err := os.MkdirAll(probe, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(probe, "SKILL.md"), []byte("---\nname: probe-skill\ndescription: probe\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var code int
	stderr := captureStderr(t, func() { code = cursorCmd([]string{"--check", "--bundle", bundle}) })
	if code != exitCouldNotCheck {
		t.Fatalf("check with an unaccounted skill returned %d, want %d (could-not-check)", code, exitCouldNotCheck)
	}
	if !strings.Contains(stderr, "probe-skill") {
		t.Fatalf("coverage failure did not name the unaccounted skill; stderr:\n%s", stderr)
	}
}

// --- packaging↔binding skew: a packaged skill with no degradation cell is caught

func TestCursorBindingSkewCaught(t *testing.T) {
	bundle := writeMinimalCursorBundle(t)
	if code := cursorCmd([]string{"--check", "--bundle", bundle}); code != exitClean {
		t.Fatalf("baseline check returned %d, want %d", code, exitClean)
	}
	binding := filepath.Join(bundle, "references", "cursor.md")
	raw, err := os.ReadFile(binding)
	if err != nil {
		t.Fatal(err)
	}
	stripped := strings.ReplaceAll(string(raw), "`the-desk`", "the-desk")
	if stripped == string(raw) {
		t.Fatal("test setup: expected a `the-desk` degradation cell to strip")
	}
	if err := os.WriteFile(binding, []byte(stripped), 0o644); err != nil {
		t.Fatal(err)
	}
	var code int
	stderr := captureStderr(t, func() { code = cursorCmd([]string{"--check", "--bundle", bundle}) })
	if code != exitCouldNotCheck {
		t.Fatalf("check with a missing binding cell returned %d, want %d (could-not-check)", code, exitCouldNotCheck)
	}
	if !strings.Contains(stderr, "the-desk") {
		t.Fatalf("binding-skew failure did not name the skill; stderr:\n%s", stderr)
	}
}

// --- an excluded entry with an empty reason is a parse error (could-not-check) -

func TestCursorExcludedEmptyReasonIsParseError(t *testing.T) {
	bundle := writeMinimalCursorBundle(t)
	packaging := filepath.Join(bundle, "cursor", "packaging.md")
	raw, err := os.ReadFile(packaging)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(raw), "author-brief\n", "author-brief :: EXCLUDED:\n", 1)
	if edited == string(raw) {
		t.Fatal("test setup: expected to find author-brief in the roster")
	}
	if err := os.WriteFile(packaging, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	var code int
	stderr := captureStderr(t, func() { code = cursorCmd([]string{"--check", "--bundle", bundle}) })
	if code != exitCouldNotCheck {
		t.Fatalf("check with an empty-reason exclusion returned %d, want %d (could-not-check)", code, exitCouldNotCheck)
	}
	if !strings.Contains(stderr, "empty reason") {
		t.Fatalf("parse error did not explain the empty reason; stderr:\n%s", stderr)
	}
}

// --- a missing resident source is could-not-check ------------------------------

func TestCursorMissingSourceIsCouldNotCheck(t *testing.T) {
	bundle := t.TempDir() // no resident-rules.md at all
	if code := cursorCmd([]string{"--check", "--bundle", bundle}); code != exitCouldNotCheck {
		t.Fatalf("check with no resident source returned %d, want %d (could-not-check)", code, exitCouldNotCheck)
	}
}

// --- an empty coverage roster is could-not-check, never a clean pass -----------

func TestCursorEmptyRosterIsCouldNotCheck(t *testing.T) {
	bundle := writeMinimalCursorBundle(t)
	packaging := filepath.Join(bundle, "cursor", "packaging.md")
	if err := os.WriteFile(packaging, []byte("<!-- assay:cursor-packaging\n-->\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := cursorCmd([]string{"--check", "--bundle", bundle}); code != exitCouldNotCheck {
		t.Fatalf("check with an empty roster returned %d, want %d (could-not-check)", code, exitCouldNotCheck)
	}
}

// --- write then check round-trips clean ---------------------------------------

func TestCursorWriteThenCheckClean(t *testing.T) {
	bundle := writeMinimalCursorBundle(t)
	if code := cursorCmd([]string{"--bundle", bundle}); code != exitClean {
		t.Fatalf("write returned %d, want %d", code, exitClean)
	}
	if code := cursorCmd([]string{"--check", "--bundle", bundle}); code != exitClean {
		t.Fatalf("check-after-write returned %d, want %d (clean)", code, exitClean)
	}
}

// writeMinimalCursorBundle builds a self-contained plugins/assay-shaped bundle
// under a temp dir: a valid resident-rules source (header, R1..R10, footer), three
// skills, a Cursor binding file with a degradation cell per skill, a full coverage
// roster, and a generated .mdc rule. Fully controlled — the tests mutate exactly
// one thing at a time.
func writeMinimalCursorBundle(t *testing.T) string {
	t.Helper()
	bundle := t.TempDir()

	mustWrite := func(rel, content string) {
		p := filepath.Join(bundle, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Build a valid source: `## Header`, `## R1 TITLE` .. `## R10 TITLE` (the
	// heading form ruleHeadingRE `^## R(\d+) ` matches), and `## Footer`.
	var src strings.Builder
	src.WriteString("## Header\n\nRESIDENT OPERATING RULES (test).\n\n")
	for i := 1; i <= 10; i++ {
		src.WriteString("## R")
		src.WriteString(itoa(i))
		src.WriteString(" RULE")
		src.WriteString(itoa(i))
		src.WriteString("\n\nRULE")
		src.WriteString(itoa(i))
		src.WriteString(": body line for rule ")
		src.WriteString(itoa(i))
		src.WriteString(".\n\n")
	}
	src.WriteString("## Footer\n\nSee the skill bodies.\n")
	mustWrite("resident-rules.md", src.String())

	skills := []string{"adopt", "author-brief", "the-desk"}
	for _, s := range skills {
		mustWrite(filepath.Join("skills", s, "SKILL.md"),
			"---\nname: "+s+"\ndescription: "+s+" test skill\n---\n# "+s+"\n")
	}

	mustWrite("references/cursor.md", "# Cursor bindings\n\n| Skill | Cursor |\n|---|---|\n"+
		"| `adopt` | runs |\n| `author-brief` | runs |\n| `the-desk` | runs |\n")

	mustWrite("cursor/packaging.md", "<!-- assay:cursor-packaging\n"+
		"adopt\nauthor-brief\nthe-desk\n"+
		"# excluded entries: <name> :: EXCLUDED: <reason>\n-->\n")

	// Generate the rule so a baseline --check is clean before any mutation.
	if code := cursorCmd([]string{"--bundle", bundle}); code != exitClean {
		t.Fatalf("writeMinimalCursorBundle: generating the baseline rule returned %d, want %d", code, exitClean)
	}

	return bundle
}

// itoa is a tiny int→string helper so the fixture builder needs no strconv import
// churn beyond the standard test set (kept local, single-digit + ten only).
func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return string(rune('0'+i/10)) + string(rune('0'+i%10))
}
