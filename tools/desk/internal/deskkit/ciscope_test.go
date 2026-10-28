package deskkit

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// ci.yml's `scope` job lets one class of pull request (a verifier-App PR whose
// every changed file is under docs/) skip the jobs that gate on its `heavy`
// output. The CI-trigger registry guard above cannot see that skip: ciJobEventGate
// treats an `if:` that does not name github.event_name as permissive. Review
// finding cr1-docs-skip-untested was exactly that gap. Every build-test-shard leg
// was gated, so the required build-test passed on those PRs without running the
// floor tests that read docs/.
//
// The guards below close that class:
//
//   - TestCIScopeSkipSparesDocsReaders fails when a job the scope decision can
//     skip carries a registry row whose reads overlap the skippable set, or
//     names a path in that set in its own text.
//   - TestCIMatrixLegsRunTheirRows fails when a registry row pinned to one
//     matrix leg (build-test-shard's `rest` or `desk-1`) points at a leg whose
//     step-level `if: matrix.X == / != '...'` conditions exclude the command the
//     row relies on. The registry guard does not model step-level `if:`, so
//     without this check the `rest` pin would also be satisfied by the desk-shard
//     step's text.

const ciWorkflow = ".github/workflows/ci.yml"

func TestCIScopeSkipSparesDocsReaders(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("../../../..", ciWorkflow))
	if err != nil {
		t.Fatal(err)
	}
	var skipped []string
	for id, skip := range ciScopeSkippableJobs(t, string(raw), ciWorkflow) {
		if skip {
			skipped = append(skipped, id)
		}
	}
	sort.Strings(skipped)
	t.Logf("skippable prefixes %v; jobs the scope decision can skip: %v",
		ciScopeSkippablePrefixes(t, string(raw), ciWorkflow), skipped)
	faults := ciScopeSkipFaults(t, string(raw), ciWorkflow, ciCrossModuleRegistry())
	for _, f := range faults {
		t.Error(f)
	}
}

// TestCIScopeSkipGuardFires is the guard's positive control on fixtures: a
// docs-reading row on a scope-gated job, a job that inherits the skip through
// `needs:`, and a job whose own text reads docs/ must each be reported; the
// healthy shape must not.
func TestCIScopeSkipGuardFires(t *testing.T) {
	const head = `on:
  pull_request:
jobs:
  scope:
    runs-on: ubuntu-latest
    outputs:
      heavy: ${{ steps.decide.outputs.heavy }}
    steps:
      - id: decide
        run: |
          outside="$(printf '%s\n' "$files" | grep -v -E '^docs/' || true)"
`
	rows := []ciEntry{{
		test: "x/reader_test.go", module: "x", workflow: "fixture.yml",
		prJob: ciJobRef{id: "floor"}, pushJob: ciJobRef{id: "floor"},
		reads: []string{"docs/streams"},
	}}
	cases := []struct {
		name, jobs, want string
	}{
		{"healthy", `  floor:
    runs-on: x
    steps:
      - run: cd x && go test ./...
  lint:
    needs: scope
    if: ${{ !cancelled() && needs.scope.outputs.heavy != 'false' }}
    runs-on: x
    steps:
      - run: go run ./tools/lint
`, ""},
		{"gated floor", `  floor:
    needs: scope
    if: ${{ !cancelled() && needs.scope.outputs.heavy != 'false' }}
    runs-on: x
    steps:
      - run: cd x && go test ./...
`, "x/reader_test.go reads docs/streams"},
		{"skip inherited through needs", `  prep:
    needs: scope
    if: ${{ needs.scope.outputs.heavy != 'false' }}
    runs-on: x
    steps:
      - run: true
  floor:
    needs: [prep]
    runs-on: x
    steps:
      - run: cd x && go test ./...
`, "x/reader_test.go reads docs/streams"},
		{"job text reads docs", `  floor:
    runs-on: x
    steps:
      - run: cd x && go test ./...
  lint:
    needs: scope
    if: ${{ needs.scope.outputs.heavy != 'false' }}
    runs-on: x
    steps:
      - run: go run . --vocab ../../docs/streams/README.md
`, "jobs.lint names ../../docs/"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := strings.Join(ciScopeSkipFaults(t, head+c.jobs, "fixture.yml", rows), "\n")
			if c.want == "" && got != "" {
				t.Fatalf("healthy shape reported: %s", got)
			}
			if c.want != "" && !strings.Contains(got, c.want) {
				t.Fatalf("want a fault containing %q, got %q", c.want, got)
			}
		})
	}
}

