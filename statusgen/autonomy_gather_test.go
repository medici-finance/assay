package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func autonomyGHFixture(t *testing.T, list, view string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	script := `#!/bin/sh
printf '%s|%s\n' "$PWD" "$*" >> "$AUTONOMY_CALLS"
case "$*" in
  *"pr list"*statusCheckRollup*) exit 42 ;;
  *"pr list"*) [ "$AUTONOMY_LIST" = failure ] && exit 45; printf '%s' "$AUTONOMY_LIST" ;;
  *"pr view"*) [ "$AUTONOMY_VIEW" = failure ] && exit 44; [ "$AUTONOMY_VIEW" = slow ] && exec sleep 5; printf '%s' "$AUTONOMY_VIEW" ;;
  *) exit 43 ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("AUTONOMY_CALLS", log)
	t.Setenv("AUTONOMY_LIST", list)
	t.Setenv("AUTONOMY_VIEW", view)
	return dir, log
}

func TestAutonomyGatesWindowedRollups(t *testing.T) {
	root, log := autonomyGHFixture(t,
		`[{"number":11,"mergedAt":"2026-07-03T12:00:00Z"},{"number":12,"mergedAt":"2026-06-30T23:59:59Z"}]`,
		`{"statusCheckRollup":[{"name":"go-test"},{"context":"lint"}]}`)
	since, until := autonomyWindow(t)
	prs, ok, cause := autonomyGates(root, since, until)
	if !ok || cause != "" || len(prs) != 1 || prs[0].Number != 11 || strings.Join(prs[0].CheckNames, ",") != "go-test,lint" {
		t.Fatalf("windowed rollups = %+v, ok=%v, cause=%q; want PR 11 with both check kinds", prs, ok, cause)
	}
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(calls)), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], "--search merged:2026-07-01T00:00:00Z..2026-07-15T00:00:00Z") || !strings.Contains(lines[1], "pr view 11 --json statusCheckRollup") {
		t.Fatalf("unexpected queries: %s", calls)
	}
	for _, line := range lines {
		if !strings.HasPrefix(line, root+"|") {
			t.Fatalf("query ignored target root: %s", line)
		}
	}
}

func TestAutonomyGatesIncompleteReadIsUnmeasured(t *testing.T) {
	one := `[{"number":11,"mergedAt":"2026-07-03T12:00:00Z"}]`
	for _, tc := range []struct{ name, list, view, cause string }{
		{"list-json", `truncated`, `{}`, gateCauseMalformed},
		{"rollup-json", one, `truncated`, gateCauseMalformed},
		{"missing-rollup", one, `{}`, gateCauseMalformed},
		{"null-list", `null`, `{}`, gateCauseMalformed},
		{"non-positive-number", `[{"number":0,"mergedAt":"2026-07-03T12:00:00Z"}]`, `{"statusCheckRollup":[]}`, gateCauseMalformed},
		{"list-failure", `failure`, `{}`, gateCauseGHFailed},
		{"rollup-failure", one, `failure`, gateCauseGHFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, _ := autonomyGHFixture(t, tc.list, tc.view)
			since, until := autonomyWindow(t)
			prs, ok, cause := autonomyGates(root, since, until)
			if ok || len(prs) != 0 || cause != tc.cause {
				t.Fatalf("incomplete read returned %+v, ok=%v, cause=%q; want cause %q", prs, ok, cause, tc.cause)
			}
		})
	}
}

// Guard the expensive query shape at every literal command call in this
// module, including wrappers: rollups belong on a single-PR read, never a list.
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

func TestAutonomyGatesFullListingIsUnmeasured(t *testing.T) {
	rows := make([]map[string]any, 500)
	for i := range rows {
		rows[i] = map[string]any{"number": i + 1, "mergedAt": "2026-07-03T12:00:00Z"}
	}
	list, _ := json.Marshal(rows)
	root, log := autonomyGHFixture(t, string(list), `{"statusCheckRollup":[]}`)
	since, until := autonomyWindow(t)
	if prs, ok, cause := autonomyGates(root, since, until); ok || len(prs) != 0 || cause != gateCauseListingCap {
		t.Fatalf("capped list returned %+v, ok=%v, cause=%q; want %q", prs, ok, cause, gateCauseListingCap)
	}
	calls, _ := os.ReadFile(log)
	if strings.Contains(string(calls), "pr view") {
		t.Fatalf("fetched rollups for an incomplete list: %s", calls)
	}
}

func TestAutonomyGatesEmptyWindow(t *testing.T) {
	root, _ := autonomyGHFixture(t, `[]`, `{}`)
	since, until := autonomyWindow(t)
	if prs, ok, cause := autonomyGates(root, since, until); !ok || len(prs) != 0 || cause != "" {
		t.Fatalf("empty window = %+v, ok=%v, cause=%q", prs, ok, cause)
	}
}

// An explicit empty rollup is a PR with no code gates (counted against the
// gated share), not an unreadable one.
func TestAutonomyGatesExplicitEmptyRollup(t *testing.T) {
	root, _ := autonomyGHFixture(t, `[{"number":11,"mergedAt":"2026-07-03T12:00:00Z"}]`, `{"statusCheckRollup":[]}`)
	since, until := autonomyWindow(t)
	prs, ok, cause := autonomyGates(root, since, until)
	if !ok || cause != "" || len(prs) != 1 || prs[0].Number != 11 || len(prs[0].CheckNames) != 0 {
		t.Fatalf("explicit empty rollup = %+v, ok=%v, cause=%q; want PR 11 with no checks", prs, ok, cause)
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
