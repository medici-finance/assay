package regression

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// shellBudget is the one finite deadline every wrapped shell suite runs under. It is a
// hang net, never a measurement: the fleet suite takes about 23s on an idle host and
// about 90s at load average 34 on 16 cores, so a budget sized to idle speed (the old
// 60s) failed a passing suite under load. A real hang still fails, at this deadline.
// The runners above it (check-floor.sh, the brief's Verify rows) bound each go test
// run longer than this, so the budget's own failure is the one a hang reports.
const shellBudget = 4 * time.Minute

// maxShellBudget caps shellBudget well inside go test's default 10m binary timeout,
// so the budget can grow with observed load but never become unbounded.
const maxShellBudget = 5 * time.Minute

func shellFloor(t *testing.T, relative string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fleet/shim fixture: exercised on Linux/macOS; native Windows behavior has its own suite")
	}
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	out, err := runShellFixture(filepath.Join(root, filepath.FromSlash(relative)), t.TempDir(), shellBudget)
	if err != nil {
		t.Fatalf("fixture suite %s: %v\n%s", relative, err, out)
	}
	t.Log(string(out))
}

// TestReg786FleetHardening pins #786, fixed by 643637114: the
// existing fixture suite checks argv redaction AND transport-error retention.
func TestReg786FleetHardening(t *testing.T) { shellFloor(t, "tools/create-fleet-gitlab_test.sh") }

// TestReg1145ShimCredential pins #1145, fixed by f84dde307. The
// shell oracle is the preserved implementation; its fixtures include the
// later role-token isolation correction, so this never grants ambient auth.
func TestReg1145ShimCredential(t *testing.T) {
	shellFloor(t, "tools/cellctl/tests/gen-shims-gh-token.test.sh")
}

// runShellFixture runs one shell suite under a finite budget. The underlying shell
// assertions are unchanged. The wrapped suites run git, so the child never inherits
// the caller's GIT_* variables, and null global and system config keep a caller's
// hooks out of their repositories. A run the budget cut short reports the deadline.
func runShellFixture(path, tmp string, budget time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", path)
	cmd.Env = FixtureEnv("KUBECONFIG=/dev/null", "TMPDIR="+tmp,
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	cmd.WaitDelay = time.Second
	out, err := cmd.CombinedOutput()
	if err != nil && ctx.Err() != nil {
		return out, fmt.Errorf("fixture exceeded its %s deadline: %w (%v)", budget, ctx.Err(), err)
	}
	return out, err
}

// TestShellDeadline proves the wrapper's deadline is real and finite: a fixture that
// outlives its budget is cut off and reported as a deadline failure, and the budget
// the wrapped suites run under is bounded.
func TestShellDeadline(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fixture")
	}
	if shellBudget <= 0 || shellBudget > maxShellBudget {
		t.Fatalf("shellBudget = %s, want a finite deadline in (0, %s]", shellBudget, maxShellBudget)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "deadline.sh")
	if err := os.WriteFile(path, []byte("#!/usr/bin/env bash\nexec sleep 1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := runShellFixture(path, dir, 20*time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline fixture completed without cancellation: err=%v", err)
	}
}

// shellBudgetFaults returns every runShellFixture call in src whose budget argument
// is not the named shellBudget, outside TestShellDeadline (which must pass a tiny
// budget to prove cancellation).
func shellBudgetFaults(name string, src []byte) (int, []string) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		return 0, []string{fmt.Sprintf("%s: %v", name, err)}
	}
	sites, faults := 0, []string(nil)
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || fn.Name.Name == "TestShellDeadline" {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if id, ok := call.Fun.(*ast.Ident); !ok || id.Name != "runShellFixture" {
				return true
			}
			sites++
			if len(call.Args) != 3 {
				faults = append(faults, fmt.Sprintf("%s: runShellFixture with %d arguments", fset.Position(call.Pos()), len(call.Args)))
				return true
			}
			if id, ok := call.Args[2].(*ast.Ident); !ok || id.Name != "shellBudget" {
				faults = append(faults, fmt.Sprintf("%s: runShellFixture budget is not shellBudget", fset.Position(call.Pos())))
			}
			return true
		})
	}
	return sites, faults
}

// TestShellBudgetNamed is the class guard for regression-shell-idle-deadline: a shell
// fixture run under a literal deadline sized to idle speed instead of the one named,
// load-tolerant shellBudget. A planted control keeps the matcher honest.
func TestShellBudgetNamed(t *testing.T) {
	files, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	sites := 0
	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		n, faults := shellBudgetFaults(file, src)
		sites += n
		if len(faults) > 0 {
			t.Errorf("%s", strings.Join(faults, "\n"))
		}
	}
	if sites < 2 {
		t.Errorf("examined %d runShellFixture sites, want at least 2 (shellFloor and the git-isolation test)", sites)
	}
	planted := `package p
func literal() { _, _ = runShellFixture(p, d, 60*time.Second) }
func other() { _, _ = runShellFixture(p, d, budget) }
func healthy() { _, _ = runShellFixture(p, d, shellBudget) }
func TestShellDeadline() { _, _ = runShellFixture(p, d, time.Millisecond) }
`
	n, faults := shellBudgetFaults("plant.go", []byte(planted))
	got := strings.Join(faults, "\n")
	if n != 3 || len(faults) != 2 || !strings.Contains(got, "plant.go:2:") || !strings.Contains(got, "plant.go:3:") {
		t.Errorf("planted controls: sites=%d faults=%d, want 3 and 2 (lines 2 and 3):\n%s", n, len(faults), got)
	}
}

// TestShellGitIsolation runs a planted shell fixture that creates and commits to its
// own repository under TMPDIR while GIT_DIR names a second repository. The wrapper
// must keep that repository byte-unchanged, as it must for the wrapped suites, which
// also run git.
func TestShellGitIsolation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fixture")
	}
	victim := HostileGitDir(t)
	before := SnapshotTree(t, victim)
	dir := t.TempDir()
	path := filepath.Join(dir, "plant.sh")
	script := "#!/usr/bin/env bash\nset -eu\nexport GIT_CONFIG_NOSYSTEM=1\n" +
		"git init -q \"$TMPDIR/plant\"\n" +
		"git -C \"$TMPDIR/plant\" -c user.name=Plant -c user.email=plant@example.invalid " +
		"-c commit.gpgsign=false commit -q --allow-empty -m plant\n"
	if err := os.WriteFile(path, []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := runShellFixture(path, dir, shellBudget)
	if changes := before.Changes(SnapshotTree(t, victim)); changes != "" {
		t.Fatalf("shell fixture wrote to the GIT_DIR-named repository\n%s\n%s", changes, out)
	}
	if err != nil {
		t.Fatalf("planted fixture failed in its own repository: %v\n%s", err, out)
	}
}
