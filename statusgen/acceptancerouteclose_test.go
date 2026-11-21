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
	mismatched := *proof
	mismatched.source.ReadmeBlob = source.BriefBlob
	if err := closeVerifyAdjudicatedHold(fx.root, "demo/01", "implemented", bf, &mismatched); err == nil || !strings.Contains(err.Error(), "adjudicated hold scan") {
		t.Fatalf("unmodified checkout with a different receipt must refuse, got %v", err)
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
				var expr ast.Expr
				switch node := n.(type) {
				case *ast.CompositeLit:
					expr = node.Type
				case *ast.CallExpr:
					if calledName(node) == "new" && len(node.Args) == 1 {
						expr = node.Args[0]
					}
				case *ast.ValueSpec:
					expr = node.Type
				}
				id, ok := expr.(*ast.Ident)
				if ok && id.Name == "verifiedAcceptanceAdjudication" {
					count++
					if owner != "planAdjudicatedClose" {
						bad = append(bad, fset.Position(n.Pos()).String())
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
	for _, statement := range []string{
		"_ = verifiedAcceptanceAdjudication{}",
		"_ = new(verifiedAcceptanceAdjudication)",
		"var receipt verifiedAcceptanceAdjudication; _ = receipt",
	} {
		plant, err := parser.ParseFile(fset, "plant.go", "package main; func plant() { "+statement+" }", 0)
		if err != nil {
			t.Fatal(err)
		}
		if _, bad := check(plant); len(bad) != 1 {
			t.Fatalf("guard failed to reject %q", statement)
		}
	}
}
