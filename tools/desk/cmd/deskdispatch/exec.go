package main

import (
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

// mintTokenFn is the seam the CLAIM step's role-token mint runs through (resolveClaimAuth,
// issue 1151), so a full-run dispatch test hands the claim child a stub token without a real
// App credential. Production binds it to the shared deskkit resolver, which shells out to the
// token minter and reads the file it names. The model stamp does NOT use it: its credential
// is read inside deskkit.ResolveForge, under the resolver's own custody hook.
var mintTokenFn = deskkit.RoleTokenForRepo

// NO FORGE CLI. Every forge read and write this verb makes — the model stamp's label
// reads and writes, the review-lane queue label — goes through the resolved deskkit.Forge
// under an explicitly minted role credential (deskkit.ResolveForge), never through `gh` or
// `glab`. The children that DO flow through runCmd are the consumer claim/decision scripts,
// deskwt, deskroster and git. The forge-CLI ban (internal/forgeban) reads this file's exec
// site as an unresolved argv[0] and this package carries no permit row, so a `gh` reaching
// this seam again is a red test, not a silent regression.
//
// WHY THERE IS NO AMBIENT FALLBACK. The only thing this verb writes to the forge is the
// dispatch attestation — two labels whose whole value is WHO applied them. Both Forge
// backends refuse to construct a client without an explicitly minted token, and the resolver
// reads the lane's own dispatcher credential; a stamp written under whatever credential the
// calling shell holds is the shape the capability floor's applier-aware reader exists to
// refuse, and worse than not stamping at all.

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
	return runCmdEnv(dir, nil, name, args...)
}

// runCmdEnv is runCmd with an explicit child environment. env follows the os/exec contract:
// nil inherits this process's environment, non-nil REPLACES it (so a caller that means to add
// one variable passes append(os.Environ(), "K=V")). It exists for the claim child (issue
// 1151), which the legacy claim script authenticates through GH_TOKEN in its environment;
// every other call site passes nil through runCmd.
func runCmdEnv(dir string, env []string, name string, args ...string) runResult {
	call := deskkit.ToolCall{Name: name, Args: args, Dir: dir, Env: env, Start: execCommand}
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
