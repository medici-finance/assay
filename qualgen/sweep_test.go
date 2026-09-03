package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/qualgen/verifier"
)

// --- fixture helpers -------------------------------------------------------

// copyTarget copies the planted target tree into a fresh temp dir and returns
// its path. The sweep reads this tree read-only.
func copyTarget(t *testing.T) string {
	t.Helper()
	src := filepath.Join("testdata", "sweep", "target")
	dst := t.TempDir()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("reading target fixtures: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", e.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(dst, e.Name()), b, 0o644); err != nil {
			t.Fatalf("writing %s: %v", e.Name(), err)
		}
	}
	return dst
}

// gitInitBestEffort makes dst a one-commit git repo so targetSHA resolves. If
// git is unavailable it is a no-op — targetSHA then records could-not-measure,
// which no test in this file asserts against.
func gitInitBestEffort(t *testing.T, dst string) {
	t.Helper()
	run := func(args ...string) bool {
		cmd := exec.Command("git", args...)
		cmd.Dir = dst
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Logf("git %v: %v (%s)", args, err, out)
			return false
		}
		return true
	}
	if !run("init", "-q") {
		return
	}
	run("-c", "user.email=t@t", "-c", "user.name=t", "add", ".")
	run("-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "-m", "fixture")
}

// cannedOutput reads a committed canned linter output fixture.
func cannedOutput(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "sweep", "canned", name))
	if err != nil {
		t.Fatalf("reading canned output %s: %v", name, err)
	}
	return string(b)
}

// cannedRunner returns a commandRunner that answers from a category→output map.
// A category not in the map is reported as a not-installed tool (lookErr), so a
// test can exercise the could-not-measure path without a live toolchain. The
// configured Command's last token names the category.
func cannedRunner(outputs map[string]string) commandRunner {
	return func(_ string, argv []string) (string, bool, error) {
		cat := argv[len(argv)-1]
		out, ok := outputs[cat]
		if !ok {
			return "", true, fmt.Errorf("canned: no tool for %q", cat)
		}
		return out, false, nil
	}
}

var fixtureRules = map[string]string{
	"dead-code":       "U1000",
	"swallowed-error": "errcheck",
	"module-size":     "lll",
	"duplication":     "dupl",
}

// toolConfigFor builds a config whose Tools map covers exactly cats.
func toolConfigFor(cats ...string) map[string]ToolConfig {
	tools := map[string]ToolConfig{}
	for _, c := range cats {
		tools[c] = ToolConfig{Command: []string{"canned", c}, Rule: fixtureRules[c]}
	}
	return tools
}

// loadFixtureVerifier loads a scripted verdict fixture and returns it so a test
// can read its Calls() count.
func loadFixtureVerifier(t *testing.T, name string) *verifier.Fixture {
	t.Helper()
	fx, err := verifier.LoadFixture(filepath.Join("testdata", "sweep", name))
	if err != nil {
		t.Fatalf("loading verifier fixture %s: %v", name, err)
	}
	return fx
}

// hashTree hashes the non-.git file contents of dir, so a test can prove the
// target tree was untouched by a sweep.
func hashTree(t *testing.T, dir string) string {
	t.Helper()
	h := sha256.New()
	var files []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		files = append(files, p)
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", dir, err)
	}
	sort.Strings(files)
	for _, p := range files {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("reading %s: %v", p, err)
		}
		rel, _ := filepath.Rel(dir, p)
		fmt.Fprintf(h, "%s\x00%d\x00", filepath.ToSlash(rel), len(b))
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil))
}

var fixedNow = time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)

// --- Verify row 3: end-to-end dereference ---------------------------------

