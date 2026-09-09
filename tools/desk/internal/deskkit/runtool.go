package deskkit

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// runtool.go — the ONE subprocess runner the desk tools shell out through.
//
// WHY IT IS SHARED. Three commands had grown their own runner (deskdispatch's runCmd,
// deskwt's runGitStreams, deskfile's runCmd), each capturing the child's stderr and each
// deciding independently how much of it survived into the error the operator finally reads.
// They diverged in exactly the way that matters: one forwarded the child's message whole,
// one reduced it to its FIRST line — which, for a desk tool, is the `assay-config:` echo and
// never the tool's own message — and one dropped it entirely. The operator-visible result
// was the same in all three cases: a bare "exit status 6" with the diagnosis one process
// away, and a bisect by hand re-running the child to see what it had already said.
//
// So the capture, the exit-code recovery, the preamble strip and the message shape live
// here, once. A caller states its own CONTEXT sentence and its own verdict (refused vs
// unverifiable, which stays the caller's decision — see ToolRun.Fail); everything about how
// the child's own words reach the message is this file's business.
//
// The runner does not decide exit codes and does not soften refusals: a child that said no
// still said no. It only makes sure that what it said is legible.

// ToolCall describes one child process to run.
type ToolCall struct {
	// Name and Args are the argv, ALWAYS as an explicit slice of literal verbs plus
	// already-validated values — never a shell string. The runner never interposes a shell,
	// so no value in Args can be re-read as an option by anything but the child itself.
	Name string
	Args []string
	// Dir is the working directory; empty inherits the parent's.
	Dir string
	// Env, when non-nil, REPLACES the child's environment (the os/exec contract). A caller
	// that means to add one variable passes append(os.Environ(), "K=V").
	Env []string
	// Start is the command constructor. Nil means exec.Command. It exists so a package that
	// already owns a recording seam (`var execCommand = exec.Command`, which its tests wrap
	// to assert on the real constructed argv) keeps that seam while delegating the capture
	// here: the runner is shared WITHOUT any package having to give up its own argv
	// assertions.
	Start func(name string, args ...string) *exec.Cmd
}

// ToolRun is the outcome of one child process: what it printed, what it exited with, and
// how long it took. Every field is populated on both the success and the failure path — a
// trace of a SUCCESSFUL step is as much of the diagnosis as the failing one.
type ToolRun struct {
	Name     string
	Args     []string
	Dir      string
	Stdout   string
	Stderr   string
	ExitCode int
	Elapsed  time.Duration
	// Err is the raw os/exec error: nil on success, an *exec.ExitError for a child that ran
	// and exited non-zero, and something else entirely for a child that could not be
	// started. The distinction is preserved rather than flattened, because "the tool did not
	// run" and "the tool said no" are different answers and only one of them is a decision.
	Err error
}

// Run executes c and captures both streams. It never returns an error of its own: a failure
// is a field of the result, so a caller cannot accidentally drop the captured stderr by
// checking only the error.
func Run(c ToolCall) ToolRun {
	start := c.Start
	if start == nil {
		start = exec.Command
	}
	cmd := start(c.Name, c.Args...)
	if c.Dir != "" {
		cmd.Dir = c.Dir
	}
	if c.Env != nil {
		cmd.Env = c.Env
	}
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	began := time.Now()
	err := cmd.Run()
	r := ToolRun{
		Name:     c.Name,
		Args:     append([]string(nil), c.Args...),
		Dir:      c.Dir,
		Stdout:   strings.TrimSpace(out.String()),
		Stderr:   strings.TrimSpace(errb.String()),
		ExitCode: ExitStatusOf(err),
		Elapsed:  time.Since(began),
		Err:      err,
	}
	traceRecord(r)
	return r
}

