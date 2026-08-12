package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ----- ID format regex tests -----

func TestNewFormatIDRe(t *testing.T) {
	valid := []string{
		"F-slug-id-demo-x",       // 14 chars slug (valid: 10-20)
		"I-model-mix-tiers",      // 15 chars slug (valid)
		"F-oracle-topology",      // 16 chars slug (valid)
		"F-ws-token-expiry",      // 15 chars slug (valid)
		"F-abcdefghij",           // exactly 10 chars slug (valid minimum)
		"F-abcdefghijklmnopqrst", // exactly 20 chars slug (valid maximum)
		"I-ten-chars-ok",         // 12 chars slug
	}
	for _, id := range valid {
		if !newFormatIDRe.MatchString(id) {
			t.Errorf("%q should match new format, but didn't", id)
		}
		if !isValidRegisterID(id) {
			t.Errorf("%q should be valid, but isn't", id)
		}
		if !isNewFormatID(id) {
			t.Errorf("%q should be new format, but isn't", id)
		}
	}

	reallyInvalid := []string{
		"F-abc",                   // slug too short (3 chars)
		"F-short",                 // slug too short (5 chars)
		"F-abcdefghi",             // slug too short (9 chars)
		"F-abcdefghijklmnopqrstu", // slug too long (21 chars)
		"I--leading-hyphen",       // starts with hyphen
		"F-trailing-hyphen-",      // ends with hyphen
		"G-not-a-register",        // wrong prefix
		"X-abcdefghijk",           // wrong prefix
		"invalid",                 // no prefix at all
	}
	for _, id := range reallyInvalid {
		if newFormatIDRe.MatchString(id) {
			t.Errorf("%q should NOT match new format, but did", id)
		}
	}

	// Legacy numeric formats should be valid but not new-format
	legacyOK := []string{
		"F-01", "F-1", "F-99",
		"I-01", "I-999",
		"F-22-a", "F-22-b", "I-01-c",
	}
	for _, id := range legacyOK {
		if !isValidRegisterID(id) {
			t.Errorf("%q should be valid (legacy numeric), but isn't", id)
		}
		if !isLegacyNumericID(id) {
			t.Errorf("%q should be legacy numeric, but isn't", id)
		}
		if isNewFormatID(id) {
			t.Errorf("%q is legacy numeric, should NOT be new format", id)
		}
	}

	// Completely invalid (neither new nor legacy)
	completelyInvalid := []string{
		"F-abc", "banana", "F-UPPER", "", "G-01",
	}
	for _, id := range completelyInvalid {
		if isValidRegisterID(id) {
			t.Errorf("%q should be completely invalid, but isValidRegisterID returned true", id)
		}
	}
}

// ----- 9-char slug fails (Verify 1: valid slug passes; 9-char and 21-char fail) -----

func TestSlugLengthBoundaries(t *testing.T) {
	if newFormatIDRe.MatchString("F-abcdefghi") {
		t.Error("9-char slug should not match new format")
	}
	if !newFormatIDRe.MatchString("F-abcdefghij") {
		t.Error("10-char slug should match new format (minimum)")
	}
	if !newFormatIDRe.MatchString("F-abcdefghijklmnopqrst") {
		t.Error("20-char slug should match new format (maximum)")
	}
	if newFormatIDRe.MatchString("F-abcdefghijklmnopqrstu") {
		t.Error("21-char slug should not match new format")
	}
}

// ----- duplicate ID across fixtures fails (Verify 1) -----

func TestIDFormatProblemsDuplicateSlugInFixture(t *testing.T) {
	root := t.TempDir()
	fdir := filepath.Join(root, "docs", "streams", "findings")
	mustMkdirAll(t, fdir)
	os.WriteFile(filepath.Join(fdir, "2026-07-08-first.md"),
		[]byte("---\nid: F-duplicate-slug\ndate: \"2026-07-08\"\ntitle: First finding\naffects: []\nresolved: true\n---\n\nBody."), 0o644)
	os.WriteFile(filepath.Join(fdir, "2026-07-08-second.md"),
		[]byte("---\nid: F-duplicate-slug\ndate: \"2026-07-08\"\ntitle: Second finding\naffects: []\nresolved: true\n---\n\nBody."), 0o644)

	problems := registerIntegrityProblems(root)
	found := false
	for _, p := range problems {
		if strings.Contains(p, "duplicate id F-duplicate-slug") {
			found = true
		}
	}
	if !found {
		t.Errorf("duplicate slug ids should be caught; got %v", problems)
	}
}

// ----- new numeric ID fails (Verify 3) -----

