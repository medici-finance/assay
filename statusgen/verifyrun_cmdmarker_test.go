package main

// Issue #1805 — `statusgen verifyrun` executed the FIRST code span of a prose
// Command cell, so a row whose prose mentions a token (a function, a file, a
// namespace, a label) ahead of the command ran the mention: exit 127, and the
// check the row describes was never witnessed.
//
// These tests pin both halves of the fix:
//
//   - the explicit `cmd:` marker, which verifyCommand (and so verifyrun, the
//     check:ci re-execution lane, newbrief and the row lint) prefers over the
//     first span;
//   - the `prose-led-command` lint NOTICE, which flags the #1805 shapes and
//     leaves ordinary command-first rows alone.
//
// Deliberately written against functions that existed BEFORE the fix
// (briefVerifyRows, rowFindings, checkWitnesses, runTranscribeVerdict) and
// against rule tags as string literals, so the pre-fix tree compiles and these
// tests fail on their assertions — the fail-first run the PR body quotes.
//
// The class guard (TestVerifyCommandLiftHasOneChokePoint) is the other half:
// it fails when any new site lifts a command out of a cell with codeSpan
// directly, instead of through verifyCommand. Its mutation spec is
// verifyrun-cmdmarker-mutations.json.

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

const proseLedRule = "prose-led-command"

func verifyTable(rows ...string) string {
	return "| # | Command | Expect |\n|---|---------|--------|\n" + strings.Join(rows, "\n") + "\n"
}

// TestVerifyRunPrefersCmdMarker — the marked span is the command, wherever it
// sits in the cell, and however many mentions precede it.
func TestVerifyRunPrefersCmdMarker(t *testing.T) {
	cases := []struct{ name, cell, want string }{
		{"mention-led prose (desk-tools/18 r11 shape)",
			"In `PublicRepoGate` (`tools/desk/internal/deskkit/repovis.go`) change `if a {` to `if true {`, then `cmd: cd tools/desk && go test ./internal/deskkit/ -count=1`; restore",
			"cd tools/desk && go test ./internal/deskkit/ -count=1"},
		{"marker as the first span", "`cmd: go test ./x -run '^TestY$' -count=1` (post apply)", "go test ./x -run '^TestY$' -count=1"},
		{"no space after the marker", "see `x.go`: `cmd:true`", "true"},
		{"double-backtick fence", "the `Foo` path: ``cmd: echo `date` >/dev/null``", "echo `date` >/dev/null"},
		{"escaped pipe survives", "In `Foo`: `cmd: grep -c x f \\| head -1`", "grep -c x f \\| head -1"},
		{"first marker wins", "`cmd: true` or else `cmd: false`", "true"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rows := briefVerifyRows(verifyTable("| 1 | " + c.cell + " | exit 0 |"))
			if len(rows) != 1 {
				t.Fatalf("want 1 row, got %d", len(rows))
			}
			if rows[0].Command != c.want {
				t.Errorf("lifted command = %q, want the marked span %q", rows[0].Command, c.want)
			}
		})
	}
}

// TestVerifyRunNoMarkerUnchanged — a cell with no marked span lifts exactly the
// first span, as before #1805: this is the compatibility guarantee that keeps
// every witness already on main at its verdict. It also pins that neither a
// GitHub Actions `run:` quotation nor a bare `cmd:` mention is a marker.
func TestVerifyRunNoMarkerUnchanged(t *testing.T) {
	cases := []struct{ name, cell, want string }{
		{"one span", "`go test ./...`", "go test ./..."},
		{"span then parenthetical", "`make test` (post out-of-repo apply)", "make test"},
		{"prose then command", "desk-tools loop includes both: `grep -oE -e a -e b f \\| sort -u \\| wc -l`", "grep -oE -e a -e b f \\| sort -u \\| wc -l"},
		{"mention-led, unmarked, keeps legacy lift", "In `PublicRepoGate` change it, then `go test ./x`", "PublicRepoGate"},
		{"no code span at all", "go test ./...", "go test ./..."},
		{"a `run:` workflow quotation is not a marker", "a planted `run: echo hi` line, then `grep -c run f`", "run: echo hi"},
		{"a bare `cmd:` mention is not a marker", "the `cmd:` marker is optional: `true`", "cmd:"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rows := briefVerifyRows(verifyTable("| 1 | " + c.cell + " | exit 0 |"))
			if len(rows) != 1 {
				t.Fatalf("want 1 row, got %d", len(rows))
			}
			if rows[0].Command != c.want {
				t.Errorf("lifted command = %q, want the legacy first-span lift %q", rows[0].Command, c.want)
			}
			if got := codeSpan(c.cell); got != c.want {
				t.Errorf("fixture drift: codeSpan(%q) = %q, want %q", c.cell, got, c.want)
			}
		})
	}
}

