package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTree writes files (relative slash paths → contents) under a fresh temp
// root and returns the root.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
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

func classes(res shardResult) []string {
	var out []string
	for _, f := range res.Findings {
		out = append(out, f.Class)
	}
	return out
}

func hasClass(res shardResult, class string) bool {
	for _, f := range res.Findings {
		if f.Class == class {
			return true
		}
	}
	return false
}

func TestGlobMatch(t *testing.T) {
	cases := []struct {
		pattern, name string
		want          bool
	}{
		{"backend/**", "backend/Main.kt", true},
		{"backend/**", "backend/a/b/c.kt", true},
		{"backend/**", "backendx/Main.kt", false},
		{"backend/", "backend/Main.kt", true},
		// ** must match ZERO segments — otherwise docs/**/README.md would miss
		// the top-level README and a shared surface would go unreported.
		{"docs/**/README.md", "docs/README.md", true},
		{"docs/**/README.md", "docs/streams/methodology/README.md", true},
		{"**/go.mod", "go.mod", true},
		{"**/go.mod", "statusgen/go.mod", true},
		{"statusgen/*.go", "statusgen/emit.go", true},
		{"statusgen/*.go", "statusgen/sub/emit.go", false},
		{"STATUS.md", "STATUS.md", true},
		{"STATUS.md", "docs/STATUS.md", false},
	}
	for _, c := range cases {
		if got := globMatch(c.pattern, c.name); got != c.want {
			t.Errorf("globMatch(%q, %q) = %v, want %v", c.pattern, c.name, got, c.want)
		}
	}
}

// TestShardcheckEmitArityCollisionIsTheControL is the positive control this
// check ships with, and it is a REPRODUCTION of a real defect rather than a
// synthetic one: statusgen's own emit() grew an 8th parameter on one branch
// while a second, independently-green branch still called it with 7 args.
// Different files. No textual conflict. Main went red on merge.
//
// A file-scoped split puts emit.go in one shard and main.go in another and sees
// two disjoint globs — which is exactly why a path-only partition would have
// APPROVED the split that produces this failure. The check must refuse it.
//
// The mutation half is the proof it can pass: with the call site removed and
// nothing else changed, the same plan over the same globs must come back
// checked-clean. A check that refused both is a constant, not a detector.
func TestShardcheckEmitArityCollisionIsTheControl(t *testing.T) {
	const emitGo = `package main

func emit(a, b, c, d, e, f, g int) string { return "" }
`
	const mainCalls = `package main

func run() string { return emit(1, 2, 3, 4, 5, 6, 7) }
`
	const mainNoCall = `package main

func run() string { return "" }
`
	plan := []ParallelStream{
		{Name: "engine", Files: []string{"statusgen/emit.go"}},
		{Name: "cli", Files: []string{"statusgen/main.go"}},
	}

	t.Run("reproduction is refused", func(t *testing.T) {
		root := writeTree(t, map[string]string{
			"statusgen/emit.go": emitGo,
			"statusgen/main.go": mainCalls,
		})
		res, err := checkShardPlan(root, plan)
		if err != nil {
			t.Fatalf("checkShardPlan: %v", err)
		}
		if !hasClass(res, "symbol-coupling") {
			t.Fatalf("the emit() arity collision was NOT detected; findings=%v", classes(res))
		}
		var detail string
		for _, f := range res.Findings {
			if f.Class == "symbol-coupling" {
				detail = f.Detail
			}
		}
		if !strings.Contains(detail, "emit") {
			t.Errorf("finding does not name the coupled symbol: %q", detail)
		}
		if got := res.state(); got != shardExitFailed {
			t.Errorf("state = %d, want %d (checked-failed)", got, shardExitFailed)
		}
	})

	t.Run("mutation removes the call and the same plan passes", func(t *testing.T) {
		root := writeTree(t, map[string]string{
			"statusgen/emit.go": emitGo,
			"statusgen/main.go": mainNoCall,
		})
		res, err := checkShardPlan(root, plan)
		if err != nil {
			t.Fatalf("checkShardPlan: %v", err)
		}
		if len(res.Findings) != 0 {
			t.Fatalf("mutant should be clean, got findings %v (%+v)", classes(res), res.Findings)
		}
		if got := res.state(); got != shardExitClean {
			t.Fatalf("state = %d, want %d (checked-clean) — the check cannot distinguish the defect from its absence", got, shardExitClean)
		}
		if res.Coverage.AnalysedPairs == 0 {
			t.Error("clean verdict with 0 analysed pairs would be vacuous")
		}
	})
}

