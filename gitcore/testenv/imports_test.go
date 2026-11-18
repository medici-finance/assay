package testenv

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func productionImportsFixture(path string, source []byte) (bool, error) {
	if strings.HasSuffix(path, "_test.go") || filepath.ToSlash(path) == "gitcore/testenv/environment.go" {
		return false, nil
	}
	f, err := parser.ParseFile(token.NewFileSet(), path, source, parser.ImportsOnly)
	if err != nil {
		return false, err
	}
	for _, imp := range f.Imports {
		value, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return false, err
		}
		if value == "github.com/medici-finance/assay/gitcore/testenv" {
			return true, nil
		}
	}
	return false, nil
}

func TestFixtureImportBoundary(t *testing.T) {
	plant := []byte("package tool\nimport _ \"github.com/medici-finance/assay/gitcore/testenv\"\n")
	if bad, err := productionImportsFixture("statusgen/planted.go", plant); err != nil || !bad {
		t.Fatalf("production import plant not rejected: %v", err)
	}
	if bad, err := productionImportsFixture("statusgen/planted_test.go", plant); err != nil || bad {
		t.Fatalf("test import rejected: %v", err)
	}
	root := filepath.Join("..", "..")
	for _, scope := range []string{"gitcore", "statusgen", "tools/desk"} {
		err := filepath.WalkDir(filepath.Join(root, scope), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() && (entry.Name() == ".git" || entry.Name() == "vendor" || entry.Name() == "testdata") {
				return filepath.SkipDir
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			bad, err := productionImportsFixture(rel, body)
			if bad {
				t.Errorf("production import of test support: %s", rel)
			}
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
