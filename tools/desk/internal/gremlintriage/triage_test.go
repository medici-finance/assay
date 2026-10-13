package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// fixtureModule is a tiny module tree: two specs (one beside the package, one in
// another directory guarding the same package), a testdata spec that is not
// JSON and must be skipped, and the Go files the specs and mutants point into.
// It is written to a temp dir rather than committed because the repo's own spec
// guards walk every mutations*.json in the tree.
var fixtureModule = map[string]string{
	"other/mutations.json": `{
  "test": "go test ./other/ ./pkg/a/ -count=1",
  "control": {"name": "method-ok", "file": "./pkg/a/a.go", "old": "if !ok {", "new": "if ok {"},
  "mutations": []
}
`,
	"pkg/a/a.go": `package a

func Guard(n int) bool {
	if n > 10 {
		return true
	}
	return false
}

func Other(n int) int {
	if n < 0 {
		return -n
	}
	if n == 0 {
		return 1
	}
	return n
}

type T struct{}

func (t *T) Method(ok bool) bool {
	if !ok {
		return false
	}
	return true
}

func Free(n int) bool {
	return n != 3
}

var pkgLevel = len("x") > 0
`,
	"pkg/a/b.go": `package a

func Unspecced(n int) bool {
	return n >= 7
}
`,
	"pkg/a/mutations.json": `{
  "test": "go test ./pkg/a/ -count=1",
  "control": {"name": "guard-boundary", "file": "pkg/a/a.go", "old": "if n > 10 {", "new": "if n >= 10 {"},
  "mutations": [
    {"name": "other-negative", "file": "pkg/a/a.go", "old": "if n < 0 {\n\t\treturn -n", "new": "if n > 0 {\n\t\treturn -n"},
    {"name": "gone", "file": "pkg/a/a.go", "old": "this text is not in the file", "new": "x"}
  ]
}
`,
	"pkg/a/testdata/mutations.json": `this is not json
`,
}

func fixtureDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range fixtureModule {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func loadFixtureSpecs(t *testing.T) *specIndex {
	t.Helper()
	root := fixtureDir(t)
	idx, err := loadSpecs(root, newSourceCache(root))
	if err != nil {
		t.Fatalf("loadSpecs: %v", err)
	}
	return idx
}

// regression: #2647
func TestSpecsReadEveryPackage(t *testing.T) {
	idx := loadFixtureSpecs(t)
	// Two specs: pkg/a's own and other/'s, which guards pkg/a code. The
	// testdata/mutations.json under pkg/a is not JSON and must be skipped.
	if got := strings.Join(idx.Specs, ","); got != "other/mutations.json,pkg/a/mutations.json" {
		t.Fatalf("specs = %q", got)
	}
	if len(idx.Stale) != 1 || idx.Stale[0].Entry != "gone" || idx.Stale[0].Reason != "old text not found" {
		t.Fatalf("stale = %+v, want the one entry whose old text is absent", idx.Stale)
	}
}

// regression: #2647
func TestClassifyLived(t *testing.T) {
	idx := loadFixtureSpecs(t)
	cases := []struct {
		file    string
		line    int
		class   string
		fn      string
		specced bool
	}{
		{"pkg/a/a.go", 4, classSpecLine, "Guard", true},       // control entry's own line
		{"pkg/a/a.go", 12, classSpecLine, "Other", true},      // second line of a two-line old
		{"pkg/a/a.go", 14, classSpecFunc, "Other", true},      // same function, other line
		{"pkg/a/a.go", 23, classSpecLine, "T.Method", true},   // guarded by a spec in another dir
		{"pkg/a/a.go", 30, classCandidate, "Free", true},      // no spec in this function
		{"pkg/a/a.go", 33, classCandidate, "", true},          // package scope
		{"pkg/a/b.go", 4, classCandidate, "Unspecced", false}, // no spec touches the file
	}
	for _, c := range cases {
		class, fn, specced := idx.classify(c.file, c.line)
		if class != c.class || fn != c.fn || specced != c.specced {
			t.Errorf("%s:%d = (%s, %q, %v), want (%s, %q, %v)", c.file, c.line, class, fn, specced, c.class, c.fn, c.specced)
		}
	}
}

// regression: #2647
func TestReadLogStripsColour(t *testing.T) {
	f, err := os.Open("testdata/partial.log")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ms, err := readLog(f)
	if err != nil {
		t.Fatal(err)
	}
	c := tally(ms)
	if c != (counts{Killed: 1, Lived: 1, NotCovered: 1, TimedOut: 1}) {
		t.Fatalf("counts = %+v", c)
	}
	if ms[1].File != "a.go" || ms[1].Line != 30 || ms[1].Column != 11 {
		t.Fatalf("lived mutant = %+v", ms[1])
	}
	// The LIVED and KILLED lines after "Shutting down gracefully" are interrupted
	// mutants: counting them would move the totals above.
	for _, m := range ms {
		if m.Line == 40 || m.Line == 44 {
			t.Fatalf("mutant after the shutdown line counted: %+v", m)
		}
	}
}

// regression: #2647
func TestReportRefusesBadShape(t *testing.T) {
	for _, p := range []string{"testdata/report-badstatus.json", "testdata/report-nomodule.json", "testdata/missing.json"} {
		if ms, err := readReport(p); err == nil {
			t.Errorf("%s: read %d mutants, want an error", p, len(ms))
		}
	}
}

func row(state, report, log string) manifestRow {
	return manifestRow{Pkg: "pkg/a", State: state, Wall: 3, Report: report, Log: log, Bin: "1/1"}
}

// regression: #2647
func TestTriageCountsAndCandidates(t *testing.T) {
	res := triage([]manifestRow{row(stateOK, "testdata/report-ok.json", "")}, loadFixtureSpecs(t))
	if res.Broke {
		t.Fatalf("survivors must never break the sweep: %+v", res.Packages[0])
	}
	p := res.Packages[0]
	want := counts{Killed: 1, Lived: 6, NotCovered: 1, TimedOut: 1, NotViable: 1}
	if p.Counts != want {
		t.Fatalf("counts = %+v, want %+v", p.Counts, want)
	}
	if p.SpecLine != 2 || p.SpecFunc != 1 {
		t.Fatalf("spec-line %d spec-func %d, want 2 and 1", p.SpecLine, p.SpecFunc)
	}
	var got []string
	for _, c := range p.Candidates {
		got = append(got, c.File+":"+itoa(c.Line)+" "+c.Func)
	}
	if strings.Join(got, ";") != "pkg/a/a.go:30 Free;pkg/a/a.go:33 ;pkg/a/b.go:4 Unspecced" {
		t.Fatalf("candidates = %q", got)
	}
	if p.Candidates[0].Source != "return n != 3" {
		t.Fatalf("source = %q", p.Candidates[0].Source)
	}
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }

// regression: #2647
func TestTriageBrokeStates(t *testing.T) {
	idx := loadFixtureSpecs(t)
	good := row(stateOK, "testdata/report-ok.json", "")
	cases := map[string]manifestRow{
		"gremlins failed":        row(stateFailed, "", ""),
		"probe red":              row(stateProbeRed, "", ""),
		"dry run failed":         row(stateDryRunFail, "", ""),
		"report missing":         row(stateOK, "testdata/missing.json", ""),
		"all not viable":         row(stateOK, "testdata/report-notviable.json", ""),
		"none covered":           row(stateOK, "testdata/report-notcovered.json", ""),
		"partial, none finished": row(statePartial, "", "testdata/partial-nofinish.log"),
		"empty report":           row(stateOK, "testdata/report-empty.json", ""),
		"partial log gone":       row(statePartial, "", "testdata/missing.log"),
		"unknown status":         row(stateOK, "testdata/report-badstatus.json", ""),
		"not a gremlins report":  row(stateOK, "testdata/report-nomodule.json", ""),
	}
	for name, bad := range cases {
		res := triage([]manifestRow{good, bad}, idx)
		if !res.Broke || res.Packages[1].Broke == "" {
			t.Errorf("%s: broke=%v pkg=%+v, want broke", name, res.Broke, res.Packages[1])
		}
	}
}

// regression: #2647
func TestTriagePartialIsEvidence(t *testing.T) {
	res := triage([]manifestRow{row(statePartial, "", "testdata/partial.log")}, loadFixtureSpecs(t))
	if res.Broke {
		t.Fatalf("a budget-stopped run is partial evidence, not a broken sweep: %+v", res.Packages[0])
	}
	if len(res.Packages[0].Candidates) != 1 || res.Packages[0].Note == "" {
		t.Fatalf("partial = %+v", res.Packages[0])
	}
}

// regression: #2647
func TestTriageNoEvidenceBreaks(t *testing.T) {
	empty := filepath.Join(t.TempDir(), "empty.log")
	if err := os.WriteFile(empty, []byte("Gathering coverage...\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := triage([]manifestRow{row(statePartial, "", empty)}, loadFixtureSpecs(t))
	if !res.Broke {
		t.Fatal("no package produced a single finished mutant; the sweep must report broke")
	}
	// No package at all: no per-package check fires, so only the sweep-wide
	// evidence check stands between this and a clean exit.
	if res := triage(nil, loadFixtureSpecs(t)); !res.Broke {
		t.Fatal("an empty manifest is no evidence; the sweep must report broke")
	}
}

// regression: #2647
func TestManifestRefusesBadRows(t *testing.T) {
	for _, in := range []string{
		"",
		"# only a comment\n",
		"pkg/a\tok\t3\t0\tr.json\tl.log\n",
		"pkg/a\tweird\t3\t0\tr.json\tl.log\t1/1\n",
		"pkg/a\tok\tx\t0\tr.json\tl.log\t1/1\n",
	} {
		if rows, err := readManifest(strings.NewReader(in)); err == nil {
			t.Errorf("%q: read %d rows, want an error", in, len(rows))
		}
	}
}

// regression: #2647
func TestRunTriageExitCodes(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	ok := write("ok.tsv", "pkg/a\tok\t3\t0\ttestdata/report-ok.json\tx.log\t1/1\n")
	md, js := filepath.Join(dir, "s.md"), filepath.Join(dir, "s.json")
	root := fixtureDir(t)
	var out, errb bytes.Buffer
	if code := run([]string{"triage", "-manifest", ok, "-module-root", root, "-md", md, "-json", js}, &out, &errb); code != 0 {
		t.Fatalf("survivors: exit %d, want 0; %s", code, errb.String())
	}
	sum, _ := os.ReadFile(md)
	if !strings.Contains(string(sum), "`pkg/a/b.go:4:11` `Unspecced`") || !strings.Contains(string(sum), "no muhar spec touches this file") {
		t.Fatalf("summary lacks the candidate row:\n%s", sum)
	}
	var res triageResult
	if b, _ := os.ReadFile(js); json.Unmarshal(b, &res) != nil || res.Schema != "gremlins-triage/v1" {
		t.Fatalf("json result unreadable or wrong schema")
	}
	bad := write("bad.tsv", "pkg/a\tfailed\t3\t1\t-\tx.log\t1/1\n")
	if code := run([]string{"triage", "-manifest", bad, "-module-root", root, "-md", md}, &out, &errb); code != exitBroke {
		t.Fatalf("broken sweep: exit %d, want %d", code, exitBroke)
	}
}

// regression: #2647
func TestPlanRotatesAllFiles(t *testing.T) {
	files := []planFile{{"c.go", 3}, {"a.go", 1}, {"sub/b.go", 2}, {"d.go", 0}, {"big.go", 9}}
	seen := map[string]int{}
	var bins int
	for day := 0; day < 20; day++ {
		p, err := makePlan(files, 3, day)
		if err != nil {
			t.Fatal(err)
		}
		bins = p.Bins
		if day >= p.Bins {
			continue
		}
		if p.Runnable > p.Capacity && len(p.Files) > 1 {
			t.Errorf("day %d: bin over capacity with several files: %+v", day, p)
		}
		re := regexp.MustCompile(p.Exclude)
		for _, f := range files {
			in := contains(p.Files, f.Name)
			if in {
				seen[f.Name]++
			}
			if in == re.MatchString(f.Name) {
				t.Errorf("day %d: %s in bin=%v but excluded=%v", day, f.Name, in, re.MatchString(f.Name))
			}
		}
	}
	if bins != 4 {
		t.Fatalf("bins = %d, want 4 (a | big | c+d | sub/b)", bins)
	}
	for _, f := range files {
		if seen[f.Name] != 1 {
			t.Errorf("%s visited %d times over one rotation, want 1", f.Name, seen[f.Name])
		}
	}
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// regression: #2647
func TestPlanOneBinNoExclude(t *testing.T) {
	var out, errb bytes.Buffer
	code := run([]string{"plan", "-dry-run", "testdata/dryrun.json", "-budget", "600", "-coverage", "2", "-workers", "2", "-day", "5"}, &out, &errb)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if out.String() != "bin 1/1 files 3 runnable 6 capacity 598\n\n" {
		t.Fatalf("plan output = %q", out.String())
	}
}

// regression: #2647
func TestPlanRefusesComma(t *testing.T) {
	if _, err := makePlan([]planFile{{"a,b.go", 1}, {"c.go", 1}}, 1, 0); err == nil {
		t.Fatal("a comma in a file name would split Gremlins' exclusion flag; want an error")
	}
}

// regression: #2647
func TestCapacityFor(t *testing.T) {
	cases := []struct {
		budget, cov float64
		workers     int
		want        int
	}{
		{600, 2, 2, 598},    // small package: budget is not the limit
		{600, 0.1, 2, 1199}, // cached or tiny coverage is floored at one second per mutant
		{600, 160, 2, 5},    // a slow suite fits only a few mutants
		{100, 160, 2, 1},    // coverage alone overruns: still try one
		{600, 2, 0, 1},
	}
	for _, c := range cases {
		if got := capacityFor(c.budget, c.cov, c.workers); got != c.want {
			t.Errorf("capacityFor(%v, %v, %d) = %d, want %d", c.budget, c.cov, c.workers, got, c.want)
		}
	}
}

// regression: #2647
func TestCoverageFromLog(t *testing.T) {
	dir := t.TempDir()
	for body, want := range map[string]float64{
		"Gathering coverage... done in 2.5s\n":          2.5,
		"x\nGathering coverage... done in 1m4s\nKILLED": 64,
	} {
		p := filepath.Join(dir, "l")
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if got, err := coverageFromLog(p); err != nil || got != want {
			t.Errorf("%q: got %v, %v; want %v", body, got, err, want)
		}
	}
	p := filepath.Join(dir, "none")
	if err := os.WriteFile(p, []byte("no timing here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := coverageFromLog(p); err == nil {
		t.Error("a log without the timing line must be an error, not a zero-second suite")
	}
}
