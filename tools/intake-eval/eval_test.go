package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runCLI runs the command in-process and returns its exit code and output.
func runCLI(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := run(args, &out, &errb)
	return rc, out.String(), errb.String()
}

func runJSON(t *testing.T, args ...string) (int, Report) {
	t.Helper()
	rc, out, errs := runCLI(t, append(args, "--json")...)
	var r Report
	if rc == exitInput {
		t.Fatalf("rc=%d stderr=%s", rc, errs)
	}
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("decode report: %v\n%s", err, out)
	}
	return rc, r
}

func val(t *testing.T, v *float64, what string) float64 {
	t.Helper()
	if v == nil {
		t.Fatalf("%s is unmeasured, want a value", what)
	}
	return *v
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestPerfectScoreEveryExitF1One(t *testing.T) {
	rc, r := runJSON(t, "--gold", "testdata/gold-perfect", "--pred", "testdata/pred-perfect")
	if rc != exitOK {
		t.Fatalf("rc=%d", rc)
	}
	if len(r.Exits) != len(exits) {
		t.Fatalf("report has %d exits, want %d", len(r.Exits), len(exits))
	}
	for _, e := range exits {
		x := r.Exits[e]
		if x == nil || x.Support == 0 {
			t.Fatalf("exit %s missing or empty", e)
		}
		for name, v := range map[string]*float64{"precision": x.Precision, "recall": x.Recall, "f1": x.F1} {
			if got := val(t, v, e+" "+name); !near(got, 1) {
				t.Errorf("%s %s = %v, want 1", e, name, got)
			}
		}
	}
	// The excluded fixture is surfaced, not scored.
	if r.Scored != 10 || len(r.PendingHuman) != 1 || r.PendingHuman[0] != "syn-p11" {
		t.Errorf("scored=%d pending=%v, want 10 scored and syn-p11 pending", r.Scored, r.PendingHuman)
	}
}

// Per-exit reporting must expose a collapsed rare class that a blended
// accuracy hides.
//
// regression: #2980
func TestRareClassCollapseVisiblePerExit(t *testing.T) {
	_, r := runJSON(t, "--gold", "testdata/gold-imbalanced", "--pred", "testdata/pred-collapse")
	if acc := val(t, r.Accuracy, "accuracy"); acc < 0.9 {
		t.Fatalf("accuracy %v; the fixture is meant to keep it high", acc)
	}
	nd := r.Exits["needs-decision"]
	if got := val(t, nd.Recall, "needs-decision recall"); got != 0 {
		t.Errorf("needs-decision recall = %v, want 0", got)
	}
	if got := val(t, nd.F1, "needs-decision f1"); got != 0 {
		t.Errorf("needs-decision F1 = %v, want 0", got)
	}
	if nd.Precision != nil {
		t.Errorf("needs-decision precision = %v, want unmeasured (nothing predicted into it)", *nd.Precision)
	}
	rw := r.Exits["rejected/watching"]
	if got := val(t, rw.Precision, "rejected/watching precision"); !near(got, 8.0/9.0) {
		t.Errorf("rejected/watching precision = %v, want 8/9", got)
	}
}

// regression: #2980
func TestThresholdVerdictWiredToExitCode(t *testing.T) {
	cases := []struct {
		name, pred, thr string
		want            int
	}{
		{"regressed fails", "testdata/pred-regressed", "testdata/thresholds.json", exitFail},
		{"advisory reports without failing", "testdata/pred-regressed", "testdata/thresholds-advisory.json", exitOK},
		{"perfect passes", "testdata/pred-perfect", "testdata/thresholds.json", exitOK},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rc, out, errs := runCLI(t, "--gold", "testdata/gold-perfect", "--pred", c.pred, "--thresholds", c.thr)
			if rc != c.want {
				t.Fatalf("rc=%d want %d\n%s%s", rc, c.want, out, errs)
			}
		})
	}
	_, r := runJSON(t, "--gold", "testdata/gold-perfect", "--pred", "testdata/pred-regressed", "--thresholds", "testdata/thresholds-advisory.json")
	if r.Verdict == nil || r.Verdict.Result != "FAIL" || !r.Verdict.Advisory {
		t.Fatalf("advisory run must still report FAIL: %+v", r.Verdict)
	}
	// The precision floor on a work-spawning exit is what catches the misroute
	// into bug/issue, independently of its F1 floor.
	var sawPrecision bool
	for _, c := range r.Verdict.Checks {
		if c.Exit == "bug/issue" && c.Metric == "precision" && c.Status == "fail" {
			sawPrecision = true
		}
	}
	if !sawPrecision {
		t.Errorf("no failing bug/issue precision check in %+v", r.Verdict.Checks)
	}
}

