package deskkit

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestAssayLintSingleWriterGuardFailsClosed executes the SHIPPED shell of the
// assay-lint composite action's single-writer step against constructed git
// repositories and asserts the exit code in each.
//
// WHY THIS EXISTS. `.github/actions/assay-lint` is adopter-facing code: it runs
// in someone else's CI, and its entire contract is its exit code. The step it
// guards has already failed open once — the original shape was
//
//	if git diff --name-only "origin/main...HEAD" | grep -qx STATUS.md; then
//
// where `git diff` is the left side of a pipeline inside an `if`. Two
// independent reasons `set -e` cannot fire there, and the `if` sees only grep's
// status, so ANY git error (rewritten base, unrelated history, shallow graft,
// missing object) was consumed as "no violation" and the step printed a pass
// while the violation sat in the diff. That is the same shape as any other
// finding where a tool's ERROR exit gets read as a NEGATIVE RESULT.
//
// A green run proves nothing about a gate. What has to be proven is that the
// gate goes RED on the violation AND RED on the error, and green ONLY when the
// branch is genuinely clean. So every scenario below except the first is a
// POSITIVE CONTROL on a failure path.
//
// LIMITS — stated rather than implied. This runs the step's `run:` script, not
// the action: `actions/setup-go`, the step-level `if: github.event_name ==
// 'pull_request'` and GitHub's own `${{ }}` interpolation are not exercised
// here (the event scope is asserted by reading action.yml in
// TestAssayLintGuardPullRequestScopedAndBashed below). It extracts the
// script by exact step NAME, so renaming the step in action.yml fails this test
// loudly rather than silently testing nothing.
func TestAssayLintSingleWriterGuardFailsClosed(t *testing.T) {
	// A guard that SKIPS on the runner is a guard that does not run where it
	// matters, and `go test` without -v prints nothing when it does. So the
	// skip is allowed on a developer machine and is a hard FAILURE in CI —
	// otherwise "the action can still fail" would be proven only locally while
	// the merge gate reported green on having checked nothing.
	requireTool := func(name string) {
		t.Helper()
		if _, err := exec.LookPath(name); err == nil {
			return
		}
		if os.Getenv("CI") != "" {
			t.Fatalf("%s is not on PATH, but CI is set. This test executes the shipped assay-lint guard; "+
				"skipping it here would report a green gate for a check that never ran.", name)
		}
		t.Skipf("%s not available (local run)", name)
	}
	requireTool("git")
	requireTool("bash")

	// Prove the body actually executed, so a future refactor cannot turn this
	// into a no-op that still reports ok.
	t.Log("assay-lint single-writer guard: executing the shipped step against constructed repos")

	script := assayLintStepScript(t, "Enforce single-writer rule — STATUS.md must not be committed on branches")

	// Guard against the extraction silently returning something inert: if the
	// script no longer contains the thing under test, every scenario below
	// would "pass" for the wrong reason.
	for _, must := range []string{"set -euo pipefail", "is-shallow-repository", "STATUS.md"} {
		if !strings.Contains(script, must) {
			t.Fatalf("extracted single-writer script does not contain %q — the step was rewritten and this "+
				"test is no longer exercising the guard it claims to. Script:\n%s", must, script)
		}
	}

	scriptPath := filepath.Join(t.TempDir(), "guard.sh")
	if err := os.WriteFile(scriptPath, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name       string
		violation  bool // does the branch commit STATUS.md?
		shallow    bool
		unrelated  bool // branch shares NO ancestor with the base — `git diff a...b` errors
		baseRef    string
		wantFail   bool
		wantOutput string // substring the failure must name, so a red for the WRONG reason is caught
	}{
		{
			name:       "clean branch, full history — green only when genuinely clean",
			violation:  false,
			baseRef:    "main",
			wantFail:   false,
			wantOutput: "single-writer rule OK",
		},
		{
			name:       "violation on full history — red on the violation",
			violation:  true,
			baseRef:    "main",
			wantFail:   true,
			wantOutput: "STATUS.md is generated — do not commit it on a branch",
		},
		{
			// Caught by the shallow PRE-CHECK, before any diff is attempted.
			// Note what this case does and does not prove: a depth-1 clone
			// whose branch was cut from the grafted tip still HAS a merge
			// base, so the diff would succeed here. This asserts the
			// pre-check's own behaviour, not the errored-diff path — that is
			// the next case's job.
			name:       "violation on a shallow clone — red at the pre-check",
			violation:  true,
			shallow:    true,
			baseRef:    "main",
			wantFail:   true,
			wantOutput: "fetch-depth: 0",
		},
		{
			// THE REGRESSION CASE, and the only one that is a genuine positive
			// control on the fail-open. The branch shares no ancestor with the
			// base, so `git diff base...HEAD` exits non-zero with "no merge
			// base" and produces NO output. Under the old pipeline-inside-`if`
			// shape grep saw an empty stream, matched nothing, and the step
			// reported a PASS with the violation sitting in the branch. The
			// shipped shape must report a FAILURE instead.
			name:       "violation on an unrelated history — errored diff is a FAILURE, not 'no violation'",
			violation:  true,
			unrelated:  true,
			baseRef:    "main",
			wantFail:   true,
			wantOutput: "could not diff this branch",
		},
		{
			// Fails closed, and says something an adopter can act on rather
			// than a bare git error plus a shell exit code.
			name:       "base ref does not exist — fails closed with a legible message",
			violation:  true,
			baseRef:    "no-such-branch",
			wantFail:   true,
			wantOutput: "could not fetch the base branch",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := assayLintFixtureRepo(t, tc.violation, tc.shallow, tc.unrelated)

			cmd := exec.Command("bash", scriptPath)
			cmd.Dir = repo
			cmd.Env = append(os.Environ(), "BASE_REF="+tc.baseRef)
			out, err := cmd.CombinedOutput()

			var exitErr *exec.ExitError
			failed := errors.As(err, &exitErr)
			if err != nil && !failed {
				t.Fatalf("could not run the guard: %v\n%s", err, out)
			}

			if failed != tc.wantFail {
				t.Fatalf("guard exit: got fail=%v want fail=%v.\nA gate that cannot fail is not a gate; a gate "+
					"that fails on a clean branch is noise.\n--- output ---\n%s", failed, tc.wantFail, out)
			}
			if !strings.Contains(string(out), tc.wantOutput) {
				t.Fatalf("guard produced the right exit for the wrong reason: output does not contain %q.\n"+
					"--- output ---\n%s", tc.wantOutput, out)
			}
		})
	}
}

