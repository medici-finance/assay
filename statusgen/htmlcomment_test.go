package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// strayOpener is prose that quotes a comment opener literally, mid-line, with no
// closer — the #1939 shape (a brief quoting a wire-format marker).
const strayOpener = "The marker literal is \"<!-- assay:example:v1\" in the source.\n\n"

func TestStripRowComments(t *testing.T) {
	cases := []struct {
		name, in, want string
		at             int
	}{
		{"none", "a\nb\n", "a\nb\n", -1},
		{"closed", "a <!-- c --> b\n", "a  b\n", -1},
		{"multiline closed", "a\n<!-- x\ny -->\nb\n", "a\n\nb\n", -1},
		{"unterminated kept", "a\n<!-- x\nb\n", "a\n<!-- x\nb\n", 2},
		{"closed then unterminated", "<!-- c -->a <!-- x\nb\n", "a <!-- x\nb\n", 12},
		{"nested opener is closed", "a <!-- x <!-- y --> b\n", "a  b\n", -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, at := stripRowComments(tc.in)
			if got != tc.want || at != tc.at {
				t.Fatalf("stripRowComments(%q) = (%q, %d), want (%q, %d)", tc.in, got, at, tc.want, tc.at)
			}
			if at >= 0 && !strings.HasPrefix(tc.in[at:], "<!--") {
				t.Fatalf("offset %d does not point at the opener in %q", at, tc.in)
			}
		})
	}
}

// TestLaterWitnessAfterOpener is the #1939 regression: a witness table that
// follows an unterminated opener is still parsed by every row reader. On the
// unfixed code the opener swallowed the table, so the row read as having no
// witness and every other reader saw an empty section.
//
// Both opener shapes are covered. A mid-line opener is literal text when the
// page renders, so reading past it matches the page. A line-start opener opens
// an HTML block that hides the rest of the section when rendered, so for that
// shape the readers deliberately see rows the page hides; the lint PROBLEM
// (TestRunLintRefusesOpener) and the closure verbs' refusal
// (TestClosureVerbsRefuseOpener) are what stop it landing.
func TestLaterWitnessAfterOpener(t *testing.T) {
	const verify = "| # | Command | Expect |\n|---|---------|--------|\n| 1 | `true` | exit 0 |\n"
	witness := witnessHeader + "\n| 1 | `true` | pass exit=0 | sha256:aaaaaaaaaaaa | 2026-09-22 | sample-verifier |\n"
	for _, tc := range []struct{ name, opener string }{{"mid-line", strayOpener}, {"line-start", lineStartOpener}} {
		t.Run(tc.name, func(t *testing.T) {
			evidence := tc.opener + witness
			findings := checkWitnesses(verify, evidence)
			if len(findings) != 1 || findings[0].State != statePass {
				t.Errorf("checkWitnesses after a stray opener = %+v, want row 1 pass", findings)
			}
			if rows := parseEvidenceRows(evidence); len(rows["1"]) != 1 {
				t.Errorf("parseEvidenceRows after a stray opener = %v, want row 1", rows)
			}
			if !evidenceHasIndependentRow(evidence) {
				t.Error("evidenceHasIndependentRow did not see the independent row after a stray opener")
			}
			if d, r := evidenceVerifierInfo(evidence); d != "2026-09-22" || r != "sample-verifier" {
				t.Errorf("evidenceVerifierInfo after a stray opener = (%q, %q)", d, r)
			}
			unrun := tc.opener + "| # | Command | Result |\n|---|---|---|\n| 1 | `true` | UNRUN — no runner |\n"
			if got := unrunRowsText(unrun); !strings.Contains(got, "UNRUN — no runner") {
				t.Errorf("unrunRowsText after a stray opener = %q", got)
			}
			if what := unterminatedCommentIn(verify, evidence); !strings.Contains(what, "no closing") {
				t.Errorf("unterminatedCommentIn missed the %s opener: %q", tc.name, what)
			}
		})
	}
}

