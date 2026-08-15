package improve

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// packageSource returns the shipped (non-test) Go source of this package, with
// line comments stripped so the package's own prose about writes does not trip
// the scanners below.
func packageSource(t *testing.T) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading package dir: %v", err)
	}
	out := map[string]string{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		var kept []string
		for _, line := range strings.Split(string(b), "\n") {
			if i := strings.Index(line, "//"); i >= 0 {
				line = line[:i]
			}
			kept = append(kept, line)
		}
		out[name] = strings.Join(kept, "\n")
	}
	if len(out) == 0 {
		t.Fatal("no shipped source found — a scanner that reads nothing passes everything")
	}
	return out
}

// TestImprovePaneHoldsNoWritePath — the read-only claim, structurally. This
// package renders; it does not run, open, create or post anything.
//
// The forbidden tokens are assembled at run time so that this test's own
// source does not contain the literals it forbids — a scanner that trips on
// itself gets weakened until it stops tripping, which is how it stops
// checking.
func TestImprovePaneHoldsNoWritePath(t *testing.T) {
	os_ := "os" + "."
	exec_ := "exec" + "."
	forbidden := []string{
		os_ + "Create", os_ + "WriteFile", os_ + "OpenFile", os_ + "Remove",
		os_ + "RemoveAll", os_ + "Mkdir", os_ + "MkdirAll", os_ + "Rename",
		os_ + "Truncate", os_ + "Symlink", os_ + "StartProcess",
		exec_ + "Command", exec_ + "CommandContext", exec_ + "LookPath",
		"ioutil" + ".WriteFile", "syscall" + ".Exec",
		"http" + ".Post", "http" + ".NewRequest",
	}
	src := packageSource(t)
	for name, body := range src {
		for _, tok := range forbidden {
			if strings.Contains(body, tok) {
				t.Fatalf("%s contains %q — this package must hold no write or execution path; the adopt action routes to the human commit path and writes nothing", name, tok)
			}
		}
	}

	// The scanner must be able to fail: a token it would catch, proven to be
	// caught, on a body it constructs itself.
	if !strings.Contains("func f() { "+os_+"Create(p) }", forbidden[0]) {
		t.Fatal("the scanner's own matching is broken — it would pass any source at all")
	}
}

// TestImprovePaneRunsNoSubprocess — the stronger form of the same claim,
// checked on the import graph rather than on text. This package declares what
// a source IS; it never runs one.
func TestImprovePaneRunsNoSubprocess(t *testing.T) {
	forbiddenImports := map[string]string{
		"os/exec":      "running a subprocess",
		"net/http":     "reaching the network",
		"io/ioutil":    "legacy file writes",
		"os/signal":    "process control",
		"database/sql": "a data store this pane has no business holding",
	}
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parsing package: %v", err)
	}
	var files int
	for _, pkg := range pkgs {
		for name, f := range pkg.Files {
			files++
			for _, imp := range f.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				if why, bad := forbiddenImports[path]; bad {
					t.Fatalf("%s imports %q (%s) — this package renders and does not act", filepath.Base(name), path, why)
				}
			}
		}
	}
	if files == 0 {
		t.Fatal("parsed no files — a check that reads nothing passes everything")
	}
	// And "os" itself may only appear in test files. The shipped source reads
	// no file at all.
	for name, body := range packageSource(t) {
		if strings.Contains(body, "\"os\"") {
			t.Fatalf("%s imports os — the shipped source of this package touches no filesystem", name)
		}
	}
}

// TestImprovePaneRowsAreUnreachableWithoutTheState — the field that would let a
// caller render a could-not-check strip's rows must not be exported, and the
// renderer must not read it directly.
func TestImprovePaneRowsAreUnreachableWithoutTheState(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "strip.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing strip.go: %v", err)
	}
	var checked bool
	ast.Inspect(f, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok || ts.Name.Name != "Strip" {
			return true
		}
		st, ok := ts.Type.(*ast.StructType)
		if !ok {
			return true
		}
		checked = true
		for _, fld := range st.Fields.List {
			for _, nm := range fld.Names {
				if ast.IsExported(nm.Name) {
					t.Fatalf("Strip.%s is exported — a caller that can reach the row slice directly can render a could-not-check strip as an empty list", nm.Name)
				}
			}
		}
		return false
	})
	if !checked {
		t.Fatal("the Strip type was not found in strip.go — this check read nothing")
	}
}
