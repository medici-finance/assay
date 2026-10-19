package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// autonomyGHFixture fakes gh for autonomyGates. list is the `pr list` output
// (or "failure"); rollup is the commit body every batched `api graphql` node
// carries — e.g. `"statusCheckRollup":{...}`, "" for a missing field — or one of
// "failure", "slow", "raw:<verbatim output>", "rawfail:<output, then exit 1>".
// AUTONOMY_PAGE is the verbatim answer to a follow-up context-page read (or
// "failure"). AUTONOMY_DELAY (seconds) is the per-call latency of the real
// forge. The `pr view` arm models gh's single-PR rollup read, the path the
// batched reader replaced (it must never be called).
func autonomyGHFixture(t *testing.T, list, rollup string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	script := `#!/bin/sh
printf '%s|%s\n' "$PWD" "$*" >> "$AUTONOMY_CALLS"
[ -n "$AUTONOMY_DELAY" ] && sleep "$AUTONOMY_DELAY"
case "$*" in
  *"pr list"*statusCheckRollup*) exit 42 ;;
  *"pr list"*) [ "$AUTONOMY_LIST" = failure ] && exit 45; printf '%s' "$AUTONOMY_LIST" ;;
  *"pr view"*) printf '%s' '{"statusCheckRollup":[{"name":"go-test"}]}' ;;
  *"api graphql"*'after:$after'*)
    [ "$AUTONOMY_PAGE" = failure ] && exit 46
    printf '%s' "$AUTONOMY_PAGE" ;;
  *"api graphql"*)
    case "$AUTONOMY_ROLLUP" in
      failure) exit 44 ;;
      slow) exec sleep 5 ;;
      raw:*) printf '%s' "${AUTONOMY_ROLLUP#raw:}"; exit 0 ;;
      rawfail:*) printf '%s' "${AUTONOMY_ROLLUP#rawfail:}"; exit 1 ;;
    esac
    sep=''
    printf '{"data":{"repository":{'
    for n in $(printf '%s' "$*" | grep -o 'pr[0-9][0-9]*: pullRequest' | tr -dc '0-9\n'); do
      printf '%s"pr%s":{"commits":{"nodes":[{"commit":{%s}}]}}' "$sep" "$n" "$AUTONOMY_ROLLUP"
      sep=','
    done
    printf '}}}' ;;
  *) exit 43 ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("AUTONOMY_CALLS", log)
	t.Setenv("AUTONOMY_LIST", list)
	t.Setenv("AUTONOMY_ROLLUP", rollup)
	t.Setenv("AUTONOMY_DELAY", "")
	t.Setenv("AUTONOMY_PAGE", "")
	return dir, log
}

const goTestLintRollup = `"statusCheckRollup":{"contexts":{"pageInfo":{"hasNextPage":false,"endCursor":null},"nodes":[{"__typename":"CheckRun","name":"go-test"},{"__typename":"StatusContext","context":"lint"}]}}`

// gatePage is a follow-up context-page answer for one PR's head commit.
func gatePage(pageInfo, nodes string) string {
	return `{"data":{"repository":{"pullRequest":{"commits":{"nodes":[{"commit":{"statusCheckRollup":{"contexts":{"pageInfo":` +
		pageInfo + `,"nodes":` + nodes + `}}}}]}}}}}`
}

// overflowRollup is a first page that holds no gate name and says more follow.
const overflowRollup = `"statusCheckRollup":{"contexts":{"pageInfo":{"hasNextPage":true,"endCursor":"c1"},"nodes":[{"__typename":"CheckRun","name":"changelog"}]}}`

func gateCalls(t *testing.T, log string) []string {
	t.Helper()
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(calls)), "\n")
}

func TestAutonomyGatesWindowedRollups(t *testing.T) {
	root, log := autonomyGHFixture(t,
		`[{"number":11,"mergedAt":"2026-07-03T12:00:00Z"},{"number":12,"mergedAt":"2026-06-30T23:59:59Z"},`+
			`{"number":13,"mergedAt":"2026-07-15T00:00:01Z"},{"number":11,"mergedAt":"2026-07-03T12:00:00Z"}]`,
		goTestLintRollup)
	since, until := autonomyWindow(t)
	prs, ok, cause := autonomyGates(root, since, until)
	if !ok || cause != "" || len(prs) != 1 || prs[0].Number != 11 || strings.Join(prs[0].CheckNames, ",") != "go-test,lint" {
		t.Fatalf("windowed rollups = %+v, ok=%v, cause=%q; want PR 11 with both check kinds", prs, ok, cause)
	}
	lines := gateCalls(t, log)
	if len(lines) != 2 || !strings.Contains(lines[0], "--search merged:2026-07-01T00:00:00Z..2026-07-15T00:00:00Z") ||
		!strings.Contains(lines[1], "api graphql") || !strings.Contains(lines[1], "pr11: pullRequest(number:11)") ||
		strings.Contains(lines[1], "pullRequest(number:12)") || !strings.Contains(lines[1], "-F owner={owner} -F name={repo}") {
		t.Fatalf("unexpected queries: %s", strings.Join(lines, "\n"))
	}
	// Both window bounds hold, a listed-twice PR is asked for once, and the
	// query selects both context kinds plus the overflow signal.
	if strings.Contains(lines[1], "pullRequest(number:13)") || strings.Count(lines[1], "pullRequest(number:11)") != 1 {
		t.Fatalf("rollup read outside the window or repeated: %s", lines[1])
	}
	for _, sel := range []string{"...on CheckRun{name}", "...on StatusContext{context}", "pageInfo{hasNextPage endCursor}"} {
		if !strings.Contains(lines[1], sel) {
			t.Fatalf("rollup query does not select %q: %s", sel, lines[1])
		}
	}
	for _, line := range lines {
		if !strings.HasPrefix(line, root+"|") {
			t.Fatalf("query ignored target root: %s", line)
		}
	}
}

func TestAutonomyGatesIncompleteReadIsUnmeasured(t *testing.T) {
	one := `[{"number":11,"mergedAt":"2026-07-03T12:00:00Z"}]`
	for _, tc := range []struct{ name, list, rollup, cause string }{
		{"list-json", `truncated`, goTestLintRollup, gateCauseMalformed},
		{"rollup-json", one, `raw:truncated`, gateCauseMalformed},
		{"missing-rollup", one, ``, gateCauseMalformed},
		{"missing-contexts", one, `"statusCheckRollup":{}`, gateCauseMalformed},
		{"missing-page-info", one, `"statusCheckRollup":{"contexts":{"nodes":[{"__typename":"CheckRun","name":"go-test"}]}}`, gateCauseMalformed},
		{"next-page-without-cursor", one, `"statusCheckRollup":{"contexts":{"pageInfo":{"hasNextPage":true,"endCursor":null},"nodes":[]}}`, gateCauseMalformed},
		{"missing-pr-node", one, `raw:{"data":{"repository":{}}}`, gateCauseMalformed},
		{"null-data", one, `raw:{"data":null}`, gateCauseMalformed},
		{"null-list", `null`, goTestLintRollup, gateCauseMalformed},
		{"non-positive-number", `[{"number":0,"mergedAt":"2026-07-03T12:00:00Z"}]`, goTestLintRollup, gateCauseMalformed},
		{"list-failure", `failure`, goTestLintRollup, gateCauseGHFailed},
		{"rollup-failure", one, `failure`, gateCauseGHFailed},
		{"graphql-errors", one, `raw:{"errors":[{"message":"timeout"}]}`, gateCauseGHFailed},
		{"graphql-errors-exit", one, `rawfail:{"errors":[{"message":"timeout"}]}`, gateCauseGHFailed},
		{"partial-data-errors", one, `raw:{"data":{"repository":{"pr11":null}},"errors":[{"message":"timeout"}]}`, gateCauseGHFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, _ := autonomyGHFixture(t, tc.list, tc.rollup)
			since, until := autonomyWindow(t)
			prs, ok, cause := autonomyGates(root, since, until)
			if ok || len(prs) != 0 || cause != tc.cause {
				t.Fatalf("incomplete read returned %+v, ok=%v, cause=%q; want cause %q", prs, ok, cause, tc.cause)
			}
		})
	}
}

// regression: #2086
// A full window's rollups are read in batched requests, never one gh call per
// merged PR: each in-window PR is asked for exactly once.
func TestGateRollupsBatched(t *testing.T) {
	const n = 45
	rows := make([]map[string]any, n)
	for i := range rows {
		rows[i] = map[string]any{"number": i + 1, "mergedAt": "2026-07-03T12:00:00Z"}
	}
	list, _ := json.Marshal(rows)
	root, log := autonomyGHFixture(t, string(list), goTestLintRollup)
	since, until := autonomyWindow(t)
	prs, ok, cause := autonomyGates(root, since, until)
	if !ok || cause != "" || len(prs) != n {
		t.Fatalf("batched read = %d PR(s), ok=%v, cause=%q; want %d", len(prs), ok, cause, n)
	}
	alias := regexp.MustCompile(`pr(\d+): pullRequest\(number:(\d+)\)`)
	reads, asked := 0, map[int]int{}
	for _, line := range gateCalls(t, log)[1:] {
		reads++
		for _, m := range alias.FindAllStringSubmatch(line, -1) {
			if m[1] != m[2] {
				t.Fatalf("alias %q does not name its PR", m[0])
			}
			v, _ := strconv.Atoi(m[2])
			asked[v]++
		}
	}
	want := (n + autonomyGateBatchSize - 1) / autonomyGateBatchSize
	if reads != want {
		t.Fatalf("%d rollup read(s) for %d PRs; want %d batched reads", reads, n, want)
	}
	for i := 1; i <= n; i++ {
		if asked[i] != 1 {
			t.Fatalf("PR %d asked %d time(s); want exactly once (asked=%v)", i, asked[i], asked)
		}
	}
}

// regression: #2086
// The symptom: with real per-call forge latency, a window of merged PRs read
// one call per PR outlives the deadline; the batched read fits inside it.
func TestGateReadFitsDeadline(t *testing.T) {
	rows := make([]map[string]any, 45)
	for i := range rows {
		rows[i] = map[string]any{"number": i + 1, "mergedAt": "2026-07-03T12:00:00Z"}
	}
	list, _ := json.Marshal(rows)
	root, _ := autonomyGHFixture(t, string(list), goTestLintRollup)
	t.Setenv("AUTONOMY_DELAY", "0.1")
	orig := autonomyGateDeadline
	autonomyGateDeadline = 3 * time.Second
	t.Cleanup(func() { autonomyGateDeadline = orig })
	since, until := autonomyWindow(t)
	prs, ok, cause := autonomyGates(root, since, until)
	if !ok || cause != "" || len(prs) != len(rows) {
		t.Fatalf("read under per-call latency = %d PR(s), ok=%v, cause=%q; want all %d inside the deadline", len(prs), ok, cause, len(rows))
	}
}

// regression: #2086
// A head commit with more than one page of contexts is read to the end (or to
// its first gate name), never counted from its first page alone.
func TestGateRollupOverflowIsPaged(t *testing.T) {
	root, log := autonomyGHFixture(t, `[{"number":11,"mergedAt":"2026-07-03T12:00:00Z"}]`, overflowRollup)
	t.Setenv("AUTONOMY_PAGE", gatePage(`{"hasNextPage":false,"endCursor":"c2"}`, `[{"__typename":"CheckRun","name":"go-test"}]`))
	since, until := autonomyWindow(t)
	prs, ok, cause := autonomyGates(root, since, until)
	if !ok || cause != "" || len(prs) != 1 || !hasDeterministicGate(prs[0].CheckNames) {
		t.Fatalf("overflowing rollup = %+v, ok=%v, cause=%q; want PR 11 gated by its second-page check", prs, ok, cause)
	}
	lines := gateCalls(t, log)
	if len(lines) != 3 || !strings.Contains(lines[2], "after:$after") ||
		!strings.Contains(lines[2], "-F num=11") || !strings.Contains(lines[2], "-f after=c1") {
		t.Fatalf("want one follow-up page read for PR 11 after cursor c1: %s", strings.Join(lines, "\n"))
	}
}

// A first page that already names a gate settles the PR: no further page read.
func TestGateRollupOverflowStopsAtGate(t *testing.T) {
	first := `"statusCheckRollup":{"contexts":{"pageInfo":{"hasNextPage":true,"endCursor":"c1"},"nodes":[{"__typename":"CheckRun","name":"go-test"}]}}`
	root, log := autonomyGHFixture(t, `[{"number":11,"mergedAt":"2026-07-03T12:00:00Z"}]`, first)
	t.Setenv("AUTONOMY_PAGE", "failure")
	since, until := autonomyWindow(t)
	prs, ok, cause := autonomyGates(root, since, until)
	if !ok || cause != "" || len(prs) != 1 || !hasDeterministicGate(prs[0].CheckNames) {
		t.Fatalf("gated first page = %+v, ok=%v, cause=%q; want PR 11 gated", prs, ok, cause)
	}
	if lines := gateCalls(t, log); len(lines) != 2 {
		t.Fatalf("paged a PR already settled by its first page: %s", strings.Join(lines, "\n"))
	}
}

// An overflowing rollup whose later pages cannot be read is could-not-check
// with a named cause, never a measured share built from the first page.
func TestGateRollupOverflowIncompleteIsUnmeasured(t *testing.T) {
	nonGate := `[{"__typename":"CheckRun","name":"changelog"}]`
	for _, tc := range []struct{ name, page, cause string }{
		{"page-failure", "failure", gateCauseGHFailed},
		{"page-errors", `{"errors":[{"message":"timeout"}]}`, gateCauseGHFailed},
		{"page-json", `truncated`, gateCauseMalformed},
		{"page-null-data", `{"data":null}`, gateCauseMalformed},
		{"page-missing-pr", `{"data":{"repository":{"pullRequest":null}}}`, gateCauseMalformed},
		{"page-null-rollup", `{"data":{"repository":{"pullRequest":{"commits":{"nodes":[{"commit":{"statusCheckRollup":null}}]}}}}}`, gateCauseMalformed},
		{"page-missing-page-info", `{"data":{"repository":{"pullRequest":{"commits":{"nodes":[{"commit":{"statusCheckRollup":{"contexts":{"nodes":[]}}}}]}}}}}`, gateCauseMalformed},
		{"page-repeats-cursor", gatePage(`{"hasNextPage":true,"endCursor":"c1"}`, nonGate), gateCauseMalformed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, _ := autonomyGHFixture(t, `[{"number":11,"mergedAt":"2026-07-03T12:00:00Z"}]`, overflowRollup)
			t.Setenv("AUTONOMY_PAGE", tc.page)
			since, until := autonomyWindow(t)
			prs, ok, cause := autonomyGates(root, since, until)
			if ok || len(prs) != 0 || cause != tc.cause {
				t.Fatalf("incomplete overflow read = %+v, ok=%v, cause=%q; want cause %q", prs, ok, cause, tc.cause)
			}
		})
	}
}

// Guard the expensive query shape at every literal command call in this
// module, including wrappers: rollups never ride inline on a `pr list`.
func bulkRollupCalls(src []byte) ([]token.Position, error) {
	fs := token.NewFileSet()
	f, err := parser.ParseFile(fs, "query.go", src, 0)
	if err != nil {
		return nil, err
	}
	var hits []token.Position
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		pr, list, rollup := false, false, false
		for _, arg := range call.Args {
			lit, ok := arg.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				continue
			}
			v, _ := strconv.Unquote(lit.Value)
			pr = pr || v == "pr"
			list = list || v == "list"
			rollup = rollup || strings.Contains(v, "statusCheckRollup")
		}
		if pr && list && rollup {
			hits = append(hits, fs.Position(call.Pos()))
		}
		return true
	})
	return hits, nil
}

func TestAutonomyBulkRollupGuard(t *testing.T) {
	// Positive control: a second caller, independent of autonomyGates.
	plant := []byte(`package main; func anotherReader() { run("pr", "list", "--json", "number,statusCheckRollup") }`)
	if hits, err := bulkRollupCalls(plant); err != nil || len(hits) != 1 {
		t.Fatalf("guard missed planted second caller: %v, %v", hits, err)
	}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		hits, err := bulkRollupCalls(src)
		if err != nil {
			t.Fatal(err)
		}
		for _, hit := range hits {
			t.Errorf("bulk status-check rollup query at %s:%d", name, hit.Line)
		}
	}
}

// perPRRollupCalls finds every literal command call in a file that reads a
// status-check rollup through gh's single-PR `view` verb. A window's rollups
// are read only through autonomyGates' batched GraphQL request (#2086): one gh
// process per merged PR is the read that outlived the deadline.
func perPRRollupCalls(src []byte) ([]token.Position, error) {
	fs := token.NewFileSet()
	f, err := parser.ParseFile(fs, "query.go", src, 0)
	if err != nil {
		return nil, err
	}
	var hits []token.Position
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		view, rollup := false, false
		for _, arg := range call.Args {
			lit, ok := arg.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				continue
			}
			v, _ := strconv.Unquote(lit.Value)
			view = view || v == "view"
			rollup = rollup || strings.Contains(v, "statusCheckRollup")
		}
		if view && rollup {
			hits = append(hits, fs.Position(call.Pos()))
		}
		return true
	})
	return hits, nil
}

// regression: #2086
func TestAutonomyPerPRRollupGuard(t *testing.T) {
	// Positive control: a second caller, independent of autonomyGates.
	plant := []byte(`package main; func anotherReader(n string) { run("pr", "view", n, "--json", "statusCheckRollup") }`)
	if hits, err := perPRRollupCalls(plant); err != nil || len(hits) != 1 {
		t.Fatalf("guard missed planted second caller: %v, %v", hits, err)
	}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		hits, err := perPRRollupCalls(src)
		if err != nil {
			t.Fatal(err)
		}
		for _, hit := range hits {
			t.Errorf("per-PR status-check rollup read at %s:%d", name, hit.Line)
		}
	}
}

func TestAutonomyGatesFullListingIsUnmeasured(t *testing.T) {
	rows := make([]map[string]any, 500)
	for i := range rows {
		rows[i] = map[string]any{"number": i + 1, "mergedAt": "2026-07-03T12:00:00Z"}
	}
	list, _ := json.Marshal(rows)
	root, log := autonomyGHFixture(t, string(list), goTestLintRollup)
	since, until := autonomyWindow(t)
	if prs, ok, cause := autonomyGates(root, since, until); ok || len(prs) != 0 || cause != gateCauseListingCap {
		t.Fatalf("capped list returned %+v, ok=%v, cause=%q; want %q", prs, ok, cause, gateCauseListingCap)
	}
	if lines := gateCalls(t, log); len(lines) != 1 {
		t.Fatalf("fetched rollups for an incomplete list: %s", strings.Join(lines, "\n"))
	}
}

func TestAutonomyGatesEmptyWindow(t *testing.T) {
	root, log := autonomyGHFixture(t, `[]`, `failure`)
	since, until := autonomyWindow(t)
	if prs, ok, cause := autonomyGates(root, since, until); !ok || len(prs) != 0 || cause != "" {
		t.Fatalf("empty window = %+v, ok=%v, cause=%q", prs, ok, cause)
	}
	if lines := gateCalls(t, log); len(lines) != 1 {
		t.Fatalf("read rollups for an empty window: %s", strings.Join(lines, "\n"))
	}
}

// An explicit empty rollup is a PR with no code gates (counted against the
// gated share), not an unreadable one. GraphQL answers a head commit with no
// checks at all as an explicit null rollup; an empty context list is the same.
func TestAutonomyGatesEmptyRollup(t *testing.T) {
	for _, rollup := range []string{`"statusCheckRollup":null`, `"statusCheckRollup":{"contexts":{"pageInfo":{"hasNextPage":false,"endCursor":null},"nodes":[]}}`} {
		root, _ := autonomyGHFixture(t, `[{"number":11,"mergedAt":"2026-07-03T12:00:00Z"}]`, rollup)
		since, until := autonomyWindow(t)
		prs, ok, cause := autonomyGates(root, since, until)
		if !ok || cause != "" || len(prs) != 1 || prs[0].Number != 11 || len(prs[0].CheckNames) != 0 {
			t.Fatalf("explicit empty rollup %s = %+v, ok=%v, cause=%q; want PR 11 with no checks", rollup, prs, ok, cause)
		}
	}
}

// A read that outlives the shared deadline names the deadline, not a gh failure.
func TestAutonomyGatesDeadline(t *testing.T) {
	root, _ := autonomyGHFixture(t, `[{"number":11,"mergedAt":"2026-07-03T12:00:00Z"}]`, `slow`)
	orig := autonomyGateDeadline
	autonomyGateDeadline = 300 * time.Millisecond
	t.Cleanup(func() { autonomyGateDeadline = orig })
	since, until := autonomyWindow(t)
	prs, ok, cause := autonomyGates(root, since, until)
	if ok || len(prs) != 0 || cause != gateCauseDeadline {
		t.Fatalf("deadline read = %+v, ok=%v, cause=%q; want %q", prs, ok, cause, gateCauseDeadline)
	}
}

// Every named cause reaches BOTH consumers as its own reason: never the
// generic gh-unreadable, and never shared with another cause.
func TestGateCauseReasonsDistinct(t *testing.T) {
	since, until := autonomyWindow(t)
	seen := map[string]string{}
	for _, cause := range gateCauses {
		in := autonomyInputs{Since: since, Until: until, Now: until, GateOK: false, GateCause: cause}
		var axis AutonomyAxis
		for _, a := range computeAutonomy(in).Axes {
			if a.Key == "deterministic_gate_share" {
				axis = a
			}
		}
		rung := ladderRungsFromAutonomy(in)[1]
		if axis.Measured || axis.Reason == "gh-unreadable" || axis.Reason == "" || axis.Detail == "" {
			t.Errorf("cause %q: axis = %+v; want its own unmeasured reason and detail", cause, axis)
		}
		if rung.Measured || rung.Reason != axis.Reason {
			t.Errorf("cause %q: ladder rung reason %q, axis reason %q; want the same named reason", cause, rung.Reason, axis.Reason)
		}
		if prev, dup := seen[axis.Reason]; dup {
			t.Errorf("causes %q and %q share reason %q", prev, cause, axis.Reason)
		}
		seen[axis.Reason] = cause
	}
	if r, _ := gateUnmeasured("", since, until); r != "gh-unreadable" {
		t.Errorf("unnamed cause reason = %q; want the generic gh-unreadable fallback", r)
	}
}

// unnamedGateFailures returns the line of every ok=false return inside the
// autonomyGates literal whose cause is not an identifier listed in gateCauses.
func unnamedGateFailures(src []byte) ([]int, error) {
	fs := token.NewFileSet()
	f, err := parser.ParseFile(fs, "autonomy.go", src, 0)
	if err != nil {
		return nil, err
	}
	allowed := map[string]bool{}
	var body *ast.FuncLit
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, sp := range gd.Specs {
			vs, ok := sp.(*ast.ValueSpec)
			if !ok || len(vs.Names) != 1 || len(vs.Values) != 1 {
				continue
			}
			switch vs.Names[0].Name {
			case "gateCauses":
				if cl, ok := vs.Values[0].(*ast.CompositeLit); ok {
					for _, e := range cl.Elts {
						if id, ok := e.(*ast.Ident); ok {
							allowed[id.Name] = true
						}
					}
				}
			case "autonomyGates":
				body, _ = vs.Values[0].(*ast.FuncLit)
			}
		}
	}
	if body == nil || len(allowed) == 0 {
		return nil, fmt.Errorf("autonomyGates literal or gateCauses list not found")
	}
	var bad []int
	ast.Inspect(body, func(n ast.Node) bool {
		ret, ok := n.(*ast.ReturnStmt)
		if !ok || len(ret.Results) != 3 {
			return true
		}
		if okID, isID := ret.Results[1].(*ast.Ident); !isID || okID.Name != "true" {
			if id, isID := ret.Results[2].(*ast.Ident); !isID || !allowed[id.Name] {
				bad = append(bad, fs.Position(ret.Pos()).Line)
			}
		}
		return true
	})
	return bad, nil
}

func TestGateFailuresNameCause(t *testing.T) {
	// Positive control: an unnamed and an unlisted cause, beside a named one.
	plant := []byte(`package main
var gateCauses = []string{gateCauseGHFailed}
var autonomyGates = func() ([]int, bool, string) {
	if a { return nil, false, gateCauseGHFailed }
	if b { return nil, false, "" }
	return nil, false, gateCauseOther
}`)
	if bad, err := unnamedGateFailures(plant); err != nil || len(bad) != 2 || bad[0] != 5 || bad[1] != 6 {
		t.Fatalf("guard missed planted unnamed causes: %v, %v", bad, err)
	}
	src, err := os.ReadFile("autonomy.go")
	if err != nil {
		t.Fatal(err)
	}
	bad, err := unnamedGateFailures(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range bad {
		t.Errorf("autonomyGates failure without a named cause at autonomy.go:%d", line)
	}
}
