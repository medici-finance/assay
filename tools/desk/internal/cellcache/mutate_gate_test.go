//go:build !windows

package cellcache

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The cache mutation gate (mutate.py) is run in full by a separate, slow job.
// These tests run it against a tiny throwaway module so every state it reports
// is proven on each ordinary test run: a control that fires, a non-compiling
// mutant that is could-not-mutate, a path that escapes the module, and a run
// killed mid-mutation that leaves the source tree clean.

const fxSource = "package fx\n\nfunc Cost(total int) int {\n\tif total >= 50 {\n\t\treturn 0\n\t}\n\treturn 5\n}\n"

const fxTest = "package fx\n\nimport \"testing\"\n\nfunc TestCost(t *testing.T) {\n" +
	"\tif Cost(10) != 5 {\n\t\tt.Errorf(\"low total\")\n\t}\n" +
	"\tif Cost(60) != 0 {\n\t\tt.Errorf(\"high total\")\n\t}\n}\n"

type fxMutation struct {
	Name string `json:"name"`
	File string `json:"file"`
	Old  string `json:"old"`
	New  string `json:"new"`
}

func fxCaught(name string) fxMutation {
	return fxMutation{Name: name, File: "fx.go", Old: "return 5", New: "return 0"}
}

func fxControl() fxMutation { return fxCaught("control") }

