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
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
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
		{"a call-shaped identifier", "Break `check()` then `go test ./...`"},
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
	s := &Stream{Name: "demo", Root: root, Dir: dir, Briefs: []Brief{{Num: "01", Status: "done"}}}
	var hit bool
	for _, n := range unfailableRowNotices([]*Stream{s}) {
		if strings.Contains(n, "["+proseLedRule+"]") && strings.Contains(n, "Verify row 1") {
			hit = true
		}
	}
	if !hit {
		t.Errorf("a prose-led row must surface as a [%s] NOTICE (closed briefs included), got %v", proseLedRule, unfailableRowNotices([]*Stream{s}))
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

// codeSpanCallAllowList is every non-test function permitted to call codeSpan.
// verifyCommand is THE Verify-cell lift; the others read a span for a purpose
// that is not "the command to run/lint" (a witness row's own Command cell, the
// lint's message text naming the span it flagged). A new entry needs the same
// justification — a site that lifts a Verify command must call verifyCommand.
var codeSpanCallAllowList = map[string]string{
	"verifyCommand":    "the one Verify-cell lift (#1805)",
	"witnessCommandOf": "reads a WITNESS row's Command cell, which verifyrun wrote as exactly one span",
	"rowFindings":      "names the flagged first span in the prose-led-command NOTICE text only",
}

// codeSpanCallers parses every non-test .go file in dir and returns
// "file:func" for each function body that calls codeSpan.
func codeSpanCallers(t *testing.T, dir string) []string {
	t.Helper()
	fset := token.NewFileSet()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
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
				if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "codeSpan" {
					out = append(out, n+":"+fn.Name.Name)
				}
				return true
			})
		}
	}
	sort.Strings(out)
	return out
}

// TestVerifyCommandLiftHasOneChokePoint — the #1805 CLASS guard: "a site lifts
// the command out of a Verify Command cell by a rule other than verifyCommand's"
// (the verifyrun first-span lift was one; the check:ci re-execution lane handing
// the RAW cell to the shell was a second). Any codeSpan caller outside the
// allow-list fails here, naming the site. The positive control proves the walker
// still sees calls, so a broken matcher cannot report clean.
func TestVerifyCommandLiftHasOneChokePoint(t *testing.T) {
	callers := codeSpanCallers(t, ".")
	seen := map[string]bool{}
	for _, c := range callers {
		fn := c[strings.Index(c, ":")+1:]
		seen[fn] = true
		if _, ok := codeSpanCallAllowList[fn]; !ok {
			t.Errorf("%s calls codeSpan directly — lift a Verify command through verifyCommand (the #1805 choke point), or add the site to codeSpanCallAllowList with the reason it is not a Verify-command lift", c)
		}
	}
	// Positive control: the choke point itself must be visible to the walker.
	if !seen["verifyCommand"] {
		t.Errorf("positive control: the walker no longer sees verifyCommand's codeSpan call (callers: %v) — the guard is blind", callers)
	}
}
