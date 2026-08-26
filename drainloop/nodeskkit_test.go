package drainloop

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// The public drainloop core MUST be deskkit-free: nothing homed here may import the house's
// guard, board, or claims packages, or otherwise couple to house infrastructure. This is a
// structural guarantee of the open-core split (convergence plan §5 step 4), asserted here so
// a stray import reddens the module's own suite rather than leaking into the public source.
//
// It is a source-only check: it parses every .go file in the module for its import paths and
// fails on any forbidden substring. It contacts nothing.
func TestNoDeskkitImports(t *testing.T) {
	forbidden := []string{
		"tools/desk",
		"deskkit",
		"internal/board",
		"internal/claims",
	}
	fset := token.NewFileSet()
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if perr != nil {
			return perr
		}
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			for _, bad := range forbidden {
				if strings.Contains(p, bad) {
					t.Errorf("%s imports %q — the public drainloop core must be deskkit-free (forbidden substring %q)", path, p, bad)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
}
