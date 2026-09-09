package main

import (
	"os/exec"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// execCommand is the single seam through which every gh invocation flows. Production
// binds it to exec.Command; tests wrap it to RECORD every argv so the "zero write calls
// on a refusal path" and "the only mutating gh verb is `issue create`/`issue comment`"
// assertions run against the real constructed argv. Nothing else in this package
// constructs commands, so there is exactly one place argv is built.
var execCommand = exec.Command

// runCmd executes name+args and returns trimmed stdout. Commands are ALWAYS built from
// an explicit argv slice — never a shell string and never a caller-supplied gh flag.
// deskfile forwards NO caller flag into gh: the only external values that reach an argv
// are the repo, title, issue number and body-file path, each placed in a fixed argv
// position, so no external input can inject an option.
//
// The subprocess's STDERR is remote-influenced text (gh echoes API messages, and API
// messages quote issue titles authored by arbitrary users on the two PUBLIC repos in the
// fixed set) and it is printed verbatim by main when the error surfaces. It is stripped of
// control/ANSI sequences here — the single choke point every gh error passes through — so
// no call site can leak it by forgetting. Terminal-active bytes never reach a terminal.
// deskkit.StripControl keeps tab and newline, so multi-line gh diagnostics stay readable.
func runCmd(name string, args ...string) (string, error) {
	// The capture and the control-stripping are the shared runner's (deskkit/runtool.go):
	// ToolRun.SaidAll runs deskkit.StripControl over the child's stderr, so the choke-point
	// property this comment promises is preserved and is now shared with every other verb
	// rather than reimplemented here. The recording seam stays LOCAL — ToolCall.Start is
	// execCommand — so the "zero write calls on a refusal path" assertions still run against
	// the real constructed argv.
	r := deskkit.Run(deskkit.ToolCall{Name: name, Args: args, Start: execCommand})
	if r.Failed() {
		// gh states the API failure class on its own stderr (`HTTP 401` / `HTTP 403` /
		// `HTTP 429`), and those are three failures with three completely different fixes.
		// Carrying that first line into the message is what lets an operator tell a revoked
		// token from a rate limit from a repo the App cannot see; the exit status and the
		// argv ride along for DESK_TRACE.
		//
		// ExitUnverifiable, not ExitRefused: gh could not answer. Whether an unanswered
		// call is a REFUSAL is the caller's decision, made above this line — the dedupe gate
		// fails closed on it deliberately — and that decision is untouched here.
		return r.Stdout, r.Fail(deskkit.ExitUnverifiable, "%s %s", name, strings.Join(args, " "))
	}
	return r.Stdout, nil
}

// gh runs a gh subcommand under the AMBIENT gh identity. deskfile gates WHETHER and WHERE
// an issue is filed, never WHO: the caller's standing gh credential —
// worker session or human — is the filing identity, unchanged. It NEVER injects a token
// and NEVER mints an App installation token; there is no desktoken call on any path.
func gh(args ...string) (string, error) {
	return runCmd("gh", args...)
}
