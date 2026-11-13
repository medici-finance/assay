package main

// Wiring tests for the human-gate sign-off digest (signoffdigest.go) in the
// board workflow's scheduled reconcile job. The digest is a surfacing-only
// roll-up: the job writes it to the tick's job summary and nowhere else. These
// tests pin that on the live workflow and its staged twin: the step runs
// --signoff-digest on main's tree before the compute step, holds no token,
// writes to the job summary, and a could-not-check digest warns without
// failing the tick.

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const signoffDigestStepName = "Sign-off digest to the job summary"

var signoffDigestWorkflows = []string{
	"../.github/workflows/assay-statusgen.yml",
	"../ci/staged-workflows/assay-statusgen.yml",
}

func signoffDigestStep(t *testing.T, path string) (wfStep, int, int) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	steps := parseWorkflow(t, raw, path).Jobs["reconcile"].Steps
	at, compute := -1, -1
	for i, st := range steps {
		switch st.Name {
		case signoffDigestStepName:
			at = i
		case reconcileJobStepName:
			compute = i
		}
	}
	if at < 0 {
		t.Fatalf("%s: reconcile job has no %q step", path, signoffDigestStepName)
	}
	return steps[at], at, compute
}

func TestReconcileSignoffDigestStep(t *testing.T) {
	for _, path := range signoffDigestWorkflows {
		st, at, compute := signoffDigestStep(t, path)
		if compute < 0 || at > compute {
			t.Errorf("%s: the digest step (%d) must run before the compute step (%d), on main's tree as cloned", path, at, compute)
		}
		if !strings.HasPrefix(st.WorkingDirectory, runnerTempExpr+"/board-src") {
			t.Errorf("%s: digest step working-directory %q, want the job-local clone", path, st.WorkingDirectory)
		}
		for k, v := range st.Env {
			if strings.Contains(v, "token") || k == "GH_TOKEN" || k == "GITHUB_TOKEN" {
				t.Errorf("%s: digest step env %s=%s: it must hold no token", path, k, v)
			}
		}
		for _, need := range []string{`--root . --signoff-digest > "$digest" || rc=$?`, `>> "${GITHUB_STEP_SUMMARY:-/dev/null}"`, "::warning::"} {
			if !strings.Contains(st.Run, need) {
				t.Errorf("%s: digest step lacks %q", path, need)
			}
		}
		for _, banned := range []string{"gh ", "issue", "curl"} {
			if strings.Contains(st.Run, banned) {
				t.Errorf("%s: digest step contains %q: it surfaces to the job summary only", path, banned)
			}
		}
	}
}

// TestReconcileSignoffDigestStepRuns executes the step's run text with a stub
// statusgen: a digest body reaches the job summary, and a could-not-check exit
// warns while the step itself exits 0.
func TestReconcileSignoffDigestStepRuns(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the step runs on a Linux runner")
	}
	requireTool(t, "bash")
	for _, path := range signoffDigestWorkflows {
		st, _, _ := signoffDigestStep(t, path)
		for _, rc := range []string{"0", "1"} {
			dir := t.TempDir()
			stub := "#!/usr/bin/env bash\necho '<!-- signoff-digest -->'\necho '## Human sign-off queue — stub'\nexit " + rc + "\n"
			if err := os.WriteFile(filepath.Join(dir, "statusgen"), []byte(stub), 0o755); err != nil {
				t.Fatal(err)
			}
			summary := filepath.Join(dir, "summary.md")
			cmd := exec.Command("bash", "-c", st.Run)
			cmd.Dir = dir
			cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "RUNNER_TEMP=" + dir, "GITHUB_STEP_SUMMARY=" + summary}
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Errorf("%s stub exit %s: step failed (%v), want exit 0:\n%s", path, rc, err, out)
			}
			sum, _ := os.ReadFile(summary)
			if !strings.Contains(string(sum), "## Human sign-off queue — stub") {
				t.Errorf("%s stub exit %s: the digest did not reach the job summary: %q", path, rc, sum)
			}
			if warned := strings.Contains(string(out), "::warning::statusgen --signoff-digest exited 1"); warned != (rc == "1") {
				t.Errorf("%s stub exit %s: warning printed=%v:\n%s", path, rc, warned, out)
			}
		}
	}
}