// The content check keeps its deliberate end-of-input reading: an unterminated
// opener still cannot pass as real content.
func TestContentCheckStillFailsClosed(t *testing.T) {
	if evidenceHasContent("<!-- contract, never closed\n| 1 | row |\n") {
		t.Fatal("an unterminated opener and what follows must not count as content")
	}
	if !evidenceHasContent("<!-- contract -->\n| 1 | row |\n") {
		t.Fatal("a row after a closed comment is content")
	}
}

func TestUnterminatedOpenerLint(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("brief-01-clean.md", "# One\n\n## Verify\n\n<!-- closed -->\n| 1 | `true` | exit 0 |\n\n## Evidence\n\n<!-- contract -->\n")
	write("brief-02-evidence.md", "# Two\n\n## Verify\n\n| 1 | `true` | exit 0 |\n\n## Evidence\n\n"+strayOpener+"| 1 | row |\n")
	write("brief-03-verify.md", "# Three\n\n## Verify (executable)\n\nsee <!-- note\n| 1 | `true` | exit 0 |\n\n## Evidence\n\n")
	write("brief-04-outside.md", "# Four\n\n## Task\n\nquotes <!-- here\n\n## Verify\n\n| 1 | `true` | exit 0 |\n")

	got := unterminatedCommentProblems([]*Stream{{Name: "sample", Dir: dir}})
	if len(got) != 2 {
		t.Fatalf("want 2 problems (brief-02 Evidence, brief-03 Verify), got %d: %q", len(got), got)
	}
	if !strings.Contains(got[0], "brief-02-evidence.md") || !strings.Contains(got[0], "`## Evidence`") || !strings.Contains(got[0], "<!-- assay:example:v1") {
		t.Errorf("brief-02 problem does not name the Evidence section and the opener: %s", got[0])
	}
	if !strings.Contains(got[1], "brief-03-verify.md") || !strings.Contains(got[1], "`## Verify`") {
		t.Errorf("brief-03 problem does not name the Verify section: %s", got[1])
	}
}

// commentStripAllowed is the allow-list for the end-of-input strip: htmlCommentRe
// may be used ONLY by the content check. A row reader that uses it drops
// everything after a stray opener (#1939) — it must use stripRowComments.
var commentStripAllowed = map[string]bool{"evidenceHasContent": true}

// endOfInputStripRe matches a regexp source whose comment pattern may end at
// end of input: a `-->` alternated with `$` or `\z`.
var endOfInputStripRe = regexp.MustCompile(`-->\s*\|\s*(\$|\\z)|(\$|\\z)\s*\|\s*-->`)

// commentStripSites returns "file:func" for every use of htmlCommentRe outside
// the allow-list, plus every string literal carrying an end-of-input comment
// pattern outside htmlCommentRe's own declaration.
func commentStripSites(fset *token.FileSet, files map[string]*ast.File) []string {
	var bad []string
	for name, f := range files {
		for _, decl := range f.Decls {
			fn, isFunc := decl.(*ast.FuncDecl)
			owner := "(package scope)"
			if isFunc {
				owner = fn.Name.Name
			}
			ast.Inspect(decl, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.ValueSpec:
					for _, id := range x.Names {
						if id.Name == "htmlCommentRe" {
							return false // the one declaration of the end-of-input pattern
						}
					}
				case *ast.Ident:
					if x.Name == "htmlCommentRe" && !(isFunc && commentStripAllowed[owner]) {
						bad = append(bad, name+":"+owner)
					}
				case *ast.BasicLit:
					if x.Kind == token.STRING && strings.Contains(x.Value, "<!--") && endOfInputStripRe.MatchString(x.Value) {
						bad = append(bad, name+":"+owner+" (literal "+x.Value+")")
					}
				}
				return true
			})
		}
	}
	sort.Strings(bad)
	return bad
}

