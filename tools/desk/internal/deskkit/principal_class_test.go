package deskkit

// principal_class_test.go — the CLASS guard for the on-behalf-of trailer.
//
// THE DEFECT CLASS. A writer renders the human an App write was on behalf of by some route
// other than the one visibility-aware resolver in principal.go — a hand-built
// `On-behalf-of: human:<login>` string, a reuse of OnBehalfOfPrefix to compose one, or a
// resolver call whose repo argument is not the repo the write actually lands in. Any of
// those puts the roster LOGIN on a public repo, where a posted review or a commit message
// cannot be taken back. The resolver itself is correct (principal_visibility_test.go); what
// this file guards is that NOTHING ELSE renders the trailer, and that every call into the
// resolver is one a reviewer has looked at.
//
// TestEveryWriteVerbNamesItsTarget (principal_visibility_test.go) already refuses an EMPTY
// repo argument. It cannot see a second renderer that never calls the resolver at all, a
// fixed literal repo, or a new call site whose repo expression names the wrong repo — this
// guard does, by parsing every non-test Go file in the module.

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

// onBehalfOfResolverFile is the one file allowed to spell or compose the trailer.
const onBehalfOfResolverFile = "internal/deskkit/principal.go"

// onBehalfOfEntryPoints are the resolver functions whose LAST argument is the target repo.
var onBehalfOfEntryPoints = map[string]bool{
	"AppendOnBehalfOf":       true,
	"OnBehalfOfLine":         true,
	"OnBehalfOfCommitSuffix": true,
	"ResolvePrincipal":       true,
}

// onBehalfOfTrailerLiteralRe matches a string literal that spells the trailer key or the
// witness-cell marker — the text a second renderer would have to contain. Prose that merely
// names the mechanism ("the on-behalf-of trailer", "on-behalf-of principal") does not match.
var onBehalfOfTrailerLiteralRe = regexp.MustCompile(`(?i)on-behalf-of(:|\s+human:)`)

// onBehalfOfCallSites is the ALLOW-LIST of every call into the resolver outside
// principal.go, keyed `<file>:<enclosing func>:<entry>(<repo argument>)`, with its count.
//
// Adding a writer means adding its line here — after confirming the repo argument is the
// repo the write LANDS in (not a home repo, not a fixed value, not the dispatcher's), since
// that argument alone decides whether the trailer names the login or the neutral name. A
// line with no matching call site is also a failure, so the list cannot drift into
// describing code that no longer exists.
var onBehalfOfCallSites = map[string]int{
	"cmd/deskevidence/deskevidence.go:cmdEvidence:OnBehalfOfCommitSuffix(repoSlug)":          1,
	"cmd/deskevidence/deskevidence.go:landEvidenceAsChange:OnBehalfOfCommitSuffix(repoSlug)": 1,
	"cmd/deskfile/deskfile.go:cmdAttach:AppendOnBehalfOf(*repo)":                             1,
	"cmd/deskfile/deskfile.go:cmdNew:AppendOnBehalfOf(*repo)":                                1,
	"cmd/deskflip/flip.go:flip:OnBehalfOfLine(repo)":                                         2,
	"cmd/deskpost/comment.go:runComment:AppendOnBehalfOf(repo)":                              1,
	"cmd/deskpost/review.go:postVerdictReview:AppendOnBehalfOf(repo)":                        1,
	"cmd/deskpr/deskpr.go:cmdCreate:AppendOnBehalfOf(facts.repo)":                            1,
	"cmd/deskpr/edit.go:cmdEdit:AppendOnBehalfOf(facts.repo)":                                1,
	"cmd/deskreply/deskreply.go:cmdReply:AppendOnBehalfOf(repo)":                             1,
	"cmd/deskreply/deskreply.go:cmdReply:OnBehalfOfLine(repo)":                               1,
	"cmd/deskreply/workpad.go:cmdWorkpadUpsert:AppendOnBehalfOf(repo)":                       1,
	"cmd/deskreply/workpad.go:cmdWorkpadUpsert:OnBehalfOfLine(repo)":                         1,
}

// onBehalfOfScan is what scanOnBehalfOfRenderers found under one module root.
type onBehalfOfScan struct {
	sites    map[string]int // allow-list keys, as observed
	findings []string       // renderer-monopoly and literal-repo violations
}