// TestCheckWitnessesMarkedRowRoundTrips — a witness verifyrun writes for a
// marked row records the lifted command, and --check matches it back to the
// row (so a marked row can reach `pass`, not a perpetual stale mismatch).
func TestCheckWitnessesMarkedRowRoundTrips(t *testing.T) {
	verify := verifyTable("| 1 | In `PublicRepoGate` run `cmd: go test ./x -count=1` | exit 0 |")
	rows := briefVerifyRows(verify)
	w := witness{ID: "1", Command: rows[0].Command, State: statePass, Exit: 0, OutHash: "0123456789ab",
		Date: "2026-09-27", Runner: "human:alex", Tree: "0123456789ab"}
	evidence := witnessHeader + "\n" + w.row() + "\n"
	fs := checkWitnesses(verify, evidence)
	if len(fs) != 1 || fs[0].State != statePass {
		t.Fatalf("marked row + its own witness must check pass, got %+v", fs)
	}
	if !strings.Contains(w.row(), "| `go test ./x -count=1` |") {
		t.Errorf("witness must record the lifted command, got %s", w.row())
	}
}

func hasRule(fs []rowFinding, rule string) bool {
	for _, f := range fs {
		if f.rule == rule {
			return true
		}
	}
	return false
}

// TestProseLedCommandLintFlagsIssueShapes — every #1805 shape (the cells are
// the reported rows' shapes, trimmed) raises the NOTICE.
func TestProseLedCommandLintFlagsIssueShapes(t *testing.T) {
	cases := []struct{ name, cell string }{
		{"desk-tools/18 r11 — function name first",
			"**Mutation demonstration.** In `PublicRepoGate` (`tools/desk/internal/deskkit/repovis.go`) change `if a {` to `if true {` — then `cd tools/desk && go test ./internal/deskkit/ -count=1`; restore"},
		{"windows-port/01 r2 — extension first",
			"The two assets carry `.exe`: `grep -c 'x-amd64[.]exe' .github/workflows/release.yml`"},
		{"windows-port/01 r5 — stream/NN reference first",
			"**Dereferencing** (proves `windows-port/00`'s split landed): `cd statusgen && GOOS=windows go build -o /tmp/x.exe .`"},
		{"forge-neutral/13 r15 — camelCase function first",
			"In `runSupersededLane` (`tools/desk/cmd/deskclose/superseded.go`) disable the check — then `cd tools/desk && go test ./cmd/deskclose/... -count=1`; restore"},
		{"forge-neutral/14 r12 — lone word first, command later",
			"In the resolver, make it fall through to the `release-runner` path — then `cd tools/desk && go test ./cmd/deskrun/... -count=1`; restore"},
		{"forge-neutral/19 r3 — owner/repo first",
			"**FIXTURE REPO ONLY — never `example-org/tracker`.** On a throwaway fixture repo attempt `gh api repos/o/r/pulls/1/merge -X PUT`"},
		{"forge-neutral/19 r4 — owner/repo first, prose only",
			"**FIXTURE REPO ONLY — never `example-org/tracker`.** Same fixture, stub now posting `success`"},
		{"a .go file first", "Edit `repovis.go` then `go test ./...`"},
		{"a .go file first, no later command span", "Edit `repovis.go` and restore it"},
		{"a call-shaped identifier", "Break `check()` then `go test ./...`"},
		{"a bare `cmd:` mention ahead of the command", "`cmd:` `go test ./c`"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if fs := rowFindings(c.cell, "exit 0"); !hasRule(fs, proseLedRule) {
				t.Errorf("want a %s NOTICE for %q, got %+v", proseLedRule, c.cell, fs)
			}
		})
	}
}

