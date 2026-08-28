package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ----- fixture helpers -----

// writeGraphStreamREADME drops a minimal-but-valid stream README carrying a
// briefs table so loadStreams/parseStreamREADME yields one brief node per row.
func writeGraphStreamREADME(t *testing.T, streamsDir, name string, briefNums []string) {
	t.Helper()
	dir := filepath.Join(streamsDir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var rows strings.Builder
	rows.WriteString("| # | Brief | Wave | Status | Verified | Reviewed |\n")
	rows.WriteString("|---|-------|------|--------|----------|----------|\n")
	for _, n := range briefNums {
		fmt.Fprintf(&rows, "| %s | brief %s | 0 | todo | — | — |\n", n, n)
	}
	body := fmt.Sprintf("---\nstream: %s\nstatus: active\npriority: P1\n---\n\n# %s\n\n%s",
		name, name, rows.String())
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeGraphBrief drops a valid brief-v1 file with the given typed edges.
func writeGraphBrief(t *testing.T, streamsDir, stream, num string, depends, unblocks, sources []string, issues []int) {
	t.Helper()
	q := func(xs []string) string {
		if len(xs) == 0 {
			return "[]"
		}
		var quoted []string
		for _, x := range xs {
			quoted = append(quoted, fmt.Sprintf("%q", x))
		}
		return "[" + strings.Join(quoted, ", ") + "]"
	}
	qi := func(xs []int) string {
		if len(xs) == 0 {
			return "[]"
		}
		var parts []string
		for _, x := range xs {
			parts = append(parts, fmt.Sprintf("%d", x))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	}
	body := fmt.Sprintf(`---
brief: %s/%s
title: fixture brief %s
wave: 0
depends: %s
unblocks: %s
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: %s
schema: brief-v1
authored: 2026-08-17 by fixture
sources: %s
---

# Brief %s
Body.
`, stream, num, num, q(depends), q(unblocks), qi(issues), q(sources), num)
	if err := os.WriteFile(filepath.Join(streamsDir, stream, "brief-"+num+".md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeGraphFinding drops a valid per-entry finding file.
func writeGraphFinding(t *testing.T, streamsDir, id, title string, affects []string) {
	t.Helper()
	dir := filepath.Join(streamsDir, "findings")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var aff strings.Builder
	for _, a := range affects {
		fmt.Fprintf(&aff, "  - %q\n", a)
	}
	body := fmt.Sprintf("---\nid: %s\ndate: 2026-08-10\ntitle: %s\naffects:\n%sresolved: false\n---\n\nBody.\n",
		id, title, aff.String())
	if err := os.WriteFile(filepath.Join(dir, "2026-08-10-"+id+".md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeGraphIntake drops a valid per-entry intake file.
func writeGraphIntake(t *testing.T, streamsDir, id, title string) {
	t.Helper()
	dir := filepath.Join(streamsDir, "intake")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf("---\nid: %s\ndate: 2026-08-11\ntitle: %s\ndisposition: new\n---\n\nBody.\n", id, title)
	if err := os.WriteFile(filepath.Join(dir, "2026-08-11-"+id+".md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// graphFixtureRoot builds a small but complete tree exercising every node type
// and every edge type, and returns the root.
//
//	stream alpha: briefs 01, 02, 03
//	  01 --unblocks--> 02        01 --issues--> #111
//	  02 --depends-->  01        02 --unblocks--> 03
//	  03 --depends-->  02        03 --sources--> F-token-expiry-bug (typed id in prose)
//	finding F-token-expiry-bug --affects--> alpha/02
//	intake  I-oauth-scope-gap
//	stream alpha --issues--> #222 (frontmatter issues)
func graphFixtureRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	streamsDir := filepath.Join(root, "docs", "streams")
	if err := os.MkdirAll(streamsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeGraphStreamREADME(t, streamsDir, "alpha", []string{"01", "02", "03"})
	writeGraphBrief(t, streamsDir, "alpha", "01", nil, []string{"alpha/02"}, nil, []int{111})
	writeGraphBrief(t, streamsDir, "alpha", "02", []string{"alpha/01"}, []string{"alpha/03"}, nil, nil)
	writeGraphBrief(t, streamsDir, "alpha", "03", []string{"alpha/02"}, nil,
		[]string{"finding F-token-expiry-bug — the register entry this brief derives from"}, nil)
	writeGraphFinding(t, streamsDir, "F-token-expiry-bug", "token expiry bug", []string{"alpha/02"})
	writeGraphIntake(t, streamsDir, "I-oauth-scope-gap", "oauth scope gap")
	return root
}

// hasEdge reports whether the model carries the exact directed edge.
func hasEdge(g *graphModel, typ, from, to string) bool {
	for _, e := range g.edges {
		if e.Type == typ && e.From == from && e.To == to {
			return true
		}
	}
	return false
}

// ----- tests -----

// TestGraphBuildNodesAndEdges asserts every node type and every edge type is
// derived from the fixture tree with the expected typed IDs.
func TestGraphBuildNodesAndEdges(t *testing.T) {
	root := graphFixtureRoot(t)
	g, err := buildGraph(root)
	if err != nil {
		t.Fatalf("buildGraph: %v", err)
	}

	wantNodes := []struct{ id, typ string }{
		{"alpha", graphTypeStream},
		{"alpha/01", graphTypeBrief},
		{"alpha/02", graphTypeBrief},
		{"alpha/03", graphTypeBrief},
		{"F-token-expiry-bug", graphTypeFinding},
		{"I-oauth-scope-gap", graphTypeIntake},
		{"#111", graphTypeIssue},
	}
	for _, w := range wantNodes {
		n, ok := g.nodes[w.id]
		if !ok {
			t.Errorf("missing node %q", w.id)
			continue
		}
		if n.Type != w.typ {
			t.Errorf("node %q: type = %q, want %q", w.id, n.Type, w.typ)
		}
	}

	wantEdges := []graphEdge{
		{"contains", "alpha", "alpha/01"},
		{"contains", "alpha", "alpha/02"},
		{"contains", "alpha", "alpha/03"},
		{"depends", "alpha/02", "alpha/01"},
		{"depends", "alpha/03", "alpha/02"},
		{"unblocks", "alpha/01", "alpha/02"},
		{"unblocks", "alpha/02", "alpha/03"},
		{"affects", "F-token-expiry-bug", "alpha/02"},
		{"sources", "alpha/03", "F-token-expiry-bug"},
		{"issues", "alpha/01", "#111"},
	}
	for _, w := range wantEdges {
		if !hasEdge(g, w.Type, w.From, w.To) {
			t.Errorf("missing edge %s: %s -> %s", w.Type, w.From, w.To)
		}
	}
}

// TestGraphAffectsEdgeDirection pins the finding→brief direction: the edge must
// run FROM the finding TO the brief it affects, never reversed. This is the
// direction the transitive finding-impact closure ("which briefs are downstream
// of this finding") depends on.
func TestGraphAffectsEdgeDirection(t *testing.T) {
	root := graphFixtureRoot(t)
	g, err := buildGraph(root)
	if err != nil {
		t.Fatalf("buildGraph: %v", err)
	}
	if !hasEdge(g, "affects", "F-token-expiry-bug", "alpha/02") {
		t.Errorf("affects edge must run finding -> brief (F-token-expiry-bug -> alpha/02)")
	}
	if hasEdge(g, "affects", "alpha/02", "F-token-expiry-bug") {
		t.Errorf("affects edge must NOT run brief -> finding (reversed direction)")
	}
}

// TestGraphAffectsBriefPrefixForm asserts the `<stream>/brief-<NN>` affects form
// resolves to the same brief node as `<stream>/<NN>`.
func TestGraphAffectsBriefPrefixForm(t *testing.T) {
	briefIDs := map[string]bool{"alpha/02": true}
	streamNames := map[string]bool{"alpha": true}
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"alpha/02", "alpha/02", true},
		{"alpha/brief-02", "alpha/02", true},
		{"alpha", "alpha", true},
		{"alpha/99", "", false}, // no such brief node
		{"ghost", "", false},    // no such stream node
	}
	for _, c := range cases {
		got, ok := graphAffectsTarget(c.in, briefIDs, streamNames)
		if got != c.want || ok != c.ok {
			t.Errorf("graphAffectsTarget(%q) = (%q,%v), want (%q,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

// TestGraphSourcesTypedIDFidelity asserts a sources edge is emitted only when a
// source names a KNOWN register id, and never for prose that merely looks
// id-shaped or names an unknown id.
func TestGraphSourcesTypedIDFidelity(t *testing.T) {
	root := t.TempDir()
	streamsDir := filepath.Join(root, "docs", "streams")
	if err := os.MkdirAll(streamsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeGraphStreamREADME(t, streamsDir, "beta", []string{"01"})
	// Two sources: one names the real finding, one names a plausible-but-unknown id.
	writeGraphBrief(t, streamsDir, "beta", "01", nil, nil,
		[]string{"F-real-register-entry — real", "F-not-a-real-entry — unknown"}, nil)
	writeGraphFinding(t, streamsDir, "F-real-register-entry", "real", nil)

	g, err := buildGraph(root)
	if err != nil {
		t.Fatalf("buildGraph: %v", err)
	}
	if !hasEdge(g, "sources", "beta/01", "F-real-register-entry") {
		t.Errorf("sources edge to the KNOWN register id must be emitted")
	}
	if hasEdge(g, "sources", "beta/01", "F-not-a-real-entry") {
		t.Errorf("sources edge to an UNKNOWN id must NOT be emitted (typed-id fidelity)")
	}
}

// TestGraphIssueNodesAsReferenced asserts an issue node exists only when a
// stream or brief references it, and unreferenced numbers never appear.
func TestGraphIssueNodesAsReferenced(t *testing.T) {
	root := graphFixtureRoot(t)
	g, err := buildGraph(root)
	if err != nil {
		t.Fatalf("buildGraph: %v", err)
	}
	if _, ok := g.nodes["#111"]; !ok {
		t.Errorf("referenced issue #111 must be a node")
	}
	if _, ok := g.nodes["#999"]; ok {
		t.Errorf("unreferenced issue #999 must NOT be a node")
	}
}

// TestGraphJSONLDeterministic asserts each JSONL line is valid JSON, nodes
// precede edges, and two independent emits are byte-identical.
func TestGraphJSONLDeterministic(t *testing.T) {
	root := graphFixtureRoot(t)
	g, err := buildGraph(root)
	if err != nil {
		t.Fatalf("buildGraph: %v", err)
	}
	var a, b bytes.Buffer
	if err := emitGraphJSONL(&a, g); err != nil {
		t.Fatalf("emit a: %v", err)
	}
	if err := emitGraphJSONL(&b, g); err != nil {
		t.Fatalf("emit b: %v", err)
	}
	if a.String() != b.String() {
		t.Fatalf("JSONL emit is not byte-deterministic across runs")
	}

	lines := strings.Split(strings.TrimRight(a.String(), "\n"), "\n")
	if len(lines) == 0 {
		t.Fatal("no JSONL output")
	}
	seenEdge := false
	for i, line := range lines {
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Fatalf("line %d is not valid JSON: %v\n%s", i, err, line)
		}
		kind, _ := obj["kind"].(string)
		switch kind {
		case "node":
			if seenEdge {
				t.Errorf("line %d: a node object appears AFTER an edge object — nodes must precede edges", i)
			}
		case "edge":
			seenEdge = true
		default:
			t.Errorf("line %d: unexpected kind %q", i, kind)
		}
	}
}

// TestGraphDOTDeterministic asserts the DOT output opens a digraph, contains
// the expected typed node/edge ids, and is byte-deterministic.
func TestGraphDOTDeterministic(t *testing.T) {
	root := graphFixtureRoot(t)
	g, err := buildGraph(root)
	if err != nil {
		t.Fatalf("buildGraph: %v", err)
	}
	var a, b bytes.Buffer
	if err := emitGraphDOT(&a, g); err != nil {
		t.Fatalf("emit a: %v", err)
	}
	if err := emitGraphDOT(&b, g); err != nil {
		t.Fatalf("emit b: %v", err)
	}
	if a.String() != b.String() {
		t.Fatalf("DOT emit is not byte-deterministic across runs")
	}
	out := a.String()
	if !strings.HasPrefix(out, "digraph ") {
		t.Errorf("DOT output must open a digraph; got:\n%s", firstLine(out))
	}
	if !strings.Contains(out, `"alpha/02" [type=brief`) {
		t.Errorf("DOT output must declare the typed brief node alpha/02")
	}
	if !strings.Contains(out, `"F-token-expiry-bug" -> "alpha/02" [label="affects"]`) {
		t.Errorf("DOT output must carry the labelled affects edge")
	}
}

// TestGraphReadOnly asserts the export neither reads nor writes STATUS.md — the
// mode is a pure derive over the parse tree.
func TestGraphReadOnly(t *testing.T) {
	root := graphFixtureRoot(t)
	g, err := buildGraph(root)
	if err != nil {
		t.Fatalf("buildGraph: %v", err)
	}
	var buf bytes.Buffer
	if err := emitGraphJSONL(&buf, g); err != nil {
		t.Fatalf("emit: %v", err)
	}
	if err := emitGraphDOT(&buf, g); err != nil {
		t.Fatalf("emit: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "STATUS.md")); !os.IsNotExist(err) {
		t.Errorf("--graph must not create STATUS.md (stat err = %v)", err)
	}
}

// TestGraphRunModeFormatValidation asserts an unknown --graph format is a usage
// error (exit 2), not a silent empty export.
func TestGraphRunModeFormatValidation(t *testing.T) {
	root := graphFixtureRoot(t)
	if rc := runGraph(root, "bogus"); rc != 2 {
		t.Errorf("runGraph with bogus format: rc = %d, want 2", rc)
	}
	if rc := runGraph(root, "dot"); rc != 0 {
		t.Errorf("runGraph dot: rc = %d, want 0", rc)
	}
	if rc := runGraph(root, "jsonl"); rc != 0 {
		t.Errorf("runGraph jsonl: rc = %d, want 0", rc)
	}
}