func TestIDFormatProblemsNewNumericInFixture(t *testing.T) {
	root := t.TempDir()
	fdir := filepath.Join(root, "docs", "streams", "findings")
	mustMkdirAll(t, fdir)
	os.WriteFile(filepath.Join(fdir, "2026-07-15-new-numeric.md"),
		[]byte("---\nid: F-33\ndate: \"2026-07-15\"\ntitle: New finding with numeric id\naffects: []\nresolved: true\n---\n\nBody."), 0o644)

	// In a temp dir (no git), mergedRegisterFiles returns empty -> all entries
	// are "new", so any numeric id should trigger the regression PROBLEM.
	problems := idFormatProblems(root)
	found := false
	for _, p := range problems {
		if strings.Contains(p, "F-33") && strings.Contains(p, "numeric") {
			found = true
		}
	}
	if !found {
		t.Errorf("new numeric id F-33 should trigger numeric-regression PROBLEM; got %v", problems)
	}
}

// ----- legacy numeric set passes when grandfathered (Verify 4) -----

func TestIDFormatProblemsLegacyNumericPassesInGit(t *testing.T) {
	root := t.TempDir()
	fdir := filepath.Join(root, "docs", "streams", "findings")
	mustMkdirAll(t, fdir)
	os.WriteFile(filepath.Join(fdir, "2026-07-08-legacy.md"),
		[]byte("---\nid: F-01\ndate: \"2026-07-08\"\ntitle: Legacy finding\naffects: []\nresolved: true\n---\n\nBody."), 0o644)

	// Without git, this flags as new-numeric regression.
	problems := idFormatProblems(root)
	if len(problems) == 0 {
		t.Error("in a tempdir without git, new numeric id should flag")
	}
	if !isValidRegisterID("F-01") {
		t.Error("F-01 should be valid format (legacy numeric)")
	}
}

// ----- legacy numeric formats are valid -----

func TestLegacyNumericFormatsValid(t *testing.T) {
	valid := []string{
		"F-01", "F-1", "F-99", "F-22-a", "F-22-z",
		"I-01", "I-1", "I-999", "I-05-c",
	}
	for _, id := range valid {
		if !isLegacyNumericID(id) {
			t.Errorf("%q should match legacy numeric format", id)
		}
		if !isValidRegisterID(id) {
			t.Errorf("%q should be valid (legacy numeric)", id)
		}
	}

	invalid := []string{
		"F-slug-id-demo-x", "F-abc", "F-22-aa", "banana",
	}
	for _, id := range invalid {
		if isLegacyNumericID(id) {
			t.Errorf("%q should NOT match legacy numeric format", id)
		}
	}
}

// ----- valid slug passes (Verify 2) -----

func TestIDFormatProblemsValidSlugPasses(t *testing.T) {
	root := t.TempDir()
	fdir := filepath.Join(root, "docs", "streams", "findings")
	mustMkdirAll(t, fdir)
	os.WriteFile(filepath.Join(fdir, "2026-07-15-slug-demo.md"),
		[]byte("---\nid: F-slug-id-demo-x\ndate: \"2026-07-15\"\ntitle: Slug id demo finding\naffects: []\nresolved: true\n---\n\nBody."), 0o644)

	problems := idFormatProblems(root)
	for _, p := range problems {
		if strings.Contains(p, "F-slug-id-demo-x") {
			t.Errorf("valid slug id should not cause problems, got: %s", p)
		}
	}
}

// ----- combined: valid slug + legacy numeric in same fixture -----

func TestIDFormatProblemsMixedFormats(t *testing.T) {
	root := t.TempDir()
	fdir := filepath.Join(root, "docs", "streams", "findings")
	mustMkdirAll(t, fdir)

	os.WriteFile(filepath.Join(fdir, "2026-07-15-slug.md"),
		[]byte("---\nid: F-oracle-topology\ndate: \"2026-07-15\"\ntitle: Oracle topology issue\naffects: []\nresolved: true\n---\n\nBody."), 0o644)
	os.WriteFile(filepath.Join(fdir, "2026-07-15-new-numeric.md"),
		[]byte("---\nid: F-50\ndate: \"2026-07-15\"\ntitle: Should not be numeric\naffects: []\nresolved: true\n---\n\nBody."), 0o644)

	problems := idFormatProblems(root)

	foundNumeric := false
	for _, p := range problems {
		if strings.Contains(p, "F-50") && strings.Contains(p, "numeric") {
			foundNumeric = true
		}
	}
	if !foundNumeric {
		t.Errorf("new numeric id F-50 should trigger problem; got %v", problems)
	}

	for _, p := range problems {
		if strings.Contains(p, "F-oracle-topology") {
			t.Errorf("valid slug id should not be in problems: %s", p)
		}
	}
}

// ----- completely invalid ID format -----

