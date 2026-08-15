package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// execCommand is the single seam through which every subprocess flows. Production
// binds it to exec.Command; tests wrap it so the "zero write calls on a refusal path"
// assertions run against the REAL constructed argv rather than against a mock of the
// decision. Nothing else in this package constructs commands.
var execCommand = exec.Command

// runCmd executes name+args and returns trimmed stdout. Commands are ALWAYS built from
// an explicit argv slice — never a shell string, and never a caller-supplied flag: the
// only external values that reach an argv are the repo, the item number and the close
// reason, each in a fixed argv position, so no external input can inject an option.
//
// The subprocess's STDERR is remote-influenced text (gh echoes API messages, which
// quote titles authored by arbitrary users on the public repos in the fixed set), so it
// is stripped of control/ANSI sequences at this single choke point. Terminal-active
// bytes never reach a terminal.
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

// runGH runs a gh subcommand under the AMBIENT gh identity. deskclose gates WHETHER
// and WHAT, never WHO: the caller's standing credential is the closing identity,
// unchanged. It NEVER injects a token and NEVER mints an App token — there is no
// desktoken call on any path.
//
// It is a variable so tests can record argv and script responses; production binds it
// to the exec seam above.
var runGH = func(args ...string) (string, error) { return runCmd("gh", args...) }

// runDisposition shells out to the SIBLING tool that owns the disposition schema
// (ground-truth/05's deskdisposition). deskclose deliberately does not re-implement
// the marker parser: the record has exactly one declared reader, and a second copy of
// the parse is a second thing to drift. A missing binary is could-not-check, not
// "no record" — see readDisposition.
var runDisposition = func(args ...string) (string, error) { return runCmd(dispositionBin, args...) }

// dispositionBin is the sibling tool's name as installed alongside deskclose.
const dispositionBin = "deskdisposition"

// allowWrite is deskkit's outward-write meter, behind a variable so the manifest
// resume test can inject a real exit-4 without manufacturing 10 audit lines.
var allowWrite = func(repo string, number int) error {
	return deskkit.AllowWrite(toolName, repo, number)
}

const toolName = "deskclose"