// fxModule writes the fixture module and its spec; returns the module dir and
// the spec path. The module sits one level below a directory that also holds
// outside.go, the file an escaping mutation would target.
func fxModule(t *testing.T, test string, control fxMutation, muts ...fxMutation) (string, string) {
	t.Helper()
	top := t.TempDir()
	mod := filepath.Join(top, "mod")
	if err := os.Mkdir(mod, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"go.mod":             "module example.com/fx\n\ngo 1.21\n",
		"fx.go":              fxSource,
		"fx_test.go":         fxTest,
		"../outside.go":      "package outside // return 5\n",
		"../outside.go.orig": "package outside // return 5\n",
	} {
		if err := os.WriteFile(filepath.Join(mod, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := json.Marshal(map[string]any{"test": test, "control": control, "mutations": muts})
	if err != nil {
		t.Fatal(err)
	}
	spec := filepath.Join(top, "spec.json")
	if err := os.WriteFile(spec, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return mod, spec
}

func gateCommand(t *testing.T, mod, spec string, extra ...string) *exec.Cmd {
	t.Helper()
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("could-not-check: python3 is not on PATH, so mutate.py cannot be exercised")
	}
	args := append([]string{"mutate.py", "--spec", spec, "--module", mod}, extra...)
	cmd := exec.Command(py, args...)
	cmd.Dir = "."
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=", "TMPDIR="+t.TempDir())
	return cmd
}

func runGate(t *testing.T, mod, spec string) (string, int) {
	t.Helper()
	out, err := gateCommand(t, mod, spec).CombinedOutput()
	var ee *exec.ExitError
	switch {
	case err == nil:
		return string(out), 0
	case errors.As(err, &ee):
		return string(out), ee.ExitCode()
	default:
		t.Fatalf("run mutate.py: %v", err)
		return "", -1
	}
}

const goSuite = "go test ./... -count=1 -timeout=60s"

func mustContain(t *testing.T, out string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("output lacks %q:\n%s", w, out)
		}
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// regression: #3148
func TestMutateGateAllCaught(t *testing.T) {
	mod, spec := fxModule(t, goSuite, fxControl(), fxCaught("flat rate"))
	out, code := runGate(t, mod, spec)
	if code != 0 {
		t.Fatalf("exit %d, want 0:\n%s", code, out)
	}
	mustContain(t, out, "control caught", "CAUGHT flat rate", "Totals: 1 caught, 0 NOT CAUGHT, 0 could-not-mutate.")
}

// regression: #3148
func TestMutateGateReportsEveryNonCaughtState(t *testing.T) {
	mod, spec := fxModule(t, goSuite, fxControl(),
		fxCaught("flat rate"),
		fxMutation{Name: "threshold shifts", File: "fx.go", Old: "total >= 50", New: "total >= 51"},
		fxMutation{Name: "does not compile", File: "fx.go", Old: "return 0", New: "return undefinedName"},
		fxMutation{Name: "dot dot", File: "../outside.go", Old: "return 5", New: "return 6"},
		fxMutation{Name: "absolute", File: "/etc/hosts", Old: "localhost", New: "elsewhere"},
		fxMutation{Name: "anchor absent", File: "fx.go", Old: "return 77", New: "return 78"},
	)
	out, code := runGate(t, mod, spec)
	if code != 1 {
		t.Fatalf("exit %d, want 1:\n%s", code, out)
	}
	mustContain(t, out,
		"CAUGHT flat rate",
		"MISSED threshold shifts",
		"COULD-NOT-MUTATE does not compile: mutant does not compile",
		"COULD-NOT-MUTATE dot dot: file '../outside.go' escapes the module",
		"COULD-NOT-MUTATE absolute: file '/etc/hosts' is not a relative path",
		"COULD-NOT-MUTATE anchor absent: `old` text not found",
		"Totals: 1 caught, 1 NOT CAUGHT, 4 could-not-mutate.")
	if got := readFile(t, filepath.Join(mod, "../outside.go")); got != readFile(t, filepath.Join(mod, "../outside.go.orig")) {
		t.Errorf("a mutation wrote outside the module: %q", got)
	}
}

// regression: #3148
func TestMutateGateCouldNotMutateAloneFailsTheGate(t *testing.T) {
	mod, spec := fxModule(t, goSuite, fxControl(),
		fxCaught("flat rate"),
		fxMutation{Name: "does not compile", File: "fx.go", Old: "return 0", New: "return undefinedName"})
	out, code := runGate(t, mod, spec)
	if code != 1 {
		t.Fatalf("a non-compiling mutant must fail the gate: exit %d, want 1:\n%s", code, out)
	}
	mustContain(t, out, "Totals: 1 caught, 0 NOT CAUGHT, 1 could-not-mutate.")
}

// regression: #3148
func TestMutateGateControlThatIsNotCaughtBreaksTheHarness(t *testing.T) {
	// The control keeps the suite green, so it cannot fire.
	control := fxMutation{Name: "dead control", File: "fx.go", Old: "total >= 50", New: "total >= 51"}
	mod, spec := fxModule(t, goSuite, control, fxCaught("flat rate"))
	out, code := runGate(t, mod, spec)
	if code != 2 {
		t.Fatalf("exit %d, want 2:\n%s", code, out)
	}
	mustContain(t, out, "HARNESS BROKEN: control 'dead control' is MISSED")
	if strings.Contains(out, "CAUGHT flat rate") {
		t.Errorf("a broken harness printed a per-mutation verdict:\n%s", out)
	}
}

// regression: #3148
func TestMutateGateRedBaselineBreaksTheHarness(t *testing.T) {
	mod, spec := fxModule(t, "go test ./nosuchpackage -count=1", fxControl(), fxCaught("flat rate"))
	out, code := runGate(t, mod, spec)
	if code != 2 {
		t.Fatalf("exit %d, want 2:\n%s", code, out)
	}
	mustContain(t, out, "HARNESS BROKEN: baseline suite is not green")
}

// regression: #3148
func TestMutateGateKilledRunLeavesSourceClean(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "planted")
	suite := `sh -c 'if grep -q HANG fx.go; then touch "$MARK"; sleep 120; fi; if grep -q SENTINEL fx.go; then echo "--- FAIL: control"; exit 1; fi'`
	control := fxMutation{Name: "control", File: "fx.go", Old: "package fx", New: "package fx // SENTINEL"}
	hang := fxMutation{Name: "hang", File: "fx.go", Old: "package fx", New: "package fx // HANG"}
	mod, spec := fxModule(t, suite, control, hang)
	before := readFile(t, filepath.Join(mod, "fx.go"))

	cmd := gateCommand(t, mod, spec)
	cmd.Env = append(cmd.Env, "MARK="+marker)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); _ = cmd.Wait() }()

	deadline := time.Now().Add(60 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the hanging mutant never started:\n%s", buf.String())
		}
		time.Sleep(50 * time.Millisecond)
	}
	// SIGKILL the gate itself, mid-mutation: no handler or finally block runs.
	if err := syscall.Kill(cmd.Process.Pid, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(mod, "fx.go")); got != before {
		t.Fatalf("a killed run left the source tree mutated:\n%s", got)
	}
}

// regression: #3148
func TestShippedSpecDeclaresAControlAndRelativeFiles(t *testing.T) {
	var spec struct {
		Test      string       `json:"test"`
		Control   fxMutation   `json:"control"`
		Mutations []fxMutation `json:"mutations"`
	}
	if err := json.Unmarshal([]byte(readFile(t, "mutations.json")), &spec); err != nil {
		t.Fatal(err)
	}
	if spec.Test == "" || spec.Control.Name == "" || len(spec.Mutations) == 0 {
		t.Fatalf("mutations.json needs test, control and mutations: %+v", spec)
	}
	for _, m := range append([]fxMutation{spec.Control}, spec.Mutations...) {
		clean := filepath.Clean(m.File)
		if filepath.IsAbs(m.File) || strings.HasPrefix(clean, "..") {
			t.Errorf("mutation %q targets %q, outside the module", m.Name, m.File)
		}
		if !strings.Contains(readFile(t, filepath.Join("..", "..", clean)), m.Old) {
			t.Errorf("mutation %q: anchor not found in %s", m.Name, m.File)
		}
	}
}