// TestProseLedCommandLintSparesCommandRows — ordinary command-first rows, a
// single-span cell, and a marked row are NOT flagged.
func TestProseLedCommandLintSparesCommandRows(t *testing.T) {
	cases := []struct{ name, cell string }{
		{"one span", "`go test ./... -count=1`"},
		{"one bare-word span", "`make`"},
		{"one path span", "`docs/streams/x/verify.d/check.sh`"},
		{"one identifier-shaped span alone", "`PublicRepoGate`"},
		{"command then parenthetical", "`go test ./...` (post out-of-repo apply)"},
		{"prose then a real command", "desk-tools loop includes both: `grep -oE -e a -e b f \\| sort -u \\| wc -l`"},
		{"prose-led lone word, no later command", "run `make` from the root"},
		{"relative script in prose", "Run `./scripts/check.sh` from the root"},
		{"script path in prose", "Run `scripts/check.sh` from the root"},
		{"marked row with a mention first", "In `PublicRepoGate` change it, then `cmd: cd tools/desk && go test ./internal/deskkit/ -count=1`"},
		{"no code span", "go test ./..."},
		{"multi-word first span", "Build: `cd statusgen && go build .` then `file x`"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if fs := rowFindings(c.cell, "exit 0"); hasRule(fs, proseLedRule) {
				t.Errorf("an ordinary row must not raise %s: %q → %+v", proseLedRule, c.cell, fs)
			}
		})
	}
}

// TestCmdMarkerAmbiguousLint — two markers in one cell is its own NOTICE.
func TestCmdMarkerAmbiguousLint(t *testing.T) {
	if fs := rowFindings("`cmd: true` then `cmd: false`", "exit 0"); !hasRule(fs, "cmd-marker-ambiguous") {
		t.Errorf("two `cmd:` spans must raise cmd-marker-ambiguous, got %+v", fs)
	}
	if fs := rowFindings("`cmd: true`", "exit 0"); hasRule(fs, "cmd-marker-ambiguous") {
		t.Errorf("one `cmd:` span must not raise cmd-marker-ambiguous, got %+v", fs)
	}
}

// TestProseLedCommandLintReachesLintOutput — the rule is wired into the lint's
// NOTICE stream (unfailableRowNotices), not only into rowFindings.
func TestProseLedCommandLintReachesLintOutput(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	brief := "---\nbrief: demo/01\ntitle: Demo\nwave: 0\ndepends: []\nunblocks: []\neffort: S\ngate: model\n" +
		"risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\nissues: []\nschema: brief-v1\n" +
		"authored: 2026-09-27 by fixture\nsources: [\"fixture\"]\n---\n\n# Demo\n\n## Verify\n" +
		verifyTable("| 1 | In `PublicRepoGate` change it, then `go test ./x` | exit 0 |") +
		"\n## Evidence\n\n## Review\nGate: model.\n"
	path := filepath.Join(dir, "brief-01-demo.md")
	if err := os.WriteFile(path, []byte(brief), 0o644); err != nil {
		t.Fatal(err)
	}
	// An OPEN brief gets the per-row notice, naming the row.
	open := &Stream{Name: "demo", Root: root, Dir: dir, Briefs: []Brief{{Num: "01", Status: "todo"}}}
	var hit bool
	for _, n := range unfailableRowNotices([]*Stream{open}) {
		if strings.Contains(n, "["+proseLedRule+"]") && strings.Contains(n, "Verify row 1") {
			hit = true
		}
	}
	if !hit {
		t.Errorf("a prose-led row in an open brief must surface as a per-row [%s] NOTICE, got %v", proseLedRule, unfailableRowNotices([]*Stream{open}))
	}
	// A CLOSED brief is not rewritten, so its rows collapse into ONE summary
	// line (the gotest-run-vacuous precedent) — counted, never silently dropped.
	closed := &Stream{Name: "demo", Root: root, Dir: dir, Briefs: []Brief{{Num: "01", Status: "done"}}}
	var summary, perRow int
	for _, n := range unfailableRowNotices([]*Stream{closed}) {
		if !strings.Contains(n, "["+proseLedRule+"]") {
			continue
		}
		if strings.Contains(n, "Verify row 1") {
			perRow++
		}
		if strings.Contains(n, "1 Verify row(s) in 1 closed brief(s)") {
			summary++
		}
	}
	if summary != 1 || perRow != 0 {
		t.Errorf("a closed brief's prose-led rows must collapse into one summary [%s] NOTICE (summary=%d perRow=%d), got %v", proseLedRule, summary, perRow, unfailableRowNotices([]*Stream{closed}))
	}
}

