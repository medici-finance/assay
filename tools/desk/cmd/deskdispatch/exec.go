package main

import (
	"bytes"
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
	cmd := execCommand(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if name == "gh" {
		cmd.Env = append(os.Environ(), "GH_TOKEN="+dispatcherToken)
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

// exitCodeOf recovers a child process's exit status so the wrapped scripts' deskkit
// contract (5 = a live holder owns it, 6 = could not be established) passes THROUGH this
// verb rather than being flattened into one generic failure.
//
// A process that did not exit with a status at all — it could not be started, or was
// signalled — is UNVERIFIABLE, never refused: "the tool did not run" and "the tool said
// no" are different answers, and only one of them is a decision.
func exitCodeOf(err error) int {
	if err == nil {
		return deskkit.ExitOK
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return deskkit.ExitUnverifiable
}

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

// toolMessage is what a wrapped desk tool actually SAID, kept whole.
//
// Every desk tool opens its stderr with the effective-config echo (`assay-config: …`) and,
// on an unpinned build, the drift warning. A step report that shows only the FIRST stderr
// line therefore shows the config echo and never the tool's own message — which is how a
// stale-branch collision reached an operator as "worktree-create failed (assay-config: …)"
// and cost several claim-acquire/steal cycles chasing a phantom claim problem.
//
// So: drop the known preamble lines and return the REST verbatim, every line of it. The
// `assay-config: REFUSED —` line is deliberately NOT preamble: when the roster is what
// refused, it is the message. An empty remainder renders as "(no output)" so a report can
// never read as though a tool said something it did not.
func toolMessage(stderr string) string {
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
