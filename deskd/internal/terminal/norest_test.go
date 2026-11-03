package terminal

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// norest_test.go is the SCROLLBACK-AT-REST audit (spec §5.1: the terminal
// scrollback is memory-only and is NEVER written under the data dir). It is the
// internal/custody/norest_test.go pattern applied to this package: a SOURCE
// audit, not a happy-path runtime check, because what must hold is that NO code
// path can spool the buffer to disk — a property of the code, not of one run.
//
// A runtime test that never happened to fill a buffer would pass against a
// build that lazily spooled a large ring to a temp file "for big buffers"; this
// walk fails that build the moment the filesystem call is compiled in.

// scrollbackWriters are the standard-library calls that could put a byte on
// disk. None of them, and no os.ReadFile, may appear in this package.
var scrollbackWriters = map[string]bool{
	"WriteFile":  true,
	"Create":     true,
	"CreateTemp": true,
	"OpenFile":   true,
	"Open":       true,
	"MkdirAll":   true,
	"Mkdir":      true,
	"ReadFile":   true,
}

// storageImports are the packages that would give this package a place to
// persist a session row, a receipt or scrollback.
var storageImports = []string{"database/sql", "/internal/index", "/internal/local", "io/ioutil", "encoding/gob"}

// TestServiceHoldsNoScrollbackAtRest asserts the terminal tree — the home of
// the cursor ring buffer — contains no filesystem write and no filesystem
// read, including driver subpackages.
// Scrollback that is never written cannot be read back after a restart, which
// is the whole of the not-at-rest guarantee (spec §5.1).
//
// the service contract extends the audit to the send path and the custom session
// store: send.go and sessions.go must be among the files walked (so a rename
// cannot quietly move them out of the audit), and no file in the package may
// import a storage package — the session table and the receipts are memory
// only, and a restart forgets them.
// regression: F-scrollback-audit-scope
func TestServiceHoldsNoScrollbackAtRest(t *testing.T) {
	files := nonTestGoFiles(t, ".")
	for _, must := range []string{"send.go", "sessions.go", "ring.go", "registry.go"} {
		found := false
		for _, f := range files {
			found = found || f == must
		}
		if !found {
			t.Errorf("%s is not among the audited files — the send path and the session store must stay inside this audit", must)
		}
	}
	for _, name := range files {
		violations, err := scrollbackViolations(name)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, violation := range violations {
			t.Errorf("%s: %s — terminal scrollback, receipts and sessions are memory-only", name, violation)
		}
	}
}

// scrollbackViolations is shared with a planted source fixture so the matcher
// itself is checked, independently of whether production contains a writer today.
func scrollbackViolations(name string) ([]string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), name, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	var violations []string
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		for _, bad := range storageImports {
			if path == bad || strings.HasSuffix(path, bad) {
				violations = append(violations, "imports "+path)
			}
		}
	}
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if ok && pkg.Name == "os" && scrollbackWriters[sel.Sel.Name] {
			violations = append(violations, "calls os."+sel.Sel.Name)
		}
		return true
	})
	return violations, nil
}

// nonTestGoFiles walks all terminal drivers as well as the ring's own package.
// Go runs tests per package; a flat ReadDir here silently excluded every driver.
func nonTestGoFiles(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	return out
}

// regression: F-scrollback-audit-scope
func TestScrollbackAuditWalk(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "next-driver"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"root.go", "next-driver/driver.go", "next-driver/driver_test.go"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("package fixture\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	files := nonTestGoFiles(t, root)
	want := []string{filepath.Join(root, "next-driver/driver.go"), filepath.Join(root, "root.go")}
	if strings.Join(files, "\n") != strings.Join(want, "\n") {
		t.Fatalf("audited files = %v, want root.go and next-driver/driver.go (tests excluded)", files)
	}
}

// regression: F-scrollback-audit-scope
func TestScrollbackAuditProbe(t *testing.T) {
	violations, err := scrollbackViolations("testdata/scrollback-writer.go.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 1 || violations[0] != "calls os.WriteFile" {
		t.Fatalf("planted driver writer = %v, want calls os.WriteFile", violations)
	}
}
