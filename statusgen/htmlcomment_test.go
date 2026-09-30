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
func TestLaterWitnessAfterOpener(t *testing.T) {
	const verify = "| # | Command | Expect |\n|---|---------|--------|\n| 1 | `true` | exit 0 |\n"
	witness := witnessHeader + "\n| 1 | `true` | pass exit=0 | sha256:aaaaaaaaaaaa | 2026-09-22 | sample-verifier |\n"
	evidence := strayOpener + witness

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
	unrun := strayOpener + "| # | Command | Result |\n|---|---|---|\n| 1 | `true` | UNRUN — no runner |\n"
	if got := unrunRowsText(unrun); !strings.Contains(got, "UNRUN — no runner") {
		t.Errorf("unrunRowsText after a stray opener = %q", got)
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