// ciScopeSkipFaults returns one line per job the scope decision can skip that
// reads the skippable set: through a registry row's reads, or a path named in the
// job's own text.
func ciScopeSkipFaults(t *testing.T, content, wfName string, registry []ciEntry) []string {
	t.Helper()
	prefixes := ciScopeSkippablePrefixes(t, content, wfName)
	if len(prefixes) == 0 {
		return nil
	}
	skippable := ciScopeSkippableJobs(t, content, wfName)
	var faults []string
	for _, e := range registry {
		if e.workflow != wfName {
			continue
		}
		// The scope decision only ever skips on pull_request.
		if !skippable[e.prJob.id] {
			continue
		}
		for _, read := range e.reads {
			if p, ok := ciReadOverlaps(read, prefixes); ok {
				faults = append(faults, fmt.Sprintf("%s: %s reads %s, which overlaps %s, but jobs.%s (%s) can be skipped by "+
					"the scope job's docs-only decision, so a change to what it reads would pass without running it. "+
					"Do not gate jobs.%s on scope (cr1-docs-skip-untested)", wfName, e.test, read, p, e.prJob.id, e.prJob.check, e.prJob.id))
			}
		}
	}
	ids := make([]string, 0, len(skippable))
	for id, skip := range skippable {
		if skip {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		lines, _ := ciWorkflowJobLines(t, content, wfName, id)
		for _, ln := range lines {
			if strings.HasPrefix(strings.TrimSpace(ln), "#") {
				continue
			}
			for _, p := range prefixes {
				if m := regexp.MustCompile(`(^|[\s'"=(:])((\.\./)*` + regexp.QuoteMeta(p) + `)`).FindStringSubmatch(ln); m != nil {
					faults = append(faults, fmt.Sprintf("%s: jobs.%s names %s in its own text (%q) but can be skipped by the scope "+
						"job's docs-only decision; do not gate it on scope", wfName, id, m[2], strings.TrimSpace(ln)))
				}
			}
		}
	}
	return faults
}

var ciScopeGrepRe = regexp.MustCompile(`grep -v -E '([^']*)'`)
var ciScopePrefixPatRe = regexp.MustCompile(`^\^\(?([A-Za-z0-9_.-]+(?:\|[A-Za-z0-9_.-]+)*)\)?/$`)

// ciScopeSkippablePrefixes reads the path prefixes the scope job treats as
// skippable, from its `grep -v -E '^<dir>/'` filter. No scope job means nothing
// can be skipped; a scope job whose filter this parser cannot read Fatals.
func ciScopeSkippablePrefixes(t *testing.T, content, wfName string) []string {
	t.Helper()
	lines, ok := ciWorkflowJobLines(t, content, wfName, "scope")
	if !ok {
		return nil
	}
	m := ciScopeGrepRe.FindAllStringSubmatch(strings.Join(lines, "\n"), -1)
	if len(m) != 1 {
		t.Fatalf("%s: jobs.scope has %d `grep -v -E '...'` filters, want exactly 1: teach "+
			"ciScopeSkippablePrefixes the new shape rather than assuming nothing is skippable", wfName, len(m))
	}
	pm := ciScopePrefixPatRe.FindStringSubmatch(m[0][1])
	if pm == nil {
		t.Fatalf("%s: jobs.scope's skippable-path pattern %q is not `^<dir>/` or `^(<a>|<b>)/`: "+
			"teach ciScopeSkippablePrefixes the shape", wfName, m[0][1])
	}
	var out []string
	for _, d := range strings.Split(pm[1], "|") {
		out = append(out, d+"/")
	}
	return out
}

// ciReadOverlaps reports whether a registry read can match a file under one of
// the prefixes: the read is that directory, lies under it, or starts with a glob
// (`**/*.go` matches docs/x.go too).
func ciReadOverlaps(read string, prefixes []string) (string, bool) {
	first := strings.SplitN(read, "/", 2)[0]
	for _, p := range prefixes {
		if read == strings.TrimSuffix(p, "/") || strings.HasPrefix(read, p) || strings.ContainsAny(first, "*?[{") {
			return p, true
		}
	}
	return "", false
}

// ciScopeSkippableJobs marks every job the scope decision can skip: its own
// job-level `if:` reads needs.scope.outputs, or it needs such a job and its `if:`
// has no status function (a skipped need skips a dependent unless its `if:`
// calls always(), cancelled() or failure()).
func ciScopeSkippableJobs(t *testing.T, content, wfName string) map[string]bool {
	t.Helper()
	ids := ciWorkflowJobIDs(t, content, wfName)
	ifs := map[string]string{}
	needs := map[string][]string{}
	for _, id := range ids {
		lines, _ := ciWorkflowJobLines(t, content, wfName, id)
		ifs[id] = ciJobScalar(lines, "if")
		needs[id] = ciJobNeeds(t, lines, wfName, id)
	}
	skip := map[string]bool{}
	for changed := true; changed; {
		changed = false
		for _, id := range ids {
			if skip[id] {
				continue
			}
			s := strings.Contains(ifs[id], "needs.scope.outputs")
			if !s && !regexp.MustCompile(`\b(always|cancelled|failure)\(\)`).MatchString(ifs[id]) {
				for _, n := range needs[id] {
					s = s || skip[n]
				}
			}
			if s {
				skip[id], changed = true, true
			}
		}
	}
	return skip
}

// ciWorkflowJobIDs lists the jobs.<id> keys in file order.
func ciWorkflowJobIDs(t *testing.T, content, wfName string) []string {
	t.Helper()
	var ids []string
	in := false
	for _, ln := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(ln)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(ln) - len(strings.TrimLeft(ln, " "))
		if indent == 0 {
			in = strings.TrimRight(ln, " \t") == "jobs:"
			continue
		}
		if in && indent == 2 && strings.HasSuffix(trimmed, ":") {
			ids = append(ids, strings.TrimSuffix(trimmed, ":"))
		}
	}
	if len(ids) == 0 {
		t.Fatalf("%s: no jobs found", wfName)
	}
	return ids
}

