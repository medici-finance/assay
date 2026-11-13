package main

// autoflipwiring_test.go pins that the model-autoflip job's own doc comment in
// the shipped CI workflow accurately describes the two-tier could-not-check
// exit policy reportAutoFlipModel implements ("Fix B" in autoflip.go). Prose
// can drift from code silently forever — this is the one test that reads the
// two together and fails the moment they disagree, the same wiring-test shape
// as corroboratewiring_test.go for `--corroborate`.
//
// It exists because the comment used to read "a candidate the tool could not
// check fails this job loudly (exit 1)" — true only for the MISCONFIGURATION
// kind of could-not-check, false for the STRUCTURALLY
// unresolvable kind (exit 0, a non-fatal NOTICE) that
// TestReportAutoFlipStructuralNonFatal pins directly against the binary. The
// CODE was already right — TestReportAutoFlipStructuralNonFatal,
// TestReportAutoFlipMisconfigFatal and TestReportAutoFlipMisconfigDominates
// (see below) all predate this file and pin it — the COMMENT overclaimed.

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestModelAutoflipWorkflowCommentMatchesExitPolicy reads the model-autoflip
// job's header comment out of assay-statusgen.yml and checks it against the
// two-tier policy the binary actually implements: only a fixable
// MISCONFIGURATION could-not-check is fatal (exit 1); a structurally
// unresolvable one is a non-fatal NOTICE (exit 0).
func TestModelAutoflipWorkflowCommentMatchesExitPolicy(t *testing.T) {
	path := filepath.Join("..", ".github", "workflows", "assay-statusgen.yml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("workflow not present in this tree: %v", err)
	}
	body := string(raw)

	idx := strings.Index(body, "\n  model-autoflip:")
	if idx == -1 {
		t.Skip("no model-autoflip job in this workflow — nothing to pin")
	}
	// The job's own doc comment is the contiguous run of `#`-prefixed lines
	// immediately above the job key.
	lines := strings.Split(body[:idx], "\n")
	start := len(lines)
	for start > 0 && strings.HasPrefix(strings.TrimSpace(lines[start-1]), "#") {
		start--
	}
	comment := strings.Join(lines[start:], "\n")
	lower := strings.ToLower(comment)

	// The comment must name the misconfiguration/structural split, or a future
	// edit can silently re-widen the claim to "every could-not-check is fatal"
	// with nothing catching it.
	if !strings.Contains(lower, "misconfig") {
		t.Errorf("the model-autoflip job comment no longer names the misconfiguration/structural "+
			"could-not-check split (autoflip.go's Fix B) — it must not claim every could-not-check "+
			"is fatal; comment:\n%s", comment)
	}
	if !strings.Contains(lower, "notice") || !strings.Contains(lower, "exits 0") {
		t.Errorf("the model-autoflip job comment must say the structurally-unresolvable kind is a "+
			"non-fatal NOTICE that exits 0 — see TestReportAutoFlipStructuralNonFatal; comment:\n%s", comment)
	}

	// The overclaim this test exists to catch: a blanket claim that ANY
	// could-not-check candidate fails the job, with no exception named.
	overclaims := []string{
		"a candidate the tool could not check fails this job loudly (exit 1)",
		"every could-not-check fails this job",
		"any could-not-check fails this job",
	}
	for _, bad := range overclaims {
		if strings.Contains(lower, bad) {
			t.Errorf("the model-autoflip job comment overclaims the exit policy (contains %q) — only a "+
				"fixable MISCONFIGURATION could-not-check is fatal; a structurally-unresolvable one is a "+
				"non-fatal NOTICE (reportAutoFlipModel / TestReportAutoFlipStructuralNonFatal)", bad)
		}
	}
}

// modelAutoflipWorkflows are the copies of the board workflow whose
// model-autoflip step is pinned below: the live file and its staged twin.
var modelAutoflipWorkflows = []string{
	filepath.Join("..", ".github", "workflows", "assay-statusgen.yml"),
	filepath.Join("..", "ci", "staged-workflows", "assay-statusgen.yml"),
}

// modelAutoflipExitPolicy returns the step's exit-policy shell (from its
// "# Exit policy of the step" comment through the closing `esac`), dedented
// to column 0, or "" when the job or the block is missing.
func modelAutoflipExitPolicy(wf string) string {
	job := jobBlock(wf, "  model-autoflip:")
	i := strings.Index(job, "          # Exit policy of the step")
	if i < 0 {
		return ""
	}
	rest := job[i:]
	const end = "\n          esac\n"
	j := strings.Index(rest, end)
	if j < 0 {
		return ""
	}
	var b strings.Builder
	for _, line := range strings.SplitAfter(rest[:j+len(end)], "\n") {
		b.WriteString(strings.TrimPrefix(line, "          "))
	}
	return b.String()
}

