package gitquiet

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/medici-finance/assay/gitquiet/goquiet"
)

// --- go telemetry class guard (#4006) ------------------------------------------------
//
// The go command starts a telemetry child it does not wait for, in the telemetry
// directory under the config directory HOME selects. A test that moves HOME to a
// scratch directory and runs go there leaves that child racing the directory's
// removal. The shared choke point is the goquiet package of the gitquiet module
// (Off, Home); a package-local helper that runs `go telemetry off` itself (cellcache's
// goEnv, perfrec's leaf-module helper) also counts.

// testOnly reports whether a file is compiled only into a test binary: a _test.go file,
// or a non-test file that imports "testing" (a shared test harness package).
func testOnly(f *ast.File, name string) bool {
	if strings.HasSuffix(name, "_test.go") {
		return true
	}
	for _, imp := range f.Imports {
		if p, _ := strconv.Unquote(imp.Path.Value); p == "testing" {
			return true
		}
	}
	return false
}

// homeScan reports what one file does: whether it moves HOME (or XDG_CONFIG_HOME, which
// moves the config directory directly) through a Setenv call or a "HOME=…" env entry,
// whether it names the go binary (a "go" string literal), and whether it quiets go
// telemetry (a goquiet call, or a call whose arguments run `telemetry off`).
func homeScan(f *ast.File) (moves, namesGo, quiet bool) {
	str := func(e ast.Expr) (string, bool) {
		lit, ok := e.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return "", false
		}
		s, err := strconv.Unquote(lit.Value)
		return s, err == nil
	}
	homeKey := func(s string) bool { return s == "HOME" || s == "XDG_CONFIG_HOME" }
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.BasicLit:
			if s, ok := str(n); ok {
				namesGo = namesGo || s == "go"
				k, _, eq := strings.Cut(s, "=")
				moves = moves || eq && homeKey(k)
			}
		case *ast.CallExpr:
			if sel, ok := n.Fun.(*ast.SelectorExpr); ok {
				if x, ok := sel.X.(*ast.Ident); ok && x.Name == "goquiet" {
					quiet = true
				}
				if sel.Sel.Name == "Setenv" && len(n.Args) > 0 {
					if s, ok := str(n.Args[0]); ok && homeKey(s) {
						moves = true
					}
				}
			}
			for i := 0; i+1 < len(n.Args); i++ {
				a, _ := str(n.Args[i])
				b, _ := str(n.Args[i+1])
				quiet = quiet || a == "telemetry" && b == "off"
			}
		}
		return true
	})
	return moves, namesGo, quiet
}