// ciChildIndent is the indent of a job's (or step's) own keys: the smallest
// indent among its non-blank, non-comment lines.
func ciChildIndent(lines []string) int {
	child := -1
	for _, ln := range lines {
		trimmed := strings.TrimSpace(ln)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if indent := len(ln) - len(strings.TrimLeft(ln, " ")); child < 0 || indent < child {
			child = indent
		}
	}
	return child
}

// ciJobScalar returns the inline value of a job-level key ("" when absent).
func ciJobScalar(jobLines []string, key string) string {
	child := ciChildIndent(jobLines)
	for _, ln := range jobLines {
		if len(ln)-len(strings.TrimLeft(ln, " ")) != child {
			continue
		}
		if rest, ok := strings.CutPrefix(strings.TrimSpace(ln), key+":"); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

// ciJobNeeds parses `needs: x`, `needs: [x, y]` and a block list.
func ciJobNeeds(t *testing.T, jobLines []string, wfName, id string) []string {
	t.Helper()
	child := ciChildIndent(jobLines)
	for i, ln := range jobLines {
		if len(ln)-len(strings.TrimLeft(ln, " ")) != child {
			continue
		}
		rest, ok := strings.CutPrefix(strings.TrimSpace(ln), "needs:")
		if !ok {
			continue
		}
		rest = strings.TrimSpace(rest)
		switch {
		case strings.HasPrefix(rest, "["):
			return ciParseFlowSequence(t, wfName, "jobs."+id+".needs", rest)
		case rest != "":
			return []string{unquoteYAMLScalar(rest)}
		}
		var out []string
		for _, b := range jobLines[i+1:] {
			bt := strings.TrimSpace(b)
			if bt == "" || strings.HasPrefix(bt, "#") {
				continue
			}
			item, isItem := strings.CutPrefix(bt, "- ")
			if !isItem || len(b)-len(strings.TrimLeft(b, " ")) <= child {
				break
			}
			out = append(out, unquoteYAMLScalar(item))
		}
		return out
	}
	return nil
}

// TestCIMatrixLegsRunTheirRows checks every ci.yml registry row pinned to a
// matrix leg against the steps that leg actually runs, evaluating step-level
// `if: matrix.<key> == '<v>'` and `!= '<v>'`. A row whose command lives only in a
// step its leg skips fails.
func TestCIMatrixLegsRunTheirRows(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("../../../..", ciWorkflow))
	if err != nil {
		t.Fatal(err)
	}
	content := string(raw)
	checked := 0
	for _, e := range ciCrossModuleRegistry() {
		if e.workflow != ciWorkflow {
			continue
		}
		for _, job := range []ciJobRef{e.prJob, e.pushJob} {
			if job.matrixValue == "" {
				continue
			}
			checked++
			for _, f := range ciMatrixLegFaults(t, content, e, job) {
				t.Error(f)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no ci.yml registry row is pinned to a matrix leg; this guard checked nothing")
	}
}

// TestCIMatrixLegGuardFires is that guard's positive control: a row pinned to a
// leg whose only matching step is gated to another leg must be reported.
func TestCIMatrixLegGuardFires(t *testing.T) {
	wf := `jobs:
  suite:
    strategy:
      matrix:
        shard: [rest, desk-1]
    steps:
      - name: rest only
        if: matrix.shard == 'rest'
        run: cd statusgen && go test ./...
      - name: desk legs
        if: matrix.shard != 'rest'
        run: cd tools/desk && go test ./...
`
	row := func(module, leg string) ciEntry {
		j := ciJobRef{id: "suite", matrixValue: leg, check: "suite (" + leg + ")"}
		return ciEntry{test: module + "/x_test.go", module: module, workflow: "fixture.yml", prJob: j, pushJob: j}
	}
	if f := ciMatrixLegFaults(t, wf, row("statusgen", "rest"), row("statusgen", "rest").prJob); len(f) != 0 {
		t.Fatalf("healthy rest row reported: %v", f)
	}
	if f := ciMatrixLegFaults(t, wf, row("tools/desk", "desk-1"), row("tools/desk", "desk-1").prJob); len(f) != 0 {
		t.Fatalf("healthy desk row reported: %v", f)
	}
	if f := ciMatrixLegFaults(t, wf, row("tools/desk", "rest"), row("tools/desk", "rest").prJob); len(f) == 0 {
		t.Fatal("a row pinned to a leg that skips its step was not reported")
	}
	if f := ciMatrixLegFaults(t, wf, row("statusgen", "desk-1"), row("statusgen", "desk-1").prJob); len(f) == 0 {
		t.Fatal("a row pinned to a leg that skips its step was not reported")
	}
}

var ciStepMatrixIfRe = regexp.MustCompile(`^matrix\.([A-Za-z0-9_-]+)\s*(==|!=)\s*'([^']*)'$`)

// ciMatrixLegFaults returns faults when the steps leg job.matrixValue runs do not
// carry the row's command (runInvokes fragments, or `cd <module> && go test`).
func ciMatrixLegFaults(t *testing.T, content string, e ciEntry, job ciJobRef) []string {
	t.Helper()
	lines, ok := ciWorkflowJobLines(t, content, e.workflow, job.id)
	if !ok {
		return []string{fmt.Sprintf("%s: no jobs.%s", e.workflow, job.id)}
	}
	keys := ciMatrixKeys(lines)
	text := ciMatrixVarRe.ReplaceAllString(strings.Join(ciLegStepText(t, lines, e.workflow, job, keys), "\n"), job.matrixValue)
	var faults []string
	if len(e.runInvokes) > 0 {
		for _, frag := range e.runInvokes {
			if !strings.Contains(text, frag) {
				faults = append(faults, fmt.Sprintf("%s: %s is pinned to %s, but no step that leg runs contains %q "+
					"(step-level `if: matrix...` evaluated); re-point the row at the leg that runs it", e.workflow, e.test, job.check, frag))
			}
		}
		return faults
	}
	if !ciContains(ciGoTestDirs(text), e.module) {
		faults = append(faults, fmt.Sprintf("%s: %s is pinned to %s, but no step that leg runs does `cd %s && go test` "+
			"(step-level `if: matrix...` evaluated); re-point the row at the leg that runs it", e.workflow, e.test, job.check, e.module))
	}
	return faults
}

// ciMatrixKeys lists the keys directly under a job's `matrix:`.
func ciMatrixKeys(jobLines []string) []string {
	var keys []string
	mIndent := -1
	for _, ln := range jobLines {
		trimmed := strings.TrimSpace(ln)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(ln) - len(strings.TrimLeft(ln, " "))
		if mIndent < 0 {
			if trimmed == "matrix:" {
				mIndent = indent
			}
			continue
		}
		if indent <= mIndent {
			break
		}
		if i := strings.IndexByte(trimmed, ':'); i > 0 && !strings.HasPrefix(trimmed, "- ") && indent == mIndent+2 {
			keys = append(keys, trimmed[:i])
		}
	}
	return keys
}

// ciLegStepText returns the text of each step the given leg runs. A step `if:`
// that names a matrix key is evaluated for `==` / `!=` against one literal; any
// other matrix condition, or a matrix with more than one key, Fatals. A step
// `if:` that names no matrix key does not depend on the leg and is kept.
func ciLegStepText(t *testing.T, jobLines []string, wfName string, job ciJobRef, keys []string) []string {
	t.Helper()
	stepsAt := -1
	child := ciChildIndent(jobLines)
	for i, ln := range jobLines {
		if strings.TrimSpace(ln) == "steps:" && len(ln)-len(strings.TrimLeft(ln, " ")) == child {
			stepsAt = i
			break
		}
	}
	if stepsAt < 0 {
		t.Fatalf("%s: jobs.%s has no steps:", wfName, job.id)
	}
	var steps [][]string
	itemIndent := -1
	for _, ln := range jobLines[stepsAt+1:] {
		trimmed := strings.TrimSpace(ln)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(ln) - len(strings.TrimLeft(ln, " "))
		if indent <= child {
			break
		}
		if strings.HasPrefix(trimmed, "- ") && (itemIndent < 0 || indent == itemIndent) {
			itemIndent = indent
			steps = append(steps, []string{ln})
			continue
		}
		if len(steps) > 0 {
			steps[len(steps)-1] = append(steps[len(steps)-1], ln)
		}
	}
	var out []string
	for _, st := range steps {
		expr := ""
		for _, ln := range st {
			trimmed := strings.TrimPrefix(strings.TrimSpace(ln), "- ")
			if len(ln)-len(strings.TrimLeft(ln, " ")) > itemIndent+2 {
				continue // nested in a run: block or with: map
			}
			if rest, ok := strings.CutPrefix(trimmed, "if:"); ok {
				expr = strings.TrimSpace(rest)
				expr = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(expr, "${{"), "}}"))
			}
		}
		if !strings.Contains(expr, "matrix.") {
			out = append(out, strings.Join(st, "\n"))
			continue
		}
		m := ciStepMatrixIfRe.FindStringSubmatch(expr)
		if m == nil || len(keys) != 1 || m[1] != keys[0] {
			t.Fatalf("%s: jobs.%s has a step `if: %s` this guard does not model (matrix keys %v): teach "+
				"ciLegStepText the shape rather than assuming the step runs", wfName, job.id, expr, keys)
		}
		if (m[2] == "==") == (m[3] == job.matrixValue) {
			out = append(out, strings.Join(st, "\n"))
		}
	}
	return out
}