func TestSweep_FixtureRepo_EndToEnd(t *testing.T) {
	repo := copyTarget(t)
	gitInitBestEffort(t, repo)
	out := t.TempDir()
	store := NewStore(out)

	cfg := SweepConfig{Tools: toolConfigFor("dead-code", "swallowed-error", "module-size", "duplication")}
	fx := loadFixtureVerifier(t, "verdicts.json")
	runner := cannedRunner(map[string]string{
		"dead-code":       cannedOutput(t, "dead-code.out"),
		"swallowed-error": cannedOutput(t, "swallowed-error.out"),
		"module-size":     cannedOutput(t, "module-size.out"),
		"duplication":     cannedOutput(t, "duplication.out"),
	})

	run, err := runSweepLane(repo, store, cfg, fx, runner, false, fixedNow)
	if err != nil {
		t.Fatalf("runSweepLane: %v", err)
	}
	if len(run.New) != 4 {
		t.Fatalf("want 4 new suspects, got %d", len(run.New))
	}

	report := renderSweepReport(run)
	// The report must NAME the planted dead function's file path and the rule
	// that flagged it — proving legs 1→2→3 dereferenced the same evidence.
	for _, want := range []string{"dead.go", "U1000", "unusedHelper"} {
		if !strings.Contains(report, want) {
			t.Errorf("report does not mention %q:\n%s", want, report)
		}
	}
	// The confirmed dead-code suspect must render as actionable.
	if !strings.Contains(report, "**confirmed**") {
		t.Errorf("report has no confirmed/actionable verdict:\n%s", report)
	}
	// The intentional-duplication false positive must be SUPPRESSED with reason,
	// not actionable.
	if !strings.Contains(report, "Suppressed") || !strings.Contains(report, "intentionally parallel") {
		t.Errorf("false-positive not rendered in suppressed section with reason:\n%s", report)
	}

	// The report file must be written under --out, and the append-only tables
	// populated.
	reportPath, err := writeSweepReport(store, run)
	if err != nil {
		t.Fatalf("writeSweepReport: %v", err)
	}
	if !strings.HasPrefix(reportPath, out) {
		t.Errorf("report %q not under out %q", reportPath, out)
	}
	suspects, err := store.ReadSuspects()
	if err != nil || len(suspects) != 4 {
		t.Fatalf("want 4 persisted suspects, got %d (err %v)", len(suspects), err)
	}
}

// --- Verify row 4: evidence enforcement (negative path) --------------------

func TestSweep_EvidenceFreeVerdictNotConfirmed(t *testing.T) {
	repo := copyTarget(t)
	out := t.TempDir()
	store := NewStore(out)

	cfg := SweepConfig{Tools: toolConfigFor("dead-code")}
	fx := loadFixtureVerifier(t, "verdicts-evidencefree.json")
	runner := cannedRunner(map[string]string{"dead-code": cannedOutput(t, "dead-code.out")})

	run, err := runSweepLane(repo, store, cfg, fx, runner, false, fixedNow)
	if err != nil {
		t.Fatalf("runSweepLane: %v", err)
	}
	if len(run.New) != 1 {
		t.Fatalf("want 1 new suspect, got %d", len(run.New))
	}
	fp := run.New[0].Fingerprint

	// The evidence-free `confirmed` must have been reclassified could-not-verify.
	v := run.Verdicts[fp]
	if v.Class != verifier.ClassCouldNotVerify {
		t.Fatalf("evidence-free confirmed not reclassified: class=%q", v.Class)
	}
	if !run.Reclassified[fp] {
		t.Errorf("reclassification not recorded for %s", fp)
	}

	report := renderSweepReport(run)
	// It must NOT appear as an actionable confirmed verdict.
	if strings.Contains(report, "**confirmed**") {
		t.Errorf("reclassified suspect still rendered actionable:\n%s", report)
	}
	// It must be listed as could-not-verify, naming the gate.
	if !strings.Contains(report, "reclassified by the evidence gate") {
		t.Errorf("could-not-verify listing missing the evidence-gate note:\n%s", report)
	}
}

// --- Verify row 5: standing-lane incrementality ----------------------------

func TestSweep_RerunSkipsAdjudicated(t *testing.T) {
	repo := copyTarget(t)
	out := t.TempDir()
	store := NewStore(out)
	cfg := SweepConfig{Tools: toolConfigFor("dead-code")}

	runnerA := cannedRunner(map[string]string{"dead-code": cannedOutput(t, "dead-code.out")})

	// Run A: one new suspect, verified exactly once.
	fxA := loadFixtureVerifier(t, "verdicts.json")
	runA, err := runSweepLane(repo, store, cfg, fxA, runnerA, false, fixedNow)
	if err != nil {
		t.Fatalf("run A: %v", err)
	}
	if len(runA.New) != 1 || fxA.Calls() != 1 {
		t.Fatalf("run A: new=%d calls=%d, want 1/1", len(runA.New), fxA.Calls())
	}

	// Run B: same tree — ZERO suspects sent to the verifier; all persistent.
	fxB := loadFixtureVerifier(t, "verdicts.json")
	runB, err := runSweepLane(repo, store, cfg, fxB, runnerA, false, fixedNow)
	if err != nil {
		t.Fatalf("run B: %v", err)
	}
	if fxB.Calls() != 0 {
		t.Errorf("run B verified %d suspect(s), want 0 (adjudicated ones must be skipped)", fxB.Calls())
	}
	if len(runB.New) != 0 || len(runB.Persistent) != 1 {
		t.Errorf("run B: new=%d persistent=%d, want 0/1", len(runB.New), len(runB.Persistent))
	}

	// Run C: a newly planted suspect (dead2.go) — verified exactly once and
	// sectioned new; the pre-existing one stays persistent and is not re-verified.
	runnerC := cannedRunner(map[string]string{"dead-code": cannedOutput(t, "dead-code-2.out")})
	fxC := loadFixtureVerifier(t, "verdicts.json")
	runC, err := runSweepLane(repo, store, cfg, fxC, runnerC, false, fixedNow)
	if err != nil {
		t.Fatalf("run C: %v", err)
	}
	if fxC.Calls() != 1 {
		t.Errorf("run C verified %d suspect(s), want exactly 1 (only the new one)", fxC.Calls())
	}
	if len(runC.New) != 1 || len(runC.Persistent) != 1 {
		t.Errorf("run C: new=%d persistent=%d, want 1/1", len(runC.New), len(runC.Persistent))
	}
	if !strings.Contains(runC.New[0].File, "dead2.go") {
		t.Errorf("run C new suspect is %q, want dead2.go", runC.New[0].File)
	}
}

