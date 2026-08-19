package deskkit

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestTreeSweepPipeCarriesPipefail is the fail-first guard for #621.
//
// THE REGRESSION IT PREVENTS. The leaksweep.yml `tree-sweep` job pipes the sweep
// through `tee` to capture its report for the post-merge SOFT-ALERT:
//
//	go run ./tools/leaksweep run --tree "${RUNNER_TEMP}/pubstage" 2>&1 | tee "${...}/leaksweep.out"
//
// A pipeline's exit status is that of its LAST command — `tee`, which always
// exits 0. So UNLESS `pipefail` is set, leaksweep's leak-exit (2) is swallowed
// and the step (hence the whole gate) goes GREEN on a real leak: exactly what
// happened on #619 (`failed=27`, job `success`), which also silently defanged
// the failure()-gated SOFT-ALERT. The fix is `set -eo pipefail` at the top of
// the step's run script (equivalently a `shell:` with pipefail, or an explicit
// `${PIPESTATUS[0]}` check). This test reddens if that plumbing is dropped, so
// the control cannot silently regress into a no-op on the runner again.
//
// It is deliberately NOT in citrigger_test.go's registry: that guard proves the
// sweep COMMAND still runs; this one proves the sweep's FAILURE still reaches the
// job. Both live in tools/desk, which tools.yml runs on `.github/workflows/**`,
// so any edit to leaksweep.yml runs this test.
func TestTreeSweepPipeCarriesPipefail(t *testing.T) {
	const repoRoot = "../../../.."
	const wf = ".github/workflows/leaksweep.yml"

	skipIfFixtureAbsent(t, filepath.Join(repoRoot, filepath.FromSlash(wf)),
		".github/ is not part of this repository's published file set")

	raw, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(wf)))
	if err != nil {
		t.Fatalf("cannot read %s: %v — the tree-sweep leak gate must exist", wf, err)
	}
	content := string(raw)

	jobLines, ok := ciWorkflowJobLines(t, content, wf, "tree-sweep")
	if !ok {
		t.Fatalf("%s has no jobs.tree-sweep — the staged-tree leak gate is gone (#621)", wf)
	}

	// Isolate the single step that runs the sweep. Splitting the job body on
	// step-list items (`- name:` / `- uses:` at the steps indent) keeps the
	// SOFT-ALERT step's own `set -euo pipefail` from satisfying this assertion
	// for the sweep step — the two must be checked independently.
	stepChunk := treeSweepStepContaining(jobLines, "leaksweep run --tree")
	if stepChunk == "" {
		t.Fatalf("jobs.tree-sweep no longer contains a step running `leaksweep run --tree` — " +
			"the leak gate's sweep command is gone or renamed (see citrigger_test.go); " +
			"cannot verify its pipe propagates a leak (#621)")
	}

	// Strip comment/blank lines so a comment merely MENTIONING pipefail (or the
	// sweep command) cannot satisfy either check below — only executable lines
	// count.
	executable := stripShellComments(stepChunk)

	// Only enforce pipefail when the sweep is actually piped: an unpiped sweep
	// (its own exit is the step's exit) needs no pipefail. The captured `tee`
	// output is what the SOFT-ALERT reads, so in practice it IS piped. Match the
	// `go run` invocation line, not the step `name:` (which also names the sweep).
	sweepLine := treeSweepCommandLine(executable, "go run ./tools/leaksweep run --tree")
	if !strings.Contains(sweepLine, "|") {
		t.Skipf("the `leaksweep run --tree` command is not piped, so pipefail is not required:\n\t%s", strings.TrimSpace(sweepLine))
	}
	if !pipefailHandled(executable) {
		t.Errorf("the tree-sweep sweep step pipes `leaksweep run --tree` through `tee` but does NOT "+
			"propagate the pipeline's failure: no `pipefail` and no `${PIPESTATUS[...]}` check in the "+
			"step body.\n\nThis is exactly #621: a pipeline's exit status is `tee`'s (0), "+
			"so leaksweep's leak-exit (2) is swallowed and the gate goes GREEN on a real leak (observed "+
			"on #619: failed=27, job success), which also disarms the failure()-gated SOFT-ALERT.\n\n"+
			"FIX the WORKFLOW: add `set -eo pipefail` at the top of the step's run script (or use "+
			"`shell: bash -eo pipefail {0}`, or check `${PIPESTATUS[0]}` after the pipe). Do NOT delete "+
			"this test.\n\nStep body was:\n%s", indentBlock(stepChunk))
	}
}

// treeSweepStepContaining returns the run-step chunk (its list item and all its
// lines) whose text contains needle, or "" if none does. It splits jobLines on
// step boundaries — a `- ` list item at the shallowest list indent in the body —
// so each returned chunk is one step and one step only.
func treeSweepStepContaining(jobLines []string, needle string) string {
	// Find the shallowest indent at which a `- ` list item appears; that is the
	// steps-list indent (the `steps:` entries), below the job-level keys.
	itemRe := regexp.MustCompile(`^(\s*)-\s`)
	stepIndent := -1
	for _, ln := range jobLines {
		if m := itemRe.FindStringSubmatch(ln); m != nil {
			if n := len(m[1]); stepIndent < 0 || n < stepIndent {
				stepIndent = n
			}
		}
	}
	if stepIndent < 0 {
		return ""
	}

	var chunks [][]string
	var cur []string
	for _, ln := range jobLines {
		if m := itemRe.FindStringSubmatch(ln); m != nil && len(m[1]) == stepIndent {
			if len(cur) > 0 {
				chunks = append(chunks, cur)
			}
			cur = []string{ln}
			continue
		}
		if len(cur) > 0 {
			cur = append(cur, ln)
		}
	}
	if len(cur) > 0 {
		chunks = append(chunks, cur)
	}

	// Match on the comment-stripped body so a leading comment block that merely
	// MENTIONS the sweep command (and attaches to the preceding step's chunk)
	// cannot masquerade as the step that actually RUNS it.
	for _, c := range chunks {
		joined := strings.Join(c, "\n")
		if strings.Contains(stripShellComments(joined), needle) {
			return joined
		}
	}
	return ""
}

// treeSweepCommandLine returns the first line of chunk containing needle.
func treeSweepCommandLine(chunk, needle string) string {
	for _, ln := range strings.Split(chunk, "\n") {
		if strings.Contains(ln, needle) {
			return ln
		}
	}
	return ""
}

// stripShellComments drops blank lines and lines whose first non-space rune is
// `#`, so a comment that merely mentions "pipefail" cannot satisfy the guard.
func stripShellComments(chunk string) string {
	var out []string
	for _, ln := range strings.Split(chunk, "\n") {
		t := strings.TrimSpace(ln)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		out = append(out, ln)
	}
	return strings.Join(out, "\n")
}

// pipefailHandled reports whether the executable step text propagates a pipe's
// failure: an explicit `pipefail` (via `set -o pipefail`, `set -eo pipefail`, or
// a `shell:` line) or a `${PIPESTATUS[...]}` inspection.
func pipefailHandled(executable string) bool {
	return strings.Contains(executable, "pipefail") || strings.Contains(executable, "PIPESTATUS")
}

// indentBlock indents every line of s by a tab for readable failure output.
func indentBlock(s string) string {
	var b strings.Builder
	for _, ln := range strings.Split(s, "\n") {
		b.WriteString("\t")
		b.WriteString(ln)
		b.WriteString("\n")
	}
	return b.String()
}
