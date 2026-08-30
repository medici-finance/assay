package acp

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// loopengineImportPath is the one package this one must never import. The
// dependency points the other way — the engine adapter imports acp, per the
// dispatch spec §4.1 — which keeps acp reusable by any future consumer. doc.go
// states the rule; this test is the mechanism that enforces it.
const loopengineImportPath = "github.com/medici-finance/assay/tools/desk/internal/loopengine"

// findLoopengineImports parses each Go source in files (filename -> source text)
// and returns the names of the files whose import block pulls in the loopengine
// package. It is a pure function over its input — it touches no filesystem — so
// the identical detection logic runs against the real package and against a
// synthetic fixture, which is what lets the violation sub-case below prove the
// check can actually fail.
func findLoopengineImports(files map[string]string) ([]string, error) {
	var offenders []string
	fset := token.NewFileSet()
	for name, src := range files {
		f, err := parser.ParseFile(fset, name, src, parser.ImportsOnly)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", name, err)
		}
		for _, imp := range f.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if path == loopengineImportPath || strings.HasSuffix(path, "/internal/loopengine") {
				offenders = append(offenders, name)
				break
			}
		}
	}
	return offenders, nil
}

// readPackageSources returns the non-test .go sources of the package rooted at
// dir. Test files are excluded: the rule guards the package's shipping sources
// (a _test.go may legitimately mention the forbidden path in a string fixture,
// as this very file does), and the dependency boundary is about what acp links
// into a consumer, not its test binary.
func readPackageSources(dir string) (map[string]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	files := make(map[string]string)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		files[name] = string(b)
	}
	return files, nil
}

// TestNoLoopengineImport is the standing reverse-import check that doc.go names.
// go test runs it with the package directory as the working directory, so "."
// is the acp package.
func TestNoLoopengineImport(t *testing.T) {
	t.Run("real_package_is_clean", func(t *testing.T) {
		files, err := readPackageSources(".")
		if err != nil {
			t.Fatalf("reading package sources: %v", err)
		}
		if len(files) == 0 {
			t.Fatal("no non-test .go sources found — the scan would pass vacuously")
		}
		offenders, err := findLoopengineImports(files)
		if err != nil {
			t.Fatalf("scanning imports: %v", err)
		}
		if len(offenders) != 0 {
			t.Errorf("acp must not import loopengine (dispatch spec §4.1); offending files: %v", offenders)
		}
	})

	// The falsifying sub-case: feed the same detector a synthetic source that
	// DOES import loopengine and assert it is reported. Without this, the clean
	// sub-case above is satisfied by a detector that can never fire, and its
	// pass proves nothing. This is the row the fail-first evidence points at.
	t.Run("synthetic_fixture_is_a_reported_violation", func(t *testing.T) {
		fixture := map[string]string{
			"synthetic_offender.go": "package acp\n\nimport (\n\t_ \"" + loopengineImportPath + "\"\n)\n",
		}
		offenders, err := findLoopengineImports(fixture)
		if err != nil {
			t.Fatalf("scanning fixture: %v", err)
		}
		if len(offenders) != 1 || offenders[0] != "synthetic_offender.go" {
			t.Fatalf("detector failed to report the forbidden import; got %v, want [synthetic_offender.go]", offenders)
		}
	})
}
