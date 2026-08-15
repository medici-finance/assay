package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// execCommand is the single seam through which every gh invocation flows. Production
// binds it to exec.Command; tests wrap it to RECORD every argv, so "no outward write on a
// refusal path" and "the only mutating gh verbs are pr edit / pr comment / label create"
// are asserted against the real constructed argv rather than inferred.
var execCommand = exec.Command

// gh runs a gh subcommand under the AMBIENT gh identity.
//
// Commands are ALWAYS built from an explicit argv slice — never a shell string and never
// a caller-supplied gh flag. The only external values that reach an argv are the repo,
// the PR number, the verdict label (drawn from a CLOSED vocabulary, so it cannot be an
// option) and a temp-file path this process created, each in a fixed argv position.
//
// The subprocess's stderr is remote-influenced text (gh echoes API messages, which quote
// titles authored by arbitrary users on the public repos in the set). It is stripped of
// control/ANSI sequences here — the one choke point every gh error passes through — so no
// call site can leak terminal-active bytes by forgetting.
func gh(args ...string) (string, error) {
	cmd := execCommand("gh", args...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return strings.TrimSpace(out.String()), fmt.Errorf("gh %s: %w (%s)",
			strings.Join(args, " "), err, deskkit.StripControl(strings.TrimSpace(errb.String())))
	}
	return strings.TrimSpace(out.String()), nil
}
