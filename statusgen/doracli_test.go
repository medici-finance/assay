package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestDoraRunEmits pins that the restored --dora entrypoint runs to a clean exit
// and emits the grouped-DORA report (the v0.14.0 back-compat restore). It calls
// runDora directly — the same in-process discipline autonomy_test.go and
// methmetrics_test.go use for their emitters.
func TestDoraRunEmits(t *testing.T) {
	// A real streams root with no in-window done-transitions still emits a report
	// (every metric degrades to an explicit unknown/needs marker) and exits 0 — a
	// could-not-check, never an error.
	const root = "testdata/briefschema"
	if code := runDora(root, "2026-07-01", "stream", false); code != 0 {
		t.Fatalf("runDora(stream) exited %d, want 0", code)
	}
	if code := runDora(root, "2026-07-01", "goal", true); code != 0 {
		t.Fatalf("runDora(goal, json) exited %d, want 0", code)
	}
	// A bogus grouping dimension is rejected, not silently coerced. This is
	// validated before any streams are loaded, so the root is irrelevant.
	if code := runDora(root, "", "bogus", false); code == 0 {
		t.Error("runDora with --by bogus exited 0, want non-zero — an invalid grouping must be refused")
	}
}

// TestBackCompatFlagsParse pins the v0.14.0 regression fix at the CLI boundary: the
// pinned daily-harvest/v0.1.0 collector shells out to `statusgen -dora` and
// `statusgen -trend`, and v0.14.0 removed both, so daily-harvest died with
// "flag provided but not defined". This exercises the ACTUAL flag surface of a built
// binary: both aliases must PARSE (never that error), `-trend` must behave
// identically to its primary `-verif-backlog`, and the primary flags must still work.
func TestBackCompatFlagsParse(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain unavailable")
	}
	bin := filepath.Join(t.TempDir(), "statusgen")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	// A root carrying a multi-week backlog history so --trend / --verif-backlog
	// produce a real curve and exit 0.
	root := t.TempDir()
	histPath := filepath.Join(root, filepath.FromSlash(historyRelPath))
	if err := os.MkdirAll(filepath.Dir(histPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := appendHistory(histPath, backlogFixture()); err != nil {
		t.Fatal(err)
	}

	// HOME is pointed at an empty dir so no adopter roster.env colours the run —
	// the metric emitters do not consult the roster, and this keeps the run
	// deterministic regardless of the developer's environment.
	env := append(os.Environ(), "HOME="+t.TempDir())

	run := func(args ...string) (string, string, int) {
		t.Helper()
		cmd := exec.Command(bin, args...)
		cmd.Env = env
		var stdout, stderr strings.Builder
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		code := 0
		if err := cmd.Run(); err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				code = ee.ExitCode()
			} else {
				t.Fatalf("running %v: %v", args, err)
			}
		}
		return stdout.String(), stderr.String(), code
	}

	notDefined := "flag provided but not defined"

	t.Run("-dora parses and emits DORA (not flag-not-defined)", func(t *testing.T) {
		stdout, stderr, code := run("-root", root, "-dora", "-since", "2026-07-01")
		if strings.Contains(stderr, notDefined) {
			t.Fatalf("`statusgen -dora` still reports %q — the back-compat flag is not defined:\n%s", notDefined, stderr)
		}
		if code != 0 {
			t.Errorf("`statusgen -dora` exited %d, want 0\nstderr:\n%s", code, stderr)
		}
		if !strings.Contains(stdout, "DORA metrics by") {
			t.Errorf("`statusgen -dora` did not emit the DORA report; stdout:\n%s", stdout)
		}
	})

	t.Run("-trend parses (not flag-not-defined) and equals -verif-backlog", func(t *testing.T) {
		trendOut, trendErr, trendCode := run("-root", root, "-trend", "-since", "2026-07-01")
		if strings.Contains(trendErr, notDefined) {
			t.Fatalf("`statusgen -trend` still reports %q — the back-compat alias is not defined:\n%s", notDefined, trendErr)
		}
		backOut, _, backCode := run("-root", root, "-verif-backlog", "-since", "2026-07-01")
		if trendCode != 0 || backCode != 0 {
			t.Fatalf("-trend exited %d, -verif-backlog exited %d, want both 0", trendCode, backCode)
		}
		if trendOut != backOut {
			t.Errorf("-trend is documented as an alias of -verif-backlog but their stdout differs:\n--trend:\n%s\n--verif-backlog:\n%s", trendOut, backOut)
		}
	})
}
