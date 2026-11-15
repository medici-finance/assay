package cellrunner

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

const processImport = "github.com/medici-finance/assay/loopadmin/process"

// Defect class: a desk launch outside cache custody. Since loopadmin/process
// is the one runner and takes its custody by injection, a second tools/desk
// importer could build a Runner with another (or a no-op) custodian. Only
// this package may import it.
func uncustodiedImporters(root string) ([]string, error) {
	var bad []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); path != root && (name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			p, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return err
			}
			if p != processImport {
				continue
			}
			rel, _ := filepath.Rel(root, path)
			if filepath.ToSlash(filepath.Dir(rel)) != "internal/cellrunner" {
				bad = append(bad, filepath.ToSlash(rel))
			}
		}
		return nil
	})
	return bad, err
}

// regression: #5492 (shared loop-admin runner move)
func TestLaunchCustodyClassGuard(t *testing.T) {
	bad, err := uncustodiedImporters("../..")
	if err != nil || len(bad) > 0 {
		t.Fatalf("loopadmin/process imported outside internal/cellrunner: %v %v", bad, err)
	}
	// Positive control: a planted second importer is reported.
	root := t.TempDir()
	for path, src := range map[string]string{
		"internal/cellrunner/launch.go": "package cellrunner\nimport _ \"" + processImport + "\"\n",
		"cmd/cellctl/raw.go":            "package main\nimport _ \"" + processImport + "\"\n",
	} {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	bad, err = uncustodiedImporters(root)
	if err != nil || len(bad) != 1 || bad[0] != "cmd/cellctl/raw.go" {
		t.Fatalf("class guard missed planted importer: %v %v", bad, err)
	}
}