// TestTranscribeVerdictCheckCIReexecutesLiftedCommand — the check:ci
// re-execution lane hands the runner the LIFTED command, never the raw cell.
// The raw cell of a backticked row is "`true`", which a shell reads as command
// substitution: it runs `true`, then runs its (empty) OUTPUT as the command.
func TestTranscribeVerdictCheckCIReexecutesLiftedCommand(t *testing.T) {
	for _, c := range []struct{ name, cell string }{
		{"backticked row", "`true`"},
		{"marked prose row", "Re-run the gate in `Foo`: `cmd: true`"},
	} {
		t.Run(c.name, func(t *testing.T) {
			scanWithRoster(t, verdictRoster())
			key := verdictTestKey(t)
			root := verdictBaseRepo(t, r6Armed)
			rel := writeVerdictBrief(t, root, "01", "model", false)
			abs := filepath.Join(root, rel)
			b, err := os.ReadFile(abs)
			if err != nil {
				t.Fatal(err)
			}
			nb := strings.Replace(string(b), "| 1 | check:ci | true | exit 0 |", "| 1 | check:ci | "+c.cell+" | exit 0 |", 1)
			if nb == string(b) {
				t.Fatal("fixture drift: the check:ci row was not found to rewrite")
			}
			if err := os.WriteFile(abs, []byte(nb), 0o644); err != nil {
				t.Fatal(err)
			}
			body := signVerdictBody(t, key, okPayload(okEntry(rel, "| 1 | check:ci | true | 0 | PASS | 2026-09-27 | verifier |")))
			list := fixtureLister(map[string][]ghIssue{verdictTestRepo: {{Number: 611}}}, "")
			resolve := fixtureVerdictResolver(map[string]verdictIssue{verdictTestRepo + "#611": {Author: verifierIdent, Body: body}})
			var got []string
			rec := func(_ string, command string) checkCIResult {
				got = append(got, command)
				return checkCIResult{Passed: true, Reason: "exit 0"}
			}
			out := captureRun(t, func() int {
				return runTranscribeVerdict(root, true, "", list, resolve, rec, noHealthHold, blessR6Resolver)
			})
			if len(got) != 1 || got[0] != "true" {
				t.Errorf("check:ci runner must receive the lifted command %q, got %q\n%s", "true", got, out.log)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Class guard — one choke point for lifting a Verify command
// ---------------------------------------------------------------------------

// liftPrimitiveAllowList is, per span-scanning primitive, every non-test
// function permitted to call it. verifyCommand is THE Verify-cell lift; every
// other entry reads spans for a purpose that is not "the command to run": a
// witness row's own Command cell, the lint's JUDGEMENT of a cell (never the
// text it then lints), or the marker scan itself. A new entry needs the same
// justification — a site that lifts a Verify command must call verifyCommand.
//
// rowFindings is deliberately NOT on any list (#1808 review, CR-1808-1): it
// lints whatever verifyCommand returns, and the NOTICE text names the span
// proseLedCommandWhy hands back, so it never calls a primitive itself. A
// function-keyed allow-list cannot tell a message-text call from a lift inside
// one function, so the lint function must hold no call at all.
var liftPrimitiveAllowList = map[string]map[string]string{
	"codeSpan": {
		"verifyCommand":    "the one Verify-cell lift (#1805)",
		"witnessCommandOf": "reads a WITNESS row's Command cell, which verifyrun wrote as exactly one span",
	},
	"codeSpans": {
		"proseLedCommandWhy": "judges whether the cell's first span is a mention; returns that span for the NOTICE text",
		"rawMarkerPresent":   "the lint's hidden-marker check: a raw marker the rendered scan does not honour",
	},
	"renderedCodeSpans": {
		"markedCommands":             "the honoured-marker scan verifyCommand consults",
		"markerOverridesCommandSpan": "the lint's check that a marker replaces a command-shaped first span",
	},
}

// primitiveCallers parses every non-test .go file in dir and returns, per
// primitive name in allow, "file:func" for each function body that calls it.
func primitiveCallers(t *testing.T, dir string, allow map[string]map[string]string) map[string][]string {
	t.Helper()
	fset := token.NewFileSet()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]string{}
	for _, e := range ents {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, n), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", n, err)
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(nd ast.Node) bool {
				call, ok := nd.(*ast.CallExpr)
				if !ok {
					return true
				}
				if id, ok := call.Fun.(*ast.Ident); ok {
					if _, watched := allow[id.Name]; watched {
						out[id.Name] = append(out[id.Name], n+":"+fn.Name.Name)
					}
				}
				return true
			})
		}
	}
	for k := range out {
		sort.Strings(out[k])
	}
	return out
}

// TestVerifyCommandLiftHasOneChokePoint — the #1805 CLASS guard: "a site lifts
// the command out of a Verify Command cell by a rule other than verifyCommand's"
// (the verifyrun first-span lift was one; the check:ci re-execution lane handing
// the RAW cell to the shell was a second). Any caller of a span-scanning
// primitive outside its allow-list fails here, naming the site. The positive
// controls prove the walker still sees calls, so a broken matcher cannot report
// clean.
func TestVerifyCommandLiftHasOneChokePoint(t *testing.T) {
	callers := primitiveCallers(t, ".", liftPrimitiveAllowList)
	for prim, sites := range callers {
		for _, c := range sites {
			fn := c[strings.Index(c, ":")+1:]
			if _, ok := liftPrimitiveAllowList[prim][fn]; !ok {
				t.Errorf("%s calls %s directly — lift a Verify command through verifyCommand (the #1805 choke point), or add the site to liftPrimitiveAllowList with the reason it is not a Verify-command lift", c, prim)
			}
		}
	}
	// Positive controls: the choke point and the marker scan must be visible.
	for prim, fn := range map[string]string{"codeSpan": "verifyCommand", "renderedCodeSpans": "markedCommands"} {
		seen := false
		for _, c := range callers[prim] {
			if strings.HasSuffix(c, ":"+fn) {
				seen = true
			}
		}
		if !seen {
			t.Errorf("positive control: the walker no longer sees %s's %s call (callers: %v) — the guard is blind", fn, prim, callers[prim])
		}
	}
}

// ---------------------------------------------------------------------------
// #1808 review — the marker must agree with the rendered table
// ---------------------------------------------------------------------------

type cmdMarkerVector struct {
	Name   string  `json:"name"`
	Cell   string  `json:"cell"`
	Marked *string `json:"marked"`
}

// loadCmdMarkerVectors reads the vector table tools/desk's executors are held
// to as well (tools/desk/internal/verifycmd reads the same file).
func loadCmdMarkerVectors(t *testing.T) []cmdMarkerVector {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "cmd-marker-vectors.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Vectors []cmdMarkerVector `json:"vectors"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Vectors) < 10 {
		t.Fatalf("positive control: only %d vectors loaded", len(doc.Vectors))
	}
	return doc.Vectors
}