// fixtureYAML renders one synthetic fixture; edit tweaks the field lines.
func fixtureYAML(id, exit, scoring, ratification, version string) string {
	return fmt.Sprintf(`id: %s
inbound: "synthetic inbound %s"
expected_exit: %s
methodology_version: "%s"
scoring: %s
provenance:
  type: synthetic-ruling
  tier: synthetic-tier
  ratification: %s
  ref: synthetic/ref
  confirmed_by: synthetic-reviewer
  date: "%s"
  quote: "synthetic quote"
`, id, id, exit, version, scoring, ratification, version)
}

// writeGold writes one scored fixture per exit plus extra files into a temp dir.
func writeGold(t *testing.T, extra map[string]string, skipExit string) string {
	t.Helper()
	dir := t.TempDir()
	for i, e := range exits {
		if e == skipExit {
			continue
		}
		id := fmt.Sprintf("syn-t%02d", i)
		if err := os.WriteFile(filepath.Join(dir, id+".yaml"), []byte(fixtureYAML(id, e, "scored", "confirmed", "2026-01-01")), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range extra {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A non-fixture file in the corpus directory is ignored.
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("not a fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// regression: #2980
func TestPreflightLintRejectsInvalidFixtures(t *testing.T) {
	noProv := "id: syn-x\ninbound: x\nexpected_exit: finding\nmethodology_version: \"2026-01-01\"\nscoring: scored\n"
	cases := []struct {
		name, body, skip, want string
	}{
		{"missing provenance", noProv, "", "missing provenance"},
		{"empty provenance quote", strings.Replace(fixtureYAML("syn-x", "finding", "scored", "confirmed", "2026-01-01"), `quote: "synthetic quote"`, `quote: ""`, 1), "", "provenance.quote"},
		{"unknown exit", fixtureYAML("syn-x", "spec", "scored", "confirmed", "2026-01-01"), "", "not one of the five exits"},
		{"scored but pending-human", fixtureYAML("syn-x", "finding", "scored", "pending-human", "2026-01-01"), "", "requires provenance.ratification: confirmed"},
		{"excluded but confirmed", fixtureYAML("syn-x", "finding", "excluded", "confirmed", "2026-01-01"), "", "inconsistent"},
		{"bad scoring", fixtureYAML("syn-x", "finding", "maybe", "confirmed", "2026-01-01"), "", "must be scored or excluded"},
		{"missing methodology_version", strings.Replace(fixtureYAML("syn-x", "finding", "scored", "confirmed", "2026-01-01"), "methodology_version: \"2026-01-01\"\n", "", 1), "", "missing methodology_version"},
		{"duplicate id", fixtureYAML("syn-t02", "finding", "scored", "confirmed", "2026-01-01"), "", "duplicate id"},
		{"exit absent from corpus", "", "needs-decision", `exit "needs-decision" has no fixture`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			extra := map[string]string{}
			if c.body != "" {
				extra["syn-x.yaml"] = c.body
			}
			dir := writeGold(t, extra, c.skip)
			_, err := loadGold(dir, "")
			var le *LintError
			if !errors.As(err, &le) {
				t.Fatalf("err = %v, want a LintError", err)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("lint error %q does not mention %q", err, c.want)
			}
			pred := filepath.Join(t.TempDir(), "pred.json")
			if err := os.WriteFile(pred, []byte("{}"), 0o644); err != nil {
				t.Fatal(err)
			}
			if rc, _, _ := runCLI(t, "--gold", dir, "--pred", pred); rc != exitInput {
				t.Errorf("rc=%d, want %d for an invalid gold set", rc, exitInput)
			}
		})
	}
}

func TestStaleFixturesFlagged(t *testing.T) {
	dir := writeGold(t, map[string]string{
		"syn-old.yaml": fixtureYAML("syn-old", "finding", "scored", "confirmed", "2025-06-01"),
	}, "")
	gs, err := loadGold(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if gs.CurrentVersion != "2026-01-01" || len(gs.Stale) != 1 || gs.Stale[0] != "syn-old" {
		t.Fatalf("current=%s stale=%v, want 2026-01-01 and [syn-old]", gs.CurrentVersion, gs.Stale)
	}
	gs, err = loadGold(dir, "2026-02-01")
	if err != nil {
		t.Fatal(err)
	}
	if len(gs.Stale) != len(gs.Fixtures) {
		t.Fatalf("with a newer current version every fixture is stale; got %v", gs.Stale)
	}
}

// An exit with no scored fixtures has undefined recall: it is unmeasured,
// never 0 and never a pass, and its precision is 0 once anything is misrouted
// into it.
//
// regression: #2980
func TestUnmeasuredExitAndPolicy(t *testing.T) {
	dir := writeGold(t, map[string]string{
		"syn-ex.yaml": fixtureYAML("syn-ex", "bug/issue", "excluded", "pending-human", "2026-01-01"),
	}, "bug/issue")
	predPath := filepath.Join(t.TempDir(), "pred.json")
	// syn-t00 is spec/brief; misroute it into the unscored bug/issue exit.
	pred := `{"syn-t00":"bug/issue","syn-t02":"finding","syn-t03":"needs-decision","syn-t04":"rejected/watching","syn-ex":"bug/issue"}`
	if err := os.WriteFile(predPath, []byte(pred), 0o644); err != nil {
		t.Fatal(err)
	}
	_, r := runJSON(t, "--gold", dir, "--pred", predPath)
	bi := r.Exits["bug/issue"]
	if bi.Recall != nil || bi.F1 != nil {
		t.Errorf("bug/issue recall/F1 = %v/%v, want unmeasured", bi.Recall, bi.F1)
	}
	if got := val(t, bi.Precision, "bug/issue precision"); got != 0 {
		t.Errorf("bug/issue precision = %v, want 0 (a misroute into an unscored exit)", got)
	}
	if len(r.IgnoredExcluded) != 1 || r.IgnoredExcluded[0] != "syn-ex" {
		t.Errorf("ignored_excluded = %v, want [syn-ex]", r.IgnoredExcluded)
	}

	write := func(body string) string {
		p := filepath.Join(t.TempDir(), "thr.json")
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	// Finding passes; only the unmeasured bug/issue F1 floor decides.
	failDefault := write(`{"exits":{"bug/issue":{"f1":0.5},"finding":{"f1":0.5}}}`)
	if rc, out, _ := runCLI(t, "--gold", dir, "--pred", predPath, "--thresholds", failDefault); rc != exitFail {
		t.Errorf("default unmeasured policy: rc=%d want %d\n%s", rc, exitFail, out)
	}
	report := write(`{"unmeasured":"report","exits":{"bug/issue":{"f1":0.5},"finding":{"f1":0.5}}}`)
	if rc, out, _ := runCLI(t, "--gold", dir, "--pred", predPath, "--thresholds", report); rc != exitOK {
		t.Errorf("unmeasured=report: rc=%d want %d\n%s", rc, exitOK, out)
	}
}

func TestPredictionInputErrors(t *testing.T) {
	write := func(body string) string {
		p := filepath.Join(t.TempDir(), "pred.json")
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	for name, body := range map[string]string{
		"unknown fixture id": `{"syn-p01":"spec/brief","syn-nope":"finding"}`,
		"invalid exit label": `{"syn-p01":"spec"}`,
		"not a mapping":      `["spec/brief"]`,
	} {
		t.Run(name, func(t *testing.T) {
			if rc, _, _ := runCLI(t, "--gold", "testdata/gold-perfect", "--pred", write(body)); rc != exitInput {
				t.Errorf("rc=%d want %d", rc, exitInput)
			}
		})
	}
	// A scored fixture with no prediction is a miss on its expected exit.
	_, r := runJSON(t, "--gold", "testdata/gold-perfect", "--pred", write(`{"syn-p02":"spec/brief"}`))
	if len(r.Missing) != 9 {
		t.Fatalf("missing = %v, want 9 ids", r.Missing)
	}
	if got := val(t, r.Exits["spec/brief"].Recall, "spec/brief recall"); !near(got, 0.5) {
		t.Errorf("spec/brief recall = %v, want 0.5", got)
	}
}

// regression: #2980
func TestRegressionBoundAndBaselineReport(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	// Delta without any baseline would silently drop the regression check.
	noBase := write("nobase.json", `{"regression_delta":0.05,"exits":{}}`)
	if rc, _, errs := runCLI(t, "--gold", "testdata/gold-perfect", "--pred", "testdata/pred-perfect", "--thresholds", noBase); rc != exitInput || !strings.Contains(errs, "no baseline") {
		t.Errorf("rc=%d stderr=%q, want input error naming the missing baseline", rc, errs)
	}
	// A previous --json report is accepted as the baseline.
	_, base, _ := runCLI(t, "--gold", "testdata/gold-perfect", "--pred", "testdata/pred-perfect", "--json")
	basePath := write("base.json", base)
	if rc, out, errs := runCLI(t, "--gold", "testdata/gold-perfect", "--pred", "testdata/pred-regressed", "--thresholds", noBase, "--baseline", basePath); rc != exitFail {
		t.Errorf("regressed vs perfect baseline: rc=%d want %d\n%s%s", rc, exitFail, out, errs)
	}
	if rc, _, _ := runCLI(t, "--gold", "testdata/gold-perfect", "--pred", "testdata/pred-perfect", "--thresholds", noBase, "--baseline", basePath); rc != exitOK {
		t.Errorf("unchanged vs baseline: rc=%d want %d", rc, exitOK)
	}
	// Within delta passes: drop of 0.2 under a 0.25 bound.
	wide := write("wide.json", `{"regression_delta":0.25,"baseline":{"bug/issue":1.0},"exits":{}}`)
	if rc, _, _ := runCLI(t, "--gold", "testdata/gold-perfect", "--pred", "testdata/pred-regressed", "--thresholds", wide); rc != exitOK {
		t.Errorf("bug/issue F1 0.8 within 0.25 of 1.0: rc=%d want %d", rc, exitOK)
	}
}

func TestThresholdConfigRejectsTyposAndRanges(t *testing.T) {
	for name, body := range map[string]string{
		"unknown key":       `{"exits":{"finding":{"f1_floor":0.7}}}`,
		"unknown exit":      `{"exits":{"findings":{"f1":0.7}}}`,
		"floor above one":   `{"exits":{"finding":{"precision":1.5}}}`,
		"bad unmeasured":    `{"unmeasured":"skip","exits":{}}`,
		"negative delta":    `{"regression_delta":-0.1,"baseline":{"finding":1},"exits":{}}`,
		"baseline no delta": `{"baseline":{"finding":1},"exits":{}}`,
		"trailing value":    `{"exits":{}} {"exits":{"finding":{"f1":2}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "thr.json")
			if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			if rc, _, _ := runCLI(t, "--gold", "testdata/gold-perfect", "--pred", "testdata/pred-perfect", "--thresholds", p); rc != exitInput {
				t.Errorf("rc=%d want %d", rc, exitInput)
			}
		})
	}
}

// A value exactly equal to its floor passes ("at or above the floor"), including
// float ratios such as 2/3, and a value just under the floor fails.
func TestFloorEqualityPasses(t *testing.T) {
	two3 := 2.0 / 3.0
	one, zero := 1.0, 0.0
	cases := []struct {
		name         string
		value, floor *float64
		want         string
	}{
		{"two-thirds at two-thirds", &two3, &two3, "PASS"},
		{"one at one", &one, &one, "PASS"},
		{"zero at zero", &zero, &zero, "PASS"},
		{"just under", &two3, func() *float64 { f := two3 + 1e-6; return &f }(), "FAIL"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := &Score{Exits: map[string]*ExitScore{"finding": {Precision: c.value, F1: c.value}}}
			for _, th := range []*Thresholds{
				{Unmeasured: unmeasuredFail, Exits: map[string]Floors{"finding": {Precision: c.floor}}},
				{Unmeasured: unmeasuredFail, Exits: map[string]Floors{"finding": {F1: c.floor}}},
			} {
				v, err := evaluate(th, s)
				if err != nil {
					t.Fatal(err)
				}
				if v.Result != c.want {
					t.Errorf("result=%s want %s", v.Result, c.want)
				}
			}
		})
	}
}

// The leak boundary. This public tool's own fixtures are invented;
// every one must say so and use the synthetic id namespace, so a real corpus
// file copied into testdata/ fails here.
func TestTestdataIsSynthetic(t *testing.T) {
	files, err := filepath.Glob("testdata/*/*.yaml")
	if err != nil || len(files) == 0 {
		t.Fatalf("no testdata fixtures found: %v", err)
	}
	// The loader also accepts .yml, so the boundary covers it too.
	more, err := filepath.Glob("testdata/*/*.yml")
	if err != nil {
		t.Fatal(err)
	}
	files = append(files, more...)
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(raw), "# SYNTHETIC test fixture") {
			t.Errorf("%s lacks the SYNTHETIC header", f)
		}
		if !strings.HasPrefix(strings.TrimSuffix(strings.TrimSuffix(filepath.Base(f), ".yaml"), ".yml"), "syn-") {
			t.Errorf("%s is outside the syn- id namespace", f)
		}
	}
}

func TestUsageErrors(t *testing.T) {
	if rc, _, _ := runCLI(t); rc != exitInput {
		t.Errorf("no flags: rc=%d want %d", rc, exitInput)
	}
	if rc, _, _ := runCLI(t, "--gold", "testdata/gold-perfect", "--pred", "testdata/pred-perfect", "--baseline", "x.json"); rc != exitInput {
		t.Errorf("--baseline without --thresholds: rc=%d want %d", rc, exitInput)
	}
	if rc, _, _ := runCLI(t, "--gold", "testdata/no-such-dir", "--pred", "testdata/pred-perfect"); rc != exitInput {
		t.Errorf("missing gold dir: rc=%d want %d", rc, exitInput)
	}
}

func TestTableOutputNamesUnmeasuredAndVerdict(t *testing.T) {
	_, out, _ := runCLI(t, "--gold", "testdata/gold-imbalanced", "--pred", "testdata/pred-collapse")
	for _, want := range []string{"needs-decision", "unmeasured", "verdict: none", "informational only"} {
		if !strings.Contains(out, want) {
			t.Errorf("table output lacks %q:\n%s", want, out)
		}
	}
	_, out, _ = runCLI(t, "--gold", "testdata/gold-perfect", "--pred", "testdata/pred-regressed", "--thresholds", "testdata/thresholds-advisory.json")
	if !strings.Contains(out, "verdict: FAIL (advisory") {
		t.Errorf("advisory table output lacks the advisory FAIL label:\n%s", out)
	}
}
