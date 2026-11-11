//go:build darwin || linux

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
)

// runCellctlSuite runs one tools/cellctl/tests suite against a binary built from this tree, under
// the minimal environment TestDownShellSuite uses, from a throwaway working directory, and
// returns its output once the suite printed its own OK line. tools names the executables the
// suite takes from the system PATH; a missing one skips the test.
func runCellctlSuite(t *testing.T, suite string, tools []string, args ...string) string {
	t.Helper()
	path := []string{}
	for _, tool := range tools {
		found, err := exec.LookPath(tool)
		if err != nil {
			t.Skipf("%s is not on PATH", tool)
		}
		path = append(path, filepath.Dir(found))
	}
	path = append(path, "/usr/bin", "/bin")
	script, err := filepath.Abs(filepath.Join("..", "..", "..", "cellctl", "tests", suite))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", append([]string{script}, args...)...)
	tmp, err := os.MkdirTemp("", "cellctl-suite-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(tmp) })
	cmd.Dir = tmp
	cmd.Env = []string{
		"PATH=" + strings.Join(path, string(os.PathListSeparator)),
		"CELLCTL=" + cellctlBinary(t), "TMPDIR=" + tmp, "HOME=" + tmp, "LC_ALL=C",
	}
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "\n"+suite+": OK\n") {
		t.Fatalf("%s %s: %v\n%s", suite, strings.Join(args, " "), err, out)
	}
	return string(out)
}

// TestHarnessShellSuite runs tools/cellctl/tests/harness.test.sh against the Go binary. No
// workflow named the suite, so its `check` rows kept the `codex config get` stub and label that
// f53fed97f retired, and its `set` refusal rows kept the two-harness wording, all red against
// this binary unnoticed (#6175).
//
// regression: #6175
func TestHarnessShellSuite(t *testing.T) {
	out := runCellctlSuite(t, "harness.test.sh", []string{"bash", "git"})
	if n := strings.Count(out, "\n  ok    "); n < 40 {
		t.Fatalf("harness.test.sh ran only %d assertions:\n%s", n, out)
	}
}

// scrubbedCases maps every case of scrubbed-cell.test.sh to whether it needs the REAL tmux
// binary: a scrubbed cell's `check` has a tmux row, and `desk` (dry run included), `lock`,
// `status` and `down` drive a private-socket tmux server, which the suite never stubs because
// proving that isolation needs a real session. The other cases run anywhere.
var scrubbedCases = map[string]bool{
	"new": false, "check-pass": true, "check-pem": false, "check-roster": false,
	"check-home-mode": false, "env-scrub": true, "plan-grammar": true, "smoke-ready": false,
	"smoke-not-ready": false, "lock": true, "status": true, "down": true, "legacy-kinds": false,
	"deskwt-canary": true,
}

var scrubbedCaseArmRE = regexp.MustCompile(`(?m)^\s+([a-z][a-z-]*)\) case_[a-z_]+ ;;$`)

// TestScrubbedCellShellSuite runs each case of tools/cellctl/tests/scrubbed-cell.test.sh against
// the Go binary. Cases that need a real tmux skip, by name, where tmux is not installed; every
// other case runs on every host. Its check-pass case kept the retired `codex config get` stub
// and its plan-grammar KEY-set row read ASSAY_TELEMETRY from the launching shell (#6175).
//
// regression: #6175
func TestScrubbedCellShellSuite(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "..", "cellctl", "tests", "scrubbed-cell.test.sh"))
	if err != nil {
		t.Fatal(err)
	}
	var arms, known []string
	for _, m := range scrubbedCaseArmRE.FindAllStringSubmatch(string(src), -1) {
		arms = append(arms, m[1])
	}
	for name := range scrubbedCases {
		known = append(known, name)
	}
	sort.Strings(arms)
	sort.Strings(known)
	if strings.Join(arms, " ") != strings.Join(known, " ") {
		t.Fatalf("scrubbed-cell.test.sh cases %v, scrubbedCases %v: classify every case", arms, known)
	}
	for _, name := range known {
		t.Run(name, func(t *testing.T) {
			tools := []string{"bash", "git"}
			if scrubbedCases[name] {
				if _, err := exec.LookPath("tmux"); err != nil {
					t.Skipf("case %s needs a real tmux binary, and tmux is not on PATH", name)
				}
				tools = append(tools, "tmux")
			}
			runCellctlSuite(t, "scrubbed-cell.test.sh", tools, "--case", name)
		})
	}
}
