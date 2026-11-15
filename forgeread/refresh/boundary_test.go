package refresh

import (
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// regression: #5399
func TestRefreshBoundary(t *testing.T) {
	// Environment-import syntax is owned by the existing source checker and its
	// registered always-on regression reader, not copied into this package.
	entries, e := os.ReadDir(".")
	if e != nil {
		t.Fatal(e)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		f, e := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, parser.ImportsOnly)
		if e != nil {
			t.Fatal(e)
		}
		for _, i := range f.Imports {
			p, e := strconv.Unquote(i.Path.Value)
			if e != nil {
				t.Fatal(e)
			}
			if p == "net" || p == "net/http" || p == "database/sql" || p == "os/exec" || strings.Contains(p, "/workflow") || strings.Contains(p, "/statusgen") || strings.Contains(p, "credential") {
				t.Fatalf("forbidden coordinator dependency: %s", p)
			}
		}
	}
	mod, e := os.ReadFile(filepath.Join("..", "go.mod"))
	if e != nil {
		t.Fatal(e)
	}
	for _, line := range strings.Split(string(mod), "\n") {
		f := strings.Fields(line)
		if len(f) > 0 && (f[0] == "require" || f[0] == "replace") {
			t.Fatal("forgeread acquired an external module dependency")
		}
	}
	for _, pkg := range []string{".", "./mirror", "./refresh"} {
		cmd := exec.Command("go", "list", "-deps", "-f", "{{.Standard}} {{.ImportPath}}", pkg)
		cmd.Dir = ".."
		cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off", "GOTOOLCHAIN=local")
		out, e := cmd.CombinedOutput()
		if e != nil {
			t.Fatalf("actual dependency graph: %v %s", e, out)
		}
		for _, line := range strings.Split(string(out), "\n") {
			f := strings.Fields(line)
			if len(f) != 2 {
				continue
			}
			p := f[1]
			if f[0] == "false" && !strings.HasPrefix(p, "github.com/medici-finance/assay/forgeread") {
				t.Fatalf("external dependency: %s", p)
			}
			if pkg == "." && strings.HasPrefix(p, "github.com/medici-finance/assay/forgeread/") {
				t.Fatalf("root imports subpackage: %s", p)
			}
			if pkg == "./mirror" && strings.Contains(p, "/refresh") {
				t.Fatal("pure mirror imports orchestration")
			}
			if pkg == "./refresh" && (p == "net" || p == "net/http" || p == "database/sql" || p == "os/exec") {
				t.Fatalf("forbidden transitive dependency: %s", p)
			}
		}
	}
}
