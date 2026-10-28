package cellcache

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// scratchHomes returns "<func>" for every function in one Go source file,
// other than goEnv, that holds a string literal starting "HOME=": a HOME set
// by hand for a child process.
func scratchHomes(f *ast.File) []string {
	var out []string
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Body == nil || fn.Name.Name == "goEnv" {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			if s, err := strconv.Unquote(lit.Value); err == nil && strings.HasPrefix(s, "HOME"+"=") {
				out = append(out, fn.Name.Name)
			}
			return true
		})
	}
	return out
}

// Class guard. Defect class: a test in this package that runs cmd/go under a
// scratch HOME it built by hand, so telemetry is on there and the go command's
// detached telemetry child can write into the HOME while t.TempDir removes it.
// goEnv is the one place a scratch HOME is made; any other function setting
// HOME for a child is a new instance of the class.
//
// regression: #3707
func TestScratchHomeViaGoEnv(t *testing.T) {
	files, err := filepath.Glob("*_test.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("no test files to scan: %v", err)
	}
	var bad []string
	for _, name := range files {
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(token.NewFileSet(), name, src, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, fn := range scratchHomes(f) {
			bad = append(bad, name+":"+fn)
		}
	}
	sort.Strings(bad)
	if len(bad) > 0 {
		t.Fatalf("sets HOME for a child by hand (use goEnv, which turns cmd/go telemetry off first): %v", bad)
	}
}

// The guard's matcher could silently stop matching and report clean forever,
// so it carries a positive control: a planted hand-built HOME must be flagged,
// and goEnv itself must not be.
//
// regression: #3707
func TestScratchHomeGuardPlant(t *testing.T) {
	src := "package p\n" +
		"func goEnv(h string) []string { return []string{\"HOME=\" + h} }\n" +
		"func build(h string) []string { return []string{\"PATH=/bin\", \"HOME=\" + h} }\n" +
		"func fine(h string) []string { return []string{\"XDG_CONFIG_HOME=\" + h} }\n"
	f, err := parser.ParseFile(token.NewFileSet(), "plant.go", src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	if got := scratchHomes(f); len(got) != 1 || got[0] != "build" {
		t.Fatalf("planted hand-built HOME not flagged exactly once: %v", got)
	}
}