// TestCommentStripSitesAllowList is the class guard for #1939: no reader other
// than the content check may strip an unterminated comment to end of input.
func TestCommentStripSitesAllowList(t *testing.T) {
	fset := token.NewFileSet()
	pkgFiles, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]*ast.File{}
	for _, p := range pkgFiles {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files[p] = f
	}
	if len(files) < 50 {
		t.Fatalf("parsed only %d non-test files — the scan is not looking at the package", len(files))
	}
	if bad := commentStripSites(fset, files); len(bad) != 0 {
		t.Fatalf("end-of-input comment strip outside the content check (use stripRowComments): %v", bad)
	}

	// Positive control: a planted reader of each shape must be flagged.
	const plant = `package main
func plantedReader(s string) string { return htmlCommentRe.ReplaceAllString(s, "") }
var plantedRe = regexp.MustCompile(` + "`(?s)<!--.*?(?:-->|$)`" + `)
`
	pf, err := parser.ParseFile(fset, "planted.go", plant, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := commentStripSites(fset, map[string]*ast.File{"planted.go": pf})
	if len(got) != 2 || !strings.HasPrefix(got[0], "planted.go:(package scope) (literal") || got[1] != "planted.go:plantedReader" {
		t.Fatalf("positive control: guard did not flag both planted sites, got %v", got)
	}
}

// lineStartOpener opens an HTML block at the start of a line and never closes
// it. Rendered, it hides every row after it; the row readers read past it. The
// lint PROBLEM is what stops that shape from landing (#1939).
const lineStartOpener = "<!-- draft, hidden when rendered\n\n"

// TestRunLintRefusesOpener pins the lint WIRING end to end: `statusgen --lint`
// over a tree whose Evidence has a line-start unterminated opener ahead of a
// witness table exits non-zero with the "no closing" PROBLEM. The control (the
// same comment, closed) lints clean, so the PROBLEM is what turns the exit red.
func TestRunLintRefusesOpener(t *testing.T) {
	witness := witnessTableFor(witnessRowOnePass, witnessRowTwoPass)

	var code int
	control := vcFixture(t, "implemented", "—", "<!-- draft, closed -->\n\n"+witness)
	stderr := captureStderr(t, func() { code = run(control, "lint", nil, nil, "") })
	if code != 0 || strings.Contains(stderr, "no closing") {
		t.Fatalf("control (closed comment) must lint clean, got exit %d:\n%s", code, stderr)
	}

	root := vcFixture(t, "implemented", "—", lineStartOpener+witness)
	stderr = captureStderr(t, func() { code = run(root, "lint", nil, nil, "") })
	if code == 0 {
		t.Errorf("--lint over a line-start unterminated opener exited 0, want non-zero:\n%s", stderr)
	}
	if !strings.Contains(stderr, "PROBLEM") || !strings.Contains(stderr, "no closing") || !strings.Contains(stderr, "brief-01-closure.md") {
		t.Errorf("--lint did not report the unterminated opener as a PROBLEM:\n%s", stderr)
	}
}