// TestAssayLintGuardPullRequestScopedAndBashed pins the two properties of
// the single-writer step that the script itself cannot show: it is scoped to
// pull_request (documented as SKIPPED, not passed, on other events — an
// undocumented widening would make the published gate table untrue), and it
// declares `shell: bash`, without which a composite step runs under the
// runner's default shell and `set -euo pipefail` is not in force.
func TestAssayLintGuardPullRequestScopedAndBashed(t *testing.T) {
	raw := assayLintActionYAML(t)

	step := assayLintStepBlock(t, raw, "Enforce single-writer rule — STATUS.md must not be committed on branches")
	if !strings.Contains(step, "if: github.event_name == 'pull_request'") {
		t.Fatalf("the single-writer step lost its `if: github.event_name == 'pull_request'` scope. Both " +
			"docs/integrations/ci.md and the action README publish an Event scope column saying it runs on " +
			"pull_request only; widening it silently makes that table wrong for every adopter.")
	}
	if !strings.Contains(step, "shell: bash") {
		t.Fatalf("the single-writer step lost `shell: bash`. Without it the composite step does not run " +
			"under bash and `set -euo pipefail` is not in force — the guard's fail-closed behaviour " +
			"depends on it.")
	}

	// Every run: step in the action must declare a shell, for the same reason.
	for _, name := range assayLintRunStepNames(t, raw) {
		block := assayLintStepBlock(t, raw, name)
		if !strings.Contains(block, "shell: bash") {
			t.Fatalf("action step %q has a `run:` but no `shell: bash`", name)
		}
	}
}

// ---- fixture + extraction helpers -------------------------------------------

// assayLintActionPath is repo-relative from this package (tools/desk/internal/deskkit).
const assayLintActionPath = "../../../../.github/actions/assay-lint/action.yml"

func assayLintActionYAML(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(assayLintActionPath)
	if err != nil {
		t.Fatalf("cannot read %s: %v — this test must never pass because it could not find the action.",
			assayLintActionPath, err)
	}
	return string(b)
}

