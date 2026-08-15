package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// exec.go — the single seam every subprocess flows through.
//
// Production binds execCommand to exec.Command. Tests wrap it so the load-bearing
// assertions ("zero pushes on a refusal path", "never `gh pr merge`", "never a push
// whose refspec targets a default branch") run against the REAL constructed argv rather
// than against a mock of the decision. Nothing else in this package constructs commands.
var execCommand = exec.Command

// runCmdIn executes name+args with cwd=dir and returns trimmed stdout. Commands are
// ALWAYS built from an explicit argv slice — never a shell string — so no external
// value can inject an option: the repo, the PR number and the ref names each occupy a
// fixed argv position.
//
// The subprocess's STDERR is remote-influenced text (git echoes branch and commit
// subjects authored by arbitrary users; gh echoes API messages quoting titles from the
// public repos in the fixed set), so it is stripped of control/ANSI sequences at this
// single choke point. Terminal-active bytes never reach a terminal.
func runCmdIn(dir, name string, args ...string) (string, error) {
	cmd := execCommand(name, args...)
	cmd.Dir = dir
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	stdout := strings.TrimSpace(out.String())
	if err != nil {
		return stdout, fmt.Errorf("%s %s: %w (%s)", name, strings.Join(args, " "),
			err, deskkit.StripControl(strings.TrimSpace(errb.String())))
	}
	return stdout, nil
}

// runGit runs git with cwd=dir. It is a variable so tests can record argv; production
// binds it to the exec seam above.
//
// Note it does NOT use `git -C`: the working directory is set on the process. A tool
// that threads -C through every call eventually forgets one and silently operates on
// the desk's own tree, which is the single outcome the temp-worktree rule exists to
// prevent.
var runGit = func(dir string, args ...string) (string, error) { return runCmdIn(dir, "git", args...) }

// runGH runs a gh subcommand under the AMBIENT gh identity, for READS only. deskmerge
// makes no mutating gh call on any path — its only write is a `git push` of the PR's
// own branch. There is no `gh pr merge`, no `gh pr ready`, no `gh api -X`.
var runGH = func(args ...string) (string, error) { return runCmdIn("", "gh", args...) }

// allowWrite is deskkit's outward-write meter, behind a variable so tests can inject a
// real exit-4 without manufacturing ten audit lines.
var allowWrite = func(repo string, pr int) error { return deskkit.AllowWrite(toolName, repo, pr) }

// auditLog is deskkit's audit appender, behind a variable so tests can assert the line
// a run wrote without writing to the operator's real audit file.
var auditLog = func(e deskkit.Entry) error { return deskkit.Log(e) }