// TestCmdMarkerSharedVectors — statusgen's marker scan and lift satisfy the
// shared table: the honoured marker where there is one, and the unchanged
// legacy first-span lift where there is none.
func TestCmdMarkerSharedVectors(t *testing.T) {
	for _, v := range loadCmdMarkerVectors(t) {
		t.Run(v.Name, func(t *testing.T) {
			m := markedCommands(v.Cell)
			switch {
			case v.Marked == nil && len(m) != 0:
				t.Errorf("markedCommands(%q) = %q, want no honoured marker", v.Cell, m)
			case v.Marked != nil && (len(m) == 0 || m[0] != *v.Marked):
				t.Errorf("markedCommands(%q) = %q, want first %q", v.Cell, m, *v.Marked)
			}
			want := codeSpan(v.Cell)
			if v.Marked != nil {
				want = *v.Marked
			}
			if got := verifyCommand(v.Cell); got != want {
				t.Errorf("verifyCommand(%q) = %q, want %q", v.Cell, got, want)
			}
		})
	}
}

// sr1808Probes are the security review's probe cells: a visible, failing,
// command-shaped first span, then a `cmd: true` the rendered table either
// shows as prose-adjacent code (probe 0) or does not show as code at all.
var sr1808Probes = []string{
	"`go test ./nonexistent-pkg-1808 -count=1` (see `cmd: true`)",
	"`go test ./nonexistent-pkg-1808 -count=1` <!-- `cmd: true` -->",
	"`go test ./nonexistent-pkg-1808 -count=1` then \\`cmd: true #\\`",
}

