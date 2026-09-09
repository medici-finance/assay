package main

import (
	"os/exec"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// execCommand is the single seam through which every git invocation flows. Production
// binds it to exec.Command; tests wrap it to RECORD every argv (so the "no --force in
// any git argv" assertion is checked on the real constructed argv) while still
// delegating to a real git process against a scratch fixture. Nothing else in this
// package constructs commands, so there is exactly one place argv is built.
var execCommand = exec.Command

// runGit executes `git <args...>` in dir and returns trimmed stdout. The argv is
// ALWAYS an explicit slice built from literal verbs plus values that have already been
// regex-validated (name/branch/base) or derived from git state — never a shell string
// and never a raw caller flag ("constructed argv only, no caller flag passthrough").
// A `--force` can therefore never reach git through this path.
func runGit(dir string, args ...string) (string, error) {
	stdout, _, err := runGitStreams(dir, args...)
	return stdout, err
}

// runGitStreams is runGit with the STDERR text handed back on success too. Some git verbs
// report the work they did on stderr rather than stdout — `git worktree prune --verbose` is
// one of them — so a caller that reads only stdout sees an empty report and counts zero no
// matter how much was pruned. Callers that need to COUNT what git did use this; everything
// else keeps the simpler runGit. Same single-seam argv discipline: the argv is still an
// explicit slice of literal verbs and validated values.
func runGitStreams(dir string, args ...string) (stdout, stderr string, err error) {
	// The capture and the exit-status recovery are the shared runner's (deskkit/runtool.go).
	// The recording seam stays LOCAL — ToolCall.Start is execCommand — so the "no --force in
	// any git argv" assertion still runs against the real constructed argv.
	r := deskkit.Run(deskkit.ToolCall{Name: "git", Args: args, Dir: dir, Start: execCommand})
	if r.Failed() {
		// The message reads as it always did — `git <argv> failed — git said: <first line>`
		// — but the whole diagnosis now RIDES on the error rather than only being rendered
		// into it: git's exit status, its stderr in full, and the argv as executed, which is
		// what DESK_TRACE prints. Before this, a deskwt failure gave a trace nothing to show.
		//
		// The verdict is deliberately ExitUnverifiable, not ExitRefused. git's own exit
		// codes are not deskkit's: a 128 means "git could not do this", never "a desk
		// control said no". Every caller that turns a git failure into a REFUSAL does so on
		// its own reading of the repository state, above this line — see cmdAdd's branch
		// guard — and that reading is untouched here.
		return r.Stdout, r.Stderr, r.Fail(deskkit.ExitUnverifiable, "git %s", strings.Join(args, " "))
	}
	return r.Stdout, r.Stderr, nil
}
