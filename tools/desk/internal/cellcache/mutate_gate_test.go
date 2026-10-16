//go:build !windows

package cellcache

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
	return fxModuleWith(t, nil, test, control, muts...)
}

// fxModuleWith is fxModule plus extra files, keyed by path relative to the
// module directory ("../dep/go.mod" is a sibling of it). An extra file replaces
// a default of the same name. A value starting "dir:" makes a directory and a
// value starting "link:" makes a symlink to the rest of the value, which may be
// "@top/outside.go" for the fixture's real outside file.
func fxModuleWith(t *testing.T, extra map[string]string, test string, control fxMutation, muts ...fxMutation) (string, string) {
	t.Helper()
	top := t.TempDir()
	mod := filepath.Join(top, "mod")
	if err := os.Mkdir(mod, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod":             "module example.com/fx\n\ngo 1.21\n",
		"fx.go":              fxSource,
		"fx_test.go":         fxTest,
		"../outside.go":      "package outside // return 5\n",
		"../outside.go.orig": "package outside // return 5\n",
	}
	for name, body := range extra {
		files[name] = body
	}
	for name, body := range files {
		path := filepath.Join(mod, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		var err error
		switch {
		case strings.HasPrefix(body, "dir:"):
			err = os.MkdirAll(path, 0o755)
		case strings.HasPrefix(body, "link:"):
			err = os.Symlink(strings.Replace(strings.TrimPrefix(body, "link:"), "@top", top, 1), path)
		default:
			err = os.WriteFile(path, []byte(body), 0o644)
		}
		if err != nil {
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
		// A silent skip would let the proof of the gate vanish from a runner
		// without python3 while the job stays green.
		t.Fatalf("could-not-check: python3 is not on PATH, so mutate.py cannot be exercised: %v", err)
	}
	args := append([]string{"mutate.py", "--spec", spec, "--module", mod}, extra...)
	cmd := exec.Command(py, args...)
	cmd.Dir = "."
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=", "TMPDIR="+t.TempDir())
	return cmd
}

func runGate(t *testing.T, mod, spec string) (string, int) {
	t.Helper()
	return runGateWith(t, mod, spec, nil)
}

func runGateWith(t *testing.T, mod, spec string, env []string, args ...string) (string, int) {
	t.Helper()
	cmd := gateCommand(t, mod, spec, args...)
	cmd.Env = append(cmd.Env, env...)
	out, err := cmd.CombinedOutput()
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

// hangSuite plants a SENTINEL as the control's catch and, for a HANG mutation,
// records its own pids in $PIDS, touches $MARK and sleeps. A pid file lets a
// test see whether the suite survived, and clean it up.
const hangSuite = `sh -c 'if grep -q HANG fx.go; then sleep 60 & echo "$$ $!" > "$PIDS"; touch "$MARK"; wait; fi; if grep -q SENTINEL fx.go; then echo "--- FAIL: control"; exit 1; fi'`

var (
	sentinelControl = fxMutation{Name: "control", File: "fx.go", Old: "package fx", New: "package fx // SENTINEL"}
	hangMutation    = fxMutation{Name: "hang", File: "fx.go", Old: "package fx", New: "package fx // HANG"}
)

func recordedPids(path string) []int {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var pids []int
	for _, f := range strings.Fields(string(raw)) {
		if n, err := strconv.Atoi(f); err == nil && n > 1 {
			pids = append(pids, n)
		}
	}
	return pids
}

func killRecorded(path string) {
	for _, pid := range recordedPids(path) {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
}

func waitMarker(t *testing.T, marker string, out *bytes.Buffer) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the hanging mutant never started:\n%s", out.String())
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func alive(pid int) bool { return syscall.Kill(pid, 0) == nil }

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
	pids := filepath.Join(t.TempDir(), "pids")
	// The suite runs in its own session; the kill below cannot reach it.
	t.Cleanup(func() { killRecorded(pids) })
	mod, spec := fxModule(t, hangSuite, sentinelControl, hangMutation)
	before := readFile(t, filepath.Join(mod, "fx.go"))

	cmd := gateCommand(t, mod, spec)
	cmd.Env = append(cmd.Env, "MARK="+marker, "PIDS="+pids)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); _ = cmd.Wait() }()

	waitMarker(t, marker, &buf)
	// SIGKILL the gate itself, mid-mutation: no handler or finally block runs.
	if err := syscall.Kill(cmd.Process.Pid, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(mod, "fx.go")); got != before {
		t.Fatalf("a killed run left the source tree mutated:\n%s", got)
	}
}

// regression: #3148
func TestMutateGateSignalStopsTheSuiteAndRemovesTheCopy(t *testing.T) {
	for _, tc := range []struct {
		name string
		sig  syscall.Signal
		exit int
	}{{"SIGTERM", syscall.SIGTERM, 143}, {"SIGINT", syscall.SIGINT, 130}} {
		t.Run(tc.name, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "planted")
			pids := filepath.Join(t.TempDir(), "pids")
			tmp := t.TempDir()
			t.Cleanup(func() { killRecorded(pids) })
			mod, spec := fxModule(t, hangSuite, sentinelControl, hangMutation)
			before := readFile(t, filepath.Join(mod, "fx.go"))

			cmd := gateCommand(t, mod, spec)
			cmd.Env = append(cmd.Env, "MARK="+marker, "PIDS="+pids, "TMPDIR="+tmp)
			var buf bytes.Buffer
			cmd.Stdout, cmd.Stderr = &buf, &buf
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			defer func() { _ = cmd.Process.Kill() }()

			waitMarker(t, marker, &buf)
			if err := cmd.Process.Signal(tc.sig); err != nil {
				t.Fatal(err)
			}
			var err error
			select {
			case err = <-done:
			case <-time.After(30 * time.Second):
				t.Fatalf("the gate ignored %s:\n%s", tc.name, buf.String())
			}
			var ee *exec.ExitError
			if !errors.As(err, &ee) || ee.ExitCode() != tc.exit {
				t.Fatalf("exit %v, want %d:\n%s", err, tc.exit, buf.String())
			}
			pidList := recordedPids(pids)
			if len(pidList) != 2 {
				t.Fatalf("suite pids %v, want the shell and its sleep", pidList)
			}
			for _, pid := range pidList {
				for i := 0; alive(pid) && i < 100; i++ {
					time.Sleep(50 * time.Millisecond)
				}
				if alive(pid) {
					t.Errorf("suite process %d survived %s: it is orphaned against a deleted copy", pid, tc.name)
				}
			}
			if left, _ := filepath.Glob(filepath.Join(tmp, "cellcache-mutate-*")); len(left) != 0 {
				t.Errorf("temporary copy left behind: %v", left)
			}
			if got := readFile(t, filepath.Join(mod, "fx.go")); got != before {
				t.Errorf("source tree changed:\n%s", got)
			}
		})
	}
}