// TestHiddenMarkerKeepsFirstSpan — a marker the rendered cell does not show
// as code is not honoured: verifyrun runs the visible first span, and the lint
// says the marker was ignored.
func TestHiddenMarkerKeepsFirstSpan(t *testing.T) {
	for _, cell := range sr1808Probes[1:] {
		rows := briefVerifyRows(verifyTable("| 1 | " + cell + " | exit 0 |"))
		if len(rows) != 1 || rows[0].Command != "go test ./nonexistent-pkg-1808 -count=1" {
			t.Errorf("hidden marker must not replace the visible span: %q lifted %+v", cell, rows)
		}
		if fs := rowFindings(cell, "exit 0"); !hasRule(fs, "cmd-marker-not-honoured") {
			t.Errorf("want a cmd-marker-not-honoured NOTICE for %q, got %+v", cell, fs)
		}
	}
	if fs := rowFindings("In `Foo` then `cmd: true`", "exit 0"); hasRule(fs, "cmd-marker-not-honoured") {
		t.Errorf("an honoured marker must not raise cmd-marker-not-honoured, got %+v", fs)
	}
}

// TestCmdMarkerOverridesLint — a visible marker behind a command-shaped first
// span is honoured (the author wrote it) but NOTICEd, so the override is never
// silent; the shape the marker exists for (mentions first) is not.
func TestCmdMarkerOverridesLint(t *testing.T) {
	if fs := rowFindings(sr1808Probes[0], "exit 0"); !hasRule(fs, "cmd-marker-overrides-command") {
		t.Errorf("want cmd-marker-overrides-command for %q, got %+v", sr1808Probes[0], fs)
	}
	for _, cell := range []string{
		"In `PublicRepoGate` (`repovis.go`) change it, then `cmd: go test ./x -count=1`",
		"`cmd: true` then `go test ./x`",
		"`cmd: true`",
	} {
		if fs := rowFindings(cell, "exit 0"); hasRule(fs, "cmd-marker-overrides-command") {
			t.Errorf("%q must not raise cmd-marker-overrides-command, got %+v", cell, fs)
		}
	}
}

// TestRowLintJudgesMarkedCommand — CR-1808-1: the row rules judge the command
// the marker names, not the first span. Each cell's marked command trips an
// existing rule that its first span (`Foo`) does not.
func TestRowLintJudgesMarkedCommand(t *testing.T) {
	for _, c := range []struct{ cell, rule string }{
		{"In `Foo` change it, then `cmd: kubectl -n <ns> get pods`", ruleMetavar},
		{"In `Foo` change it, then `cmd: go test ./x -run TestY -count=1`", ruleGoTestRunVacuous},
	} {
		if fs := rowFindings(c.cell, "exit 0"); !hasRule(fs, c.rule) {
			t.Errorf("the marked command of %q must raise %s, got %+v", c.cell, c.rule, fs)
		}
	}
}

