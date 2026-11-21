package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

func TestAdjudicatedHoldRechecksBoundSource(t *testing.T) {
	fx := newCreditRepo(t)
	putFile(t, filepath.Join(filepath.Dir(fx.brief), "README.md"), "# Stream\n")
	evidence := "**VERIFY: HELD**\n"
	fx.commit(t, vfMailVerifier, creditPreface+evidence)
	bf := &BriefFile{Path: fx.brief, Evidence: evidence}
	if err := closeVerifyAdjudicatedHold(fx.root, "demo/01", "implemented", bf, nil); err == nil {
		t.Fatal("ordinary close must continue to refuse the hold")
	}
	source, err := readAcceptanceSource(fx.root, fx.brief)
	if err != nil {
		t.Fatal(err)
	}
	proof := &verifiedAcceptanceAdjudication{source: source}
	if err := closeVerifyAdjudicatedHold(fx.root, "demo/01", "implemented", bf, proof); err != nil {
		t.Fatalf("bound hold check: %v", err)
	}
	putFile(t, fx.brief, getFile(t, fx.brief)+"\n**VERIFY: COULD-NOT-CHECK**\n")
	if err := closeVerifyAdjudicatedHold(fx.root, "demo/01", "implemented", bf, proof); err == nil {
		t.Fatal("source changed after adjudication")
	}
}

func TestAdjudicatedCloseProofHasSingleConstructor(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	check := func(f *ast.File) (int, []string) {
		count := 0
		var bad []string
		for _, decl := range f.Decls {
			owner := ""
			if fn, ok := decl.(*ast.FuncDecl); ok {
				owner = fn.Name.Name
			}
			ast.Inspect(decl, func(n ast.Node) bool {
				literal, ok := n.(*ast.CompositeLit)
				if !ok {
					return true
				}
				id, ok := literal.Type.(*ast.Ident)
				if ok && id.Name == "verifiedAcceptanceAdjudication" {
					count++
					if owner != "planAdjudicatedClose" {
						bad = append(bad, fset.Position(literal.Pos()).String())
					}
				}
				return true
			})
		}
		return count, bad
	}
	count := 0
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		n, bad := check(file)
		count += n
		if len(bad) != 0 {
			t.Errorf("adjudicated close proof constructed outside its readiness owner: %v", bad)
		}
	}
	if count != 1 {
		t.Errorf("found %d proof construction sites, want one", count)
	}
	plant, err := parser.ParseFile(fset, "plant.go", "package main; func plant() { _ = verifiedAcceptanceAdjudication{} }", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, bad := check(plant); len(bad) != 1 {
		t.Fatal("guard failed to reject a second constructor")
	}
}