// --- Verify row 6: read-only posture ---------------------------------------

func TestSweep_TargetTreeUnmodified(t *testing.T) {
	repo := copyTarget(t)
	out := t.TempDir()
	store := NewStore(out)
	cfg := SweepConfig{Tools: toolConfigFor("dead-code", "swallowed-error", "module-size", "duplication")}
	fx := loadFixtureVerifier(t, "verdicts.json")
	runner := cannedRunner(map[string]string{
		"dead-code":       cannedOutput(t, "dead-code.out"),
		"swallowed-error": cannedOutput(t, "swallowed-error.out"),
		"module-size":     cannedOutput(t, "module-size.out"),
		"duplication":     cannedOutput(t, "duplication.out"),
	})

	before := hashTree(t, repo)
	run, err := runSweepLane(repo, store, cfg, fx, runner, false, fixedNow)
	if err != nil {
		t.Fatalf("runSweepLane: %v", err)
	}
	if _, err := writeSweepReport(store, run); err != nil {
		t.Fatalf("writeSweepReport: %v", err)
	}
	after := hashTree(t, repo)

	if before != after {
		t.Errorf("target tree modified by sweep: %s != %s", before, after)
	}
	// All artifacts must land under --out.
	sweepDir := filepath.Join(out, qualityDir, sweepSubdir)
	for _, f := range []string{suspectsTable, verdictsTable, SweepReportName(run.RunDate)} {
		if _, err := os.Stat(filepath.Join(sweepDir, f)); err != nil {
			t.Errorf("expected artifact %s under out: %v", f, err)
		}
	}
}

// --- Verify row 7: three-state, config-driven ------------------------------

func TestSweep_NoToolsConfigured_CouldNotMeasure(t *testing.T) {
	repo := copyTarget(t)
	out := t.TempDir()
	store := NewStore(out)

	// Empty tool config: no category has a tool.
	cfg := SweepConfig{Tools: map[string]ToolConfig{}}
	runner := cannedRunner(map[string]string{}) // never consulted

	run, err := runSweepLane(repo, store, cfg, nil, runner, false, fixedNow)
	if err != nil {
		t.Fatalf("runSweepLane: %v", err)
	}
	if len(run.New) != 0 || len(run.Persistent) != 0 {
		t.Fatalf("empty config produced suspects: new=%d persistent=%d", len(run.New), len(run.Persistent))
	}
	if len(run.Categories) != len(SweepCategories) {
		t.Fatalf("want %d categories reported, got %d", len(SweepCategories), len(run.Categories))
	}
	for _, cat := range run.Categories {
		if cat.State.State != StateCouldNotMeasure {
			t.Errorf("category %q is %q, want could-not-measure (a hardcoded default tool set would fail this)", cat.Category, cat.State.State)
		}
	}
	report := renderSweepReport(run)
	if strings.Count(report, "could-not-measure") < len(SweepCategories) {
		t.Errorf("report does not mark every category could-not-measure:\n%s", report)
	}
	// No category CELL may read measured-zero (the explanatory footer may name
	// the term; the table rows must not).
	if strings.Contains(report, measureStateCell(MeasuredZero[int]())) {
		t.Errorf("report shows a measured-zero cell where it should show could-not-measure:\n%s", report)
	}
}