// scanOnBehalfOfRenderers parses every non-test Go file under root (skipping testdata,
// vendor and dot-directories) and reports (a) each call into the resolver, keyed the way
// onBehalfOfCallSites is, and (b) every place outside the resolver file that spells the
// trailer, references OnBehalfOfPrefix, or hands the resolver a literal repo.
func scanOnBehalfOfRenderers(root string) (onBehalfOfScan, error) {
	out := onBehalfOfScan{sites: map[string]int{}}
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if path != root && (name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return perr
		}
		scanOnBehalfOfFile(fset, rel, f, &out)
		return nil
	})
	sort.Strings(out.findings)
	return out, err
}

func scanOnBehalfOfFile(fset *token.FileSet, rel string, f *ast.File, out *onBehalfOfScan) {
	inResolver := rel == onBehalfOfResolverFile
	where := func(n ast.Node) string { return fmt.Sprintf("%s:%d", rel, fset.Position(n.Pos()).Line) }

	visit := func(fn string) func(ast.Node) bool {
		return func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.BasicLit:
				if inResolver || x.Kind != token.STRING {
					return true
				}
				s, err := strconv.Unquote(x.Value)
				if err != nil {
					s = x.Value
				}
				if onBehalfOfTrailerLiteralRe.MatchString(s) {
					out.findings = append(out.findings, where(x)+": spells the on-behalf-of trailer in a string "+
						"literal — only "+onBehalfOfResolverFile+" may render it; call AppendOnBehalfOf / "+
						"OnBehalfOfLine / OnBehalfOfCommitSuffix with the write's target repo instead")
				}
			case *ast.Ident:
				if !inResolver && x.Name == "OnBehalfOfPrefix" {
					out.findings = append(out.findings, where(x)+": references OnBehalfOfPrefix — composing the "+
						"trailer outside "+onBehalfOfResolverFile+" bypasses the visibility split")
				}
			case *ast.CallExpr:
				name := ""
				switch fun := x.Fun.(type) {
				case *ast.Ident:
					name = fun.Name
				case *ast.SelectorExpr:
					name = fun.Sel.Name
				}
				if inResolver || !onBehalfOfEntryPoints[name] || len(x.Args) == 0 {
					return true
				}
				repoArg := x.Args[len(x.Args)-1]
				if lit, ok := repoArg.(*ast.BasicLit); ok {
					out.findings = append(out.findings, where(x)+": passes the literal "+lit.Value+" as "+name+
						"'s target repo — the repo must be the one the write lands in, never a fixed value")
				}
				out.sites[rel+":"+fn+":"+name+"("+types.ExprString(repoArg)+")"]++
			}
			return true
		}
	}

	for _, decl := range f.Decls {
		fn := "<package>"
		if fd, ok := decl.(*ast.FuncDecl); ok {
			fn = fd.Name.Name
		}
		ast.Inspect(decl, visit(fn))
	}
}

// diffOnBehalfOfSites compares observed call sites against an allow-list and returns one
// line per disagreement: a site the list does not carry (a new, unreviewed writer) or a
// list entry whose count the tree no longer matches.
func diffOnBehalfOfSites(observed, allowed map[string]int) []string {
	var problems []string
	for k, n := range observed {
		if want, ok := allowed[k]; !ok {
			problems = append(problems, fmt.Sprintf("unlisted on-behalf-of call site %s (x%d) — confirm its repo "+
				"argument is the repo the write lands in, then add it to onBehalfOfCallSites", k, n))
		} else if want != n {
			problems = append(problems, fmt.Sprintf("on-behalf-of call site %s: %d in the tree, %d allow-listed", k, n, want))
		}
	}
	for k := range allowed {
		if _, ok := observed[k]; !ok {
			problems = append(problems, "allow-listed on-behalf-of call site "+k+" no longer exists — remove it")
		}
	}
	sort.Strings(problems)
	return problems
}

// TestOnBehalfOfRenderedOnlyThroughTheResolver is the class guard over the real module:
// no second renderer, no OnBehalfOfPrefix composition, no literal repo, and every resolver
// call site on the reviewed allow-list.
func TestOnBehalfOfRenderedOnlyThroughTheResolver(t *testing.T) {
	root := filepath.Join("..", "..")
	scan, err := scanOnBehalfOfRenderers(root)
	if err != nil {
		t.Fatalf("scanning %s: %v", root, err)
	}
	if len(scan.sites) == 0 {
		t.Fatalf("found no on-behalf-of call sites under %s — this guard is looking in the wrong place", root)
	}
	for _, f := range scan.findings {
		t.Error(f)
	}
	for _, p := range diffOnBehalfOfSites(scan.sites, onBehalfOfCallSites) {
		t.Error(p)
	}
}

