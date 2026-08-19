package main

import (
	"os"
	"os/exec"
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

// ----- new numeric ID fails when git IS present but origin/main is
// unresolvable — T9's fail-CLOSED default -----

func TestIDFormatProblemsNewNumericInFixture(t *testing.T) {
	root := t.TempDir()
	gitRun(t, root, "init", "-q")
	fdir := filepath.Join(root, "docs", "streams", "findings")
	mustMkdirAll(t, fdir)
	writeTemp(t, fdir, "2026-07-15-new-numeric.md",
		"---\nid: F-33\ndate: \"2026-07-15\"\ntitle: New finding with numeric id\naffects: []\nresolved: true\n---\n\nBody.")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "add F-33")

	// A real git checkout with no origin/main ref fails CLOSED (T9): all ids
	// are treated as new, so any numeric id should trigger the regression
	// PROBLEM. This is deliberately unchanged — the fix there
	// is scoped to trees with NO .git directory at all (see
	// TestIDFormatProblemsLegacyNumericSkippedWithoutGit below), not to a real
	// checkout whose origin/main happens to be unresolvable.
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

// ----- a tree with NO .git directory at all (a `git archive`
// export) must not misjudge a pre-existing legacy-numeric id as a freshly
// minted regression -----

func TestIDFormatProblemsLegacyNumericSkippedWithoutGit(t *testing.T) {
	root := t.TempDir()
	fdir := filepath.Join(root, "docs", "streams", "findings")
	mustMkdirAll(t, fdir)
	os.WriteFile(filepath.Join(fdir, "2026-07-08-legacy.md"),
		[]byte("---\nid: F-01\ndate: \"2026-07-08\"\ntitle: Legacy finding\naffects: []\nresolved: true\n---\n\nBody."), 0o644)

	// root has no .git directory at all — the git-archive export scenario.
	// Without ANY history to consult, the numeric-regression
	// rule cannot distinguish a pre-existing legacy id from one freshly
	// minted on this branch, so it must be skipped outright rather than
	// mis-fire against every legacy entry (87 of them, in the field
	// report).
	problems := idFormatProblems(root)
	for _, p := range problems {
		if strings.Contains(p, "F-01") {
			t.Errorf("legacy numeric id F-01 must not fire a PROBLEM with no .git directory present; got %v", problems)
		}
	}
	if !isValidRegisterID("F-01") {
		t.Error("F-01 should be valid format (legacy numeric)")
	}

	// The degradation must still be visible, not silently indistinguishable
	// from a clean run: grandfatheredBaseFallbackNotices names the missing
	// .git directory as the cause.
	notices := grandfatheredBaseFallbackNotices(root)
	found := false
	for _, n := range notices {
		if strings.Contains(n, "no .git directory") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a NOTICE naming the missing .git directory as the cause; got %v", notices)
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
	gitRun(t, root, "init", "-q")
	fdir := filepath.Join(root, "docs", "streams", "findings")
	mustMkdirAll(t, fdir)

	writeTemp(t, fdir, "2026-07-15-slug.md",
		"---\nid: F-oracle-topology\ndate: \"2026-07-15\"\ntitle: Oracle topology issue\naffects: []\nresolved: true\n---\n\nBody.")
	writeTemp(t, fdir, "2026-07-15-new-numeric.md",
		"---\nid: F-50\ndate: \"2026-07-15\"\ntitle: Should not be numeric\naffects: []\nresolved: true\n---\n\nBody.")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "add fixtures")

	// A real git checkout with no origin/main ref (T9 fail-closed): the
	// numeric id is treated as new and flagged; the slug id is unaffected.
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
	gitRun(t, root, "init", "-q")
	idir := filepath.Join(root, "docs", "streams", "intake")
	mustMkdirAll(t, idir)

	writeTemp(t, idir, "2026-07-15-valid.md",
		"---\nid: I-model-mix-tiers\ndate: \"2026-07-15\"\ntitle: Model mix tiers\ndisposition: new\n---\n\nBody.")
	writeTemp(t, idir, "2026-07-15-numeric.md",
		"---\nid: I-99\ndate: \"2026-07-15\"\ntitle: Should not be numeric\ndisposition: new\n---\n\nBody.")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "add fixtures")

	// A real git checkout with no origin/main ref (T9 fail-closed).
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

// gitCapture runs git in root and returns trimmed stdout — the read-only
// counterpart of gitRun, used by the #885 positive control to prove the decoy
// actually shadows the bare short name at this git version.
func gitCapture(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

// TestGrandfatheredIDsResolvesQualifiedRemoteRef is the #885 positive control
// for statusgen's grandfathering: it must resolve refs/remotes/origin/main, not
// the bare short name `origin/main`. A checkout carrying a stray LOCAL branch
// literally named `refs/heads/origin/main` makes the short name ambiguous, and
// refs/heads/ wins gitrevisions disambiguation — so bare `origin/main` silently
// resolves to that decoy. Parked at a commit that PREDATES a legacy numeric
// finding, the decoy would make the grandfathered set be computed against the
// stale tree: the landed legacy ID is then treated as new and fires a spurious
// numeric-regression PROBLEM — statusgen "false-disproving" against a base
// nobody is on. Qualified, the merge-base is the real remote tip and the landed
// ID is correctly grandfathered.
func TestGrandfatheredIDsResolvesQualifiedRemoteRef(t *testing.T) {
	root := t.TempDir()
	gitRun(t, root, "init", "-q")
	fdir := filepath.Join(root, "docs", "streams", "findings")
	mustMkdirAll(t, fdir)

	// C0: a stale commit that PREDATES the legacy finding.
	writeTemp(t, root, "README.md", "seed\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "seed")

	// C1: land a legacy numeric finding F-01 on main. HEAD is now C1.
	writeTemp(t, fdir, "2026-07-08-legacy.md",
		"---\nid: F-01\ndate: \"2026-07-08\"\ntitle: Legacy finding\naffects: []\nresolved: true\n---\n\nBody.")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "main: land legacy F-01")

	// The REAL remote tip includes the landed F-01; the DECOY branch is parked
	// one commit back (C0), before it landed.
	gitRun(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")
	gitRun(t, root, "update-ref", "refs/heads/origin/main", "HEAD~1")

	// Positive control: prove the decoy actually shadows the bare name at this
	// git version, so a green result below is meaningful and not a void test.
	bare := gitCapture(t, root, "rev-parse", "origin/main")
	remote := gitCapture(t, root, "rev-parse", "refs/remotes/origin/main")
	stale := gitCapture(t, root, "rev-parse", "HEAD~1")
	if bare != stale {
		t.Fatalf("positive control void: bare origin/main = %s, want the decoy %s", bare, stale)
	}
	if remote == stale {
		t.Fatal("positive control void: the remote tip did not advance past the decoy")
	}

	// With the qualified ref, the merge-base is the real tip (C1), so the landed
	// F-01 IS grandfathered and fires NO numeric-regression problem.
	if g := grandfatheredIDs(root); !g["F-01"] {
		t.Errorf("F-01 landed on the real origin/main but was not grandfathered — "+
			"the bare-name decoy shadowed the remote tip; got %v", g)
	}
	for _, p := range idFormatProblems(root) {
		if strings.Contains(p, "F-01") && strings.Contains(p, "numeric") {
			t.Errorf("landed legacy F-01 fired a spurious numeric-regression PROBLEM "+
				"(grandfathering resolved the stale decoy): %q", p)
		}
	}
}

// TestGrandfatheredFallbackCountsSplitIntake guards the evidenceexport.go /
// idvalidate.go gap the reviewer found on issue-loop/15: an intake entry that
// lives ONLY under a split-layout subdir (new/, decision-needed/, watching/,
// completed/, rejected/) — no root-level files at all — must still be
// counted by grandfatheredBaseFallbackNotices. Before routing this count
// through listIntakeFiles, a root-only os.ReadDir("docs/streams/intake")
// would see zero entries here and silently suppress the degraded-validation
// NOTICE even though a real entry exists.
func TestGrandfatheredFallbackCountsSplitIntake(t *testing.T) {
	root := t.TempDir()
	gitRun(t, root, "init", "-q")

	newDir := filepath.Join(root, "docs", "streams", "intake", "new")
	mustMkdirAll(t, newDir)
	writeTemp(t, newDir, "2026-07-16-x.md",
		"---\nid: I-split\ndate: \"2026-07-16\"\ntitle: Split entry\ndisposition: new\n---\n\nBody.")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "add split intake entry")

	// No origin/main ref is ever created, so the fallback path is taken.
	notices := grandfatheredBaseFallbackNotices(root)
	if len(notices) == 0 {
		t.Fatal("a split-layout-only intake entry must still be counted; degraded-validation NOTICE did not fire")
	}
	if !strings.Contains(notices[0], "1 register entry") {
		t.Errorf("expected the notice to count exactly 1 register entry, got %q", notices[0])
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
