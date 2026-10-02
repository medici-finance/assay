package regression

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
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

// cleanEnvFuncs are the two constructors a floor subprocess may take its Env from:
// FixtureEnv (the caller's environment minus GIT_*) and fixedGitEnv (a literal list,
// used only for HostileGitDir's victim so a mutated FixtureEnv cannot reach it).
var cleanEnvFuncs = map[string]bool{"FixtureEnv": true, "fixedGitEnv": true}

func callsFixtureEnv(e ast.Expr) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			switch f := c.Fun.(type) {
			case *ast.Ident:
				found = found || cleanEnvFuncs[f.Name]
			case *ast.SelectorExpr:
				found = found || cleanEnvFuncs[f.Sel.Name]
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

// TestFloorRunnerGitIsolation runs the floor runner itself, check-floor.sh, over a
// one-row manifest whose planted test runs git with the environment it inherits,
// while GIT_DIR, GIT_WORK_TREE and GIT_INDEX_FILE name a second repository. Reused
// manifest rows keep their own fixtures, so the runner's scrub is what stands between
// them and the caller's repository: it must stay byte-unchanged.
func TestFloorRunnerGitIsolation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX runner: check-floor.sh is the Linux/macOS entrypoint")
	}
	victim := HostileGitDir(t)
	before := TreeDigest(t, victim)
	manifest, err := filepath.Abs(filepath.Join("testdata", "runner-manifest.md"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "check-floor.sh", manifest)
	cmd.Env = FixtureEnv("KUBECONFIG=/dev/null",
		"GIT_DIR="+filepath.Join(victim, ".git"), "GIT_WORK_TREE="+victim,
		"GIT_INDEX_FILE="+filepath.Join(victim, ".git", "index"))
	cmd.WaitDelay = time.Second
	out, err := cmd.CombinedOutput()
	if after := TreeDigest(t, victim); after != before {
		t.Fatalf("floor runner let a row write to the GIT_DIR-named repository %s\n%s", victim, out)
	}
	if err != nil || !strings.Contains(string(out), "seed passes=1") {
		t.Fatalf("floor runner over the planted manifest: %v\n%s", err, out)
	}
}

var (
	shGoSpawn = regexp.MustCompile(`(^|[\s;&|(])go\s+\S`)
	pyGoSpawn = regexp.MustCompile(`["']go["']\s*[,\]]`)
)

// goSpawnFaults reports every non-comment line of a floor script that starts the go
// tool other than through floor-go.sh: a shell `go <args>` command, or a Python
// argv element that is exactly "go".
func goSpawnFaults(name, src string) []string {
	var faults []string
	for i, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if shGoSpawn.MatchString(line) || pyGoSpawn.MatchString(line) {
			faults = append(faults, fmt.Sprintf("%s:%d: starts go outside floor-go.sh", name, i+1))
		}
	}
	return faults
}

// TestFloorGoChokePoint is the class guard for the runner half of
// regression-fixture-git-env-isolation: every floor script (shell or Python, at any
// depth under this package) must start the go tool through floor-go.sh, the one place
// the caller's GIT_* variables are cleared. Go sources are TestFloorExecEnv's.
func TestFloorGoChokePoint(t *testing.T) {
	examined := map[string]bool{}
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		ext := filepath.Ext(path)
		if (ext != ".sh" && ext != ".py") || path == "floor-go.sh" {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		examined[filepath.ToSlash(path)] = true
		for _, f := range goSpawnFaults(filepath.ToSlash(path), string(src)) {
			t.Error(f)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"check-floor.sh", "testdata/mutate_guard.py"} {
		if !examined[want] {
			t.Errorf("floor script %s was not examined", want)
		}
	}

	planted := "go test ./x\nfoo && exec go vet .\n# go test in a comment\n" +
		"bash \"$here/floor-go.sh\" test -run x\n" +
		"subprocess.run([\"go\", \"test\"])\nsubprocess.run([\"bash\", floor_go, \"test\"])\n"
	got := strings.Join(goSpawnFaults("plant", planted), "\n")
	want := "plant:1: starts go outside floor-go.sh\nplant:2: starts go outside floor-go.sh\n" +
		"plant:5: starts go outside floor-go.sh"
	if got != want {
		t.Errorf("planted controls: got\n%s\nwant\n%s", got, want)
	}
}
