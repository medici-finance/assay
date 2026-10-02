package regression

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// floorFiles returns the code this floor introduced: every Go source in this package
// plus each file declaring a TestReg* entry point named in MANIFEST.md. Reused
// pre-existing tests keep their own fixtures and are outside this set.
func floorFiles(t *testing.T, root string) []string {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for i, f := range files {
		files[i] = filepath.Join(root, "tools", "desk", "internal", "regression", f)
	}
	body, err := os.ReadFile("MANIFEST.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(body), "\n") {
		if strings.HasPrefix(line, "## Dropped") {
			break
		}
		cells := strings.Split(line, "|")
		if len(cells) != 7 || !strings.HasPrefix(strings.TrimSpace(cells[4]), "TestReg") {
			continue
		}
		pkg, name := strings.TrimSpace(cells[3]), strings.TrimSpace(cells[4])
		if pkg == "tools/desk/internal/regression" {
			continue
		}
		matches, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(pkg), "*_test.go"))
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, m := range matches {
			src, err := os.ReadFile(m)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(src), "func "+name+"(") {
				files, found = append(files, m), true
			}
		}
		if !found {
			t.Fatalf("floor entry point %s not found in %s", name, pkg)
		}
	}
	return files
}

func isCall(c *ast.CallExpr, pkg string, names ...string) bool {
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if id, ok := sel.X.(*ast.Ident); !ok || id.Name != pkg {
		return false
	}
	for _, n := range names {
		if sel.Sel.Name == n {
			return true
		}
	}
	return false
}

func callsFixtureEnv(e ast.Expr) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			switch f := c.Fun.(type) {
			case *ast.Ident:
				found = found || f.Name == "FixtureEnv"
			case *ast.SelectorExpr:
				found = found || f.Sel.Name == "FixtureEnv"
			}
		}
		return !found
	})
	return found
}

// execEnvFaults reports every subprocess site in src that can hand the caller's GIT_*
// variables to a child: an exec.Command/CommandContext result whose own .Env is not
// assigned from FixtureEnv in the same function, and any os.Environ() read outside
// fixtureenv.go. It returns the number of subprocess sites examined.
func execEnvFaults(name string, src []byte) (int, []string) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		return 0, []string{err.Error()}
	}
	sites := 0
	var faults []string
	inherits := func(c *ast.CallExpr) {
		faults = append(faults, fmt.Sprintf("%s: subprocess inherits the caller's GIT_* environment", fset.Position(c.Pos())))
	}
	var fn func(body *ast.BlockStmt)
	fn = func(body *ast.BlockStmt) {
		bound := map[*ast.CallExpr]string{}
		var execs []*ast.CallExpr
		clean := map[string]bool{}
		ast.Inspect(body, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncLit:
				fn(n.Body)
				return false
			case *ast.AssignStmt:
				for i, rhs := range n.Rhs {
					if c, ok := rhs.(*ast.CallExpr); ok && i < len(n.Lhs) {
						if id, ok := n.Lhs[i].(*ast.Ident); ok {
							bound[c] = id.Name
						}
					}
					if sel, ok := n.Lhs[i].(*ast.SelectorExpr); ok && sel.Sel.Name == "Env" && callsFixtureEnv(rhs) {
						if id, ok := sel.X.(*ast.Ident); ok {
							clean[id.Name] = true
						}
					}
				}
			case *ast.CallExpr:
				if isCall(n, "exec", "Command", "CommandContext") {
					execs = append(execs, n)
				}
			}
			return true
		})
		for _, c := range execs {
			sites++
			if v, ok := bound[c]; !ok || !clean[v] {
				inherits(c)
			}
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncDecl:
			if n.Body != nil {
				fn(n.Body)
			}
			return false
		case *ast.FuncLit:
			fn(n.Body)
			return false
		case *ast.CallExpr:
			if isCall(n, "exec", "Command", "CommandContext") {
				sites++
				inherits(n)
			}
		}
		return true
	})
	if filepath.Base(name) != "fixtureenv.go" {
		ast.Inspect(f, func(n ast.Node) bool {
			if c, ok := n.(*ast.CallExpr); ok && isCall(c, "os", "Environ") {
				faults = append(faults, fmt.Sprintf("%s: os.Environ() outside FixtureEnv", fset.Position(c.Pos())))
			}
			return true
		})
	}
	return sites, faults
}

// TestFloorExecEnv is the class guard for regression-fixture-git-env-isolation: no
// floor fixture may start git, or a shell that runs git, with an environment that can
// carry the caller's GIT_DIR. The planted controls keep the matcher honest.
func TestFloorExecEnv(t *testing.T) {
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	sites, examined := 0, map[string]bool{}
	for _, file := range floorFiles(t, root) {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		rel, _ := filepath.Rel(root, file)
		n, faults := execEnvFaults(filepath.ToSlash(rel), src)
		if len(faults) > 0 {
			t.Errorf("%s", strings.Join(faults, "\n"))
		}
		sites += n
		examined[filepath.ToSlash(rel)] = true
	}
	for _, want := range []string{
		"tools/desk/cmd/deskclaim-ref/regression_test.go",
		"tools/desk/internal/regression/shell_test.go",
		"tools/desk/internal/regression/fixtureenv.go",
	} {
		if !examined[want] {
			t.Errorf("floor file %s was not examined", want)
		}
	}
	if sites < 3 {
		t.Errorf("examined %d subprocess sites, want at least 3", sites)
	}

	planted := `package p
func inline() { _ = exec.Command("git", "status").Run() }
func inherited() { cmd := exec.Command("git"); cmd.Env = append(os.Environ(), "A=1"); _ = cmd.Run() }
func second() { a := exec.Command("git"); a.Env = FixtureEnv(); b := exec.Command("git"); _, _ = a, b }
func closure() { run := func() { c := exec.CommandContext(ctx, "bash"); _ = c.Run() }; run() }
func healthy() { c := exec.Command("git"); c.Env = regression.FixtureEnv("A=1"); _ = c.Run() }
`
	n, faults := execEnvFaults("plant.go", []byte(planted))
	got := strings.Join(faults, "\n")
	for _, want := range []string{"plant.go:2:", "plant.go:3:", "plant.go:4:", "plant.go:5:", "os.Environ() outside FixtureEnv"} {
		if !strings.Contains(got, want) {
			t.Errorf("planted site %s was missed:\n%s", want, got)
		}
	}
	if n != 6 || len(faults) != 5 || strings.Contains(got, "plant.go:6:") {
		t.Errorf("planted controls: sites=%d faults=%d, want 6 and 5 with the healthy site clean:\n%s", n, len(faults), got)
	}
}
