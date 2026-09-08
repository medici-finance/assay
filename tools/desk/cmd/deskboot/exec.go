package main

import (
	"bytes"
	"os/exec"
	"strings"
)

// execCommand is the single seam through which EVERY child process this verb starts
// flows. Production binds it to exec.Command; tests wrap it to record the real
// constructed argv, so assertions like "the boot never runs a write verb before the
// preflight is green" are checked against what would actually have been executed.
// Nothing else in this package builds a command, so there is exactly one place argv is
// assembled — and every value in it is a literal verb or an already-validated flag
// value, never a shell string.
var execCommand = exec.Command

// runResult is one child process's outcome. stderr is kept separate from stdout because
// a step's FAILURE line is what the boot must name, and folding the two streams makes a
// noisy tool's progress chatter indistinguishable from its diagnosis.
type runResult struct {
	stdout string
	stderr string
	err    error
}

// runCmd executes name+args with cwd=dir (empty dir = inherit) and returns the trimmed
// streams. It never interprets the exit status: the CALLER decides what a non-zero exit
// means for its own step, because "the prune found nothing to do" and "the preflight came
// back red" are not the same kind of non-zero.
func runCmd(dir, name string, args ...string) runResult {
	cmd := execCommand(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	return runResult{
		stdout: strings.TrimSpace(out.String()),
		stderr: strings.TrimSpace(errb.String()),
		err:    err,
	}
}

// firstLine reduces a tool's output to its most useful single line for a step report. An
// empty result renders as "(no output)" rather than as an empty string, so a report can
// never read as though a tool said something it did not.
func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "(no output)"
	}
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

// preflightSummary extracts the roster-preflight's OWN verdict from a red run's captured
// streams, skipping the two preamble lines every desk tool prints on stderr BEFORE its
// real message: the effective-config echo (`assay-config: …`) and the unpinned-build
// warning. A naive firstLine over that output quotes the banner's
// `… configured=true` header instead of the `preflight role=… RED n/5` summary and the
// per-check `<check>=checked-failed: … → fix: …` remediations, so the failing check never
// reaches the pod log — the boot logs the banner three ticks running and the real red stays
// unknown (assay#660). The REFUSED roster line is deliberately NOT preamble: when the
// roster itself refused, that line IS the message. Every kept line is returned verbatim so
// the summary and every remediation survive; an empty remainder renders as "(no output)"
// so a report never reads as though the tool spoke.
func preflightSummary(streams ...string) string {
	var kept []string
	for _, s := range streams {
		for _, line := range strings.Split(s, "\n") {
			t := strings.TrimSpace(line)
			if t == "" {
				continue
			}
			if strings.HasPrefix(t, "assay-config: ") && !strings.HasPrefix(t, "assay-config: REFUSED") {
				continue
			}
			if strings.HasPrefix(t, "desk-tools WARNING: running UNPINNED") {
				continue
			}
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		return "(no output)"
	}
	return strings.Join(kept, "\n")
}
