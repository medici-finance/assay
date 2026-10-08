package deskkit

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A typed finding's severity alone never says whether it still blocks: a re-review CR lists
// the findings it now records as resolved beside the open ones. A classifier that reads
// severity without the lifecycle state counts those resolved entries as live blockers
// (#1985). Finding.StandingBlockerAt is the one predicate; outside reviewfinding.go (the
// ALLOW-LIST), no non-test file under tools/desk may read a finding's Severity field, name
// SeverityBlocking, or call blocking().
const standingBlockerHome = "internal/deskkit/reviewfinding.go"

func rawSeverityReads(path string, src any) ([]string, error) {
	fs := token.NewFileSet()
	f, err := parser.ParseFile(fs, path, src, 0)
	if err != nil {
		return nil, err
	}
	var found []string
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr: // f.blocking() — the method; a plain `.blocking` FIELD elsewhere is unrelated
			if sel, ok := x.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "blocking" {
				found = append(found, fmt.Sprint(fs.Position(sel.Sel.Pos()))+" .blocking()")
			}
		case *ast.SelectorExpr:
			if x.Sel.Name == "Severity" {
				found = append(found, fmt.Sprint(fs.Position(x.Sel.Pos()))+" .Severity")
			}
		case *ast.Ident:
			if x.Name == "SeverityBlocking" {
				found = append(found, fmt.Sprint(fs.Position(x.Pos()))+" "+x.Name)
			}
		}
		return true
	})
	return found, nil
}

func TestStandingBlockerIsTheOnlyReader(t *testing.T) {
	home := filepath.Clean(filepath.Join("../..", standingBlockerHome))
	err := filepath.WalkDir("../..", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" || strings.HasPrefix(d.Name(), ".") && d.Name() != ".." {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || filepath.Clean(path) == home {
			return nil
		}
		found, err := rawSeverityReads(path, nil)
		if err != nil {
			return err
		}
		for _, at := range found {
			t.Errorf("raw finding-severity read at %s; classify with Finding.StandingBlockerAt (severity AND not resolved)", at)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Positive control: the matcher flags every spelling of a severity-only classifier.
func TestStandingBlockerGuardControl(t *testing.T) {
	plant := `package example
func a(f deskkit.Finding) bool { return f.Severity == deskkit.SeverityBlocking }
func b(f Finding) bool { return f.blocking() }
func c(f Finding) bool { return string(f.Severity) == "blocking" }`
	got, err := rawSeverityReads("planted.go", plant)
	if err != nil || len(got) != 4 {
		t.Fatalf("planted severity-only classifiers: %v %v", got, err)
	}
}
