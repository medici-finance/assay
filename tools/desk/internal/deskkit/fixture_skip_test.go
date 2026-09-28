package deskkit

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// fixtureRepoRoot is the repository root relative to this package
// (tools/desk/internal/deskkit sits four levels down). Every
// skipIfFixtureAbsent caller lives in this package, so every fixture path it
// is handed resolves against the same root.
const fixtureRepoRoot = "../../../.."

// skipIfFixtureAbsent skips a test whose fixture is a file or tree that is not
// part of every checkout of this repository, when that path is genuinely absent
// from the current checkout.
//
// It guards the desk-tools suite when it runs inside a published subset of this
// repository that does not carry .github/ (CI wiring), .claude/ (skills and
// settings; the shipped adopter skills live under plugins/assay/skills) or
// go.work (the subset ships its own).
//
// Fail-closed intent is preserved: ONLY os.ErrNotExist skips, so a fixture that
// exists but cannot be read still fails; and where every such fixture is
// present, the guard never fires and the test runs in full against the real
// tree.
//
// Class guard (a guard test that skips forever on a full checkout): a skip is
// only legitimate in a SUBSET tree. When the checkout is the full repository —
// it carries .github/workflows/ — an absent fixture is a path that does not
// exist here, and a test that skips on it can never fail: a false green that
// reads as a present guard. That is exactly how the release-stamp guard read a
// workflow file this repository never had and skipped on every run. On a full
// checkout, then, an absent fixture FAILS the test unless the test is on the
// committed knownAbsentFixtures register below, with the exact fixture path.
func skipIfFixtureAbsent(t *testing.T, path, why string) {
	t.Helper()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		return
	}
	if problem := absentFixtureProblem(t.Name(), path, isFullCheckout(fixtureRepoRoot)); problem != "" {
		t.Fatal(problem)
	}
	t.Skipf("fixture %s not present in this tree — %s", path, why)
}

// isFullCheckout reports whether root is a full checkout of this repository
// rather than a published subset: a subset does not carry .github/, a full
// checkout always carries .github/workflows/.
func isFullCheckout(root string) bool {
	fi, err := os.Stat(filepath.Join(root, ".github", "workflows"))
	return err == nil && fi.IsDir()
}

// knownAbsentFixtures is the committed register of the tests whose fixture is
// KNOWN absent from a full checkout of this repository, keyed by top-level test
// name, valued by the repo-relative fixture path each one is allowed to skip
// on. Every entry is a guard that does not run here — debt, not a pass — and is
// tracked for repair (point it at the real path, or retire it). The register
// exists so that a NEW always-skipping guard cannot land silently: a test not
// listed here, or listed with a different path, fails instead of skipping.
//
// Never add an entry to make a red test green. An entry is only for a fixture
// that genuinely belongs to another tree; a guard whose fixture lives in THIS
// repository under another name must be pointed at it instead.
var knownAbsentFixtures = map[string]string{
	"TestAssayLintSingleWriterGuardFailsClosed":        ".github/actions/assay-lint/action.yml",
	"TestAssayLintGuardPullRequestScopedAndBashed":     ".github/actions/assay-lint/action.yml",
	"TestCrossModuleTestsAreTriggeredByWhatTheyRead":   ".github/workflows/tools.yml",
	"TestCrossModuleReaderRegistryIsNotSilentlyStale":  ".github/workflows/tools.yml",
	"TestEveryDeclaredLoopIdentityIsKnown":             ".claude/skills",
	"TestRosterCarriesNoUndeclaredCanonicalName":       ".claude/skills",
	"TestPreRegistrationsAreRetiredWhenDeclared":       ".claude/skills",
	"TestGoModModulesUseAssayPrefix":                   "go.work",
	"TestSkillsRaisedByVocabularyMatchesTheRoster":     ".claude/skills",
	"TestSkillsRaisedByRolesCarryNoPersonalIdentifier": ".claude/skills",
	"TestSkillBodiesCarryNoRetiredDeskName":            ".claude/skills",
	"TestS2SweepExclusionsAreLive":                     "docs/leak-sweep-tokens.yaml",
	"TestActingDeskSkillsCarryNoHardcodedRepoList":     ".claude/skills",
	"TestTreeSweepPipeCarriesPipefail":                 ".github/workflows/leaksweep.yml",
}

