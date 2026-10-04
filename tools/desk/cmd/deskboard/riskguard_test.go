package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// gateIdents are the verdict, CI and draft eligibility names a risk term may not sit under.
var gateIdents = map[string]bool{
	"approved": true, "atHead": true, "blocking": true, "approvedAtHead": true,
	"IsDraft": true, "draft": true, "fail": true, "pending": true, "ciGreen": true,
}

// mentionsGate reports whether an expression reads any verdict, CI or draft name.
func mentionsGate(e ast.Node) bool {
	gated := false
	if e == nil {
		return false
	}
	ast.Inspect(e, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && gateIdents[id.Name] {
			gated = true
		}
		return !gated
	})
	return gated
}

// isRiskTarget matches `riskClassed`, `x.riskClassed` and a `RiskClassed:` key.
func isRiskTarget(e ast.Expr) bool {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name == "riskClassed" || v.Name == "RiskClassed"
	case *ast.SelectorExpr:
		return v.Sel.Name == "riskClassed" || v.Sel.Name == "RiskClassed"
	}
	return false
}

// assignsRisk reports whether any statement under n writes the risk classification.
func assignsRisk(n ast.Node) bool {
	hit := false
	ast.Inspect(n, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.AssignStmt:
			for _, lhs := range v.Lhs {
				hit = hit || isRiskTarget(lhs)
			}
		case *ast.KeyValueExpr:
			hit = hit || isRiskTarget(v.Key)
		}
		return !hit
	})
	return hit
}

// verdictGatedRisk guards the defect class: risk dispatch metadata may not be
// assigned underneath verdict, CI or draft eligibility. It scans every function in
// the source for three shapes: an if condition, a switch tag or case expression, and
// the value assigned to the risk field. Each shape is named in its finding.
func verdictGatedRisk(src []byte) ([]string, error) {
	f, err := parser.ParseFile(token.NewFileSet(), "risk.go", src, 0)
	if err != nil {
		return nil, err
	}
	var found []string
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		name := fn.Name.Name
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.IfStmt:
				if mentionsGate(v.Cond) && assignsRisk(v.Body) {
					found = append(found, name+":if")
				}
			case *ast.SwitchStmt:
				if v.Tag != nil && mentionsGate(v.Tag) && assignsRisk(v.Body) {
					found = append(found, name+":switch")
				}
			case *ast.CaseClause:
				for _, e := range v.List {
					if mentionsGate(e) && assignsRisk(&ast.BlockStmt{List: v.Body}) {
						found = append(found, name+":case")
						break
					}
				}
			case *ast.AssignStmt:
				for i, lhs := range v.Lhs {
					if !isRiskTarget(lhs) {
						continue
					}
					rhs := ast.Node(nil)
					if len(v.Rhs) == len(v.Lhs) {
						rhs = v.Rhs[i]
					} else if len(v.Rhs) == 1 {
						rhs = v.Rhs[0]
					}
					if mentionsGate(rhs) {
						found = append(found, name+":value")
					}
				}
			case *ast.KeyValueExpr:
				if isRiskTarget(v.Key) && mentionsGate(v.Value) {
					found = append(found, name+":value")
				}
			}
			return true
		})
	}
	return found, nil
}

func TestRiskNotVerdictGated(t *testing.T) {
	src, err := os.ReadFile("board.go")
	if err != nil {
		t.Fatal(err)
	}
	found, err := verdictGatedRisk(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) > 0 {
		t.Fatalf("verdict-gated risk assignments: %v", found)
	}
}

func TestRiskGuardSecondSite(t *testing.T) {
	// Planted siblings are the positive controls, one per shape the guard scans;
	// a matcher that stops seeing a shape fails here instead of silently certifying
	// the real source. clean() reads the gate names without gating risk on them.
	src := []byte(`package main
 func sibling(rs state) { riskClassed := false; if rs.approved { riskClassed = true }; _ = riskClassed }
 func caseCI(fail, pending int) { riskClassed := false; switch { case fail == 0 && pending == 0 && hot(): riskClassed = true }; _ = riskClassed }
 func tagDraft(p pr) { var riskClassed bool; switch p.IsDraft { case true: riskClassed = true }; _ = riskClassed }
 func valueCI(fail int, in *input) { in.riskClassed = fail == 0 && hot() }
 func pairDraft(p pr) { riskClassed, why := p.IsDraft && hot(), ""; _, _ = riskClassed, why }
 func keyVerdict(rs state) actionRow { return actionRow{RiskClassed: rs.approved && hot()} }
 func clean(rs state, fail int) { riskClassed := hot(); if rs.approved && fail == 0 { act() }; switch { case hot(): riskClassed = true }; _ = riskClassed }
 `)
	found, err := verdictGatedRisk(src)
	if err != nil {
		t.Fatal(err)
	}
	want := "sibling:if,caseCI:case,tagDraft:switch,valueCI:value,pairDraft:value,keyVerdict:value"
	if strings.Join(found, ",") != want {
		t.Fatalf("planted siblings: found %v want %s", found, want)
	}
}
