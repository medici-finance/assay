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
// constructed argv, which is how the assertion "nothing is mutated after a refused
// condition" is checked against what would actually have run. Nothing else in this package
// builds a command, so there is exactly one place argv is assembled — and every value in
// it is a literal verb or an already-validated value, never a shell string.
var execCommand = exec.Command

// mintTokenFn is the seam the App-token lookup runs through, so a test can exercise the
// verb without a real App credential. Production binds it to the shared deskkit resolver,
// which shells out to the token minter and reads the file it names.
var mintTokenFn = deskkit.RoleTokenForRepo

// ghToken is the App installation token EVERY `gh` invocation from this verb authenticates
// with. It is set once, by the app-token condition, before the first forge read.
//
// WHY AN EMPTY VALUE IS A HARD REFUSAL AND NEVER A FALLBACK. deskflip mutates a PR: it
// takes it out of draft and rewrites the queue labels a human reads to decide what is
// waiting on them. With no token in the child's environment `gh` authenticates as whatever
// account the shell's keyring holds — in practice the operator's own login — so the write
// lands under a HUMAN identity and reads, in the timeline and to everyone after, as a
// human decision. A role verb acting under an operator's credential is exactly the
// ambient-identity lane the custody rules retire, and unlike a failed read it cannot be
// taken back once it is written. So an unset token refuses; it never degrades.
var ghToken string

type runResult struct {
	stdout string
	stderr string
	err    error
}

func runCmd(dir, name string, args ...string) runResult {
	// The fail-closed backstop for the rule above: even if a future code path reached a
	// forge call before the app-token condition ran, the call does not happen. The
	// condition is the check a caller sees; this is the one that cannot be forgotten.
	if name == "gh" && ghToken == "" {
		return runResult{err: errors.New(
			"refusing to run gh with no App installation token — deskflip never falls back to the " +
				"ambient gh identity/keyring for a write it makes under a role identity")}
	}
	cmd := execCommand(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if name == "gh" {
		cmd.Env = append(os.Environ(), "GH_TOKEN="+ghToken)
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