// Path overlap is the ONE class a path partition prevents — and only when
// someone checks it. An asserted disjointness is not a checked one.
func TestShardcheckPathOverlapRefused(t *testing.T) {
	root := writeTree(t, map[string]string{
		"pkg/a.txt": "a",
		"pkg/b.txt": "b",
	})
	res, err := checkShardPlan(root, []ParallelStream{
		{Name: "one", Files: []string{"pkg/**"}},
		{Name: "two", Files: []string{"pkg/b.txt"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !hasClass(res, "path-overlap") {
		t.Fatalf("overlapping globs not detected; findings=%v", classes(res))
	}
	if res.state() != shardExitFailed {
		t.Errorf("state = %d, want checked-failed", res.state())
	}
}

// The brief-rules numbering space: two shards editing DIFFERENT lines of it
// still collide, because the contended resource is the number, not the byte.
// This is the class that put duplicate rules 25 and 26 on main.
func TestShardcheckSharedSurfaceRefused(t *testing.T) {
	root := writeTree(t, map[string]string{
		"docs/brief-rules.md":                "1. **a**\n",
		"docs/streams/example/README.md":     "| # | Brief |\n",
		"docs/streams/example/brief-01-x.md": "x",
		"docs/streams/example/brief-02-y.md": "y",
	})
	res, err := checkShardPlan(root, []ParallelStream{
		{Name: "rules", Files: []string{"docs/brief-rules.md"}},
		{Name: "briefs", Files: []string{"docs/streams/example/brief-*.md"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !hasClass(res, "shared-surface") {
		t.Fatalf("shared numbering space not detected; findings=%v", classes(res))
	}
	// The finding must say WHICH surface class, so a dispatcher can act on it
	// without re-deriving why the file is special.
	var detail string
	for _, f := range res.Findings {
		if f.Class == "shared-surface" {
			detail = f.Detail
		}
	}
	if !strings.Contains(detail, "numbering-space") {
		t.Errorf("finding does not name the surface class: %q", detail)
	}

	// And the stream README row table is the same class.
	res2, err := checkShardPlan(root, []ParallelStream{
		{Name: "board", Files: []string{"docs/streams/example/README.md"}},
		{Name: "briefs", Files: []string{"docs/streams/example/brief-*.md"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !hasClass(res2, "shared-surface") {
		t.Errorf("stream README row table not detected as a shared surface; findings=%v", classes(res2))
	}
}

// The three-state invariant, applied to the checker itself: a plan whose
// cross-shard pairs it cannot analyse is could-not-check, and a caller must
// treat that exactly like a refusal. A path-disjoint, finding-free plan that
// nothing verified must NEVER come back 0.
func TestShardcheckUnanalysablePairIsCouldNotCheck(t *testing.T) {
	root := writeTree(t, map[string]string{
		"backend/Main.kt": "fun main() {}",
		"frontend/app.ts": "export const x = 1",
	})
	res, err := checkShardPlan(root, []ParallelStream{
		{Name: "backend", Files: []string{"backend/**"}},
		{Name: "fe", Files: []string{"frontend/**"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Findings) != 0 {
		t.Fatalf("no collision exists here; got %v", classes(res))
	}
	if res.state() != shardExitCouldNot {
		t.Fatalf("state = %d, want %d (could-not-check) — an unanalysed pair reported as clean is the silent cap this design forbids", res.state(), shardExitCouldNot)
	}
	if len(res.Coverage.Gaps) == 0 {
		t.Error("could-not-check with no gap printed tells the caller nothing")
	}
	joined := strings.Join(res.Coverage.Gaps, "\n")
	if !strings.Contains(joined, "non-Go") {
		t.Errorf("gap does not name why it could not check: %q", joined)
	}
}

// Cross-package Go is unanalysable too, and must not fall through to clean
// merely because both sides happen to be .go files.
func TestShardcheckCrossPackageGoIsCouldNotCheck(t *testing.T) {
	root := writeTree(t, map[string]string{
		"a/a.go": "package a\n\nfunc F() {}\n",
		"b/b.go": "package b\n\nfunc G() {}\n",
	})
	res, err := checkShardPlan(root, []ParallelStream{
		{Name: "a", Files: []string{"a/**"}},
		{Name: "b", Files: []string{"b/**"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.state() != shardExitCouldNot {
		t.Fatalf("state = %d, want could-not-check for a cross-package pair", res.state())
	}
}

// A glob that matches nothing is a shard that would run with a scope it cannot
// fill — a typo or a dead shard, never a pass.
func TestShardcheckDeadGlobRefused(t *testing.T) {
	root := writeTree(t, map[string]string{"pkg/a.go": "package pkg\n"})
	res, err := checkShardPlan(root, []ParallelStream{
		{Name: "real", Files: []string{"pkg/**"}},
		{Name: "typo", Files: []string{"pgk/**"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !hasClass(res, "dead-glob") {
		t.Fatalf("dead glob not detected; findings=%v", classes(res))
	}
}

// The committed negative control. Its positive twin is the LIVE tree —
// statusgen/main.go:466 calls emit() declared at statusgen/emit.go:221, so
// `shardcheck --root . --shard engine=statusgen/emit.go --shard
// cli=statusgen/main.go` refuses on this repo as it stands. Keeping the green
// half in the test suite means a change that made the checker refuse
// everything (the cheapest way to fake a working detector) turns this red.
func TestShardcheckCommittedNegativeControl(t *testing.T) {
	res, err := checkShardPlan("testdata/shardcheck/decoupled", []ParallelStream{
		{Name: "a", Files: []string{"alpha.go"}},
		{Name: "b", Files: []string{"beta.go"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.state() != shardExitClean {
		t.Fatalf("negative control is not clean: state=%d findings=%v gaps=%v", res.state(), classes(res), res.Coverage.Gaps)
	}
	if res.Coverage.AnalysedPairs != 1 {
		t.Errorf("AnalysedPairs = %d, want 1 — a clean verdict over 0 analysed pairs proves nothing", res.Coverage.AnalysedPairs)
	}
}

func TestShardFlagsParsing(t *testing.T) {
	var f shardFlags
	if err := f.Set("engine=statusgen/**,cmd/**"); err != nil {
		t.Fatal(err)
	}
	if len(f) != 1 || f[0].Name != "engine" || len(f[0].Files) != 2 {
		t.Fatalf("unexpected: %+v", f)
	}
	for _, bad := range []string{"engine", "=glob", "engine=", ""} {
		if err := f.Set(bad); err == nil {
			t.Errorf("malformed --shard %q accepted", bad)
		}
	}
}

func TestCheckParallelStreamShape(t *testing.T) {
	cases := []struct {
		name    string
		in      []ParallelStream
		wantSub string // "" = expect no problems
	}{
		{"absent is the default and never a finding", nil, ""},
		{"one shard is not a split", []ParallelStream{{Name: "solo", Files: []string{"a/**"}}}, "at least 2"},
		{"empty name", []ParallelStream{{Name: "", Files: []string{"a/**"}}, {Name: "b", Files: []string{"b/**"}}}, "empty name"},
		{"duplicate name", []ParallelStream{{Name: "a", Files: []string{"a/**"}}, {Name: "a", Files: []string{"b/**"}}}, "declared twice"},
		{"no globs", []ParallelStream{{Name: "a"}, {Name: "b", Files: []string{"b/**"}}}, "no files: globs"},
		{"valid", []ParallelStream{{Name: "a", Files: []string{"a/**"}}, {Name: "b", Files: []string{"b/**"}}}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := checkParallelStreamShape(c.in)
			joined := strings.Join(got, "\n")
			if c.wantSub == "" {
				if len(got) != 0 {
					t.Fatalf("want no problems, got %q", joined)
				}
				return
			}
			if !strings.Contains(joined, c.wantSub) {
				t.Fatalf("want a problem containing %q, got %q", c.wantSub, joined)
			}
		})
	}
}

func TestParallelStreamList(t *testing.T) {
	ok := []any{
		map[string]any{"name": "engine", "files": []any{"statusgen/**"}},
		map[string]any{"name": "docs", "files": []any{"docs/**", "README.md"}},
	}
	got, err := parallelStreamList(ok)
	if err != nil {
		t.Fatalf("parallelStreamList: %v", err)
	}
	if len(got) != 2 || got[0].Name != "engine" || len(got[1].Files) != 2 {
		t.Fatalf("unexpected parse: %+v", got)
	}

	// An unknown key is REJECTED, not ignored: a silently dropped scoping key
	// yields a shard running wider than its author declared.
	if _, err := parallelStreamList([]any{
		map[string]any{"name": "a", "files": []any{"a/**"}, "conflicts_with": "b"},
	}); err == nil {
		t.Error("unknown key was accepted; a dropped scoping key is a shard with the wrong scope")
	}
	for _, bad := range []any{
		"engine",
		[]any{"engine"},
		[]any{map[string]any{"files": []any{"a/**"}}},
		[]any{map[string]any{"name": 7, "files": []any{"a/**"}}},
		[]any{map[string]any{"name": "a", "files": "a/**"}},
	} {
		if _, err := parallelStreamList(bad); err == nil {
			t.Errorf("malformed parallel-streams accepted: %#v", bad)
		}
	}
	if got, err := parallelStreamList(nil); err != nil || got != nil {
		t.Errorf("absent must parse to nil with no error; got %v, %v", got, err)
	}
}

// End-to-end through the sub-command, including the case that matters most for
// backward compatibility: every brief on file today omits the field and must
// come back "none-declared", exit 0, with no split implied.
func TestRunShardcheckSubcommand(t *testing.T) {
	root := t.TempDir()
	briefNoSplit := filepath.Join(root, "brief-01-x.md")
	if err := os.WriteFile(briefNoSplit, []byte(briefV1(t, "")), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out := captureShardcheck(t, []string{"--brief", briefNoSplit, "--root", root})
	if code != shardExitClean {
		t.Fatalf("absent parallel-streams: exit %d, want 0\n%s", code, out)
	}
	if !strings.Contains(out, "none-declared") {
		t.Errorf("a brief with no split must say so, not imply an approved one: %q", out)
	}

	// A declared split over a real collision must exit 1 and print the class.
	root2 := writeTree(t, map[string]string{
		"statusgen/emit.go": "package main\n\nfunc emit(a int) {}\n",
		"statusgen/main.go": "package main\n\nfunc run() { emit(1) }\n",
	})
	split := "parallel-streams:\n  - {name: engine, files: [\"statusgen/emit.go\"]}\n  - {name: cli, files: [\"statusgen/main.go\"]}\n"
	briefSplit := filepath.Join(root2, "brief-01-x.md")
	if err := os.WriteFile(briefSplit, []byte(briefV1(t, split)), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out = captureShardcheck(t, []string{"--brief", briefSplit, "--root", root2})
	if code != shardExitFailed {
		t.Fatalf("collision: exit %d, want 1\n%s", code, out)
	}
	if !strings.Contains(out, "symbol-coupling") || !strings.Contains(out, "REFUSED") {
		t.Errorf("refusal must name the class and the verdict: %q", out)
	}
	if !strings.Contains(out, "ONE worker") {
		t.Errorf("refusal must tell the dispatcher what to do instead: %q", out)
	}

	// A missing brief is could-not-check, never clean.
	code, _ = captureShardcheck(t, []string{"--brief", filepath.Join(root, "nope.md"), "--root", root})
	if code != shardExitCouldNot {
		t.Errorf("unreadable brief: exit %d, want %d", code, shardExitCouldNot)
	}
	code, _ = captureShardcheck(t, []string{"--root", root})
	if code != shardExitCouldNot {
		t.Errorf("missing --brief: exit %d, want %d", code, shardExitCouldNot)
	}
}

// briefV1 renders a minimal valid brief-v1 file, with extra frontmatter lines
// spliced in.
func briefV1(t *testing.T, extra string) string {
	t.Helper()
	return "---\n" +
		"brief: example/01\n" +
		"title: t\n" +
		"wave: 1\n" +
		"depends: []\n" +
		"unblocks: []\n" +
		"effort: L\n" +
		"gate: model\n" +
		"risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n" +
		"issues: []\n" +
		"schema: brief-v1\n" +
		"authored: 2026-08-13 by test\n" +
		"sources: []\n" +
		extra +
		"---\n\n# Brief 01 — t\n"
}

func captureShardcheck(t *testing.T, args []string) (int, string) {
	t.Helper()
	dir := t.TempDir()
	outPath := filepath.Join(dir, "out")
	out, err := os.Create(outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	errF, err := os.Create(filepath.Join(dir, "err"))
	if err != nil {
		t.Fatal(err)
	}
	defer errF.Close()
	code := runShardcheck(args, out, errF)
	out.Sync()
	errF.Sync()
	rawOut, _ := os.ReadFile(outPath)
	rawErr, _ := os.ReadFile(filepath.Join(dir, "err"))
	return code, string(rawOut) + string(rawErr)
}