// TestOnBehalfOfClassGuardCatchesPlantedInstances is the guard's own control: a tree
// carrying one planted instance of each shape the class can take must be reported, shape by
// shape, so a guard that silently stopped seeing (a parser change, a narrowed regexp) goes
// red here rather than passing over the real tree.
func TestOnBehalfOfClassGuardCatchesPlantedInstances(t *testing.T) {
	root := t.TempDir()
	write := func(rel, src string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// The resolver file itself may spell the trailer — it must NOT be reported.
	write(onBehalfOfResolverFile, "package deskkit\n\nconst OnBehalfOfPrefix = \"On-behalf-of:\"\n\n"+
		"func line(v string) string { return OnBehalfOfPrefix + \" human:\" + v }\n")
	// A reviewed writer, as it would appear on the allow-list.
	write("cmd/example/ok.go", "package main\n\nfunc runOK(repo string) { _, _ = deskkit.OnBehalfOfLine(\"\", repo) }\n")
	// Planted: a second renderer, a prefix composition, a literal repo, and a new site.
	write("cmd/example/planted.go", "package main\n\n"+
		"func renderLiteral(login string) string { return \"On-behalf-of: human:\" + login }\n\n"+
		"func renderWitness(login string) string { return \"on-behalf-of human:\" + login }\n\n"+
		"func renderPrefix(login string) string { return deskkit.OnBehalfOfPrefix + \" human:\" + login }\n\n"+
		"func postFixed(b []byte) { _, _ = deskkit.AppendOnBehalfOf(b, \"\", \"example-org/one\") }\n\n"+
		"func postNew(b []byte, home string) { _, _ = deskkit.AppendOnBehalfOf(b, \"\", home) }\n")
	// Prose that only names the mechanism is not a renderer.
	write("cmd/example/prose.go", "package main\n\nconst help = \"every write carries the on-behalf-of trailer\"\n")

	scan, err := scanOnBehalfOfRenderers(root)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(scan.findings, "\n")
	for _, want := range []string{
		"cmd/example/planted.go:3: spells the on-behalf-of trailer",
		"cmd/example/planted.go:5: spells the on-behalf-of trailer",
		"cmd/example/planted.go:7: references OnBehalfOfPrefix",
		"cmd/example/planted.go:9: passes the literal \"example-org/one\"",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("planted instance not reported: want a finding containing %q; findings:\n%s", want, joined)
		}
	}
	for _, f := range scan.findings {
		if !strings.HasPrefix(f, "cmd/example/planted.go:") {
			t.Errorf("guard reported a permitted site (the resolver file, a reviewed writer, or prose): %s", f)
		}
	}
	if len(scan.findings) != 4 {
		t.Errorf("findings = %d, want exactly the 4 planted shapes:\n%s", len(scan.findings), joined)
	}

	allowed := map[string]int{"cmd/example/ok.go:runOK:OnBehalfOfLine(repo)": 1}
	diff := strings.Join(diffOnBehalfOfSites(scan.sites, allowed), "\n")
	for _, want := range []string{
		"unlisted on-behalf-of call site cmd/example/planted.go:postNew:AppendOnBehalfOf(home)",
		"unlisted on-behalf-of call site cmd/example/planted.go:postFixed:AppendOnBehalfOf(\"example-org/one\")",
	} {
		if !strings.Contains(diff, want) {
			t.Errorf("allow-list diff missing %q; got:\n%s", want, diff)
		}
	}
	if strings.Contains(diff, "ok.go") {
		t.Errorf("allow-list diff reported the listed site; got:\n%s", diff)
	}
	// A stale entry is reported too.
	allowed["cmd/example/gone.go:runGone:OnBehalfOfLine(repo)"] = 1
	if d := strings.Join(diffOnBehalfOfSites(scan.sites, allowed), "\n"); !strings.Contains(d, "gone.go") {
		t.Errorf("a stale allow-list entry was not reported; got:\n%s", d)
	}
}