// assayLintStepBlock returns the YAML text of the step whose `name:` matches,
// from that name line to the start of the next sibling step. tools/desk is
// dependency-free, so this is a deliberate line-scanner rather than a YAML
// parse; it Fatals on any shape it does not understand rather than returning
// empty, since an empty block would make the assertions above vacuous.
func assayLintStepBlock(t *testing.T, raw, stepName string) string {
	t.Helper()
	lines := strings.Split(raw, "\n")
	start := -1
	for i, ln := range lines {
		if strings.HasSuffix(strings.TrimRight(ln, " \t"), "name: "+stepName) && strings.Contains(ln, "name:") {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatalf("no step named %q in %s — it was renamed or removed, so whatever this test asserts about "+
			"it is no longer being asserted about anything.", stepName, assayLintActionPath)
	}
	indent := len(lines[start]) - len(strings.TrimLeft(lines[start], " "))
	// A step's name line may be "- name: X" (indent points at the dash) or
	// "  name: X" under a leading "-". Either way the next step begins at a
	// line whose first non-space character is "-" at <= this indent.
	var b strings.Builder
	b.WriteString(lines[start] + "\n")
	for _, ln := range lines[start+1:] {
		trimmed := strings.TrimLeft(ln, " ")
		ind := len(ln) - len(trimmed)
		if strings.HasPrefix(trimmed, "- ") && ind <= indent && strings.TrimSpace(ln) != "" {
			break
		}
		b.WriteString(ln + "\n")
	}
	return b.String()
}

// assayLintStepScript returns the dedented body of a step's `run: |` block.
func assayLintStepScript(t *testing.T, stepName string) string {
	t.Helper()
	block := assayLintStepBlock(t, assayLintActionYAML(t), stepName)
	lines := strings.Split(block, "\n")
	start := -1
	for i, ln := range lines {
		if strings.TrimSpace(ln) == "run: |" {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatalf("step %q has no `run: |` block literal — it was rewritten into a form this test cannot "+
			"execute, so it is no longer proving the guard fails closed.", stepName)
	}
	runIndent := len(lines[start]) - len(strings.TrimLeft(lines[start], " "))
	var body []string
	bodyIndent := -1
	for _, ln := range lines[start+1:] {
		if strings.TrimSpace(ln) == "" {
			body = append(body, "")
			continue
		}
		ind := len(ln) - len(strings.TrimLeft(ln, " "))
		if ind <= runIndent {
			break
		}
		if bodyIndent < 0 {
			bodyIndent = ind
		}
		body = append(body, ln[bodyIndent:])
	}
	if len(body) == 0 {
		t.Fatalf("step %q has an empty `run:` body", stepName)
	}
	return strings.Join(body, "\n") + "\n"
}

// assayLintRunStepNames lists the names of every step in the action that has a
// `run:` key.
func assayLintRunStepNames(t *testing.T, raw string) []string {
	t.Helper()
	var names []string
	var current string
	for _, ln := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(ln)
		if strings.HasPrefix(trimmed, "- name: ") {
			current = strings.TrimPrefix(trimmed, "- name: ")
			continue
		}
		if strings.HasPrefix(trimmed, "name: ") && strings.HasPrefix(strings.TrimLeft(ln, " "), "name: ") {
			current = strings.TrimPrefix(trimmed, "name: ")
			continue
		}
		if (trimmed == "run: |" || strings.HasPrefix(trimmed, "run: ")) && current != "" {
			names = append(names, current)
			current = ""
		}
	}
	if len(names) == 0 {
		t.Fatal("found no `run:` steps in the action — the scanner is broken and every per-step assertion " +
			"built on it is vacuous.")
	}
	return names
}

// assayLintFixtureRepo builds an upstream repo with a committed STATUS.md and a
// clone on a feature branch, and returns the clone. `shallow` produces a depth-1
// clone; `unrelated` cuts the feature branch as an ORPHAN, so it shares no
// ancestor with the base and `git diff base...HEAD` errors for want of a merge
// base — the condition under which the guard used to report a false pass.
func assayLintFixtureRepo(t *testing.T, violation, shallow, unrelated bool) string {
	t.Helper()
	base := t.TempDir()
	up := filepath.Join(base, "up")
	clone := filepath.Join(base, "clone")

	git := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
		}
	}
	write := func(dir, name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.MkdirAll(up, 0o755); err != nil {
		t.Fatal(err)
	}
	git(up, "init", "-q", "--initial-branch=main")
	write(up, "STATUS.md", "generated board\n")
	write(up, "code.txt", "base\n")
	git(up, "add", "-A")
	git(up, "commit", "-qm", "base")
	// Extra commits so a depth-1 clone truly has no merge base to reach.
	for _, n := range []string{"1", "2", "3"} {
		write(up, "code.txt", "base\n"+n+"\n")
		git(up, "commit", "-qam", "c"+n)
	}

	if shallow {
		// A depth-1 clone requires a URL, not a bare path, for the shallow
		// negotiation to apply.
		git(base, "clone", "-q", "--depth", "1", "file://"+up, clone)
	} else {
		git(base, "clone", "-q", up, clone)
	}
	if unrelated {
		git(clone, "checkout", "-q", "--orphan", "feat")
		// --orphan keeps the index; clear it so the orphan commit shares no
		// blob lineage either and the branch is unambiguously unrelated.
		git(clone, "rm", "-rq", "--cached", ".")
		if err := os.Remove(filepath.Join(clone, "code.txt")); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(filepath.Join(clone, "STATUS.md")); err != nil {
			t.Fatal(err)
		}
	} else {
		git(clone, "checkout", "-qb", "feat")
	}

	if violation {
		write(clone, "STATUS.md", "generated board\nhand-edited on a branch\n")
	} else {
		write(clone, "code.txt", "feature work\n")
	}
	git(clone, "add", "-A")
	git(clone, "commit", "-qm", "branch work")

	return clone
}
