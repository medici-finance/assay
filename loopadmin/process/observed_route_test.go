package process

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// callee names a call's target: a bare function or a method/selector.
func callee(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		return fun.Sel.Name
	}
	return ""
}

// The observed entrypoints delegate to the existing public launch wrappers.
// This extends their custody/admission class boundary without a second owner.
func observedRouteIssues(source []byte) ([]string, error) {
	tree, err := parser.ParseFile(token.NewFileSet(), "source.go", source, 0)
	if err != nil {
		return nil, err
	}
	var issues []string
	delegates := map[string]string{"RunObserved": "Run", "RunInteractiveObserved": "RunInteractive"}
	for _, decl := range tree.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		expected := delegates[fn.Name.Name]
		found := false
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			name := callee(call)
			if expected != "" && name == expected {
				found = true
			}
			if name == "runObserved" && fn.Name.Name != "run" {
				issues = append(issues, fn.Name.Name+": raw observed launch bypass")
			}
			if expected != "" && (name == "run" || name == "withCustody" || name == "newProcessTree" || name == "newInteractiveProcessTree") {
				issues = append(issues, fn.Name.Name+": public launch bypass")
			}
			return true
		})
		if expected != "" && !found {
			issues = append(issues, fn.Name.Name+": missing public launch delegation")
		}
	}
	return issues, nil
}

// Defect class: raw process launch outside launch custody. Enumerate every
// call to the only raw runner and require it to sit in an exported launch
// wrapper that also enters withCustody.
func rawCustodyBypasses(source []byte) ([]string, error) {
	f, err := parser.ParseFile(token.NewFileSet(), "source.go", source, 0)
	if err != nil {
		return nil, err
	}
	var bad []string
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		hasCustody, raw := false, false
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				switch callee(call) {
				case "run":
					raw = true
				case "withCustody":
					hasCustody = true
				}
			}
			return true
		})
		if raw && (fn.Name.Name != "Run" && fn.Name.Name != "RunInteractive" || !hasCustody) {
			bad = append(bad, fn.Name.Name)
		}
	}
	return bad, nil
}

func TestCacheObservedRoutes(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		issues, err := observedRouteIssues(b)
		if err != nil || len(issues) > 0 {
			t.Fatalf("observed custody routes %s: %v %v", path, issues, err)
		}
		bad, err := rawCustodyBypasses(b)
		if err != nil || len(bad) > 0 {
			t.Fatalf("uncustodied raw launch %s: %v %v", path, bad, err)
		}
	}
	for _, source := range []string{
		"package fixture;func secondSite(){runObserved()}",
		"package fixture;func RunObserved(){run()}",
		"package fixture;func RunInteractiveObserved(){runObserved()}",
		"package fixture;func (r *Runner) RunObserved(){r.withCustody()}",
		"package fixture;func (r *Runner) RunObserved(){r.RunInteractive()}",
	} {
		issues, err := observedRouteIssues([]byte(source))
		if err != nil || len(issues) == 0 {
			t.Fatal("class guard missed planted route", source, issues, err)
		}
	}
	for _, source := range []string{
		"package fixture\nfunc secondSite(){ run(nil,nil,nil,\"\",nil,nil,nil) }\n",
		"package fixture\nfunc (r *Runner) Run(){ run() }\n",
	} {
		bad, err := rawCustodyBypasses([]byte(source))
		if err != nil || len(bad) != 1 {
			t.Fatalf("class guard missed planted raw site %q: %v %v", source, bad, err)
		}
	}
}