// TestProseLedRowNotExecuted — #1808 review A1: a prose-led row is recorded
// could-not-run WITHOUT executing its first span. The fixture's mention is
// `true`, which exits 0: executed, it would record a PASS for a check that
// never ran (the `gh` shape — `gh` with no arguments also exits 0). The same
// cell with the command marked runs the marked command.
func TestProseLedRowNotExecuted(t *testing.T) {
	rows := briefVerifyRows(verifyTable(
		"| 1 | Check `true` first, then run `go test ./nonexistent -count=1` | exit 0 |",
		"| 2 | Check `x` first, then `cmd: true` | exit 0 |",
	))
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %d", len(rows))
	}
	ws := runWitnesses(t.TempDir(), rows, "human:tester", "", "0000", "2026-09-30", 30*time.Second, false)
	if len(ws) != 2 {
		t.Fatalf("want 2 witnesses, got %d", len(ws))
	}
	if ws[0].State != stateCouldNotRun || ws[0].Exit != -1 || !strings.Contains(ws[0].Note, proseLedRule) {
		t.Errorf("a prose-led row must be could-not-run, not executed, with a %s note; got state=%s exit=%d note=%q",
			proseLedRule, ws[0].State, ws[0].Exit, ws[0].Note)
	}
	if ws[1].State != statePass || ws[1].Exit != 0 {
		t.Errorf("the marked row must execute its marked command and pass; got state=%s exit=%d note=%q",
			ws[1].State, ws[1].Exit, ws[1].Note)
	}
}

// TestCmdMarkerVacuousLint — #1808 review A3: a marked command that cannot
// fail passes whatever the tree holds; the lint NOTICEs it. A real command,
// and an unmarked cell, do not raise it.
func TestCmdMarkerVacuousLint(t *testing.T) {
	for _, cell := range []string{
		"`cmd: true`",
		"In `Foo` change it, then `cmd: exit  0`",
		"see `x.go`: `cmd::`",
		"`cmd: /usr/bin/true`",
	} {
		if fs := rowFindings(cell, "exit 0"); !hasRule(fs, "cmd-marker-vacuous") {
			t.Errorf("%q must raise cmd-marker-vacuous, got %+v", cell, fs)
		}
	}
	for _, cell := range []string{
		"`cmd: go test ./x -count=1`",
		"`cmd: true && go test ./x -count=1`",
		"`true`",
	} {
		if fs := rowFindings(cell, "exit 0"); hasRule(fs, "cmd-marker-vacuous") {
			t.Errorf("%q must not raise cmd-marker-vacuous, got %+v", cell, fs)
		}
	}
}

// TestWitnessRowRoundTripsBacktickCommand — #1808 review A4: a marked command
// that contains backticks is written into the witness row as ONE code span that
// witnessCommandOf lifts back whole, so --check matches it to its row. A
// command with no backtick keeps the single-backtick cell byte for byte.
func TestWitnessRowRoundTripsBacktickCommand(t *testing.T) {
	for _, cmd := range []string{
		"echo `date` >/dev/null",
		"echo ``a`` b",
		"`date`",
	} {
		w := witness{ID: "1", Command: cmd, State: statePass, Exit: 0, OutHash: "0123456789ab",
			Date: "2026-09-30", Runner: "human:alex", Tree: "0123456789ab"}
		if got := witnessCommandOf(w.row()); got != cmd {
			t.Errorf("witness row for %q lifts back %q; row: %s", cmd, got, w.row())
		}
	}
	w := witness{ID: "1", Command: "go test ./x -count=1", State: statePass, Exit: 0, OutHash: "0123456789ab",
		Date: "2026-09-30", Runner: "human:alex", Tree: "0123456789ab"}
	if !strings.Contains(w.row(), "| `go test ./x -count=1` |") {
		t.Errorf("a backtick-free command must keep the single-backtick cell, got %s", w.row())
	}
	// End to end: the double-fence marked row checks pass against its own witness.
	verify := verifyTable("| 1 | the `Foo` path: ``cmd: echo `date` >/dev/null`` | exit 0 |")
	rows := briefVerifyRows(verify)
	w = witness{ID: "1", Command: rows[0].Command, State: statePass, Exit: 0, OutHash: "0123456789ab",
		Date: "2026-09-30", Runner: "human:alex", Tree: "0123456789ab"}
	if fs := checkWitnesses(verify, witnessHeader+"\n"+w.row()+"\n"); len(fs) != 1 || fs[0].State != statePass {
		t.Errorf("a backtick-bearing marked row must check pass against its own witness, got %+v (row %s)", fs, w.row())
	}
}
