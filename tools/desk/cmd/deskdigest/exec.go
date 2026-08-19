package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// execCommand is the single seam through which every subprocess flows. Production
// binds it to exec.Command; tests wrap it so the "this path constructs no mutating
// argv" assertions run against the REAL constructed argv rather than against a mock of
// the decision. Nothing else in this package constructs commands.
var execCommand = exec.Command

// runCmd executes name+args and returns trimmed stdout. Commands are ALWAYS built from
// an explicit argv slice — never a shell string. The only external values that reach an
// argv are the repo slug, the issue number, the digest title and a temp body path, each
// in a fixed argv position, so no external input can inject an option.
//
// The subprocess's STDERR is remote-influenced text (gh echoes API messages, which quote
// titles authored by arbitrary users), so it is stripped of control/ANSI sequences at
// this single choke point. Terminal-active bytes never reach a terminal.
func runCmd(name string, args ...string) (string, error) {
	cmd := execCommand(name, args...)
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

// runGH runs a gh subcommand under the AMBIENT gh identity. deskdigest gates WHETHER
// and WHAT, never WHO: the caller's standing credential is the posting identity,
// unchanged. It NEVER injects a token and NEVER mints an App token — there is no
// desktoken call on any path.
//
// It is a variable so tests can record argv and script responses; production binds it
// to the exec seam above.
var runGH = func(args ...string) (string, error) { return runCmd("gh", args...) }

// mutatingGHVerbs are the gh sub-verbs that CHANGE something on the remote. deskdigest
// is allowed exactly two of them and only from the --post path: `issue create` and
// `issue edit`, both against its own weekly digest issue. Everything else on this list
// belongs to another tool — `issue close` to deskclose, `issue comment`/`issue create`
// elsewhere to deskfile, `pr ready` to deskpost — and a deskdigest that reached for one
// would be deciding rather than reporting.
//
// TestReportOnlyNeverMutates enumerates every argv the read paths construct and fails on
// any entry here, which is what makes "the digest reports" a property of the binary
// rather than a claim in a comment.
var mutatingGHVerbs = map[string]bool{
	"close": true, "reopen": true, "edit": true, "create": true, "comment": true,
	"delete": true, "merge": true, "ready": true, "transfer": true, "pin": true,
	"unpin": true, "lock": true, "unlock": true, "add": true, "remove": true,
}

// ghArgvMutates reports whether an argv reaches a mutating gh verb, and names it.
//
// gh's grammar is `gh <noun> <verb> …`, so the verb is at index 1 and NOT wherever a
// keyword happens to appear: `gh label list` is a read whose noun collides with nothing,
// and a scan that matched any position would flag `gh issue list --search "… create …"`
// on the contents of a search string. Position is what makes this check mean something.
//
// `gh api` is judged on its METHOD instead: a bare `gh api <path>` is a GET, and anything
// carrying -X/--method with a non-GET verb is a write. That flag is scanned in every
// position because it is a flag, not a verb.
func ghArgvMutates(argv []string) (string, bool) {
	for i, a := range argv {
		if (a == "-X" || a == "--method") && i+1 < len(argv) && !strings.EqualFold(argv[i+1], "GET") {
			return "api -X " + argv[i+1], true
		}
		if strings.HasPrefix(a, "--method=") && !strings.EqualFold(strings.TrimPrefix(a, "--method="), "GET") {
			return a, true
		}
	}
	if len(argv) >= 2 && argv[0] != "api" && mutatingGHVerbs[argv[1]] {
		return argv[0] + " " + argv[1], true
	}
	return "", false
}

// allowWrite, allowWriteRepoWide and bodyCheck are deskkit's outward-write guards, behind
// variables so the guard-wiring tests can prove each one is CONSULTED — a stub that errors
// must stop the write — without manufacturing audit lines or a body that trips the real
// scanner. Deleting the call site (not the stub) is then a mutation the suite catches: the
// write proceeds despite the stubbed refusal and the test sees the create it was promised
// would not happen.
var (
	allowWrite = func(repo string, number int) error {
		return deskkit.AllowWrite(toolName, repo, number)
	}
	allowWriteRepoWide = func(repo string) error {
		return deskkit.AllowWriteRepoWide(toolName, repo)
	}
	bodyCheck = func(body []byte) error {
		return deskkit.BodyCheck(body)
	}
)
