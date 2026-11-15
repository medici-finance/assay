package deskkit

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// exitCodeConsts is the exit-code table in exitcodes.go. The release mutation gate
// (mutations.json) rewrites one of these values onto its neighbour and needs this
// package to still COMPILE, so TestExitCodeTableMatchesDerivation can catch the drift.
var exitCodeConsts = map[string]bool{
	"ExitOK": true, "ExitDisabled": true, "ExitRateLimited": true,
	"ExitRefused": true, "ExitUnverifiable": true,
}

func exitConstName(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		if exitCodeConsts[x.Name] {
			return x.Name
		}
	case *ast.SelectorExpr:
		if exitCodeConsts[x.Sel.Name] {
			return x.Sel.Name
		}
	}
	return ""
}

// exitConstCollisions lists every construct in f that Go requires to hold DISTINCT
// constants and that names two or more exit-code constants: an expression switch's case
// values, or a composite literal's keys. Two such constants made equal by a mutation is a
// duplicate case or a duplicate key, the package stops compiling, and the gate reports
// could-not-mutate instead of a verdict (#6201).
func exitConstCollisions(fset *token.FileSet, f *ast.File) []string {
	var hits []string
	report := func(pos token.Pos, what string, names []string) {
		if len(names) >= 2 {
			p := fset.Position(pos)
			hits = append(hits, fmt.Sprintf("%s:%d: %s %s", filepath.Base(p.Filename), p.Line, what, strings.Join(names, ", ")))
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.SwitchStmt:
			if x.Tag == nil {
				return true // a tagless switch's cases are bool expressions, never duplicate-checked
			}
			var names []string
			for _, st := range x.Body.List {
				if cc, ok := st.(*ast.CaseClause); ok {
					for _, e := range cc.List {
						if name := exitConstName(e); name != "" {
							names = append(names, name)
						}
					}
				}
			}
			report(x.Pos(), "switch cases", names)
		case *ast.CompositeLit:
			var names []string
			for _, el := range x.Elts {
				if kv, ok := el.(*ast.KeyValueExpr); ok {
					if name := exitConstName(kv.Key); name != "" {
						names = append(names, name)
					}
				}
			}
			report(x.Pos(), "literal keys", names)
		}
		return true
	})
	return hits
}

func exitConstCollisionsIn(t *testing.T, glob string) []string {
	t.Helper()
	files, err := filepath.Glob(glob)
	if err != nil || len(files) == 0 {
		t.Fatalf("no Go files matched %s (err %v): the guard read nothing, which is not a pass", glob, err)
	}
	sort.Strings(files)
	fset := token.NewFileSet()
	var hits []string
	for _, path := range files {
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		hits = append(hits, exitConstCollisions(fset, f)...)
	}
	return hits
}

// regression: #6201
// TestNoConstSwitchOnExitCodes keeps this package compilable under the exit-code
// neighbour mutation: no switch case list or literal key set may name two exit-code
// constants. Branch with if/else on IsRefused, IsRateLimited and their siblings instead,
// or a slice of pairs in place of a map keyed by the codes. Test files are included,
// because the mutation gate's suite compiles them too.
func TestNoConstSwitchOnExitCodes(t *testing.T) {
	for _, h := range exitConstCollisionsIn(t, "*.go") {
		t.Errorf("%s — two exit codes that must differ for this package to compile; a mutation that drifts one onto the other leaves the gate could-not-mutate (#6201)", h)
	}
}

// TestExitSwitchGuardFlagsPlant is the guard's positive control: a committed fixture
// carries one planted switch and one planted map, and the guard must name both.
func TestExitSwitchGuardFlagsPlant(t *testing.T) {
	hits := exitConstCollisionsIn(t, filepath.Join("testdata", "exitcodeswitch", "*.go"))
	want := []string{"planted.go:9: switch cases ExitRefused, ExitRateLimited", "planted.go:18: literal keys ExitDisabled, ExitUnverifiable"}
	if strings.Join(hits, "\n") != strings.Join(want, "\n") {
		t.Fatalf("guard found:\n%s\nwant:\n%s", strings.Join(hits, "\n"), strings.Join(want, "\n"))
	}
}
