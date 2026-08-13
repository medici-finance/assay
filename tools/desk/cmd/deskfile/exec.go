package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// execCommand is the single seam through which every gh invocation flows. Production
// binds it to exec.Command; tests wrap it to RECORD every argv so the "zero write calls
// on a refusal path" and "the only mutating gh verb is `issue create`/`issue comment`"
// assertions run against the real constructed argv. Nothing else in this package
// constructs commands, so there is exactly one place argv is built.
var execCommand = exec.Command

// runCmd executes name+args and returns trimmed stdout. Commands are ALWAYS built from
// an explicit argv slice — never a shell string and never a caller-supplied gh flag.
// deskfile forwards NO caller flag into gh: the only external values that reach an argv
// are the repo, title, issue number and body-file path, each placed in a fixed argv
// position, so no external input can inject an option.
//
// The subprocess's STDERR is remote-influenced text (gh echoes API messages, and API
// messages quote issue titles authored by arbitrary users on the two PUBLIC repos in the
// fixed set) and it is printed verbatim by main when the error surfaces. It is stripped of
// control/ANSI sequences here — the single choke point every gh error passes through — so
// no call site can leak it by forgetting. Terminal-active bytes never reach a terminal.
// deskkit.StripControl keeps tab and newline, so multi-line gh diagnostics stay readable.
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

// gh runs a gh subcommand under the AMBIENT gh identity. deskfile gates WHETHER and WHERE
// an issue is filed, never WHO: the caller's standing gh credential —
// worker session or human — is the filing identity, unchanged. It NEVER injects a token
// and NEVER mints an App installation token; there is no desktoken call on any path.
func gh(args ...string) (string, error) {
	return runCmd("gh", args...)
}
