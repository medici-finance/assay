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

// Class guard for row 5 of statusgen/05.
//
// DEFECT CLASS: a critical-tier arm whose input is fixed in SOURCE — a compiled-in
// authority identity, or a hard-wired main-red answer — instead of arriving through
// its one sanctioned channel. The instances were `criticalStampAuthorities =
// map[string]bool{}` (a placeholder waiting for a source edit naming an identity)
// and `mainRedCritical` returning a constant false. The fix routes each input
// through exactly one writer: the stamp authority set through
// wireCriticalStampAuthorities (roster configuration), the main-health input through
// main()'s --main-health parse. This guard fails if any other non-test site writes
// either input, or if a package-level initializer compiles in a non-empty authority
// set.

// criticalInputWriters maps each guarded package var to the only functions allowed
// to assign it.
var criticalInputWriters = map[string]map[string]bool{
	"criticalStampAuthorities":    {"wireCriticalStampAuthorities": true},
	"criticalStampAuthoritiesSet": {"wireCriticalStampAuthorities": true},
	"activeMainHealth":            {"main": true},
}

func criticalInputViolations(fset *token.FileSet, name string, src []byte) ([]string, error) {
	f, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			// Package-level initializers: a guarded var may only start EMPTY.
			for _, spec := range d.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, id := range vs.Names {
					if _, guarded := criticalInputWriters[id.Name]; !guarded || i >= len(vs.Values) {
						continue
					}
					if cl, ok := vs.Values[i].(*ast.CompositeLit); ok && len(cl.Elts) > 0 {
						out = append(out, fset.Position(id.Pos()).String()+": "+id.Name+" is initialised with a compiled-in value")
					}
					if bl, ok := vs.Values[i].(*ast.Ident); ok && bl.Name == "true" {
						out = append(out, fset.Position(id.Pos()).String()+": "+id.Name+" is initialised true in source")
					}
				}
			}
		case *ast.FuncDecl:
			fn := d.Name.Name
			ast.Inspect(d, func(n ast.Node) bool {
				as, ok := n.(*ast.AssignStmt)
				if !ok {
					return true
				}
				for _, lhs := range as.Lhs {
					base := lhs
					if ix, ok := base.(*ast.IndexExpr); ok {
						base = ix.X
					}
					id, ok := base.(*ast.Ident)
					if !ok {
						continue
					}
					if writers, guarded := criticalInputWriters[id.Name]; guarded && !writers[fn] {
						out = append(out, fset.Position(as.Pos()).String()+": "+fn+" writes "+id.Name)
					}
				}
				return true
			})
		}
	}
	return out, nil
}

// TestCriticalTierInputsHaveOneWriter is the class guard: each critical-tier input
// has exactly one writer and starts empty in source.
func TestCriticalTierInputsHaveOneWriter(t *testing.T) {
	// Positive control: planted second instances of the class — a compiled-in
	// authority, a hard-wired main-red answer, and a stray writer — MUST all be
	// flagged, so a matcher that silently stopped matching fails here.
	planted := []byte(`package main
var criticalStampAuthorities = map[string]bool{"security-desk": true}
func sneak() {
	activeMainHealth = MainHealth{State: "red"}
	criticalStampAuthorities["x"] = true
}
`)
	got, err := criticalInputViolations(token.NewFileSet(), "planted.go", planted)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("positive control: the guard must flag all three planted sites, got %v", got)
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
		v, err := criticalInputViolations(fset, name, src)
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
		t.Fatalf("a critical-tier input may only be written through its one sanctioned channel:\n%s", strings.Join(violations, "\n"))
	}
}