// regression: #3148
func TestMutateGateRefusesFilesThatAreNotPlainFilesInTheModule(t *testing.T) {
	mod, spec := fxModuleWith(t, map[string]string{
		"link.txt": "link:@top/outside.go", // a symlink out of the module, onto a real file
		"pkgdir":   "dir:",
	}, goSuite, fxControl(),
		fxMutation{Name: "symlink out", File: "link.txt", Old: "return 5", New: "return 6"},
		fxMutation{Name: "a directory", File: "pkgdir", Old: "x", New: "y"},
		fxMutation{Name: "the module root", File: ".", Old: "x", New: "y"},
		fxMutation{Name: "nul byte", File: "nul\x00.go", Old: "package fx", New: "package fy"},
		fxMutation{Name: "inner dot dot back in", File: "pkgdir/../fx.go", Old: "return 5", New: "return 0"},
	)
	out, code := runGate(t, mod, spec)
	if code != 1 {
		t.Fatalf("exit %d, want 1:\n%s", code, out)
	}
	mustContain(t, out,
		"COULD-NOT-MUTATE symlink out: file 'link.txt' escapes the module",
		"COULD-NOT-MUTATE a directory: file 'pkgdir' is not a file in the module",
		"COULD-NOT-MUTATE the module root: file '.' escapes the module",
		"COULD-NOT-MUTATE nul byte: file 'nul\\x00.go' is not a relative path",
		"CAUGHT inner dot dot back in",
		"Totals: 1 caught, 0 NOT CAUGHT, 4 could-not-mutate.")
	if got := readFile(t, filepath.Join(mod, "../outside.go")); got != readFile(t, filepath.Join(mod, "../outside.go.orig")) {
		t.Errorf("a mutation reached the file the symlink points at: %q", got)
	}
}

// regression: #3148
func TestMutateGateRefusesNoOpAndEmptyEdits(t *testing.T) {
	mod, spec := fxModule(t, goSuite, fxControl(),
		fxMutation{Name: "same text", File: "fx.go", Old: "return 5", New: "return 5"},
		fxMutation{Name: "empty old", File: "fx.go", Old: "", New: "return 5"},
	)
	out, code := runGate(t, mod, spec)
	if code != 1 {
		t.Fatalf("exit %d, want 1:\n%s", code, out)
	}
	mustContain(t, out,
		"COULD-NOT-MUTATE same text: `old` equals `new`",
		"COULD-NOT-MUTATE empty old: `old` is empty",
		"Totals: 0 caught, 0 NOT CAUGHT, 2 could-not-mutate.")
}

