package main

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
	"github.com/medici-finance/assay/tools/desk/internal/gitexec"
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

// gitexecVerbs is deskmerge's fenced set — every verb below is a git argv[0] runGit
// still routes to the git BINARY, via the one audited fallback seam
// (internal/gitexec), rather than to gitcore. Each is here for a proven reason, not a
// deferral; see internal/gitexec.allowlist's doc for the full explanation of each:
//
//   - "merge"    — the trial merge itself (--no-ff --no-commit) and its --abort rollback.
//   - "diff"     — ONLY the --diff-filter=U conflict-path reads (enumeration after a
//     failed trial merge, and the residual-hunks check after regenerable resolution),
//     which read the same mid-merge conflict-stage index the trial merge produces.
//     Every other deskmerge `diff` (CI-contract drift, the semantic-probe's
//     changed-path scoping) is on gitcore.DiffNames and never reaches runGit.
//   - "add"      — ONLY the regenerable-conflict resolution stage: verified empirically
//     that go-git cannot clear a path's conflict-stage index entries (see
//     internal/gitcore/write.go's doc), so this one add stays beside the merge it
//     resolves.
//   - "worktree" — the scratch-worktree family (add/remove/prune); linked worktrees are
//     a separate, larger go-git gap, explicitly out of scope for this brief and left to
//     the named follow-on stream.
//   - "fetch", "push" — the transport verbs; not yet migrated (briefs 05 and 06 own them).
var gitexecVerbs = map[string]bool{
	"merge": true, "diff": true, "add": true, "worktree": true,
	"fetch": true, "push": true,
}

// runGit runs git with cwd=dir. It is a variable so tests can record argv; production
// dispatches each call by verb: the fenced set above goes through internal/gitexec —
// the ONE audited git-binary fallback, allowlist-checked before anything spawns; any
// other verb reaching here would be a bug (every migrated verb now calls gitcore
// directly, in assess.go / currency.go / merge.go, and never reaches this seam at
// all), so it falls through to the plain exec seam rather than refusing outright,
// matching this function's pre-migration behaviour for whatever such a caller intended.
//
// Note it does NOT use `git -C`: the working directory is set on the process. A tool
// that threads -C through every call eventually forgets one and silently operates on
// the desk's own tree, which is the single outcome the temp-worktree rule exists to
// prevent.
var runGit = func(dir string, args ...string) (string, error) {
	if len(args) > 0 && gitexecVerbs[args[0]] {
		out, err := gitexec.Run(toolName, dir, args...)
		if err != nil {
			// gitexec's own error does not scrub stderr (it is a low-level, tool-agnostic
			// seam); deskmerge's stderr is remote-influenced (see runCmdIn's own doc), so
			// the same scrub applies here, at the point deskmerge re-enters its own error
			// handling.
			return out, errors.New(deskkit.StripControl(err.Error()))
		}
		return out, nil
	}
	return runCmdIn(dir, "git", args...)
}

// gitcoreCommit is the seam through which commitMerge writes the merge commit — the
// gitcore analogue of runGit, kept as a var for the same reason: a test can replace it
// to prove the surrounding checks (verifyTwoParent above all) catch a differently-shaped
// result, driven through the SAME real git tooling the fixture already uses elsewhere
// rather than a mock of the decision.
var gitcoreCommit = func(dir string, opts gitcore.CommitOpts) (string, error) {
	repo, err := gitcore.Open(dir)
	if err != nil {
		return "", err
	}
	return repo.Commit(opts)
}

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
