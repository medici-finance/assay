package cellscratch

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// Every low-level source reader is inventoried, so an additional caller must
// establish admission explicitly instead of inheriting trust from its filename.
func sourceSites(f *ast.File) map[string]bool {
	sites := map[string]bool{}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			name := ""
			switch expr := n.(type) {
			case *ast.Ident:
				switch expr.Name {
				case "sourceGitCmd", "sourceWorktree", "sourceRoot", "openInput":
					name = expr.Name
				}
			case *ast.SelectorExpr:
				pkg, ok := expr.X.(*ast.Ident)
				if ok && ((pkg.Name == "os" && expr.Sel.Name == "OpenRoot") || (pkg.Name == "exec" && (expr.Sel.Name == "Command" || expr.Sel.Name == "CommandContext"))) {
					name = pkg.Name + "." + expr.Sel.Name
				}
			}
			if name != "" {
				sites[fn.Name.Name+":"+name] = true
			}
			return true
		})
	}
	return sites
}

func TestScratchSourceSites(t *testing.T) {
	allowed := map[string]bool{
		"sourceGitCmd:exec.CommandContext": true,
		"sourceWorktree:sourceGitCmd":      true,
		"sourceRoot:sourceWorktree":        true,
		"SourceRevision:sourceRoot":        true,
		"SourceRevision:sourceGitCmd":      true,
		"Inputs:sourceRoot":                true,
		"Inputs:sourceWorktree":            true,
		"Inputs:os.OpenRoot":               true,
		"Inputs:openInput":                 true,
		"Snapshot:sourceRoot":              true,
		"Snapshot:sourceGitCmd":            true,
		"Open:os.OpenRoot":                 true, // owned scratch store, never an import source
	}
	seen := map[string]bool{}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for site := range sourceSites(f) {
			if !allowed[site] {
				t.Errorf("unadmitted source reader %s in %s", site, entry.Name())
			}
			seen[site] = true
		}
	}
	for site := range allowed {
		if !seen[site] {
			t.Errorf("missing source boundary %s", site)
		}
	}
	// Positive control: an independent second caller must be visible to the guard.
	planted, err := parser.ParseFile(token.NewFileSet(), "planted.go", `package cellscratch
 func newReader(source string) { sourceGitCmd(nil, source, "ls-tree") }
 `, 0)
	if err != nil {
		t.Fatal(err)
	}
	sites := sourceSites(planted)
	if !sites["newReader:sourceGitCmd"] || allowed["newReader:sourceGitCmd"] {
		t.Fatal("source class guard missed planted caller")
	}
}
