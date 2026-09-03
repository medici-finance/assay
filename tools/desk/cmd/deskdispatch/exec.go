package main

import (
	"bytes"
	"errors"
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

type runResult struct {
	stdout string
	stderr string
	err    error
}

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