// regression: #3148
func TestMutateGateSuiteTimeoutIsCouldNotMutate(t *testing.T) {
	pids := filepath.Join(t.TempDir(), "pids")
	marker := filepath.Join(t.TempDir(), "planted")
	t.Cleanup(func() { killRecorded(pids) })
	mod, spec := fxModule(t, hangSuite, sentinelControl, hangMutation)
	out, code := runGateWith(t, mod, spec, []string{"MARK=" + marker, "PIDS=" + pids, "CELLCACHE_MUTATE_SUITE_TIMEOUT=3"})
	if code != 1 {
		t.Fatalf("exit %d, want 1:\n%s", code, out)
	}
	mustContain(t, out,
		"COULD-NOT-MUTATE hang: suite timed out",
		"Totals: 0 caught, 0 NOT CAUGHT, 1 could-not-mutate.")
	// The timeout kills the suite's whole group, not just its shell.
	for _, pid := range recordedPids(pids) {
		for i := 0; alive(pid) && i < 100; i++ {
			time.Sleep(50 * time.Millisecond)
		}
		if alive(pid) {
			t.Errorf("suite process %d survived the timeout", pid)
		}
	}
}

// regression: #3148
func TestMutateGateBuildFailureInTheBaselineBreaksTheHarness(t *testing.T) {
	// Exit 0 with a build failure in the output: only the output check sees it.
	mod, spec := fxModule(t, `sh -c 'echo "FAIL example.com/fx [build failed]"'`, fxControl(), fxCaught("flat rate"))
	out, code := runGate(t, mod, spec)
	if code != 2 {
		t.Fatalf("exit %d, want 2:\n%s", code, out)
	}
	mustContain(t, out, "HARNESS BROKEN: baseline suite is not green")
}

// regression: #3148
func TestMutateGateCopiesRelativeReplaceTargets(t *testing.T) {
	mod, spec := fxModuleWith(t, map[string]string{
		"go.mod":        "module example.com/fx\n\ngo 1.21\n\nrequire example.com/dep v0.0.0\n\nreplace example.com/dep => ../dep\n",
		"fx.go":         "package fx\n\nimport \"example.com/dep\"\n\nfunc Cost(total int) int {\n\tif total >= 50 {\n\t\treturn 0\n\t}\n\treturn dep.Rate()\n}\n",
		"../dep/go.mod": "module example.com/dep\n\ngo 1.21\n",
		"../dep/dep.go": "package dep\n\nfunc Rate() int { return 5 }\n",
	}, goSuite, fxMutation{Name: "control", File: "fx.go", Old: "return dep.Rate()", New: "return dep.Rate() + 1"},
		fxMutation{Name: "flat rate", File: "fx.go", Old: "return dep.Rate()", New: "return dep.Rate() + 2"})
	out, code := runGate(t, mod, spec)
	if code != 0 {
		t.Fatalf("a module with a relative replace target must run: exit %d:\n%s", code, out)
	}
	mustContain(t, out, "CAUGHT flat rate", "Totals: 1 caught, 0 NOT CAUGHT, 0 could-not-mutate.")
}

// regression: #3148
func TestMutateGateShardsPartitionTheSpec(t *testing.T) {
	mod, spec := fxModule(t, goSuite, fxControl(), fxCaught("m0"), fxCaught("m1"), fxCaught("m2"))
	for shard, want := range map[string][]string{"0/2": {"m0", "m2"}, "1/2": {"m1"}} {
		out, code := runGateWith(t, mod, spec, nil, "--shard", shard)
		if code != 0 {
			t.Fatalf("shard %s: exit %d:\n%s", shard, code, out)
		}
		for _, m := range []string{"m0", "m1", "m2"} {
			has := strings.Contains(out, "CAUGHT "+m+"\n")
			if wanted := contains(want, m); has != wanted {
				t.Errorf("shard %s: %s run=%v, want %v:\n%s", shard, m, has, wanted, out)
			}
		}
	}
	for _, bad := range []string{"2/2", "-1/3", "a/b", "1"} {
		if out, code := runGateWith(t, mod, spec, nil, "--shard", bad); code != 2 {
			t.Errorf("--shard %s: exit %d, want 2:\n%s", bad, code, out)
		}
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
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