// TestModelAutoflipStepPassesCheck pins the wiring docs/board-stuck-autoflips.md
// ("In CI") describes: the flip runs with --check, its stdout is kept for the
// counts, and the step's exit policy has a block of its own.
func TestModelAutoflipStepPassesCheck(t *testing.T) {
	for _, path := range modelAutoflipWorkflows {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		job := jobBlock(string(raw), "  model-autoflip:")
		if job == "" {
			t.Fatalf("%s: no model-autoflip job", path)
		}
		for _, need := range []string{
			`--auto-flip-model --check --acceptance-forge > "${RUNNER_TEMP}/autoflip-out.txt" || flip_rc=$?`,
			"ASSAY_AUTOFLIP_STUCK_FAILS: ${{ vars.ASSAY_AUTOFLIP_STUCK_FAILS }}",
			`>> "${GITHUB_STEP_SUMMARY:-/dev/null}"`,
		} {
			if !strings.Contains(job, need) {
				t.Errorf("%s: model-autoflip job lacks %q", path, need)
			}
		}
		if modelAutoflipExitPolicy(string(raw)) == "" {
			t.Errorf("%s: model-autoflip step has no exit-policy block ending in esac", path)
		}
	}
}

// TestModelAutoflipExitPolicyRuns executes the step's exit-policy block for
// each exit the binary can give, with the switch off and on: exit 2 warns and
// stays green unless ASSAY_AUTOFLIP_STUCK_FAILS=true; exit 1 and any other
// non-zero exit fail the job whatever the switch says.
func TestModelAutoflipExitPolicyRuns(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the step runs on a Linux runner")
	}
	requireTool(t, "bash")
	requireTool(t, "grep")
	const out = "x/01 FLIPPED verified→done — stamp\n" +
		"x/02 REFUSED (stays verified) — no approval\n" +
		"x/03 REFUSED (stays verified) — not released\n" +
		"x/04 NOTICE COULD-NOT-CHECK (stays verified) — no merged PR\n"
	cases := []struct {
		rc, sw   string
		wantExit int
		want     string
	}{
		{"0", "", 0, "1 flipped, 2 REFUSED, 1 COULD-NOT-CHECK"},
		{"2", "", 0, "::warning::2 gate:model brief(s) REFUSED and 1 COULD-NOT-CHECK"},
		{"2", "false", 0, "::warning::"},
		{"2", "true", 2, "::error::2 gate:model brief(s) REFUSED"},
		{"1", "", 1, "::error::statusgen --auto-flip-model reported a fixable MISCONFIGURATION"},
		{"1", "true", 1, "MISCONFIGURATION"},
		{"3", "", 1, "neither a stuck candidate (2) nor a misconfiguration (1)"},
	}
	for _, path := range modelAutoflipWorkflows {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		block := modelAutoflipExitPolicy(string(raw))
		if block == "" {
			t.Fatalf("%s: no exit-policy block", path)
		}
		for _, c := range cases {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "autoflip-out.txt"), []byte(out), 0o644); err != nil {
				t.Fatal(err)
			}
			summary := filepath.Join(dir, "summary.md")
			cmd := exec.Command("bash", "-c", "set -uo pipefail\nflip_rc="+c.rc+"\n"+block)
			cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "RUNNER_TEMP=" + dir,
				"GITHUB_STEP_SUMMARY=" + summary, "ASSAY_AUTOFLIP_STUCK_FAILS=" + c.sw}
			got, err := cmd.CombinedOutput()
			code := 0
			if ee, ok := err.(*exec.ExitError); ok {
				code = ee.ExitCode()
			} else if err != nil {
				t.Fatalf("run: %v", err)
			}
			if code != c.wantExit || !strings.Contains(string(got), c.want) {
				t.Errorf("%s rc=%s switch=%q: exit %d (want %d), output must contain %q:\n%s",
					path, c.rc, c.sw, code, c.wantExit, c.want, got)
			}
			sum, _ := os.ReadFile(summary)
			if !strings.Contains(string(sum), "1 flipped, 2 REFUSED, 1 COULD-NOT-CHECK") {
				t.Errorf("%s rc=%s: job summary lacks the counts line: %q", path, c.rc, sum)
			}
		}
		// A Go runtime panic also exits 2. An exit 2 with no REFUSED or
		// COULD-NOT-CHECK line is a crash and must fail, whatever the switch says.
		for _, sw := range []string{"", "true"} {
			dir := t.TempDir()
			crash := "panic: runtime error: index out of range\n"
			if err := os.WriteFile(filepath.Join(dir, "autoflip-out.txt"), []byte(crash), 0o644); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("bash", "-c", "set -uo pipefail\nflip_rc=2\n"+block)
			cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "RUNNER_TEMP=" + dir,
				"GITHUB_STEP_SUMMARY=" + filepath.Join(dir, "summary.md"), "ASSAY_AUTOFLIP_STUCK_FAILS=" + sw}
			got, err := cmd.CombinedOutput()
			code := 0
			if ee, ok := err.(*exec.ExitError); ok {
				code = ee.ExitCode()
			} else if err != nil {
				t.Fatalf("run: %v", err)
			}
			if code != 1 || !strings.Contains(string(got), "not a stuck candidate") {
				t.Errorf("%s crash rc=2 switch=%q: exit %d (want 1), output must say it is not a stuck candidate:\n%s",
					path, sw, code, got)
			}
		}
	}
}
