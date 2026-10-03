package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// Guard the defect class: risk dispatch metadata may not be assigned underneath
// verdict, CI or draft eligibility. Scan every function, including future siblings.
func verdictGatedRisk(src []byte) ([]string, error) {
	f, err := parser.ParseFile(token.NewFileSet(), "risk.go", src, 0)
	if err != nil {
		return nil, err
	}
	var found []string
	ast.Inspect(f, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}
		ast.Inspect(fn, func(n ast.Node) bool {
			branch, ok := n.(*ast.IfStmt)
			if !ok {
				return true
			}
			gated := false
			ast.Inspect(branch.Cond, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok {
					switch id.Name {
					case "approved", "atHead", "blocking", "IsDraft", "fail", "pending":
						gated = true
					}
				}
				return true
			})
			if gated {
				ast.Inspect(branch.Body, func(n ast.Node) bool {
					if assign, ok := n.(*ast.AssignStmt); ok {
						for _, lhs := range assign.Lhs {
							if id, ok := lhs.(*ast.Ident); ok && id.Name == "riskClassed" {
								found = append(found, fn.Name.Name)
							}
						}
					}
					return true
				})
			}
			return true
		})
		return false
	})
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
	// A planted sibling is the positive control; removing the scan's matches must
	// fail this test instead of silently certifying the real source.
	src := []byte(`package main
 func sibling(rs state) { riskClassed := false; if rs.approved { riskClassed = true }; _ = riskClassed }
 func clean() { riskClassed := true; _ = riskClassed }
 `)
	found, err := verdictGatedRisk(src)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(found, ",") != "sibling" {
		t.Fatalf("planted sibling: found %v", found)
	}
}
