package deskkit

// perfshared_test.go — the perf writer (perf-v1 and perf-v2) exists ONCE, in the shared perfrec module, and every
// recording tool calls it. #3101: statusgen records the same local perf-v1 line as the desk
// tools, and a second copy of the writer (day-file layout, retention, install id, schema) is
// exactly the drift this guards against.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/medici-finance/assay/perfrec"
)

// dayFilePattern matches a literal that names a date followed by .jsonl — a per-day record
// file, which is what the perf writer's day-file regexp is. (The audit and CI-check ledgers
// put the date AFTER .jsonl, so they are different files and do not match.)
var dayFilePattern = regexp.MustCompile(`\\d\{4\}-\\d\{2\}-\\d\{2\}\)?\\\.jsonl`)

// secondWriters returns the non-test Go files under root, outside the perfrec directory, that
// carry a perf writer's fingerprint in a string literal or call: a perf-v1 literal or the bare
// perf-v2 tag; a day-file
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
				// perf-v1 as before (any literal naming it); perf-v2 as the bare tag, since
				// deskperf's report legitimately labels its perf-v2 group in prose.
				schema = schema || strings.Contains(v, "perf-v1") || v == strconv.Quote(perfrec.SchemaV2) || v == "`"+perfrec.SchemaV2+"`"
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
			{schema, "perf schema literal"}, {dayFile, "day-file pattern"},
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
	plantV2 := t.TempDir()
	if err := os.WriteFile(filepath.Join(plantV2, "copy.go"), []byte("package x\n\nconst schema = \"perf-v2\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := secondWriters(t, plantV2); len(got) != 1 {
		t.Fatalf("the scan missed a planted perf-v2 schema tag: %v", got)
	}

	if hits := secondWriters(t, repoRootFromHere(t)); len(hits) != 0 {
		t.Fatalf("a second perf writer exists outside the shared perfrec module:\n  %s\n"+
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

// perfrecViolations scans every non-test Go file in dir (all build tags, so the per-OS CPU
// readers are covered too) and returns what breaks the shared writer's local-only and
// ledger-isolation contract: a string literal naming the audit ledger, a reference to a
// ledger function, or an import that is not the standard library or that is a network or
// process-launch package.
func perfrecViolations(t *testing.T, dir string) []string {
	t.Helper()
	ledgerFuncs := map[string]bool{"auditPath": true, "segmentPaths": true, "Log": true, "LoadEntries": true,
		"loadEntriesFrom": true, "LastEntry": true, "FirstTS": true, "rotateIfNeeded": true}
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatalf("no Go files under %s", dir)
	}
	var bad []string
	for _, p := range files {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), p, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}
		base := filepath.Base(p)
		for _, imp := range f.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			first, _, _ := strings.Cut(path, "/")
			if strings.Contains(first, ".") || path == "net" || strings.HasPrefix(path, "net/") || path == "os/exec" {
				bad = append(bad, base+": imports "+path)
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.BasicLit:
				if x.Kind == token.STRING && strings.Contains(strings.ToLower(x.Value), "audit") {
					bad = append(bad, base+": string literal "+x.Value+" names the audit ledger")
				}
			case *ast.Ident:
				if ledgerFuncs[x.Name] {
					bad = append(bad, base+": references ledger function "+x.Name)
				}
			}
			return true
		})
	}
	return bad
}

// TestPerfrecStaysLocalAndOffLedger covers the shared writer's own sources, including the
// per-OS CPU reader files, from a suite CI runs. The perfrec module's own tests assert the
// same contract but no CI job runs that module's suite, so this is the guard that does. Each
// planted case is the negative control proving the scan sees that violation.
//
// regression: #3101
func TestPerfrecStaysLocalAndOffLedger(t *testing.T) {
	for name, src := range map[string]string{
		"network import":      "package p\n\nimport _ \"net/http\"\n",
		"net import":          "package p\n\nimport _ \"net\"\n",
		"process launch":      "package p\n\nimport _ \"os/exec\"\n",
		"third-party":         "package p\n\nimport _ \"example.com/x/y\"\n",
		"ledger literal":      "package p\n\nvar f = \"audit.jsonl\"\n",
		"ledger function":     "package p\n\nfunc f() { LoadEntries() }\n",
		"tag-excluded reader": "//go:build unix\n\npackage p\n\nimport _ \"net/http\"\n",
	} {
		t.Run("planted "+name, func(t *testing.T) {
			plant := t.TempDir()
			fname := "zz_planted.go"
			if name == "tag-excluded reader" {
				fname = "cpu_windows.go" // excluded from a linux build, so only a tag-blind scan sees it
			}
			if err := os.WriteFile(filepath.Join(plant, fname), []byte(src), 0o600); err != nil {
				t.Fatal(err)
			}
			if got := perfrecViolations(t, plant); len(got) != 1 {
				t.Fatalf("the scan missed a planted %s: %v", name, got)
			}
		})
	}

	if bad := perfrecViolations(t, filepath.Join(repoRootFromHere(t), "perfrec")); len(bad) != 0 {
		t.Fatalf("the shared perf writer breaks its local-only / off-ledger contract:\n  %s", strings.Join(bad, "\n  "))
	}
}

// perfrecClosure returns the forbidden packages in perfrec's full dependency closure under
// the given GOOS (go list -deps): the standard library's own transitive imports included, so
// a stdlib package that itself reaches the network is caught where the direct-import scan
// above cannot see it.
func perfrecClosure(t *testing.T, dir, goos string) []string {
	t.Helper()
	cmd := exec.Command("go", "list", "-deps", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod", "GOOS="+goos)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list -deps (GOOS=%s) in %s: %v", goos, dir, err)
	}
	var bad []string
	for _, d := range strings.Fields(string(out)) {
		first, _, _ := strings.Cut(d, "/")
		if d == "net" || strings.HasPrefix(d, "net/") || d == "os/exec" ||
			(strings.Contains(first, ".") && d != "github.com/medici-finance/assay/perfrec") {
			bad = append(bad, d)
		}
	}
	return bad
}

// TestPerfrecClosureCarriesNothingOnline: the shared writer's dependency closure holds no
// network or process-launch package on any supported OS, with a planted import as the
// negative control.
//
// regression: #3101
func TestPerfrecClosureCarriesNothingOnline(t *testing.T) {
	src := filepath.Join(repoRootFromHere(t), "perfrec")

	plant := t.TempDir()
	srcs, err := filepath.Glob(filepath.Join(src, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	files := []string{"go.mod"}
	for _, p := range srcs {
		if !strings.HasSuffix(p, "_test.go") {
			files = append(files, filepath.Base(p))
		}
	}
	for _, f := range files {
		b, err := os.ReadFile(filepath.Join(src, f))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(plant, f), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(plant, "zz_planted.go"), []byte("package perfrec\n\nimport _ \"net/http\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := perfrecClosure(t, plant, "linux"); len(got) == 0 {
		t.Fatal("the closure check missed a planted net/http import")
	}

	for _, goos := range []string{"linux", "darwin", "windows"} {
		if bad := perfrecClosure(t, src, goos); len(bad) != 0 {
			t.Errorf("perfrec's closure on %s reaches %v — it must stay standard-library, offline", goos, bad)
		}
	}
}
