package deskkit

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// trace.go — DESK_TRACE, the one diagnostic switch every desk verb honours.
//
// THE CONTRACT, in three sentences.
//
//  1. OFF (the default) the tools behave and print exactly as they did before this file
//     existed. ReportError with tracing off emits err.Error() and a newline — byte-identical
//     to the `fmt.Fprintln(os.Stderr, err.Error())` every main() used to call — so no
//     transcript, no parser and no test that pins a message shape moves.
//  2. ON (`DESK_TRACE=1`, or a global `--trace` on a verb that parses flags) the same first
//     line is followed by a `desk-trace:` block: the full cause chain, the exact command line
//     of every child process the run started, each child's exit status and elapsed time, and
//     the failing child's stderr in full rather than its first line.
//  3. Everything in that block goes through Scrub first. A trace prints argv and child stderr
//     — two places a credential really travels — so redaction is not optional and is not a
//     per-call-site decision: it happens at this one choke point. A trace must never be the
//     leak the tools' outward-write scan exists to prevent.
//
// WHY AN ENV VAR IS THE PRIMARY FORM. The callers that most need this are dispatched agents
// and loop supervisors, which invoke desk verbs through wrappers, scripts and other desk
// verbs. An env var crosses all of those without every intermediate layer having to learn
// and forward a flag; a flag would be honoured by the verb the operator typed and lost by
// every child it shells out to. The flag exists as the interactive convenience and sets the
// same switch.

var (
	traceMu sync.Mutex
	// traceOn is the resolved switch. It is a tri-state internally — unset, on, off — so a
	// verb that calls SetTrace from a --trace flag WINS over the environment, and so the
	// environment is read exactly once per process rather than on every child.
	traceResolved bool
	traceOn       bool
	// traceSteps is the per-process ledger of child processes. It is only appended to while
	// tracing is ON: with the switch off the runner records nothing, which is both why the
	// off-path costs nothing and why no child's stderr is retained in memory by a process
	// that was never asked to trace it.
	traceSteps []ToolRun
)

// traceStepCap bounds the ledger. A loop verb can start thousands of children in one
// process; a trace block is a diagnostic, not an audit log, and the tail is the part that
// explains a failure. Past the cap the OLDEST entries are dropped and the block says so.
const traceStepCap = 200

// traceDropped counts entries evicted by the cap, so the rendered block can say "…" instead
// of silently presenting a truncated ledger as complete.
var traceDropped int

// TraceEnabled reports whether trace diagnostics are on for this process.
func TraceEnabled() bool {
	traceMu.Lock()
	defer traceMu.Unlock()
	if !traceResolved {
		traceOn = envTruthy(os.Getenv("DESK_TRACE"))
		traceResolved = true
	}
	return traceOn
}

// SetTrace turns tracing on or off explicitly, overriding the environment for the rest of
// the process. A verb that parses a global `--trace` calls SetTrace(true); tests call it
// both ways and pair it with ResetTrace.
func SetTrace(on bool) {
	traceMu.Lock()
	defer traceMu.Unlock()
	traceOn = on
	traceResolved = true
}

// ResetTrace clears the resolved switch and the ledger. It exists for tests, which must be
// able to run an on-case and an off-case in one process without the first leaking into the
// second.
func ResetTrace() {
	traceMu.Lock()
	defer traceMu.Unlock()
	traceResolved = false
	traceOn = false
	traceSteps = nil
	traceDropped = 0
}

// envTruthy reads the env-var spellings an operator actually types. Anything else — including
// the empty string and "0" — is off, so an accidentally-exported empty variable does not turn
// diagnostics on for a whole session.
func envTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// TakeTraceFlag strips a global `--trace` / `-trace` from args and sets the switch. It is
// called by a verb's run() BEFORE any flag parsing or subcommand dispatch, which is what
// makes `--trace` global rather than per-subcommand: a verb with three subcommands and three
// FlagSets does not have to declare it three times, and a verb whose positional grammar
// would reject an unknown flag never sees it.
//
// It returns args with the flag removed, so every downstream parser sees exactly what it saw
// before. A bare `--` terminator stops the scan: after it, `--trace` is a positional value
// belonging to the verb and is left alone.
func TakeTraceFlag(args []string) []string {
	out := make([]string, 0, len(args))
	for i, a := range args {
		if a == "--" {
			out = append(out, args[i:]...)
			return out
		}
		if a == "--trace" || a == "-trace" {
			SetTrace(true)
			continue
		}
		out = append(out, a)
	}
	return out
}

// traceRecord appends one child-process outcome to the ledger. Called by Run; a no-op when
// tracing is off.
func traceRecord(r ToolRun) {
	if !TraceEnabled() {
		return
	}
	traceMu.Lock()
	defer traceMu.Unlock()
	traceSteps = append(traceSteps, r)
	if len(traceSteps) > traceStepCap {
		drop := len(traceSteps) - traceStepCap
		traceSteps = append([]ToolRun(nil), traceSteps[drop:]...)
		traceDropped += drop
	}
}