// runToFiles runs a verb with stdout redirected to a temp file and returns the
// exit code and what it wrote there.
func runToFiles(t *testing.T, fn func(stdout, stderr *os.File) int) (int, string) {
	t.Helper()
	dir := t.TempDir()
	out, err := os.Create(filepath.Join(dir, "out"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	errF, err := os.Create(filepath.Join(dir, "err"))
	if err != nil {
		t.Fatal(err)
	}
	defer errF.Close()
	code := fn(out, errF)
	raw, err := os.ReadFile(out.Name())
	if err != nil {
		t.Fatal(err)
	}
	return code, string(raw)
}

// TestClosureVerbsRefuseOpener: `verifyclosure` and `verifyrun --check` do not
// run the lint, so each refuses a witness table behind a line-start
// unterminated opener itself. The control (same comment, closed) passes, so the
// refusal is the opener's doing. `brief --check-verified` is covered by
// TestBriefInfoCheckVerified.
func TestClosureVerbsRefuseOpener(t *testing.T) {
	witness := witnessTableFor(witnessRowOnePass, witnessRowTwoPass)
	for _, tc := range []struct {
		name, evidence string
		refused        bool
	}{
		{"control", "<!-- draft, closed -->\n\n" + witness, false},
		{"line-start", lineStartOpener + witness, true},
		{"mid-line", strayOpener + witness, true},
	} {
		t.Run("verifyclosure/"+tc.name, func(t *testing.T) {
			root := vcFixture(t, "verified", stamped, tc.evidence)
			code, out := runToFiles(t, func(o, e *os.File) int {
				return runVerifyclosure([]string{"--root", root, "--brief", "vc/01"}, o, e)
			})
			if !tc.refused {
				if code != verifyclosureExitAccepted {
					t.Fatalf("control exit = %d, want accepted: %q", code, out)
				}
				return
			}
			if code != verifyclosureExitNotAccepted || !strings.Contains(out, "no closing") {
				t.Fatalf("exit = %d, want NOT accepted naming the opener: %q", code, out)
			}
		})
		t.Run("verifyrun-check/"+tc.name, func(t *testing.T) {
			root := vcFixture(t, "verified", stamped, tc.evidence)
			path := filepath.Join(root, "docs", "streams", "vc", "brief-01-closure.md")
			code, out := runToFiles(t, func(o, e *os.File) int {
				return runVerifyrun([]string{"--check", path}, o, e)
			})
			if !tc.refused {
				if code != verifyrunExitPass {
					t.Fatalf("control exit = %d, want pass: %q", code, out)
				}
				return
			}
			if code != verifyrunExitCouldNot || !strings.Contains(out, "no closing") {
				t.Fatalf("exit = %d, want a could-not refusal naming the opener: %q", code, out)
			}
		})
	}
}

// witnessCallersAllowed: checkWitnesses may be called only from the closure
// helper (which refuses an unterminated opener first) and from the lint's own
// witness gate (which runs beside the #1939 lint PROBLEM). A new verb that calls
// it directly would skip the refusal.
var witnessCallersAllowed = map[string]bool{"closureWitnesses": true, "witnessGateChecks": true}

// witnessCallSites returns "file:func" for every call of checkWitnesses outside
// the allow-list.
func witnessCallSites(files map[string]*ast.File) []string {
	var bad []string
	for name, f := range files {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			ast.Inspect(fn, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "checkWitnesses" && !witnessCallersAllowed[fn.Name.Name] {
					bad = append(bad, name+":"+fn.Name.Name)
				}
				return true
			})
		}
	}
	sort.Strings(bad)
	return bad
}

// TestWitnessCallerAllowList is the class guard for the closure refusal.
func TestWitnessCallerAllowList(t *testing.T) {
	fset := token.NewFileSet()
	pkgFiles, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]*ast.File{}
	for _, p := range pkgFiles {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files[p] = f
	}
	if len(files) < 50 {
		t.Fatalf("parsed only %d non-test files — the scan is not looking at the package", len(files))
	}
	if bad := witnessCallSites(files); len(bad) != 0 {
		t.Fatalf("checkWitnesses called outside closureWitnesses (use it, so an unterminated opener is refused): %v", bad)
	}

	// Positive control: a planted direct caller must be flagged.
	const plant = "package main\nfunc plantedVerb(v, e string) int { return len(checkWitnesses(v, e)) }\n"
	pf, err := parser.ParseFile(fset, "planted.go", plant, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := witnessCallSites(map[string]*ast.File{"planted.go": pf}); len(got) != 1 || got[0] != "planted.go:plantedVerb" {
		t.Fatalf("positive control: guard did not flag the planted caller, got %v", got)
	}
}
