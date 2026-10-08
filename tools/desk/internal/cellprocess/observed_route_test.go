package cellprocess

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
			id, ok := call.Fun.(*ast.Ident)
			if !ok {
				return true
			}
			if expected != "" && id.Name == expected {
				found = true
			}
			if id.Name == "runObserved" && fn.Name.Name != "run" {
				issues = append(issues, fn.Name.Name+": raw observed launch bypass")
			}
			if expected != "" && (id.Name == "run" || id.Name == "newProcessTree" || id.Name == "newInteractiveProcessTree") {
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
	}
	for _, source := range []string{
		"package fixture;func secondSite(){runObserved()}",
		"package fixture;func RunObserved(){run()}",
		"package fixture;func RunInteractiveObserved(){runObserved()}",
	} {
		issues, err := observedRouteIssues([]byte(source))
		if err != nil || len(issues) == 0 {
			t.Fatal("class guard missed planted route", issues, err)
		}
	}
}
