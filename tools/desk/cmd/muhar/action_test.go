package main

// The reusable CI action (actions/muhar, #2642) reads muhar's stdout: the Totals line
// and the exit code. These tests keep the action and muhar from drifting apart. They
// live beside muhar, not beside the action, so a change to Report.Summary or to the
// flags reddens the same `go test` that changed it.

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// actionScript is the action's logic script in this checkout; it skips where no
// POSIX shell runs it (the action itself runs under bash on every runner).
func actionScript(t *testing.T, name string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell script; the action runs it under bash")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is not on PATH")
	}
	return filepath.Join(repoRoot(t), "actions", "muhar", name)
}

// runBash runs `bash script args...` under a finite deadline and returns its combined
// output and exit code (-1 when it did not exit normally).
func runBash(t *testing.T, env []string, script string, args ...string) (string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", append([]string{script}, args...)...)
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return string(out), 0
	case errors.As(err, &exit) && ctx.Err() == nil:
		return string(out), exit.ExitCode()
	default:
		t.Fatalf("bash %s: %v\n%s", script, err, out)
		return "", -1
	}
}

// regression: #2642
// TestActionShellSuite makes the action's offline suite part of every `go test`.
func TestActionShellSuite(t *testing.T) {
	suite := actionScript(t, "muhar-action.test.sh")
	out, code := runBash(t, []string{"TMPDIR=" + t.TempDir()}, suite)
	if code != 0 || !strings.Contains(out, "\nmuhar-action.test.sh: OK\n") {
		t.Fatalf("muhar-action.test.sh exited %d:\n%s", code, out)
	}
	if n := strings.Count(out, "\n  ok    "); n < 50 {
		t.Fatalf("muhar-action.test.sh ran only %d assertions:\n%s", n, out)
	}
}

// regression: #2642
// TestActionParsesSummary feeds the action's parser the text Report.Summary really
// prints, so a change to the Totals wording cannot leave the action reading nothing.
func TestActionParsesSummary(t *testing.T) {
	script := actionScript(t, "muhar-action.sh")
	healthy := Report{BaselineGreen: true, ControlResult: Caught, Results: []MutationResult{
		{Mutation: Mutation{Name: "a"}, Result: Caught},
		{Mutation: Mutation{Name: "b"}, Result: Caught},
		{Mutation: Mutation{Name: "Totals: 9 caught, 0 NOT CAUGHT, 0 could-not-mutate."}, Result: Caught},
		{Mutation: Mutation{Name: "c"}, Result: NotCaught},
		{Mutation: Mutation{Name: "d"}, Result: CouldNotMutate, Err: errors.New("Old text not found")},
	}}
	broken := Report{Broken: true, BrokenReason: "baseline suite is RED"}
	dir := t.TempDir()
	for _, tc := range []struct {
		name string
		rep  Report
		want string
		code int
	}{
		{"healthy", healthy, "caught=3 not-caught=1 could-not-mutate=1", 0},
		{"broken", broken, "", 3},
	} {
		path := filepath.Join(dir, tc.name+".out")
		if err := os.WriteFile(path, []byte(tc.rep.Summary()), 0o644); err != nil {
			t.Fatal(err)
		}
		out, code := runBash(t, nil, script, "parse", path)
		if code != tc.code || strings.TrimSpace(out) != tc.want {
			t.Errorf("%s: parse exited %d with %q, want %d with %q\nreport:\n%s", tc.name, code, strings.TrimSpace(out), tc.code, tc.want, tc.rep.Summary())
		}
	}
}

// regression: #2642
// TestActionRunsRealMuhar drives the action's run step against a muhar binary built
// from this tree: the flags it passes are ones muhar accepts, a healthy run publishes
// the real counts, a shard reaches muhar, and a red baseline is exit 2.
func TestActionRunsRealMuhar(t *testing.T) {
	script := actionScript(t, "muhar-action.sh")
	needGo(t)
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "muhar")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("go build muhar: %v\n%s", err, out)
	}
	work := filepath.Join(tmp, "work")
	writeFiles(t, work, map[string]string{
		"guard.txt": "limit = 10\nfloor = 3\n# note\n",
		"healthy.json": `{"test": "grep -q 'limit = 10' guard.txt && grep -q 'floor = 3' guard.txt",
 "control": {"name": "limit", "file": "guard.txt", "old": "limit = 10", "new": "limit = 11"},
 "mutations": [
  {"name": "floor", "file": "guard.txt", "old": "floor = 3", "new": "floor = 4"},
  {"name": "note", "file": "guard.txt", "old": "# note", "new": "# changed"},
  {"name": "gone", "file": "guard.txt", "old": "absent text", "new": "x"}]}`,
		"broken.json": `{"test": "false",
 "control": {"name": "limit", "file": "guard.txt", "old": "limit = 10", "new": "limit = 11"},
 "mutations": []}`,
	})
	run := func(name string, env ...string) (string, int, string) {
		outputs := filepath.Join(tmp, name+".outputs")
		base := []string{"MUHAR_BIN=" + bin, "MUHAR_WORKDIR=" + work, "MUHAR_JOBS=1", "MUHAR_SHARD=",
			"MUHAR_FAIL_ON_NOT_CAUGHT=false", "MUHAR_FAIL_ON_COULD_NOT_MUTATE=false",
			"GITHUB_OUTPUT=" + outputs, "GITHUB_STEP_SUMMARY=" + filepath.Join(tmp, name+".summary"),
			"RUNNER_TEMP=" + tmp}
		out, code := runBash(t, append(base, env...), script, "run")
		got, _ := os.ReadFile(outputs)
		return out, code, string(got)
	}

	out, code, outputs := run("healthy", "MUHAR_SPECS=healthy.json")
	if code != 0 || !strings.Contains(outputs, "caught=1\nnot-caught=1\ncould-not-mutate=1\n") {
		t.Fatalf("healthy run: exit %d, outputs %q\n%s", code, outputs, out)
	}
	out, code, outputs = run("shard", "MUHAR_SPECS=healthy.json", "MUHAR_SHARD=1/2")
	if code != 0 || !strings.Contains(outputs, "caught=0\nnot-caught=1\ncould-not-mutate=0\n") || !strings.Contains(out, "shard 1/2") {
		t.Fatalf("shard 1/2: exit %d, outputs %q\n%s", code, outputs, out)
	}
	out, code, outputs = run("broken", "MUHAR_SPECS=broken.json")
	if code != 2 || !strings.Contains(out, "harness broken — not clean") || strings.Contains(outputs, "caught=") {
		t.Fatalf("broken run: exit %d, outputs %q\n%s", code, outputs, out)
	}
}