// TraceSteps returns a copy of the recorded child-process ledger.
func TraceSteps() []ToolRun {
	traceMu.Lock()
	defer traceMu.Unlock()
	return append([]ToolRun(nil), traceSteps...)
}

// ReportError is the shared main/exit path: every retrofitted verb's main() calls it in
// place of `fmt.Fprintln(os.Stderr, err.Error())`, then returns ExitCodeOf(err).
//
// It is the ONE place the trace block is rendered, which is what makes DESK_TRACE global
// rather than per-verb: a verb opts in by routing its terminal error through here, and
// everything the switch promises — cause chain, command lines, exit codes, timings,
// redaction — arrives with it.
//
// With tracing OFF the output is exactly err.Error() and a newline. Nothing else. A nil
// error prints nothing at all.
func ReportError(w io.Writer, err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(w, err.Error())
	if !TraceEnabled() {
		return
	}
	writeTrace(w, err)
}

// subprocessDetail walks the whole cause chain and returns the FIRST *DeskError carrying
// subprocess detail (Cmd set — only ToolRun.Fail sets it), or nil when no child process
// produced this error.
func subprocessDetail(err error) *DeskError {
	for e := err; e != nil; e = errors.Unwrap(e) {
		if de, ok := e.(*DeskError); ok && de.Cmd != "" {
			return de
		}
	}
	return nil
}

// writeTrace renders the diagnostic block. Every line is prefixed `desk-trace: ` so it can
// be grepped out of a mixed transcript, and every value passes through Scrub.
func writeTrace(w io.Writer, err error) {
	fmt.Fprintln(w, "desk-trace: ---- DESK_TRACE diagnostics ----")

	// 1. The cause chain, outermost first. This is the part that the one-line message
	// deliberately compresses: a DeskError renders "<msg>: <cause>", and a cause three
	// wrappers deep reaches the operator as one run-on sentence with no indication of where
	// the boundaries are.
	fmt.Fprintln(w, "desk-trace: cause chain:")
	depth := 0
	for e := err; e != nil; e = errors.Unwrap(e) {
		depth++
		fmt.Fprintf(w, "desk-trace:   %d. (%T) %s\n", depth, e, Scrub(e.Error()))
		if depth > 32 { // a cyclic or pathological chain is a bug, not a reason to hang
			fmt.Fprintln(w, "desk-trace:   … chain truncated at 32")
			break
		}
	}

	// 2. The subprocess detail carried on the error itself — set by ToolRun.Fail. This is
	// the child whose failure produced this error, named exactly and quoted in full.
	//
	// It is found by WALKING the chain rather than by a single errors.As, because errors.As
	// stops at the OUTERMOST *DeskError and the subprocess detail is almost always on an
	// INNER one: a step wraps the runner's error in its own context sentence, so the outer
	// DeskError carries the sentence and the inner one carries the command. Stopping at the
	// first match printed nothing at all for exactly the nested case the trace is for.
	if de := subprocessDetail(err); de != nil {
		if de.Cmd != "" {
			fmt.Fprintf(w, "desk-trace: failing command: %s\n", de.Cmd)
		}
		if de.ExitStatus != 0 {
			fmt.Fprintf(w, "desk-trace: child exit status: %d\n", de.ExitStatus)
		}
		if de.Stderr != "" {
			fmt.Fprintln(w, "desk-trace: child stderr (full):")
			for _, line := range strings.Split(de.Stderr, "\n") {
				fmt.Fprintf(w, "desk-trace:   | %s\n", line)
			}
		}
	}

	// 3. Every child this process started, in order, with its exit status and elapsed time.
	// A step that SUCCEEDED is as much of the diagnosis as the one that failed: the commonest
	// question a stuck operator has is which of the steps ran at all.
	steps := TraceSteps()
	traceMu.Lock()
	dropped := traceDropped
	traceMu.Unlock()
	if len(steps) == 0 && dropped == 0 {
		fmt.Fprintln(w, "desk-trace: child processes: none recorded (no subprocess ran through the shared runner)")
		return
	}
	if dropped > 0 {
		fmt.Fprintf(w, "desk-trace: child processes (%d shown, %d older entries dropped):\n", len(steps), dropped)
	} else {
		fmt.Fprintf(w, "desk-trace: child processes (%d):\n", len(steps))
	}
	for i, s := range steps {
		where := ""
		if s.Dir != "" {
			where = " [dir " + Scrub(s.Dir) + "]"
		}
		fmt.Fprintf(w, "desk-trace:   [%d] exit %d in %s%s — %s\n",
			i+1, s.ExitCode, s.Elapsed.Round(time.Millisecond), where, s.CommandLine())
		if s.Failed() {
			for _, line := range strings.Split(s.SaidAll(), "\n") {
				fmt.Fprintf(w, "desk-trace:       | %s\n", line)
			}
		}
	}
}
