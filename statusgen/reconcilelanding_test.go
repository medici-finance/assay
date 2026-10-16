package main

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The landing path for `reconcile --apply` on a driven board (#2694). The
// scheduled reconcile job commits a tick only when every changed path is a
// stream README (its stray-path check), and deskreconcile's changedReadmes
// refuses the same way. Re-rendering a drive-plan page's snapshot region is
// therefore opt-in (--regenerate): a default --apply on a board whose active
// drive has a plan page must still land its undriven rows and hold the driven
// ones, exactly as before.

// jobStrayCheck returns the reconcile job's own stray-path check, read from
// the staged workflow's compute step: its README pattern and the block from
// the status read through the refusal. It is run as the job runs it.
func jobStrayCheck(t *testing.T) string {
	t.Helper()
	lines := strings.Split(reconcileStep(t, reconcileJobStepName).Run, "\n")
	var re string
	var block []string
	for i, l := range lines {
		s := strings.TrimSpace(l)
		if re == "" && strings.HasPrefix(s, "readme_re=") {
			re = s
		}
		if block == nil && strings.HasPrefix(s, `porcelain="$(git status`) {
			for _, b := range lines[i:] {
				block = append(block, b)
				if strings.TrimSpace(b) == "fi" {
					break
				}
			}
		}
	}
	if re == "" || len(block) < 3 || strings.TrimSpace(block[len(block)-1]) != "fi" {
		t.Fatalf("%s: the compute step's stray-path check moved; re-anchor jobStrayCheck", reconcileWorkflowPath())
	}
	return "set -euo pipefail\n" + re + "\n" + strings.Join(block, "\n") + "\necho LANDS\n"
}

// drivenPulls is one merged PR per witnessed row of driveManyRowBoard, each
// carrying that brief's `Brief:` trailer (the job's trailer-only witness).
func drivenPulls(cells []BriefCell) []map[string]any {
	var pulls []map[string]any
	for i, c := range cells {
		pulls = append(pulls, mergedPull(10+i, "feat/"+strings.ReplaceAll(c.ID, "/", "-"), "Implements it.\n\nBrief: "+c.ID+"\n"))
	}
	return pulls
}

// landingRun runs `reconcile --apply` (plus extra) on a committed driven
// board, then the job's stray-path check on the result.
func landingRun(t *testing.T, extra ...string) (res reconcileResult, root, planBefore, checkOut string, checkRC int) {
	t.Helper()
	requireJobTools(t)
	pinNow(t, driveTestNow)
	t.Setenv("GITHUB_TOKEN", "test-token")
	root, cells := driveManyRowBoard(t)
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.invalid"}, args...)...)
		cmd.Dir = root
		cmd.Env = append(scrubbedEnv(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return string(out)
	}
	git("init", "-q", "-b", "main")
	git("add", "-A")
	git("commit", "-q", "-m", "seed")
	planBefore = readPlan(t, root, "d1")
	stubPullsServer(t, http.StatusOK, drivenPulls(cells))

	args := append([]string{"--apply", "--root", root, "--repo", "o/r", "--json"}, extra...)
	code, stdout := runReconcileCaptured(t, args...)
	if code != reconcileOK {
		t.Fatalf("reconcile %v: exit %d\n%s", args, code, stdout)
	}
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("decoding --json: %v\n%s", err, stdout)
	}
	cmd := exec.Command("bash", scriptFile(t, jobStrayCheck(t)))
	cmd.Dir = root
	cmd.Env = append(scrubbedEnv(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	out, err := cmd.CombinedOutput()
	if ee, ok := err.(*exec.ExitError); ok {
		checkRC = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return res, root, planBefore, string(out), checkRC
}

// regression: #2694
func TestApplyReconcileLandsDriven(t *testing.T) {
	res, root, planBefore, out, rc := landingRun(t)
	if rc != 0 || !strings.Contains(out, "LANDS") {
		t.Fatalf("a default --apply tick on a driven board must pass the job's stray-path check, got rc %d:\n%s", rc, out)
	}
	if got := readPlan(t, root, "d1"); got != planBefore {
		t.Fatalf("a default --apply wrote the drive plan page:\n%s", got)
	}
	isDriven := func(id string) bool { return strings.HasPrefix(id, "mr00/") || strings.HasPrefix(id, "mr02/") }
	if len(res.Applied) != 4 {
		t.Fatalf("want the 4 undriven rows landed, got %+v", res.Applied)
	}
	for _, a := range res.Applied {
		if isDriven(a.ID) || len(a.Generated) != 0 {
			t.Fatalf("a default --apply wrote driven row %s (generated %+v)", a.ID, a.Generated)
		}
	}
	if len(res.Held) != 4 {
		t.Fatalf("want the 4 driven rows held, got %+v", res.Held)
	}
	for _, h := range res.Held {
		if !isDriven(h.ID) || !strings.Contains(strings.Join(h.Problems, "\n"), "drive-region-drift") {
			t.Fatalf("want only driven rows held, on drive-region-drift, got %+v", h)
		}
	}
}

// TestApplyReconcileRegenStrays is the opt-in's side of the same check, and
// the positive control that the check above can fail: --regenerate writes the
// plan page, which the job's stray-path check refuses by name. That is why the
// opt-in stays off on the landing callers.
func TestApplyReconcileRegenStrays(t *testing.T) {
	res, root, planBefore, out, rc := landingRun(t, "--regenerate")
	if len(res.Held) != 0 || len(res.Applied) != 8 {
		t.Fatalf("with --regenerate want all 8 rows written, got applied=%d held=%+v", len(res.Applied), res.Held)
	}
	if readPlan(t, root, "d1") == planBefore {
		t.Fatal("--apply --regenerate must write the re-rendered region")
	}
	want := filepath.ToSlash(filepath.Join("docs", "roadmap", "drives", "d1.md"))
	if rc == 0 || !strings.Contains(out, want) {
		t.Fatalf("the job's stray-path check must refuse the plan page %s, got rc %d:\n%s", want, rc, out)
	}
}

// TestRegenerateNeedsApply: the opt-in only changes what the hold admits, so on
// a run with neither --apply nor --backfill it is a usage error, never ignored.
func TestRegenerateNeedsApply(t *testing.T) {
	code, _ := runReconcileCaptured(t, "--regenerate", "--offline", "--root", t.TempDir())
	if code != reconcileUsageErr {
		t.Fatalf("--regenerate alone: exit %d, want %d", code, reconcileUsageErr)
	}
}
