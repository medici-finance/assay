package mirror_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func checkSources(dir string) error {
	return filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range parsed.Imports {
			dep, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return err
			}
			if dep == "os" || dep == "syscall" || dep == "net/http" || dep == "os/exec" || dep == "database/sql" || strings.Contains(dep, "/statusgen") || strings.Contains(dep, "/workflow") {
				return &badImport{dep}
			}
		}
		return nil
	})
}

type badImport struct{ name string }

func (e *badImport) Error() string { return "forbidden kernel import " + e.name }
func runGo(t *testing.T, dir string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off", "GOTOOLCHAIN=local")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go %v: %v\n%s", args, err, out)
	}
	return out
}

// regression: forge-neutral/46
func TestMirrorKernelBoundaries(t *testing.T) {
	if err := checkSources("."); err != nil {
		t.Fatal(err)
	}
	mod, err := os.ReadFile("../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(mod), "\n") {
		f := strings.Fields(line)
		if len(f) > 0 && (f[0] == "require" || f[0] == "replace") {
			t.Fatal("new module dependency")
		}
	}
	for _, pkg := range []string{".", "./mirror", "./mirror/mirrortest"} {
		deps := strings.Fields(string(runGo(t, "..", "list", "-deps", "-f", "{{if not .Standard}}{{.ImportPath}}{{end}}", pkg)))
		for _, dep := range deps {
			if !strings.HasPrefix(dep, "github.com/medici-finance/assay/forgeread") {
				t.Fatalf("external dependency %s", dep)
			}
			if pkg == "." && dep != "github.com/medici-finance/assay/forgeread" {
				t.Fatalf("root linked subpackage %s", dep)
			}
		}
	}
	// Source-only plants are parsed regardless of build tags or quote spelling.
	for _, plant := range []string{"package mirror\nimport _ \"net/http\"\n", "//go:build windows\n\npackage mirror\nimport _ \"os/exec\"\n", "package mirror\nimport _ `database/sql`\n"} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "plant.go"), []byte(plant), 0600); err != nil {
			t.Fatal(err)
		}
		if err := checkSources(dir); err == nil {
			t.Fatal("source boundary missed planted import")
		}
	}
	// Compile and exercise the existing actual checker, rather than a copy of it.
	binary := filepath.Join(t.TempDir(), "source-check")
	runGo(t, "../..", "build", "-o", binary, "tools/forge-import-check/check_env_imports.go")
	for _, plant := range []string{"package mirror\n", "package mirror\nimport _ `os`\n", "//go:build windows\n\npackage mirror\nimport _ \"\\x6f\\x73\"\n", "package mirror\nimport _ \"syscall\"\n"} {
		dir := t.TempDir()
		target := filepath.Join(dir, "forgeread", "mirror")
		if err := os.MkdirAll(target, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(target, "plant.go"), []byte(plant), 0600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(binary, "forgeread/mirror")
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if plant == "package mirror\n" {
			if err != nil || strings.TrimSpace(string(out)) != "0" {
				t.Fatalf("checker positive: %v %s", err, out)
			}
		} else if err == nil || !strings.Contains(string(out), "forbidden import") {
			t.Fatalf("checker missed plant: %v %s", err, out)
		}
	}
	missing := exec.Command(binary, "forgeread/mirror")
	missing.Dir = t.TempDir()
	if err := missing.Run(); err == nil {
		t.Fatal("missing source counted clean")
	}
}
