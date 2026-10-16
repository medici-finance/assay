package deskkit

// perfshared_test.go — the perf-v1 writer exists ONCE, in the shared perfrec module, and every
// recording tool calls it. #3101: statusgen records the same local perf-v1 line as the desk
// tools, and a second copy of the writer (day-file layout, retention, install id, schema) is
// exactly the drift this guards against.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/medici-finance/assay/perfrec"
)

// dayFilePattern matches a literal that names a date followed by .jsonl — a per-day record
// file, which is what the perf writer's day-file regexp is. (The audit and CI-check ledgers
// put the date AFTER .jsonl, so they are different files and do not match.)
var dayFilePattern = regexp.MustCompile(`\\d\{4\}-\\d\{2\}-\\d\{2\}\)?\\\.jsonl`)

// secondWriters returns the non-test Go files under root, outside the perfrec directory, that
// carry a perf-v1 writer's fingerprint in a string literal or call: the schema tag; a day-file
// pattern (a date regexp naming .jsonl); the install-id file name together with the
// 32-hex id pattern; or a getrusage CPU read. Comments are not literals, so prose may still
// name them.
func secondWriters(t *testing.T, root string) []string {
	t.Helper()
	var hits []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "testdata", "vendor", "perfrec":
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if perr != nil {
			return nil // a file this guard cannot parse cannot hide a writer from the compiler
		}
		var schema, dayFile, idName, idPattern, rusage bool
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.BasicLit:
				if x.Kind != token.STRING {
					break
				}
				v := x.Value
				schema = schema || strings.Contains(v, "perf-v1")
				dayFile = dayFile || dayFilePattern.MatchString(v)
				idName = idName || strings.Contains(v, "install-id")
				idPattern = idPattern || strings.Contains(v, "[0-9a-f]{32}")
			case *ast.SelectorExpr:
				if id, ok := x.X.(*ast.Ident); ok && id.Name == "syscall" && x.Sel.Name == "Getrusage" {
					rusage = true
				}
			}
			return true
		})
		var why []string
		for _, c := range []struct {
			on   bool
			name string
		}{
			{schema, "perf-v1 schema literal"}, {dayFile, "day-file pattern"},
			{idName && idPattern, "install-id file + id pattern"}, {rusage, "syscall.Getrusage"},
		} {
			if c.on {
				why = append(why, c.name)
			}
		}
		if len(why) > 0 {
			rel, _ := filepath.Rel(root, path)
			hits = append(hits, filepath.ToSlash(rel)+" ("+strings.Join(why, ", ")+")")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return hits
}

func repoRootFromHere(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "perfrec", "go.mod")); err != nil {
		t.Fatalf("repo root %s has no perfrec/go.mod: %v", root, err)
	}
	return root
}

// TestOnePerfWriterInTheRepo fails when any Go source outside perfrec re-implements the
// writer, with a planted second copy as the negative control proving the scan can see one.
//
// regression: #3101
func TestOnePerfWriterInTheRepo(t *testing.T) {
	// Negative control: a planted copy must be found, or the scan below proves nothing.
	plant := t.TempDir()
	if err := os.WriteFile(filepath.Join(plant, "copy.go"), []byte(
		"package x\n\nvar day = `^(\\d{4}-\\d{2}-\\d{2})\\.jsonl$`\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := secondWriters(t, plant); len(got) != 1 {
		t.Fatalf("the scan missed a planted second writer: %v", got)
	}

	if hits := secondWriters(t, repoRootFromHere(t)); len(hits) != 0 {
		t.Fatalf("a second perf-v1 writer exists outside the shared perfrec module:\n  %s\n"+
			"move it into perfrec and call that (#3101)", strings.Join(hits, "\n  "))
	}
}

// TestPerfWriterConsumersWiring: both recording tools resolve perfrec through the same
// require + relative replace, and both import it from non-test code.
//
// regression: #3101
func TestPerfWriterConsumersWiring(t *testing.T) {
	root := repoRootFromHere(t)
	const mod = "github.com/medici-finance/assay/perfrec"
	for _, c := range []struct{ dir, replace string }{
		{"tools/desk", "../../perfrec"},
		{"statusgen", "../perfrec"},
	} {
		raw, err := os.ReadFile(filepath.Join(root, c.dir, "go.mod"))
		if err != nil {
			t.Fatalf("%s/go.mod: %v", c.dir, err)
		}
		text := string(raw)
		if !strings.Contains(text, mod+" v0.0.0-00010101000000-000000000000") {
			t.Errorf("%s/go.mod does not require %s", c.dir, mod)
		}
		if !strings.Contains(text, "replace "+mod+" => "+c.replace) {
			t.Errorf("%s/go.mod lacks `replace %s => %s`", c.dir, mod, c.replace)
		}
		if !importsInNonTest(t, filepath.Join(root, c.dir), mod) {
			t.Errorf("no non-test Go file under %s imports %s", c.dir, mod)
		}
	}
}

func importsInNonTest(t *testing.T, dir, path string) bool {
	t.Helper()
	found := false
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || found {
			return nil
		}
		if d.IsDir() {
			if d.Name() == "testdata" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(token.NewFileSet(), p, nil, parser.ImportsOnly)
		if perr != nil {
			return nil
		}
		for _, im := range f.Imports {
			if strings.Trim(im.Path.Value, `"`) == path {
				found = true
			}
		}
		return nil
	})
	return found
}

// TestPerfSharedConstantsAgree pins the equality perfrec cannot assert itself, because it
// cannot import this package: its exit codes and result strings are the desk kit's.
func TestPerfSharedConstantsAgree(t *testing.T) {
	for exit, want := range map[int]string{
		ExitOK: ResultOK, ExitDisabled: ResultDisabled, ExitRefused: ResultRefused,
		ExitUnverifiable: ResultUnverifiable, 42: ResultUnverifiable,
	} {
		if got := perfrec.ResultForExit(exit); got != want {
			t.Errorf("perfrec.ResultForExit(%d) = %q, want the desk kit's %q", exit, got, want)
		}
	}
	if perfrec.RoleNone != "none" || perfrec.Other != "other" {
		t.Errorf("perfrec fallbacks drifted: %q %q", perfrec.RoleNone, perfrec.Other)
	}
}
