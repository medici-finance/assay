package main

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func autonomyGHFixture(t *testing.T, list, view string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	script := `#!/bin/sh
printf '%s|%s\n' "$PWD" "$*" >> "$AUTONOMY_CALLS"
case "$*" in
  *"pr list"*statusCheckRollup*) exit 42 ;;
  *"pr list"*) printf '%s' "$AUTONOMY_LIST" ;;
  *"pr view"*) [ "$AUTONOMY_VIEW" = failure ] && exit 44; printf '%s' "$AUTONOMY_VIEW" ;;
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
	prs, ok := autonomyGates(root, since, until)
	if !ok || len(prs) != 1 || prs[0].Number != 11 || strings.Join(prs[0].CheckNames, ",") != "go-test,lint" {
		t.Fatalf("windowed rollups = %+v, ok=%v; want PR 11 with both check kinds", prs, ok)
	}
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(calls)), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], "--search merged:2026-07-01..2026-07-15") || !strings.Contains(lines[1], "pr view 11 --json statusCheckRollup") {
		t.Fatalf("unexpected queries: %s", calls)
	}
	for _, line := range lines {
		if !strings.HasPrefix(line, root+"|") {
			t.Fatalf("query ignored target root: %s", line)
		}
	}
}

func TestAutonomyGatesIncompleteReadIsUnmeasured(t *testing.T) {
	for _, tc := range []struct{ name, list, view string }{
		{"list-json", `truncated`, `{}`},
		{"rollup-json", `[{"number":11,"mergedAt":"2026-07-03T12:00:00Z"}]`, `truncated`},
		{"missing-rollup", `[{"number":11,"mergedAt":"2026-07-03T12:00:00Z"}]`, `{}`},
		{"rollup-failure", `[{"number":11,"mergedAt":"2026-07-03T12:00:00Z"}]`, `failure`},
		{"null-list", `null`, `{}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, _ := autonomyGHFixture(t, tc.list, tc.view)
			since, until := autonomyWindow(t)
			prs, ok := autonomyGates(root, since, until)
			if ok || len(prs) != 0 {
				t.Fatalf("incomplete read returned %+v, ok=%v", prs, ok)
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
	if prs, ok := autonomyGates(root, since, until); ok || len(prs) != 0 {
		t.Fatalf("capped list returned %+v, ok=%v", prs, ok)
	}
	calls, _ := os.ReadFile(log)
	if strings.Contains(string(calls), "pr view") {
		t.Fatalf("fetched rollups for an incomplete list: %s", calls)
	}
}

func TestAutonomyGatesEmptyWindow(t *testing.T) {
	root, _ := autonomyGHFixture(t, `[]`, `{}`)
	since, until := autonomyWindow(t)
	if prs, ok := autonomyGates(root, since, until); !ok || len(prs) != 0 {
		t.Fatalf("empty window = %+v, ok=%v", prs, ok)
	}
}
