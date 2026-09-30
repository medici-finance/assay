package main

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

// Class guard for the high-unblocks arm (statusgen/05, row R residual).
//
// DEFECT CLASS: a count that feeds the critical tier walked over one-sided depends
// edges. The instance was nextUp handing criticalTierArm the SCORE's blockedCount
// (buildRevDeps walks every declared edge). The fix makes criticalTierArm take a
// reciprocatedRevDeps graph instead of an int; this guard pins that the graph type
// can only be BUILT in its one constructor, so no other site can populate one from
// the score's unfiltered graph and hand it to the arm.
//
// The scan covers every non-test .go file in the package: a composite literal of
// the type, or an assignment to one of its fields, outside
// buildReciprocatedRevDeps is a violation naming the file and line.

// reciprocatedGraphBuilders lists the ONLY functions allowed to construct or
// populate a reciprocatedRevDeps value.
var reciprocatedGraphBuilders = map[string]bool{"buildReciprocatedRevDeps": true}

// reciprocatedGraphViolations parses one Go source and returns every site outside
// the allow-list that builds a reciprocatedRevDeps literal or assigns to its
// recipRev/recipStatus fields (named distinctly so a selector match cannot hit an
// unrelated type's field).
func reciprocatedGraphViolations(fset *token.FileSet, name string, src []byte) ([]string, error) {
	f, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, decl := range f.Decls {
		fn, isFn := decl.(*ast.FuncDecl)
		if isFn && reciprocatedGraphBuilders[fn.Name.Name] {
			continue
		}
		ast.Inspect(decl, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CompositeLit:
				if id, ok := x.Type.(*ast.Ident); ok && id.Name == "reciprocatedRevDeps" && len(x.Elts) > 0 {
					out = append(out, fset.Position(x.Pos()).String()+": builds a populated reciprocatedRevDeps literal")
				}
			case *ast.AssignStmt:
				for _, lhs := range x.Lhs {
					base := lhs
					if ix, ok := base.(*ast.IndexExpr); ok {
						base = ix.X
					}
					if sel, ok := base.(*ast.SelectorExpr); ok && (sel.Sel.Name == "recipRev" || sel.Sel.Name == "recipStatus") {
						out = append(out, fset.Position(x.Pos()).String()+": assigns to a reciprocated-graph field ."+sel.Sel.Name)
					}
				}
			}
			return true
		})
	}
	return out, nil
}

// TestReciprocatedRevDepsSingleConstructor is the class guard: no non-test file in
// the package builds or populates a reciprocatedRevDeps outside its constructor.
func TestReciprocatedRevDepsSingleConstructor(t *testing.T) {
	// Positive control: a planted second instance — a function that wraps the
	// score's unfiltered graph as a reciprocated one — MUST be flagged, so a matcher
	// that silently stopped matching fails here instead of reporting clean.
	planted := []byte(`package main
func launder(streams []*Stream) reciprocatedRevDeps {
	rev, status := buildRevDeps(streams)
	return reciprocatedRevDeps{recipRev: rev, recipStatus: status}
}
func launder2(g reciprocatedRevDeps, streams []*Stream) {
	g.recipRev, _ = buildRevDeps(streams)
}
`)
	got, err := reciprocatedGraphViolations(token.NewFileSet(), "planted.go", planted)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("positive control: the guard must flag both planted sites, got %v", got)
	}

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	fset := token.NewFileSet()
	var violations []string
	scanned := 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		v, err := reciprocatedGraphViolations(fset, name, src)
		if err != nil {
			t.Fatal(err)
		}
		violations = append(violations, v...)
		scanned++
	}
	if scanned == 0 {
		t.Fatal("could-not-check: no non-test source files were scanned")
	}
	if len(violations) > 0 {
		t.Fatalf("reciprocatedRevDeps may only be built in buildReciprocatedRevDeps — the high-unblocks arm must never read a graph walked over one-sided edges:\n%s",
			strings.Join(violations, "\n"))
	}
}
