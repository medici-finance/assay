package main

// principal_class_test.go — statusgen's half of the on-behalf-of CLASS guard (the desk
// half is tools/desk/internal/deskkit/principal_class_test.go; the two modules share no
// code, so each guards its own tree).
//
// THE DEFECT CLASS. An on-behalf-of annotation rendered by any route other than the one
// visibility-aware renderer (principal.go's onBehalfOfSuffix, which names the login only on
// a repo the roster states is `:private`) — a hand-built `on-behalf-of human:<login>`
// string, or a renderer call whose repo argument is not the repo the row lands in. Either
// puts the roster login into a public repo's Evidence table, where it stays in history.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// witnessOnBehalfOfLiteralRe matches a literal that spells the trailer key or the witness
// marker. Prose naming the mechanism ("on-behalf-of principal") does not match.
var witnessOnBehalfOfLiteralRe = regexp.MustCompile(`(?i)on-behalf-of(:|\s+human:)`)

// witnessOnBehalfOfCallSites is the ALLOW-LIST of every onBehalfOfSuffix call, keyed
// `<file>:<enclosing func>:onBehalfOfSuffix(<repo argument>)`. A new caller must be added
// here after confirming its repo argument names the repo the row is written INTO.
var witnessOnBehalfOfCallSites = map[string]int{
	"verifyrun.go:row:onBehalfOfSuffix(w.Repo)": 1,
}

// scanWitnessOnBehalfOf parses every non-test Go file directly in dir and returns the
// renderer call sites and the violations: a trailer literal outside principal.go, or a
// literal repo handed to the renderer.
func scanWitnessOnBehalfOf(dir string) (map[string]int, []string, error) {
	sites := map[string]int{}
	var findings []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, perr := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if perr != nil {
			return nil, nil, perr
		}
		inRenderer := name == "principal.go"
		// A literal handed straight to regexp.Compile/MustCompile is a MATCHER (the
		// corroboration lanes' reader), never a renderer — it cannot put text into a row.
		matchers := map[*ast.BasicLit]bool{}
		ast.Inspect(f, func(n ast.Node) bool {
			if c, ok := n.(*ast.CallExpr); ok {
				if sel, ok := c.Fun.(*ast.SelectorExpr); ok {
					if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "regexp" {
						for _, a := range c.Args {
							if lit, ok := a.(*ast.BasicLit); ok {
								matchers[lit] = true
							}
						}
					}
				}
			}
			return true
		})
		for _, decl := range f.Decls {
			fn := "<package>"
			if fd, ok := decl.(*ast.FuncDecl); ok {
				fn = fd.Name.Name
			}
			ast.Inspect(decl, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.BasicLit:
					if inRenderer || x.Kind != token.STRING || matchers[x] {
						return true
					}
					s, uerr := strconv.Unquote(x.Value)
					if uerr != nil {
						s = x.Value
					}
					if witnessOnBehalfOfLiteralRe.MatchString(s) {
						findings = append(findings, fmt.Sprintf("%s:%d: spells the on-behalf-of annotation in a "+
							"string literal — only principal.go's onBehalfOfSuffix may render it",
							name, fset.Position(x.Pos()).Line))
					}
				case *ast.CallExpr:
					id, ok := x.Fun.(*ast.Ident)
					if !ok || id.Name != "onBehalfOfSuffix" || inRenderer || len(x.Args) == 0 {
						return true
					}
					repoArg := x.Args[len(x.Args)-1]
					if lit, isLit := repoArg.(*ast.BasicLit); isLit {
						findings = append(findings, fmt.Sprintf("%s:%d: passes the literal %s as the witness "+
							"row's target repo", name, fset.Position(x.Pos()).Line, lit.Value))
					}
					sites[name+":"+fn+":onBehalfOfSuffix("+types.ExprString(repoArg)+")"]++
				}
				return true
			})
		}
	}
	sort.Strings(findings)
	return sites, findings, nil
}

func diffWitnessOnBehalfOfSites(observed, allowed map[string]int) []string {
	var problems []string
	for k, n := range observed {
		if allowed[k] != n {
			problems = append(problems, fmt.Sprintf("on-behalf-of renderer call site %s: %d in the tree, %d "+
				"allow-listed — confirm the repo argument is the repo the row lands in, then update "+
				"witnessOnBehalfOfCallSites", k, n, allowed[k]))
		}
	}
	for k := range allowed {
		if _, ok := observed[k]; !ok {
			problems = append(problems, "allow-listed renderer call site "+k+" no longer exists — remove it")
		}
	}
	sort.Strings(problems)
	return problems
}

// TestWitnessOnBehalfOfRenderedOnlyThroughTheRenderer is the class guard over this package.
func TestWitnessOnBehalfOfRenderedOnlyThroughTheRenderer(t *testing.T) {
	sites, findings, err := scanWitnessOnBehalfOf(".")
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) == 0 {
		t.Fatal("found no onBehalfOfSuffix call sites — this guard is looking in the wrong place")
	}
	for _, f := range findings {
		t.Error(f)
	}
	for _, p := range diffWitnessOnBehalfOfSites(sites, witnessOnBehalfOfCallSites) {
		t.Error(p)
	}
}

// TestWitnessOnBehalfOfClassGuardCatchesPlantedInstances is the guard's own control: each
// planted shape must be reported, and the permitted ones must not.
func TestWitnessOnBehalfOfClassGuardCatchesPlantedInstances(t *testing.T) {
	dir := t.TempDir()
	write := func(name, src string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("principal.go", "package main\n\nfunc onBehalfOfSuffix(token, repo string) string { return \"on-behalf-of human:\" + repo }\n")
	write("verifyrun.go", "package main\n\ntype witness struct{ Runner, Repo string }\n\n"+
		"func (w witness) row() string { return onBehalfOfSuffix(w.Runner, w.Repo) }\n")
	write("planted.go", "package main\n\n"+
		"func cellLiteral(login string) string { return \"(on-behalf-of human:\" + login + \")\" }\n\n"+
		"func cellFixed(tok string) string { return onBehalfOfSuffix(tok, \"example-org/one\") }\n")
	write("prose.go", "package main\n\nimport \"regexp\"\n\nconst msg = \"no on-behalf-of principal\"\n\n"+
		"var reader = regexp.MustCompile(`(?i)on-behalf-of:?\\s+human:(\\S+)`)\n")

	sites, findings, err := scanWitnessOnBehalfOf(dir)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(findings, "\n")
	for _, want := range []string{
		"planted.go:3: spells the on-behalf-of annotation",
		"planted.go:5: passes the literal \"example-org/one\"",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("planted instance not reported: want %q; findings:\n%s", want, joined)
		}
	}
	if len(findings) != 2 {
		t.Errorf("findings = %d, want exactly the 2 planted shapes:\n%s", len(findings), joined)
	}
	diff := strings.Join(diffWitnessOnBehalfOfSites(sites, witnessOnBehalfOfCallSites), "\n")
	if !strings.Contains(diff, "planted.go:cellFixed:onBehalfOfSuffix(\"example-org/one\")") {
		t.Errorf("the unlisted planted call site was not reported; got:\n%s", diff)
	}
	if strings.Contains(diff, "verifyrun.go:row") {
		t.Errorf("the allow-listed call site was reported; got:\n%s", diff)
	}
}