// ExitStatusOf recovers a child process's exit status so a wrapped tool's own deskkit exit
// contract (5 = it refused, 6 = it could not establish something) passes THROUGH the wrapper
// rather than being flattened into one generic failure.
//
// A process that did not exit with a status at all — it could not be started, or was
// signalled — is ExitUnverifiable, never ExitRefused.
func ExitStatusOf(err error) int {
	if err == nil {
		return ExitOK
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return ExitUnverifiable
}

// Failed reports whether the child did not exit 0.
func (r ToolRun) Failed() bool { return r.Err != nil }

// CommandLine renders the argv AS EXECUTED — the exact spelling an operator can paste to
// reproduce the step — scrubbed of credentials. Arguments carrying whitespace are quoted so
// the rendering is unambiguous; it is a faithful transcript, not a shell-escaped one, and it
// is never re-parsed by anything.
func (r ToolRun) CommandLine() string {
	parts := make([]string, 0, len(r.Args)+1)
	parts = append(parts, r.Name)
	for _, a := range r.Args {
		if a == "" || strings.ContainsAny(a, " \t\"'") {
			parts = append(parts, fmt.Sprintf("%q", a))
		} else {
			parts = append(parts, a)
		}
	}
	return Scrub(strings.Join(parts, " "))
}

// ToolMessage is what a wrapped desk tool actually SAID, kept whole.
//
// Every desk tool opens its stderr with the effective-config echo (`assay-config: …`) and,
// on an unpinned build, the drift warning. A report that shows only the FIRST stderr line
// therefore shows the config echo and never the tool's own message — which is how a
// stale-branch collision reached an operator as "worktree-create failed (assay-config: …)"
// and cost several claim-acquire/steal cycles chasing a phantom claim problem.
//
// So: drop the known preamble lines and return the REST verbatim, every line of it. The
// `assay-config: REFUSED —` line is deliberately NOT preamble: when the roster is what
// refused, it is the message. An empty remainder renders as "(no output)" so a report can
// never read as though a tool said something it did not.
func ToolMessage(stderr string) string {
	var kept []string
	for _, line := range strings.Split(strings.TrimSpace(stderr), "\n") {
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
	if len(kept) == 0 {
		return "(no output)"
	}
	return strings.Join(kept, "\n")
}

// SaidAll is every line the child said on stderr that is the child's OWN message — preamble
// stripped, control sequences stripped, credentials redacted. It falls back to stdout when
// stderr carried nothing but preamble, because a tool that reports on stdout (the claim
// tool's dedup lines are one) has still said something.
func (r ToolRun) SaidAll() string {
	msg := ToolMessage(r.Stderr)
	if msg == "(no output)" && strings.TrimSpace(r.Stdout) != "" {
		msg = strings.TrimSpace(r.Stdout)
	}
	return Scrub(msg)
}

// Said is the FIRST line of SaidAll — the one-line form a single-line refusal message ends
// on. The rest of the message is never lost: it is carried on the DeskError's Stderr field
// and printed in full under DESK_TRACE.
func (r ToolRun) Said() string {
	msg := r.SaidAll()
	if i := strings.IndexByte(msg, '\n'); i >= 0 {
		return strings.TrimSpace(msg[:i])
	}
	return msg
}

// Fail builds the DeskError for a failed run: the caller's context sentence, then the
// child's own first line, in the fixed shape
//
//	<context> — <tool> said: <first stderr line>
//
// The VERDICT stays the caller's: code is whatever the caller decided this failure means
// (typically ExitRefused when the child's own exit code says it refused, ExitUnverifiable
// otherwise — see ExitStatusOf). The runner never upgrades or downgrades a refusal.
//
// The full stderr, the command line as executed and the child's exit status ride along on
// the error's Stderr / Cmd / ExitStatus fields for DESK_TRACE to print, and the raw exec
// error stays the Cause so errors.Is / errors.As still reach *exec.ExitError.
func (r ToolRun) Fail(code int, format string, a ...any) *DeskError {
	return &DeskError{
		Code:       code,
		Msg:        fmt.Sprintf(format, a...) + " — " + r.Name + " said: " + r.Said(),
		Err:        r.Err,
		Cmd:        r.CommandLine(),
		Stderr:     r.SaidAll(),
		ExitStatus: r.ExitCode,
	}
}
