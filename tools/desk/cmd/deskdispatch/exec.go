package main

import (
	"errors"
	"os"
	"os/exec"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// execCommand is the single seam through which EVERY child process this verb starts
// flows. Production binds it to exec.Command; tests wrap it to record the real
// constructed argv, which is how assertions like "no steal verb is ever invoked" and "no
// prompt is emitted after a refused claim" are checked against what would actually have
// run. Nothing else in this package builds a command.
//
// Every argv here is an explicit slice of literal verbs plus already-validated values —
// never a shell string. The item key in particular is regex-bounded before it reaches a
// child process, so a key cannot be read as a flag or escape into a path.
var execCommand = exec.Command

// lookPath is the PATH probe used to decide whether the pure-Go claim binary
// (goClaimBinary) is available when a repo carries no tools/dispatch-claim.sh. It is a seam
// ONLY so a test controls availability deterministically instead of depending on whatever
// happens to be installed on the test runner's PATH.
var lookPath = exec.LookPath

// mintTokenFn is the seam the DISPATCHER App-token lookup runs through, so the stamp step
// can be exercised without a real App credential. Production binds it to the shared
// deskkit resolver, which shells out to the token minter and reads the file it names.
var mintTokenFn = deskkit.RoleTokenForRepo

// dispatcherToken is the DISPATCHER App installation token every `gh` invocation from this
// verb authenticates with. It is set by the stamp step, from the role deskkit declares as
// the dispatcher, before the first label is applied.
//
// WHY AN EMPTY VALUE IS A REFUSAL AND NEVER A FALLBACK. The only thing this verb writes to
// the forge is the dispatch attestation — two labels whose whole value is WHO applied
// them. With no token in the child's environment `gh` authenticates as whatever credential
// the calling shell holds (another role's App, or the operator's own login), and the
// capability floor's applier-aware reader then sees a dispatched-* label from a
// non-dispatcher: the exact shape it exists to refuse. The result is worse than not
// stamping at all — an unstamped PR reads UNKNOWN and proceeds with a NOTICE, while a
// PR stamped under the wrong identity refuses every authority-bearing write made on it.
// So the ambient credential is never a fallback here.
var dispatcherToken string

type runResult struct {
	stdout string
	stderr string
	err    error
	// run is the shared runner's whole record of the child: its argv as executed, its exit
	// status, how long it took, and its stderr with the config-echo preamble stripped. The
	// three legacy fields above are kept as-is so no existing call site moves; every step
	// that REPORTS a failure reads its diagnosis from here instead, because `.stderr` is the
	// raw capture whose first line is the config echo and never the tool's own message.
	run deskkit.ToolRun
}

func runCmd(dir, name string, args ...string) runResult {
	// The fail-closed backstop for the rule above: even if a future code path reached a
	// forge call before the token was minted, the call does not happen. The stamp step's
	// own mint is the check a caller sees; this is the one that cannot be forgotten.
	if name == "gh" && dispatcherToken == "" {
		return runResult{err: errors.New(
			"refusing to run gh with no dispatcher App installation token — the dispatch stamp is an " +
				"attestation about WHO applied it, so it is never written under the ambient gh identity")}
	}
	call := deskkit.ToolCall{Name: name, Args: args, Dir: dir, Start: execCommand}
	if name == "gh" {
		call.Env = append(os.Environ(), "GH_TOKEN="+dispatcherToken)
	}
	// The capture, the exit-status recovery and the preamble strip are the shared runner's
	// (deskkit/runtool.go). The recording seam stays LOCAL — ToolCall.Start is execCommand —
	// so every argv assertion in this package's tests still runs against the real
	// constructed argv.
	r := deskkit.Run(call)
	return runResult{stdout: r.Stdout, stderr: r.Stderr, err: r.Err, run: r}
}

// exitCodeOf recovers a child process's exit status. It delegates to deskkit.ExitStatusOf,
// which is the single implementation of the rule it used to state here: the wrapped scripts'
// deskkit contract (5 = a live holder owns it, 6 = could not be established) passes THROUGH
// this verb rather than being flattened, and a process that did not exit with a status at all
// — it could not be started, or was signalled — is UNVERIFIABLE, never refused.
func exitCodeOf(err error) int { return deskkit.ExitStatusOf(err) }

// firstLine reduces output to one line for a step report; an empty result renders as
// "(no output)" so a report can never read as though a tool said something it did not.
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

// toolMessage is what a wrapped desk tool actually SAID, kept whole — the config echo and
// the unpinned-build warning dropped, every other line verbatim. The rule now lives in
// deskkit (runtool.go) because three commands needed it and only this one had it; this stays
// as the local spelling its call sites read.
func toolMessage(stderr string) string { return deskkit.ToolMessage(stderr) }

// repoSlugFromURL reduces an origin URL to owner/name for the SSH and HTTPS spellings.
// Anything else returns "" so the caller refuses instead of acting on a guess.
func repoSlugFromURL(url string) string {
	s := strings.TrimSpace(url)
	s = strings.TrimSuffix(s, ".git")
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
		if at := strings.Index(s, "@"); at >= 0 {
			s = s[at+1:]
		}
	} else if i := strings.Index(s, ":"); i >= 0 && strings.Contains(s[:i], "@") {
		s = s[i+1:]
		parts := strings.Split(s, "/")
		if len(parts) < 2 {
			return ""
		}
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}
	parts := strings.Split(strings.Trim(s, "/"), "/")
	if len(parts) < 2 {
		return ""
	}
	return parts[len(parts)-2] + "/" + parts[len(parts)-1]
}