// unquietHome walks root and returns, sorted and relative to root, every package whose
// test-only code both moves HOME and names the go binary but never quiets go telemetry.
// It is a package-level reach: one quieting call anywhere in the package satisfies it,
// and a package whose tests reach cmd/go only through another package's code is not
// matched. It also returns how many packages matched both arms, so a scan that matched
// nothing is visible as such.
func unquietHome(root string) (missing []string, examined int, err error) {
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if !d.IsDir() {
			return nil
		}
		if path != root && (d.Name() == "testdata" || d.Name() == "vendor" || d.Name() == "node_modules" || strings.HasPrefix(d.Name(), ".")) {
			return filepath.SkipDir
		}
		entries, rerr := os.ReadDir(path)
		if rerr != nil {
			return rerr
		}
		fset := token.NewFileSet()
		moves, namesGo, quiet := false, false, false
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
				continue
			}
			f, perr := parser.ParseFile(fset, filepath.Join(path, e.Name()), nil, parser.SkipObjectResolution)
			if perr != nil {
				return perr
			}
			if !testOnly(f, e.Name()) {
				continue
			}
			m, g, q := homeScan(f)
			moves, namesGo, quiet = moves || m, namesGo || g, quiet || q
		}
		if !moves || !namesGo {
			return nil
		}
		examined++
		if !quiet {
			rel, _ := filepath.Rel(root, path)
			missing = append(missing, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(missing)
	return missing, examined, err
}

// noGoUnderHome records the packages the scan matches whose tests were read and found
// NOT to run go with a moved HOME: each runs go only before HOME moves or with an
// environment that keeps the real home, or moves HOME only for a non-go binary. An entry
// here is a reading, not a waiver: a new test in the package that does run go under a
// scratch home belongs on goquiet.Home or goquiet.Off instead.
var noGoUnderHome = map[string]string{
	// A leaf module that cannot import goquiet. Its go commands (goCmd, depsOf) inherit
	// this process's environment from tests that never move HOME; the two tests that
	// t.Setenv HOME call the recorder directly, and every "HOME=" entry is the
	// environment of the built consumer binary, not of a go command.
	"perfrec": "go never runs with a moved HOME",
}

// TestGoPackagesQuietHome — the class guard. Any package in this repository whose tests
// move HOME and run go must turn go telemetry off in that home through goquiet (or a
// package-local `go telemetry off` helper), so a scratch home in a NEW package cannot
// bring the telemetry child's cleanup race back. Fix a failure with goquiet.Home(t) in
// place of t.TempDir() for the home, or goquiet.Off(nil) right after os.Setenv("HOME",
// …) in a TestMain fixture; or, when the package's go commands never see the moved
// HOME, record that reading in noGoUnderHome.
//
// regression: #4006
func TestGoPackagesQuietHome(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..")
	missing, examined, err := unquietHome(root)
	if err != nil {
		t.Fatal(err)
	}
	// Non-vacuous: the scan matched the packages this guard was written for.
	if examined < 20 {
		t.Fatalf("the scan found only %d packages whose tests move HOME and run go; the matcher has stopped matching", examined)
	}
	var bad []string
	flagged := map[string]bool{}
	for _, p := range missing {
		flagged[p] = true
		if noGoUnderHome[p] == "" {
			bad = append(bad, p)
		}
	}
	for p := range noGoUnderHome {
		if !flagged[p] {
			bad = append(bad, p+" (recorded in noGoUnderHome but no longer matched: drop the entry)")
		}
	}
	sort.Strings(bad)
	if len(bad) > 0 {
		t.Fatalf("%d package(s) move HOME and run go in tests without turning go telemetry off there, so "+
			"a go command's telemetry child races the scratch home's removal:\n  %s",
			len(bad), strings.Join(bad, "\n  "))
	}
}

// TestGoGuardControl — the guard's positive control. testdata/goguard holds a package
// that sets HOME with t.Setenv and runs go (must be flagged), one that passes a
// "HOME=" entry to a go command (must be flagged), one that moves XDG_CONFIG_HOME
// (must be flagged), one whose go launch sits in a non-test harness file (must be
// flagged), one on goquiet.Home and one on goquiet.Off (must pass), one with a
// package-local `go telemetry off` helper (must pass), and one that runs go without
// moving HOME and one that moves HOME without running go (both ignored).
//
// regression: #4006
func TestGoGuardControl(t *testing.T) {
	missing, examined, err := unquietHome(filepath.Join("testdata", "goguard"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"envlist", "harness", "setenv", "xdg"}
	if strings.Join(missing, ",") != strings.Join(want, ",") || examined != 7 {
		t.Fatalf("guard over testdata/goguard flagged %v of %d packages; want %v of 7", missing, examined, want)
	}
}

// scratchGoEnv is this process's environment with HOME moved to home and
// XDG_CONFIG_HOME dropped, so the go command's config directory (and its telemetry
// directory under it) follows the scratch home on every Unix.
func scratchGoEnv(home string) []string {
	var env []string
	for _, kv := range os.Environ() {
		if k, _, _ := strings.Cut(kv, "="); k == "XDG_CONFIG_HOME" || k == "HOME" {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "HOME="+home, "GOTOOLCHAIN=local", "GOFLAGS=")
}

// TestGoQuietOffPin — the deterministic pin for the defect goquiet closes. A go command
// in a fresh scratch home writes telemetry there and, on the first run of the day,
// starts a telemetry child it does not wait for. The red arm proves the first half
// without the race: a fresh upload token is planted so the child cannot start, and the
// go command still writes counter files in the scratch home, which Wrote reports. The
// green arm runs the same go command through goquiet.Cmd: the telemetry directory holds
// only the mode file (off), so no counter was opened and no upload token was taken,
// which is the precondition for the child. goquiet.Home's cleanup check is the same
// Wrote call.
//
// regression: #4006
func TestGoQuietOffPin(t *testing.T) {
	if runtime.GOOS == "windows" || runtime.GOOS == "plan9" {
		t.Skip("the go telemetry directory does not follow HOME on " + runtime.GOOS)
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Fatalf("no go command on PATH: %v", err)
	}

	// Red: telemetry left on in the scratch home.
	red := scratchGoEnv(t.TempDir())
	dir := goquiet.TelemetryDir(red)
	if dir == "" {
		t.Fatal("a moved HOME gave no scratch telemetry directory")
	}
	if err := os.MkdirAll(filepath.Join(dir, "local"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "local", "upload.token"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "env", "GOVERSION")
	cmd.Env, cmd.Dir = red, os.TempDir()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go env: %v: %s", err, out)
	}
	wrote, err := goquiet.Wrote(red)
	if err != nil {
		t.Fatal(err)
	}
	counted := false
	for _, w := range wrote {
		counted = counted || strings.HasSuffix(w, ".count")
	}
	if !counted {
		t.Fatalf("red arm: a go command with telemetry on wrote %q in the scratch home; want a counter file (the pin no longer reproduces the defect)", wrote)
	}

	// Green: the same go command through goquiet.Cmd.
	green := scratchGoEnv(t.TempDir())
	cmd = exec.Command("go", "env", "GOVERSION")
	cmd.Env, cmd.Dir = green, os.TempDir()
	if out, err := goquiet.Cmd(cmd).CombinedOutput(); err != nil {
		t.Fatalf("go env through goquiet.Cmd: %v: %s", err, out)
	}
	if wrote, err := goquiet.Wrote(green); err != nil || len(wrote) > 0 {
		t.Fatalf("green arm: goquiet.Cmd left telemetry on in the scratch home: wrote %q (err %v)", wrote, err)
	}
	mode, err := os.ReadFile(filepath.Join(goquiet.TelemetryDir(green), "mode"))
	if err != nil || !strings.HasPrefix(string(mode), "off") {
		t.Fatalf("green arm: mode file %q (err %v); want off", mode, err)
	}
}

// TestGoQuietHostUntouched — Off never changes the developer's own telemetry setting:
// an environment whose config directory is this process's own has no scratch telemetry
// directory, and Off on it runs nothing.
//
// regression: #4006
func TestGoQuietHostUntouched(t *testing.T) {
	if d := goquiet.TelemetryDir(nil); d != "" {
		t.Fatalf("TelemetryDir(this environment) = %q; want \"\" (the host directory is never scratch)", d)
	}
	if d := goquiet.TelemetryDir([]string{"HOME=relative"}); d != "" && runtime.GOOS != "windows" {
		t.Fatalf("TelemetryDir(relative HOME) = %q; want \"\"", d)
	}
	if err := goquiet.Off(nil); err != nil {
		t.Fatal(err)
	}
}