// fixtureRepoRel returns path relative to the repository root, slash-separated,
// or path itself (cleaned, slash-separated) when it cannot be related.
func fixtureRepoRel(path string) string {
	absRoot, err1 := filepath.Abs(fixtureRepoRoot)
	absPath, err2 := filepath.Abs(path)
	if err1 == nil && err2 == nil {
		if rel, err := filepath.Rel(absRoot, absPath); err == nil {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(filepath.Clean(path))
}

// absentFixtureProblem decides whether an ABSENT fixture may skip. It returns
// "" when the skip is legitimate — a subset tree, or a registered known-absent
// fixture — and the failure message otherwise. testName may be a subtest name;
// the register is keyed by the top-level test.
func absentFixtureProblem(testName, path string, fullCheckout bool) string {
	if !fullCheckout {
		return ""
	}
	top, _, _ := strings.Cut(testName, "/")
	rel := fixtureRepoRel(path)
	want, listed := knownAbsentFixtures[top]
	if listed && want == rel {
		return ""
	}
	if listed {
		return fmt.Sprintf("%s: fixture %s is absent, but knownAbsentFixtures registers this test for %s — a changed fixture path must be pointed at a file that exists, never re-registered to skip", top, rel, want)
	}
	return fmt.Sprintf("%s: fixture %s is absent from this full checkout (it carries .github/workflows/), so this guard would SKIP on every run and can never fail — point it at the file that actually exists in this repository", top, rel)
}

// TestAbsentFixtureOffRegisterIsRefused is the positive control for the class
// guard above: a planted absent fixture on a full checkout must be refused, and
// the two legitimate skips (a subset tree; a registered test at its registered
// path) must still pass through. A class guard whose matcher could silently stop
// matching carries this control so a broken guard fails instead of reporting
// clean.
func TestAbsentFixtureOffRegisterIsRefused(t *testing.T) {
	planted := filepath.Join(fixtureRepoRoot, ".github", "workflows", "planted-no-such-workflow.yml")
	if _, err := os.Stat(planted); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("planted fixture %s unexpectedly exists (err=%v) — the control would prove nothing", planted, err)
	}

	if p := absentFixtureProblem("TestPlantedAlwaysSkippingGuard", planted, true); p == "" {
		t.Error("an unregistered test skipping on an absent fixture in a full checkout was NOT refused — the class guard lets always-skipping guards land")
	} else if !strings.Contains(p, "TestPlantedAlwaysSkippingGuard") || !strings.Contains(p, ".github/workflows/planted-no-such-workflow.yml") {
		t.Errorf("refusal does not name the planted test and fixture: %q", p)
	}
	if p := absentFixtureProblem("TestPlantedAlwaysSkippingGuard/sub", planted, true); p == "" {
		t.Error("a subtest of an unregistered test was NOT refused")
	}
	if p := absentFixtureProblem("TestPlantedAlwaysSkippingGuard", planted, false); p != "" {
		t.Errorf("a subset tree (no .github/workflows/) must still skip, got refusal %q", p)
	}

	// A registered test skips only at its REGISTERED path.
	var name, reg string
	for n, r := range knownAbsentFixtures {
		name, reg = n, r
		break
	}
	if name == "" {
		return // empty register: nothing further to control
	}
	if p := absentFixtureProblem(name, filepath.Join(fixtureRepoRoot, filepath.FromSlash(reg)), true); p != "" {
		t.Errorf("registered test %s at its registered path %s was refused: %q", name, reg, p)
	}
	if p := absentFixtureProblem(name, planted, true); p == "" {
		t.Errorf("registered test %s skipping on a DIFFERENT absent path was not refused — the register would launder any path", name)
	}
}

// TestKnownAbsentFixturesAreStillAbsent keeps the register honest: an entry
// whose fixture now exists is stale (its test runs in full) and must be
// removed, so the register only ever lists guards that genuinely do not run.
// The release-stamp guard in particular must never be registered: it reads a
// workflow this repository carries.
func TestKnownAbsentFixturesAreStillAbsent(t *testing.T) {
	if !isFullCheckout(fixtureRepoRoot) {
		t.Skip("subset tree (no .github/workflows/): the register describes the full checkout")
	}
	names := make([]string, 0, len(knownAbsentFixtures))
	for n := range knownAbsentFixtures {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		rel := knownAbsentFixtures[n]
		if _, err := os.Stat(filepath.Join(fixtureRepoRoot, filepath.FromSlash(rel))); err == nil {
			t.Errorf("knownAbsentFixtures lists %s for %s, but that fixture now exists — remove the stale entry", n, rel)
		}
	}
	for _, n := range []string{"TestVersionStampedFromReleaseWorkflow", "TestReleaseTagStampMissingIsCaught"} {
		if _, listed := knownAbsentFixtures[n]; listed {
			t.Errorf("%s is registered as known-absent — the release-stamp guard must run against .github/workflows/release.yml, never skip", n)
		}
	}
}
