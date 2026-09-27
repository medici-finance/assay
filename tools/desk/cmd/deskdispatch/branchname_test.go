package main

// branchname_test.go — the default-branch rule (branchname.go) as a pure function, plus the
// CLASS guard: no non-test source in this package may spell a `feat/` branch literal outside
// defaultBranch, so a second derivation cannot bring the raw item key back into a branch name.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestDefaultBranchRule(t *testing.T) {
	s := &stub{}
	home, _ := s.install(t)
	// Private fixture first: every key keeps the historical derivation.
	for _, item := range []string{"other-tracker--issue-4242", "assay--issue-7", "example-stream/07", "item-1"} {
		if got, want := defaultBranch(item, allowedRepo), "feat/"+sanitizeSegment(item); got != want {
			t.Errorf("private target: defaultBranch(%q) = %q, want %q", item, got, want)
		}
	}
	plantRosterWithAllowedRepoPublic(t, home)
	cases := []struct{ item, want string }{
		// Keys with no `--` label carry no foreign repo name: unchanged.
		{"example-stream/07", "feat/example-stream-07"},
		{"item-1", "feat/item-1"},
		// The target's own label (basename, with no alias configured): unchanged.
		{"assay--issue-7", "feat/assay--issue-7"},
		{"assay--example-stream--07", "feat/assay--example-stream--07"},
		// A foreign brief claim key keeps only its stream and number.
		{"other-tracker--example-stream--07", "feat/example-stream-07"},
	}
	for _, c := range cases {
		if got := defaultBranch(c.item, allowedRepo); got != c.want {
			t.Errorf("public target: defaultBranch(%q) = %q, want %q", c.item, got, c.want)
		}
	}
	// A foreign non-brief key is hashed: deterministic, fixed shape, carries nothing of the key.
	a := defaultBranch("other-tracker--issue-4242", allowedRepo)
	if a != defaultBranch("other-tracker--issue-4242", allowedRepo) {
		t.Errorf("neutral branch is not deterministic")
	}
	if len(a) != len("feat/item-")+10 || !strings.HasPrefix(a, "feat/item-") {
		t.Errorf("neutral branch %q is not feat/item-<10 hex>", a)
	}
	if strings.Contains(a, "other") || strings.Contains(a, "4242") {
		t.Errorf("neutral branch %q carries part of the foreign key", a)
	}
	if b := defaultBranch("other-tracker--issue-4243", allowedRepo); b == a {
		t.Errorf("two distinct items derived the same neutral branch %q", a)
	}
	if !branchNameRe.MatchString(a) {
		t.Errorf("neutral branch %q fails the worktree verb's branch grammar", a)
	}
}

// feat/ branch literals are allowed in non-test source ONLY inside these functions.
var featLiteralAllowList = map[string]bool{"defaultBranch": true}

// featBranchLiteralSites returns "<file>:<func>" for every string literal whose value starts
// with "feat/" in the given parsed file, outside the allow-listed functions.
func featBranchLiteralSites(fset *token.FileSet, f *ast.File) []string {
	var out []string
	for _, decl := range f.Decls {
		name := ""
		if fd, ok := decl.(*ast.FuncDecl); ok {
			name = fd.Name.Name
		}
		ast.Inspect(decl, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			v, err := strconv.Unquote(lit.Value)
			if err != nil || !strings.HasPrefix(v, "feat/") {
				return true
			}
			if !featLiteralAllowList[name] {
				where := name
				if where == "" {
					where = "(package scope)"
				}
				out = append(out, filepath.Base(fset.Position(lit.Pos()).Filename)+":"+where)
			}
			return true
		})
	}
	return out
}

// CLASS GUARD. Every default-branch derivation goes through defaultBranch; any other `feat/…`
// literal in this package's non-test source is a second derivation that can put the raw item
// key back into a public branch or fragment name.
func TestNoFeatBranchLiteralOutsideDefaultBranch(t *testing.T) {
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var bad []string
	scanned := 0
	for _, p := range paths {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}
		scanned++
		bad = append(bad, featBranchLiteralSites(fset, f)...)
	}
	if scanned == 0 {
		t.Fatal("scanned no source files — the guard is looking in the wrong place")
	}
	sort.Strings(bad)
	if len(bad) > 0 {
		t.Errorf("a `feat/` branch literal outside defaultBranch (branchname.go) — derive the default "+
			"branch through defaultBranch so the public-target rule applies: %v", bad)
	}
}

// POSITIVE CONTROL. The guard's matcher must flag a planted second derivation, in both the
// concatenation and the format-string shapes, and must not flag the allow-listed function.
func TestFeatBranchLiteralGuardFlagsPlantedSite(t *testing.T) {
	const src = `package main
import "fmt"
func defaultBranch(item, repo string) string { return "feat/" + item }
func plantedConcat(item string) string { return "feat/" + item }
func plantedFormat(item string) string { return fmt.Sprintf("feat/%s", item) }
func harmless() string { return "a feat/ mid-string mention" }
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "planted.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := featBranchLiteralSites(fset, f)
	want := []string{"planted.go:plantedConcat", "planted.go:plantedFormat"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("guard flagged %v, want %v", got, want)
	}
}