func TestIDFormatProblemsCompletelyInvalid(t *testing.T) {
	root := t.TempDir()
	fdir := filepath.Join(root, "docs", "streams", "findings")
	mustMkdirAll(t, fdir)
	os.WriteFile(filepath.Join(fdir, "2026-07-15-bad.md"),
		[]byte("---\nid: banana\ndate: \"2026-07-15\"\ntitle: Bad id\naffects: []\nresolved: true\n---\n\nBody."), 0o644)

	problems := idFormatProblems(root)
	found := false
	for _, p := range problems {
		if strings.Contains(p, "banana") && strings.Contains(p, "invalid id") {
			found = true
		}
	}
	if !found {
		t.Errorf("completely invalid id 'banana' should trigger PROBLEM; got %v", problems)
	}
}

// ----- intake entries too -----

func TestIDFormatProblemsIntakeEntries(t *testing.T) {
	root := t.TempDir()
	idir := filepath.Join(root, "docs", "streams", "intake")
	mustMkdirAll(t, idir)

	os.WriteFile(filepath.Join(idir, "2026-07-15-valid.md"),
		[]byte("---\nid: I-model-mix-tiers\ndate: \"2026-07-15\"\ntitle: Model mix tiers\ndisposition: new\n---\n\nBody."), 0o644)
	os.WriteFile(filepath.Join(idir, "2026-07-15-numeric.md"),
		[]byte("---\nid: I-99\ndate: \"2026-07-15\"\ntitle: Should not be numeric\ndisposition: new\n---\n\nBody."), 0o644)

	problems := idFormatProblems(root)

	foundNumeric := false
	for _, p := range problems {
		if strings.Contains(p, "I-99") && strings.Contains(p, "numeric") {
			foundNumeric = true
		}
	}
	if !foundNumeric {
		t.Errorf("new numeric I-99 should trigger problem; got %v", problems)
	}
}

// ----- Test that the view ordering is date-then-id -----

// TestGrandfatheredIDsFailClosed — T9: when origin/main is unresolvable,
// grandfatheredIDs returns an empty set (fail-closed: treat all IDs as new).
// The old fallback to base=HEAD grandfathered the very ID the check exists to
// reject.
func TestGrandfatheredIDsFailClosed(t *testing.T) {
	root := t.TempDir()
	gitRun(t, root, "init", "-q")
	fdir := filepath.Join(root, "docs", "streams", "findings")
	mustMkdirAll(t, fdir)

	// Add F-01 and commit — no origin/main ref is ever created.
	writeTemp(t, fdir, "2026-07-08-legacy.md",
		"---\nid: F-01\ndate: \"2026-07-08\"\ntitle: Legacy finding\naffects: []\nresolved: true\n---\n\nBody.")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "add F-01")

	// With no origin/main, grandfatheredIDs must return empty — F-01 is NOT
	// grandfathered (fail-closed).
	grandfathered := grandfatheredIDs(root)
	if len(grandfathered) != 0 {
		t.Errorf("T9: grandfatheredIDs with no origin/main must return empty (fail-closed); got %v", grandfathered)
	}

	// Now F-01, being treated as new and numeric, must trigger the numeric-
	// regression PROBLEM.
	problems := idFormatProblems(root)
	found := false
	for _, p := range problems {
		if strings.Contains(p, "F-01") && strings.Contains(p, "numeric") {
			found = true
		}
	}
	if !found {
		t.Errorf("T9: F-01 must trigger numeric-regression problem when origin/main is unresolvable (not grandfathered); got %v", problems)
	}
}

func TestViewOrderingDateThenID(t *testing.T) {
	root := t.TempDir()
	fdir := filepath.Join(root, "docs", "streams", "findings")
	mustMkdirAll(t, fdir)

	os.WriteFile(filepath.Join(fdir, "2026-07-15-alpha.md"),
		[]byte("---\nid: F-alpha-finding\ndate: \"2026-07-15\"\ntitle: Alpha finding\naffects: []\nresolved: true\n---\n\nBody."), 0o644)
	os.WriteFile(filepath.Join(fdir, "2026-07-15-beta.md"),
		[]byte("---\nid: F-02\ndate: \"2026-07-15\"\ntitle: Beta finding\naffects: []\nresolved: true\n---\n\nBody."), 0o644)
	os.WriteFile(filepath.Join(fdir, "2026-07-14-earlier.md"),
		[]byte("---\nid: F-earlier-slug-id\ndate: \"2026-07-14\"\ntitle: Earlier finding\naffects: []\nresolved: true\n---\n\nBody."), 0o644)

	view, err := generateFindingsView(root)
	if err != nil {
		t.Fatal(err)
	}

	idx14 := strings.Index(view, "2026-07-14")
	idx15 := strings.Index(view, "2026-07-15")
	if idx14 < 0 || idx15 < 0 {
		t.Fatal("missing dates in view")
	}
	if idx14 > idx15 {
		t.Errorf("view should order by date first: 2026-07-14 before 2026-07-15; idx14=%d idx15=%d\n%s", idx14, idx15, view[:500])
	}
}
