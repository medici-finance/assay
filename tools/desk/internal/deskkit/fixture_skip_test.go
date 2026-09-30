package deskkit

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
//
// It takes testing.TB rather than *testing.T so that
// TestSkipHelperWiringRefuses can drive THIS function — the wiring, not
// only the decision in absentFixtureProblem — with a recording TB. Every
// caller passes its *testing.T unchanged.
func skipIfFixtureAbsent(t testing.TB, path, why string) {
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
//
// It fails CLOSED: only a .github/workflows/ that is provably absent
// (os.ErrNotExist) makes the tree a subset. Any other stat outcome — the
// directory exists, or it cannot be examined (permission denied, an I/O
// error) — counts as a full checkout, because the subset answer is the one
// that lets an absent fixture skip, and a tree that could not be examined has
// not proved it is a subset.
func isFullCheckout(root string) bool {
	_, err := os.Stat(filepath.Join(root, ".github", "workflows"))
	return !errors.Is(err, os.ErrNotExist)
}

// fullCheckoutWitness is a file every full checkout of this repository carries
// and no published subset does: the release workflow the release-stamp guard
// reads (the subset ships no .github/ at all). The controls below use it as a
// signal INDEPENDENT of isFullCheckout's own probe, so a broken or inverted
// probe cannot vouch for itself.
var fullCheckoutWitness = releaseWorkflowPath

// witnessSaysFullCheckout reports whether fullCheckoutWitness is present. It
// fails closed the same way isFullCheckout does: only a provably absent witness
// means a subset tree.
func witnessSaysFullCheckout() bool {
	_, err := os.Stat(fullCheckoutWitness)
	return !errors.Is(err, os.ErrNotExist)
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
		// A subset skip is legitimate only if the tree really is a subset. The
		// witness is independent of isFullCheckout's probe: when it is present,
		// the probe is broken, and skipping here would hide that.
		if witnessSaysFullCheckout() {
			t.Fatalf("isFullCheckout(%s) reports a subset tree, but %s is present — the full-checkout probe is broken, so every always-skipping guard would pass as a subset skip", fixtureRepoRoot, fullCheckoutWitness)
		}
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

// skipRecorder is a testing.TB that records how skipIfFixtureAbsent ended
// instead of ending the real test. Fatal/Skip end the helper's goroutine with
// runtime.Goexit exactly as the real ones do, so the helper runs unchanged;
// every method not overridden here falls through to the real test.
type skipRecorder struct {
	testing.TB
	name    string
	failed  string // Fatal/Fatalf/FailNow/Error/Errorf/Fail
	skipped string // Skip/Skipf/SkipNow
}

func (r *skipRecorder) Helper()      {}
func (r *skipRecorder) Name() string { return r.name }
func (r *skipRecorder) Fail()        { r.failed += "Fail;" }
func (r *skipRecorder) FailNow()     { r.failed += "FailNow;"; runtime.Goexit() }
func (r *skipRecorder) Error(args ...any) {
	r.failed += fmt.Sprint(args...) + ";"
}
func (r *skipRecorder) Errorf(format string, args ...any) {
	r.failed += fmt.Sprintf(format, args...) + ";"
}
func (r *skipRecorder) Fatal(args ...any) {
	r.failed += fmt.Sprint(args...) + ";"
	runtime.Goexit()
}
func (r *skipRecorder) Fatalf(format string, args ...any) {
	r.failed += fmt.Sprintf(format, args...) + ";"
	runtime.Goexit()
}
func (r *skipRecorder) Skip(args ...any) { r.skipped += fmt.Sprint(args...) + ";"; runtime.Goexit() }
func (r *skipRecorder) Skipf(format string, args ...any) {
	r.skipped += fmt.Sprintf(format, args...) + ";"
	runtime.Goexit()
}
func (r *skipRecorder) SkipNow() { r.skipped += "SkipNow;"; runtime.Goexit() }

// driveSkipHelper runs the REAL skipIfFixtureAbsent, as the test named name,
// on path, and reports how it ended.
func driveSkipHelper(t *testing.T, name, path string) *skipRecorder {
	t.Helper()
	r := &skipRecorder{TB: t, name: name}
	done := make(chan struct{})
	go func() {
		defer close(done)
		skipIfFixtureAbsent(r, path, "control")
	}()
	<-done
	return r
}

// TestSkipHelperWiringRefuses is the end-to-end control for the class guard:
// where TestAbsentFixtureOffRegisterIsRefused drives the decision function with
// fullCheckout supplied, this drives skipIfFixtureAbsent ITSELF against the
// real repository root, so the two seams that arm the guard — the helper's call
// into absentFixtureProblem and isFullCheckout's probe — are both on the path.
// Deleting that call, inverting the probe, or mis-spelling its path makes a
// planted always-skipping guard SKIP here on a full checkout, and this fails.
//
// Whether the tree is a full checkout is decided by fullCheckoutWitness, never
// by isFullCheckout, so a broken probe cannot make the expectation agree with it.
func TestSkipHelperWiringRefuses(t *testing.T) {
	full := witnessSaysFullCheckout()
	planted := filepath.Join(fixtureRepoRoot, ".github", "workflows", "planted-no-such-workflow.yml")
	if _, err := os.Stat(planted); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("planted fixture %s unexpectedly exists (err=%v) — the control would prove nothing", planted, err)
	}

	r := driveSkipHelper(t, "TestPlantedAlwaysSkippingGuard", planted)
	switch {
	case full && r.failed == "":
		t.Errorf("full checkout (%s present): skipIfFixtureAbsent let an unregistered guard SKIP on an absent fixture (skip: %q) — the class guard is disarmed", fullCheckoutWitness, r.skipped)
	case full && !strings.Contains(r.failed, "TestPlantedAlwaysSkippingGuard"):
		t.Errorf("refusal does not name the planted test: %q", r.failed)
	case full && r.skipped != "":
		t.Errorf("the planted guard was refused but ALSO skipped (%q) — a refusal must end the test", r.skipped)
	case !full && (r.skipped == "" || r.failed != ""):
		t.Errorf("subset tree (%s absent): an absent fixture must skip, got failed=%q skipped=%q", fullCheckoutWitness, r.failed, r.skipped)
	}

	// A present fixture neither fails nor skips: the guard runs in full.
	present := filepath.Join("..", "..", "go.mod")
	if r := driveSkipHelper(t, "TestPlantedAlwaysSkippingGuard", present); r.failed != "" || r.skipped != "" {
		t.Errorf("present fixture %s: skipIfFixtureAbsent must return and let the test run, got failed=%q skipped=%q", present, r.failed, r.skipped)
	}

	// A registered test at its registered (absent) path still skips, on either
	// tree. The first entry in sorted order keeps the control deterministic.
	names := make([]string, 0, len(knownAbsentFixtures))
	for n := range knownAbsentFixtures {
		names = append(names, n)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return
	}
	reg := filepath.Join(fixtureRepoRoot, filepath.FromSlash(knownAbsentFixtures[names[0]]))
	if _, err := os.Stat(reg); !errors.Is(err, os.ErrNotExist) {
		return // stale entry: TestKnownAbsentFixturesAreStillAbsent reports it
	}
	if r := driveSkipHelper(t, names[0], reg); r.failed != "" || r.skipped == "" {
		t.Errorf("registered test %s at its registered path: must skip, got failed=%q skipped=%q", names[0], r.failed, r.skipped)
	}
}

// TestFullCheckoutProbe pins isFullCheckout's contract on constructed trees,
// including the fail-closed case: a .github/workflows/ that cannot be examined
// is NOT proof of a subset, so it must read as a full checkout.
func TestFullCheckoutProbe(t *testing.T) {
	subset := t.TempDir()
	if isFullCheckout(subset) {
		t.Errorf("a tree with no .github/ must read as a subset")
	}

	full := t.TempDir()
	if err := os.MkdirAll(filepath.Join(full, ".github", "workflows"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !isFullCheckout(full) {
		t.Errorf("a tree carrying .github/workflows/ must read as a full checkout")
	}

	sealed := t.TempDir()
	gh := filepath.Join(sealed, ".github")
	if err := os.MkdirAll(filepath.Join(gh, "workflows"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(gh, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(gh, 0o755) })
	if _, err := os.Stat(filepath.Join(gh, "workflows")); err == nil {
		t.Log("stat of an unreadable .github/workflows/ succeeded here (privileged user or a platform without mode bits) — the fail-closed branch is not exercised on this run")
	}
	if !isFullCheckout(sealed) {
		t.Errorf("a .github/workflows/ that cannot be examined must read as a full checkout (fail closed), not as a subset that lets absent fixtures skip")
	}
}
